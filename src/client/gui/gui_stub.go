// SPDX-License-Identifier: MIT
// AI.md PART 32: GUI stub — compiled when -tags gui is NOT provided.
// Standard CGO_ENABLED=0 builds always use this file; native GUI requires -tags gui.

//go:build !gui

package gui

import "errors"

// ErrGUIUnsupported is returned when the binary was not compiled with -tags gui.
var ErrGUIUnsupported = errors.New("GUI not available: binary was not built with -tags gui")

// Config holds the configuration passed to the GUI launcher.
// Defined here (not in gui.go) so both stub and real builds share the same type.
type Config struct {
	ServerURL string
	Token     string
	Version   string
	BinaryName string
}

// IsAvailable returns false in standard (non-GUI) builds.
func IsAvailable() bool {
	return false
}

// Launch always returns ErrGUIUnsupported in standard builds.
func Launch(_ *Config) error {
	return ErrGUIUnsupported
}
