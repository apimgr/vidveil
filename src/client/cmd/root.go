// SPDX-License-Identifier: MIT
// AI.md PART 33: CLI Client - Root Command
package cmd

import (
	"fmt"
	"log"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/apimgr/vidveil/src/client/api"
	"github.com/apimgr/vidveil/src/client/paths"
	"github.com/apimgr/vidveil/src/common/display"
	"github.com/apimgr/vidveil/src/common/terminal"
	"github.com/apimgr/vidveil/src/config"
	"gopkg.in/yaml.v3"
)

// Build info (set by main.go)
var (
	ProjectName  = "vidveil"
	Version      = "dev"
	CommitID     = "unknown"
	BuildDate    = "unknown"
	OfficialSite = ""
	BinaryName   = "vidveil-cli"
)

// CLIServerConfig holds the persisted server configuration.
// It reads both the spec-aligned primary key and the legacy address key,
// but writes only the spec-aligned form.
type CLIServerConfig struct {
	Address    string
	Cluster    []string
	APIVersion string
	AdminPath  string
	Token      string
	Timeout    int
	Retry      int
	RetryDelay int
}

// CLIAuthConfig holds persisted authentication settings.
type CLIAuthConfig struct {
	Token     string `yaml:"token"`
	TokenFile string `yaml:"token_file"`
}

// CLIOutputConfig holds persisted output preferences.
type CLIOutputConfig struct {
	Format  string `yaml:"format"`
	Color   string `yaml:"color"`
	Pager   string `yaml:"pager"`
	Quiet   bool   `yaml:"quiet"`
	Verbose bool   `yaml:"verbose"`
}

// CLILoggingConfig holds persisted logging preferences.
type CLILoggingConfig struct {
	Level    string `yaml:"level"`
	File     string `yaml:"file"`
	MaxSize  string `yaml:"max_size"`
	MaxFiles int    `yaml:"max_files"`
}

// CLICacheConfig holds persisted cache preferences.
type CLICacheConfig struct {
	Enabled bool   `yaml:"enabled"`
	TTL     string `yaml:"ttl"`
	MaxSize string `yaml:"max_size"`
}

// CLITUIConfig holds persisted TUI preferences.
type CLITUIConfig struct {
	Enabled bool   `yaml:"enabled"`
	Theme   string `yaml:"theme"`
	Mouse   bool   `yaml:"mouse"`
	Unicode bool   `yaml:"unicode"`
}

// ParseCLITimeoutSeconds parses a cli.yml timeout value into whole seconds.
func ParseCLITimeoutSeconds(timeoutValue interface{}) (int, error) {
	switch parsedTimeoutValue := timeoutValue.(type) {
	case nil:
		return 0, nil
	case int:
		if parsedTimeoutValue <= 0 {
			return 0, nil
		}
		return parsedTimeoutValue, nil
	case int64:
		if parsedTimeoutValue <= 0 {
			return 0, nil
		}
		return int(parsedTimeoutValue), nil
	case float64:
		if parsedTimeoutValue <= 0 {
			return 0, nil
		}
		return int(parsedTimeoutValue), nil
	case string:
		trimmedTimeoutValue := strings.TrimSpace(parsedTimeoutValue)
		if trimmedTimeoutValue == "" {
			return 0, nil
		}

		timeoutDuration, err := time.ParseDuration(trimmedTimeoutValue)
		if err == nil {
			if timeoutDuration <= 0 {
				return 0, nil
			}
			return int(timeoutDuration / time.Second), nil
		}

		var timeoutSeconds int
		if _, scanErr := fmt.Sscanf(trimmedTimeoutValue, "%d", &timeoutSeconds); scanErr == nil && timeoutSeconds > 0 {
			return timeoutSeconds, nil
		}

		return 0, fmt.Errorf("invalid timeout value %q", parsedTimeoutValue)
	default:
		return 0, fmt.Errorf("unsupported timeout type %T", timeoutValue)
	}
}

// FormatCLITimeoutDuration converts whole seconds to the spec-aligned cli.yml duration form.
func FormatCLITimeoutDuration(timeoutSeconds int) string {
	if timeoutSeconds <= 0 {
		return ""
	}

	return fmt.Sprintf("%ds", timeoutSeconds)
}

