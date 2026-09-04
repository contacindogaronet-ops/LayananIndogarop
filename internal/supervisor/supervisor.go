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
	Name      string    `json:"name"`
	Path      string    `json:"path"`
	Status    string    `json:"status"`
	Restarts  int       `json:"restarts"`
	LastStart time.Time `json:"last_start"`
	LastError string    `json:"last_error,omitempty"`
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

// isELFBinary memvalidasi magic header ELF Linux
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

func (s *Supervisor) StartAll(ctx context.Context, wg *sync.WaitGroup) {
	_ = executor.KillPortHolders(2007)

	targetBinPath := filepath.Join(s.workDir, "coba")

	if info, err := os.Stat(targetBinPath); err == nil && !info.IsDir() {
		if isELFBinary(targetBinPath) {
			_ = os.Chmod(targetBinPath, 0755)
			s.logger.Log("SUPERVISOR", fmt.Sprintf("Primary binary ready: %s", targetBinPath))

			s.mu.Lock()
			s.processes["coba"] = &ProcessInfo{
				Name:      "coba",
				Path:      targetBinPath,
				Status:    "STARTING",
				Restarts:  0,
				LastStart: time.Now(),
			}
			s.mu.Unlock()

			wg.Add(1)
			go s.superviseProcess(ctx, wg, "coba", targetBinPath)
		} else {
			s.logger.Log("CRITICAL", "File 'coba' is not a valid ARM64 ELF binary!")
		}
	} else {
		s.logger.Log("WARNING", fmt.Sprintf("Target binary 'coba' not found in %s", s.workDir))
	}
}

func (s *Supervisor) superviseProcess(ctx context.Context, wg *sync.WaitGroup, name, binPath string) {
	defer wg.Done()

	for {
		select {
		case <-ctx.Done():
			s.updateStatus(name, "STOPPED", "")
			return
		default:
		}

		s.logger.Log("SUPERVISOR", fmt.Sprintf("Launching binary '%s'...", name))
		s.updateStatus(name, "RUNNING", "")

		cmd := exec.CommandContext(ctx, binPath)
		cmd.Dir = s.workDir

		// Inject lengkap Environment Android
		envMap := os.Environ()
		envMap = append(envMap,
			fmt.Sprintf("HOME=%s", s.workDir),
			fmt.Sprintf("TMPDIR=%s", s.workDir),
			fmt.Sprintf("ANDROID_DATA_DIR=%s", s.workDir),
			fmt.Sprintf("PATH=%s:/system/bin:/system/xbin", s.workDir),
		)
		cmd.Env = envMap
		cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}

		// Tangkap stdout dan stderr secara real-time
		stdoutPipe, err := cmd.StdoutPipe()
		if err != nil {
			s.logger.Log("ERROR", fmt.Sprintf("Stdout pipe failed: %v", err))
		}
		stderrPipe, err := cmd.StderrPipe()
		if err != nil {
			s.logger.Log("ERROR", fmt.Sprintf("Stderr pipe failed: %v", err))
		}

		err = cmd.Start()
		if err != nil {
			errMsg := fmt.Sprintf("Failed to spawn %s: %v", name, err)
			s.logger.Log("CRITICAL", errMsg)
			s.updateStatus(name, "FAILED", errMsg)
			s.incrementRestart(name)
			time.Sleep(3 * time.Second)
			continue
		}

		// Stream stdout
		if stdoutPipe != nil {
			go s.streamOutput(name, stdoutPipe, "INFO")
		}
		// Stream stderr
		if stderrPipe != nil {
			go s.streamOutput(name, stderrPipe, "ERR")
		}

		err = cmd.Wait()
		if ctx.Err() != nil {
			return
		}

		errMsg := fmt.Sprintf("Process %s died: %v. Auto-restarting in 2s...", name, err)
		s.logger.Log("WARNING", errMsg)

		// Bersihkan port 2007 jika tertahan oleh zombie process
		_ = executor.KillPortHolders(2007)

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

func (s *Supervisor) GetStatuses() []ProcessInfo {
	s.mu.RLock()
	defer s.mu.RUnlock()

	res := make([]ProcessInfo, 0, len(s.processes))
	for _, v := range s.processes {
		res = append(res, *v)
	}
	return res
}
