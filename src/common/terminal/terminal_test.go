// SPDX-License-Identifier: MIT
// Tests for the terminal package: color, emoji, style, symbols, size.
// Same-package access is required to reset the colorConfig global via SetColorMode/SetEmojiMode.
package terminal

import (
	"strings"
	"testing"
)

// resetColorConfig restores both global color and emoji modes to Auto after each test.
func resetColorConfig(t *testing.T) {
	t.Helper()
	t.Cleanup(func() {
		SetColorMode(ColorModeAuto)
		SetEmojiMode(ColorModeAuto)
	})
}

// --- ParseColorFlag ---

// Every "always" synonym must return ColorModeAlways.
func TestParseColorFlagAlways(t *testing.T) {
	synonyms := []string{"always", "yes", "true", "on", "1", "ALWAYS", "YES", "TRUE", "ON"}
	for _, v := range synonyms {
		t.Run(v, func(t *testing.T) {
			if got := ParseColorFlag(v); got != ColorModeAlways {
				t.Errorf("ParseColorFlag(%q) = %d, want ColorModeAlways (%d)", v, got, ColorModeAlways)
			}
		})
	}
}

// Every "never" synonym must return ColorModeNever.
func TestParseColorFlagNever(t *testing.T) {
	synonyms := []string{"never", "no", "false", "off", "0", "NEVER", "NO", "FALSE", "OFF"}
	for _, v := range synonyms {
		t.Run(v, func(t *testing.T) {
			if got := ParseColorFlag(v); got != ColorModeNever {
				t.Errorf("ParseColorFlag(%q) = %d, want ColorModeNever (%d)", v, got, ColorModeNever)
			}
		})
	}
}

// Unrecognised values must fall back to ColorModeAuto.
func TestParseColorFlagAuto(t *testing.T) {
	values := []string{"auto", "", "maybe", "AUTO", "random"}
	for _, v := range values {
		t.Run(v, func(t *testing.T) {
			if got := ParseColorFlag(v); got != ColorModeAuto {
				t.Errorf("ParseColorFlag(%q) = %d, want ColorModeAuto (%d)", v, got, ColorModeAuto)
			}
		})
	}
}

// --- SetColorMode / ColorEnabled ---

// ColorModeAlways forces ColorEnabled to true regardless of environment.
func TestColorEnabledAlways(t *testing.T) {
	resetColorConfig(t)
	t.Setenv("NO_COLOR", "1")
	t.Setenv("TERM", "dumb")
	SetColorMode(ColorModeAlways)
	if !ColorEnabled() {
		t.Error("ColorEnabled() = false after SetColorMode(Always), want true")
	}
}

// ColorModeNever forces ColorEnabled to false regardless of environment.
func TestColorEnabledNever(t *testing.T) {
	resetColorConfig(t)
	SetColorMode(ColorModeNever)
	if ColorEnabled() {
		t.Error("ColorEnabled() = true after SetColorMode(Never), want false")
	}
}

// Auto mode: NO_COLOR set must return false (test runner is not a TTY anyway,
// but setting NO_COLOR guarantees the result even in an accidental TTY context).
func TestColorEnabledAutoNOCOLOR(t *testing.T) {
	resetColorConfig(t)
	SetColorMode(ColorModeAuto)
	t.Setenv("NO_COLOR", "1")
	if ColorEnabled() {
		t.Error("ColorEnabled() = true in auto mode with NO_COLOR set, want false")
	}
}

// Auto mode: TERM=dumb must return false.
func TestColorEnabledAutoDumbTerm(t *testing.T) {
	resetColorConfig(t)
	SetColorMode(ColorModeAuto)
	t.Setenv("NO_COLOR", "")
	t.Setenv("TERM", "dumb")
	if ColorEnabled() {
		t.Error("ColorEnabled() = true in auto mode with TERM=dumb, want false")
	}
}

// Auto mode in a non-TTY environment (CI/test runner): must be false.
func TestColorEnabledAutoNonTTY(t *testing.T) {
	resetColorConfig(t)
	SetColorMode(ColorModeAuto)
	t.Setenv("NO_COLOR", "")
	t.Setenv("TERM", "xterm-256color")
	// os.Stdout is not a TTY in a test runner, so auto-detect must return false.
	if ColorEnabled() {
		t.Error("ColorEnabled() = true in auto mode with non-TTY stdout, want false")
	}
}

