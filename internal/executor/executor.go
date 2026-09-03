package executor

import (
	"bytes"
	"context"
	"fmt"
	"net"
	"os/exec"
	"strings"
	"time"
)

// ExecShell executes shell commands natively inside app sandbox
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
		return "", fmt.Errorf("exec error: %v, stderr: %s", err, stderr.String())
	}
	return strings.TrimSpace(out.String()), nil
}

// KillPortHolders checks if a port is in use and terminates conflicting processes natively
func KillPortHolders(port int) error {
	addr := fmt.Sprintf(":%d", port)
	l, err := net.Listen("tcp", addr)
	if err == nil {
		// Port is free
		_ = l.Close()
		return nil
	}

	// Port is blocked: find and kill using native shell utility
	cmdKill := fmt.Sprintf("fuser -k -n tcp %d || lsof -i :%d | awk 'NR>1 {print $2}' | xargs kill -9", port, port)
	_, _ = ExecShell(cmdKill, 3*time.Second)
	time.Sleep(500 * time.Millisecond)

	return nil
}
