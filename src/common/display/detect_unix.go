// SPDX-License-Identifier: MIT
//go:build !windows

// Package display provides display/terminal detection
// See AI.md PART 7 for specification
package display

import (
	"os"
	"os/exec"
	"runtime"
)

// detectDisplay detects the display environment for Unix-like systems
func (e *DisplayEnv) detectDisplay() {
	switch runtime.GOOS {
	case "linux", "freebsd", "openbsd", "netbsd":
		e.detectUnixDisplay()
	case "darwin":
		e.detectMacOSDisplay()
	default:
		e.DisplayType = "none"
		e.HasDisplay = false
	}
}

// detectUnixDisplay detects display on Linux/BSD
func (e *DisplayEnv) detectUnixDisplay() {
	// Check Wayland first (preferred on modern Linux)
	if os.Getenv("WAYLAND_DISPLAY") != "" {
		e.DisplayType = "wayland"
		e.HasDisplay = true
		return
	}

	// Check X11
	display := os.Getenv("DISPLAY")
	if display != "" {
		// Verify X server is actually accessible
		cmd := exec.Command("xset", "q")
		if err := cmd.Run(); err == nil {
			e.DisplayType = "x11"
			e.HasDisplay = true
			return
		}
	}

	e.DisplayType = "none"
	e.HasDisplay = false
}

// detectMacOSDisplay detects display on macOS
func (e *DisplayEnv) detectMacOSDisplay() {
	// macOS always has a display unless running headless
	// Check if we're in a graphical session
	if os.Getenv("TERM_PROGRAM") != "" || os.Getenv("Apple_PubSub_Socket_Render") != "" {
		e.DisplayType = "macos"
		e.HasDisplay = true
		return
	}

	// SSH session to macOS - no local display access
	if e.IsSSH {
		e.DisplayType = "none"
		e.HasDisplay = false
		return
	}

	// Assume display available on macOS
	e.DisplayType = "macos"
	e.HasDisplay = true
}
