// SPDX-License-Identifier: MIT
// AI.md PART 36: CLI Client - Root Command
package cmd

import (
	"fmt"
	"os"

	"github.com/apimgr/vidveil/src/client/api"
	"github.com/apimgr/vidveil/src/client/paths"
	"github.com/charmbracelet/x/term"
	"gopkg.in/yaml.v3"
)

// Build info (set by main.go)
var (
	ProjectName = "vidveil"
	Version     = "dev"
	CommitID    = "unknown"
	BuildDate   = "unknown"
	BinaryName  = "vidveil-cli"
)

// Config holds CLI configuration
type Config struct {
	Server struct {
		Address string `yaml:"address"`
		Token   string `yaml:"token"`
		Timeout int    `yaml:"timeout"`
	} `yaml:"server"`
	Output struct {
		Format string `yaml:"format"`
		Color  string `yaml:"color"`
	} `yaml:"output"`
	TUI struct {
		Theme     string `yaml:"theme"`
		ShowHints bool   `yaml:"show_hints"`
	} `yaml:"tui"`
}

// Global flags per AI.md PART 36
// Short flags only for -h (help) and -v (version)
var (
	cfgFile    string
	serverAddr string
	apiToken   string
	outputFmt  string
	noColor    bool
	timeout    int
	tuiMode    bool
	debugMode  bool
)

// Global config and client
var (
	cfg    Config
	client *api.Client
)

// Execute runs the CLI
// Per AI.md PART 36: Auto-detect TUI mode when interactive terminal + no command
func Execute() error {
	args := os.Args[1:]

	// Parse global flags first
	args = parseGlobalFlags(args)

	// Load config
	loadConfig()

	// Initialize API client
	initClient()

	// Per AI.md PART 36: Automatic Mode Detection
	// - Interactive terminal + no command = TUI mode
	// - Interactive terminal + only config flags = TUI mode
	// - Interactive terminal + command provided = CLI mode
	// - Piped/redirected output = Plain output (no TUI)
	if len(args) == 0 {
		// Check if stdout is interactive terminal
		if term.IsTerminal(os.Stdout.Fd()) {
			return runTUI()
		}
		// Non-interactive: show help
		printHelp()
		return nil
	}

	// Route to command
	switch args[0] {
	case "help", "-h", "--help":
		printHelp()
	case "version", "-v", "--version":
		printVersion()
	case "config":
		return runConfig(args[1:])
	case "search":
		return runSearch(args[1:])
	case "tui":
		return runTUI()
	default:
		// Treat first arg as search query
		return runSearch(args)
	}

	return nil
}

// parseGlobalFlags parses global flags per AI.md PART 36
// Short flags only for -h (help) and -v (version)
func parseGlobalFlags(args []string) []string {
	var remaining []string
	i := 0
	for i < len(args) {
		switch args[i] {
		case "--server":
			if i+1 < len(args) {
				serverAddr = args[i+1]
				i += 2
			} else {
				i++
			}
		case "--token":
			if i+1 < len(args) {
				apiToken = args[i+1]
				i += 2
			} else {
				i++
			}
		case "--output":
			if i+1 < len(args) {
				outputFmt = args[i+1]
				i += 2
			} else {
				i++
			}
		case "--config":
			if i+1 < len(args) {
				cfgFile = args[i+1]
				i += 2
			} else {
				i++
			}
		case "--no-color":
			noColor = true
			i++
		case "--timeout":
			if i+1 < len(args) {
				fmt.Sscanf(args[i+1], "%d", &timeout)
				i += 2
			} else {
				i++
			}
		case "--debug":
			debugMode = true
			i++
		case "--tui":
			tuiMode = true
			i++
		case "-h", "--help":
			printHelp()
			os.Exit(0)
		case "-v", "--version":
			printVersion()
			os.Exit(0)
		default:
			remaining = append(remaining, args[i])
			i++
		}
	}
	return remaining
}

func loadConfig() {
	// Default config
	cfg.Server.Timeout = 30
	cfg.Output.Format = "table"
	cfg.Output.Color = "auto"
	cfg.TUI.Theme = "default"
	cfg.TUI.ShowHints = true

	// Determine config path per AI.md PART 36
	// Uses paths module for OS-specific resolution
	if cfgFile == "" {
		cfgFile = paths.ConfigFile()
	}

	// Read config file if exists
	data, err := os.ReadFile(cfgFile)
	if err == nil {
		yaml.Unmarshal(data, &cfg)
	}

	// Command-line flags override config
	if serverAddr != "" {
		cfg.Server.Address = serverAddr
	}
	if apiToken != "" {
		cfg.Server.Token = apiToken
	}
	if outputFmt != "" {
		cfg.Output.Format = outputFmt
	}
	if timeout > 0 {
		cfg.Server.Timeout = timeout
	}
	if noColor {
		cfg.Output.Color = "never"
	}

	// Environment variables (middle priority)
	if env := os.Getenv("VIDVEIL_CLI_TOKEN"); env != "" && cfg.Server.Token == "" {
		cfg.Server.Token = env
	}
	if env := os.Getenv("VIDVEIL_CLI_SERVER"); env != "" && cfg.Server.Address == "" {
		cfg.Server.Address = env
	}
}

func initClient() {
	client = api.NewClient(cfg.Server.Address, cfg.Server.Token, cfg.Server.Timeout)
}

// printHelp prints help per AI.md PART 36 format
func printHelp() {
	fmt.Printf(`%s %s - CLI client for VidVeil video search

Usage:
  %s [command] [flags]
  %s <query>              Search for videos (shortcut)

Commands:
  search <query>    Search for videos
  config            Manage configuration

Flags:
      --config string    Config file to load (default: cli.yml)
      --server string    Server address (default: https://x.scour.li)
      --token string     API token for authentication
      --output string    Output format: json, table, plain (default: table)
      --no-color         Disable colored output
      --timeout int      Request timeout in seconds (default: 30)
      --debug            Enable debug output
  -h, --help             Show help
  -v, --version          Show version

Note: Run without arguments to launch interactive TUI mode.

Examples:
  %s "amateur"                    Search for videos
  %s search --limit 20 "test"     Search with limit
  %s --output json "query"        Output as JSON

Use "%s [command] --help" for more information about a command.
`, BinaryName, Version, BinaryName, BinaryName, BinaryName, BinaryName, BinaryName, BinaryName)
}

func printVersion() {
	// Per AI.md PART 36: CLI --version format
	fmt.Printf("%s %s (%s) built %s\n", BinaryName, Version, CommitID, BuildDate)
}
