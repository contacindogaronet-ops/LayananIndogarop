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

	binDir := os.Getenv("BIN_DIR")
	if binDir == "" {
		binDir = filepath.Join(rootDir, "bin")
	}
	_ = os.MkdirAll(binDir, 0755)

	cfg, err := config.LoadConfig(configPath)
	if err != nil || cfg == nil {
		cfg = &config.Config{}
	}
	// Pastikan mengikat ke 0.0.0.0 agar bisa diakses dari v2rayNG / Browser
	cfg.Server.Host = "0.0.0.0"
	if cfg.Server.Port == 0 {
		cfg.Server.Port = 8080
	}

	// Auto clean conflicting port
	_ = executor.KillPortHolders(cfg.Server.Port)

	apiLogger := logger.NewAPILogger(500)
	apiLogger.Log("CORE", fmt.Sprintf("Aiku Engine started. RootDir=%s, BinDir=%s", rootDir, binDir))

	sup := supervisor.NewSupervisor(cfg, apiLogger, binDir)

	// HTTP Status & Process Inspector
	http.HandleFunc("/api/v1/status", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"daemon":    "UP",
			"timestamp": time.Now(),
			"port":      cfg.Server.Port,
			"root_dir":  rootDir,
			"bin_dir":   binDir,
			"processes": sup.GetStatuses(),
		})
	})

	// Shell Exec Remote API
	http.HandleFunc("/api/v1/exec", func(w http.ResponseWriter, r *http.Request) {
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

	http.HandleFunc("/api/v1/logs", apiLogger.ServeHTTPLogs)

	serverAddr := fmt.Sprintf("%s:%d", cfg.Server.Host, cfg.Server.Port)
	server := &http.Server{Addr: serverAddr}

	go func() {
		apiLogger.Log("SYSTEM", fmt.Sprintf("Telemetry & API server live on http://%s", serverAddr))
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			apiLogger.Log("FATAL", fmt.Sprintf("Server collision (%v), retrying port kill...", err))
			_ = executor.KillPortHolders(cfg.Server.Port)
			_ = server.ListenAndServe()
		}
	}()

	ctx, cancel := context.WithCancel(context.Background())
	var wg sync.WaitGroup

	sup.StartAll(ctx, &wg)

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)

	<-sigChan
	apiLogger.Log("SYSTEM", "Aiku Service shutting down...")

	cancel()
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer shutdownCancel()
	_ = server.Shutdown(shutdownCtx)

	wg.Wait()
}
