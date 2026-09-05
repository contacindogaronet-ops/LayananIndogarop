package supervisor

import (
	"context"
	"encoding/json"
	"io"
	"log"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"sync"
	"time"

	"aiku-daemon/internal/config"
	"aiku-daemon/internal/logger"
)

type EngineState struct {
	Status        string `json:"status"`
	UptimeSeconds int64  `json:"uptime_seconds"`
	PrimaryPort   string `json:"primary_port"`
	SecondaryPort string `json:"secondary_port"`
	PrimaryLive   bool   `json:"primary_live"`
	SecondaryLive bool   `json:"secondary_live"`
	PID           int    `json:"binary_pid"`
	AllocatedRAM  string `json:"allocated_ram_limit"`
	LastCheck     string `json:"last_check"`
}

type Supervisor struct {
	cfg       *config.Config
	logger    *logger.APILogger
	dataDir   string
	cmd       *exec.Cmd
	mu        sync.Mutex
	isRunning bool
	cancel    context.CancelFunc
	startTime time.Time
}

func NewSupervisor(cfg *config.Config, l *logger.APILogger, dataDir string) *Supervisor {
	return &Supervisor{
		cfg:     cfg,
		logger:  l,
		dataDir: dataDir,
	}
}

func (s *Supervisor) Start(ctx context.Context) {
	s.mu.Lock()
	if s.isRunning {
		s.mu.Unlock()
		return
	}
	s.isRunning = true
	s.startTime = time.Now()
	runCtx, cancel := context.WithCancel(ctx)
	s.cancel = cancel
	s.mu.Unlock()

	log.Println("[SUPERVISOR] Starting Indogaro High-Capacity Engine...")

	go s.watchdogLoop(runCtx)
	s.processLoop(runCtx)
}

func (s *Supervisor) RestartSubProcess() {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.cmd != nil && s.cmd.Process != nil {
		log.Printf("[SUPERVISOR] Killing active sub-binary PID %d for hot-reload...", s.cmd.Process.Pid)
		_ = s.cmd.Process.Kill()
	}
}

func (s *Supervisor) Stop() {
	s.mu.Lock()
	defer s.mu.Unlock()

	if !s.isRunning {
		return
	}
	s.isRunning = false

	if s.cancel != nil {
		s.cancel()
	}

	if s.cmd != nil && s.cmd.Process != nil {
		_ = s.cmd.Process.Kill()
	}
	s.writeState("STOPPED", false, false, 0)
	log.Println("[SUPERVISOR] Supervisor stopped.")
}

func (s *Supervisor) processLoop(ctx context.Context) {
	cobaBin := filepath.Join(s.dataDir, "coba")

	for {
		select {
		case <-ctx.Done():
			return
		default:
		}

		if _, err := os.Stat(cobaBin); os.IsNotExist(err) {
			time.Sleep(1 * time.Second)
			continue
		}

		_ = os.Chmod(cobaBin, 0755)

		cmd := exec.Command(cobaBin)
		cmd.Dir = s.dataDir
		cmd.Env = append(os.Environ(),
			"ANDROID_DATA_DIR="+s.dataDir,
			"HOME="+s.dataDir,
			"TMPDIR="+s.dataDir,
			"GOMEMLIMIT=280MiB",
		)

		// Pipe stdout/stderr to io.Discard agar tidak blocking pipe buffer
		cmd.Stdout = io.Discard
		cmd.Stderr = io.Discard

		s.mu.Lock()
		s.cmd = cmd
		s.mu.Unlock()

		if err := cmd.Start(); err != nil {
			log.Printf("[SUPERVISOR] Failed spawning 'coba': %v", err)
			time.Sleep(2 * time.Second)
			continue
		}

		log.Printf("[SUPERVISOR] Sub-binary 'coba' running (PID: %d)", cmd.Process.Pid)
		_ = cmd.Wait()

		select {
		case <-ctx.Done():
			return
		default:
			time.Sleep(1 * time.Second)
		}
	}
}

func (s *Supervisor) watchdogLoop(ctx context.Context) {
	ticker := time.NewTicker(6 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			p1 := s.probeSocket("127.0.0.3:2007")
			p2 := s.probeSocket("127.0.0.3:2008")

			pid := 0
			s.mu.Lock()
			if s.cmd != nil && s.cmd.Process != nil {
				pid = s.cmd.Process.Pid
			}
			s.mu.Unlock()

			status := "CARRIER_ACTIVE"
			if !p1 && !p2 {
				status = "DEGRADED"
			}

			s.writeState(status, p1, p2, pid)
		}
	}
}

func (s *Supervisor) probeSocket(addr string) bool {
	conn, err := net.DialTimeout("tcp", addr, 1000*time.Millisecond)
	if err != nil {
		return false
	}
	_ = conn.Close()
	return true
}

func (s *Supervisor) writeState(status string, p1, p2 bool, pid int) {
	state := EngineState{
		Status:        status,
		UptimeSeconds: int64(time.Since(s.startTime).Seconds()),
		PrimaryPort:   "127.0.0.3:2007",
		SecondaryPort: "127.0.0.3:2008",
		PrimaryLive:   p1,
		SecondaryLive: p2,
		PID:           pid,
		AllocatedRAM:  "300MB Dedicated Buffer Pool",
		LastCheck:     time.Now().Format(time.RFC3339),
	}

	data, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		return
	}

	stateFile := filepath.Join(s.dataDir, "state.json")
	_ = os.WriteFile(stateFile, data, 0644)
}
