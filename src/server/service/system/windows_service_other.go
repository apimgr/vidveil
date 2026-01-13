// SPDX-License-Identifier: MIT
// AI.md PART 25: Windows Service stubs for non-Windows
//go:build !windows

package system

import "errors"

// WindowsServiceName is the service name for Windows
const WindowsServiceName = "vidveil"

// RunAsWindowsService is a no-op on non-Windows platforms
func RunAsWindowsService(runFunc func() error) error {
	return errors.New("Windows service not supported on this platform")
}

// IsWindowsService returns false on non-Windows platforms
func IsWindowsService() bool {
	return false
}
