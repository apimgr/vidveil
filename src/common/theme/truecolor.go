// SPDX-License-Identifier: MIT
// Package theme provides unified theming
// See AI.md PART 7 for specification
package theme

import "github.com/muesli/termenv"

// SupportsTrueColor reports whether the current terminal has advertised
// true-color (24-bit) support. Per AI.md PART 16 "CLI/TUI Color Mapping",
// the ANSI-mapped TerminalPalette is the required baseline for CLI/TUI
// output — the literal hex ColorPalette may only be used as an opt-in
// enhancement, and only when this returns true.
func SupportsTrueColor() bool {
	return termenv.ColorProfile() == termenv.TrueColor
}
