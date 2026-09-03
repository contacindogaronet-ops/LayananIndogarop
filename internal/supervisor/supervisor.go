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
	binDir    string
	mu        sync.RWMutex
	processes map[string]*ProcessInfo
}

func NewSupervisor(cfg *config.Config, logger *logger.APILogger, binDir string) *Supervisor {
	return &Supervisor{
		cfg:       cfg,
		logger:    logger,
		binDir:    binDir,
		processes: make(map[string]*ProcessInfo),
	}
}

func (s *Supervisor) StartAll(ctx context.Context, wg *sync.WaitGroup) {
	// 1. Bersihkan port jika ada process sebelumnya yang tersangkut
	if err := executor.KillPortHolders(s.cfg.Server.Port); err != nil {
		s.logger.Log("PORT_CLEANER", fmt.Sprintf("Port %d cleanup note: %v", s.cfg.Server.Port, err))
	}

	// 2. Cari binary 'coba' di direktori bin atau root
	candidates := []string{
		filepath.Join(s.binDir, "coba"),
		filepath.Join(filepath.Dir(s.binDir), "coba"),
		"./bin/coba",
		"./coba",
	}

	foundCoba := false
	for _, path := range candidates {
		if info, err := os.Stat(path); err == nil && !info.IsDir() {
			_ = os.Chmod(path, 0755)
			s.logger.Log("SUPERVISOR", fmt.Sprintf("Found target binary 'coba' at %s", path))

			s.mu.Lock()
			s.processes["coba"] = &ProcessInfo{
				Name:      "coba",
				Path:      path,
				Status:    "STARTING",
				Restarts:  0,
				LastStart: time.Now(),
			}
			s.mu.Unlock()

			wg.Add(1)
			go s.superviseProcess(ctx, wg, "coba", path)
			foundCoba = true
			break
		}
	}

	// 3. Scan semua binary lain yang ada di folder bin/
	entries, err := os.ReadDir(s.binDir)
	if err == nil {
		for _, entry := range entries {
			if !entry.IsDir() && entry.Name() != "coba" && entry.Name() != "aiku-daemon" {
				binPath := filepath.Join(s.binDir, entry.Name())
				_ = os.Chmod(binPath, 0755)

				s.mu.Lock()
				s.processes[entry.Name()] = &ProcessInfo{
					Name:      entry.Name(),
					Path:      binPath,
					Status:    "STARTING",
					Restarts:  0,
					LastStart: time.Now(),
				}
				s.mu.Unlock()

				wg.Add(1)
				go s.superviseProcess(ctx, wg, entry.Name(), binPath)
			}
		}
	}

	if !foundCoba {
		s.logger.Log("WARNING", fmt.Sprintf("Binary 'coba' not found yet in %s. Waiting for extraction...", s.binDir))
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

		s.logger.Log("SUPERVISOR", fmt.Sprintf("Spawning process: %s [%s]", name, binPath))
		s.updateStatus(name, "RUNNING", "")

		cmd := exec.CommandContext(ctx, binPath)
		cmd.Dir = filepath.Dir(binPath)
		cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}

		err := cmd.Start()
		if err != nil {
			errMsg := fmt.Sprintf("Failed to exec %s: %v", name, err)
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

		errMsg := fmt.Sprintf("Binary %s exited: %v", name, err)
		s.logger.Log("WARNING", errMsg)
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
