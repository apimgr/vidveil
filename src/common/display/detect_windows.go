// SPDX-License-Identifier: MIT
//go:build windows

// Package display provides display/terminal detection
// See AI.md PART 7 for specification
package display

import (
	"os"
)

// detectDisplay detects the display environment for Windows
func (e *DisplayEnv) detectDisplay() {
	e.detectWindowsDisplay()
}

// detectWindowsDisplay detects display on Windows
func (e *DisplayEnv) detectWindowsDisplay() {
	// Windows: check for console vs GUI session
	e.DisplayType = "windows"
	e.HasDisplay = true // Windows desktop always available unless service

	// Detect Windows service mode (no display)
	if os.Getenv("USERPROFILE") == "" {
		e.DisplayType = "none"
		e.HasDisplay = false
	}
}