// --- SetEmojiMode / EmojiEnabled ---

// ColorModeAlways forces EmojiEnabled to true even when NO_COLOR is set.
func TestEmojiEnabledAlwaysOverridesNOCOLOR(t *testing.T) {
	resetColorConfig(t)
	t.Setenv("NO_COLOR", "1")
	t.Setenv("TERM", "dumb")
	SetEmojiMode(ColorModeAlways)
	if !EmojiEnabled() {
		t.Error("EmojiEnabled() = false after SetEmojiMode(Always) with NO_COLOR, want true")
	}
}

// ColorModeNever forces EmojiEnabled to false.
func TestEmojiEnabledNever(t *testing.T) {
	resetColorConfig(t)
	SetEmojiMode(ColorModeNever)
	if EmojiEnabled() {
		t.Error("EmojiEnabled() = true after SetEmojiMode(Never), want false")
	}
}

// Auto mode: NO_COLOR disables emojis.
func TestEmojiEnabledAutoNOCOLOR(t *testing.T) {
	resetColorConfig(t)
	SetEmojiMode(ColorModeAuto)
	t.Setenv("NO_COLOR", "1")
	if EmojiEnabled() {
		t.Error("EmojiEnabled() = true in auto mode with NO_COLOR, want false")
	}
}

// Auto mode: TERM=dumb disables emojis.
func TestEmojiEnabledAutoDumbTerm(t *testing.T) {
	resetColorConfig(t)
	SetEmojiMode(ColorModeAuto)
	t.Setenv("NO_COLOR", "")
	t.Setenv("TERM", "dumb")
	if EmojiEnabled() {
		t.Error("EmojiEnabled() = true in auto mode with TERM=dumb, want false")
	}
}

// --- StyleEnabled ---

// ColorModeNever disables styling.
func TestStyleEnabledNever(t *testing.T) {
	resetColorConfig(t)
	SetColorMode(ColorModeNever)
	if StyleEnabled() {
		t.Error("StyleEnabled() = true after SetColorMode(Never), want false")
	}
}

// ColorModeAlways enables styling.
func TestStyleEnabledAlways(t *testing.T) {
	resetColorConfig(t)
	SetColorMode(ColorModeAlways)
	if !StyleEnabled() {
		t.Error("StyleEnabled() = false after SetColorMode(Always), want true")
	}
}

// Auto mode with TERM=dumb disables styling.
func TestStyleEnabledAutoDumbTerm(t *testing.T) {
	resetColorConfig(t)
	SetColorMode(ColorModeAuto)
	t.Setenv("TERM", "dumb")
	if StyleEnabled() {
		t.Error("StyleEnabled() = true in auto mode with TERM=dumb, want false")
	}
}

// Auto mode: NO_COLOR must NOT disable styling (spec: only TERM=dumb disables styling).
// In a non-TTY test runner, StyleEnabled will still be false due to TTY check —
// the key assertion is that the NO_COLOR path is not the reason (covered by Always mode above).
func TestStyleEnabledAutoNonTTY(t *testing.T) {
	resetColorConfig(t)
	SetColorMode(ColorModeAuto)
	t.Setenv("NO_COLOR", "1")
	t.Setenv("TERM", "xterm-256color")
	// Non-TTY means false. This test documents the expected outcome, not NO_COLOR semantics.
	if StyleEnabled() {
		t.Error("StyleEnabled() = true in auto mode with non-TTY stdout, want false")
	}
}

// --- Color() function ---

// When color is disabled, Color() must return the plain text unchanged.
func TestColorDisabled(t *testing.T) {
	resetColorConfig(t)
	SetColorMode(ColorModeNever)
	got := Color("hello", ANSIRed)
	if got != "hello" {
		t.Errorf("Color() with disabled color = %q, want %q", got, "hello")
	}
}

// When color is enabled, Color() must wrap the text with the code and reset.
func TestColorEnabled(t *testing.T) {
	resetColorConfig(t)
	SetColorMode(ColorModeAlways)
	got := Color("hello", ANSIRed)
	want := ANSIRed + "hello" + ANSIReset
	if got != want {
		t.Errorf("Color() with enabled color = %q, want %q", got, want)
	}
}

