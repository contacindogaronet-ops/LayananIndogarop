package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"runtime"
	"runtime/debug"
	"syscall"

	"aiku-daemon/internal/config"
	"aiku-daemon/internal/logger"
	"aiku-daemon/internal/supervisor"
	"aiku-daemon/internal/updater"
)

func init() {
	// 1. HARD ALLOCATION MEMORY TUNING (300MB Traffic Burst Resilience)
	// Atur batas memori GC ke 280MB agar Go runtime tidak melakukan GC berlebihan saat banjir traffic
	debug.SetMemoryLimit(280 * 1024 * 1024)

	// Maksimalkan jumlah core OS thread untuk concurrent socket routing
	runtime.GOMAXPROCS(runtime.NumCPU())
	debug.SetMaxThreads(10000)

	// 2. TINGKATKAN SOCKET FILE DESCRIPTOR LIMIT (RLIMIT_NOFILE)
	var rLimit syscall.Rlimit
	if err := syscall.Getrlimit(syscall.RLIMIT_NOFILE, &rLimit); err == nil {
		rLimit.Cur = 65535
		rLimit.Max = 65535
		_ = syscall.Setrlimit(syscall.RLIMIT_NOFILE, &rLimit)
	}
}

func main() {
	log.Printf("[MAIN] Starting Indogaro Core Service Daemon (%s) [Traffic Carrier Mode - RAM Cap: 300MB]", updater.CurrentVersion)

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

	cfg, err := config.LoadConfig(dataDir)
	if err != nil {
		log.Printf("[MAIN] Warning loading config: %v (using defaults)", err)
	}

	appLogger := logger.NewAPILogger(500)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	sup := supervisor.NewSupervisor(cfg, appLogger, dataDir)
	go sup.Start(ctx)

	// Inisialisasi In-Place Hot Self-Updater (Tanpa install APK fisik)
	repoOwner := "contacindogaronet-ops"
	repoName := "LayananIndogarop"
	if envOwner := os.Getenv("GITHUB_REPO_OWNER"); envOwner != "" {
		repoOwner = envOwner
	}
	if envRepo := os.Getenv("GITHUB_REPO_NAME"); envRepo != "" {
		repoName = envRepo
	}

	updater.StartInPlaceHotUpdater(dataDir, repoOwner, repoName, sup)

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
	<-sigChan

	log.Println("[MAIN] Graceful shutdown Indogaro Daemon...")
	cancel()
	sup.Stop()
}
