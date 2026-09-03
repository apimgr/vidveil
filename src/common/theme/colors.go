// SPDX-License-Identifier: MIT
// Package theme provides unified theming
// See AI.md PART 7 for specification
package theme

// ColorPalette defines the color scheme for the application
type ColorPalette struct {
	Background, Foreground             string
	Primary, Secondary, Accent         string
	Success, Warning, Error, Info      string
	Surface, SurfaceAlt, Border, Muted string
}

var (
	// Dark is the default dark color palette (Dracula inspired)
	// Values match AI.md PART 16 "Themes" ThemePaletteDark exactly.
	Dark = ColorPalette{
		Background: "#282a36", Foreground: "#f8f8f2",
		Primary: "#bd93f9", Secondary: "#50fa7b", Accent: "#ff79c6",
		Success: "#50fa7b", Warning: "#ffb86c", Error: "#ff5555", Info: "#8be9fd",
		Surface: "#2b2d3a", SurfaceAlt: "#21222c", Border: "#44475a", Muted: "#6272a4",
	}

	// Light is the light color palette (GitHub Light inspired)
	// Values match AI.md PART 16 "Themes" ThemePaletteLight exactly.
	Light = ColorPalette{
		Background: "#ffffff", Foreground: "#1f2328",
		Primary: "#0969da", Secondary: "#1a7f37", Accent: "#8250df",
		Success: "#1a7f37", Warning: "#9a6700", Error: "#d1242f", Info: "#0969da",
		Surface: "#f6f8fa", SurfaceAlt: "#eff2f5", Border: "#d1d9e0", Muted: "#59636e",
	}
)

// GetColorPalette returns the appropriate palette based on mode
// Supported modes: "dark", "light", "auto"
func GetColorPalette(mode string) ColorPalette {
	switch mode {
	case "light":
		return Light
	case "auto":
		if DetectSystemDark() {
			return Dark
		}
		return Light
	default:
		return Dark
	}
}

// GetColorPaletteName returns the name of the palette based on mode
func GetColorPaletteName(mode string) string {
	switch mode {
	case "light":
		return "light"
	case "auto":
		if DetectSystemDark() {
			return "dark"
		}
		return "light"
	default:
		return "dark"
	}
}

// TerminalPalette holds ANSI 16-color indices (0-15) for CLI/TUI — never
// the literal hex ColorPalette. lipgloss.Color() and the ESC[38;5;{n}m
// escape both accept these indices directly. This is the REQUIRED baseline
// for CLI/TUI output; the literal hex ColorPalette is an opt-in enhancement
// only for terminals that report true-color support.
// Values match AI.md PART 16 "Themes" TerminalPalette exactly.
type TerminalPalette struct {
	Foreground string
	Muted      string
	Primary    string
	Success    string
	Warning    string
	Error      string
	Info       string
	Border     string
}

var (
	// TerminalPaletteDark is the ANSI-mapped baseline for the dark theme.
	// Values match AI.md PART 16 "Themes" TerminalPaletteDark exactly.
	TerminalPaletteDark = TerminalPalette{
		Foreground: "15", Muted: "7", Primary: "13",
		Success: "10", Warning: "11", Error: "9", Info: "12", Border: "13",
	}

	// TerminalPaletteLight is the ANSI-mapped baseline for the light theme.
	// Values match AI.md PART 16 "Themes" TerminalPaletteLight exactly.
	TerminalPaletteLight = TerminalPalette{
		Foreground: "0", Muted: "8", Primary: "4",
		Success: "2", Warning: "3", Error: "1", Info: "4", Border: "4",
	}
)

// GetTerminalPalette returns the ANSI-mapped baseline palette based on mode.
// Supported modes: "dark", "light", "auto"
func GetTerminalPalette(mode string) TerminalPalette {
	switch mode {
	case "light":
		return TerminalPaletteLight
	case "auto":
		if DetectSystemDark() {
			return TerminalPaletteDark
		}
		return TerminalPaletteLight
	default:
		return TerminalPaletteDark
	}
}
