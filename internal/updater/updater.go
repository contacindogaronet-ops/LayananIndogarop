package updater

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os/exec"
	"strings"
	"time"

	"github.com/rs/zerolog/log"
)

const CurrentVersion = "v1.0.0"

type GitHubRelease struct {
	TagName string `json:"tag_name"`
	HTMLURL string `json:"html_url"`
}

// getRepoTarget secara dinamis membaca owner/repo dari remote git lokal
func getRepoTarget() string {
	cmd := exec.Command("git", "config", "--get", "remote.origin.url")
	out, err := cmd.Output()
	if err != nil {
		return ""
	}
	urlStr := strings.TrimSpace(string(out))
	urlStr = strings.TrimSuffix(urlStr, ".git")
	
	// Mendukung format HTTPS atau SSH (git@github.com:owner/repo)
	if strings.Contains(urlStr, "github.com/") {
		parts := strings.Split(urlStr, "github.com/")
		if len(parts) == 2 {
			return parts[1]
		}
	} else if strings.Contains(urlStr, "github.com:") {
		parts := strings.Split(urlStr, "github.com:")
		if len(parts) == 2 {
			return parts[1]
		}
	}
	return ""
}

// CheckForUpdate mengecek pembaruan dari GitHub Releases API berdasarkan remote git aktif
func CheckForUpdate() (hasUpdate bool, latestVersion string, releaseURL string, err error) {
	repoTarget := getRepoTarget()
	if repoTarget == "" {
		return false, "", "", fmt.Errorf("remote git origin bukan GitHub atau belum diset")
	}

	client := &http.Client{Timeout: 4 * time.Second}
	url := fmt.Sprintf("https://api.github.com/repos/%s/releases/latest", repoTarget)

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return false, "", "", err
	}
	req.Header.Set("User-Agent", "aicli-daemon-termux")

	resp, err := client.Do(req)
	if err != nil {
		return false, "", "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return false, "", "", fmt.Errorf("github api status: %d", resp.StatusCode)
	}

	var release GitHubRelease
	if err := json.NewDecoder(resp.Body).Decode(&release); err != nil {
		return false, "", "", err
	}

	latest := strings.TrimSpace(release.TagName)
	if latest != "" && latest != CurrentVersion {
		return true, latest, release.HTMLURL, nil
	}

	return false, latest, release.HTMLURL, nil
}

// PerformSelfUpdate melakukan pull pembaruan git terbaru dan compile ulang binary secara otonom
func PerformSelfUpdate() error {
	log.Info().Msg("Memulai proses self-update repository...")
	
	cmdPull := exec.Command("git", "pull")
	if err := cmdPull.Run(); err != nil {
		return fmt.Errorf("git pull gagal: %w", err)
	}

	cmdBuild := exec.Command("go", "build", "-o", "aicli", "main.go")
	if err := cmdBuild.Run(); err != nil {
		return fmt.Errorf("go build ulang gagal: %w", err)
	}

	log.Info().Msg("Self-update berhasil dieksekusi")
	return nil
}
