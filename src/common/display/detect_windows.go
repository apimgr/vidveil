// SPDX-License-Identifier: MIT
//go:build windows

// Package display provides display/terminal detection
// See AI.md PART 7 for specification
package display

import (
	"os"
	"strings"

	"golang.org/x/sys/windows"
)

// detectPlatformDisplay detects the display environment for Windows
func (e *DisplayEnv) detectPlatformDisplay() {
	e.detectWindowsDisplay()
}

// detectWindowsDisplay detects display on Windows
func (e *DisplayEnv) detectWindowsDisplay() {
	kernel32 := windows.NewLazySystemDLL("kernel32.dll")
	getConsoleWindow := kernel32.NewProc("GetConsoleWindow")
	hwnd, _, _ := getConsoleWindow.Call()

	var sessionID uint32
	if err := windows.ProcessIdToSessionId(windows.GetCurrentProcessId(), &sessionID); err != nil || sessionID == 0 {
		e.DisplayType = "none"
		e.HasDisplay = false
		return
	}

	if strings.HasPrefix(os.Getenv("SESSIONNAME"), "RDP-Tcp") {
		e.DisplayType = "windows-rdp"
		e.HasDisplay = true
		return
	}

	e.HasDisplay = hwnd != 0
	if e.HasDisplay {
		e.DisplayType = "windows"
	} else {
		e.DisplayType = "none"
	}
}
