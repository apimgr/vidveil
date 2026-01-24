// SPDX-License-Identifier: MIT
// AI.md PART 8: Signal Handling (Unix)
//go:build !windows

package signal

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"
)

// Per AI.md PART 8: Unix signal handling
// SIGTERM (15) → Graceful shutdown
// SIGINT (2)   → Graceful shutdown
// SIGQUIT (3)  → Graceful shutdown
// SIGHUP (1)   → Ignored (config auto-reloads via file watcher)
// SIGUSR1 (10) → Reopen logs (log rotation)
// SIGUSR2 (12) → Status dump
// SIGRTMIN+3 (37) → Graceful shutdown (Docker STOPSIGNAL)

var (
	shuttingDown bool
	logReopenFn  func()
	statusDumpFn func()
)

// SetLogReopenFunc sets the function called on SIGUSR1
func SetLogReopenFunc(fn func()) {
	logReopenFn = fn
}

// SetStatusDumpFunc sets the function called on SIGUSR2
func SetStatusDumpFunc(fn func()) {
	statusDumpFn = fn
}

// IsShuttingDown returns true if shutdown is in progress
func IsShuttingDown() bool {
	return shuttingDown
}

// SetupSignalHandler configures graceful shutdown per AI.md PART 8
func SetupSignalHandler(server *http.Server, pidFile string) {
	sigChan := make(chan os.Signal, 1)

	// Register signals per PART 8
	signal.Notify(sigChan,
		syscall.SIGTERM,  // 15 - kill (default)
		syscall.SIGINT,   // 2 - Ctrl+C
		syscall.SIGQUIT,  // 3 - Ctrl+\
		syscall.SIGUSR1,  // 10 - Reopen logs
		syscall.SIGUSR2,  // 12 - Status dump
	)

	// Handle SIGRTMIN+3 (37) - Docker STOPSIGNAL per PART 8
	signal.Notify(sigChan, syscall.Signal(37))

	// Ignore SIGHUP - config reloads automatically via file watcher per PART 8
	signal.Ignore(syscall.SIGHUP)

	go func() {
		for sig := range sigChan {
			switch sig {
			case syscall.SIGUSR1:
				// Reopen logs for rotation per PART 8
				log.Println("Received SIGUSR1, reopening logs...")
				if logReopenFn != nil {
					logReopenFn()
				}

			case syscall.SIGUSR2:
				// Dump status to log per PART 8
				log.Println("Received SIGUSR2, dumping status...")
				if statusDumpFn != nil {
					statusDumpFn()
				}

			default:
				// Graceful shutdown (SIGTERM, SIGINT, SIGQUIT, SIGRTMIN+3)
				log.Printf("Received %v, starting graceful shutdown...", sig)
				gracefulShutdown(server, pidFile)
			}
		}
	}()
}

// WaitForShutdown blocks until a shutdown signal is received
func WaitForShutdown(ctx context.Context) os.Signal {
	quit := make(chan os.Signal, 1)
	signal.Notify(quit,
		syscall.SIGINT,
		syscall.SIGTERM,
		syscall.SIGQUIT,
	)
	// SIGRTMIN+3 for systemd socket activation
	signal.Notify(quit, syscall.Signal(37))

	// Ignore SIGHUP per PART 8
	signal.Ignore(syscall.SIGHUP)

	select {
	case sig := <-quit:
		return sig
	case <-ctx.Done():
		return syscall.SIGTERM
	}
}

// NotifyReload registers a reload signal handler
// Note: Per PART 8, SIGHUP is ignored - config auto-reloads via file watcher
// This function is kept for backwards compatibility but is a no-op
func NotifyReload(handler func()) {
	// SIGHUP is ignored per PART 8
	// Config reloads automatically via file watcher
}

// GetStopSignal returns the appropriate stop signal for this platform
func GetStopSignal() os.Signal {
	return syscall.SIGTERM
}

// gracefulShutdown performs orderly shutdown per AI.md PART 8
func gracefulShutdown(server *http.Server, pidFile string) {
	// Set shutdown flag for health checks (return 503)
	shuttingDown = true

	// Create context with 30s timeout per PART 8
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Stop accepting new connections, wait for in-flight requests
	if server != nil {
		if err := server.Shutdown(ctx); err != nil {
			log.Printf("Shutdown error: %v", err)
		}
	}

	// Remove PID file per PART 8
	if pidFile != "" {
		os.Remove(pidFile)
	}

	// Exit
	os.Exit(0)
}

