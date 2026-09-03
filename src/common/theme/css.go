// SPDX-License-Identifier: MIT
// Package theme provides unified theming
// See AI.md PART 7 for specification
package theme

import (
	"fmt"
	"strings"
)

// ToCSS generates CSS variables for the palette
func (p ColorPalette) ToCSS() string {
	var sb strings.Builder
	sb.WriteString(":root {\n")
	sb.WriteString(fmt.Sprintf("  --bg-color: %s;\n", p.Background))
	sb.WriteString(fmt.Sprintf("  --text-color: %s;\n", p.Foreground))
	sb.WriteString(fmt.Sprintf("  --primary-color: %s;\n", p.Primary))
	sb.WriteString(fmt.Sprintf("  --secondary-color: %s;\n", p.Secondary))
	sb.WriteString(fmt.Sprintf("  --accent-color: %s;\n", p.Accent))
	sb.WriteString(fmt.Sprintf("  --success-color: %s;\n", p.Success))
	sb.WriteString(fmt.Sprintf("  --warning-color: %s;\n", p.Warning))
	sb.WriteString(fmt.Sprintf("  --danger-color: %s;\n", p.Error))
	sb.WriteString(fmt.Sprintf("  --info-color: %s;\n", p.Info))
	sb.WriteString(fmt.Sprintf("  --surface-color: %s;\n", p.Surface))
	sb.WriteString(fmt.Sprintf("  --surface-alt-color: %s;\n", p.SurfaceAlt))
	sb.WriteString(fmt.Sprintf("  --border-color: %s;\n", p.Border))
	sb.WriteString(fmt.Sprintf("  --muted-color: %s;\n", p.Muted))
	sb.WriteString("}\n")
	return sb.String()
}

// ToCSSClass generates CSS class definitions for theme elements
func (p ColorPalette) ToCSSClass(className string) string {
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf(".%s {\n", className))
	sb.WriteString(fmt.Sprintf("  --bg-color: %s;\n", p.Background))
	sb.WriteString(fmt.Sprintf("  --text-color: %s;\n", p.Foreground))
	sb.WriteString(fmt.Sprintf("  --primary-color: %s;\n", p.Primary))
	sb.WriteString(fmt.Sprintf("  --secondary-color: %s;\n", p.Secondary))
	sb.WriteString(fmt.Sprintf("  --accent-color: %s;\n", p.Accent))
	sb.WriteString(fmt.Sprintf("  --success-color: %s;\n", p.Success))
	sb.WriteString(fmt.Sprintf("  --warning-color: %s;\n", p.Warning))
	sb.WriteString(fmt.Sprintf("  --danger-color: %s;\n", p.Error))
	sb.WriteString(fmt.Sprintf("  --info-color: %s;\n", p.Info))
	sb.WriteString(fmt.Sprintf("  --surface-color: %s;\n", p.Surface))
	sb.WriteString(fmt.Sprintf("  --surface-alt-color: %s;\n", p.SurfaceAlt))
	sb.WriteString(fmt.Sprintf("  --border-color: %s;\n", p.Border))
	sb.WriteString(fmt.Sprintf("  --muted-color: %s;\n", p.Muted))
	sb.WriteString("}\n")
	return sb.String()
}

// ToLipgloss returns lipgloss-compatible color values for terminal UI
func (p ColorPalette) ToLipgloss() map[string]string {
	return map[string]string{
		"background":  p.Background,
		"foreground":  p.Foreground,
		"primary":     p.Primary,
		"secondary":   p.Secondary,
		"accent":      p.Accent,
		"success":     p.Success,
		"warning":     p.Warning,
		"error":       p.Error,
		"info":        p.Info,
		"surface":     p.Surface,
		"surface_alt": p.SurfaceAlt,
		"border":      p.Border,
		"muted":       p.Muted,
	}
}
