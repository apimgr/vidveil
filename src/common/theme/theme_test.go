// SPDX-License-Identifier: MIT
// Tests for the theme package: color palettes, CSS generation, and dark-mode detection.
// Same-package access is required to reach unexported detectLinuxDark.
package theme

import (
	"strings"
	"testing"
)

// --- ColorPalette field completeness ---

// Every field of the Dark palette must be non-empty.
func TestDarkPaletteAllFieldsNonEmpty(t *testing.T) {
	fields := map[string]string{
		"Background": Dark.Background,
		"Foreground": Dark.Foreground,
		"Primary":    Dark.Primary,
		"Secondary":  Dark.Secondary,
		"Accent":     Dark.Accent,
		"Success":    Dark.Success,
		"Warning":    Dark.Warning,
		"Error":      Dark.Error,
		"Info":       Dark.Info,
		"Surface":    Dark.Surface,
		"SurfaceAlt": Dark.SurfaceAlt,
		"Border":     Dark.Border,
		"Muted":      Dark.Muted,
	}
	for name, val := range fields {
		if val == "" {
			t.Errorf("Dark.%s is empty, want non-empty hex color", name)
		}
	}
}

// Every field of the Light palette must be non-empty.
func TestLightPaletteAllFieldsNonEmpty(t *testing.T) {
	fields := map[string]string{
		"Background": Light.Background,
		"Foreground": Light.Foreground,
		"Primary":    Light.Primary,
		"Secondary":  Light.Secondary,
		"Accent":     Light.Accent,
		"Success":    Light.Success,
		"Warning":    Light.Warning,
		"Error":      Light.Error,
		"Info":       Light.Info,
		"Surface":    Light.Surface,
		"SurfaceAlt": Light.SurfaceAlt,
		"Border":     Light.Border,
		"Muted":      Light.Muted,
	}
	for name, val := range fields {
		if val == "" {
			t.Errorf("Light.%s is empty, want non-empty hex color", name)
		}
	}
}

// Dark and Light palettes must differ in at least Background color.
func TestDarkAndLightPalettesDiffer(t *testing.T) {
	if Dark.Background == Light.Background {
		t.Errorf("Dark.Background == Light.Background (%q), palettes must differ", Dark.Background)
	}
}

// --- GetColorPalette ---

// "dark" must return the Dark palette.
func TestGetColorPaletteDark(t *testing.T) {
	p := GetColorPalette("dark")
	if p.Background != Dark.Background {
		t.Errorf("GetColorPalette(\"dark\").Background = %q, want %q", p.Background, Dark.Background)
	}
}

// "light" must return the Light palette.
func TestGetColorPaletteLight(t *testing.T) {
	p := GetColorPalette("light")
	if p.Background != Light.Background {
		t.Errorf("GetColorPalette(\"light\").Background = %q, want %q", p.Background, Light.Background)
	}
}

// "auto" must return a non-empty palette regardless of system state.
func TestGetColorPaletteAuto(t *testing.T) {
	p := GetColorPalette("auto")
	if p.Background == "" {
		t.Error("GetColorPalette(\"auto\").Background is empty, want non-empty")
	}
}

// An empty mode string must return the Dark palette (default).
func TestGetColorPaletteEmptyIsDefault(t *testing.T) {
	p := GetColorPalette("")
	if p.Background != Dark.Background {
		t.Errorf("GetColorPalette(\"\").Background = %q, want Dark (%q)", p.Background, Dark.Background)
	}
}

// An unknown mode string must return the Dark palette (default).
func TestGetColorPaletteUnknownIsDefault(t *testing.T) {
	p := GetColorPalette("unknown")
	if p.Background != Dark.Background {
		t.Errorf("GetColorPalette(\"unknown\").Background = %q, want Dark (%q)", p.Background, Dark.Background)
	}
}

// --- GetColorPaletteName ---

// "dark" must return "dark".
func TestGetColorPaletteNameDark(t *testing.T) {
	got := GetColorPaletteName("dark")
	if got != "dark" {
		t.Errorf("GetColorPaletteName(\"dark\") = %q, want %q", got, "dark")
	}
}

