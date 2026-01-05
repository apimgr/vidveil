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

// Config holds banner configuration
type Config struct {
	AppName    string
	Version    string
	Mode       string   // production/development
	Debug      bool
	URLs       []string
	ShowSetup  bool   // Show setup token (server only, first run)
	SetupToken string
}

// Print prints the startup banner based on terminal size
func Print(cfg Config) {
	size := terminal.GetSize()

	switch {
	case size.Mode >= terminal.SizeModeStandard:
		printFull(cfg, size)
	case size.Mode >= terminal.SizeModeCompact:
		printCompact(cfg)
	case size.Mode >= terminal.SizeModeMinimal:
		printMinimal(cfg)
	default:
		printMicro(cfg)
	}
}

// printFull prints the full banner with ASCII art
func printFull(cfg Config, size terminal.Size) {
	symbols := terminal.GetSymbols()
	p := theme.Get("auto")

	// Print ASCII art
	art := GetASCIIArt(cfg.AppName)
	for _, line := range art {
		fmt.Println(line)
	}

	fmt.Println()

	// Print version and mode
	modeColor := "\033[32m" // Green for production
	if cfg.Mode == "development" {
		modeColor = "\033[33m" // Yellow for development
	}
	if cfg.Debug {
		modeColor = "\033[35m" // Magenta for debug
	}
	fmt.Printf("  Version: %s  %sMode: %s\033[0m\n", cfg.Version, modeColor, cfg.Mode)

	// Print URLs
	if len(cfg.URLs) > 0 {
		fmt.Println()
		fmt.Printf("  %s Listening on:\n", symbols.Arrow)
		for _, url := range cfg.URLs {
			fmt.Printf("    %s %s\n", symbols.Bullet, url)
		}
	}

	// Print setup token if applicable
	if cfg.ShowSetup && cfg.SetupToken != "" {
		fmt.Println()
		fmt.Printf("  \033[33m%s First-time setup:\033[0m\n", symbols.Arrow)
		fmt.Printf("    Setup Token: %s\n", cfg.SetupToken)
	}

	// Print theme info
	themeName := theme.Name("auto")
	_ = p // Use palette for potential future color output
	fmt.Printf("\n  Theme: %s\n", themeName)

	fmt.Println()
}

// printCompact prints a compact banner without ASCII art
func printCompact(cfg Config) {
	symbols := terminal.GetSymbols()

	modeSymbol := symbols.Checkmark
	if cfg.Debug {
		modeSymbol = "D"
	}

	fmt.Printf("%s %s %s [%s %s]\n", symbols.Bullet, cfg.AppName, cfg.Version, modeSymbol, cfg.Mode)

	if len(cfg.URLs) > 0 {
		fmt.Printf("%s %s\n", symbols.Arrow, strings.Join(cfg.URLs, ", "))
	}

	if cfg.ShowSetup && cfg.SetupToken != "" {
		fmt.Printf("%s Setup: %s\n", symbols.Arrow, cfg.SetupToken)
	}
}

// printMinimal prints a minimal one-line banner
func printMinimal(cfg Config) {
	symbols := terminal.GetSymbols()
	url := ""
	if len(cfg.URLs) > 0 {
		url = " " + cfg.URLs[0]
	}
	fmt.Printf("%s %s %s%s\n", symbols.Bullet, cfg.AppName, cfg.Version, url)
}

// printMicro prints the most minimal banner for very small terminals
func printMicro(cfg Config) {
	fmt.Printf("%s %s\n", cfg.AppName, cfg.Version)
}

// PrintToWriter prints the banner to a specific writer
func PrintToWriter(w *os.File, cfg Config) {
	// Redirect stdout temporarily
	oldStdout := os.Stdout
	os.Stdout = w
	Print(cfg)
	os.Stdout = oldStdout
}
