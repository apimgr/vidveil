// SPDX-License-Identifier: MIT
// AI.md PART 32: Linux/BSD GUI stub — pending native GUI implementation decision.
// CGO_ENABLED=0 is always required; gotk4 uses CGO internally and is therefore
// not usable. Pure-Go GUI toolkit selection (fyne.io or similar) requires user
// decision per TODO.AI.md PART 32 before implementation proceeds.

//go:build (linux || freebsd || openbsd || netbsd) && gui

package gui

func launchNativeGUI(_ *Config) error {
	return ErrGUIUnsupported
}