func (cliServerConfig *CLIServerConfig) UnmarshalYAML(node *yaml.Node) error {
	var rawServerConfig struct {
		Primary    string      `yaml:"primary"`
		Cluster    []string    `yaml:"cluster"`
		Address    string      `yaml:"address"`
		APIVersion string      `yaml:"api_version"`
		AdminPath  string      `yaml:"admin_path"`
		Token      string      `yaml:"token"`
		Timeout    interface{} `yaml:"timeout"`
		Retry      int         `yaml:"retry"`
		RetryDelay interface{} `yaml:"retry_delay"`
	}

	if err := node.Decode(&rawServerConfig); err != nil {
		return err
	}

	cliServerConfig.Address = rawServerConfig.Primary
	if cliServerConfig.Address == "" {
		cliServerConfig.Address = rawServerConfig.Address
	}
	cliServerConfig.Cluster = rawServerConfig.Cluster
	cliServerConfig.APIVersion = rawServerConfig.APIVersion
	cliServerConfig.AdminPath = rawServerConfig.AdminPath
	cliServerConfig.Token = rawServerConfig.Token
	timeoutSeconds, err := ParseCLITimeoutSeconds(rawServerConfig.Timeout)
	if err != nil {
		return err
	}
	cliServerConfig.Timeout = timeoutSeconds
	cliServerConfig.Retry = rawServerConfig.Retry
	retryDelaySeconds, err := ParseCLITimeoutSeconds(rawServerConfig.RetryDelay)
	if err != nil {
		return err
	}
	cliServerConfig.RetryDelay = retryDelaySeconds

	return nil
}

func (cliServerConfig CLIServerConfig) MarshalYAML() (interface{}, error) {
	return struct {
		Primary    string `yaml:"primary,omitempty"`
		Cluster    []string `yaml:"cluster"`
		APIVersion string `yaml:"api_version,omitempty"`
		AdminPath  string `yaml:"admin_path,omitempty"`
		Token      string `yaml:"token,omitempty"`
		Timeout    string `yaml:"timeout,omitempty"`
		Retry      int    `yaml:"retry,omitempty"`
		RetryDelay string `yaml:"retry_delay,omitempty"`
	}{
		Primary:    cliServerConfig.Address,
		Cluster:    cliServerConfig.Cluster,
		APIVersion: cliServerConfig.APIVersion,
		AdminPath:  cliServerConfig.AdminPath,
		Token:      cliServerConfig.Token,
		Timeout:    FormatCLITimeoutDuration(cliServerConfig.Timeout),
		Retry:      cliServerConfig.Retry,
		RetryDelay: FormatCLITimeoutDuration(cliServerConfig.RetryDelay),
	}, nil
}

// CLIConfig holds CLI configuration
// Per AI.md PART 1: Type names MUST be specific - "Config" is ambiguous
type CLIConfig struct {
	Server CLIServerConfig `yaml:"server"`
	Auth   CLIAuthConfig   `yaml:"auth"`
	Token  string          `yaml:"token,omitempty"`
	Output CLIOutputConfig `yaml:"output"`
	Logging CLILoggingConfig `yaml:"logging"`
	Cache   CLICacheConfig   `yaml:"cache"`
	TUI    CLITUIConfig    `yaml:"tui"`
	Debug  bool            `yaml:"debug"`
}

// Global flags per AI.md PART 33
// Short flags only for -h (help) and -v (version)
// Per AI.md PART 1: Variable names MUST reveal intent
var (
	cliConfigFilePath string
	serverAddressFlag string
	apiTokenFlag      string
	tokenFilePath     string
	outputFormatFlag  string
	// Per AI.md PART 8: --color flag (always, never, auto)
	colorFlag             string
	requestTimeoutSeconds int
	debugModeEnabled      bool
	debugFlagProvided     bool
)

// Global config and client
// Per AI.md PART 1: Variable names MUST be specific
var (
	cliConfig                      *CLIConfig
	apiClient                      *api.APIClient
	cliConfigHasSavedServerAddress bool
	cliLogOutputFile               *os.File
)

// ExecuteCLI runs the CLI application
// Per AI.md PART 33: Auto-detect TUI mode when interactive terminal + no command
func ExecuteCLI() error {
	args := os.Args[1:]

	// Parse global flags first
	args = ParseCLIGlobalFlags(args)

	// Per AI.md PART 33: After parsing flags, ensure the standard client directories exist.
	if err := paths.EnsureClientDirs(); err != nil {
		return fmt.Errorf("creating client directories: %w", err)
	}

	// Load config
	if err := LoadCLIConfigFromFile(); err != nil {
		return err
	}
	if err := InitializeCLILogging(); err != nil {
		return err
	}

	// Initialize API client
	InitAPIClient()

	// Per AI.md PART 33: Automatic Mode Detection using display.DetectDisplayEnv()
	// - Interactive terminal + no command = TUI mode
	// - Interactive terminal + only config flags = TUI mode
	// - Interactive terminal + command provided = CLI mode
	// - Piped/redirected output = Plain output (no TUI)
	if len(args) == 0 {
		// Use display.DetectDisplayEnv() from src/common/display for mode detection
		displayEnv := display.DetectDisplayEnv()
		if displayEnv.Mode == display.DisplayModeTUI && displayEnv.IsTerminal && IsCLITUIEnabled() {
			// Per AI.md PART 33: CLI First-Run Flow
			// Run setup wizard if no server configured
			if !IsServerConfigured() {
				if err := RunSetupWizard(); err != nil {
					return err
				}
				// Reload config after setup wizard
				if err := LoadCLIConfigFromFile(); err != nil {
					return err
				}
				if err := InitializeCLILogging(); err != nil {
					return err
				}
				InitAPIClient()
			}
			return RunInteractiveTUI()
		}
		// Non-interactive or headless: show help
		PrintCLIHelpMessage()
		return nil
	}

	// Route to command
	// Per AI.md PART 33: No tui command (auto-launches), no config command (edit cli.yml directly)
	switch args[0] {
	case "search":
		// Check server connection before search commands
		if healthy, err := CheckServerConnection(); !healthy && cliConfig.Server.Address != "" {
			PrintConnectionWarning(err)
		}
		return RunSearchCommand(args[1:])
	case "engines":
		// Check server connection before engines command
		if healthy, err := CheckServerConnection(); !healthy && cliConfig.Server.Address != "" {
			PrintConnectionWarning(err)
		}
		return RunEnginesCommand(args[1:])
	case "bangs":
		// Check server connection before bangs command
		if healthy, err := CheckServerConnection(); !healthy && cliConfig.Server.Address != "" {
			PrintConnectionWarning(err)
		}
		return RunBangsCommand(args[1:])
	case "login":
		return RunLoginCommand(args[1:])
	case "probe":
		// Probe command tests connection itself
		return RunProbeCommand(args[1:])
	default:
		// Treat first arg as search query
		// Check server connection before search
		if healthy, err := CheckServerConnection(); !healthy && cliConfig.Server.Address != "" {
			PrintConnectionWarning(err)
		}
		return RunSearchCommand(args)
	}

	return nil
}

