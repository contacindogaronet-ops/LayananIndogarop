package updater

import (
	"archive/tar"
	"archive/zip"
	"bytes"
	"compress/gzip"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"syscall"
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

// StartInPlaceHotUpdater mengunduh dan mengganti biner tanpa intervensi user/tanpa install APK
func StartInPlaceHotUpdater(dataDir, repoOwner, repoName string, sup *supervisor.Supervisor) {
	ticker := time.NewTicker(CheckInterval)
	go func() {
		time.Sleep(15 * time.Second)
		checkAndPerformInPlaceUpdate(dataDir, repoOwner, repoName, sup)

		for range ticker.C {
			checkAndPerformInPlaceUpdate(dataDir, repoOwner, repoName, sup)
		}
	}()
}

func checkAndPerformInPlaceUpdate(dataDir, repoOwner, repoName string, sup *supervisor.Supervisor) {
	apiURL := fmt.Sprintf("https://api.github.com/repos/%s/%s/releases/latest", repoOwner, repoName)
	client := &http.Client{Timeout: 15 * time.Second}

	req, err := http.NewRequest("GET", apiURL, nil)
	if err != nil {
		return
	}
	req.Header.Set("User-Agent", "Indogaro-HotInPlace-Engine")

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
		log.Printf("[HOT-UPDATER] Versi biner baru tersedia: %s -> Memulai update in-place tanpa install APK...", latestTag)

		// 1. Cari file binary langsung atau APK bundle untuk diekstrak asetnya
		for _, asset := range release.Assets {
			// Prioritas A: Update biner aiku-daemon secara langsung jika disediakan
			if asset.Name == "aiku-daemon" || asset.Name == "aiku-daemon-arm64" {
				if downloadAndReplaceBinary(filepath.Join(dataDir, "aiku-daemon"), asset.BrowserDownloadURL) {
					log.Printf("[HOT-UPDATER] Biner aiku-daemon berhasil di-swap in-place! Restarting daemon runtime...")
					restartCurrentProcess()
					return
				}
			}

			// Prioritas B: Ekstrak biner internal langsung dari release APK tanpa buka UI installer
			if strings.HasSuffix(asset.Name, ".apk") {
				if extractAndSwapFromApk(dataDir, asset.BrowserDownloadURL, sup) {
					log.Printf("[HOT-UPDATER] Biner diekstrak & di-swap sukses dari APK bundle. Hot-reload aktif!")
					return
				}
			}
		}
	}
}

func isNewerVersion(current, latest string) bool {
	cleanCur := strings.TrimPrefix(strings.TrimSpace(current), "v")
	cleanLat := strings.TrimPrefix(strings.TrimSpace(latest), "v")
	return cleanCur != "" && cleanLat != "" && cleanCur != cleanLat
}

func downloadAndReplaceBinary(targetPath, downloadURL string) bool {
	tmpPath := targetPath + ".new"
	client := &http.Client{Timeout: 60 * time.Second}

	resp, err := client.Get(downloadURL)
	if err != nil {
		return false
	}
	defer resp.Body.Close()

	out, err := os.OpenFile(tmpPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0755)
	if err != nil {
		return false
	}

	_, err = io.Copy(out, resp.Body)
	_ = out.Close()

	if err != nil {
		_ = os.Remove(tmpPath)
		return false
	}

	_ = os.Chmod(tmpPath, 0755)
	_ = os.Rename(tmpPath, targetPath)
	return true
}

func extractAndSwapFromApk(dataDir, apkURL string, sup *supervisor.Supervisor) bool {
	client := &http.Client{Timeout: 120 * time.Second}
	resp, err := client.Get(apkURL)
	if err != nil {
		return false
	}
	defer resp.Body.Close()

	apkBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return false
	}

	zipReader, err := zip.NewReader(bytes.NewReader(apkBytes), int64(len(apkBytes)))
	if err != nil {
		return false
	}

	var updatedCore, updatedCoba bool

	for _, file := range zipReader.File {
		if file.Name == "assets/coba" || file.Name == "assets/aiku-daemon" {
			rc, err := file.Open()
			if err != nil {
				continue
			}

			targetName := filepath.Base(file.Name)
			targetPath := filepath.Join(dataDir, targetName)
			tmpPath := targetPath + ".hot"

			outFile, err := os.OpenFile(tmpPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0755)
			if err != nil {
				rc.Close()
				continue
			}

			_, _ = io.Copy(outFile, rc)
			outFile.Close()
			rc.Close()

			_ = os.Chmod(tmpPath, 0755)
			_ = os.Rename(tmpPath, targetPath)

			if targetName == "coba" {
				updatedCoba = true
			}
			if targetName == "aiku-daemon" {
				updatedCore = true
			}
		}
	}

	// Jika sub-biner coba terupdate, restart proses coba via supervisor
	if updatedCoba && sup != nil {
		log.Println("[HOT-UPDATER] Reloading 'coba' packet engine...")
		sup.RestartSubProcess()
	}

	// Jika main supervisor daemon terupdate, hot-exec diri sendiri
	if updatedCore {
		restartCurrentProcess()
	}

	return updatedCore || updatedCoba
}

func restartCurrentProcess() {
	execPath, err := os.Executable()
	if err != nil {
		return
	}
	// Hot exec mengganti proses memori secara instan
	_ = syscall.Exec(execPath, os.Args, os.Environ())
}