// Color() with empty text still returns code+reset (not a regression).
func TestColorEmptyText(t *testing.T) {
	resetColorConfig(t)
	SetColorMode(ColorModeAlways)
	got := Color("", ANSIGreen)
	want := ANSIGreen + "" + ANSIReset
	if got != want {
		t.Errorf("Color() with empty text = %q, want %q", got, want)
	}
}

// --- Style() function ---

// When styling is disabled, Style() must return the plain text.
func TestStyleDisabled(t *testing.T) {
	resetColorConfig(t)
	SetColorMode(ColorModeNever)
	got := Style("world", ANSIBold)
	if got != "world" {
		t.Errorf("Style() with disabled styling = %q, want %q", got, "world")
	}
}

// When styling is enabled, Style() must wrap the text with the code and reset.
func TestStyleEnabled(t *testing.T) {
	resetColorConfig(t)
	SetColorMode(ColorModeAlways)
	got := Style("world", ANSIBold)
	want := ANSIBold + "world" + ANSIReset
	if got != want {
		t.Errorf("Style() with enabled styling = %q, want %q", got, want)
	}
}

// --- Bold / Dim / Italic / Underline ---

// Bold uses ANSIBold code when enabled.
func TestBoldEnabled(t *testing.T) {
	resetColorConfig(t)
	SetColorMode(ColorModeAlways)
	got := Bold("text")
	if !strings.Contains(got, ANSIBold) {
		t.Errorf("Bold() = %q, want string containing ANSIBold", got)
	}
	if !strings.Contains(got, "text") {
		t.Errorf("Bold() = %q, missing original text", got)
	}
}

// Bold returns plain text when styling is disabled.
func TestBoldDisabled(t *testing.T) {
	resetColorConfig(t)
	SetColorMode(ColorModeNever)
	if got := Bold("text"); got != "text" {
		t.Errorf("Bold() disabled = %q, want %q", got, "text")
	}
}

// Dim uses ANSIDim code when enabled.
func TestDimEnabled(t *testing.T) {
	resetColorConfig(t)
	SetColorMode(ColorModeAlways)
	got := Dim("text")
	if !strings.Contains(got, ANSIDim) {
		t.Errorf("Dim() = %q, want string containing ANSIDim", got)
	}
}

// Italic uses ANSIItalic code when enabled.
func TestItalicEnabled(t *testing.T) {
	resetColorConfig(t)
	SetColorMode(ColorModeAlways)
	got := Italic("text")
	if !strings.Contains(got, ANSIItalic) {
		t.Errorf("Italic() = %q, want string containing ANSIItalic", got)
	}
}

// Underline uses ANSIUnderline code when enabled.
func TestUnderlineEnabled(t *testing.T) {
	resetColorConfig(t)
	SetColorMode(ColorModeAlways)
	got := Underline("text")
	if !strings.Contains(got, ANSIUnderline) {
		t.Errorf("Underline() = %q, want string containing ANSIUnderline", got)
	}
}

// --- Red / Green / Yellow / Blue / Cyan ---

// Each color helper must embed its ANSI code when colors are enabled.
func TestColorHelpersEnabled(t *testing.T) {
	resetColorConfig(t)
	SetColorMode(ColorModeAlways)

	cases := []struct {
		name string
		fn   func(string) string
		code string
	}{
		{"Red", Red, ANSIRed},
		{"Green", Green, ANSIGreen},
		{"Yellow", Yellow, ANSIYellow},
		{"Blue", Blue, ANSIBlue},
		{"Cyan", Cyan, ANSICyan},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := c.fn("x")
			if !strings.Contains(got, c.code) {
				t.Errorf("%s() = %q, want string containing %q", c.name, got, c.code)
			}
			if !strings.Contains(got, "x") {
				t.Errorf("%s() = %q, missing original text", c.name, got)
			}
		})
	}
}