// ResolveCLILogFilePath returns the configured CLI log file path or the default path.
func ResolveCLILogFilePath() string {
	if cliConfig != nil {
		configuredLogFilePath := strings.TrimSpace(cliConfig.Logging.File)
		if configuredLogFilePath != "" {
			return configuredLogFilePath
		}
	}

	return paths.LogFile()
}

// InitializeCLILogging creates and opens the CLI log file, then binds the standard logger to it.
func InitializeCLILogging() error {
	logFilePath := ResolveCLILogFilePath()
	if logFilePath == "" {
		return fmt.Errorf("CLI log file path is required")
	}

	logDirPath := filepath.Dir(logFilePath)
	if err := os.MkdirAll(logDirPath, 0700); err != nil {
		return fmt.Errorf("creating CLI log directory: %w", err)
	}
	if err := os.Chmod(logDirPath, 0700); err != nil {
		return fmt.Errorf("setting CLI log directory permissions: %w", err)
	}
	if err := paths.EnsurePathOwnership(logDirPath); err != nil {
		return fmt.Errorf("verifying CLI log directory ownership: %w", err)
	}

	logFile, err := os.OpenFile(logFilePath, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0600)
	if err != nil {
		return fmt.Errorf("opening CLI log file: %w", err)
	}
	if err := os.Chmod(logFilePath, 0600); err != nil {
		_ = logFile.Close()
		return fmt.Errorf("setting CLI log file permissions: %w", err)
	}
	if err := paths.EnsurePathOwnership(logFilePath); err != nil {
		_ = logFile.Close()
		return fmt.Errorf("verifying CLI log file ownership: %w", err)
	}

	CloseCLILogging()
	cliLogOutputFile = logFile
	log.SetOutput(logFile)
	log.SetFlags(log.LstdFlags)

	return nil
}

// CloseCLILogging closes the current CLI log file when one is open.
func CloseCLILogging() {
	if cliLogOutputFile != nil {
		_ = cliLogOutputFile.Close()
		cliLogOutputFile = nil
	}
}

