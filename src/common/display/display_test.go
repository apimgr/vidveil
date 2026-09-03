// SPDX-License-Identifier: MIT
// Package display - Tests for terminal/display detection
// See AI.md PART 7 for specification
package display

import (
	"os"
	"testing"
)

// --- DisplayMode constants ---

// Iota ordering must be stable: Headless < CLI < TUI < GUI.
func TestDisplayModeConstantOrder(t *testing.T) {
	if DisplayModeHeadless >= DisplayModeCLI {
		t.Errorf("DisplayModeHeadless (%d) must be < DisplayModeCLI (%d)", DisplayModeHeadless, DisplayModeCLI)
	}
	if DisplayModeCLI >= DisplayModeTUI {
		t.Errorf("DisplayModeCLI (%d) must be < DisplayModeTUI (%d)", DisplayModeCLI, DisplayModeTUI)
	}
	if DisplayModeTUI >= DisplayModeGUI {
		t.Errorf("DisplayModeTUI (%d) must be < DisplayModeGUI (%d)", DisplayModeTUI, DisplayModeGUI)
	}
}

// --- DisplayMode.String ---

func TestDisplayModeString(t *testing.T) {
	tests := []struct {
		mode DisplayMode
		want string
	}{
		{DisplayModeHeadless, "headless"},
		{DisplayModeCLI, "cli"},
		{DisplayModeTUI, "tui"},
		{DisplayModeGUI, "gui"},
	}
	for _, tt := range tests {
		t.Run(tt.want, func(t *testing.T) {
			if got := tt.mode.String(); got != tt.want {
				t.Errorf("DisplayMode(%d).String() = %q, want %q", tt.mode, got, tt.want)
			}
		})
	}
}

// --- DisplayMode.SupportsInteraction ---

// Only TUI and GUI support interaction; Headless and CLI do not.
func TestDisplayModeSupportsInteraction(t *testing.T) {
	tests := []struct {
		mode DisplayMode
		want bool
	}{
		{DisplayModeHeadless, false},
		{DisplayModeCLI, false},
		{DisplayModeTUI, true},
		{DisplayModeGUI, true},
	}
	for _, tt := range tests {
		t.Run(tt.mode.String(), func(t *testing.T) {
			if got := tt.mode.SupportsInteraction(); got != tt.want {
				t.Errorf("%s.SupportsInteraction() = %v, want %v", tt.mode, got, tt.want)
			}
		})
	}
}

// --- DisplayMode.SupportsColors (on the mode type) ---

// Headless does not support colors; all others do.
func TestDisplayModeSupportsColors(t *testing.T) {
	tests := []struct {
		mode DisplayMode
		want bool
	}{
		{DisplayModeHeadless, false},
		{DisplayModeCLI, true},
		{DisplayModeTUI, true},
		{DisplayModeGUI, true},
	}
	for _, tt := range tests {
		t.Run(tt.mode.String(), func(t *testing.T) {
			if got := tt.mode.SupportsColors(); got != tt.want {
				t.Errorf("%s.SupportsColors() = %v, want %v", tt.mode, got, tt.want)
			}
		})
	}
}

// --- DisplayEnv.IsDumbTerminal ---

func TestIsDumbTerminal(t *testing.T) {
	dumb := DisplayEnv{TerminalType: "dumb"}
	if !dumb.IsDumbTerminal() {
		t.Error("IsDumbTerminal() = false for TERM=dumb, want true")
	}

	xterm := DisplayEnv{TerminalType: "xterm-256color"}
	if xterm.IsDumbTerminal() {
		t.Error("IsDumbTerminal() = true for TERM=xterm-256color, want false")
	}

	empty := DisplayEnv{TerminalType: ""}
	if empty.IsDumbTerminal() {
		t.Error("IsDumbTerminal() = true for empty TERM, want false")
	}
}

// --- DisplayEnv.autoDetectDisplayMode ---

// No TTY and no display must produce Headless.
func TestAutoDetectDisplayModeHeadless(t *testing.T) {
	e := &DisplayEnv{IsTerminal: false, HasDisplay: false, TerminalType: "xterm"}
	if got := e.autoDetectDisplayMode(); got != DisplayModeHeadless {
		t.Errorf("autoDetectDisplayMode() = %s, want headless", got)
	}
}

