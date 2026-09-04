package supervisor

import (
	"context"
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

type Supervisor struct {
	cfg       *config.Config
	logger    *logger.APILogger
	dataDir   string
	cmd       *exec.Cmd
	mu        sync.Mutex
	isRunning bool
	cancel    context.CancelFunc
}

func NewSupervisor(cfg *config.Config, l *logger.APILogger, dataDir string) *Supervisor {
	return &Supervisor{
		cfg:     cfg,
		logger:  l,
		dataDir: dataDir,
	}
}

// Start menjalankan supervisor lifecycle & watchdog
func (s *Supervisor) Start(ctx context.Context) {
	s.mu.Lock()
	if s.isRunning {
		s.mu.Unlock()
		return
	}
	s.isRunning = true
	runCtx, cancel := context.WithCancel(ctx)
	s.cancel = cancel
	s.mu.Unlock()

	log.Println("[SUPERVISOR] Starting binary supervisor & watchdog loop...")

	go s.watchdogLoop(runCtx)
	s.processLoop(runCtx)
}

// Stop menghentikan seluruh child process dan watchdog
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
	log.Println("[SUPERVISOR] Supervisor stopped successfully.")
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
			time.Sleep(3 * time.Second)
			continue
		}

		_ = os.Chmod(cobaBin, 0755)

		// Zero-IO silence mode: redirect output to /dev/null
		cmd := exec.Command(cobaBin)
		cmd.Dir = s.dataDir
		cmd.Env = append(os.Environ(),
			"ANDROID_DATA_DIR="+s.dataDir,
			"HOME="+s.dataDir,
			"TMPDIR="+s.dataDir,
		)

		devNull, err := os.OpenFile(os.DevNull, os.O_WRONLY, 0)
		if err == nil {
			cmd.Stdout = devNull
			cmd.Stderr = devNull
		}

		s.mu.Lock()
		s.cmd = cmd
		s.mu.Unlock()

		if err := cmd.Start(); err != nil {
			log.Printf("[SUPERVISOR] Error starting 'coba': %v", err)
			time.Sleep(2 * time.Second)
			continue
		}

		log.Printf("[SUPERVISOR] 'coba' binary started with PID %d", cmd.Process.Pid)
		_ = cmd.Wait()

		if devNull != nil {
			_ = devNull.Close()
		}

		select {
		case <-ctx.Done():
			return
		default:
			time.Sleep(1 * time.Second)
		}
	}
}

func (s *Supervisor) watchdogLoop(ctx context.Context) {
	ticker := time.NewTicker(15 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			s.checkPortDeadlock("127.0.0.3:2007")
			s.checkPortDeadlock("127.0.0.3:2008")
		}
	}
}

func (s *Supervisor) checkPortDeadlock(addr string) {
	conn, err := net.DialTimeout("tcp", addr, 2*time.Second)
	if err != nil {
		// Port freeze / unresponsive
		return
	}
	_ = conn.Close()
}
