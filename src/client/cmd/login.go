// SPDX-License-Identifier: MIT
// AI.md PART 36: CLI Client - Login Command
package cmd

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/apimgr/vidveil/src/client/paths"
	"gopkg.in/yaml.v3"
)

// RunLoginCommand handles the login command
// Per AI.md PART 1: Function names MUST reveal intent - "runLogin" is ambiguous
// Per AI.md PART 36: Saves token for future use
func RunLoginCommand(args []string) error {
	// Parse args for help
	for _, arg := range args {
		if arg == "-h" || arg == "--help" {
			PrintLoginCommandHelp()
			return nil
		}
	}

	reader := bufio.NewReader(os.Stdin)

	// Get server address
	serverURL := cliConfig.Server.Address
	if serverURL == "" {
		fmt.Print("Server URL: ")
		input, _ := reader.ReadString('\n')
		serverURL = strings.TrimSpace(input)
		if serverURL == "" {
			return fmt.Errorf("server URL is required")
		}
	} else {
		fmt.Printf("Server URL: %s\n", serverURL)
	}

	// Get token
	fmt.Print("API Token: ")
	input, _ := reader.ReadString('\n')
	apiTokenInput := strings.TrimSpace(input)
	if apiTokenInput == "" {
		return fmt.Errorf("token is required")
	}

	// Save token to token file
	tokenFileLocation := paths.TokenFile()
	tokenDirPath := filepath.Dir(tokenFileLocation)

	// Ensure directory exists with correct permissions (0700)
	if err := os.MkdirAll(tokenDirPath, 0700); err != nil {
		return fmt.Errorf("creating token directory: %w", err)
	}

	// Write token file with restricted permissions (0600)
	if err := os.WriteFile(tokenFileLocation, []byte(apiTokenInput), 0600); err != nil {
		return fmt.Errorf("writing token file: %w", err)
	}

	// Also update config with server address
	configFileLocation := paths.ConfigFile()
	configDirPath := filepath.Dir(configFileLocation)

	// Ensure config directory exists with correct permissions (0700)
	if err := os.MkdirAll(configDirPath, 0700); err != nil {
		return fmt.Errorf("creating config directory: %w", err)
	}

	// Load or create config
	var fileCLIConfig CLIConfig
	if data, err := os.ReadFile(configFileLocation); err == nil {
		yaml.Unmarshal(data, &fileCLIConfig)
	}

	// Update server address (don't store token in config, it's in token file)
	fileCLIConfig.Server.Address = serverURL
	if fileCLIConfig.Server.Timeout == 0 {
		fileCLIConfig.Server.Timeout = 30
	}
	if fileCLIConfig.Output.Format == "" {
		fileCLIConfig.Output.Format = "table"
	}
	if fileCLIConfig.Output.Color == "" {
		fileCLIConfig.Output.Color = "auto"
	}

	// Write config
	data, err := yaml.Marshal(fileCLIConfig)
	if err != nil {
		return fmt.Errorf("marshaling config: %w", err)
	}

	// Per AI.md PART 5: Comments go ABOVE the setting
	content := "# VidVeil CLI Configuration\n# Edit this file or use --server/--token flags\n\n" + string(data)
	if err := os.WriteFile(configFileLocation, []byte(content), 0600); err != nil {
		return fmt.Errorf("writing config file: %w", err)
	}

	fmt.Printf("\nLogged in successfully!\n")
	fmt.Printf("  Server: %s\n", serverURL)
	fmt.Printf("  Token saved to: %s\n", tokenFileLocation)
	fmt.Printf("  Config saved to: %s\n", configFileLocation)

	return nil
}

// PrintLoginCommandHelp prints help for the login command
// Per AI.md PART 1: Function names MUST reveal intent - "loginHelp" is ambiguous
func PrintLoginCommandHelp() {
	fmt.Printf(`Save API token for future use

Usage:
  %s login

This command prompts for:
  - Server URL (if not already configured)
  - API Token

The token is saved securely to %s
The server URL is saved to %s

You can also use environment variables or flags:
  VIDVEIL_SERVER    Server URL
  VIDVEIL_TOKEN     API token (not recommended for scripts)
  --token-file      Read token from a file
`, BinaryName, paths.TokenFile(), paths.ConfigFile())
}
