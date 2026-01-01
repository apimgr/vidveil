// SPDX-License-Identifier: MIT
// AI.md PART 36: CLI Client - Root Command
package cmd

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/apimgr/vidveil/src/client/api"
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

// Global flags
var (
	cfgFile    string
	serverAddr string
	apiToken   string
	outputFmt  string
	noColor    bool
	timeout    int
	tuiMode    bool
)

// Global config and client
var (
	cfg    Config
	client *api.Client
)

// Execute runs the CLI
func Execute() error {
	args := os.Args[1:]

	// No args - show help
	if len(args) == 0 {
		printHelp()
		return nil
	}

	// Parse global flags first
	args = parseGlobalFlags(args)

	// Load config
	loadConfig()

	// Initialize API client
	initClient()

	// Handle remaining args
	if len(args) == 0 {
		if tuiMode {
			return runTUI()
		}
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

func parseGlobalFlags(args []string) []string {
	var remaining []string
	i := 0
	for i < len(args) {
		switch args[i] {
		case "-s", "--server":
			if i+1 < len(args) {
				serverAddr = args[i+1]
				i += 2
			} else {
				i++
			}
		case "-t", "--token":
			if i+1 < len(args) {
				apiToken = args[i+1]
				i += 2
			} else {
				i++
			}
		case "-o", "--output":
			if i+1 < len(args) {
				outputFmt = args[i+1]
				i += 2
			} else {
				i++
			}
		case "-c", "--config":
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

	// Determine config path
	if cfgFile == "" {
		home, _ := os.UserHomeDir()
		cfgFile = filepath.Join(home, ".config", ProjectName, "cli.yml")
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

func printHelp() {
	fmt.Printf(`%s v%s - CLI client for VidVeil video search

Usage:
  %s [command] [flags]
  %s <query>              Search for videos (shortcut)

Commands:
  search <query>    Search for videos
  config            Manage configuration
  tui               Launch interactive TUI
  version           Show version information
  help              Show this help

Flags:
  -s, --server string    Server address (default: config or https://x.scour.li)
  -t, --token string     API token for authentication
  -o, --output string    Output format: json, table, plain (default: table)
  -c, --config string    Path to config file
      --no-color         Disable colored output
      --timeout int      Request timeout in seconds (default: 30)
      --tui              Launch TUI mode
  -h, --help             Show help
  -v, --version          Show version

Examples:
  %s "amateur"                    Search for videos
  %s search --limit 20 "test"     Search with limit
  %s --output json "query"        Output as JSON
  %s tui                          Launch interactive TUI

Use "%s [command] --help" for more information about a command.
`, BinaryName, Version, BinaryName, BinaryName, BinaryName, BinaryName, BinaryName, BinaryName, BinaryName)
}

func printVersion() {
	fmt.Printf("%s v%s (%s) built %s\n", BinaryName, Version, CommitID, BuildDate)
}
