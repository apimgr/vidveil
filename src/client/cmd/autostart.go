// SPDX-License-Identifier: MIT
// AI.md PART 32: per-user CLI autostart entries for the native setup wizard.
package cmd

import "runtime"

func cliAutostartSupported() bool {
	return runtime.GOOS == "linux" || runtime.GOOS == "darwin" || runtime.GOOS == "windows"
}

func cliAutostartEnabled() bool {
	return platformCLIAutostartEnabled()
}

func setCLIAutostartEnabled(enabled bool) error {
	return setPlatformCLIAutostartEnabled(enabled)
}
