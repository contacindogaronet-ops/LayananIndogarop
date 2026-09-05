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

	"aiku-daemon/internal/supervisor"
)

var (
	CurrentVersion = "v1.2.0"
	CheckInterval  = 15 * time.Minute
)

type GitHubRelease struct {
	TagName string `json:"tag_name"`
	Name    string `json:"name"`
	Assets  []struct {
		Name               string `json:"name"`
		Size               int64  `json:"size"`
		BrowserDownloadURL string `json:"browser_download_url"`
	} `json:"assets"`
}

// StartInPlaceHotUpdater memantau GitHub Release dan mengunduh APK terbaru secara otomatis
func StartInPlaceHotUpdater(dataDir, repoOwner, repoName string, sup *supervisor.Supervisor) {
	ticker := time.NewTicker(CheckInterval)
	go func() {
		// Delay awal setelah booting sebelum cek pembaruan
		time.Sleep(20 * time.Second)
		checkAndDownloadApkUpdate(dataDir, repoOwner, repoName)

		for range ticker.C {
			checkAndDownloadApkUpdate(dataDir, repoOwner, repoName)
		}
	}()
}

func checkAndDownloadApkUpdate(dataDir, repoOwner, repoName string) {
	apiURL := fmt.Sprintf("https://api.github.com/repos/%s/%s/releases/latest", repoOwner, repoName)
	client := &http.Client{Timeout: 15 * time.Second}

	req, err := http.NewRequest("GET", apiURL, nil)
	if err != nil {
		return
	}
	req.Header.Set("User-Agent", "Indogaro-Apk-AutoUpdater")

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
		log.Printf("[UPDATER] Versi APK baru terdeteksi di GitHub: %s (Versi aktif: %s)", latestTag, CurrentVersion)

		var apkURL string
		var expectedSize int64
		for _, asset := range release.Assets {
			if strings.HasSuffix(asset.Name, ".apk") {
				apkURL = asset.BrowserDownloadURL
				expectedSize = asset.Size
				break
			}
		}

		if apkURL != "" {
			downloadApkAndSignalAndroid(dataDir, apkURL, expectedSize)
		}
	}
}

func isNewerVersion(current, latest string) bool {
	cleanCur := strings.TrimPrefix(strings.TrimSpace(current), "v")
	cleanLat := strings.TrimPrefix(strings.TrimSpace(latest), "v")
	return cleanCur != "" && cleanLat != "" && cleanCur != cleanLat
}

func downloadApkAndSignalAndroid(dataDir, apkURL string, expectedSize int64) {
	tmpApk := filepath.Join(dataDir, "update.apk.tmp")
	finalApk := filepath.Join(dataDir, "update.apk")
	sigFile := filepath.Join(dataDir, "trigger_update.sig")

	log.Printf("[UPDATER] Mengunduh paket APK pembaruan otomatis dari %s...", apkURL)

	client := &http.Client{Timeout: 300 * time.Second}
	resp, err := client.Get(apkURL)
	if err != nil {
		log.Printf("[UPDATER] Gagal mengunduh APK: %v", err)
		return
	}
	defer resp.Body.Close()

	out, err := os.Create(tmpApk)
	if err != nil {
		log.Printf("[UPDATER] Gagal membuat file temp: %v", err)
		return
	}

	written, err := io.Copy(out, resp.Body)
	_ = out.Close()

	if err != nil {
		log.Printf("[UPDATER] Gagal menyimpan stream APK: %v", err)
		_ = os.Remove(tmpApk)
		return
	}

	if expectedSize > 0 && written < (expectedSize-1024) {
		log.Printf("[UPDATER] Ukuran APK tidak cocok (%d vs %d), membatalkan...", written, expectedSize)
		_ = os.Remove(tmpApk)
		return
	}

	_ = os.Chmod(tmpApk, 0644)
	_ = os.Rename(tmpApk, finalApk)

	// Tulis sinyal trigger agar Foreground Service Java langsung mengeksekusi instalasi
	timestamp := fmt.Sprintf("%d", time.Now().Unix())
	if err := os.WriteFile(sigFile, []byte(timestamp), 0644); err != nil {
		log.Printf("[UPDATER] Gagal menulis trigger_update.sig: %v", err)
		return
	}

	log.Printf("[UPDATER] APK berhasil diunduh (%d bytes). Sinyal update dikirim ke Android Service!", written)
}