// ParseCLIGlobalFlags parses global CLI flags
// Per AI.md PART 33: Short flags only for -h (help) and -v (version)
// Per AI.md PART 33: NO --tui/--cli/--gui flags - UI mode is auto-detected
// Note: -h/--help and -v/--version only trigger exit if they appear BEFORE any command
func ParseCLIGlobalFlags(args []string) []string {
	var remaining []string
	commandSeen := false
	i := 0
	for i < len(args) {
		flagName, _, _ := ParseCLILongFlagArgument(args[i])

		switch flagName {
		case "--shell":
			shellCommandArgs := make([]string, 0, len(args)-i)
			_, inlineFlagValue, hasInlineFlagValue := ParseCLILongFlagArgument(args[i])
			if hasInlineFlagValue {
				shellCommandArgs = append(shellCommandArgs, inlineFlagValue)
			}
			if i+1 < len(args) {
				shellCommandArgs = append(shellCommandArgs, args[i+1:]...)
			}
			if err := RunShellCommand(shellCommandArgs); err != nil {
				fmt.Fprintln(os.Stderr, err)
				os.Exit(1)
			}
			os.Exit(0)
		case "--server":
			flagValue, nextIndex, hasFlagValue := ReadCLILongFlagValue(args, i)
			if hasFlagValue {
				serverAddressFlag = flagValue
				i = nextIndex + 1
			} else {
				i++
			}
		case "--token":
			flagValue, nextIndex, hasFlagValue := ReadCLILongFlagValue(args, i)
			if hasFlagValue {
				apiTokenFlag = flagValue
				i = nextIndex + 1
			} else {
				i++
			}
		case "--token-file":
			flagValue, nextIndex, hasFlagValue := ReadCLILongFlagValue(args, i)
			if hasFlagValue {
				tokenFilePath = flagValue
				i = nextIndex + 1
			} else {
				i++
			}
		case "--output":
			flagValue, nextIndex, hasFlagValue := ReadCLILongFlagValue(args, i)
			if hasFlagValue {
				outputFormatFlag = flagValue
				i = nextIndex + 1
			} else {
				i++
			}
		case "--config":
			flagValue, nextIndex, hasFlagValue := ReadCLILongFlagValue(args, i)
			if hasFlagValue {
				cliConfigFilePath = paths.ResolveConfigFilePath(flagValue)
				i = nextIndex + 1
			} else {
				i++
			}
		case "--color":
			// Per AI.md PART 8: --color {always|never|auto}
			flagValue, nextIndex, hasFlagValue := ReadCLILongFlagValue(args, i)
			if hasFlagValue {
				colorFlag = flagValue
				i = nextIndex + 1
			} else {
				i++
			}
		case "--timeout":
			flagValue, nextIndex, hasFlagValue := ReadCLILongFlagValue(args, i)
			if hasFlagValue {
				fmt.Sscanf(flagValue, "%d", &requestTimeoutSeconds)
				i = nextIndex + 1
			} else {
				i++
			}
		case "--debug":
			debugModeEnabled = true
			debugFlagProvided = true
			i++
		case "-h", "--help":
			if !commandSeen {
				PrintCLIHelpMessage()
				os.Exit(0)
			}
			remaining = append(remaining, args[i])
			i++
		case "-v", "--version":
			if !commandSeen {
				PrintCLIVersionInfo()
				os.Exit(0)
			}
			remaining = append(remaining, args[i])
			i++
		default:
			remaining = append(remaining, args[i])
			// First non-flag argument is a command
			if !commandSeen && !strings.HasPrefix(args[i], "-") {
				commandSeen = true
			}
			i++
		}
	}
	return remaining
}

