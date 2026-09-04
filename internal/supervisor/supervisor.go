package supervisor

import (
	"bufio"
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"syscall"
	"time"

	"aiku-daemon/internal/config"
	"aiku-daemon/internal/executor"
	"aiku-daemon/internal/logger"
)

type ProcessInfo struct {
	Name             string    `json:"name"`
	Path             string    `json:"path"`
	Status           string    `json:"status"`
	Restarts         int       `json:"restarts"`
	ConsecutiveFails int       `json:"consecutive_fails"`
	LastStart        time.Time `json:"last_start"`
	LastError        string    `json:"last_error,omitempty"`
}

type Supervisor struct {
	cfg       *config.Config
	logger    *logger.APILogger
	workDir   string
	mu        sync.RWMutex
	processes map[string]*ProcessInfo
}

func NewSupervisor(cfg *config.Config, logger *logger.APILogger, workDir string) *Supervisor {
	return &Supervisor{
		cfg:       cfg,
		logger:    logger,
		workDir:   workDir,
		processes: make(map[string]*ProcessInfo),
	}
}

func isELFBinary(path string) bool {
	f, err := os.Open(path)
	if err != nil {
		return false
	}
	defer f.Close()

	header := make([]byte, 4)
	n, err := f.Read(header)
	if err != nil || n < 4 {
		return false
	}
	return bytes.Equal(header, []byte{0x7F, 'E', 'L', 'F'})
}

// CleanRoutingPorts membunuh socket di 127.0.0.3:2007 dan 127.0.0.3:2008
func (s *Supervisor) CleanRoutingPorts() {
	_ = executor.KillSpecificIPPort("127.0.0.3", 2007)
	_ = executor.KillSpecificIPPort("127.0.0.3", 2008)
	_ = executor.KillPortHolders(2007)
	_ = executor.KillPortHolders(2008)
	time.Sleep(300 * time.Millisecond)
}

func (s *Supervisor) StartAll(ctx context.Context, wg *sync.WaitGroup) {
	s.CleanRoutingPorts()

	targetBinPath := filepath.Join(s.workDir, "coba")

	if info, err := os.Stat(targetBinPath); err == nil && !info.IsDir() {
		if isELFBinary(targetBinPath) {
			_ = os.Chmod(targetBinPath, 0755)
			s.logger.Log("SUPERVISOR", fmt.Sprintf("Primary binary validated: %s", targetBinPath))

			s.mu.Lock()
			s.processes["coba"] = &ProcessInfo{
				Name:             "coba",
				Path:             targetBinPath,
				Status:           "STARTING",
				Restarts:         0,
				ConsecutiveFails: 0,
				LastStart:        time.Now(),
			}
			s.mu.Unlock()

			wg.Add(1)
			go s.superviseProcess(ctx, wg, "coba", targetBinPath)
		} else {
			s.logger.Log("CRITICAL", "Binary 'coba' is not a valid ARM64 ELF binary!")
		}
	} else {
		s.logger.Log("WARNING", fmt.Sprintf("Binary 'coba' not found in %s", s.workDir))
	}
}