// "light" must return "light".
func TestGetColorPaletteNameLight(t *testing.T) {
	got := GetColorPaletteName("light")
	if got != "light" {
		t.Errorf("GetColorPaletteName(\"light\") = %q, want %q", got, "light")
	}
}

// An empty string must return "dark" (default).
func TestGetColorPaletteNameEmptyIsDefault(t *testing.T) {
	got := GetColorPaletteName("")
	if got != "dark" {
		t.Errorf("GetColorPaletteName(\"\") = %q, want %q", got, "dark")
	}
}

// "auto" must return either "dark" or "light" — never empty.
func TestGetColorPaletteNameAutoNotEmpty(t *testing.T) {
	got := GetColorPaletteName("auto")
	if got != "dark" && got != "light" {
		t.Errorf("GetColorPaletteName(\"auto\") = %q, want \"dark\" or \"light\"", got)
	}
}

// --- ColorPalette.ToCSS ---

// Dark.ToCSS must open with ":root {" and close with "}\n".
func TestDarkToCSSStructure(t *testing.T) {
	css := Dark.ToCSS()
	if !strings.Contains(css, ":root {") {
		t.Errorf("Dark.ToCSS() = %q, want to contain \":root {\"", css)
	}
	if !strings.HasSuffix(css, "}\n") {
		t.Errorf("Dark.ToCSS() does not end with \"}\\n\"")
	}
}

// Dark.ToCSS must include the Dark background color as --bg-color.
func TestDarkToCSSBackgroundColor(t *testing.T) {
	css := Dark.ToCSS()
	if !strings.Contains(css, "--bg-color: #1a1b26") {
		t.Errorf("Dark.ToCSS() = %q, want to contain \"--bg-color: #1a1b26\"", css)
	}
}

// Dark.ToCSS must include --text-color.
func TestDarkToCSSTextColor(t *testing.T) {
	css := Dark.ToCSS()
	if !strings.Contains(css, "--text-color:") {
		t.Errorf("Dark.ToCSS() = %q, want to contain \"--text-color:\"", css)
	}
}

// Dark.ToCSS must include --primary-color.
func TestDarkToCSSSPrimaryColor(t *testing.T) {
	css := Dark.ToCSS()
	if !strings.Contains(css, "--primary-color:") {
		t.Errorf("Dark.ToCSS() = %q, want to contain \"--primary-color:\"", css)
	}
}

// Light.ToCSS must include the Light background (#ffffff).
func TestLightToCSSBackgroundColor(t *testing.T) {
	css := Light.ToCSS()
	if !strings.Contains(css, "#ffffff") {
		t.Errorf("Light.ToCSS() = %q, want to contain \"#ffffff\"", css)
	}
}

// --- ColorPalette.ToCSSClass ---

// Dark.ToCSSClass("night") must open with ".night {" and close with "}\n".
func TestDarkToCSSClassStructure(t *testing.T) {
	css := Dark.ToCSSClass("night")
	if !strings.HasPrefix(css, ".night {") {
		t.Errorf("Dark.ToCSSClass(\"night\") = %q, want prefix \".night {\"", css)
	}
	if !strings.HasSuffix(css, "}\n") {
		t.Errorf("Dark.ToCSSClass(\"night\") does not end with \"}\\n\"")
	}
}

// ToCSSClass must include all 13 CSS custom properties.
func TestDarkToCSSClassAllVariables(t *testing.T) {
	css := Dark.ToCSSClass("night")
	vars := []string{
		"--bg-color:", "--text-color:", "--primary-color:", "--secondary-color:",
		"--accent-color:", "--success-color:", "--warning-color:", "--danger-color:",
		"--info-color:", "--surface-color:", "--surface-alt-color:", "--border-color:",
		"--muted-color:",
	}
	for _, v := range vars {
		if !strings.Contains(css, v) {
			t.Errorf("Dark.ToCSSClass(\"night\") missing CSS variable %q", v)
		}
	}
}

// --- ColorPalette.ToLipgloss ---

