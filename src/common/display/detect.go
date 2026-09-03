// SPDX-License-Identifier: MIT
// Package display provides display/terminal detection
// See AI.md PART 7 for specification
package display

import (
	"os"
	"strings"

	"github.com/charmbracelet/x/term"
)

// DisplayEnv holds information about the current display environment
type DisplayEnv struct {
	// Mode is the current display mode
	Mode DisplayMode
	// HasDisplay indicates X11, Wayland, Windows, or macOS display
	HasDisplay bool
	// DisplayType is "x11", "wayland", "windows", "macos", or "none"
	DisplayType string
	// IsTerminal indicates stdout is a TTY
	IsTerminal bool
	// IsSSH indicates running over SSH
	IsSSH bool
	// IsMosh indicates running over mosh
	IsMosh bool
	// IsScreen indicates running in screen/tmux
	IsScreen bool
	// TerminalType is the TERM environment value
	TerminalType string
	// Cols is terminal columns (0 if no terminal)
	Cols int
	// Rows is terminal rows (0 if no terminal)
	Rows int
}

// DetectDisplayEnv detects the current display environment
// Per AI.md PART 1: "Detect()" alone doesn't say what it detects
func DetectDisplayEnv() DisplayEnv {
	env := DisplayEnv{}

	// Check terminal
	env.IsTerminal = term.IsTerminal(os.Stdout.Fd())
	if env.IsTerminal {
		w, h, err := term.GetSize(os.Stdout.Fd())
		if err == nil {
			env.Cols = w
			env.Rows = h
		}
	}
	env.TerminalType = os.Getenv("TERM")

	// Check remote session
	env.IsSSH = os.Getenv("SSH_CLIENT") != "" || os.Getenv("SSH_TTY") != ""
	env.IsMosh = os.Getenv("MOSH") != "" || strings.Contains(os.Getenv("TERM"), "mosh")
	env.IsScreen = os.Getenv("STY") != "" || os.Getenv("TMUX") != ""

	// Detect display environment (platform-specific)
	env.detectPlatformDisplay()

	// Auto-detect display mode
	env.Mode = env.autoDetectDisplayMode()

	return env
}

// autoDetectDisplayMode determines the display mode from the detected environment.
// Per AI.md PART 7: headless check runs first, then TERM=dumb, then GUI/TUI.
func (e *DisplayEnv) autoDetectDisplayMode() DisplayMode {
	// No TTY and no display = headless
	if !e.IsTerminal && !e.HasDisplay {
		return DisplayModeHeadless
	}

	// TERM=dumb: force CLI mode (no TUI, no ANSI escapes)
	if e.TerminalType == "dumb" {
		return DisplayModeCLI
	}

	// Has native display = GUI possible
	if e.HasDisplay && !e.IsSSH && !e.IsMosh {
		return DisplayModeGUI
	}

	// Has terminal = TUI possible
	if e.IsTerminal {
		return DisplayModeTUI
	}

	// Fallback to CLI
	return DisplayModeCLI
}

// IsDumbTerminal returns true if running in a dumb terminal (no ANSI support).
// Per AI.md PART 7: TERM=dumb forces CLI mode, no ANSI escapes, no emojis.
func (e *DisplayEnv) IsDumbTerminal() bool {
	return e.TerminalType == "dumb"
}

// IsAutoDetectDisplayModeGUI returns true if mode is GUI.
func (e DisplayEnv) IsAutoDetectDisplayModeGUI() bool { return e.Mode == DisplayModeGUI }

// IsAutoDetectDisplayModeTUI returns true if mode is TUI.
func (e DisplayEnv) IsAutoDetectDisplayModeTUI() bool { return e.Mode == DisplayModeTUI }

// IsAutoDetectDisplayModeCLI returns true if mode is CLI.
func (e DisplayEnv) IsAutoDetectDisplayModeCLI() bool { return e.Mode == DisplayModeCLI }

// IsAutoDetectDisplayModeHeadless returns true if mode is headless.
func (e DisplayEnv) IsAutoDetectDisplayModeHeadless() bool { return e.Mode == DisplayModeHeadless }

// String returns a string representation of the display environment
func (e DisplayEnv) String() string {
	return e.Mode.String()
}

// SupportsColors returns true if the terminal supports colors
func (e DisplayEnv) SupportsColors() bool {
	if !e.IsTerminal {
		return false
	}
	// Most modern terminals support colors
	t := strings.ToLower(e.TerminalType)
	return strings.Contains(t, "color") ||
		strings.Contains(t, "256") ||
		strings.Contains(t, "xterm") ||
		strings.Contains(t, "screen") ||
		strings.Contains(t, "tmux") ||
		t == "linux"
}

// SupportsUnicode returns true if the terminal supports Unicode
func (e DisplayEnv) SupportsUnicode() bool {
	// Check locale for UTF-8 support
	lang := os.Getenv("LANG")
	lcAll := os.Getenv("LC_ALL")
	return strings.Contains(strings.ToUpper(lang), "UTF") ||
		strings.Contains(strings.ToUpper(lcAll), "UTF")
}

// IsRemoteSession returns true if running over SSH or mosh
func IsRemoteSession() bool {
	// SSH detection
	if os.Getenv("SSH_CLIENT") != "" || os.Getenv("SSH_TTY") != "" {
		return true
	}
	// Mosh detection
	if os.Getenv("MOSH") != "" {
		return true
	}
	// Check for SSH connection string
	if os.Getenv("SSH_CONNECTION") != "" {
		return true
	}
	return false
}
