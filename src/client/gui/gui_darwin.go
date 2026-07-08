// SPDX-License-Identifier: MIT
// AI.md PART 32: macOS GUI stub — pending native GUI implementation decision.
// CGO is forbidden (CGO_ENABLED=0 always). Native macOS GUI implementation
// via fyne.io or another pure-Go toolkit requires user decision per TODO.AI.md.

//go:build darwin && gui

package gui

func launchCocoaGui(_ *Config) error {
	return ErrGUIUnsupported
}
