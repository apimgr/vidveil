// SPDX-License-Identifier: MIT
// AI.md PART 8: Daemonization (Unix)
//go:build !windows

package daemon

import (
	"fmt"
	"os"
	"os/exec"
	"syscall"
)

// Daemonize forks the process and detaches from terminal per AI.md PART 8
func Daemonize() error {
	// Already daemonized? Check if parent is init (PID 1)
	if os.Getppid() == 1 {
		return nil
	}

	// Check if we are the child (re-executed with marker env var)
	if os.Getenv("_DAEMON_CHILD") != "" {
		// We are the child - continue execution
		return nil
	}

	// Prepare to re-exec as daemon
	execPath, err := os.Executable()
	if err != nil {
		return fmt.Errorf("getting executable path: %w", err)
	}

	// Build command with same args (minus --daemon to prevent loop)
	args := filterDaemonFlag(os.Args[1:])

	cmd := exec.Command(execPath, args...)
	cmd.Env = append(os.Environ(), "_DAEMON_CHILD=1")

	// Detach from terminal per AI.md PART 8 7908-7915
	cmd.Stdin = nil
	cmd.Stdout = nil
	cmd.Stderr = nil
	cmd.SysProcAttr = &syscall.SysProcAttr{
		// Create new session (detach from controlling terminal)
		Setsid: true,
	}

	if err := cmd.Start(); err != nil {
		return fmt.Errorf("starting daemon: %w", err)
	}

	// Parent exits, child continues per AI.md PART 8 7921-7923
	fmt.Printf("Daemon started with PID %d\n", cmd.Process.Pid)
	os.Exit(0)
	return nil
}

// filterDaemonFlag removes --daemon from args to prevent infinite loop per AI.md PART 8 7927-7936
func filterDaemonFlag(args []string) []string {
	filtered := make([]string, 0, len(args))
	for _, arg := range args {
		if arg != "--daemon" && arg != "-d" {
			filtered = append(filtered, arg)
		}
	}
	return filtered
}
