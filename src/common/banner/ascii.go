// SPDX-License-Identifier: MIT
// Package banner provides startup banner printing
// See AI.md PART 7 for specification
package banner

import (
	"strings"
)

// GetASCIIArt returns ASCII art for the given app name
func GetASCIIArt(appName string) []string {
	// VidVeil-specific ASCII art
	if strings.EqualFold(appName, "vidveil") {
		return vidveilArt
	}
	// Generic fallback
	return generateSimpleArt(appName)
}

// vidveilArt is the VidVeil ASCII art
var vidveilArt = []string{
	"",
	"  ╦  ╦╦╔╦╗╦  ╦╔═╗╦╦  ",
	"  ╚╗╔╝║ ║║╚╗╔╝║╣ ║║  ",
	"   ╚╝ ╩═╩╝ ╚╝ ╚═╝╩╩═╝",
	"",
	"  Privacy-First Adult Video Search",
	"",
}

// generateSimpleArt generates a simple ASCII art header for any app name
func generateSimpleArt(appName string) []string {
	upper := strings.ToUpper(appName)
	width := len(upper) + 4

	topBorder := "  ╔" + strings.Repeat("═", width) + "╗"
	midLine := "  ║  " + upper + "  ║"
	botBorder := "  ╚" + strings.Repeat("═", width) + "╝"

	return []string{
		"",
		topBorder,
		midLine,
		botBorder,
		"",
	}
}

// GetCompactHeader returns a compact header for smaller terminals
func GetCompactHeader(appName string) string {
	return "=== " + strings.ToUpper(appName) + " ==="
}

// GetMicroHeader returns a minimal header
func GetMicroHeader(appName string) string {
	return "[" + strings.ToUpper(appName) + "]"
}
