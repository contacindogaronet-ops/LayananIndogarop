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
	Status    string    `json:"status"`
	Restarts  int       `json:"restarts"`
	LastStart time.Time `json:"last_start"`
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
	// Auto kill conflicting port before binding network routes
	if err := executor.KillPortHolders(s.cfg.Server.Port); err != nil {
		s.logger.Log("PORT_CLEANER", fmt.Sprintf("Port %d cleanup note: %v", s.cfg.Server.Port, err))
	}

	// Scan network binary directory
	entries, err := os.ReadDir(s.binDir)
	if err != nil {
		s.logger.Log("SUPERVISOR", fmt.Sprintf("No routing binaries in %s: %v", s.binDir, err))
		return
	}

	for _, entry := range entries {
		if !entry.IsDir() {
			binPath := filepath.Join(s.binDir, entry.Name())
			
			// Enforce rwxr-xr-x permissions
			_ = os.Chmod(binPath, 0755)

			s.mu.Lock()
			s.processes[entry.Name()] = &ProcessInfo{
				Name:      entry.Name(),
				Status:    "INITIALIZING",
				Restarts:  0,
				LastStart: time.Now(),
			}
			s.mu.Unlock()

			wg.Add(1)
			go s.superviseProcess(ctx, wg, entry.Name(), binPath)
		}
	}
}

func (s *Supervisor) superviseProcess(ctx context.Context, wg *sync.WaitGroup, name, binPath string) {
	defer wg.Done()

	for {
		select {
		case <-ctx.Done():
			s.updateStatus(name, "STOPPED")
			return
		default:
		}

		s.logger.Log("SUPERVISOR", fmt.Sprintf("Executing routing binary: %s", name))
		s.updateStatus(name, "RUNNING")

		cmd := exec.CommandContext(ctx, binPath)
		cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}

		err := cmd.Start()
		if err != nil {
			s.logger.Log("CRITICAL", fmt.Sprintf("Failed to spawn %s: %v. Retrying in 3s...", name, err))
			s.incrementRestart(name)
			time.Sleep(3 * time.Second)
			continue
		}

		err = cmd.Wait()
		if ctx.Err() != nil {
			return
		}

		s.logger.Log("WARNING", fmt.Sprintf("Routing process %s exited (%v). Auto-restarting...", name, err))
		s.incrementRestart(name)
		s.updateStatus(name, "CRASHED_RESTARTING")
		time.Sleep(2 * time.Second)
	}
}

func (s *Supervisor) updateStatus(name, status string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if proc, exists := s.processes[name]; exists {
		proc.Status = status
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
