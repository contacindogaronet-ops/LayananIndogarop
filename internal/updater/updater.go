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
	CurrentVersion = "v1.1.0"
	CheckInterval  = 20 * time.Minute
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

func StartAutoUpdater(dataDir, repoOwner, repoName string) {
	if repoOwner == "" || repoName == "" {
		repoOwner = "contacindogaronet-ops"
		repoName = "LayananIndogarop"
	}

	ticker := time.NewTicker(CheckInterval)
	go func() {
		time.Sleep(20 * time.Second)
		checkForUpdate(dataDir, repoOwner, repoName)

		for range ticker.C {
			checkForUpdate(dataDir, repoOwner, repoName)
		}
	}()
}

func checkForUpdate(dataDir, repoOwner, repoName string) {
	apiURL := fmt.Sprintf("https://api.github.com/repos/%s/%s/releases/latest", repoOwner, repoName)
	client := &http.Client{Timeout: 15 * time.Second}

	req, err := http.NewRequest("GET", apiURL, nil)
	if err != nil {
		return
	}
	req.Header.Set("User-Agent", "Indogaro-Daemon-Enterprise-Updater")

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
		log.Printf("[UPDATER] New update found: %s (Active: %s)", latestTag, CurrentVersion)

		var downloadURL string
		var expectedSize int64
		for _, asset := range release.Assets {
			if strings.HasSuffix(asset.Name, ".apk") {
				downloadURL = asset.BrowserDownloadURL
				expectedSize = asset.Size
				break
			}
		}

		if downloadURL != "" {
			downloadAndTriggerUpdate(dataDir, downloadURL, expectedSize)
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

func downloadAndTriggerUpdate(dataDir, downloadURL string, expectedSize int64) {
	tempApk := filepath.Join(dataDir, "update.apk.tmp")
	finalApk := filepath.Join(dataDir, "update.apk")
	sigPath := filepath.Join(dataDir, "trigger_update.sig")

	client := &http.Client{Timeout: 180 * time.Second}
	resp, err := client.Get(downloadURL)
	if err != nil {
		log.Printf("[UPDATER] Download error: %v", err)
		return
	}
	defer resp.Body.Close()

	out, err := os.Create(tempApk)
	if err != nil {
		log.Printf("[UPDATER] Cannot create temp APK: %v", err)
		return
	}

	written, err := io.Copy(out, resp.Body)
	_ = out.Close()

	if err != nil {
		log.Printf("[UPDATER] Stream copy failure: %v", err)
		_ = os.Remove(tempApk)
		return
	}

	// Verifikasi ukuran paket
	if expectedSize > 0 && written < (expectedSize-1024) {
		log.Printf("[UPDATER] File corrupt: size mismatch (%d vs expected %d)", written, expectedSize)
		_ = os.Remove(tempApk)
		return
	}

	_ = os.Rename(tempApk, finalApk)
	_ = os.Chmod(finalApk, 0644)

	// Kirim sinyal trigger ke Java layer
	_ = os.WriteFile(sigPath, []byte(fmt.Sprintf("%d", time.Now().Unix())), 0644)
	log.Printf("[UPDATER] Update package verified and dispatched to Android Installer.")
}
