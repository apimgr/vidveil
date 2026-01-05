// SPDX-License-Identifier: MIT
// AI.md PART 36: CLI Client - TUI Styles from Palette
package tui

import (
	"github.com/apimgr/vidveil/src/common/theme"
	"github.com/charmbracelet/lipgloss"
)

// Styles holds the TUI styles derived from the theme palette
type Styles struct {
	Base     lipgloss.Style
	Title    lipgloss.Style
	Selected lipgloss.Style
	Error    lipgloss.Style
	Success  lipgloss.Style
	Warning  lipgloss.Style
	Muted    lipgloss.Style
	Border   lipgloss.Style
}

// StylesFromPalette creates TUI styles from a theme palette
func StylesFromPalette(p theme.Palette) Styles {
	return Styles{
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

// DefaultStyles returns the default dark theme styles
func DefaultStyles() Styles {
	return StylesFromPalette(theme.Dark)
}

// LightStyles returns light theme styles
func LightStyles() Styles {
	return StylesFromPalette(theme.Light)
}