// LoadCLIConfigFromFile loads CLI configuration from file
// Per AI.md PART 1: Function names MUST reveal intent - "loadConfig" is ambiguous
func LoadCLIConfigFromFile() error {
	// Initialize config
	cliConfig = &CLIConfig{}
	cliConfigHasSavedServerAddress = false

	// Default config
	cliConfig.Server.APIVersion = "v1"
	cliConfig.Server.AdminPath = "admin"
	cliConfig.Server.Timeout = 30
	cliConfig.Server.Retry = 3
	cliConfig.Server.RetryDelay = 1
	cliConfig.Output.Format = "table"
	cliConfig.Output.Color = "auto"
	cliConfig.Output.Pager = "auto"
	cliConfig.Logging.Level = "warn"
	cliConfig.Logging.MaxSize = "10MB"
	cliConfig.Logging.MaxFiles = 5
	cliConfig.Cache.Enabled = true
	cliConfig.Cache.TTL = "5m"
	cliConfig.Cache.MaxSize = "100MB"
	cliConfig.TUI.Enabled = true
	cliConfig.TUI.Theme = "dark"
	cliConfig.TUI.Mouse = true
	cliConfig.TUI.Unicode = true
	cliConfig.Debug = false

	// Determine config path per AI.md PART 33
	// Uses paths module for OS-specific resolution
	cliConfigFilePath = GetCLIConfigFilePath()

	// Read config file if exists
	data, err := os.ReadFile(cliConfigFilePath)
	if err == nil {
		// Ignore unmarshal errors - use defaults if config is invalid
		_ = yaml.Unmarshal(data, cliConfig)
	} else if os.IsNotExist(err) {
		if err := WriteCLIConfigFile(*cliConfig, cliConfigFilePath); err != nil {
			return err
		}
	} else {
		return fmt.Errorf("reading config file: %w", err)
	}
	if err := EnsureCLIConfigFilePermissions(cliConfigFilePath); err != nil {
		return err
	}

	configAuthToken := strings.TrimSpace(cliConfig.Auth.Token)
	legacyTopLevelToken := strings.TrimSpace(cliConfig.Token)
	legacyServerToken := strings.TrimSpace(cliConfig.Server.Token)
	cliConfig.Server.Token = ""
	if cliConfig.Auth.TokenFile != "" {
		if tokenFromFile, err := ReadCLIAuthTokenFile(cliConfig.Auth.TokenFile); err == nil {
			cliConfig.Server.Token = tokenFromFile
		}
	}
	if cliConfig.Server.Token == "" {
		switch {
		case configAuthToken != "":
			cliConfig.Server.Token = configAuthToken
		case legacyTopLevelToken != "":
			cliConfig.Server.Token = legacyTopLevelToken
		case legacyServerToken != "":
			cliConfig.Server.Token = legacyServerToken
		}
	}
	cliConfigHasSavedServerAddress = cliConfig.Server.Address != ""
	fileCLIConfig := *cliConfig
	debugModeEnabled = cliConfig.Debug

	// Per AI.md PART 33: Token priority
	// 1. --token flag (highest)
	// 2. --token-file flag
	// 3. VIDVEIL_TOKEN env var (VIDVEIL_CLI_TOKEN accepted as compatibility alias)
	// 4. config file (already loaded above)
	// 5. token file at default path (lowest)

	// Default token file location
	if cliConfig.Server.Token == "" {
		defaultTokenFilePath := paths.TokenFile()
		if err := EnsureCLIDefaultTokenFilePermissions(defaultTokenFilePath); err != nil {
			return err
		}
		if tokenFromFile, err := ReadCLIAuthTokenFile(defaultTokenFilePath); err == nil {
			cliConfig.Server.Token = tokenFromFile
		}
	}

	if err := ApplyCLIEnvironmentOverrides(); err != nil {
		return err
	}

	// Token file flag overrides environment variables and config file values.
	if tokenFilePath != "" {
		if tokenFromFile, err := ReadCLIAuthTokenFile(tokenFilePath); err == nil {
			cliConfig.Server.Token = tokenFromFile
		}
	}

	// Command-line flags override everything (highest priority)
	if serverAddressFlag != "" {
		if err := ValidateCLIServerURL(serverAddressFlag); err != nil {
			return fmt.Errorf("invalid --server URL: %w", err)
		}
	if fileCLIConfig.Server.Address == "" {
			fileCLIConfig.Server.Address = serverAddressFlag
			if err := WriteCLIConfigFile(fileCLIConfig, cliConfigFilePath); err != nil {
				return err
			}
		}
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
	// Per AI.md PART 8: --color flag overrides config
	if colorFlag != "" {
		cliConfig.Output.Color = colorFlag
	}
	if debugFlagProvided {
		debugModeEnabled = true
	}

	// Per AI.md PART 8: Initialize color mode
	// Priority: CLI flag > config > NO_COLOR env > auto-detect
	terminal.SetColorMode(terminal.ParseColorFlag(cliConfig.Output.Color))

	return nil
}

// EnsureCLIConfigFilePermissions normalizes the selected cli.yml file to user-only access.
func EnsureCLIConfigFilePermissions(configFilePath string) error {
	if err := os.Chmod(configFilePath, 0600); err != nil {
		return fmt.Errorf("setting CLI config file permissions: %w", err)
	}
	if err := paths.EnsurePathOwnership(configFilePath); err != nil {
		return fmt.Errorf("verifying CLI config file ownership: %w", err)
	}

	return nil
}

// EnsureCLIDefaultTokenFilePermissions normalizes the default token file to user-only access.
func EnsureCLIDefaultTokenFilePermissions(tokenFilePath string) error {
	if _, err := os.Stat(tokenFilePath); err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return fmt.Errorf("stating CLI token file: %w", err)
	}
	if err := os.Chmod(tokenFilePath, 0600); err != nil {
		return fmt.Errorf("setting CLI token file permissions: %w", err)
	}
	if err := paths.EnsurePathOwnership(tokenFilePath); err != nil {
		return fmt.Errorf("verifying CLI token file ownership: %w", err)
	}

	return nil
}

// WriteCLIDefaultTokenFile writes the default CLI token file with user-only access.
func WriteCLIDefaultTokenFile(tokenValue string) error {
	tokenFilePath := paths.TokenFile()
	tokenDirPath := filepath.Dir(tokenFilePath)
	if err := os.MkdirAll(tokenDirPath, 0700); err != nil {
		return fmt.Errorf("creating token directory: %w", err)
	}
	if err := os.Chmod(tokenDirPath, 0700); err != nil {
		return fmt.Errorf("setting token directory permissions: %w", err)
	}
	if err := paths.EnsurePathOwnership(tokenDirPath); err != nil {
		return fmt.Errorf("verifying token directory ownership: %w", err)
	}
	if err := os.WriteFile(tokenFilePath, []byte(tokenValue), 0600); err != nil {
		return fmt.Errorf("writing token file: %w", err)
	}
	if err := EnsureCLIDefaultTokenFilePermissions(tokenFilePath); err != nil {
		return err
	}

	return nil
}

// ReadCLIAuthTokenFile reads a token file and trims surrounding whitespace.
func ReadCLIAuthTokenFile(tokenFilePath string) (string, error) {
	tokenData, err := os.ReadFile(tokenFilePath)
	if err != nil {
		return "", err
	}

	return strings.TrimSpace(string(tokenData)), nil
}

// GetCLIConfigFilePath returns the resolved CLI config file path.
func GetCLIConfigFilePath() string {
	if cliConfigFilePath == "" {
		cliConfigFilePath = paths.ResolveConfigFilePath("")
	}

	return cliConfigFilePath
}

// GetCLIAuthTokenFromEnv returns the canonical CLI token environment variable,
// falling back to the legacy alias when needed.
func GetCLIAuthTokenFromEnv() string {
	if envToken := os.Getenv("VIDVEIL_TOKEN"); envToken != "" {
		return envToken
	}

	return os.Getenv("VIDVEIL_CLI_TOKEN")
}

