// SPDX-License-Identifier: MIT
// Package display provides display/terminal detection
// See AI.md PART 7 for specification
package display

// Mode represents the display mode for the application
type Mode int

const (
	ModeHeadless Mode = iota // No display, no TTY (daemon, service, cron)
	ModeCLI                  // Command-line only (piped or command provided)
	ModeTUI                  // Terminal UI (interactive terminal)
	ModeGUI                  // Native graphical UI
)

// String returns the string representation of the display mode
func (m Mode) String() string {
	return [...]string{"headless", "cli", "tui", "gui"}[m]
}

// SupportsInteraction returns true if the mode supports user interaction
func (m Mode) SupportsInteraction() bool {
	return m >= ModeTUI
}

// SupportsColors returns true if the mode supports ANSI colors
func (m Mode) SupportsColors() bool {
	return m >= ModeCLI
}