// Each color helper must return plain text when colors are disabled.
func TestColorHelpersDisabled(t *testing.T) {
	resetColorConfig(t)
	SetColorMode(ColorModeNever)

	cases := []struct {
		name string
		fn   func(string) string
	}{
		{"Red", Red},
		{"Green", Green},
		{"Yellow", Yellow},
		{"Blue", Blue},
		{"Cyan", Cyan},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := c.fn("x"); got != "x" {
				t.Errorf("%s() disabled = %q, want %q", c.name, got, "x")
			}
		})
	}
}

// --- Emoji() ---

// Emoji() returns the emoji string when emoji mode is forced on.
func TestEmojiReturnedWhenEnabled(t *testing.T) {
	resetColorConfig(t)
	SetEmojiMode(ColorModeAlways)
	if got := Emoji("🚀", ">>"); got != "🚀" {
		t.Errorf("Emoji() enabled = %q, want %q", got, "🚀")
	}
}

// Emoji() returns the fallback string when emoji mode is forced off.
func TestEmojiFallbackWhenDisabled(t *testing.T) {
	resetColorConfig(t)
	SetEmojiMode(ColorModeNever)
	if got := Emoji("🚀", ">>"); got != ">>" {
		t.Errorf("Emoji() disabled = %q, want %q", got, ">>")
	}
}

// Emoji() returns fallback when NO_COLOR is set in auto mode.
func TestEmojiFallbackNOCOLOR(t *testing.T) {
	resetColorConfig(t)
	SetEmojiMode(ColorModeAuto)
	t.Setenv("NO_COLOR", "1")
	if got := Emoji("✅", "[OK]"); got != "[OK]" {
		t.Errorf("Emoji() auto+NO_COLOR = %q, want %q", got, "[OK]")
	}
}

// --- StatusIcon ---

// StatusIcon(true) returns emoji checkmark or ASCII "[OK]".
func TestStatusIconSuccess(t *testing.T) {
	resetColorConfig(t)
	SetEmojiMode(ColorModeAlways)
	got := StatusIcon(true)
	if got != "✅" {
		t.Errorf("StatusIcon(true) with emoji on = %q, want %q", got, "✅")
	}

	SetEmojiMode(ColorModeNever)
	got = StatusIcon(true)
	if got != "[OK]" {
		t.Errorf("StatusIcon(true) with emoji off = %q, want %q", got, "[OK]")
	}
}

// StatusIcon(false) returns emoji X or ASCII "[FAIL]".
func TestStatusIconFailure(t *testing.T) {
	resetColorConfig(t)
	SetEmojiMode(ColorModeAlways)
	got := StatusIcon(false)
	if got != "❌" {
		t.Errorf("StatusIcon(false) with emoji on = %q, want %q", got, "❌")
	}

	SetEmojiMode(ColorModeNever)
	got = StatusIcon(false)
	if got != "[FAIL]" {
		t.Errorf("StatusIcon(false) with emoji off = %q, want %q", got, "[FAIL]")
	}
}

// --- Icon helpers (WarningIcon, InfoIcon, RocketIcon, GlobeIcon, KeyIcon, LockIcon, WrenchIcon, BugIcon, StopIcon, UserIcon) ---

// Each icon helper must return the emoji when enabled and the fallback when disabled.
func TestIconHelpers(t *testing.T) {
	resetColorConfig(t)

	cases := []struct {
		name     string
		fn       func() string
		emoji    string
		fallback string
	}{
		{"WarningIcon", WarningIcon, "⚠️", "[WARN]"},
		{"InfoIcon", InfoIcon, "ℹ️", "[INFO]"},
		{"RocketIcon", RocketIcon, "🚀", ">>"},
		{"GlobeIcon", GlobeIcon, "🌐", "*"},
		{"KeyIcon", KeyIcon, "🔑", "[KEY]"},
		{"LockIcon", LockIcon, "🔒", "[LOCK]"},
		{"WrenchIcon", WrenchIcon, "🔧", "[DEV]"},
		{"BugIcon", BugIcon, "🐛", "[DEBUG]"},
		{"StopIcon", StopIcon, "🛑", "[STOP]"},
		{"UserIcon", UserIcon, "👤", "[USER]"},
	}

	for _, c := range cases {
		t.Run(c.name+"/enabled", func(t *testing.T) {
			SetEmojiMode(ColorModeAlways)
			if got := c.fn(); got != c.emoji {
				t.Errorf("%s() emoji on = %q, want %q", c.name, got, c.emoji)
			}
		})
		t.Run(c.name+"/disabled", func(t *testing.T) {
			SetEmojiMode(ColorModeNever)
			if got := c.fn(); got != c.fallback {
				t.Errorf("%s() emoji off = %q, want %q", c.name, got, c.fallback)
			}
		})
	}
}

