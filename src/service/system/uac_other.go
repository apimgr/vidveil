// SPDX-License-Identifier: MIT
//go:build !windows

// TEMPLATE.md PART 4: Non-Windows stub for UAC functions
package system

import (
	"fmt"
	"os"
	"os/exec"
)

// ElevationResult represents the result of an elevation attempt
type ElevationResult int

const (
	ElevationAlreadyAdmin ElevationResult = iota
	ElevationSuccess
	ElevationCanceled
	ElevationFailed
)

// IsElevated checks if the current process has root privileges
func IsElevated() bool {
	return os.Getuid() == 0
}

// RequestElevation attempts to restart with elevated privileges using sudo/doas/pkexec
// per TEMPLATE.md PART 4 escalation order
func RequestElevation(args ...string) ElevationResult {
	if IsElevated() {
		return ElevationAlreadyAdmin
	}

	// This would restart with sudo - but typically we just check and fail
	return ElevationFailed
}

// RunAsAdmin runs a command with elevated privileges
func RunAsAdmin(command string, args ...string) error {
	if IsElevated() {
		cmd := exec.Command(command, args...)
		return cmd.Run()
	}

	// Try escalation methods in priority order per TEMPLATE.md PART 4
	escalator := DetectEscalation()
	if escalator == "" {
		return fmt.Errorf("no escalation method available (need sudo, doas, or pkexec)")
	}

	fullArgs := append([]string{command}, args...)
	cmd := exec.Command(escalator, fullArgs...)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

// RequireAdmin checks if root privileges are required and returns error if not available
// Returns false (no exit needed) and nil error if already root
// Returns false and error if not root and no escalation available
func RequireAdmin(operation string) (bool, error) {
	if IsElevated() {
		return false, nil
	}

	return false, fmt.Errorf("operation '%s' requires root privileges. Try running with sudo", operation)
}

// GetWindowsServiceAccount returns empty string on non-Windows
func GetWindowsServiceAccount(serviceName string) string {
	return ""
}

// IsRunningAsService checks if running as a system service
func IsRunningAsService() bool {
	// Check if started by systemd or similar
	return os.Getppid() == 1
}
