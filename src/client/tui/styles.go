// SPDX-License-Identifier: MIT
// AI.md PART 33: CLI Client - TUI TUIStyles from Palette
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

// TUIStylesFromPalette creates TUI styles from a theme palette
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

// DefaultTUIStyles returns the default dark theme styles
func DefaultTUIStyles() TUIStyles {
	return TUIStylesFromPalette(theme.Dark)
}

// LightTUIStyles returns light theme styles
func LightTUIStyles() TUIStyles {
	return TUIStylesFromPalette(theme.Light)
}
