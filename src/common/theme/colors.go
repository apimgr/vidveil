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
