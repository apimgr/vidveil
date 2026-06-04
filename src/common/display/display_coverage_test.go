// SPDX-License-Identifier: MIT
package display

import (
	"testing"
)

// --- DisplayMode exact integer values ---
// The existing test only checks ordering (Headless < CLI < TUI < GUI).
// These tests nail down the iota values so a reordering is caught immediately.

func TestDisplayModeExactValues(t *testing.T) {
	tests := []struct {
		mode DisplayMode
		want int
	}{
		{DisplayModeHeadless, 0},
		{DisplayModeCLI, 1},
		{DisplayModeTUI, 2},
		{DisplayModeGUI, 3},
	}
	for _, tt := range tests {
		t.Run(tt.mode.String(), func(t *testing.T) {
			if int(tt.mode) != tt.want {
				t.Errorf("DisplayMode value = %d, want %d", int(tt.mode), tt.want)
			}
		})
	}
}

// --- DisplayEnv struct construction with all fields populated ---
// Verifies the struct type accepts every field without a compile error and
// that reading them back works correctly.

func TestDisplayEnvAllFieldsConstruction(t *testing.T) {
	e := DisplayEnv{
		Mode:         DisplayModeTUI,
		HasDisplay:   true,
		DisplayType:  "x11",
		IsTerminal:   true,
		IsSSH:        false,
		IsMosh:       false,
		IsScreen:     true,
		TerminalType: "xterm-256color",
		Cols:         220,
		Rows:         50,
	}
	if e.Mode != DisplayModeTUI {
		t.Errorf("Mode = %v, want TUI", e.Mode)
	}
	if !e.HasDisplay {
		t.Error("HasDisplay = false, want true")
	}
	if e.DisplayType != "x11" {
		t.Errorf("DisplayType = %q, want x11", e.DisplayType)
	}
	if !e.IsTerminal {
		t.Error("IsTerminal = false, want true")
	}
	if e.IsSSH {
		t.Error("IsSSH = true, want false")
	}
	if e.IsMosh {
		t.Error("IsMosh = true, want false")
	}
	if !e.IsScreen {
		t.Error("IsScreen = false, want true")
	}
	if e.TerminalType != "xterm-256color" {
		t.Errorf("TerminalType = %q, want xterm-256color", e.TerminalType)
	}
	if e.Cols != 220 {
		t.Errorf("Cols = %d, want 220", e.Cols)
	}
	if e.Rows != 50 {
		t.Errorf("Rows = %d, want 50", e.Rows)
	}
}

// Zero-value struct must be valid and represent Headless mode.
func TestDisplayEnvZeroValue(t *testing.T) {
	var e DisplayEnv
	if e.Mode != DisplayModeHeadless {
		t.Errorf("zero DisplayEnv.Mode = %v, want Headless (0)", e.Mode)
	}
	if e.IsTerminal || e.IsSSH || e.IsMosh || e.IsScreen || e.HasDisplay {
		t.Error("zero DisplayEnv has unexpected true flags")
	}
	if e.Cols != 0 || e.Rows != 0 {
		t.Errorf("zero DisplayEnv Cols=%d Rows=%d, want 0/0", e.Cols, e.Rows)
	}
}

// --- DetectDisplayEnv: SSH detection via SSH_TTY (distinct from SSH_CLIENT) ---
// The existing test clears SSH_CLIENT and SSH_TTY and checks IsSSH=false.
// This test sets only SSH_TTY and verifies IsSSH=true in the returned struct.

func TestDetectDisplayEnvSSHTTYSetsIsSSH(t *testing.T) {
	t.Setenv("SSH_CLIENT", "")
	t.Setenv("SSH_TTY", "/dev/pts/1")
	e := DetectDisplayEnv()
	if !e.IsSSH {
		t.Error("DetectDisplayEnv().IsSSH = false with SSH_TTY set, want true")
	}
}

// --- DetectDisplayEnv: IsMosh via MOSH env var (not via TERM) ---
// The existing test covers the TERM=mosh-* path; this covers the MOSH= path.

func TestDetectDisplayEnvMoshEnvVar(t *testing.T) {
	t.Setenv("MOSH", "1")
	t.Setenv("TERM", "xterm-256color")
	e := DetectDisplayEnv()
	if !e.IsMosh {
		t.Error("DetectDisplayEnv().IsMosh = false with MOSH=1, want true")
	}
}

// MOSH unset and TERM without "mosh" must leave IsMosh false.
func TestDetectDisplayEnvMoshUnset(t *testing.T) {
	t.Setenv("MOSH", "")
	t.Setenv("TERM", "xterm-256color")
	e := DetectDisplayEnv()
	if e.IsMosh {
		t.Error("DetectDisplayEnv().IsMosh = true with MOSH unset, want false")
	}
}

// --- DetectDisplayEnv: IsSSH via SSH_CLIENT (independent from SSH_TTY path) ---

