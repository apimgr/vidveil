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
	Dark = ColorPalette{
		Background: "#282a36", Foreground: "#f8f8f2",
		Primary: "#bd93f9", Secondary: "#50fa7b", Accent: "#ff79c6",
		Success: "#50fa7b", Warning: "#ffb86c", Error: "#ff5555", Info: "#8be9fd",
		Surface: "#44475a", SurfaceAlt: "#1e1f29", Border: "#44475a", Muted: "#7585b8",
	}

	// Light is the light color palette
	Light = ColorPalette{
		Background: "#ffffff", Foreground: "#1a1b26",
		Primary: "#2e7de9", Secondary: "#587539", Accent: "#7847bd",
		Success: "#587539", Warning: "#8c6c3e", Error: "#c64343", Info: "#007197",
		Surface: "#f5f5f5", SurfaceAlt: "#e9e9ec", Border: "#c0caf5", Muted: "#6172b0",
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