// TERM=dumb with any terminal must produce CLI.
func TestAutoDetectDisplayModeDumbTerminal(t *testing.T) {
	e := &DisplayEnv{IsTerminal: true, HasDisplay: false, TerminalType: "dumb"}
	if got := e.autoDetectDisplayMode(); got != DisplayModeCLI {
		t.Errorf("autoDetectDisplayMode() with dumb TERM = %s, want cli", got)
	}
}

// TERM=dumb even with a display must produce CLI (dumb check comes first).
func TestAutoDetectDisplayModeDumbTerminalWithDisplay(t *testing.T) {
	e := &DisplayEnv{IsTerminal: true, HasDisplay: true, TerminalType: "dumb"}
	if got := e.autoDetectDisplayMode(); got != DisplayModeCLI {
		t.Errorf("autoDetectDisplayMode() dumb+display = %s, want cli", got)
	}
}

// Has display, not SSH, not Mosh must produce GUI.
func TestAutoDetectDisplayModeGUI(t *testing.T) {
	e := &DisplayEnv{IsTerminal: true, HasDisplay: true, TerminalType: "xterm-256color", IsSSH: false, IsMosh: false}
	if got := e.autoDetectDisplayMode(); got != DisplayModeGUI {
		t.Errorf("autoDetectDisplayMode() with display, no SSH = %s, want gui", got)
	}
}

// Has display but running over SSH must fall through to TUI.
func TestAutoDetectDisplayModeSSHWithDisplay(t *testing.T) {
	e := &DisplayEnv{IsTerminal: true, HasDisplay: true, TerminalType: "xterm", IsSSH: true}
	if got := e.autoDetectDisplayMode(); got != DisplayModeTUI {
		t.Errorf("autoDetectDisplayMode() SSH+display = %s, want tui", got)
	}
}

// TTY available, no display, no SSH must produce TUI.
func TestAutoDetectDisplayModeTUI(t *testing.T) {
	e := &DisplayEnv{IsTerminal: true, HasDisplay: false, TerminalType: "xterm-256color"}
	if got := e.autoDetectDisplayMode(); got != DisplayModeTUI {
		t.Errorf("autoDetectDisplayMode() tty no display = %s, want tui", got)
	}
}

// No TTY but has display: headless path still wins (no terminal, check order).
func TestAutoDetectDisplayModeNoTTYWithDisplay(t *testing.T) {
	e := &DisplayEnv{IsTerminal: false, HasDisplay: true, TerminalType: "xterm"}
	// Per spec: "No TTY and no display = headless" — HasDisplay is true so NOT headless.
	// Falls through: TERM != dumb, HasDisplay+!SSH -> GUI.
	got := e.autoDetectDisplayMode()
	if got != DisplayModeGUI {
		t.Errorf("autoDetectDisplayMode() no-tty+display = %s, want gui", got)
	}
}

// Mosh session with display must fall to TUI, not GUI.
func TestAutoDetectDisplayModeMoshWithDisplay(t *testing.T) {
	e := &DisplayEnv{IsTerminal: true, HasDisplay: true, TerminalType: "xterm", IsMosh: true}
	if got := e.autoDetectDisplayMode(); got != DisplayModeTUI {
		t.Errorf("autoDetectDisplayMode() mosh+display = %s, want tui", got)
	}
}

// --- DisplayEnv helper predicates ---

func TestDisplayEnvModePredicates(t *testing.T) {
	tests := []struct {
		mode         DisplayMode
		wantGUI      bool
		wantTUI      bool
		wantCLI      bool
		wantHeadless bool
	}{
		{DisplayModeGUI, true, false, false, false},
		{DisplayModeTUI, false, true, false, false},
		{DisplayModeCLI, false, false, true, false},
		{DisplayModeHeadless, false, false, false, true},
	}
	for _, tt := range tests {
		t.Run(tt.mode.String(), func(t *testing.T) {
			e := DisplayEnv{Mode: tt.mode}
			if got := e.IsAutoDetectDisplayModeGUI(); got != tt.wantGUI {
				t.Errorf("IsAutoDetectDisplayModeGUI() = %v, want %v", got, tt.wantGUI)
			}
			if got := e.IsAutoDetectDisplayModeTUI(); got != tt.wantTUI {
				t.Errorf("IsAutoDetectDisplayModeTUI() = %v, want %v", got, tt.wantTUI)
			}
			if got := e.IsAutoDetectDisplayModeCLI(); got != tt.wantCLI {
				t.Errorf("IsAutoDetectDisplayModeCLI() = %v, want %v", got, tt.wantCLI)
			}
			if got := e.IsAutoDetectDisplayModeHeadless(); got != tt.wantHeadless {
				t.Errorf("IsAutoDetectDisplayModeHeadless() = %v, want %v", got, tt.wantHeadless)
			}
		})
	}
}

