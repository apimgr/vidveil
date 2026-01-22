// SPDX-License-Identifier: MIT
// Package banner provides startup banner printing
// See AI.md PART 7 and PART 17 for specification
package banner

import (
	"fmt"
	"os"
	"strings"

	"github.com/apimgr/vidveil/src/common/terminal"
)

// BannerConfig holds banner configuration
// Per AI.md PART 1: Type names MUST be specific, not generic "Config"
type BannerConfig struct {
	AppName    string
	Version    string
	Mode       string   // production/development
	Debug      bool
	URLs       []string
	AdminPath  string   // admin panel path (default: "admin")
	ShowSetup  bool     // Show setup token (server only, first run)
	SetupToken string
	TorEnabled bool     // Tor hidden service enabled
	TorAddress string   // .onion address if Tor enabled
	SMTPStatus string   // SMTP status message (e.g., "Auto-detected (localhost:25)")
}

// BoxWidth is the standard width for the boxed banner
const BoxWidth = 72

// PrintStartupBanner prints the startup banner based on terminal size
// Per AI.md PART 17: Uses boxed format with Unicode box drawing characters
func PrintStartupBanner(config BannerConfig) {
	size := terminal.GetTerminalSize()

	switch {
	case size.Mode >= terminal.SizeModeStandard:
		printFullBoxedBanner(config)
	case size.Mode >= terminal.SizeModeCompact:
		printCompactBanner(config)
	case size.Mode >= terminal.SizeModeMinimal:
		printMinimalBanner(config)
	default:
		printMicroBanner(config)
	}
}

// printFullBoxedBanner prints the full boxed banner per AI.md PART 17 lines 22712-22739
func printFullBoxedBanner(config BannerConfig) {
	// Box drawing characters
	topLeft := "╔"
	topRight := "╗"
	bottomLeft := "╚"
	bottomRight := "╝"
	horizontal := "═"
	vertical := "║"
	midLeft := "╠"
	midRight := "╣"

	// Build top border
	topBorder := topLeft + strings.Repeat(horizontal, BoxWidth) + topRight
	bottomBorder := bottomLeft + strings.Repeat(horizontal, BoxWidth) + bottomRight
	midBorder := midLeft + strings.Repeat(horizontal, BoxWidth) + midRight

	// Helper to print a line in the box
	printLine := func(content string) {
		// Calculate padding needed (accounting for emoji width)
		contentLen := displayWidth(content)
		padding := BoxWidth - contentLen - 2 // -2 for spaces around content
		if padding < 0 {
			padding = 0
		}
		fmt.Printf("%s  %s%s  %s\n", vertical, content, strings.Repeat(" ", padding), vertical)
	}

	// Empty line in box
	printEmpty := func() {
		fmt.Printf("%s%s%s\n", vertical, strings.Repeat(" ", BoxWidth), vertical)
	}

	// Print header section
	fmt.Println(topBorder)
	printEmpty()

	// App name and version
	appTitle := fmt.Sprintf("%s %s", strings.ToUpper(config.AppName), config.Version)
	printLine(appTitle)
	printEmpty()

	// Status line
	statusText := "Running"
	if config.ShowSetup {
		statusText = "Running (first run - setup available)"
	}
	modeIcon := "🔒"
	if config.Mode == "development" {
		modeIcon = "🔧"
	}
	if config.Debug {
		modeIcon = "🐛"
	}
	printLine(fmt.Sprintf("Status: %s %s %s", statusText, modeIcon, config.Mode))
	printEmpty()

	// Separator
	fmt.Println(midBorder)
	printEmpty()

	// Web Interface URLs
	if len(config.URLs) > 0 {
		printLine("🌐 Web Interface:")
		for _, url := range config.URLs {
			printLine(fmt.Sprintf("   %s", url))
		}
		printEmpty()
	}

	// Admin Panel
	if config.AdminPath != "" {
		printLine("🔧 Admin Panel:")
		// Use first URL as base
		if len(config.URLs) > 0 {
			baseURL := config.URLs[0]
			// Remove trailing slash if present
			baseURL = strings.TrimSuffix(baseURL, "/")
			printLine(fmt.Sprintf("   %s/%s", baseURL, config.AdminPath))
		}
		printEmpty()
	}

	// Setup Token (first run only)
	if config.ShowSetup && config.SetupToken != "" {
		printLine(fmt.Sprintf("🔑 Setup Token (use at /%s):", config.AdminPath))
		printLine(fmt.Sprintf("   %s", config.SetupToken))
		printEmpty()
	}

	// Tor hidden service
	if config.TorEnabled && config.TorAddress != "" {
		printLine("🧅 Tor Hidden Service:")
		printLine(fmt.Sprintf("   http://%s", config.TorAddress))
		printEmpty()
	}

	// SMTP status
	if config.SMTPStatus != "" {
		printLine(fmt.Sprintf("📧 SMTP: %s", config.SMTPStatus))
		printEmpty()
	}

	// Warning for first run
	if config.ShowSetup && config.SetupToken != "" {
		printLine("⚠️  Save the setup token! It will not be shown again.")
		printEmpty()
	}

	// Print bottom border
	fmt.Println(bottomBorder)
	fmt.Println()
}

// displayWidth calculates the display width of a string, accounting for emoji
// Emoji typically take 2 columns in most terminals
func displayWidth(s string) int {
	width := 0
	for _, r := range s {
		if r > 0x1F000 || (r >= 0x2600 && r <= 0x27BF) || (r >= 0x1F300 && r <= 0x1F9FF) {
			// Emoji - count as 2
			width += 2
		} else if r > 127 {
			// Other Unicode - varies, estimate as 1
			width += 1
		} else {
			width += 1
		}
	}
	return width
}

// printCompactBanner prints a compact banner without box drawing
func printCompactBanner(config BannerConfig) {
	symbols := terminal.GetTerminalSymbols()

	modeSymbol := symbols.Checkmark
	if config.Debug {
		modeSymbol = "D"
	}

	fmt.Printf("🚀 %s %s [%s %s]\n", config.AppName, config.Version, modeSymbol, config.Mode)

	if len(config.URLs) > 0 {
		fmt.Printf("🌐 %s\n", strings.Join(config.URLs, ", "))
	}

	if config.TorEnabled && config.TorAddress != "" {
		fmt.Printf("🧅 %s\n", config.TorAddress)
	}

	if config.ShowSetup && config.SetupToken != "" {
		fmt.Printf("🔑 Setup: %s\n", config.SetupToken)
	}

	fmt.Println()
}

// printMinimalBanner prints a minimal one-line banner
func printMinimalBanner(config BannerConfig) {
	url := ""
	if len(config.URLs) > 0 {
		url = " " + config.URLs[0]
	}
	fmt.Printf("%s %s%s\n", config.AppName, config.Version, url)
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
