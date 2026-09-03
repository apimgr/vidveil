// SPDX-License-Identifier: MIT
// Package banner provides startup banner printing
// Per AI.md PART 7 and PART 8
package banner

import (
	"fmt"
	"strings"

	"github.com/apimgr/vidveil/src/common/terminal"
)

// BannerConfig holds banner configuration
// Per AI.md PART 7
type BannerConfig struct {
	AppName string
	Version string
	// AppMode is "production" or "development"
	AppMode string
	Debug   bool
	URLs    []string
	// ShowSetup indicates whether to show setup token (server only, first run)
	ShowSetup  bool
	SetupToken string
}

// PrintStartupBanner prints the startup banner based on terminal size
// Per AI.md PART 7 and PART 8 (respects NO_COLOR)
func PrintStartupBanner(cfg BannerConfig) {
	size := terminal.GetTerminalSize()

	switch {
	case size.Mode >= terminal.SizeModeStandard:
		printStartupBannerFull(cfg)
	case size.Mode >= terminal.SizeModeCompact:
		printStartupBannerCompact(cfg)
	case size.Mode >= terminal.SizeModeMinimal:
		printStartupBannerMinimal(cfg)
	default:
		printStartupBannerMicro(cfg)
	}
}

// printStartupBannerFull prints full banner with ASCII art
// Per AI.md PART 7 and PART 8
func printStartupBannerFull(cfg BannerConfig) {
	// Full ASCII art logo
	fmt.Println(getASCIIArt(cfg.AppName))
	fmt.Println()
	fmt.Printf("%s %s v%s\n", terminal.RocketIcon(), cfg.AppName, cfg.Version)
	printStartupBannerAppModeLine(cfg.AppMode, cfg.Debug, true)
	fmt.Println()
	for _, url := range cfg.URLs {
		fmt.Printf("  %s %s\n", terminal.GlobeIcon(), url)
	}
	// Setup token for first run
	if cfg.ShowSetup && cfg.SetupToken != "" {
		fmt.Println()
		fmt.Printf("  %s Setup Token: %s\n", terminal.KeyIcon(), cfg.SetupToken)
		fmt.Printf("  %s Save the setup token! It will not be shown again.\n", terminal.WarningIcon())
	}
	fmt.Println()
}

// printStartupBannerCompact prints compact banner without ASCII art
// Per AI.md PART 7 and PART 8
func printStartupBannerCompact(cfg BannerConfig) {
	fmt.Printf("%s %s v%s\n", terminal.RocketIcon(), cfg.AppName, cfg.Version)
	printStartupBannerAppModeLine(cfg.AppMode, cfg.Debug, true)
	for _, url := range cfg.URLs {
		fmt.Printf("%s %s\n", terminal.GlobeIcon(), url)
	}
	if cfg.ShowSetup && cfg.SetupToken != "" {
		fmt.Printf("%s Setup: %s\n", terminal.KeyIcon(), cfg.SetupToken)
	}
}

// printStartupBannerMinimal prints minimal banner without icons
// Per AI.md PART 7 and PART 8
func printStartupBannerMinimal(cfg BannerConfig) {
	fmt.Printf("%s %s\n", cfg.AppName, cfg.Version)
	for _, url := range cfg.URLs {
		fmt.Println(extractHostPort(url))
	}
}

// printStartupBannerMicro prints single line for very narrow terminals
// Per AI.md PART 7 and PART 8
func printStartupBannerMicro(cfg BannerConfig) {
	if len(cfg.URLs) > 0 {
		fmt.Printf("%s %s\n", cfg.AppName, extractHostPort(cfg.URLs[0]))
	} else {
		fmt.Println(cfg.AppName)
	}
}

// printStartupBannerAppModeLine prints the mode line with optional icons
// Per AI.md PART 7 and PART 8
func printStartupBannerAppModeLine(appMode string, debug bool, useIcons bool) {
	if useIcons {
		var icon string
		if debug {
			icon = terminal.BugIcon()
		} else if appMode == "development" {
			icon = terminal.WrenchIcon()
		} else {
			icon = terminal.LockIcon()
		}
		fmt.Printf("%s Running in mode: %s\n", icon, appMode)
	} else {
		fmt.Printf("Mode: %s\n", appMode)
	}
}

// getASCIIArt returns ASCII art for the app name
// Per AI.md PART 7
func getASCIIArt(appName string) string {
	// VidVeil ASCII art
	if strings.ToLower(appName) == "vidveil" {
		return `
 __      ___     ___      __    _ _
 \ \    / (_)   | \ \    / /   (_) |
  \ \  / / _  __| |\ \  / /__  _| |
   \ \/ / | |/ _` + "`" + ` | \ \/ / _ \| | |
    \  /  | | (_| |  \  /  __/| | |
     \/   |_|\__,_|   \/ \___||_|_|`
	}
	// Generic fallback
	return fmt.Sprintf("=== %s ===", strings.ToUpper(appName))
}

// extractHostPort extracts host:port from a URL
// Per AI.md PART 7
func extractHostPort(url string) string {
	// Remove protocol
	s := url
	if idx := strings.Index(s, "://"); idx >= 0 {
		s = s[idx+3:]
	}
	// Remove path
	if idx := strings.Index(s, "/"); idx >= 0 {
		s = s[:idx]
	}
	return s
}