// --- DisplayEnv.String ---

func TestDisplayEnvString(t *testing.T) {
	e := DisplayEnv{Mode: DisplayModeTUI}
	if got := e.String(); got != "tui" {
		t.Errorf("DisplayEnv{Mode: TUI}.String() = %q, want %q", got, "tui")
	}
}

// --- DisplayEnv.SupportsColors ---

// No terminal must always be false regardless of TERM value.
func TestSupportsColorsNoTerminal(t *testing.T) {
	e := DisplayEnv{IsTerminal: false, TerminalType: "xterm-256color"}
	if e.SupportsColors() {
		t.Error("SupportsColors() = true with no terminal, want false")
	}
}

// Known color-capable TERM values must return true.
func TestSupportsColorsKnownTerminals(t *testing.T) {
	colorTerms := []string{
		"xterm-256color",
		"xterm-color",
		"screen-256color",
		"tmux-256color",
		"linux",
	}
	for _, term := range colorTerms {
		t.Run(term, func(t *testing.T) {
			e := DisplayEnv{IsTerminal: true, TerminalType: term}
			if !e.SupportsColors() {
				t.Errorf("SupportsColors() = false for TERM=%q, want true", term)
			}
		})
	}
}

// Plain "dumb" or unknown TERM with a terminal must return false.
func TestSupportsColorsUnknownTerm(t *testing.T) {
	terms := []string{"dumb", "vt100", "ansi"}
	for _, term := range terms {
		t.Run(term, func(t *testing.T) {
			e := DisplayEnv{IsTerminal: true, TerminalType: term}
			if e.SupportsColors() {
				t.Errorf("SupportsColors() = true for TERM=%q, want false", term)
			}
		})
	}
}

// Empty TERM with a terminal must return false.
func TestSupportsColorsEmptyTerm(t *testing.T) {
	e := DisplayEnv{IsTerminal: true, TerminalType: ""}
	if e.SupportsColors() {
		t.Error("SupportsColors() = true for empty TERM, want false")
	}
}

// --- DisplayEnv.SupportsUnicode ---

// LANG=en_US.UTF-8 must return true.
func TestSupportsUnicodeLangUTF8(t *testing.T) {
	t.Setenv("LANG", "en_US.UTF-8")
	t.Setenv("LC_ALL", "")
	e := DisplayEnv{}
	if !e.SupportsUnicode() {
		t.Error("SupportsUnicode() = false for LANG=en_US.UTF-8, want true")
	}
}

// LC_ALL=en_US.UTF-8 must return true even if LANG is empty.
func TestSupportsUnicodeLCAllUTF8(t *testing.T) {
	t.Setenv("LANG", "")
	t.Setenv("LC_ALL", "en_US.UTF-8")
	e := DisplayEnv{}
	if !e.SupportsUnicode() {
		t.Error("SupportsUnicode() = false for LC_ALL=en_US.UTF-8, want true")
	}
}

// No UTF-8 in either var must return false.
func TestSupportsUnicodeNoUTF8(t *testing.T) {
	t.Setenv("LANG", "C")
	t.Setenv("LC_ALL", "C")
	e := DisplayEnv{}
	if e.SupportsUnicode() {
		t.Error("SupportsUnicode() = true for LANG=C/LC_ALL=C, want false")
	}
}

// Both vars empty must return false.
func TestSupportsUnicodeEmptyVars(t *testing.T) {
	t.Setenv("LANG", "")
	t.Setenv("LC_ALL", "")
	e := DisplayEnv{}
	if e.SupportsUnicode() {
		t.Error("SupportsUnicode() = true with no locale vars, want false")
	}
}