// KillProcess sends signal to process per AI.md PART 8
func KillProcess(pid int, graceful bool) error {
	process, err := os.FindProcess(pid)
	if err != nil {
		return err
	}
	if graceful {
		return process.Signal(syscall.SIGTERM)
	}
	return process.Signal(syscall.SIGKILL)
}

// isProcessRunning checks if a process with given PID exists (Unix)
// Per AI.md PART 8
func isProcessRunning(pid int) bool {
	process, err := os.FindProcess(pid)
	if err != nil {
		return false
	}
	// On Unix, FindProcess always succeeds - need to send signal 0
	err = process.Signal(syscall.Signal(0))
	return err == nil
}

// isOurProcess verifies the process is actually our binary (Unix)
// Per AI.md PART 8
// Uses exact binary name matching to prevent false positives
func isOurProcess(pid int, binaryName string) bool {
	// Read /proc/{pid}/exe symlink (Linux)
	exePath, err := os.Readlink(fmt.Sprintf("/proc/%d/exe", pid))
	if err != nil {
		// On macOS/BSD, use ps command
		return isOurProcessDarwin(pid, binaryName)
	}
	// Per AI.md PART 8: Use exact matching, not substring
	// Prevents false positives (e.g., "vid" matching "vidveil")
	return filepath.Base(exePath) == binaryName
}

// isOurProcessDarwin checks process on macOS/BSD
func isOurProcessDarwin(pid int, binaryName string) bool {
	cmd := exec.Command("ps", "-p", strconv.Itoa(pid), "-o", "comm=")
	output, err := cmd.Output()
	if err != nil {
		return false
	}
	// Per AI.md PART 8: Use exact matching, not substring
	return strings.TrimSpace(string(output)) == binaryName
}

// CheckPIDFile checks if PID file exists and if the process is still running
// Per AI.md PART 8
// Returns: (isRunning bool, pid int, err error)
func CheckPIDFile(pidPath string, binaryName string) (bool, int, error) {
	data, err := os.ReadFile(pidPath)
	if os.IsNotExist(err) {
		// No PID file, not running
		return false, 0, nil
	}
	if err != nil {
		return false, 0, fmt.Errorf("reading pid file: %w", err)
	}

	pid, err := strconv.Atoi(strings.TrimSpace(string(data)))
	if err != nil {
		// Corrupt PID file - remove it
		os.Remove(pidPath)
		return false, 0, nil
	}

	// Check if process is running
	if !isProcessRunning(pid) {
		// Stale PID file - remove it
		os.Remove(pidPath)
		return false, 0, nil
	}

	// Process exists - verify it's actually our process (not PID reuse)
	if !isOurProcess(pid, binaryName) {
		// PID was reused by another process - remove stale file
		os.Remove(pidPath)
		return false, 0, nil
	}

	return true, pid, nil
}

// WritePIDFile writes current process PID to file
// Per AI.md PART 8
func WritePIDFile(pidPath string, binaryName string) error {
	// Check for existing running instance first
	running, existingPID, err := CheckPIDFile(pidPath, binaryName)
	if err != nil {
		return err
	}
	if running {
		return fmt.Errorf("already running (pid %d)", existingPID)
	}

	// Create parent directory if needed
	pidDir := filepath.Dir(pidPath)
	perm := os.FileMode(0755)
	if os.Getuid() != 0 {
		perm = 0700
	}
	if err := os.MkdirAll(pidDir, perm); err != nil {
		return fmt.Errorf("creating pid directory: %w", err)
	}

	// Write our PID
	pid := os.Getpid()
	filePerm := os.FileMode(0644)
	if os.Getuid() != 0 {
		filePerm = 0600
	}
	return os.WriteFile(pidPath, []byte(strconv.Itoa(pid)), filePerm)
}

// RemovePIDFile removes PID file on shutdown
func RemovePIDFile(pidPath string) error {
	return os.Remove(pidPath)
}