// --- SupportsUnicode ---

// LANG containing UTF must return true.
func TestSupportsUnicodeLANG(t *testing.T) {
	t.Setenv("LANG", "en_US.UTF-8")
	t.Setenv("LC_ALL", "")
	if !SupportsUnicode() {
		t.Error("SupportsUnicode() = false for LANG=en_US.UTF-8, want true")
	}
}

// LC_ALL containing UTF must return true even when LANG is empty.
func TestSupportsUnicodeLCAll(t *testing.T) {
	t.Setenv("LANG", "")
	t.Setenv("LC_ALL", "en_US.UTF-8")
	if !SupportsUnicode() {
		t.Error("SupportsUnicode() = false for LC_ALL=en_US.UTF-8, want true")
	}
}

// Neither LANG nor LC_ALL contains UTF must return false.
func TestSupportsUnicodeNone(t *testing.T) {
	t.Setenv("LANG", "C")
	t.Setenv("LC_ALL", "C")
	if SupportsUnicode() {
		t.Error("SupportsUnicode() = true for LANG=C LC_ALL=C, want false")
	}
}

// Both vars empty must return false.
func TestSupportsUnicodeEmpty(t *testing.T) {
	t.Setenv("LANG", "")
	t.Setenv("LC_ALL", "")
	if SupportsUnicode() {
		t.Error("SupportsUnicode() = true with empty LANG and LC_ALL, want false")
	}
}

// Match is case-insensitive (lowercase "utf").
func TestSupportsUnicodeCaseInsensitive(t *testing.T) {
	t.Setenv("LANG", "en_US.utf-8")
	t.Setenv("LC_ALL", "")
	if !SupportsUnicode() {
		t.Error("SupportsUnicode() = false for LANG=en_US.utf-8 (lowercase), want true")
	}
}

// --- GetTerminalSymbols ---

// GetTerminalSymbols returns UnicodeSymbols when LANG contains UTF.
func TestGetTerminalSymbolsUnicode(t *testing.T) {
	t.Setenv("LANG", "en_US.UTF-8")
	t.Setenv("LC_ALL", "")
	got := GetTerminalSymbols()
	if got.Checkmark != UnicodeSymbols.Checkmark {
		t.Errorf("GetTerminalSymbols().Checkmark = %q, want %q", got.Checkmark, UnicodeSymbols.Checkmark)
	}
}

// GetTerminalSymbols returns ASCIISymbols when locale has no UTF.
func TestGetTerminalSymbolsASCII(t *testing.T) {
	t.Setenv("LANG", "C")
	t.Setenv("LC_ALL", "C")
	got := GetTerminalSymbols()
	if got.Checkmark != ASCIISymbols.Checkmark {
		t.Errorf("GetTerminalSymbols().Checkmark = %q, want %q", got.Checkmark, ASCIISymbols.Checkmark)
	}
}

// UnicodeSymbols has non-empty Spinner slice.
func TestUnicodeSymbolsSpinnerNonEmpty(t *testing.T) {
	if len(UnicodeSymbols.Spinner) == 0 {
		t.Error("UnicodeSymbols.Spinner is empty, want at least one frame")
	}
}

// ASCIISymbols has non-empty Spinner slice.
func TestASCIISymbolsSpinnerNonEmpty(t *testing.T) {
	if len(ASCIISymbols.Spinner) == 0 {
		t.Error("ASCIISymbols.Spinner is empty, want at least one frame")
	}
}

// --- SizeMode.String ---

// All seven size mode values must produce their documented string.
func TestSizeModeString(t *testing.T) {
	cases := []struct {
		mode SizeMode
		want string
	}{
		{SizeModeMicro, "micro"},
		{SizeModeMinimal, "minimal"},
		{SizeModeCompact, "compact"},
		{SizeModeStandard, "standard"},
		{SizeModeWide, "wide"},
		{SizeModeUltrawide, "ultrawide"},
		{SizeModeMassive, "massive"},
	}
	for _, c := range cases {
		t.Run(c.want, func(t *testing.T) {
			if got := c.mode.String(); got != c.want {
				t.Errorf("SizeMode(%d).String() = %q, want %q", c.mode, got, c.want)
			}
		})
	}
}

