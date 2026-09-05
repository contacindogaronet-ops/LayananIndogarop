package updater

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"aiku-daemon/internal/supervisor"
)

var (
	// CurrentVersion diinjeksi saat build melalui ldflags (-X ...CurrentVersion=1.0.760)
	CurrentVersion = "1.0.760"
	CheckInterval  = 10 * time.Minute
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

// StartInPlaceHotUpdater memantau rilis baru dengan parsing versi monotonic
func StartInPlaceHotUpdater(dataDir, repoOwner, repoName string, sup *supervisor.Supervisor) {
	ticker := time.NewTicker(CheckInterval)
	go func() {
		// Delay 15 detik setelah daemon booting sebelum melakukan pengecekan pertama
		time.Sleep(15 * time.Second)
		checkAndDownloadApkUpdate(dataDir, repoOwner, repoName)

		for range ticker.C {
			checkAndDownloadApkUpdate(dataDir, repoOwner, repoName)
		}
	}()
}

func checkAndDownloadApkUpdate(dataDir, repoOwner, repoName string) {
	apiURL := fmt.Sprintf("https://api.github.com/repos/%s/%s/releases/latest", repoOwner, repoName)
	client := &http.Client{Timeout: 20 * time.Second}

	req, err := http.NewRequest("GET", apiURL, nil)
	if err != nil {
		return
	}
	req.Header.Set("User-Agent", "Indogaro-Monotonic-AutoUpdater")

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

	remoteTag := strings.TrimSpace(release.TagName)
	if IsNewerMonotonicVersion(CurrentVersion, remoteTag) {
		log.Printf("[UPDATER] Versi baru terdeteksi: %s (Versi aktif: %s) -> Memulai auto-download APK...", remoteTag, CurrentVersion)

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
	} else {
		log.Printf("[UPDATER] Sistem berada pada versi terbaru (%s). Tidak ada update.", CurrentVersion)
	}
}

// IsNewerMonotonicVersion membandingkan dua string versi (misal: 1.0.37 vs 1.0.760) secara numerik
func IsNewerMonotonicVersion(current, remote string) bool {
	curScore := parseVersionScore(current)
	remScore := parseVersionScore(remote)

	if remScore == 0 || curScore == 0 {
		cleanCur := strings.TrimPrefix(strings.TrimSpace(current), "v")
		cleanRem := strings.TrimPrefix(strings.TrimSpace(remote), "v")
		return cleanCur != cleanRem && cleanRem != ""
	}

	return remScore > curScore
}

func parseVersionScore(ver string) int64 {
	clean := strings.TrimPrefix(strings.TrimSpace(ver), "v")
	parts := strings.Split(clean, ".")
	if len(parts) == 0 {
		return 0
	}

	var major, minor, patch int64

	if len(parts) >= 1 {
		major, _ = strconv.ParseInt(parts[0], 10, 64)
	}
	if len(parts) >= 2 {
		minor, _ = strconv.ParseInt(parts[1], 10, 64)
	}
	if len(parts) >= 3 {
		numOnly := strings.Split(parts[2], "-")[0]
		patch, _ = strconv.ParseInt(numOnly, 10, 64)
	}

	return (major * 100000000) + (minor * 10000) + patch
}

func downloadApkAndSignalAndroid(dataDir, apkURL string, expectedSize int64) {
	tmpApk := filepath.Join(dataDir, "update.apk.tmp")
	finalApk := filepath.Join(dataDir, "update.apk")
	sigFile := filepath.Join(dataDir, "trigger_update.sig")

	log.Printf("[UPDATER] Mengunduh APK dari: %s", apkURL)

	client := &http.Client{Timeout: 300 * time.Second}
	resp, err := client.Get(apkURL)
	if err != nil {
		log.Printf("[UPDATER] Gagal mengunduh: %v", err)
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
		log.Printf("[UPDATER] Gagal transfer stream: %v", err)
		_ = os.Remove(tmpApk)
		return
	}

	if expectedSize > 0 && written < (expectedSize-2048) {
		log.Printf("[UPDATER] Ukuran korup (%d vs %d), batalkan.", written, expectedSize)
		_ = os.Remove(tmpApk)
		return
	}

	_ = os.Chmod(tmpApk, 0644)
	_ = os.Rename(tmpApk, finalApk)

	// Kirim sinyal trigger ke Java layer
	timestamp := fmt.Sprintf("%d", time.Now().Unix())
	_ = os.WriteFile(sigFile, []byte(timestamp), 0644)

	log.Printf("[UPDATER] APK berhasil diunduh (%d bytes). Trigger update dikirim ke Android!", written)
}
