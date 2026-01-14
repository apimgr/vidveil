// SPDX-License-Identifier: MIT
// AI.md PART 36: CLI Client - Root Command
package cmd

import (
	"fmt"
	"os"
	"strings"

	"github.com/apimgr/vidveil/src/client/api"
	"github.com/apimgr/vidveil/src/client/paths"
	"github.com/apimgr/vidveil/src/common/display"
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

// CLIConfig holds CLI configuration
// Per AI.md PART 1: Type names MUST be specific - "Config" is ambiguous
type CLIConfig struct {
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
// Per AI.md PART 1: Variable names MUST reveal intent
var (
	cliConfigFilePath     string
	serverAddressFlag     string
	apiTokenFlag          string
	tokenFilePath         string
	outputFormatFlag      string
	colorDisabled         bool
	requestTimeoutSeconds int
	debugModeEnabled      bool
)

// Global config and client
// Per AI.md PART 1: Variable names MUST be specific
var (
	cliConfig *CLIConfig
	apiClient *api.Client
)

// ExecuteCLI runs the CLI application
// Per AI.md PART 36: Auto-detect TUI mode when interactive terminal + no command
func ExecuteCLI() error {
	args := os.Args[1:]

	// Parse global flags first
	args = ParseCLIGlobalFlags(args)

	// Load config
	LoadCLIConfigFromFile()

	// Initialize API client
	InitAPIClient()

	// Per AI.md PART 36: Automatic Mode Detection using display.DetectDisplayEnv()
	// - Interactive terminal + no command = TUI mode
	// - Interactive terminal + only config flags = TUI mode
	// - Interactive terminal + command provided = CLI mode
	// - Piped/redirected output = Plain output (no TUI)
	if len(args) == 0 {
		// Use display.DetectDisplayEnv() from src/common/display for mode detection
		displayEnv := display.DetectDisplayEnv()
		if displayEnv.Mode == display.DisplayModeTUI && displayEnv.IsTerminal {
			return RunInteractiveTUI()
		}
		// Non-interactive or headless: show help
		PrintCLIHelpMessage()
		return nil
	}

	// Route to command
	// Per AI.md PART 36: No tui command (auto-launches), no config command (edit cli.yml directly)
	switch args[0] {
	case "help", "-h", "--help":
		PrintCLIHelpMessage()
	case "version", "-v", "--version":
		PrintCLIVersionInfo()
	case "search":
		return RunSearchCommand(args[1:])
	case "login":
		return RunLoginCommand(args[1:])
	case "shell":
		return RunShellCommand(args[1:])
	case "probe":
		return RunProbeCommand(args[1:])
	default:
		// Treat first arg as search query
		return RunSearchCommand(args)
	}

	return nil
}

// ParseCLIGlobalFlags parses global CLI flags
// Per AI.md PART 36: Short flags only for -h (help) and -v (version)
// Per AI.md PART 36: NO --tui/--cli/--gui flags - UI mode is auto-detected
// Note: -h/--help and -v/--version only trigger exit if they appear BEFORE any command
func ParseCLIGlobalFlags(args []string) []string {
	var remaining []string
	commandSeen := false
	i := 0
	for i < len(args) {
		// If we've already seen a non-flag arg (command), pass remaining args through
		// This allows commands to handle their own --help flags
		if commandSeen {
			remaining = append(remaining, args[i])
			i++
			continue
		}

		switch args[i] {
		case "--server":
			if i+1 < len(args) {
				serverAddressFlag = args[i+1]
				i += 2
			} else {
				i++
			}
		case "--token":
			if i+1 < len(args) {
				apiTokenFlag = args[i+1]
				i += 2
			} else {
				i++
			}
		case "--token-file":
			if i+1 < len(args) {
				tokenFilePath = args[i+1]
				i += 2
			} else {
				i++
			}
		case "--output":
			if i+1 < len(args) {
				outputFormatFlag = args[i+1]
				i += 2
			} else {
				i++
			}
		case "--config":
			if i+1 < len(args) {
				cliConfigFilePath = args[i+1]
				i += 2
			} else {
				i++
			}
		case "--no-color":
			colorDisabled = true
			i++
		case "--timeout":
			if i+1 < len(args) {
				fmt.Sscanf(args[i+1], "%d", &requestTimeoutSeconds)
				i += 2
			} else {
				i++
			}
		case "--debug":
			debugModeEnabled = true
			i++
		case "-h", "--help":
			// Only handle global help if no command has been seen yet
			PrintCLIHelpMessage()
			os.Exit(0)
		case "-v", "--version":
			// Only handle global version if no command has been seen yet
			PrintCLIVersionInfo()
			os.Exit(0)
		default:
			remaining = append(remaining, args[i])
			// First non-flag argument is a command
			if !strings.HasPrefix(args[i], "-") {
				commandSeen = true
			}
			i++
		}
	}
	return remaining
}

// LoadCLIConfigFromFile loads CLI configuration from file
// Per AI.md PART 1: Function names MUST reveal intent - "loadConfig" is ambiguous
func LoadCLIConfigFromFile() {
	// Initialize config
	cliConfig = &CLIConfig{}

	// Default config
	cliConfig.Server.Timeout = 30
	cliConfig.Output.Format = "table"
	cliConfig.Output.Color = "auto"
	cliConfig.TUI.Theme = "default"
	cliConfig.TUI.ShowHints = true

	// Determine config path per AI.md PART 36
	// Uses paths module for OS-specific resolution
	if cliConfigFilePath == "" {
		cliConfigFilePath = paths.ConfigFile()
	}

	// Read config file if exists
	data, err := os.ReadFile(cliConfigFilePath)
	if err == nil {
		yaml.Unmarshal(data, cliConfig)
	}

	// Per AI.md PART 36: Token priority
	// 1. --token flag (highest)
	// 2. --token-file flag
	// 3. VIDVEIL_TOKEN env var
	// 4. config file (already loaded above)
	// 5. token file at default path (lowest)

	// Environment variables (lower priority than flags)
	// Per AI.md PART 36: Use VIDVEIL_TOKEN not VIDVEIL_CLI_TOKEN
	if cliConfig.Server.Token == "" {
		if env := os.Getenv("VIDVEIL_TOKEN"); env != "" {
			cliConfig.Server.Token = env
		}
	}
	if cliConfig.Server.Address == "" {
		if env := os.Getenv("VIDVEIL_SERVER"); env != "" {
			cliConfig.Server.Address = env
		}
	}

	// Token file (if specified via flag or config)
	if cliConfig.Server.Token == "" && tokenFilePath != "" {
		if data, err := os.ReadFile(tokenFilePath); err == nil {
			cliConfig.Server.Token = strings.TrimSpace(string(data))
		}
	}

	// Default token file location
	if cliConfig.Server.Token == "" {
		defaultTokenFilePath := paths.TokenFile()
		if data, err := os.ReadFile(defaultTokenFilePath); err == nil {
			cliConfig.Server.Token = strings.TrimSpace(string(data))
		}
	}

	// Command-line flags override everything (highest priority)
	if serverAddressFlag != "" {
		cliConfig.Server.Address = serverAddressFlag
	}
	if apiTokenFlag != "" {
		cliConfig.Server.Token = apiTokenFlag
	}
	if outputFormatFlag != "" {
		cliConfig.Output.Format = outputFormatFlag
	}
	if requestTimeoutSeconds > 0 {
		cliConfig.Server.Timeout = requestTimeoutSeconds
	}
	if colorDisabled {
		cliConfig.Output.Color = "never"
	}
}

// InitAPIClient initializes the API client
// Per AI.md PART 1: Function names MUST reveal intent - "initClient" is ambiguous
func InitAPIClient() {
	apiClient = api.NewClient(cliConfig.Server.Address, cliConfig.Server.Token, cliConfig.Server.Timeout)
	// Per AI.md PART 36: User-Agent uses hardcoded project name with version
	apiClient.SetUserAgent(Version)
}

// PrintCLIHelpMessage prints CLI help message
// Per AI.md PART 1: Function names MUST reveal intent - "printHelp" is ambiguous
func PrintCLIHelpMessage() {
	fmt.Printf(`%s %s - CLI client for VidVeil video search

Usage:
  %s [command] [flags]
  %s <query>              Search for videos (shortcut)

Commands:
  search <query>    Search for videos
  probe             Test engine availability
  login             Save API token to config
  shell             Shell completion commands

Flags:
      --config string      Config file (default: ~/.config/apimgr/vidveil/cli.yml)
      --server string      Server address
      --token string       API token for authentication
      --token-file string  Read token from file
      --output string      Output format: json, table, plain (default: table)
      --no-color           Disable colored output
      --timeout int        Request timeout in seconds (default: 30)
      --debug              Enable debug output
  -h, --help               Show help
  -v, --version            Show version

Environment Variables:
  VIDVEIL_SERVER    Server address
  VIDVEIL_TOKEN     API token

Note: Run without arguments to launch interactive TUI mode.

Examples:
  %s "search term"                Search for videos
  %s search --limit 20 "test"     Search with limit
  %s --output json "query"        Output as JSON
  %s login                        Save token interactively
  %s shell completions bash       Output bash completions

Use "%s [command] --help" for more information about a command.
`, BinaryName, Version, BinaryName, BinaryName, BinaryName, BinaryName, BinaryName, BinaryName, BinaryName, BinaryName)
}

// PrintCLIVersionInfo prints CLI version information
// Per AI.md PART 1: Function names MUST reveal intent - "printVersion" is ambiguous
func PrintCLIVersionInfo() {
	// Per AI.md PART 36: CLI --version format
	fmt.Printf("%s %s (%s) built %s\n", BinaryName, Version, CommitID, BuildDate)
}
