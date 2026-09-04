package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"

	"aiku-daemon/internal/config"
	"aiku-daemon/internal/logger"
	"aiku-daemon/internal/supervisor"
	"aiku-daemon/internal/updater"
)

func main() {
	log.Printf("[MAIN] Starting Indogaro Core Service Daemon (%s)", updater.CurrentVersion)

	// Dapatkan root data directory
	dataDir := os.Getenv("ANDROID_DATA_DIR")
	if dataDir == "" {
		dataDir = os.Getenv("HOME")
	}
	if dataDir == "" {
		var err error
		dataDir, err = os.Getwd()
		if err != nil {
			dataDir = "."
		}
	}

	// 1. Inisialisasi Config
	cfg, err := config.LoadConfig(dataDir)
	if err != nil {
		log.Printf("[MAIN] Warning loading config: %v (using defaults)", err)
	}

	// 2. Inisialisasi Logger
	appLogger := logger.NewAPILogger()

	// 3. Inisialisasi Context
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// 4. Inisialisasi Supervisor
	sup := supervisor.NewSupervisor(cfg, appLogger, dataDir)

	// Jalankan supervisor di background goroutine
	go sup.Run(ctx)

	// 5. Inisialisasi Background Auto-Updater (GitHub Releases)
	repoOwner := "indogaro"
	repoName := "service"
	if cfg != nil {
		if envOwner := os.Getenv("GITHUB_REPO_OWNER"); envOwner != "" {
			repoOwner = envOwner
		}
		if envRepo := os.Getenv("GITHUB_REPO_NAME"); envRepo != "" {
			repoName = envRepo
		}
	}
	updater.StartAutoUpdater(dataDir, repoOwner, repoName)

	// 6. Graceful Shutdown Signal Handler
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
	<-sigChan

	log.Println("[MAIN] Shutting down Indogaro Core Service Daemon...")
	cancel()
}