func TestDetectDisplayEnvSSHClientSetsIsSSH(t *testing.T) {
	t.Setenv("SSH_CLIENT", "10.0.0.1 50000 22")
	t.Setenv("SSH_TTY", "")
	e := DetectDisplayEnv()
	if !e.IsSSH {
		t.Error("DetectDisplayEnv().IsSSH = false with SSH_CLIENT set, want true")
	}
}

// Both SSH vars cleared must produce IsSSH=false.
func TestDetectDisplayEnvNoSSHClearsIsSSH(t *testing.T) {
	t.Setenv("SSH_CLIENT", "")
	t.Setenv("SSH_TTY", "")
	e := DetectDisplayEnv()
	if e.IsSSH {
		t.Error("DetectDisplayEnv().IsSSH = true with both SSH vars cleared, want false")
	}
}

// --- DetectDisplayEnv: Mode is always a valid DisplayMode ---
// Complements the existing string check with a numeric range check.

func TestDetectDisplayEnvModeInRange(t *testing.T) {
	e := DetectDisplayEnv()
	if e.Mode < DisplayModeHeadless || e.Mode > DisplayModeGUI {
		t.Errorf("DetectDisplayEnv().Mode = %d, out of valid range [0,3]", e.Mode)
	}
}

// --- autoDetectDisplayMode: no-TTY + display → GUI (HasDisplay alone drives GUI) ---
// Regression: the first if-branch checks !IsTerminal && !HasDisplay; when HasDisplay
// is true but IsTerminal is false the headless branch must be skipped.

func TestAutoDetectDisplayModeDisplayWithoutTTY(t *testing.T) {
	e := &DisplayEnv{IsTerminal: false, HasDisplay: true, TerminalType: "xterm", IsSSH: false, IsMosh: false}
	got := e.autoDetectDisplayMode()
	if got != DisplayModeGUI {
		t.Errorf("autoDetectDisplayMode() with display but no TTY = %s, want gui", got)
	}
}

// --- autoDetectDisplayMode: no-TTY + no-display + dumb-terminal → still Headless ---
// Headless check runs before the dumb-terminal check; combining them must still
// produce Headless.

func TestAutoDetectDisplayModeHeadlessDumbCombined(t *testing.T) {
	e := &DisplayEnv{IsTerminal: false, HasDisplay: false, TerminalType: "dumb"}
	got := e.autoDetectDisplayMode()
	if got != DisplayModeHeadless {
		t.Errorf("autoDetectDisplayMode() no-tty+no-display+dumb = %s, want headless", got)
	}
}

// --- autoDetectDisplayMode: SSH with no display, but has TTY → TUI ---

func TestAutoDetectDisplayModeSSHNoDisplay(t *testing.T) {
	e := &DisplayEnv{IsTerminal: true, HasDisplay: false, TerminalType: "xterm-256color", IsSSH: true}
	got := e.autoDetectDisplayMode()
	if got != DisplayModeTUI {
		t.Errorf("autoDetectDisplayMode() SSH+no-display+TTY = %s, want tui", got)
	}
}

// --- IsDumbTerminal: non-empty non-dumb TERM is false ---

func TestIsDumbTerminalVariousFalse(t *testing.T) {
	terms := []string{"xterm", "xterm-256color", "screen", "tmux", "linux", "vt100"}
	for _, term := range terms {
		t.Run(term, func(t *testing.T) {
			e := DisplayEnv{TerminalType: term}
			if e.IsDumbTerminal() {
				t.Errorf("IsDumbTerminal() = true for TERM=%q, want false", term)
			}
		})
	}
}

// --- DisplayEnv.String delegates to Mode.String ---

func TestDisplayEnvStringAllModes(t *testing.T) {
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
			e := DisplayEnv{Mode: tt.mode}
			if got := e.String(); got != tt.want {
				t.Errorf("DisplayEnv{Mode:%s}.String() = %q, want %q", tt.mode, got, tt.want)
			}
		})
	}
}

// --- IsRemoteSession: SSH_CONNECTION path (not tested in IsRemoteSession block
//     for the struct-level but here we cover the function variant too) ---

func TestIsRemoteSessionAllCombinationsCleared(t *testing.T) {
	t.Setenv("SSH_CLIENT", "")
	t.Setenv("SSH_TTY", "")
	t.Setenv("SSH_CONNECTION", "")
	t.Setenv("MOSH", "")
	if IsRemoteSession() {
		t.Error("IsRemoteSession() = true with all remote vars cleared, want false")
	}
}

// --- SupportsColors: boundary at DisplayModeCLI exactly ---
// Mode-level SupportsColors uses >= DisplayModeCLI; CLI itself must return true.

func TestDisplayModeSupportsColorsAtCLIBoundary(t *testing.T) {
	if !DisplayModeCLI.SupportsColors() {
		t.Error("DisplayModeCLI.SupportsColors() = false, want true (boundary)")
	}
}

// Headless is strictly below CLI so it must return false.
func TestDisplayModeSupportsColorsHeadlessFalse(t *testing.T) {
	if DisplayModeHeadless.SupportsColors() {
		t.Error("DisplayModeHeadless.SupportsColors() = true, want false")
	}
}