// Case-insensitive match: "utf-8" lowercase variant.
func TestSupportsUnicodeCaseInsensitive(t *testing.T) {
	t.Setenv("LANG", "en_US.utf-8")
	t.Setenv("LC_ALL", "")
	e := DisplayEnv{}
	if !e.SupportsUnicode() {
		t.Error("SupportsUnicode() = false for LANG=en_US.utf-8, want true (case-insensitive)")
	}
}

// --- IsRemoteSession ---

// SSH_CLIENT set must return true.
func TestIsRemoteSessionSSHClient(t *testing.T) {
	t.Setenv("SSH_CLIENT", "192.168.1.1 12345 22")
	t.Setenv("SSH_TTY", "")
	t.Setenv("SSH_CONNECTION", "")
	t.Setenv("MOSH", "")
	if !IsRemoteSession() {
		t.Error("IsRemoteSession() = false with SSH_CLIENT set, want true")
	}
}

// SSH_TTY set must return true.
func TestIsRemoteSessionSSHTTY(t *testing.T) {
	t.Setenv("SSH_CLIENT", "")
	t.Setenv("SSH_TTY", "/dev/pts/0")
	t.Setenv("SSH_CONNECTION", "")
	t.Setenv("MOSH", "")
	if !IsRemoteSession() {
		t.Error("IsRemoteSession() = false with SSH_TTY set, want true")
	}
}

// SSH_CONNECTION set must return true.
func TestIsRemoteSessionSSHConnection(t *testing.T) {
	t.Setenv("SSH_CLIENT", "")
	t.Setenv("SSH_TTY", "")
	t.Setenv("SSH_CONNECTION", "192.168.1.1 12345 192.168.1.2 22")
	t.Setenv("MOSH", "")
	if !IsRemoteSession() {
		t.Error("IsRemoteSession() = false with SSH_CONNECTION set, want true")
	}
}

// MOSH set must return true.
func TestIsRemoteSessionMosh(t *testing.T) {
	t.Setenv("SSH_CLIENT", "")
	t.Setenv("SSH_TTY", "")
	t.Setenv("SSH_CONNECTION", "")
	t.Setenv("MOSH", "1")
	if !IsRemoteSession() {
		t.Error("IsRemoteSession() = false with MOSH set, want true")
	}
}

// No remote vars must return false.
func TestIsRemoteSessionNone(t *testing.T) {
	t.Setenv("SSH_CLIENT", "")
	t.Setenv("SSH_TTY", "")
	t.Setenv("SSH_CONNECTION", "")
	t.Setenv("MOSH", "")
	if IsRemoteSession() {
		t.Error("IsRemoteSession() = true with no remote vars, want false")
	}
}

// --- DetectDisplayEnv (smoke + no-panic) ---

// DetectDisplayEnv must not panic in any detectable environment.
func TestDetectDisplayEnvNoPanic(t *testing.T) {
	defer func() {
		if r := recover(); r != nil {
			t.Errorf("DetectDisplayEnv() panicked: %v", r)
		}
	}()
	_ = DetectDisplayEnv()
}

// DetectDisplayEnv must return a valid DisplayMode string.
func TestDetectDisplayEnvModeString(t *testing.T) {
	e := DetectDisplayEnv()
	valid := map[string]bool{"headless": true, "cli": true, "tui": true, "gui": true}
	if !valid[e.Mode.String()] {
		t.Errorf("DetectDisplayEnv().Mode.String() = %q, want one of headless/cli/tui/gui", e.Mode.String())
	}
}

// DetectDisplayEnv in a non-SSH test environment (clear SSH vars) must not set IsSSH.
func TestDetectDisplayEnvSSHDetection(t *testing.T) {
	t.Setenv("SSH_CLIENT", "")
	t.Setenv("SSH_TTY", "")
	e := DetectDisplayEnv()
	if e.IsSSH {
		t.Error("DetectDisplayEnv().IsSSH = true with SSH vars cleared, want false")
	}
}

// DetectDisplayEnv picks up TERM from environment.
func TestDetectDisplayEnvTermType(t *testing.T) {
	t.Setenv("TERM", "xterm-256color")
	e := DetectDisplayEnv()
	if e.TerminalType != "xterm-256color" {
		t.Errorf("DetectDisplayEnv().TerminalType = %q, want %q", e.TerminalType, "xterm-256color")
	}
}

