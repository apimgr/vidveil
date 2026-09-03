// SPDX-License-Identifier: MIT
// Package theme provides unified theming
// See AI.md PART 7 for specification
package theme

import (
	"os"
	"os/exec"
	"runtime"
	"strings"
)

// DetectSystemDark detects if the system is using a dark theme
func DetectSystemDark() bool {
	switch runtime.GOOS {
	case "linux", "freebsd", "openbsd", "netbsd":
		return detectLinuxDark()
	case "darwin":
		return detectMacOSDark()
	case "windows":
		return detectWindowsDark()
	default:
		// Default to dark
		return true
	}
}

// detectLinuxDark checks for dark theme on Linux
func detectLinuxDark() bool {
	// Check GNOME/GTK
	if theme := os.Getenv("GTK_THEME"); theme != "" {
		return strings.Contains(strings.ToLower(theme), "dark")
	}

	// Try gsettings for GNOME
	cmd := exec.Command("gsettings", "get", "org.gnome.desktop.interface", "gtk-theme")
	if output, err := cmd.Output(); err == nil {
		return strings.Contains(strings.ToLower(string(output)), "dark")
	}

	// Check KDE Plasma
	if theme := os.Getenv("QT_QPA_PLATFORMTHEME"); theme != "" {
		return strings.Contains(strings.ToLower(theme), "dark")
	}

	// Check for common dark theme environment variables
	if colorScheme := os.Getenv("COLOR_SCHEME"); colorScheme != "" {
		return strings.Contains(strings.ToLower(colorScheme), "dark")
	}

	// Default to dark for terminals (most developers prefer dark)
	return true
}

// detectMacOSDark checks for dark mode on macOS
func detectMacOSDark() bool {
	cmd := exec.Command("defaults", "read", "-g", "AppleInterfaceStyle")
	output, err := cmd.Output()
	if err != nil {
		// If command fails, light mode is likely active
		return false
	}
	return strings.TrimSpace(string(output)) == "Dark"
}

// detectWindowsDark checks for dark mode on Windows
func detectWindowsDark() bool {
	// Windows uses registry to store theme preference
	// HKEY_CURRENT_USER\Software\Microsoft\Windows\CurrentVersion\Themes\Personalize
	// AppsUseLightTheme = 0 means dark mode

	cmd := exec.Command("reg", "query",
		"HKEY_CURRENT_USER\\Software\\Microsoft\\Windows\\CurrentVersion\\Themes\\Personalize",
		"/v", "AppsUseLightTheme")
	output, err := cmd.Output()
	if err != nil {
		// Default to dark
		return true
	}

	// Parse output - looking for "0x0" which means dark mode
	return strings.Contains(string(output), "0x0")
}
