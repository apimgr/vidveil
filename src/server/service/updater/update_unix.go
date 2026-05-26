// SPDX-License-Identifier: MIT
// AI.md PART 22: Update Command — Unix platform-specific binary replacement

//go:build !windows

package updater

import (
	"fmt"
	"os"
	"os/exec"
	"syscall"
)

// replaceBinary replaces the running binary atomically on Unix.
// On Unix we can rename over a running executable — the old binary stays in
// memory until the process exits; the new one takes over on next start.
func replaceBinary(currentPath, newBinaryPath string) error {
	info, err := os.Stat(currentPath)
	if err != nil {
		return fmt.Errorf("stat current binary: %w", err)
	}

	// Atomic rename: new binary replaces current
	if err := os.Rename(newBinaryPath, currentPath); err != nil {
		return fmt.Errorf("rename binary: %w", err)
	}

	// Restore original permissions
	if err := os.Chmod(currentPath, info.Mode()); err != nil {
		return fmt.Errorf("restore permissions: %w", err)
	}
	return nil
}

// restartSelf re-executes the current process via syscall.Exec on Unix,
// replacing the current process image with the new binary.
func restartSelf() error {
	exe, err := os.Executable()
	if err != nil {
		return fmt.Errorf("get executable: %w", err)
	}
	return syscall.Exec(exe, os.Args, os.Environ())
}

// restartLinuxService restarts the service via systemd or the generic service
// command per AI.md PART 22.
func restartLinuxService() error {
	if _, err := exec.LookPath("systemctl"); err == nil {
		cmd := exec.Command("systemctl", "restart", "vidveil")
		return cmd.Run()
	}
	cmd := exec.Command("service", "vidveil", "restart")
	return cmd.Run()
}

// restartDarwinService restarts via launchctl kickstart per AI.md PART 22.
func restartDarwinService() error {
	label := "io.github.apimgr.vidveil"
	cmd := exec.Command("launchctl", "kickstart", "-k", "system/"+label)
	return cmd.Run()
}

// restartBSDService restarts via service command on BSD systems.
func restartBSDService() error {
	cmd := exec.Command("service", "vidveil", "restart")
	return cmd.Run()
}
