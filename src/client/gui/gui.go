// SPDX-License-Identifier: MIT
// AI.md PART 32: GUI mode — compiled only when -tags gui is provided.
// Native GUI uses platform-specific toolkits: GTK4 (Linux/BSD), Cocoa (macOS), Win32 (Windows).

//go:build gui

package gui

import (
	"errors"
	"runtime"

	"github.com/apimgr/vidveil/src/common/display"
)

// ErrGUIUnsupported is returned when the current platform has no GUI launcher.
var ErrGUIUnsupported = errors.New("GUI not supported on this platform")

// Config holds the configuration passed to the GUI launcher.
type Config struct {
	ServerURL  string
	Token      string
	Version    string
	BinaryName string
}

// IsAvailable reports whether a native GUI can be launched in the current environment.
// Remote sessions (SSH/Mosh) never have GUI even when DISPLAY is set.
func IsAvailable() bool {
	env := display.DetectDisplayEnv()
	return env.HasDisplay && !env.IsSSH && !env.IsMosh
}

// Launch starts the native GUI application for the current platform.
// Falls back to ErrGUIUnsupported on unsupported platforms.
func Launch(cfg *Config) error {
	switch runtime.GOOS {
	case "linux":
		return launchGTKGui(cfg)
	case "darwin":
		return launchCocoaGui(cfg)
	case "windows":
		return launchWin32Gui(cfg)
	case "freebsd", "openbsd", "netbsd":
		return launchGTKGui(cfg)
	default:
		return ErrGUIUnsupported
	}
}