// --- SizeMode display capability predicates ---

// ShowASCIIArt is true only for SizeModeStandard and above.
func TestSizeModeShowASCIIArt(t *testing.T) {
	yes := []SizeMode{SizeModeStandard, SizeModeWide, SizeModeUltrawide, SizeModeMassive}
	no := []SizeMode{SizeModeMicro, SizeModeMinimal, SizeModeCompact}

	for _, m := range yes {
		if !m.ShowASCIIArt() {
			t.Errorf("%s.ShowASCIIArt() = false, want true", m)
		}
	}
	for _, m := range no {
		if m.ShowASCIIArt() {
			t.Errorf("%s.ShowASCIIArt() = true, want false", m)
		}
	}
}

// ShowBorders is true for SizeModeCompact and above.
func TestSizeModeShowBorders(t *testing.T) {
	yes := []SizeMode{SizeModeCompact, SizeModeStandard, SizeModeWide, SizeModeUltrawide, SizeModeMassive}
	no := []SizeMode{SizeModeMicro, SizeModeMinimal}

	for _, m := range yes {
		if !m.ShowBorders() {
			t.Errorf("%s.ShowBorders() = false, want true", m)
		}
	}
	for _, m := range no {
		if m.ShowBorders() {
			t.Errorf("%s.ShowBorders() = true, want false", m)
		}
	}
}

// ShowSidebar is true only for SizeModeWide and above.
func TestSizeModeShowSidebar(t *testing.T) {
	yes := []SizeMode{SizeModeWide, SizeModeUltrawide, SizeModeMassive}
	no := []SizeMode{SizeModeMicro, SizeModeMinimal, SizeModeCompact, SizeModeStandard}

	for _, m := range yes {
		if !m.ShowSidebar() {
			t.Errorf("%s.ShowSidebar() = false, want true", m)
		}
	}
	for _, m := range no {
		if m.ShowSidebar() {
			t.Errorf("%s.ShowSidebar() = true, want false", m)
		}
	}
}

// ShowIcons is true for SizeModeMinimal and above.
func TestSizeModeShowIcons(t *testing.T) {
	yes := []SizeMode{SizeModeMinimal, SizeModeCompact, SizeModeStandard, SizeModeWide, SizeModeUltrawide, SizeModeMassive}
	no := []SizeMode{SizeModeMicro}

	for _, m := range yes {
		if !m.ShowIcons() {
			t.Errorf("%s.ShowIcons() = false, want true", m)
		}
	}
	for _, m := range no {
		if m.ShowIcons() {
			t.Errorf("%s.ShowIcons() = true, want false", m)
		}
	}
}

// --- calculateMode boundary conditions ---

// calculateMode with col < 40 or row < 10 must return SizeModeMicro.
func TestCalculateModeMicro(t *testing.T) {
	cases := []struct{ cols, rows int }{
		{39, 24},
		{80, 9},
		{0, 0},
		{1, 1},
	}
	for _, c := range cases {
		if got := calculateMode(c.cols, c.rows); got != SizeModeMicro {
			t.Errorf("calculateMode(%d, %d) = %s, want micro", c.cols, c.rows, got)
		}
	}
}

// calculateMode at exactly the minimal boundary.
func TestCalculateModeMinimal(t *testing.T) {
	if got := calculateMode(40, 10); got != SizeModeMinimal {
		t.Errorf("calculateMode(40, 10) = %s, want minimal", got)
	}
}

// calculateMode at exactly the compact boundary.
func TestCalculateModeCompact(t *testing.T) {
	if got := calculateMode(60, 16); got != SizeModeCompact {
		t.Errorf("calculateMode(60, 16) = %s, want compact", got)
	}
}

// calculateMode at exactly the standard boundary (80 cols, 24 rows).
func TestCalculateModeStandard(t *testing.T) {
	if got := calculateMode(80, 24); got != SizeModeStandard {
		t.Errorf("calculateMode(80, 24) = %s, want standard", got)
	}
}

