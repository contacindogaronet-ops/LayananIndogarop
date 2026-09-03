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
	// Android APK Foreground Service extraction target
	dataDir := os.Getenv("ANDROID_DATA_DIR")
	if dataDir != "" && exists(filepath.Join(dataDir, "config.yaml")) {
		_ = os.Chdir(dataDir)
		return dataDir
	}

	exePath, err := os.Executable()
	if err == nil {
		exeDir := filepath.Dir(exePath)
		if exists(filepath.Join(exeDir, "config.yaml")) {
			_ = os.Chdir(exeDir)
			return exeDir
		}
	}

	pwd, _ := os.Getwd()
	return pwd
}

func exists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

func main() {
	rootDir := resolveWorkDir()
	configPath := filepath.Join(rootDir, "config.yaml")
	binDir := filepath.Join(rootDir, "bin")

	// Ensure runtime directories
	_ = os.MkdirAll(binDir, 0755)

	cfg, err := config.LoadConfig(configPath)
	if err != nil {
		cfg = &config.Config{}
		cfg.Server.Host = "127.0.0.1"
		cfg.Server.Port = 8080
	}

	// Auto-Kill port conflict on launch
	_ = executor.KillPortHolders(cfg.Server.Port)

	apiLogger := logger.NewAPILogger(500)
	apiLogger.Log("CORE", fmt.Sprintf("Android Pure Service Daemon Initialized. Root: %s", rootDir))

	sup := supervisor.NewSupervisor(cfg, apiLogger, binDir)

	// Shell Execution Remote Command via HTTP API (Restricted to localhost)
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
		apiLogger.Log("SYSTEM", fmt.Sprintf("Native HTTP Service listening on http://%s", serverAddr))
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			apiLogger.Log("FATAL", fmt.Sprintf("Port conflict or crash: %v. Re-killing port and retrying...", err))
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
	apiLogger.Log("SYSTEM", "Android Foreground Service Terminating...")

	cancel()
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer shutdownCancel()
	_ = server.Shutdown(shutdownCtx)

	wg.Wait()
}
