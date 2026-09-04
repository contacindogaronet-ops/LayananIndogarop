package executor

import (
	"bytes"
	"context"
	"fmt"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"
)

// IPToHex mengonversi IP v4 string ke format hex little-endian Linux kernel /proc/net/tcp
func ipToHex(ipStr string) string {
	ip := net.ParseIP(ipStr).To4()
	if ip == nil {
		return ""
	}
	return fmt.Sprintf("%02X%02X%02X%02X", ip[3], ip[2], ip[1], ip[0])
}

// KillSpecificIPPort membersihkan socket pada IP:Port tertentu (misal 127.0.0.3:2007)
func KillSpecificIPPort(targetIP string, targetPort int) error {
	hexPort := fmt.Sprintf("%04X", targetPort)
	hexIP := ipToHex(targetIP)
	myPid := os.Getpid()

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
				if len(parts) == 2 {
					ipMatches := (hexIP == "" || strings.EqualFold(parts[0], hexIP) || parts[0] == "00000000")
					portMatches := strings.EqualFold(parts[1], hexPort)

					if ipMatches && portMatches {
						inode := fields[9]
						if inode != "0" {
							targetInodes[inode] = true
						}
					}
				}
			}
		}
	}

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
						// Kill Process Group dan Process Langsung
						_ = syscall.Kill(-pid, syscall.SIGKILL)
						_ = syscall.Kill(pid, syscall.SIGKILL)
					}
				}
			}
		}
	}

	// Fallback pkill pada binary 'coba'
	if targetPort == 2007 || targetPort == 2008 {
		_ = exec.Command("pkill", "-9", "-f", "coba").Run()
	}

	return nil
}

// KillPortHolders membersihkan pemegang port pada semua antarmuka (0.0.0.0 / ANY)
func KillPortHolders(port int) error {
	return KillSpecificIPPort("", port)
}

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
