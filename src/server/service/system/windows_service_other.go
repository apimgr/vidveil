// SPDX-License-Identifier: MIT
// AI.md PART 24: Windows Service stubs for non-Windows
//go:build !windows

package system

import (
	"context"
	"errors"
)

// RunAsWindowsService reports that Windows services are unsupported here.
func RunAsWindowsService(runFunc func(context.Context) error) error {
	return errors.New("windows service not supported on this platform")
}

// IsWindowsService returns false on non-Windows platforms
func IsWindowsService() bool {
	return false
}
