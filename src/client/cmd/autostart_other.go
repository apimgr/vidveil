// SPDX-License-Identifier: MIT
//go:build !linux && !darwin && !windows

package cmd

import "errors"

func platformCLIAutostartEnabled() bool { return false }

func setPlatformCLIAutostartEnabled(bool) error {
	return errors.New("CLI autostart is not supported on this platform")
}
