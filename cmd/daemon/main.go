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

func setMaxFDLimit() {
	var rLimit syscall.Rlimit
	rLimit.Max = 65535
	rLimit.Cur = 65535
	_ = syscall.Setrlimit(syscall.RLIMIT_NOFILE, &rLimit)
}

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
	setMaxFDLimit()

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

	apiLogger := logger.NewAPILogger(500)
	apiLogger.Log("CORE", fmt.Sprintf("Aiku pure daemon initialized at %s", rootDir))

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

	// Root Ping
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, _ = fmt.Fprintf(w, "<h3>✓ Aiku Daemon Active</h3><p>Multiplexer: 127.0.0.3:2007 | Telemetry: 127.0.0.3:2008</p>")
	})

	serverAddr := fmt.Sprintf("%s:%d", cfg.Server.Host, cfg.Server.Port)
	server := &http.Server{Addr: serverAddr}

	go func() {
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			apiLogger.Log("FATAL", fmt.Sprintf("Server failed on %s: %v", serverAddr, err))
		}
	}()

	ctx, cancel := context.WithCancel(context.Background())
	var wg sync.WaitGroup

	// Jalankan binary 'coba'
	sup.StartAll(ctx, &wg)

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)

	<-sigChan
	apiLogger.Log("SYSTEM", "Aiku Core stopping...")

	cancel()
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer shutdownCancel()
	_ = server.Shutdown(shutdownCtx)

	wg.Wait()
}
