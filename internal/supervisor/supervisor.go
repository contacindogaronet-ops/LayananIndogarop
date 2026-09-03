package supervisor

import (
	"context"
	"sync"

	"aiku-daemon/internal/config"
	"aiku-daemon/internal/logger"
)

type ProcessStatus struct {
	Name   string `json:"name"`
	Status string `json:"status"`
}

type Supervisor struct {
	cfg    *config.Config
	logger *logger.APILogger
	binDir string
}

func NewSupervisor(cfg *config.Config, logger *logger.APILogger, binDir string) *Supervisor {
	return &Supervisor{
		cfg:    cfg,
		logger: logger,
		binDir: binDir,
	}
}

func (s *Supervisor) StartAll(ctx context.Context, wg *sync.WaitGroup) {
	s.logger.Log("SUPERVISOR", "Starting supervised processes...")
}

func (s *Supervisor) GetStatuses() []ProcessStatus {
	return []ProcessStatus{
		{Name: "daemon", Status: "RUNNING"},
	}
}
