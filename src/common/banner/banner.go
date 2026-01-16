// SPDX-License-Identifier: MIT
// Package banner provides startup banner printing
// See AI.md PART 7 for specification
package banner

import (
	"fmt"
	"os"
	"strings"

	"github.com/apimgr/vidveil/src/common/terminal"
	"github.com/apimgr/vidveil/src/common/theme"
)

// BannerConfig holds banner configuration
// Per AI.md PART 1: Type names MUST be specific, not generic "Config"
type BannerConfig struct {
	AppName    string
	Version    string
	Mode       string   // production/development
	Debug      bool
	URLs       []string
	ShowSetup  bool   // Show setup token (server only, first run)
	SetupToken string
}

// PrintStartupBanner prints the startup banner based on terminal size
// Per AI.md PART 1: Function names MUST reveal intent
func PrintStartupBanner(config BannerConfig) {
	size := terminal.GetTerminalSize()

	switch {
	case size.Mode >= terminal.SizeModeStandard:
		printFullBanner(config, size)
	case size.Mode >= terminal.SizeModeCompact:
		printCompactBanner(config)
	case size.Mode >= terminal.SizeModeMinimal:
		printMinimalBanner(config)
	default:
		printMicroBanner(config)
	}
}

// printFullBanner prints the full banner with ASCII art
func printFullBanner(config BannerConfig, size terminal.TerminalSize) {
	symbols := terminal.GetTerminalSymbols()
	p := theme.GetColorPalette("auto")

	// Print ASCII art
	art := GetASCIIArt(config.AppName)
	for _, line := range art {
		fmt.Println(line)
	}

	fmt.Println()

	// Print version and mode
	modeColor := "\033[32m" // Green for production
	if config.Mode == "development" {
		modeColor = "\033[33m" // Yellow for development
	}
	if config.Debug {
		modeColor = "\033[35m" // Magenta for debug
	}
	fmt.Printf("  Version: %s  %sMode: %s\033[0m\n", config.Version, modeColor, config.Mode)

	// Print URLs
	if len(config.URLs) > 0 {
		fmt.Println()
		fmt.Printf("  %s Listening on:\n", symbols.Arrow)
		for _, url := range config.URLs {
			fmt.Printf("    %s %s\n", symbols.Bullet, url)
		}
	}

	// Print setup token if applicable
	if config.ShowSetup && config.SetupToken != "" {
		fmt.Println()
		fmt.Printf("  \033[33m%s First-time setup:\033[0m\n", symbols.Arrow)
		fmt.Printf("    Setup Token: %s\n", config.SetupToken)
	}

	// Print theme info
	themeName := theme.GetColorPaletteName("auto")
	_ = p // Use palette for potential future color output
	fmt.Printf("\n  Theme: %s\n", themeName)

	fmt.Println()
}

// printCompactBanner prints a compact banner without ASCII art
func printCompactBanner(config BannerConfig) {
	symbols := terminal.GetTerminalSymbols()

	modeSymbol := symbols.Checkmark
	if config.Debug {
		modeSymbol = "D"
	}

	fmt.Printf("%s %s %s [%s %s]\n", symbols.Bullet, config.AppName, config.Version, modeSymbol, config.Mode)

	if len(config.URLs) > 0 {
		fmt.Printf("%s %s\n", symbols.Arrow, strings.Join(config.URLs, ", "))
	}

	if config.ShowSetup && config.SetupToken != "" {
		fmt.Printf("%s Setup: %s\n", symbols.Arrow, config.SetupToken)
	}
}

// printMinimalBanner prints a minimal one-line banner
func printMinimalBanner(config BannerConfig) {
	symbols := terminal.GetTerminalSymbols()
	url := ""
	if len(config.URLs) > 0 {
		url = " " + config.URLs[0]
	}
	fmt.Printf("%s %s %s%s\n", symbols.Bullet, config.AppName, config.Version, url)
}

// printMicroBanner prints the most minimal banner for very small terminals
func printMicroBanner(config BannerConfig) {
	fmt.Printf("%s %s\n", config.AppName, config.Version)
}

// PrintStartupBannerToWriter prints the banner to a specific writer
func PrintStartupBannerToWriter(w *os.File, config BannerConfig) {
	// Redirect stdout temporarily
	oldStdout := os.Stdout
	os.Stdout = w
	PrintStartupBanner(config)
	os.Stdout = oldStdout
}
