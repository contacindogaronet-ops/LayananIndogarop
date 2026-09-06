package updater

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"syscall"
	"time"

	"aiku-daemon/internal/config"
	"aiku-daemon/internal/logger"
)

// ReleaseAsset merepresentasikan informasi binary/asset release GitHub
type ReleaseAsset struct {
	Name               string `json:"name"`
	BrowserDownloadURL string `json:"browser_download_url"`
}

// GitHubRelease merepresentasikan payload JSON release dari GitHub API
type GitHubRelease struct {
	TagName string         `json:"tag_name"`
	Assets  []ReleaseAsset `json:"assets"`
}

// StartInPlaceHotUpdater menjalankan background updater loop untuk auto hot-patch
func StartInPlaceHotUpdater(ctx context.Context, cfg *config.Config, currentVersion string) {
	log := logger.GetLogger()
	ticker := time.NewTicker(30 * time.Minute)
	defer ticker.Stop()

	log.Info().Str("version", currentVersion).Msg("Autonomous In-Place Hot Updater aktif")

	for {
		select {
		case <-ctx.Done():
			log.Info().Msg("Autonomous In-Place Hot Updater dihentikan")
			return
		case <-ticker.C:
			log.Debug().Msg("Memeriksa rilis update terbaru...")
			if err := CheckAndApplyUpdate(cfg, currentVersion); err != nil {
				log.Warn().Err(err).Msg("Pemeriksaan update gagal")
			}
		}
	}
}

// CheckAndApplyUpdate memeriksa dan mengunduh binary terbaru jika tersedia versi baru
func CheckAndApplyUpdate(cfg *config.Config, currentVersion string) error {
	log := logger.GetLogger()

	repo := "indogaro/aiku-daemon"
	if cfg != nil && cfg.UpdateRepo != "" {
		repo = cfg.UpdateRepo
	}

	apiURL := fmt.Sprintf("https://api.github.com/repos/%s/releases/latest", repo)
	req, err := http.NewRequest("GET", apiURL, nil)
	if err != nil {
		return err
	}
	req.Header.Set("User-Agent", "aiku-daemon-updater")

	client := &http.Client{Timeout: 15 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("github api status: %s", resp.Status)
	}

	var rel GitHubRelease
	if err := json.NewDecoder(resp.Body).Decode(&rel); err != nil {
		return err
	}

	if rel.TagName == "" || rel.TagName == currentVersion {
		log.Debug().Str("current", currentVersion).Msg("Versi sudah yang paling mutakhir")
		return nil
	}

	log.Info().Str("current", currentVersion).Str("new", rel.TagName).Msg("Versi baru ditemukan, mempersiapkan in-place update...")

	expectedAssetName := fmt.Sprintf("aiku-daemon-%s-%s", runtime.GOOS, runtime.GOARCH)
	var downloadURL string
	for _, a := range rel.Assets {
		if a.Name == expectedAssetName || a.Name == "aiku-daemon" {
			downloadURL = a.BrowserDownloadURL
			break
		}
	}

	if downloadURL == "" {
		return fmt.Errorf("asset untuk arsitektur %s-%s tidak ditemukan pada rilis %s", runtime.GOOS, runtime.GOARCH, rel.TagName)
	}

	execPath, err := os.Executable()
	if err != nil {
		return fmt.Errorf("gagal mendapatkan path executable: %w", err)
	}

	tempPath := execPath + ".tmp"
	if err := downloadFile(downloadURL, tempPath); err != nil {
		_ = os.Remove(tempPath)
		return fmt.Errorf("gagal mengunduh binary: %w", err)
	}

	if err := os.Chmod(tempPath, 0755); err != nil {
		_ = os.Remove(tempPath)
		return fmt.Errorf("gagal chmod binary baru: %w", err)
	}

	// In-place atomic binary replace
	if err := os.Rename(tempPath, execPath); err != nil {
		_ = os.Remove(tempPath)
		return fmt.Errorf("gagal mengganti binary lama: %w", err)
	}

	log.Info().Str("version", rel.TagName).Msg("Binary berhasil diperbarui. Me-restart daemon...")

	// Restart in-place process via syscall exec
	if err := syscall.Exec(execPath, os.Args, os.Environ()); err != nil {
		// Fallback jika direct exec tidak didukung
		cmd := exec.Command(execPath, os.Args[1:]...)
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		if err := cmd.Start(); err != nil {
			return fmt.Errorf("gagal memulai instance baru: %w", err)
		}
		os.Exit(0)
	}

	return nil
}

func downloadFile(url, destPath string) error {
	out, err := os.Create(destPath)
	if err != nil {
		return err
	}
	defer out.Close()

	resp, err := http.Get(url)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("status response gagal: %s", resp.Status)
	}

	_, err = io.Copy(out, resp.Body)
	return err
}