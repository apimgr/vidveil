// SPDX-License-Identifier: MIT
// AI.md PART 32: CLI Client - Login Command
package cmd

import (
	"bufio"
	"fmt"
	"os"
	"strings"

	paths "github.com/apimgr/vidveil/src/client/paths"
	"gopkg.in/yaml.v3"
)

// RunLoginCommand handles the login command
// Per AI.md PART 1: Function names MUST reveal intent - "runLogin" is ambiguous
// Per AI.md PART 32: Saves token for future use
func RunLoginCommand(args []string) error {
	// Parse args for help
	for _, arg := range args {
		if arg == "-h" || arg == "--help" {
			printLoginCommandHelp()
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
			return NewExitError(ExitUsage, fmt.Errorf("server URL is required"))
		}
	} else {
		fmt.Printf("Server URL: %s\n", serverURL)
	}

	// Get token
	fmt.Print("API Token: ")
	input, _ := reader.ReadString('\n')
	apiTokenInput := strings.TrimSpace(input)
	if apiTokenInput == "" {
		return NewExitError(ExitUsage, fmt.Errorf("token is required"))
	}

	if err := validateCLIServerURL(serverURL); err != nil {
		return fmt.Errorf("invalid server URL: %w", err)
	}
	if err := saveCLILogin(serverURL, apiTokenInput); err != nil {
		return err
	}
	tokenFileLocation := paths.TokenFile()
	configFileLocation := getCLIConfigFilePath()

	fmt.Printf("\nLogged in successfully!\n")
	fmt.Printf("  Server: %s\n", serverURL)
	fmt.Printf("  Token saved to: %s\n", tokenFileLocation)
	fmt.Printf("  Config saved to: %s\n", configFileLocation)
	return nil
}

func saveCLILogin(serverURL, apiTokenInput string) error {
	if err := validateCLIServerURL(serverURL); err != nil {
		return fmt.Errorf("invalid server URL: %w", err)
	}
	if strings.TrimSpace(apiTokenInput) == "" {
		return fmt.Errorf("token is required")
	}
	if err := writeCLIDefaultTokenFile(apiTokenInput); err != nil {
		return err
	}

	// Also update config with server address
	configFileLocation := getCLIConfigFilePath()

	// Load or create config
	// Ignore unmarshal errors - use defaults if config is invalid
	var fileCLIConfig CLIConfig
	if data, err := os.ReadFile(configFileLocation); err == nil {
		_ = yaml.Unmarshal(data, &fileCLIConfig)
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

	if err := writeCLIConfigFile(fileCLIConfig, configFileLocation); err != nil {
		return err
	}

	return nil
}

// saveCLIServerConfiguration persists the optional-token first-run GUI setup.
func saveCLIServerConfiguration(serverURL, apiTokenInput string) error {
	if err := validateCLIServerURL(serverURL); err != nil {
		return fmt.Errorf("invalid server URL: %w", err)
	}
	if strings.TrimSpace(apiTokenInput) != "" {
		return saveCLILogin(serverURL, apiTokenInput)
	}
	configFileLocation := getCLIConfigFilePath()
	var fileCLIConfig CLIConfig
	if data, err := os.ReadFile(configFileLocation); err == nil {
		_ = yaml.Unmarshal(data, &fileCLIConfig)
	} else if !os.IsNotExist(err) {
		return fmt.Errorf("reading CLI config: %w", err)
	}
	fileCLIConfig.Server.Address = serverURL
	if fileCLIConfig.Server.APIVersion == "" {
		fileCLIConfig.Server.APIVersion = "v1"
	}
	return writeCLIConfigFile(fileCLIConfig, configFileLocation)
}

// printLoginCommandHelp prints help for the login command
// Per AI.md PART 1: Function names MUST reveal intent - "loginHelp" is ambiguous
func printLoginCommandHelp() {
	fmt.Printf(`Save API token for future use

Usage:
  %s login

This command prompts for:
  - Server URL (if not already configured)
  - API Token

The token is saved securely to %s
The server URL is saved to the selected config file (default: %s)

You can also use environment variables or flags:
  VIDVEIL_SERVER_PRIMARY Server URL (canonical)
  VIDVEIL_SERVER    Server URL compatibility alias
  VIDVEIL_TOKEN     API token (canonical)
  VIDVEIL_CLI_TOKEN API token compatibility alias
  --token-file      Read token from a file
`, BinaryName, paths.TokenFile(), paths.ConfigFile())
}
