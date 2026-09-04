package updater

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"
)

var (
	// CurrentVersion diinjeksi saat build via ldflags (-X ...CurrentVersion=v1.0.0)
	CurrentVersion = "v1.0.0"
	// CheckInterval interval pengecekan update otomatis
	CheckInterval = 30 * time.Minute
)

type GitHubRelease struct {
	TagName string `json:"tag_name"`
	Name    string `json:"name"`
	Assets  []struct {
		Name               string `json:"name"`
		BrowserDownloadURL string `json:"browser_download_url"`
	} `json:"assets"`
}

// StartAutoUpdater berjalan di latar belakang untuk memantau GitHub Release
func StartAutoUpdater(dataDir, repoOwner, repoName string) {
	if repoOwner == "" || repoName == "" {
		repoOwner = "indogaro"
		repoName = "service"
	}

	ticker := time.NewTicker(CheckInterval)
	go func() {
		// Pengecekan awal saat daemon baru menyala (setelah delay 30 detik agar network stabil)
		time.Sleep(30 * time.Second)
		checkForUpdate(dataDir, repoOwner, repoName)

		for range ticker.C {
			checkForUpdate(dataDir, repoOwner, repoName)
		}
	}()
}

func checkForUpdate(dataDir, repoOwner, repoName string) {
	apiURL := fmt.Sprintf("https://api.github.com/repos/%s/%s/releases/latest", repoOwner, repoName)
	client := &http.Client{Timeout: 20 * time.Second}

	req, err := http.NewRequest("GET", apiURL, nil)
	if err != nil {
		return
	}
	req.Header.Set("User-Agent", "Indogaro-Daemon-Updater")

	resp, err := client.Do(req)
	if err != nil {
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return
	}

	var release GitHubRelease
	if err := json.NewDecoder(resp.Body).Decode(&release); err != nil {
		return
	}

	latestTag := strings.TrimSpace(release.TagName)
	if isNewerVersion(CurrentVersion, latestTag) {
		log.Printf("[UPDATER] Versi baru terdeteksi: %s (Versi aktif: %s)", latestTag, CurrentVersion)

		var downloadURL string
		for _, asset := range release.Assets {
			if strings.HasSuffix(asset.Name, ".apk") {
				downloadURL = asset.BrowserDownloadURL
				break
			}
		}

		if downloadURL != "" {
			downloadAndTriggerUpdate(dataDir, downloadURL)
		}
	}
}

func isNewerVersion(current, latest string) bool {
	cleanCur := strings.TrimPrefix(strings.TrimSpace(current), "v")
	cleanLat := strings.TrimPrefix(strings.TrimSpace(latest), "v")

	if cleanCur == "" || cleanLat == "" {
		return false
	}
	return cleanCur != cleanLat
}

func downloadAndTriggerUpdate(dataDir, downloadURL string) {
	apkPath := filepath.Join(dataDir, "update.apk")
	sigPath := filepath.Join(dataDir, "trigger_update.sig")

	// 1. Download file APK
	client := &http.Client{Timeout: 120 * time.Second}
	resp, err := client.Get(downloadURL)
	if err != nil {
		log.Printf("[UPDATER] Gagal mengunduh APK: %v", err)
		return
	}
	defer resp.Body.Close()

	out, err := os.Create(apkPath)
	if err != nil {
		log.Printf("[UPDATER] Gagal membuat file target APK: %v", err)
		return
	}
	defer out.Close()

	_, err = io.Copy(out, resp.Body)
	if err != nil {
		log.Printf("[UPDATER] Gagal menulis stream APK: %v", err)
		return
	}

	// 2. Beri izin read file APK
	_ = os.Chmod(apkPath, 0644)

	// 3. Tulis trigger file agar Java Foreground Service mengeksekusi install intent
	if err := os.WriteFile(sigPath, []byte(time.Now().String()), 0644); err != nil {
		log.Printf("[UPDATER] Gagal menulis trigger signal: %v", err)
		return
	}

	log.Printf("[UPDATER] Sinyal update berhasil dikirim ke IndogaroForegroundService")
}
