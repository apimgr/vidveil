// SPDX-License-Identifier: MIT
// AI.md PART 32: CLI Client - TUI TUIStyles from Palette
package tui

import (
	"github.com/apimgr/vidveil/src/common/theme"
	"github.com/charmbracelet/lipgloss"
)

// TUIStyles holds the TUI styles derived from the theme palette
type TUIStyles struct {
	Base     lipgloss.Style
	Title    lipgloss.Style
	Selected lipgloss.Style
	Error    lipgloss.Style
	Success  lipgloss.Style
	Warning  lipgloss.Style
	Muted    lipgloss.Style
	Border   lipgloss.Style
}

// StylesFromTerminalPalette creates TUI styles from the ANSI-mapped
// TerminalPalette — see AI.md PART 16 "CLI/TUI Color Mapping". This is the
// REQUIRED baseline for CLI/TUI output; TUIStylesFromPalette (literal hex)
// is an opt-in enhancement only, never the default.
func StylesFromTerminalPalette(p theme.TerminalPalette) TUIStyles {
	return TUIStyles{
		Base: lipgloss.NewStyle().
			Foreground(lipgloss.Color(p.Foreground)),
		Title: lipgloss.NewStyle().
			Foreground(lipgloss.Color(p.Primary)).Bold(true),
		Selected: lipgloss.NewStyle().Reverse(true),
		Error:    lipgloss.NewStyle().Foreground(lipgloss.Color(p.Error)),
		Success:  lipgloss.NewStyle().Foreground(lipgloss.Color(p.Success)),
		Warning:  lipgloss.NewStyle().Foreground(lipgloss.Color(p.Warning)),
		Muted:    lipgloss.NewStyle().Foreground(lipgloss.Color(p.Muted)),
		Border:   lipgloss.NewStyle().BorderForeground(lipgloss.Color(p.Border)),
	}
}

// TUIStylesFromPalette creates TUI styles from the literal hex ColorPalette.
// Opt-in enhancement only for terminals that report true-color support
// (theme.SupportsTrueColor) — never used as the CLI/TUI baseline.
func TUIStylesFromPalette(p theme.ColorPalette) TUIStyles {
	return TUIStyles{
		Base: lipgloss.NewStyle().
			Foreground(lipgloss.Color(p.Foreground)).
			Background(lipgloss.Color(p.Background)),
		Title: lipgloss.NewStyle().
			Foreground(lipgloss.Color(p.Primary)).Bold(true),
		Selected: lipgloss.NewStyle().
			Foreground(lipgloss.Color(p.Background)).
			Background(lipgloss.Color(p.Primary)),
		Error:   lipgloss.NewStyle().Foreground(lipgloss.Color(p.Error)),
		Success: lipgloss.NewStyle().Foreground(lipgloss.Color(p.Success)),
		Warning: lipgloss.NewStyle().Foreground(lipgloss.Color(p.Warning)),
		Muted:   lipgloss.NewStyle().Foreground(lipgloss.Color(p.Muted)),
		Border:  lipgloss.NewStyle().BorderForeground(lipgloss.Color(p.Border)),
	}
}

// DefaultTUIStyles returns the default dark theme styles — ANSI baseline
// unless the terminal reports true-color support, in which case the
// literal hex palette is used as an opt-in enhancement.
func DefaultTUIStyles() TUIStyles {
	if theme.SupportsTrueColor() {
		return TUIStylesFromPalette(theme.Dark)
	}
	return StylesFromTerminalPalette(theme.TerminalPaletteDark)
}

// LightTUIStyles returns light theme styles — ANSI baseline unless the
// terminal reports true-color support, in which case the literal hex
// palette is used as an opt-in enhancement.
func LightTUIStyles() TUIStyles {
	if theme.SupportsTrueColor() {
		return TUIStylesFromPalette(theme.Light)
	}
	return StylesFromTerminalPalette(theme.TerminalPaletteLight)
}