// GetCLIServerAddressFromEnv returns the canonical CLI server environment variable,
// falling back to the legacy alias when needed.
func GetCLIServerAddressFromEnv() string {
	if envServerAddress := os.Getenv("VIDVEIL_SERVER_PRIMARY"); envServerAddress != "" {
		return envServerAddress
	}

	return os.Getenv("VIDVEIL_SERVER")
}

// GetCLIOutputFormatFromEnv returns the canonical output format env var,
// falling back to the generic alias when needed.
func GetCLIOutputFormatFromEnv() string {
	if envOutputFormat := os.Getenv("VIDVEIL_OUTPUT_FORMAT"); envOutputFormat != "" {
		return envOutputFormat
	}

	return os.Getenv("VIDVEIL_FORMAT")
}

// GetCLIOutputColorFromEnv returns the canonical output color env var,
// falling back to the generic alias when needed.
func GetCLIOutputColorFromEnv() string {
	if envOutputColor := os.Getenv("VIDVEIL_OUTPUT_COLOR"); envOutputColor != "" {
		return envOutputColor
	}

	return os.Getenv("VIDVEIL_COLOR")
}

// GetCLITimeoutSecondsFromEnv returns the canonical timeout env var,
// falling back to the generic alias when needed.
func GetCLITimeoutSecondsFromEnv() (int, error) {
	timeoutEnvValue := os.Getenv("VIDVEIL_SERVER_TIMEOUT")
	if timeoutEnvValue == "" {
		timeoutEnvValue = os.Getenv("VIDVEIL_TIMEOUT")
	}
	if timeoutEnvValue == "" {
		return 0, nil
	}

	var timeoutSeconds int
	if _, err := fmt.Sscanf(timeoutEnvValue, "%d", &timeoutSeconds); err != nil || timeoutSeconds <= 0 {
		return 0, fmt.Errorf("invalid timeout value %q", timeoutEnvValue)
	}

	return timeoutSeconds, nil
}

// ApplyCLIEnvironmentOverrides applies env vars after config load but before flags.
func ApplyCLIEnvironmentOverrides() error {
	if envServerAddress := GetCLIServerAddressFromEnv(); envServerAddress != "" {
		if err := ValidateCLIServerURL(envServerAddress); err != nil {
			return fmt.Errorf("invalid server environment variable: %w", err)
		}
		cliConfig.Server.Address = envServerAddress
	}

	if envToken := GetCLIAuthTokenFromEnv(); envToken != "" {
		cliConfig.Server.Token = envToken
	}

	if envOutputFormat := GetCLIOutputFormatFromEnv(); envOutputFormat != "" {
		cliConfig.Output.Format = envOutputFormat
	}

	if envOutputColor := GetCLIOutputColorFromEnv(); envOutputColor != "" {
		cliConfig.Output.Color = envOutputColor
	}

	timeoutSeconds, err := GetCLITimeoutSecondsFromEnv()
	if err != nil {
		return err
	}
	if timeoutSeconds > 0 {
		cliConfig.Server.Timeout = timeoutSeconds
	}

	if cliConfig.Server.Address == "" {
		cliConfig.Server.Address = GetCLIDefaultServerAddress()
	}

	if debugEnvValue := os.Getenv("VIDVEIL_DEBUG"); debugEnvValue != "" {
		debugModeEnabled = config.ParseBool(debugEnvValue)
	}

	return nil
}

// ValidateCLIServerURL verifies that a server URL is absolute and uses http or https.
func ValidateCLIServerURL(serverURL string) error {
	if serverURL == "" {
		return fmt.Errorf("server URL is required")
	}

	parsedServerURL, err := url.ParseRequestURI(serverURL)
	if err != nil {
		return fmt.Errorf("parse server URL: %w", err)
	}
	if parsedServerURL.Scheme != "http" && parsedServerURL.Scheme != "https" {
		return fmt.Errorf("URL must start with http:// or https://")
	}
	if parsedServerURL.Host == "" {
		return fmt.Errorf("server URL must include a host")
	}

	return nil
}

// GetCLIDefaultServerAddress returns the compiled official site when available.
func GetCLIDefaultServerAddress() string {
	return strings.TrimSpace(OfficialSite)
}

