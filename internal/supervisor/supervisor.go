package supervisor

import (
	"context"
	"fmt"
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

func (s *Supervisor) StartAll(ctx context.Context, wg *sync.WaitGroup) {
	// Auto kill conflicting port 2007 (v2rayNG route)
	_ = executor.KillPortHolders(2007)

	targetBinaries := []string{"coba"}

	// Tambahkan binary lain jika ada di folder kerja
	entries, err := os.ReadDir(s.workDir)
	if err == nil {
		for _, entry := range entries {
			name := entry.Name()
			if !entry.IsDir() && name != "aiku-daemon" && name != "config.yaml" && name != ".env" && name != "brain.dat" && name != "state.json" {
				if !contains(targetBinaries, name) && isExecutable(filepath.Join(s.workDir, name)) {
					targetBinaries = append(targetBinaries, name)
				}
			}
		}
	}

	for _, binName := range targetBinaries {
		binPath := filepath.Join(s.workDir, binName)
		if _, err := os.Stat(binPath); err == nil {
			_ = os.Chmod(binPath, 0777)
			s.logger.Log("SUPERVISOR", fmt.Sprintf("Registering binary '%s' at %s", binName, binPath))

			s.mu.Lock()
			s.processes[binName] = &ProcessInfo{
				Name:      binName,
				Path:      binPath,
				Status:    "STARTING",
				Restarts:  0,
				LastStart: time.Now(),
			}
			s.mu.Unlock()

			wg.Add(1)
			go s.superviseProcess(ctx, wg, binName, binPath)
		} else {
			s.logger.Log("WARNING", fmt.Sprintf("Binary %s not found in %s", binName, binPath))
		}
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

		s.logger.Log("SUPERVISOR", fmt.Sprintf("Executing binary '%s' (WorkingDir: %s)", name, s.workDir))
		s.updateStatus(name, "RUNNING", "")

		cmd := exec.CommandContext(ctx, binPath)
		cmd.Dir = s.workDir // Sejajar dengan .env, brain.dat, state.json, blocklists/
		cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}

		err := cmd.Start()
		if err != nil {
			errMsg := fmt.Sprintf("Failed to spawn %s: %v", name, err)
			s.logger.Log("CRITICAL", errMsg)
			s.updateStatus(name, "FAILED", errMsg)
			s.incrementRestart(name)
			time.Sleep(3 * time.Second)
			continue
		}

		err = cmd.Wait()
		if ctx.Err() != nil {
			return
		}

		errMsg := fmt.Sprintf("Process %s died (%v). Re-killing port 2007 & auto-restarting in 2s...", name, err)
		s.logger.Log("WARNING", errMsg)
		_ = executor.KillPortHolders(2007)

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

func (s *Supervisor) GetStatuses() []ProcessInfo {
	s.mu.RLock()
	defer s.mu.RUnlock()

	res := make([]ProcessInfo, 0, len(s.processes))
	for _, v := range s.processes {
		res = append(res, *v)
	}
	return res
}

func contains(arr []string, target string) bool {
	for _, item := range arr {
		if item == target {
			return true
		}
	}
	return false
}

func isExecutable(path string) bool {
	info, err := os.Stat(path)
	if err != nil {
		return false
	}
	return info.Mode().Perm()&0111 != 0
}
