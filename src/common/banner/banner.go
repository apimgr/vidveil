// SPDX-License-Identifier: MIT
// Package banner provides startup banner printing
// Per AI.md PART 7 lines 8081-8115 and PART 17 lines 17284-17373
package banner

import (
	"fmt"
	"strings"

	"github.com/apimgr/vidveil/src/common/terminal"
)

// BannerConfig holds banner configuration
// Per AI.md PART 7 lines 8092-8100
type BannerConfig struct {
	AppName    string
	Version    string
	AppMode    string // production/development
	Debug      bool
	URLs       []string
	ShowSetup  bool // Show setup token (server only, first run)
	SetupToken string
}

// PrintStartupBanner prints the startup banner based on terminal size
// Per AI.md PART 7 lines 8102-8115
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
// Per AI.md PART 17 lines 17323-17333
func printStartupBannerFull(cfg BannerConfig) {
	// Full ASCII art logo
	fmt.Println(getASCIIArt(cfg.AppName))
	fmt.Println()
	fmt.Printf("🚀 %s v%s\n", cfg.AppName, cfg.Version)
	printStartupBannerAppModeLine(cfg.AppMode, cfg.Debug, true)
	fmt.Println()
	for _, url := range cfg.URLs {
		fmt.Printf("  🌐 %s\n", url)
	}
	// Setup token for first run
	if cfg.ShowSetup && cfg.SetupToken != "" {
		fmt.Println()
		fmt.Printf("  🔑 Setup Token: %s\n", cfg.SetupToken)
		fmt.Println("  ⚠️  Save the setup token! It will not be shown again.")
	}
	fmt.Println()
}

// printStartupBannerCompact prints compact banner without ASCII art
// Per AI.md PART 17 lines 17336-17343
func printStartupBannerCompact(cfg BannerConfig) {
	fmt.Printf("🚀 %s v%s\n", cfg.AppName, cfg.Version)
	printStartupBannerAppModeLine(cfg.AppMode, cfg.Debug, true)
	for _, url := range cfg.URLs {
		fmt.Printf("🌐 %s\n", url)
	}
	if cfg.ShowSetup && cfg.SetupToken != "" {
		fmt.Printf("🔑 Setup: %s\n", cfg.SetupToken)
	}
}

// printStartupBannerMinimal prints minimal banner without icons
// Per AI.md PART 17 lines 17345-17351
func printStartupBannerMinimal(cfg BannerConfig) {
	fmt.Printf("%s %s\n", cfg.AppName, cfg.Version)
	for _, url := range cfg.URLs {
		fmt.Println(extractHostPort(url))
	}
}

// printStartupBannerMicro prints single line for very narrow terminals
// Per AI.md PART 17 lines 17354-17360
func printStartupBannerMicro(cfg BannerConfig) {
	if len(cfg.URLs) > 0 {
		fmt.Printf("%s %s\n", cfg.AppName, extractHostPort(cfg.URLs[0]))
	} else {
		fmt.Println(cfg.AppName)
	}
}

// printStartupBannerAppModeLine prints the mode line with optional icons
// Per AI.md PART 17 lines 17363-17373
func printStartupBannerAppModeLine(appMode string, debug bool, useIcons bool) {
	if useIcons {
		icon := "🔒"
		if appMode == "development" {
			icon = "🔧"
		}
		if debug {
			icon = "🐛"
		}
		fmt.Printf("%s Running in mode: %s\n", icon, appMode)
	} else {
		fmt.Printf("Mode: %s\n", appMode)
	}
}

// getASCIIArt returns ASCII art for the app name
// Per AI.md PART 17 line 17325
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
// Per AI.md PART 17 line 17350
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