// WriteCLIConfigFile writes the CLI config file to disk using the spec-aligned yaml shape.
func WriteCLIConfigFile(fileCLIConfig CLIConfig, configFilePath string) error {
	configDirPath := filepath.Dir(configFilePath)
	if err := os.MkdirAll(configDirPath, 0700); err != nil {
		return fmt.Errorf("creating config directory: %w", err)
	}
	if err := os.Chmod(configDirPath, 0700); err != nil {
		return fmt.Errorf("setting config directory permissions: %w", err)
	}
	if err := paths.EnsurePathOwnership(configDirPath); err != nil {
		return fmt.Errorf("verifying config directory ownership: %w", err)
	}

	if fileCLIConfig.Server.APIVersion == "" {
		fileCLIConfig.Server.APIVersion = "v1"
	}
	if fileCLIConfig.Server.AdminPath == "" {
		fileCLIConfig.Server.AdminPath = "admin"
	}
	if fileCLIConfig.Server.Retry == 0 {
		fileCLIConfig.Server.Retry = 3
	}
	if fileCLIConfig.Server.RetryDelay == 0 {
		fileCLIConfig.Server.RetryDelay = 1
	}
	if fileCLIConfig.Output.Pager == "" {
		fileCLIConfig.Output.Pager = "auto"
	}
	if fileCLIConfig.Logging.Level == "" {
		fileCLIConfig.Logging.Level = "warn"
	}
	if fileCLIConfig.Logging.MaxSize == "" {
		fileCLIConfig.Logging.MaxSize = "10MB"
	}
	if fileCLIConfig.Logging.MaxFiles == 0 {
		fileCLIConfig.Logging.MaxFiles = 5
	}
	if fileCLIConfig.Cache.TTL == "" {
		fileCLIConfig.Cache.TTL = "5m"
	}
	if fileCLIConfig.Cache.MaxSize == "" {
		fileCLIConfig.Cache.MaxSize = "100MB"
	}
	if fileCLIConfig.Auth.Token == "" {
		if fileCLIConfig.Token != "" {
			fileCLIConfig.Auth.Token = fileCLIConfig.Token
		} else if fileCLIConfig.Server.Token != "" {
			fileCLIConfig.Auth.Token = fileCLIConfig.Server.Token
		}
	}
	fileCLIConfig.Server.Token = ""
	fileCLIConfig.Token = ""

	data, err := yaml.Marshal(fileCLIConfig)
	if err != nil {
		return fmt.Errorf("marshaling config: %w", err)
	}

	content := "# VidVeil CLI Configuration\n# Edit this file or use --server/--token flags\n\n" + string(data)
	if err := os.WriteFile(configFilePath, []byte(content), 0600); err != nil {
		return fmt.Errorf("writing config file: %w", err)
	}

	return nil
}

// InitAPIClient initializes the API client
// Per AI.md PART 1: Function names MUST reveal intent - "initClient" is ambiguous
func InitAPIClient() {
	resolvedServerAddress := ResolveCLIReachableServerAddress()
	if resolvedServerAddress != "" {
		cliConfig.Server.Address = resolvedServerAddress
	}
	apiClient = api.NewAPIClient(cliConfig.Server.Address, cliConfig.Server.Token, cliConfig.Server.Timeout, cliConfig.Server.APIVersion)
	// Per AI.md PART 33: User-Agent uses hardcoded project name with version
	apiClient.SetUserAgent(Version)
	StartCLIBackgroundDiscovery()
}

// CheckServerConnection checks if the server is reachable
// Per AI.md PART 1: Function names MUST reveal intent
// Returns true if healthy, false otherwise (with optional error)
func CheckServerConnection() (bool, error) {
	if apiClient == nil || cliConfig == nil || cliConfig.Server.Address == "" {
		return false, nil
	}
	return apiClient.Health()
}

// ResolveCLIReachableServerAddress returns the first healthy server from primary then cluster nodes.
func ResolveCLIReachableServerAddress() string {
	if cliConfig == nil {
		return ""
	}

	serverCandidates := make([]string, 0, 1+len(cliConfig.Server.Cluster))
	if cliConfig.Server.Address != "" {
		serverCandidates = append(serverCandidates, cliConfig.Server.Address)
	}
	serverCandidates = append(serverCandidates, cliConfig.Server.Cluster...)

	seenServerAddress := make(map[string]struct{}, len(serverCandidates))
	for _, serverAddress := range serverCandidates {
		trimmedServerAddress := strings.TrimSpace(serverAddress)
		if trimmedServerAddress == "" {
			continue
		}
		if _, alreadySeen := seenServerAddress[trimmedServerAddress]; alreadySeen {
			continue
		}
		seenServerAddress[trimmedServerAddress] = struct{}{}

		testClient := api.NewAPIClient(trimmedServerAddress, cliConfig.Server.Token, cliConfig.Server.Timeout, cliConfig.Server.APIVersion)
		isHealthy, err := testClient.Health()
		if err == nil && isHealthy {
			return trimmedServerAddress
		}
	}

	return cliConfig.Server.Address
}

// StartCLIBackgroundDiscovery refreshes server connection settings from /api/autodiscover.
func StartCLIBackgroundDiscovery() {
	if apiClient == nil || cliConfig == nil || cliConfig.Server.Address == "" {
		return
	}

	discoveryClient := apiClient
	configFilePath := GetCLIConfigFilePath()
	fileCLIConfig := *cliConfig
	go func() {
		discoveredConfig, err := DiscoverCLIServerConfig(discoveryClient, fileCLIConfig)
		if err != nil {
			return
		}
		_ = WriteCLIConfigFile(discoveredConfig, configFilePath)
	}()
}

