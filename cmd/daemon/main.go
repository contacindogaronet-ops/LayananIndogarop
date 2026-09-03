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
	"aiku-daemon/internal/executor"
	"aiku-daemon/internal/logger"
	"aiku-daemon/internal/supervisor"
)

func resolveWorkDir() string {
	if dataDir := os.Getenv("ANDROID_DATA_DIR"); dataDir != "" {
		_ = os.Chdir(dataDir)
		return dataDir
	}

	exePath, err := os.Executable()
	if err == nil {
		exeDir := filepath.Dir(exePath)
		_ = os.Chdir(exeDir)
		return exeDir
	}

	pwd, _ := os.Getwd()
	return pwd
}

func main() {
	rootDir := resolveWorkDir()
	configPath := filepath.Join(rootDir, "config.yaml")

	cfg, err := config.LoadConfig(configPath)
	if err != nil || cfg == nil {
		cfg = &config.Config{}
	}

	cfg.Server.Host = "0.0.0.0"
	if cfg.Server.Port == 0 {
		cfg.Server.Port = 8080
	}

	// 1. Bersihkan Port Telemetri (8080) dan Port Routing Binary Coba (2007)
	_ = executor.KillPortHolders(cfg.Server.Port)
	_ = executor.KillPortHolders(2007)

	apiLogger := logger.NewAPILogger(500)
	apiLogger.Log("CORE", fmt.Sprintf("Aiku pure daemon started at %s", rootDir))

	// Supervisor menjalankan binary 'coba' sejajar di rootDir
	sup := supervisor.NewSupervisor(cfg, apiLogger, rootDir)

	// API Status
	http.HandleFunc("/api/v1/status", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("Access-Control-Allow-Origin", "*")
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"daemon":    "UP",
			"timestamp": time.Now(),
			"port":      cfg.Server.Port,
			"root_dir":  rootDir,
			"processes": sup.GetStatuses(),
		})
	})

	// API Logs
	http.HandleFunc("/api/v1/logs", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		apiLogger.ServeHTTPLogs(w, r)
	})

	// API Shell Exec
	http.HandleFunc("/api/v1/exec", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		if r.Method != http.MethodPost {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		var payload struct {
			Command string `json:"command"`
			Timeout int    `json:"timeout"`
		}
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		if payload.Timeout == 0 {
			payload.Timeout = 5
		}

		out, err := executor.ExecShell(payload.Command, time.Duration(payload.Timeout)*time.Second)
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"output": out,
			"error":  fmt.Sprint(err),
		})
	})

	// Root Ping Handler
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, _ = fmt.Fprintf(w, "<h2>✓ Aiku Network Core is RUNNING</h2><p>Working Dir: %s</p><p><a href='/api/v1/status'>Check Status JSON</a> | <a href='/api/v1/logs'>Check Logs</a></p>", rootDir)
	})

	serverAddr := fmt.Sprintf("%s:%d", cfg.Server.Host, cfg.Server.Port)
	server := &http.Server{Addr: serverAddr}

	go func() {
		apiLogger.Log("SYSTEM", fmt.Sprintf("HTTP Telemetry listening on %s (0.0.0.0:%d)", serverAddr, cfg.Server.Port))
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			apiLogger.Log("FATAL", fmt.Sprintf("HTTP Server bind failed: %v", err))
		}
	}()

	ctx, cancel := context.WithCancel(context.Background())
	var wg sync.WaitGroup

	sup.StartAll(ctx, &wg)

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)

	<-sigChan
	apiLogger.Log("SYSTEM", "Aiku Core shutting down...")

	cancel()
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer shutdownCancel()
	_ = server.Shutdown(shutdownCtx)

	wg.Wait()
}
