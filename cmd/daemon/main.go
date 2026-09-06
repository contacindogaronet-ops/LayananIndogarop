package main

import (
	"context"
	"flag"
	"os"
	"os/signal"
	"syscall"

	"aiku-daemon/internal/config"
	"aiku-daemon/internal/executor"
	"aiku-daemon/internal/logger"
	"aiku-daemon/internal/supervisor"
	"aiku-daemon/internal/updater"
)

var (
	Version   = "v1.0.0"
	GitCommit = "unknown"
)

func main() {
	configPath := flag.String("config", "config.yaml", "Path ke file konfigurasi YAML")
	flag.Parse()

	// Inisialisasi Logger
	logger.InitLogger()
	log := logger.GetLogger()
	log.Info().
		Str("version", Version).
		Str("commit", GitCommit).
		Msg("Memulai Aiku / Indogaro Autonomous Core Daemon")

	// Inisialisasi Konfigurasi
	cfg, err := config.LoadConfig(*configPath)
	if err != nil {
		log.Warn().Err(err).Msg("Gagal memuat config, menggunakan konfigurasi fallback bawaan")
		cfg = config.DefaultConfig()
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Inisialisasi Engine Executor
	execEngine := executor.NewEngine(cfg)

	// Inisialisasi System Supervisor
	sup := supervisor.NewSupervisor(cfg, execEngine)
	if err := sup.Start(ctx); err != nil {
		log.Fatal().Err(err).Msg("Supervisor gagal dijalankan")
	}

	// Jalankan Autonomous In-Place Hot Updater
	go updater.StartInPlaceHotUpdater(ctx, cfg, Version)

	// Tangani Shutdown Signals (Graceful Shutdown)
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM, syscall.SIGHUP)

	sig := <-sigChan
	log.Info().Str("signal", sig.String()).Msg("Menerima sinyal termination, memulai graceful shutdown...")

	cancel()
	sup.Stop()

	log.Info().Msg("Aiku Core Daemon berhenti dengan aman.")
}