// Dark.ToLipgloss must return a map with exactly 13 keys.
func TestDarkToLipglossKeyCount(t *testing.T) {
	m := Dark.ToLipgloss()
	if len(m) != 13 {
		t.Errorf("Dark.ToLipgloss() returned %d entries, want 13", len(m))
	}
}

// Dark.ToLipgloss must contain all required semantic keys.
func TestDarkToLipglossRequiredKeys(t *testing.T) {
	m := Dark.ToLipgloss()
	keys := []string{
		"background", "foreground", "primary", "secondary", "accent",
		"success", "warning", "error", "info",
		"surface", "surface_alt", "border", "muted",
	}
	for _, k := range keys {
		if _, ok := m[k]; !ok {
			t.Errorf("Dark.ToLipgloss() missing key %q", k)
		}
	}
}

// Light.ToLipgloss must have the correct background value.
func TestLightToLipglossBackground(t *testing.T) {
	m := Light.ToLipgloss()
	if m["background"] != "#ffffff" {
		t.Errorf("Light.ToLipgloss()[\"background\"] = %q, want %q", m["background"], "#ffffff")
	}
}

// All values in Dark.ToLipgloss must be non-empty.
func TestDarkToLipglossAllValuesNonEmpty(t *testing.T) {
	for k, v := range Dark.ToLipgloss() {
		if v == "" {
			t.Errorf("Dark.ToLipgloss()[%q] is empty, want non-empty", k)
		}
	}
}

// --- DetectSystemDark ---

// DetectSystemDark must return a boolean without panicking on the current platform.
func TestDetectSystemDarkNoPanic(t *testing.T) {
	defer func() {
		if r := recover(); r != nil {
			t.Errorf("DetectSystemDark() panicked: %v", r)
		}
	}()
	result := DetectSystemDark()
	// Result is system-dependent; just verify it is a valid bool by asserting its type at compile time.
	_ = result
}

// --- detectLinuxDark ---

// GTK_THEME containing ":dark" must be detected as dark.
func TestDetectLinuxDarkGTKThemeDark(t *testing.T) {
	t.Setenv("GTK_THEME", "Adwaita:dark")
	t.Setenv("COLOR_SCHEME", "")
	t.Setenv("QT_QPA_PLATFORMTHEME", "")
	if !detectLinuxDark() {
		t.Error("detectLinuxDark() = false for GTK_THEME=Adwaita:dark, want true")
	}
}

// GTK_THEME without "dark" must be detected as light.
func TestDetectLinuxDarkGTKThemeLight(t *testing.T) {
	t.Setenv("GTK_THEME", "Adwaita")
	t.Setenv("COLOR_SCHEME", "")
	t.Setenv("QT_QPA_PLATFORMTHEME", "")
	if detectLinuxDark() {
		t.Error("detectLinuxDark() = true for GTK_THEME=Adwaita (no dark), want false")
	}
}

// COLOR_SCHEME=prefer-dark with no GTK_THEME set must be detected as dark.
func TestDetectLinuxDarkColorScheme(t *testing.T) {
	t.Setenv("GTK_THEME", "")
	t.Setenv("QT_QPA_PLATFORMTHEME", "")
	t.Setenv("COLOR_SCHEME", "prefer-dark")
	if !detectLinuxDark() {
		t.Error("detectLinuxDark() = false for COLOR_SCHEME=prefer-dark, want true")
	}
}

// No env vars set must fall through to the default (true = dark).
func TestDetectLinuxDarkDefaultFallback(t *testing.T) {
	t.Setenv("GTK_THEME", "")
	t.Setenv("QT_QPA_PLATFORMTHEME", "")
	t.Setenv("COLOR_SCHEME", "")
	// gsettings is unlikely to be installed or configured in CI, so the function
	// falls through to the hard-coded default (true).
	// We cannot assert a deterministic result when gsettings IS available and
	// returns a non-dark theme, so we only verify no panic occurs.
	defer func() {
		if r := recover(); r != nil {
			t.Errorf("detectLinuxDark() panicked with no env vars: %v", r)
		}
	}()
	_ = detectLinuxDark()
}