// DetectDisplayEnv sets IsScreen when TMUX is set.
func TestDetectDisplayEnvTmux(t *testing.T) {
	t.Setenv("TMUX", "/tmp/tmux-1000/default,1234,0")
	t.Setenv("STY", "")
	e := DetectDisplayEnv()
	if !e.IsScreen {
		t.Error("DetectDisplayEnv().IsScreen = false with TMUX set, want true")
	}
}

// DetectDisplayEnv sets IsScreen when STY (GNU screen) is set.
func TestDetectDisplayEnvScreen(t *testing.T) {
	t.Setenv("STY", "1234.pts-0.hostname")
	t.Setenv("TMUX", "")
	e := DetectDisplayEnv()
	if !e.IsScreen {
		t.Error("DetectDisplayEnv().IsScreen = false with STY set, want true")
	}
}

// DetectDisplayEnv returns IsScreen=false when neither TMUX nor STY is set.
func TestDetectDisplayEnvNoScreen(t *testing.T) {
	t.Setenv("TMUX", "")
	t.Setenv("STY", "")
	e := DetectDisplayEnv()
	if e.IsScreen {
		t.Error("DetectDisplayEnv().IsScreen = true with no multiplexer vars, want false")
	}
}

// DetectDisplayEnv in a headless environment (no TTY, no display) must produce Headless mode.
func TestDetectDisplayEnvHeadlessMode(t *testing.T) {
	// Clear all display-related env vars that could trigger GUI/TUI paths.
	t.Setenv("DISPLAY", "")
	t.Setenv("WAYLAND_DISPLAY", "")
	t.Setenv("TERM_PROGRAM", "")
	t.Setenv("Apple_PubSub_Socket_Render", "")
	t.Setenv("SSH_CLIENT", "")
	t.Setenv("SSH_TTY", "")

	e := DetectDisplayEnv()
	// In a test runner (non-TTY pipe), IsTerminal is false.
	// If IsTerminal is also false and HasDisplay is false, mode must be Headless.
	if !e.IsTerminal && !e.HasDisplay {
		if e.Mode != DisplayModeHeadless {
			t.Errorf("Mode = %s with no TTY and no display, want headless", e.Mode)
		}
	}
}

// --- Regression: IsMosh detection via TERM containing "mosh" ---

func TestDetectDisplayEnvMoshViaTerm(t *testing.T) {
	t.Setenv("MOSH", "")
	t.Setenv("TERM", "mosh-custom")
	e := DetectDisplayEnv()
	if !e.IsMosh {
		t.Error("DetectDisplayEnv().IsMosh = false for TERM containing 'mosh', want true")
	}
}

// Unset MOSH and non-mosh TERM must not set IsMosh.
func TestDetectDisplayEnvNoMosh(t *testing.T) {
	t.Setenv("MOSH", "")
	t.Setenv("TERM", "xterm-256color")
	e := DetectDisplayEnv()
	if e.IsMosh {
		t.Error("DetectDisplayEnv().IsMosh = true with no mosh vars, want false")
	}
}

// --- Cols and Rows are zero when not a terminal ---

func TestDetectDisplayEnvColsRowsNonTTY(t *testing.T) {
	e := DetectDisplayEnv()
	// os.Stdout in a test pipeline is not a TTY so IsTerminal=false.
	if !e.IsTerminal {
		if e.Cols != 0 || e.Rows != 0 {
			t.Errorf("Cols=%d Rows=%d with no TTY, want 0/0", e.Cols, e.Rows)
		}
	}
}

// --- DisplayEnv.SupportsColors on env (not mode) ---

// Ensure SupportsUnicode reads live env vars (not cached).
func TestSupportsUnicodeLiveLookup(t *testing.T) {
	e := DisplayEnv{}

	t.Setenv("LANG", "en_US.UTF-8")
	t.Setenv("LC_ALL", "")
	if !e.SupportsUnicode() {
		t.Error("SupportsUnicode() = false after setting LANG=en_US.UTF-8")
	}

	os.Unsetenv("LANG")
	t.Setenv("LANG", "C")
	if e.SupportsUnicode() {
		t.Error("SupportsUnicode() = true after changing LANG to C")
	}
}
