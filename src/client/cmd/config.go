// SPDX-License-Identifier: MIT
// AI.md PART 33: CLI Client - Config Command
package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

func runConfig(args []string) error {
	if len(args) == 0 {
		return configShow()
	}

	switch args[0] {
	case "show":
		return configShow()
	case "init":
		return configInit()
	case "set":
		if len(args) < 3 {
			return fmt.Errorf("usage: config set <key> <value>")
		}
		return configSet(args[1], args[2])
	case "get":
		if len(args) < 2 {
			return fmt.Errorf("usage: config get <key>")
		}
		return configGet(args[1])
	case "path":
		fmt.Println(cfgFile)
		return nil
	case "--help", "-h":
		configHelp()
		return nil
	default:
		return fmt.Errorf("unknown config command: %s", args[0])
	}
}

func configHelp() {
	fmt.Printf(`Manage CLI configuration

Usage:
  %s config <command>

Commands:
  show              Display current configuration
  init              Create default config file
  set <key> <value> Set configuration value
  get <key>         Get configuration value
  path              Show config file path

Keys:
  server.address    Server URL
  server.token      API token
  server.timeout    Request timeout (seconds)
  output.format     Output format (json, table, plain)
  output.color      Color mode (auto, always, never)
  tui.theme         TUI theme (default, minimal, compact)
  tui.show_hints    Show TUI hints (true, false)

Examples:
  %s config init
  %s config set server.address https://example.com
  %s config get server.address
  %s config show
`, BinaryName, BinaryName, BinaryName, BinaryName, BinaryName)
}

func configShow() error {
	fmt.Printf("Config file: %s\n\n", cfgFile)

	// Check if file exists
	if _, err := os.Stat(cfgFile); os.IsNotExist(err) {
		fmt.Println("(No config file - using defaults)")
		fmt.Println()
	}

	fmt.Println("Current configuration:")
	fmt.Printf("  server.address:  %s\n", cfg.Server.Address)
	fmt.Printf("  server.token:    %s\n", maskToken(cfg.Server.Token))
	fmt.Printf("  server.timeout:  %d\n", cfg.Server.Timeout)
	fmt.Printf("  output.format:   %s\n", cfg.Output.Format)
	fmt.Printf("  output.color:    %s\n", cfg.Output.Color)
	fmt.Printf("  tui.theme:       %s\n", cfg.TUI.Theme)
	fmt.Printf("  tui.show_hints:  %v\n", cfg.TUI.ShowHints)

	return nil
}

func configInit() error {
	// Create directory if needed
	dir := filepath.Dir(cfgFile)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("creating config directory: %w", err)
	}

	// Check if file exists
	if _, err := os.Stat(cfgFile); err == nil {
		return fmt.Errorf("config file already exists: %s", cfgFile)
	}

	// Default config
	defaultCfg := Config{}
	defaultCfg.Server.Address = "https://x.scour.li"
	defaultCfg.Server.Timeout = 30
	defaultCfg.Output.Format = "table"
	defaultCfg.Output.Color = "auto"
	defaultCfg.TUI.Theme = "default"
	defaultCfg.TUI.ShowHints = true

	data, err := yaml.Marshal(defaultCfg)
	if err != nil {
		return fmt.Errorf("marshaling config: %w", err)
	}

	// Add header comment
	content := "# VidVeil CLI Configuration\n# " + cfgFile + "\n\n" + string(data)

	if err := os.WriteFile(cfgFile, []byte(content), 0600); err != nil {
		return fmt.Errorf("writing config file: %w", err)
	}

	fmt.Printf("Created config file: %s\n", cfgFile)
	return nil
}

func configSet(key, value string) error {
	// Load existing config or create new
	var fileCfg Config
	data, err := os.ReadFile(cfgFile)
	if err == nil {
		yaml.Unmarshal(data, &fileCfg)
	}

	// Set value based on key
	switch key {
	case "server.address":
		fileCfg.Server.Address = value
	case "server.token":
		fileCfg.Server.Token = value
	case "server.timeout":
		fmt.Sscanf(value, "%d", &fileCfg.Server.Timeout)
	case "output.format":
		fileCfg.Output.Format = value
	case "output.color":
		fileCfg.Output.Color = value
	case "tui.theme":
		fileCfg.TUI.Theme = value
	case "tui.show_hints":
		fileCfg.TUI.ShowHints = value == "true" || value == "1" || value == "yes"
	default:
		return fmt.Errorf("unknown key: %s", key)
	}

	// Create directory if needed
	dir := filepath.Dir(cfgFile)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("creating config directory: %w", err)
	}

	// Write config
	newData, err := yaml.Marshal(fileCfg)
	if err != nil {
		return fmt.Errorf("marshaling config: %w", err)
	}

	content := "# VidVeil CLI Configuration\n\n" + string(newData)
	if err := os.WriteFile(cfgFile, []byte(content), 0600); err != nil {
		return fmt.Errorf("writing config file: %w", err)
	}

	fmt.Printf("Set %s = %s\n", key, value)
	return nil
}

func configGet(key string) error {
	var value string
	switch key {
	case "server.address":
		value = cfg.Server.Address
	case "server.token":
		value = cfg.Server.Token
	case "server.timeout":
		value = fmt.Sprintf("%d", cfg.Server.Timeout)
	case "output.format":
		value = cfg.Output.Format
	case "output.color":
		value = cfg.Output.Color
	case "tui.theme":
		value = cfg.TUI.Theme
	case "tui.show_hints":
		value = fmt.Sprintf("%v", cfg.TUI.ShowHints)
	default:
		return fmt.Errorf("unknown key: %s", key)
	}

	fmt.Println(value)
	return nil
}

func maskToken(token string) string {
	if token == "" {
		return "(not set)"
	}
	if len(token) <= 8 {
		return strings.Repeat("*", len(token))
	}
	return token[:4] + strings.Repeat("*", len(token)-8) + token[len(token)-4:]
}
