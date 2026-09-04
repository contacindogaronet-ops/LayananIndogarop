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

const (
	CurrentVersion = "v1.0.0"
	GitHubRepo     = "username/repo" // Otomatis disesuaikan atau di-override via .env
)

type GitHubRelease struct {
	TagName string `json:"tag_name"`
	Assets  []struct {
		Name               string `json:"name"`
		BrowserDownloadURL string `json:"browser_download_url"`
	} `json:"assets"`
}

type AutoUpdater struct {
	logger  *logger.APILogger
	workDir string
	repo    string
}

func NewAutoUpdater(logger *logger.APILogger, workDir string) *AutoUpdater {
	repo := os.Getenv("GITHUB_REPOSITORY")
	if repo == "" {
		repo = GitHubRepo
	}
	return &AutoUpdater{
		logger:  logger,
		workDir: workDir,
		repo:    repo,
	}
}

func (u *AutoUpdater) StartWorker() {
	go func() {
		// Tunggu 30 detik setelah startup sebelum cek update pertama kali
		time.Sleep(30 * time.Second)
		for {
			u.checkAndUpdate()
			time.Sleep(30 * time.Minute) // Cek setiap 30 menit
		}
	}()
}

func (u *AutoUpdater) checkAndUpdate() {
	if u.repo == "username/repo" || u.repo == "" {
		return
	}

	url := fmt.Sprintf("https://api.github.com/repos/%s/releases/latest", u.repo)
	client := &http.Client{Timeout: 15 * time.Second}
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return
	}
	req.Header.Set("User-Agent", "Aiku-Daemon-Updater")

	resp, err := client.Do(req)
	if err != nil || resp.StatusCode != http.StatusOK {
		return
	}
	defer resp.Body.Close()

	var rel GitHubRelease
	if err := json.NewDecoder(resp.Body).Decode(&rel); err != nil {
		return
	}

	if rel.TagName != "" && rel.TagName != CurrentVersion {
		u.logger.Log("UPDATER", fmt.Sprintf("New version detected: %s (Current: %s). Initiating download...", rel.TagName, CurrentVersion))
		for _, asset := range rel.Assets {
			if strings.HasSuffix(asset.Name, ".apk") {
				u.downloadAndTriggerUpdate(asset.BrowserDownloadURL, asset.Name)
				break
			}
		}
	}
}

func (u *AutoUpdater) downloadAndTriggerUpdate(downloadURL, fileName string) {
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

	_, err = io.Copy(out, resp.Body)
	if err != nil {
		u.logger.Log("UPDATER", fmt.Sprintf("Write error: %v", err))
		return
	}

	u.logger.Log("UPDATER", fmt.Sprintf("Update package downloaded to %s. Ready for package install.", outPath))
}
