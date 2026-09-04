package main

import (
	"log"
	"os"
	"os/signal"
	"syscall"

	"aiku-daemon/internal/config"
	"aiku-daemon/internal/supervisor"
	"aiku-daemon/internal/updater"
)

func main() {
	log.Printf("[MAIN] Starting Indogaro Core Service Engine (%s)", updater.CurrentVersion)

	cfg := config.LoadConfig()

	// Inisialisasi supervisor sub-proses biner
	sup := supervisor.NewSupervisor(cfg)
	sup.Start()

	// Inisialisasi background auto updater
	updater.StartAutoUpdater(cfg.DataDir, cfg.RepoOwner, cfg.RepoName)

	// Graceful shutdown handling
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
	<-sigChan

	log.Println("[MAIN] Shutting down Indogaro Core Service...")
	sup.Stop()
}
