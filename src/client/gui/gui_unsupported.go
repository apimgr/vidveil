// SPDX-License-Identifier: MIT
// AI.md PART 32: GUI mode — BSD fallback. gogpu/gogpu's windowing layer does
// not implement a BSD backend, so -tags gui on freebsd/netbsd/openbsd falls
// back to this stub (TUI/CLI remain available) instead of gui.go.

//go:build gui && (freebsd || netbsd || openbsd)

package gui

import "errors"

// errGUIUnsupportedBSD is returned when -tags gui is used on a BSD target,
// where no native GUI backend exists yet.
var errGUIUnsupportedBSD = errors.New("GUI not available: no gogpu/gogpu backend for this platform")

// Config holds the configuration passed to the GUI launcher.
// Defined here (not in gui.go) so this build variant shares the same type.
type Config struct {
	ServerURL  string
	Token      string
	Version    string
	BinaryName string
}

// IsAvailable always returns false on BSD — no GUI backend is implemented.
func IsAvailable() bool {
	return false
}

// Launch always returns errGUIUnsupportedBSD on BSD.
func Launch(_ *Config) error {
	return errGUIUnsupportedBSD
}