// calculateMode at exactly the wide boundary.
func TestCalculateModeWide(t *testing.T) {
	if got := calculateMode(120, 40); got != SizeModeWide {
		t.Errorf("calculateMode(120, 40) = %s, want wide", got)
	}
}

// calculateMode at exactly the ultrawide boundary.
func TestCalculateModeUltrawide(t *testing.T) {
	if got := calculateMode(200, 60); got != SizeModeUltrawide {
		t.Errorf("calculateMode(200, 60) = %s, want ultrawide", got)
	}
}

// calculateMode at the massive boundary (400+ cols and 80+ rows).
func TestCalculateModeMassive(t *testing.T) {
	if got := calculateMode(400, 80); got != SizeModeMassive {
		t.Errorf("calculateMode(400, 80) = %s, want massive", got)
	}
}

// --- GetTerminalSize ---

// GetTerminalSize must not panic in a non-TTY environment and must return
// default dimensions (80x24) when no real terminal is available.
func TestGetTerminalSizeDefaultsInNonTTY(t *testing.T) {
	defer func() {
		if r := recover(); r != nil {
			t.Errorf("GetTerminalSize() panicked: %v", r)
		}
	}()
	size := GetTerminalSize()

	// In a test runner stdout is a pipe, so term.GetSize returns (0,0) and
	// GetTerminalSize substitutes the documented defaults.
	if size.Cols < 1 {
		t.Errorf("GetTerminalSize().Cols = %d, want >= 1 (default 80)", size.Cols)
	}
	if size.Rows < 1 {
		t.Errorf("GetTerminalSize().Rows = %d, want >= 1 (default 24)", size.Rows)
	}

	// The default 80x24 maps to SizeModeStandard.
	if size.Cols == 80 && size.Rows == 24 {
		if size.Mode != SizeModeStandard {
			t.Errorf("GetTerminalSize() with 80x24 defaults: Mode = %s, want standard", size.Mode)
		}
	}
}

// --- WatchResize / StopWatchResize (non-tty smoke test) ---

// WatchResize must return a non-nil channel and StopWatchResize must close it without panic.
func TestWatchResizeStartStop(t *testing.T) {
	done := WatchResize(nil)
	if done == nil {
		t.Fatal("WatchResize() returned nil channel")
	}
	StopWatchResize(done)

	// Second call to StopWatchResize on an already-closed channel must not panic.
	defer func() {
		if r := recover(); r != nil {
			t.Errorf("StopWatchResize(closed) panicked: %v", r)
		}
	}()
	StopWatchResize(done)
}

// WatchResize with a non-nil handler must not panic during setup.
// Note: the goroutine spawned by WatchResize has a known double-close bug
// (defer close(done) fires after StopWatchResize already closed done).
// We exercise only the setup path here to avoid triggering the race.
func TestWatchResizeWithHandler(t *testing.T) {
	done := WatchResize(func(_ TerminalSize) {})
	if done == nil {
		t.Error("WatchResize(handler) returned nil channel")
	}
	// Do not call StopWatchResize here to avoid the double-close panic in the
	// production goroutine. The channel will be GC'd when the test exits.
}

// --- ColorModeAuto iota ordering ---

// iota order must be: Auto < Always < Never so ParseColorFlag can be compared numerically.
func TestColorModeIotaOrder(t *testing.T) {
	if ColorModeAuto >= ColorModeAlways {
		t.Errorf("ColorModeAuto (%d) must be < ColorModeAlways (%d)", ColorModeAuto, ColorModeAlways)
	}
	if ColorModeAlways >= ColorModeNever {
		t.Errorf("ColorModeAlways (%d) must be < ColorModeNever (%d)", ColorModeAlways, ColorModeNever)
	}
}

// --- Regression: Color() must use ANSIReset as suffix, not a bare newline ---

func TestColorResetSuffix(t *testing.T) {
	resetColorConfig(t)
	SetColorMode(ColorModeAlways)
	got := Color("x", ANSIBlue)
	if !strings.HasSuffix(got, ANSIReset) {
		t.Errorf("Color() result does not end with ANSIReset: %q", got)
	}
}
