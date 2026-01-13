// SPDX-License-Identifier: MIT
// Package display provides display/terminal detection
// See AI.md PART 7 for specification
package display

// DisplayMode represents the display mode for the application
// Per AI.md PART 1: "Mode" alone is ambiguous - could be app mode, theme mode, etc.
type DisplayMode int

const (
	DisplayModeHeadless DisplayMode = iota // No display, no TTY (daemon, service, cron)
	DisplayModeCLI                         // Command-line only (piped or command provided)
	DisplayModeTUI                         // Terminal UI (interactive terminal)
	DisplayModeGUI                         // Native graphical UI
)

// String returns the string representation of the display mode
func (m DisplayMode) String() string {
	return [...]string{"headless", "cli", "tui", "gui"}[m]
}

// SupportsInteraction returns true if the mode supports user interaction
func (m DisplayMode) SupportsInteraction() bool {
	return m >= DisplayModeTUI
}

// SupportsColors returns true if the mode supports ANSI colors
func (m DisplayMode) SupportsColors() bool {
	return m >= DisplayModeCLI
}
