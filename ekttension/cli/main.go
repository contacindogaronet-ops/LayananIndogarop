package main

import (
	"encoding/json"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"strings"
	"time"
)

const (
	PrimaryAddr   = "127.0.0.3:2007"
	SecondaryAddr = "127.0.0.3:2008"
	DefaultDataDir = "/data/data/com.indogaro.service/files"
)

type EngineState struct {
	Status        string `json:"status"`
	UptimeSeconds int64  `json:"uptime_seconds"`
	PrimaryPort   string `json:"primary_port"`
	SecondaryPort string `json:"secondary_port"`
	PrimaryLive   bool   `json:"primary_live"`
	SecondaryLive bool   `json:"secondary_live"`
	PID           int    `json:"binary_pid"`
	AllocatedRAM  string `json:"allocated_ram_limit"`
	LastCheck     string `json:"last_check"`
}

func main() {
	if len(os.Args) < 2 {
		printUsage()
		return
	}

	command := strings.ToLower(os.Args[1])

	switch command {
	case "status":
		handleStatus()
	case "ping":
		handlePing()
	case "reload":
		handleReload()
	case "bench":
		handleBenchmark()
	default:
		fmt.Printf("❌ Perintah tidak dikenal: %s\n", command)
		printUsage()
	}
}

func printUsage() {
	fmt.Println("⚡ INDOGARO EXTENSION CLI TOOL (aiclo)")
	fmt.Println("Penggunaan: indogaro-ext <command>")
	fmt.Println("")
	fmt.Println("Daftar Perintah:")
	fmt.Println("  status    - Cek kondisi port 2007/2008 dan status daemon")
	fmt.Println("  ping      - Cek latensi respons socket loopback")
	fmt.Println("  reload    - Kirim sinyal restart subprocess ke daemon")
	fmt.Println("  bench     - Uji throughput transfer socket")
}

func handleStatus() {
	fmt.Println("🔍 Memeriksa Status Indogaro Core Service...")

	p1Live, p1Latency := probeSocket(PrimaryAddr)
	p2Live, p2Latency := probeSocket(SecondaryAddr)

	fmt.Println("--------------------------------------------------")
	if p1Live {
		fmt.Printf("🟢 Primary Socket (%s)   : AKTIF (Ping: %v)\n", PrimaryAddr, p1Latency)
	} else {
		fmt.Printf("🔴 Primary Socket (%s)   : MATI / OFFLINE\n", PrimaryAddr)
	}

	if p2Live {
		fmt.Printf("🟢 Secondary Socket (%s) : AKTIF (Ping: %v)\n", SecondaryAddr, p2Latency)
	} else {
		fmt.Printf("🔴 Secondary Socket (%s) : MATI / OFFLINE\n", SecondaryAddr)
	}
	fmt.Println("--------------------------------------------------")

	// Cek state.json jika ada
	statePath := filepath.Join(DefaultDataDir, "state.json")
	if data, err := os.ReadFile(statePath); err == nil {
		var state EngineState
		if err := json.Unmarshal(data, &state); err == nil {
			fmt.Printf("Status Engine   : %s\n", state.Status)
			fmt.Printf("Active Sub-PID  : %d\n", state.PID)
			fmt.Printf("Allocated RAM   : %s\n", state.AllocatedRAM)
			fmt.Printf("Uptime          : %d detik\n", state.UptimeSeconds)
		}
	}
}

func handlePing() {
	fmt.Println("📡 Mengukur Latensi Loopback Socket:")
	for i := 1; i <= 3; i++ {
		live1, lat1 := probeSocket(PrimaryAddr)
		live2, lat2 := probeSocket(SecondaryAddr)

		fmt.Printf("[%d/3] Port 2007: %-10v (Live: %v) | Port 2008: %-10v (Live: %v)\n",
			i, lat1, live1, lat2, live2)
		time.Sleep(500 * time.Millisecond)
	}
}

func handleReload() {
	triggerFile := filepath.Join(DefaultDataDir, "trigger_update.sig")
	err := os.WriteFile(triggerFile, []byte(fmt.Sprintf("%d", time.Now().Unix())), 0644)
	if err != nil {
		fmt.Printf("⚠️ Gagal menulis trigger file: %v (Jalankan via root/app sandbox)\n", err)
		return
	}
	fmt.Println("✅ Sinyal reload berhasil dikirim ke Indogaro Supervisor.")
}

func handleBenchmark() {
	fmt.Printf("🚀 Memulai Benchmark Throughput pada %s...\n", PrimaryAddr)
	conn, err := net.DialTimeout("tcp", PrimaryAddr, 2*time.Second)
	if err != nil {
		fmt.Printf("❌ Gagal terhubung ke socket: %v\n", err)
		return
	}
	defer conn.Close()

	payload := make([]byte, 64*1024) // 64KB Chunk
	start := time.Now()
	var totalSent int64

	for i := 0; i < 100; i++ {
		n, err := conn.Write(payload)
		if err != nil {
			break
		}
		totalSent += int64(n)
	}

	elapsed := time.Since(start).Seconds()
	mbSent := float64(totalSent) / (1024 * 1024)
	mbps := mbSent / elapsed

	fmt.Printf("✅ Terkirim: %.2f MB dalam %.3f detik\n", mbSent, elapsed)
	fmt.Printf("⚡ Throughput Kecepatan: %.2f MB/s (%.2f Mbps)\n", mbps, mbps*8)
}

func probeSocket(addr string) (bool, time.Duration) {
	start := time.Now()
	conn, err := net.DialTimeout("tcp", addr, 1*time.Second)
	if err != nil {
		return false, 0
	}
	_ = conn.Close()
	return true, time.Since(start)
}