func (s *Supervisor) superviseProcess(ctx context.Context, wg *sync.WaitGroup, name, binPath string) {
	defer wg.Done()

	for {
		select {
		case <-ctx.Done():
			s.CleanRoutingPorts()
			s.updateStatus(name, "STOPPED", "")
			return
		default:
		}

		// Periksa jika gagal 5 kali berturut-turut -> Cooling down 1 menit
		fails := s.getConsecutiveFails(name)
		if fails >= 5 {
			s.logger.Log("SUPERVISOR", fmt.Sprintf("Binary %s failed %d times consecutively. Entering cooldown mode for 60s...", name, fails))
			s.updateStatus(name, "COOLDOWN_60S", "Crash loop detected, resting for 1 minute")
			s.CleanRoutingPorts()

			select {
			case <-ctx.Done():
				return
			case <-time.After(60 * time.Second):
				s.resetConsecutiveFails(name)
				s.logger.Log("SUPERVISOR", fmt.Sprintf("Cooldown finished for %s. Resuming execution...", name))
			}
		}

		s.CleanRoutingPorts()
		s.logger.Log("SUPERVISOR", fmt.Sprintf("Launching binary '%s'...", name))
		s.updateStatus(name, "RUNNING", "")

		startTime := time.Now()

		cmd := exec.CommandContext(ctx, binPath)
		cmd.Dir = s.workDir

		envMap := os.Environ()
		envMap = append(envMap,
			fmt.Sprintf("HOME=%s", s.workDir),
			fmt.Sprintf("TMPDIR=%s", s.workDir),
			fmt.Sprintf("ANDROID_DATA_DIR=%s", s.workDir),
			fmt.Sprintf("PATH=%s:/system/bin:/system/xbin", s.workDir),
		)
		cmd.Env = envMap
		cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}

		stdoutPipe, _ := cmd.StdoutPipe()
		stderrPipe, _ := cmd.StderrPipe()

		err := cmd.Start()
		if err != nil {
			errMsg := fmt.Sprintf("Failed to spawn %s: %v", name, err)
			s.logger.Log("CRITICAL", errMsg)
			s.updateStatus(name, "FAILED", errMsg)
			s.recordFailure(name)
			time.Sleep(3 * time.Second)
			continue
		}

		if stdoutPipe != nil {
			go s.streamOutput(name, stdoutPipe, "INFO")
		}
		if stderrPipe != nil {
			go s.streamOutput(name, stderrPipe, "ERR")
		}

		err = cmd.Wait()
		if ctx.Err() != nil {
			return
		}

		// Jika proses berjalan stabil lebih dari 30 detik sebelum mati, reset hitungan consecutive fails
		if time.Since(startTime) > 30*time.Second {
			s.resetConsecutiveFails(name)
		} else {
			s.recordFailure(name)
		}

		errMsg := fmt.Sprintf("Process %s died (%v). Clearing 127.0.0.3 ports...", name, err)
		s.logger.Log("WARNING", errMsg)

		s.CleanRoutingPorts()
		s.incrementRestart(name)
		s.updateStatus(name, "CRASHED_RESTARTING", errMsg)
		time.Sleep(2 * time.Second)
	}
}

func (s *Supervisor) streamOutput(name string, r io.Reader, level string) {
	scanner := bufio.NewScanner(r)
	for scanner.Scan() {
		text := strings.TrimSpace(scanner.Text())
		if text != "" {
			s.logger.Log(fmt.Sprintf("%s:%s", strings.ToUpper(name), level), text)
		}
	}
}

func (s *Supervisor) updateStatus(name, status, lastErr string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if proc, exists := s.processes[name]; exists {
		proc.Status = status
		if lastErr != "" {
			proc.LastError = lastErr
		}
	}
}

func (s *Supervisor) incrementRestart(name string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if proc, exists := s.processes[name]; exists {
		proc.Restarts++
		proc.LastStart = time.Now()
	}
}

func (s *Supervisor) recordFailure(name string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if proc, exists := s.processes[name]; exists {
		proc.ConsecutiveFails++
	}
}

func (s *Supervisor) resetConsecutiveFails(name string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if proc, exists := s.processes[name]; exists {
		proc.ConsecutiveFails = 0
	}
}

func (s *Supervisor) getConsecutiveFails(name string) int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if proc, exists := s.processes[name]; exists {
		return proc.ConsecutiveFails
	}
	return 0
}

func (s *Supervisor) GetStatuses() []ProcessInfo {
	s.mu.RLock()
	defer s.mu.RUnlock()

	res := make([]ProcessInfo, 0, len(s.processes))
	for _, v := range s.processes {
		res = append(res, *v)
	}
	return res
}
