package updater

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"aiku-daemon/internal/logger"
)

// Version otomatis diinjeksi saat build via -ldflags="-X aiku-daemon/internal/updater.CurrentVersion=..."
var CurrentVersion = "v1.0.0"

type GitHubRelease struct {
	TagName string `json:"tag_name"`
	Assets  []struct {
		Name               string `json:"name"`
		BrowserDownloadURL string `json:"browser_download_url"`
		Size               int64  `json:"size"`
	} `json:"assets"`
}

type AutoUpdater struct {
	logger  *logger.APILogger
	workDir string
	repo    string
}

func NewAutoUpdater(logger *logger.APILogger, workDir string) *AutoUpdater {
	repo := os.Getenv("GITHUB_REPOSITORY")
	return &AutoUpdater{
		logger:  logger,
		workDir: workDir,
		repo:    repo,
	}
}

func (u *AutoUpdater) StartWorker() {
	go func() {
		time.Sleep(30 * time.Second)
		for {
			u.checkAndUpdate()
			time.Sleep(30 * time.Minute)
		}
	}()
}

func (u *AutoUpdater) checkAndUpdate() {
	if u.repo == "" {
		return
	}

	url := fmt.Sprintf("https://api.github.com/repos/%s/releases/latest", u.repo)
	client := &http.Client{Timeout: 15 * time.Second}
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return
	}
	req.Header.Set("User-Agent", "Aiku-Daemon-Engine")

	resp, err := client.Do(req)
	if err != nil || resp.StatusCode != http.StatusOK {
		return
	}
	defer resp.Body.Close()

	var rel GitHubRelease
	if err := json.NewDecoder(resp.Body).Decode(&rel); err != nil {
		return
	}

	latestTag := strings.TrimPrefix(rel.TagName, "v")
	currentTag := strings.TrimPrefix(CurrentVersion, "v")

	if latestTag != "" && latestTag != currentTag {
		u.logger.Log("UPDATER", fmt.Sprintf("New release available: v%s (Current: v%s)", latestTag, currentTag))
		for _, asset := range rel.Assets {
			if strings.HasSuffix(asset.Name, ".apk") {
				u.downloadAPK(asset.BrowserDownloadURL, asset.Size)
				break
			}
		}
	}
}

func (u *AutoUpdater) downloadAPK(downloadURL string, expectedSize int64) {
	outPath := filepath.Join(u.workDir, "update.apk")
	resp, err := http.Get(downloadURL)
	if err != nil {
		u.logger.Log("UPDATER", fmt.Sprintf("Download failed: %v", err))
		return
	}
	defer resp.Body.Close()

	out, err := os.Create(outPath)
	if err != nil {
		return
	}
	defer out.Close()

	written, err := io.Copy(out, resp.Body)
	if err != nil || (expectedSize > 0 && written != expectedSize) {
		u.logger.Log("UPDATER", "Update payload incomplete/corrupted. Discarding.")
		_ = os.Remove(outPath)
		return
	}

	u.logger.Log("UPDATER", fmt.Sprintf("Update APK verified & saved to %s", outPath))
	// Tulis trigger update signal untuk dibaca Java Service
	_ = os.WriteFile(filepath.Join(u.workDir, "trigger_update.sig"), []byte("READY"), 0644)
}
