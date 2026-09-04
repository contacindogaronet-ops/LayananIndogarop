package supervisor

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net"
	"os"
	"os/exec"
	"path/filepath"
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
			s.logger.Log("SUPERVISOR", fmt.Sprintf("Validated primary binary: %s", targetBinPath))

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

// checkPortAlive melakukan TCP ping ringan untuk memastikan binary tidak hang/deadlock
func checkPortAlive(ipPort string) bool {
	conn, err := net.DialTimeout("tcp", ipPort, 1200*time.Millisecond)
	if err != nil {
		return false
	}
	_ = conn.Close()
	return true
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

		fails := s.getConsecutiveFails(name)
		if fails >= 5 {
			s.logger.Log("SUPERVISOR", fmt.Sprintf("Binary %s crashed 5 times consecutively. Entering 60s cooldown...", name))
			s.updateStatus(name, "COOLDOWN_60S", "Crash loop detected, resting for 1 minute")
			s.CleanRoutingPorts()

			select {
			case <-ctx.Done():
				return
			case <-time.After(60 * time.Second):
				s.resetConsecutiveFails(name)
				s.logger.Log("SUPERVISOR", fmt.Sprintf("Cooldown finished for %s. Resuming...", name))
			}
		}

		s.CleanRoutingPorts()
		s.logger.Log("SUPERVISOR", fmt.Sprintf("Launching binary '%s' (DevNull Silence Mode)...", name))
		s.updateStatus(name, "RUNNING", "")

		startTime := time.Now()

		cmd := exec.CommandContext(ctx, binPath)
		cmd.Dir = s.workDir

		cmd.Env = append(os.Environ(),
			fmt.Sprintf("HOME=%s", s.workDir),
			fmt.Sprintf("TMPDIR=%s", s.workDir),
			fmt.Sprintf("ANDROID_DATA_DIR=%s", s.workDir),
			fmt.Sprintf("PATH=%s:/system/bin:/system/xbin", s.workDir),
		)
		cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}

		// DEV NULL MODE: Membuang total output agar bebas beban CPU & logcat
		cmd.Stdout = io.Discard
		cmd.Stderr = io.Discard

		err := cmd.Start()
		if err != nil {
			errMsg := fmt.Sprintf("Failed to spawn %s: %v", name, err)
			s.logger.Log("CRITICAL", errMsg)
			s.updateStatus(name, "FAILED", errMsg)
			s.recordFailure(name)
			time.Sleep(3 * time.Second)
			continue
		}

		// Watchdog Deadlock Check: Pantau socket 127.0.0.3:2007 secara periodik
		deadlockCtx, stopDeadlockCheck := context.WithCancel(context.Background())
		go func() {
			time.Sleep(4 * time.Second) // Beri waktu warmup 4 detik
			ticker := time.NewTicker(15 * time.Second)
			defer ticker.Stop()

			deadCount := 0
			for {
				select {
				case <-deadlockCtx.Done():
					return
				case <-ticker.C:
					// Ping port 2007 atau 2008
					if !checkPortAlive("127.0.0.3:2007") && !checkPortAlive("127.0.0.3:2008") {
						deadCount++
						if deadCount >= 3 {
							s.logger.Log("WATCHDOG", "Port 2007/2008 unresponsive (deadlock detected). Force restarting...")
							if cmd.Process != nil {
								_ = syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL)
								_ = cmd.Process.Kill()
							}
							return
						}
					} else {
						deadCount = 0
					}
				}
			}
		}()

		err = cmd.Wait()
		stopDeadlockCheck()

		if ctx.Err() != nil {
			return
		}

		if time.Since(startTime) > 30*time.Second {
			s.resetConsecutiveFails(name)
		} else {
			s.recordFailure(name)
		}

		errMsg := fmt.Sprintf("Process %s died (%v). Clearing socket state...", name, err)
		s.logger.Log("WARNING", errMsg)

		s.CleanRoutingPorts()
		s.incrementRestart(name)
		s.updateStatus(name, "CRASHED_RESTARTING", errMsg)
		time.Sleep(2 * time.Second)
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
