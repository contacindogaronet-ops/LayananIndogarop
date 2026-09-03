package supervisor

import (
	"bytes"
	"context"
	"fmt"
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

// isELFBinary mengecek apakah file adalah biner Linux/Android ELF asli (bukan xml/json/log)
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
	// Bersihkan port routing v2rayNG (2007)
	_ = executor.KillPortHolders(2007)

	targetBinPath := filepath.Join(s.workDir, "coba")

	// 1. Eksekusi target utama biner: coba
	if info, err := os.Stat(targetBinPath); err == nil && !info.IsDir() {
		if isELFBinary(targetBinPath) {
			_ = os.Chmod(targetBinPath, 0755)
			s.logger.Log("SUPERVISOR", fmt.Sprintf("Registered primary ELF binary: %s", targetBinPath))

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
			s.logger.Log("CRITICAL", fmt.Sprintf("File 'coba' is NOT a valid ELF binary!"))
		}
	} else {
		s.logger.Log("WARNING", fmt.Sprintf("Target binary 'coba' not found in %s", s.workDir))
	}

	// 2. Scan jika ada ELF binary tambahan khusus (abaikan .xml, .json, .log, .yaml, .env, .dat, .txt)
	entries, err := os.ReadDir(s.workDir)
	if err == nil {
		for _, entry := range entries {
			name := entry.Name()
			if entry.IsDir() || name == "coba" || name == "aiku-daemon" {
				continue
			}

			// Blacklist ekstensi non-binary
			if strings.HasSuffix(name, ".xml") || strings.HasSuffix(name, ".json") ||
				strings.HasSuffix(name, ".log") || strings.HasSuffix(name, ".yaml") ||
				strings.HasSuffix(name, ".env") || strings.HasSuffix(name, ".dat") ||
				strings.HasSuffix(name, ".txt") || strings.HasPrefix(name, ".") {
				continue
			}

			binPath := filepath.Join(s.workDir, name)
			if isELFBinary(binPath) {
				_ = os.Chmod(binPath, 0755)
				s.logger.Log("SUPERVISOR", fmt.Sprintf("Registered extra ELF binary: %s", name))

				s.mu.Lock()
				s.processes[name] = &ProcessInfo{
					Name:      name,
					Path:      binPath,
					Status:    "STARTING",
					Restarts:  0,
					LastStart: time.Now(),
				}
				s.mu.Unlock()

				wg.Add(1)
				go s.superviseProcess(ctx, wg, name, binPath)
			}
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

		s.logger.Log("SUPERVISOR", fmt.Sprintf("Executing binary '%s' (Dir: %s)", name, s.workDir))
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