// DiscoverCLIServerConfig merges autodiscover settings into the current CLI config.
func DiscoverCLIServerConfig(discoveryClient *api.APIClient, fileCLIConfig CLIConfig) (CLIConfig, error) {
	discoveryResponse, err := discoveryClient.Autodiscover()
	if err != nil {
		return fileCLIConfig, err
	}

	if discoveryResponse.Primary != "" {
		if err := ValidateCLIServerURL(discoveryResponse.Primary); err == nil {
			fileCLIConfig.Server.Address = discoveryResponse.Primary
		}
	}

	filteredClusterNodes := make([]string, 0, len(discoveryResponse.Cluster))
	for _, clusterNodeAddress := range discoveryResponse.Cluster {
		if err := ValidateCLIServerURL(clusterNodeAddress); err == nil {
			filteredClusterNodes = append(filteredClusterNodes, clusterNodeAddress)
		}
	}
	fileCLIConfig.Server.Cluster = filteredClusterNodes

	if discoveryResponse.APIVersion != "" {
		fileCLIConfig.Server.APIVersion = discoveryResponse.APIVersion
	}
	if discoveryResponse.Timeout > 0 {
		fileCLIConfig.Server.Timeout = discoveryResponse.Timeout
	}
	if discoveryResponse.Retry > 0 {
		fileCLIConfig.Server.Retry = discoveryResponse.Retry
	}
	if discoveryResponse.RetryDelay > 0 {
		fileCLIConfig.Server.RetryDelay = discoveryResponse.RetryDelay
	}

	return fileCLIConfig, nil
}

// PrintConnectionWarning prints a warning if server is unreachable
// Per AI.md PART 1: Function names MUST reveal intent
// Per AI.md PART 8: Respects NO_COLOR
func PrintConnectionWarning(err error) {
	fmt.Fprintf(os.Stderr, "%s Cannot reach server at %s\n",
		terminal.WarningIcon(), cliConfig.Server.Address)
	if err != nil && debugModeEnabled {
		fmt.Fprintf(os.Stderr, "  Error: %v\n", err)
	}
}

// PrintCLIHelpMessage prints CLI help message
// Per AI.md PART 1: Function names MUST reveal intent - "printHelp" is ambiguous
func PrintCLIHelpMessage() {
	fmt.Printf(`%s %s - CLI client for VidVeil video search

Usage:
  %s [args] [flags]
  %s                    # TUI mode (no args)

Flags:
  -h, --help                        Show help
  -v, --version                     Show version
      --shell completions [SHELL]   Print shell completions (auto-detect if SHELL omitted)
      --shell init [SHELL]          Print shell init command (auto-detect if SHELL omitted)
      --shell --help                Show shell integration help
      --server URL                  Server URL (default: %s)
      --token TOKEN                 API token for authentication
      --token-file FILE             Read token from file
      --config FILE                 Config file name or path (default: cli.yml)
      --output FORMAT               Output format: json, yaml, csv, table, plain (default: table)
      --timeout SECONDS             Request timeout in seconds (default: 30)
      --debug                       Debug output
      --color {always|never|auto}   Color output (default: auto)

Commands:
  search <query>                    Search for videos
  engines                           List available search engines
  bangs                             List bang shortcuts
  probe                             Test engine availability
  login                             Save API token for future use

Shells: bash, zsh, fish, sh, dash, ksh, powershell, pwsh

Environment Variables:
  VIDVEIL_SERVER_PRIMARY            Server address (canonical)
  VIDVEIL_SERVER                    Server address compatibility alias
  VIDVEIL_TOKEN                     API token

Run without arguments for interactive TUI mode.

Examples:
  %s "search term"                Search for videos
  %s search --limit 20 "test"     Search with limit
  %s engines --enabled            List enabled engines
  %s bangs                        List bang shortcuts
  %s --output json "query"        Output as JSON
  %s login                        Save token interactively
  %s --shell completions bash     Output bash completions

Run '%s <command> --help' for more information about a command.
`, BinaryName, Version, BinaryName, BinaryName, GetCLIHelpServerDefault(), BinaryName, BinaryName, BinaryName, BinaryName, BinaryName, BinaryName, BinaryName, BinaryName)
}

func GetCLIHelpServerDefault() string {
	defaultServerAddress := GetCLIDefaultServerAddress()
	if defaultServerAddress == "" {
		return "from config"
	}

	return defaultServerAddress
}

// IsCLITUIEnabled reports whether interactive TUI autostart is enabled.
func IsCLITUIEnabled() bool {
	return cliConfig == nil || cliConfig.TUI.Enabled
}

// PrintCLIVersionInfo prints CLI version information
// Per AI.md PART 1: Function names MUST reveal intent - "printVersion" is ambiguous
func PrintCLIVersionInfo() {
	// Per AI.md PART 33: CLI --version format
	fmt.Printf("%s %s (%s) built %s\n", BinaryName, Version, CommitID, BuildDate)
}