// --- SupportsInteraction: boundary at DisplayModeTUI exactly ---

func TestDisplayModeSupportsInteractionAtTUIBoundary(t *testing.T) {
	if !DisplayModeTUI.SupportsInteraction() {
		t.Error("DisplayModeTUI.SupportsInteraction() = false, want true (boundary)")
	}
}

// CLI is strictly below TUI.
func TestDisplayModeSupportsInteractionCLIFalse(t *testing.T) {
	if DisplayModeCLI.SupportsInteraction() {
		t.Error("DisplayModeCLI.SupportsInteraction() = true, want false")
	}
}

// --- DetectDisplayEnv: TerminalType reflects TERM env var ---
// Distinct from the existing test: verify empty TERM stays empty.

func TestDetectDisplayEnvEmptyTERMPreserved(t *testing.T) {
	t.Setenv("TERM", "")
	e := DetectDisplayEnv()
	if e.TerminalType != "" {
		t.Errorf("DetectDisplayEnv().TerminalType = %q with TERM='', want empty", e.TerminalType)
	}
}

// --- autoDetectDisplayMode: no-TTY + HasDisplay + IsSSH → CLI fallback ---
// This path reaches the final "return DisplayModeCLI" because:
//   !IsTerminal && !HasDisplay = false (HasDisplay is true) → not headless
//   TerminalType != "dumb"
//   HasDisplay && !IsSSH = true && false = false → not GUI
//   IsTerminal = false → not TUI
//   → CLI fallback
func TestAutoDetectDisplayModeCLIFallback(t *testing.T) {
	e := &DisplayEnv{IsTerminal: false, HasDisplay: true, TerminalType: "xterm-256color", IsSSH: true}
	got := e.autoDetectDisplayMode()
	if got != DisplayModeCLI {
		t.Errorf("autoDetectDisplayMode() no-tty+display+SSH = %s, want cli", got)
	}
}

// --- DisplayEnv.SupportsColors (env-level method) vs DisplayMode.SupportsColors ---
// The env-level method applies additional terminal-name heuristics and also
// requires IsTerminal=true. This is separate from the mode-level method.

func TestDisplayEnvSupportsColorsRequiresIsTerminal(t *testing.T) {
	e := DisplayEnv{IsTerminal: false, TerminalType: "xterm-256color"}
	if e.SupportsColors() {
		t.Error("DisplayEnv.SupportsColors() = true with IsTerminal=false, want false")
	}
}

func TestDisplayEnvSupportsColorsTrueWithColorTerm(t *testing.T) {
	e := DisplayEnv{IsTerminal: true, TerminalType: "xterm-256color"}
	if !e.SupportsColors() {
		t.Error("DisplayEnv.SupportsColors() = false for xterm-256color with terminal, want true")
	}
}

// --- detectUnixDisplay: Wayland path via WAYLAND_DISPLAY env var ---
// When WAYLAND_DISPLAY is set, detectUnixDisplay must set DisplayType="wayland"
// and HasDisplay=true without calling any external binary.

func TestDetectUnixDisplay_WaylandDisplaySet(t *testing.T) {
	t.Setenv("WAYLAND_DISPLAY", "wayland-0")
	t.Setenv("DISPLAY", "")

	e := &DisplayEnv{}
	e.detectUnixDisplay()

	if e.DisplayType != "wayland" {
		t.Errorf("detectUnixDisplay() DisplayType = %q, want wayland", e.DisplayType)
	}
	if !e.HasDisplay {
		t.Error("detectUnixDisplay() HasDisplay = false, want true")
	}
}

// --- detectUnixDisplay: no-display path (no env vars set) ---
// When neither WAYLAND_DISPLAY nor DISPLAY is set, DisplayType must be "none"
// and HasDisplay must be false.

func TestDetectUnixDisplay_NoDisplayVars(t *testing.T) {
	t.Setenv("WAYLAND_DISPLAY", "")
	t.Setenv("DISPLAY", "")

	e := &DisplayEnv{}
	e.detectUnixDisplay()

	if e.DisplayType != "none" {
		t.Errorf("detectUnixDisplay() no-vars: DisplayType = %q, want none", e.DisplayType)
	}
	if e.HasDisplay {
		t.Error("detectUnixDisplay() no-vars: HasDisplay = true, want false")
	}
}

// --- detectUnixDisplay: DISPLAY set but xset unavailable → "none" ---
// When DISPLAY is set but the X server (xset) is not accessible, fall through
// to DisplayType="none".

func TestDetectUnixDisplay_DisplaySetXsetFails(t *testing.T) {
	t.Setenv("WAYLAND_DISPLAY", "")
	t.Setenv("DISPLAY", ":99")

	e := &DisplayEnv{}
	e.detectUnixDisplay()

	// In a headless CI container xset will fail, so we expect none.
	// If somehow xset succeeds (real display), accept "x11" too.
	if e.DisplayType != "none" && e.DisplayType != "x11" {
		t.Errorf("detectUnixDisplay() DISPLAY set: DisplayType = %q, want none or x11", e.DisplayType)
	}
}
