// SPDX-License-Identifier: MIT
// AI.md PART 33: CLI Client - Cross-platform browser opening
package browser

import (
	"fmt"
	"os/exec"
	"runtime"
)

// Browser open command constants
// Per AI.md PART 1: No magic strings - use named constants
const (
	cmdXDGOpen    = "xdg-open"
	cmdOpen       = "open"
	cmdWindowsCmd = "cmd"
	cmdWindowsArg = "/c"
	cmdStart      = "start"

	platformLinux   = "linux"
	platformDarwin  = "darwin"
	platformWindows = "windows"
	platformFreeBSD = "freebsd"
	platformOpenBSD = "openbsd"
	platformNetBSD  = "netbsd"
)

// OpenURL opens a URL in the default browser
// Per AI.md PART 1: Function names MUST reveal intent
func OpenURL(url string) error {
	var cmd *exec.Cmd

	switch runtime.GOOS {
	case platformLinux:
		// Try xdg-open first, fallback to alternatives
		cmd = exec.Command(cmdXDGOpen, url)
	case platformDarwin:
		cmd = exec.Command(cmdOpen, url)
	case platformWindows:
		// Use cmd /c start to handle URLs properly
		cmd = exec.Command(cmdWindowsCmd, cmdWindowsArg, cmdStart, "", url)
	case platformFreeBSD, platformOpenBSD, platformNetBSD:
		// BSD systems typically have xdg-open or can use open
		cmd = exec.Command(cmdXDGOpen, url)
	default:
		return fmt.Errorf("unsupported platform: %s", runtime.GOOS)
	}

	// Run in background, don't wait for browser to close
	return cmd.Start()
}

// CanOpenBrowser checks if we can open a browser on this system
// Per AI.md PART 1: Function names MUST reveal intent
func CanOpenBrowser() bool {
	switch runtime.GOOS {
	case platformLinux, platformFreeBSD, platformOpenBSD, platformNetBSD:
		// Check if xdg-open is available
		_, err := exec.LookPath(cmdXDGOpen)
		return err == nil
	case platformDarwin, platformWindows:
		// Always available on macOS and Windows
		return true
	default:
		return false
	}
}
