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
	Mode         Mode   // Current display mode
	HasDisplay   bool   // X11, Wayland, Windows, macOS display
	DisplayType  string // "x11", "wayland", "windows", "macos", "none"
	IsTerminal   bool   // stdout is a TTY
	IsSSH        bool   // Running over SSH
	IsMosh       bool   // Running over mosh
	IsScreen     bool   // Running in screen/tmux
	TerminalType string // TERM value
	Cols         int    // Terminal columns (0 if no terminal)
	Rows         int    // Terminal rows (0 if no terminal)
}

// Detect detects the current display environment
func Detect() DisplayEnv {
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
	env.detectDisplay()

	// Determine mode
	env.Mode = env.determineMode()

	return env
}

// determineMode determines the display mode based on environment
func (e *DisplayEnv) determineMode() Mode {
	// No TTY and no display = headless
	if !e.IsTerminal && !e.HasDisplay {
		return ModeHeadless
	}

	// Has native display = GUI possible
	if e.HasDisplay && !e.IsSSH && !e.IsMosh {
		return ModeGUI
	}

	// Has terminal = TUI possible
	if e.IsTerminal {
		return ModeTUI
	}

	// Fallback to CLI
	return ModeCLI
}

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
