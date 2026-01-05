// SPDX-License-Identifier: MIT
// Package theme provides unified theming
// See AI.md PART 7 for specification
package theme

// Palette defines the color scheme for the application
type Palette struct {
	Background, Foreground         string
	Primary, Secondary, Accent     string
	Success, Warning, Error, Info  string
	Surface, SurfaceAlt, Border, Muted string
}

var (
	// Dark is the default dark color palette (Tokyo Night inspired)
	Dark = Palette{
		Background: "#1a1b26", Foreground: "#c0caf5",
		Primary: "#7aa2f7", Secondary: "#9ece6a", Accent: "#bb9af7",
		Success: "#9ece6a", Warning: "#e0af68", Error: "#f7768e", Info: "#7dcfff",
		Surface: "#24283b", SurfaceAlt: "#1f2335", Border: "#414868", Muted: "#565f89",
	}

	// Light is the light color palette
	Light = Palette{
		Background: "#ffffff", Foreground: "#1a1b26",
		Primary: "#2e7de9", Secondary: "#587539", Accent: "#7847bd",
		Success: "#587539", Warning: "#8c6c3e", Error: "#c64343", Info: "#007197",
		Surface: "#f5f5f5", SurfaceAlt: "#e9e9ec", Border: "#c0caf5", Muted: "#6172b0",
	}
)

// Get returns the appropriate palette based on mode
// Supported modes: "dark", "light", "auto"
func Get(mode string) Palette {
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

// Name returns the name of the palette based on mode
func Name(mode string) string {
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
