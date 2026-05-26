// SPDX-License-Identifier: MIT
// AI.md PART 22: Update Command — Windows platform-specific binary replacement

//go:build windows

package updater

import (
	"fmt"
	"os"
	"os/exec"
	"time"

	"golang.org/x/sys/windows"
)

// replaceBinary replaces the running binary on Windows.
// Windows cannot delete or rename a running executable, so:
// 1. Rename current binary to .old (works while running)
// 2. Move new binary to current path
// 3. Schedule .old for deletion on reboot via MoveFileEx
func replaceBinary(currentPath, newBinaryPath string) error {
	oldPath := currentPath + ".old"

	// Remove any leftover .old from a previous update attempt
	_ = os.Remove(oldPath)

	// Rename running binary to .old
	if err := os.Rename(currentPath, oldPath); err != nil {
		return fmt.Errorf("rename current binary: %w", err)
	}

	// Move new binary to current path
	if err := os.Rename(newBinaryPath, currentPath); err != nil {
		// Attempt to restore original on failure
		_ = os.Rename(oldPath, currentPath)
		return fmt.Errorf("move new binary: %w", err)
	}

	// Schedule old binary for deletion on reboot (MoveFileEx DELAY_UNTIL_REBOOT)
	oldPathPtr, err := windows.UTF16PtrFromString(oldPath)
	if err == nil {
		_ = windows.MoveFileEx(oldPathPtr, nil, windows.MOVEFILE_DELAY_UNTIL_REBOOT)
	}

	return nil
}

// restartSelf starts a new instance of the updated binary and exits the current
// process. Windows does not support syscall.Exec so we spawn a child and exit.
func restartSelf() error {
	exe, err := os.Executable()
	if err != nil {
		return fmt.Errorf("get executable: %w", err)
	}

	cmd := exec.Command(exe, os.Args[1:]...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Stdin = os.Stdin

	if err := cmd.Start(); err != nil {
		return fmt.Errorf("start new process: %w", err)
	}

	// Give the new process time to initialise before exiting
	time.Sleep(100 * time.Millisecond)
	os.Exit(0)
	return nil // unreachable
}

// restartWindowsService stops and starts the Windows service via sc.exe per
// AI.md PART 22.
func restartWindowsService() error {
	stopCmd := exec.Command("sc", "stop", "vidveil")
	_ = stopCmd.Run() // ignore error if not running

	time.Sleep(2 * time.Second)

	startCmd := exec.Command("sc", "start", "vidveil")
	return startCmd.Run()
}

// These stubs satisfy the build for non-Linux/Darwin/BSD paths referenced in
// updater.go's restartService switch.

func restartLinuxService() error  { return fmt.Errorf("not supported on windows") }
func restartDarwinService() error { return fmt.Errorf("not supported on windows") }
func restartBSDService() error    { return fmt.Errorf("not supported on windows") }
