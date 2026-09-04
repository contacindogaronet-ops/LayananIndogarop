package executor

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"
)

// KillPortHolders membersihkan proses yang menahan port tertentu di Android (/proc & shell fallback)
func KillPortHolders(port int) error {
	hexPort := fmt.Sprintf("%04X", port)
	myPid := os.Getpid()

	// 1. Scan /proc/net/tcp & /proc/net/tcp6 untuk menemukan inode pemegang port
	targetInodes := make(map[string]bool)
	for _, netFile := range []string{"/proc/net/tcp", "/proc/net/tcp6"} {
		content, err := os.ReadFile(netFile)
		if err != nil {
			continue
		}
		lines := strings.Split(string(content), "\n")
		for _, line := range lines {
			fields := strings.Fields(line)
			if len(fields) >= 10 {
				localAddr := fields[1]
				parts := strings.Split(localAddr, ":")
				if len(parts) == 2 && strings.EqualFold(parts[1], hexPort) {
					inode := fields[9]
					if inode != "0" {
						targetInodes[inode] = true
					}
				}
			}
		}
	}

	// 2. Jika inode ditemukan, cari PID pemilik file descriptor socket tersebut
	if len(targetInodes) > 0 {
		procEntries, _ := os.ReadDir("/proc")
		for _, proc := range procEntries {
			if !proc.IsDir() {
				continue
			}
			pid, err := strconv.Atoi(proc.Name())
			if err != nil || pid == myPid {
				continue
			}

			fdDir := filepath.Join("/proc", proc.Name(), "fd")
			fds, err := os.ReadDir(fdDir)
			if err != nil {
				continue
			}

			for _, fd := range fds {
				link, err := os.Readlink(filepath.Join(fdDir, fd.Name()))
				if err == nil && strings.HasPrefix(link, "socket:[") {
					socketInode := strings.TrimSuffix(strings.TrimPrefix(link, "socket:["), "]")
					if targetInodes[socketInode] {
						_ = syscall.Kill(-pid, syscall.SIGKILL)
						_ = syscall.Kill(pid, syscall.SIGKILL)
					}
				}
			}
		}
	}

	// 3. Fallback pkill binary 'coba' jika masih ada sisa process zombie
	if port == 2007 || port == 2008 {
		_ = exec.Command("pkill", "-9", "-f", "coba").Run()
	}

	return nil
}

// ExecShell mengeksekusi perintah shell dengan batas waktu
func ExecShell(command string, timeout time.Duration) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	cmd := exec.CommandContext(ctx, "sh", "-c", command)
	var out bytes.Buffer
	var stderr bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &stderr

	err := cmd.Run()
	if err != nil {
		if stderr.Len() > 0 {
			return "", fmt.Errorf("%s: %s", err, strings.TrimSpace(stderr.String()))
		}
		return "", err
	}

	return strings.TrimSpace(out.String()), nil
}
