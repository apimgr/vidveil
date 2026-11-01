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
	_, _ = fmt.Fprintf(&sb, "  --bg-color: %s;\n", p.Background)
	_, _ = fmt.Fprintf(&sb, "  --text-color: %s;\n", p.Foreground)
	_, _ = fmt.Fprintf(&sb, "  --primary-color: %s;\n", p.Primary)
	_, _ = fmt.Fprintf(&sb, "  --secondary-color: %s;\n", p.Secondary)
	_, _ = fmt.Fprintf(&sb, "  --accent-color: %s;\n", p.Accent)
	_, _ = fmt.Fprintf(&sb, "  --success-color: %s;\n", p.Success)
	_, _ = fmt.Fprintf(&sb, "  --warning-color: %s;\n", p.Warning)
	_, _ = fmt.Fprintf(&sb, "  --danger-color: %s;\n", p.Error)
	_, _ = fmt.Fprintf(&sb, "  --info-color: %s;\n", p.Info)
	_, _ = fmt.Fprintf(&sb, "  --surface-color: %s;\n", p.Surface)
	_, _ = fmt.Fprintf(&sb, "  --surface-alt-color: %s;\n", p.SurfaceAlt)
	_, _ = fmt.Fprintf(&sb, "  --border-color: %s;\n", p.Border)
	_, _ = fmt.Fprintf(&sb, "  --muted-color: %s;\n", p.Muted)
	sb.WriteString("}\n")
	return sb.String()
}

// ToCSSClass generates CSS class definitions for theme elements
func (p ColorPalette) ToCSSClass(className string) string {
	var sb strings.Builder
	_, _ = fmt.Fprintf(&sb, ".%s {\n", className)
	_, _ = fmt.Fprintf(&sb, "  --bg-color: %s;\n", p.Background)
	_, _ = fmt.Fprintf(&sb, "  --text-color: %s;\n", p.Foreground)
	_, _ = fmt.Fprintf(&sb, "  --primary-color: %s;\n", p.Primary)
	_, _ = fmt.Fprintf(&sb, "  --secondary-color: %s;\n", p.Secondary)
	_, _ = fmt.Fprintf(&sb, "  --accent-color: %s;\n", p.Accent)
	_, _ = fmt.Fprintf(&sb, "  --success-color: %s;\n", p.Success)
	_, _ = fmt.Fprintf(&sb, "  --warning-color: %s;\n", p.Warning)
	_, _ = fmt.Fprintf(&sb, "  --danger-color: %s;\n", p.Error)
	_, _ = fmt.Fprintf(&sb, "  --info-color: %s;\n", p.Info)
	_, _ = fmt.Fprintf(&sb, "  --surface-color: %s;\n", p.Surface)
	_, _ = fmt.Fprintf(&sb, "  --surface-alt-color: %s;\n", p.SurfaceAlt)
	_, _ = fmt.Fprintf(&sb, "  --border-color: %s;\n", p.Border)
	_, _ = fmt.Fprintf(&sb, "  --muted-color: %s;\n", p.Muted)
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
