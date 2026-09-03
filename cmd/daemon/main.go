package main

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"sync"
	"syscall"
	"time"

	"aiku-daemon/internal/config"
	"aiku-daemon/internal/logger"
	"aiku-daemon/internal/supervisor"
)

func resolveWorkDir() string {
	if _, err := os.Stat("config.yaml"); err == nil {
		pwd, err := os.Getwd()
		if err == nil {
			return pwd
		}
	}

	exePath, err := os.Executable()
	if err == nil {
		exeDir := filepath.Dir(exePath)
		if _, err := os.Stat(filepath.Join(exeDir, "config.yaml")); err == nil {
			_ = os.Chdir(exeDir)
			return exeDir
		}
		parentDir := filepath.Dir(filepath.Dir(exeDir))
		if _, err := os.Stat(filepath.Join(parentDir, "config.yaml")); err == nil {
			_ = os.Chdir(parentDir)
			return parentDir
		}
	}

	pwd, _ := os.Getwd()
	return pwd
}

func main() {
	rootDir := resolveWorkDir()

	fmt.Println("==================================================")
	fmt.Println("       AIKU DAEMON - ENGINE SUPERVISOR RUNNER    ")
	fmt.Printf("       Working Directory: %s\n", rootDir)
	fmt.Println("==================================================")

	configPath := filepath.Join(rootDir, "config.yaml")
	binDir := filepath.Join(rootDir, "bin")

	// Load Configuration
	cfg, err := config.LoadConfig(configPath)
	if err != nil {
		fmt.Printf("[FATAL] Failed to load config at %s: %v\n", configPath, err)
		os.Exit(1)
	}

	// Initialize Logger & Supervisor
	apiLogger := logger.NewAPILogger(500)
	sup := supervisor.NewSupervisor(cfg, apiLogger, binDir)

	// Register HTTP Endpoints
	http.HandleFunc("/api/v1/logs", apiLogger.ServeHTTPLogs)
	http.HandleFunc("/api/v1/status", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"daemon":    "UP",
			"timestamp": time.Now(),
			"root_dir":  rootDir,
			"processes": sup.GetStatuses(),
		})
	})

	serverAddr := fmt.Sprintf("%s:%d", cfg.Server.Host, cfg.Server.Port)
	server := &http.Server{Addr: serverAddr}

	go func() {
		apiLogger.Log("SYSTEM", fmt.Sprintf("HTTP Telemetry API active on http://%s", serverAddr))
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			apiLogger.Log("FATAL", fmt.Sprintf("API Server crashed: %v", err))
		}
	}()

	// Lifecycle Manager
	ctx, cancel := context.WithCancel(context.Background())
	var wg sync.WaitGroup

	sup.StartAll(ctx, &wg)

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)

	<-sigChan
	apiLogger.Log("SYSTEM", "Shutdown signal received. Stopping binaries...")

	cancel()

	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer shutdownCancel()
	_ = server.Shutdown(shutdownCtx)

	wg.Wait()
	apiLogger.Log("SYSTEM", "Daemon shut down cleanly.")
}
