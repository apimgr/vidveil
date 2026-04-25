// SPDX-License-Identifier: MIT
package cmd

import (
	"bytes"
	"log"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/apimgr/vidveil/src/client/api"
	"github.com/apimgr/vidveil/src/client/paths"
	tea "github.com/charmbracelet/bubbletea"
)

func TestParseCLILongFlagArgument(t *testing.T) {
	testCases := []struct {
		name         string
		argument     string
		wantFlagName string
		wantValue    string
		wantHasValue bool
	}{
		{
			name:         "space syntax",
			argument:     "--server",
			wantFlagName: "--server",
			wantValue:    "",
			wantHasValue: false,
		},
		{
			name:         "equals syntax",
			argument:     "--server=https://x.scour.li",
			wantFlagName: "--server",
			wantValue:    "https://x.scour.li",
			wantHasValue: true,
		},
		{
			name:         "non flag argument",
			argument:     "search",
			wantFlagName: "search",
			wantValue:    "",
			wantHasValue: false,
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			gotFlagName, gotValue, gotHasValue := ParseCLILongFlagArgument(testCase.argument)
			if gotFlagName != testCase.wantFlagName {
				t.Fatalf("flag name = %q, want %q", gotFlagName, testCase.wantFlagName)
			}
			if gotValue != testCase.wantValue {
				t.Fatalf("flag value = %q, want %q", gotValue, testCase.wantValue)
			}
			if gotHasValue != testCase.wantHasValue {
				t.Fatalf("has value = %t, want %t", gotHasValue, testCase.wantHasValue)
			}
		})
	}
}

func TestReadCLILongFlagValue(t *testing.T) {
	testCases := []struct {
		name          string
		args          []string
		index         int
		wantValue     string
		wantNextIndex int
		wantHasValue  bool
	}{
		{
			name:          "reads equals syntax",
			args:          []string{"--token=abc123"},
			index:         0,
			wantValue:     "abc123",
			wantNextIndex: 0,
			wantHasValue:  true,
		},
		{
			name:          "reads following argument",
			args:          []string{"--token", "abc123"},
			index:         0,
			wantValue:     "abc123",
			wantNextIndex: 1,
			wantHasValue:  true,
		},
		{
			name:          "missing value",
			args:          []string{"--token"},
			index:         0,
			wantValue:     "",
			wantNextIndex: 0,
			wantHasValue:  false,
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			gotValue, gotNextIndex, gotHasValue := ReadCLILongFlagValue(testCase.args, testCase.index)
			if gotValue != testCase.wantValue {
				t.Fatalf("flag value = %q, want %q", gotValue, testCase.wantValue)
			}
			if gotNextIndex != testCase.wantNextIndex {
				t.Fatalf("next index = %d, want %d", gotNextIndex, testCase.wantNextIndex)
			}
			if gotHasValue != testCase.wantHasValue {
				t.Fatalf("has value = %t, want %t", gotHasValue, testCase.wantHasValue)
			}
		})
	}
}

func TestParseCLIGlobalFlagsSupportsEqualsSyntax(t *testing.T) {
	cliConfigFilePath = ""
	serverAddressFlag = ""
	apiTokenFlag = ""
	tokenFilePath = ""
	outputFormatFlag = ""
	colorFlag = ""
	requestTimeoutSeconds = 0
	debugModeEnabled = false
	debugFlagProvided = false

	remainingArgs := ParseCLIGlobalFlags([]string{
		"--server=https://x.scour.li",
		"--token=abc123",
		"--token-file=/tmp/token",
		"--output=json",
		"--config=/tmp/cli.yml",
		"--color=never",
		"--timeout=45",
		"--debug",
		"search",
		"demo",
	})

	if serverAddressFlag != "https://x.scour.li" {
		t.Fatalf("server address = %q, want %q", serverAddressFlag, "https://x.scour.li")
	}
	if apiTokenFlag != "abc123" {
		t.Fatalf("API token = %q, want %q", apiTokenFlag, "abc123")
	}
	if tokenFilePath != "/tmp/token" {
		t.Fatalf("token file = %q, want %q", tokenFilePath, "/tmp/token")
	}
	if outputFormatFlag != "json" {
		t.Fatalf("output format = %q, want %q", outputFormatFlag, "json")
	}
	if cliConfigFilePath != "/tmp/cli.yml" {
		t.Fatalf("config file = %q, want %q", cliConfigFilePath, "/tmp/cli.yml")
	}
	if colorFlag != "never" {
		t.Fatalf("color flag = %q, want %q", colorFlag, "never")
	}
	if requestTimeoutSeconds != 45 {
		t.Fatalf("timeout = %d, want %d", requestTimeoutSeconds, 45)
	}
	if !debugModeEnabled {
		t.Fatal("debug mode = false, want true")
	}
	if len(remainingArgs) != 2 || remainingArgs[0] != "search" || remainingArgs[1] != "demo" {
		t.Fatalf("remaining args = %#v, want [search demo]", remainingArgs)
	}
}

func TestParseCLIGlobalFlagsSupportsGlobalFlagsAfterCommand(t *testing.T) {
	cliConfigFilePath = ""
	serverAddressFlag = ""
	apiTokenFlag = ""
	tokenFilePath = ""
	outputFormatFlag = ""
	colorFlag = ""
	requestTimeoutSeconds = 0
	debugModeEnabled = false
	debugFlagProvided = false

	remainingArgs := ParseCLIGlobalFlags([]string{
		"engines",
		"--all",
		"--output",
		"json",
		"--color=never",
		"--timeout",
		"45",
	})

	if outputFormatFlag != "json" {
		t.Fatalf("output format after command = %q, want %q", outputFormatFlag, "json")
	}
	if colorFlag != "never" {
		t.Fatalf("color flag after command = %q, want %q", colorFlag, "never")
	}
	if requestTimeoutSeconds != 45 {
		t.Fatalf("timeout after command = %d, want %d", requestTimeoutSeconds, 45)
	}
	if len(remainingArgs) != 2 || remainingArgs[0] != "engines" || remainingArgs[1] != "--all" {
		t.Fatalf("remaining args after command = %#v, want [engines --all]", remainingArgs)
	}
}

func TestParseCLIGlobalFlagsResolvesNamedConfigFiles(t *testing.T) {
	homeDir := t.TempDir()
	t.Setenv("HOME", homeDir)
	t.Setenv("APPDATA", filepath.Join(homeDir, "AppData", "Roaming"))
	t.Setenv("LOCALAPPDATA", filepath.Join(homeDir, "AppData", "Local"))

	cliConfigFilePath = ""
	serverAddressFlag = ""
	apiTokenFlag = ""
	tokenFilePath = ""
	outputFormatFlag = ""
	colorFlag = ""
	requestTimeoutSeconds = 0
	debugModeEnabled = false
	debugFlagProvided = false

	remainingArgs := ParseCLIGlobalFlags([]string{"--config", "test", "search", "demo"})
	wantConfigFilePath := filepath.Join(paths.ConfigDir(), "test.yml")

	if cliConfigFilePath != wantConfigFilePath {
		t.Fatalf("config file = %q, want %q", cliConfigFilePath, wantConfigFilePath)
	}
	if len(remainingArgs) != 2 || remainingArgs[0] != "search" || remainingArgs[1] != "demo" {
		t.Fatalf("remaining args = %#v, want [search demo]", remainingArgs)
	}
}

func TestGetCLIAuthTokenFromEnv(t *testing.T) {
	originalCanonicalToken := os.Getenv("VIDVEIL_TOKEN")
	originalLegacyToken := os.Getenv("VIDVEIL_CLI_TOKEN")

	t.Cleanup(func() {
		if originalCanonicalToken == "" {
			os.Unsetenv("VIDVEIL_TOKEN")
		} else {
			os.Setenv("VIDVEIL_TOKEN", originalCanonicalToken)
		}

		if originalLegacyToken == "" {
			os.Unsetenv("VIDVEIL_CLI_TOKEN")
		} else {
			os.Setenv("VIDVEIL_CLI_TOKEN", originalLegacyToken)
		}
	})

	os.Unsetenv("VIDVEIL_TOKEN")
	os.Unsetenv("VIDVEIL_CLI_TOKEN")

	if gotToken := GetCLIAuthTokenFromEnv(); gotToken != "" {
		t.Fatalf("token with no env = %q, want empty", gotToken)
	}

	os.Setenv("VIDVEIL_CLI_TOKEN", "legacy-token")
	if gotToken := GetCLIAuthTokenFromEnv(); gotToken != "legacy-token" {
		t.Fatalf("legacy token = %q, want %q", gotToken, "legacy-token")
	}

	os.Setenv("VIDVEIL_TOKEN", "canonical-token")
	if gotToken := GetCLIAuthTokenFromEnv(); gotToken != "canonical-token" {
		t.Fatalf("canonical token = %q, want %q", gotToken, "canonical-token")
	}
}

func TestGetCLIServerAddressFromEnv(t *testing.T) {
	originalCanonicalServerAddress := os.Getenv("VIDVEIL_SERVER_PRIMARY")
	originalLegacyServerAddress := os.Getenv("VIDVEIL_SERVER")

	t.Cleanup(func() {
		if originalCanonicalServerAddress == "" {
			os.Unsetenv("VIDVEIL_SERVER_PRIMARY")
		} else {
			os.Setenv("VIDVEIL_SERVER_PRIMARY", originalCanonicalServerAddress)
		}

		if originalLegacyServerAddress == "" {
			os.Unsetenv("VIDVEIL_SERVER")
		} else {
			os.Setenv("VIDVEIL_SERVER", originalLegacyServerAddress)
		}
	})

	os.Unsetenv("VIDVEIL_SERVER_PRIMARY")
	os.Unsetenv("VIDVEIL_SERVER")

	if gotServerAddress := GetCLIServerAddressFromEnv(); gotServerAddress != "" {
		t.Fatalf("server address with no env = %q, want empty", gotServerAddress)
	}

	os.Setenv("VIDVEIL_SERVER", "https://legacy.example.com")
	if gotServerAddress := GetCLIServerAddressFromEnv(); gotServerAddress != "https://legacy.example.com" {
		t.Fatalf("legacy server address = %q, want %q", gotServerAddress, "https://legacy.example.com")
	}

	os.Setenv("VIDVEIL_SERVER_PRIMARY", "https://canonical.example.com")
	if gotServerAddress := GetCLIServerAddressFromEnv(); gotServerAddress != "https://canonical.example.com" {
		t.Fatalf("canonical server address = %q, want %q", gotServerAddress, "https://canonical.example.com")
	}
}

func TestLoadCLIConfigFromFileWritesServerPrimaryWhenConfigEmpty(t *testing.T) {
	homeDir := t.TempDir()
	t.Setenv("HOME", homeDir)
	t.Setenv("APPDATA", filepath.Join(homeDir, "AppData", "Roaming"))
	t.Setenv("LOCALAPPDATA", filepath.Join(homeDir, "AppData", "Local"))
	t.Setenv("VIDVEIL_SERVER_PRIMARY", "")
	t.Setenv("VIDVEIL_SERVER", "")
	t.Setenv("VIDVEIL_TOKEN", "")
	t.Setenv("VIDVEIL_CLI_TOKEN", "")

	cliConfigFilePath = filepath.Join(paths.ConfigDir(), "cli.yml")
	serverAddressFlag = "https://saved.example.com"
	apiTokenFlag = ""
	tokenFilePath = ""
	outputFormatFlag = ""
	colorFlag = ""
	requestTimeoutSeconds = 0
	debugModeEnabled = false
	debugFlagProvided = false

	if err := LoadCLIConfigFromFile(); err != nil {
		t.Fatalf("loading config: %v", err)
	}

	if cliConfig.Server.Address != "https://saved.example.com" {
		t.Fatalf("server address = %q, want %q", cliConfig.Server.Address, "https://saved.example.com")
	}

	configData, err := os.ReadFile(cliConfigFilePath)
	if err != nil {
		t.Fatalf("reading config file: %v", err)
	}
	configText := string(configData)
	if !strings.Contains(configText, "primary: https://saved.example.com") {
		t.Fatalf("config file missing server.primary entry:\n%s", configText)
	}
	if strings.Contains(configText, "address:") {
		t.Fatalf("config file should not write legacy server.address entry:\n%s", configText)
	}
}

func TestLoadCLIConfigFromFilePreservesExistingServerPrimary(t *testing.T) {
	homeDir := t.TempDir()
	t.Setenv("HOME", homeDir)
	t.Setenv("APPDATA", filepath.Join(homeDir, "AppData", "Roaming"))
	t.Setenv("LOCALAPPDATA", filepath.Join(homeDir, "AppData", "Local"))
	t.Setenv("VIDVEIL_SERVER_PRIMARY", "")
	t.Setenv("VIDVEIL_SERVER", "")
	t.Setenv("VIDVEIL_TOKEN", "")
	t.Setenv("VIDVEIL_CLI_TOKEN", "")

	cliConfigFilePath = filepath.Join(paths.ConfigDir(), "cli.yml")
	serverAddressFlag = "https://session-only.example.com"
	apiTokenFlag = ""
	tokenFilePath = ""
	outputFormatFlag = ""
	colorFlag = ""
	requestTimeoutSeconds = 0
	debugModeEnabled = false
	debugFlagProvided = false

	if err := os.MkdirAll(filepath.Dir(cliConfigFilePath), 0700); err != nil {
		t.Fatalf("creating config dir: %v", err)
	}
	if err := os.WriteFile(cliConfigFilePath, []byte("server:\n  primary: https://configured.example.com\n  timeout: 30\n"), 0600); err != nil {
		t.Fatalf("writing config file: %v", err)
	}

	if err := LoadCLIConfigFromFile(); err != nil {
		t.Fatalf("loading config: %v", err)
	}

	if cliConfig.Server.Address != "https://session-only.example.com" {
		t.Fatalf("server address = %q, want %q", cliConfig.Server.Address, "https://session-only.example.com")
	}

	configData, err := os.ReadFile(cliConfigFilePath)
	if err != nil {
		t.Fatalf("reading config file: %v", err)
	}
	if strings.Contains(string(configData), "session-only.example.com") {
		t.Fatalf("config file should keep the original saved server:\n%s", string(configData))
	}
}

func TestLoadCLIConfigFromFileNormalizesConfigPermissions(t *testing.T) {
	homeDir := t.TempDir()
	t.Setenv("HOME", homeDir)
	t.Setenv("APPDATA", filepath.Join(homeDir, "AppData", "Roaming"))
	t.Setenv("LOCALAPPDATA", filepath.Join(homeDir, "AppData", "Local"))
	t.Setenv("VIDVEIL_SERVER_PRIMARY", "")
	t.Setenv("VIDVEIL_SERVER", "")
	t.Setenv("VIDVEIL_TOKEN", "")
	t.Setenv("VIDVEIL_CLI_TOKEN", "")
	t.Setenv("VIDVEIL_OUTPUT_FORMAT", "")
	t.Setenv("VIDVEIL_OUTPUT_COLOR", "")
	t.Setenv("VIDVEIL_SERVER_TIMEOUT", "")
	t.Setenv("VIDVEIL_DEBUG", "")

	cliConfigFilePath = filepath.Join(paths.ConfigDir(), "cli.yml")
	serverAddressFlag = ""
	apiTokenFlag = ""
	tokenFilePath = ""
	outputFormatFlag = ""
	colorFlag = ""
	requestTimeoutSeconds = 0
	debugModeEnabled = false
	debugFlagProvided = false

	if err := os.MkdirAll(filepath.Dir(cliConfigFilePath), 0700); err != nil {
		t.Fatalf("creating config dir: %v", err)
	}
	if err := os.WriteFile(cliConfigFilePath, []byte("server:\n  primary: https://configured.example.com\n"), 0644); err != nil {
		t.Fatalf("writing config file: %v", err)
	}

	if err := LoadCLIConfigFromFile(); err != nil {
		t.Fatalf("loading config: %v", err)
	}

	fileInfo, err := os.Stat(cliConfigFilePath)
	if err != nil {
		t.Fatalf("stating config file: %v", err)
	}
	if fileInfo.Mode().Perm() != 0600 {
		t.Fatalf("config file permissions = %v, want %v", fileInfo.Mode().Perm(), os.FileMode(0600))
	}
	if err := paths.EnsurePathOwnership(cliConfigFilePath); err != nil {
		t.Fatalf("verifying config ownership: %v", err)
	}
}

func TestLoadCLIConfigFromFileReadsTopLevelToken(t *testing.T) {
	homeDir := t.TempDir()
	t.Setenv("HOME", homeDir)
	t.Setenv("APPDATA", filepath.Join(homeDir, "AppData", "Roaming"))
	t.Setenv("LOCALAPPDATA", filepath.Join(homeDir, "AppData", "Local"))
	t.Setenv("VIDVEIL_SERVER_PRIMARY", "")
	t.Setenv("VIDVEIL_SERVER", "")
	t.Setenv("VIDVEIL_TOKEN", "")
	t.Setenv("VIDVEIL_CLI_TOKEN", "")

	cliConfigFilePath = filepath.Join(paths.ConfigDir(), "cli.yml")
	serverAddressFlag = ""
	apiTokenFlag = ""
	tokenFilePath = ""
	outputFormatFlag = ""
	colorFlag = ""
	requestTimeoutSeconds = 0
	debugModeEnabled = false
	debugFlagProvided = false

	if err := os.MkdirAll(filepath.Dir(cliConfigFilePath), 0700); err != nil {
		t.Fatalf("creating config dir: %v", err)
	}
	if err := os.WriteFile(cliConfigFilePath, []byte("server:\n  primary: https://configured.example.com\n  timeout: 30\ntoken: cfg-token\n"), 0600); err != nil {
		t.Fatalf("writing config file: %v", err)
	}

	if err := LoadCLIConfigFromFile(); err != nil {
		t.Fatalf("loading config: %v", err)
	}

	if cliConfig.Server.Token != "cfg-token" {
		t.Fatalf("config token = %q, want %q", cliConfig.Server.Token, "cfg-token")
	}
}

func TestLoadCLIConfigFromFileAutoCreatesMissingConfig(t *testing.T) {
	homeDir := t.TempDir()
	t.Setenv("HOME", homeDir)
	t.Setenv("APPDATA", filepath.Join(homeDir, "AppData", "Roaming"))
	t.Setenv("LOCALAPPDATA", filepath.Join(homeDir, "AppData", "Local"))
	t.Setenv("VIDVEIL_SERVER_PRIMARY", "")
	t.Setenv("VIDVEIL_SERVER", "")
	t.Setenv("VIDVEIL_TOKEN", "")
	t.Setenv("VIDVEIL_CLI_TOKEN", "")
	t.Setenv("VIDVEIL_OUTPUT_FORMAT", "")
	t.Setenv("VIDVEIL_OUTPUT_COLOR", "")
	t.Setenv("VIDVEIL_SERVER_TIMEOUT", "")
	t.Setenv("VIDVEIL_DEBUG", "")

	originalOfficialSite := OfficialSite
	t.Cleanup(func() {
		OfficialSite = originalOfficialSite
	})
	OfficialSite = ""

	cliConfigFilePath = filepath.Join(paths.ConfigDir(), "cli.yml")
	serverAddressFlag = ""
	apiTokenFlag = ""
	tokenFilePath = ""
	outputFormatFlag = ""
	colorFlag = ""
	requestTimeoutSeconds = 0
	debugModeEnabled = false
	debugFlagProvided = false

	if err := LoadCLIConfigFromFile(); err != nil {
		t.Fatalf("loading config: %v", err)
	}

	configData, err := os.ReadFile(cliConfigFilePath)
	if err != nil {
		t.Fatalf("reading created config file: %v", err)
	}
	configText := string(configData)
	if !strings.Contains(configText, "timeout: 30s") {
		t.Fatalf("config file missing default timeout:\n%s", configText)
	}
	if !strings.Contains(configText, "api_version: v1") {
		t.Fatalf("config file missing default server.api_version:\n%s", configText)
	}
	if !strings.Contains(configText, "admin_path: admin") {
		t.Fatalf("config file missing default server.admin_path:\n%s", configText)
	}
	if !strings.Contains(configText, "cluster: []") {
		t.Fatalf("config file missing default server.cluster:\n%s", configText)
	}
	if !strings.Contains(configText, "retry: 3") {
		t.Fatalf("config file missing default server.retry:\n%s", configText)
	}
	if !strings.Contains(configText, "retry_delay: 1s") {
		t.Fatalf("config file missing default server.retry_delay:\n%s", configText)
	}
	if !strings.Contains(configText, "auth:") {
		t.Fatalf("config file missing auth section:\n%s", configText)
	}
	if !strings.Contains(configText, "token_file: \"\"") {
		t.Fatalf("config file missing default auth.token_file entry:\n%s", configText)
	}
	if !strings.Contains(configText, "format: table") {
		t.Fatalf("config file missing default output format:\n%s", configText)
	}
	if !strings.Contains(configText, "color: auto") {
		t.Fatalf("config file missing default color mode:\n%s", configText)
	}
	if !strings.Contains(configText, "pager: auto") {
		t.Fatalf("config file missing default output pager:\n%s", configText)
	}
	if !strings.Contains(configText, "quiet: false") {
		t.Fatalf("config file missing default output quiet setting:\n%s", configText)
	}
	if !strings.Contains(configText, "verbose: false") {
		t.Fatalf("config file missing default output verbose setting:\n%s", configText)
	}
	if !strings.Contains(configText, "logging:") {
		t.Fatalf("config file missing logging section:\n%s", configText)
	}
	if !strings.Contains(configText, "level: warn") {
		t.Fatalf("config file missing default logging level:\n%s", configText)
	}
	if !strings.Contains(configText, "max_size: 10MB") {
		t.Fatalf("config file missing default logging max size:\n%s", configText)
	}
	if !strings.Contains(configText, "max_files: 5") {
		t.Fatalf("config file missing default logging max files:\n%s", configText)
	}
	if !strings.Contains(configText, "cache:") {
		t.Fatalf("config file missing cache section:\n%s", configText)
	}
	if !strings.Contains(configText, "enabled: true") {
		t.Fatalf("config file missing default cache enabled setting:\n%s", configText)
	}
	if !strings.Contains(configText, "ttl: 5m") {
		t.Fatalf("config file missing default cache ttl:\n%s", configText)
	}
	if !strings.Contains(configText, "max_size: 100MB") {
		t.Fatalf("config file missing default cache max size:\n%s", configText)
	}
	if !strings.Contains(configText, "enabled: true") {
		t.Fatalf("config file missing default tui.enabled:\n%s", configText)
	}
	if !strings.Contains(configText, "theme: dark") {
		t.Fatalf("config file missing default theme:\n%s", configText)
	}
	if !strings.Contains(configText, "mouse: true") {
		t.Fatalf("config file missing default tui.mouse:\n%s", configText)
	}
	if !strings.Contains(configText, "unicode: true") {
		t.Fatalf("config file missing default tui.unicode:\n%s", configText)
	}
	if !strings.Contains(configText, "debug: false") {
		t.Fatalf("config file missing default debug setting:\n%s", configText)
	}
}

func TestWriteCLIConfigFilePromotesLegacyServerTokenToAuthSection(t *testing.T) {
	homeDir := t.TempDir()
	t.Setenv("HOME", homeDir)
	t.Setenv("APPDATA", filepath.Join(homeDir, "AppData", "Roaming"))
	t.Setenv("LOCALAPPDATA", filepath.Join(homeDir, "AppData", "Local"))

	configFilePath := filepath.Join(paths.ConfigDir(), "cli.yml")
	fileCLIConfig := CLIConfig{}
	fileCLIConfig.Server.Address = "https://configured.example.com"
	fileCLIConfig.Server.Token = "cfg-token"
	fileCLIConfig.Server.Timeout = 30
	fileCLIConfig.Output.Format = "table"
	fileCLIConfig.Output.Color = "auto"

	if err := WriteCLIConfigFile(fileCLIConfig, configFilePath); err != nil {
		t.Fatalf("writing config file: %v", err)
	}

	configData, err := os.ReadFile(configFilePath)
	if err != nil {
		t.Fatalf("reading config file: %v", err)
	}
	configText := string(configData)
	if !strings.Contains(configText, "auth:\n    token: cfg-token") {
		t.Fatalf("config file missing auth.token entry:\n%s", configText)
	}
	if strings.Contains(configText, "\ntoken: cfg-token\n") {
		t.Fatalf("config file should not write legacy top-level token entry:\n%s", configText)
	}
}

func TestLoadCLIConfigFromFileEnvironmentOverridesConfig(t *testing.T) {
	homeDir := t.TempDir()
	t.Setenv("HOME", homeDir)
	t.Setenv("APPDATA", filepath.Join(homeDir, "AppData", "Roaming"))
	t.Setenv("LOCALAPPDATA", filepath.Join(homeDir, "AppData", "Local"))
	t.Setenv("VIDVEIL_SERVER_PRIMARY", "https://env.example.com")
	t.Setenv("VIDVEIL_SERVER", "")
	t.Setenv("VIDVEIL_TOKEN", "env-token")
	t.Setenv("VIDVEIL_CLI_TOKEN", "")
	t.Setenv("VIDVEIL_OUTPUT_FORMAT", "json")
	t.Setenv("VIDVEIL_OUTPUT_COLOR", "never")
	t.Setenv("VIDVEIL_SERVER_TIMEOUT", "45")
	t.Setenv("VIDVEIL_DEBUG", "enabled")

	cliConfigFilePath = filepath.Join(paths.ConfigDir(), "cli.yml")
	serverAddressFlag = ""
	apiTokenFlag = ""
	tokenFilePath = ""
	outputFormatFlag = ""
	colorFlag = ""
	requestTimeoutSeconds = 0
	debugModeEnabled = false
	debugFlagProvided = false

	configYAML := "server:\n  primary: https://configured.example.com\n  timeout: 30s\ntoken: cfg-token\noutput:\n  format: table\n  color: auto\n"
	if err := os.MkdirAll(filepath.Dir(cliConfigFilePath), 0700); err != nil {
		t.Fatalf("creating config dir: %v", err)
	}
	if err := os.WriteFile(cliConfigFilePath, []byte(configYAML), 0600); err != nil {
		t.Fatalf("writing config file: %v", err)
	}

	if err := LoadCLIConfigFromFile(); err != nil {
		t.Fatalf("loading config: %v", err)
	}

	if cliConfig.Server.Address != "https://env.example.com" {
		t.Fatalf("server address = %q, want %q", cliConfig.Server.Address, "https://env.example.com")
	}
	if cliConfig.Server.Token != "env-token" {
		t.Fatalf("token = %q, want %q", cliConfig.Server.Token, "env-token")
	}
	if cliConfig.Output.Format != "json" {
		t.Fatalf("output format = %q, want %q", cliConfig.Output.Format, "json")
	}
	if cliConfig.Output.Color != "never" {
		t.Fatalf("output color = %q, want %q", cliConfig.Output.Color, "never")
	}
	if cliConfig.Server.Timeout != 45 {
		t.Fatalf("timeout = %d, want %d", cliConfig.Server.Timeout, 45)
	}
	if !debugModeEnabled {
		t.Fatal("debug mode = false, want true")
	}
}

func TestLoadCLIConfigFromFileTokenFileFlagOverridesEnvironment(t *testing.T) {
	homeDir := t.TempDir()
	t.Setenv("HOME", homeDir)
	t.Setenv("APPDATA", filepath.Join(homeDir, "AppData", "Roaming"))
	t.Setenv("LOCALAPPDATA", filepath.Join(homeDir, "AppData", "Local"))
	t.Setenv("VIDVEIL_SERVER_PRIMARY", "")
	t.Setenv("VIDVEIL_SERVER", "")
	t.Setenv("VIDVEIL_TOKEN", "env-token")
	t.Setenv("VIDVEIL_CLI_TOKEN", "")
	t.Setenv("VIDVEIL_OUTPUT_FORMAT", "")
	t.Setenv("VIDVEIL_OUTPUT_COLOR", "")
	t.Setenv("VIDVEIL_SERVER_TIMEOUT", "")
	t.Setenv("VIDVEIL_DEBUG", "")

	tokenFilePath = filepath.Join(t.TempDir(), "token")
	if err := os.WriteFile(tokenFilePath, []byte("flag-file-token\n"), 0600); err != nil {
		t.Fatalf("writing token file: %v", err)
	}

	cliConfigFilePath = filepath.Join(paths.ConfigDir(), "cli.yml")
	serverAddressFlag = ""
	apiTokenFlag = ""
	outputFormatFlag = ""
	colorFlag = ""
	requestTimeoutSeconds = 0
	debugModeEnabled = false
	debugFlagProvided = false

	if err := LoadCLIConfigFromFile(); err != nil {
		t.Fatalf("loading config: %v", err)
	}

	if cliConfig.Server.Token != "flag-file-token" {
		t.Fatalf("token = %q, want %q", cliConfig.Server.Token, "flag-file-token")
	}
}

func TestLoadCLIConfigFromFileNormalizesDefaultTokenFilePermissions(t *testing.T) {
	homeDir := t.TempDir()
	t.Setenv("HOME", homeDir)
	t.Setenv("APPDATA", filepath.Join(homeDir, "AppData", "Roaming"))
	t.Setenv("LOCALAPPDATA", filepath.Join(homeDir, "AppData", "Local"))
	t.Setenv("VIDVEIL_SERVER_PRIMARY", "")
	t.Setenv("VIDVEIL_SERVER", "")
	t.Setenv("VIDVEIL_TOKEN", "")
	t.Setenv("VIDVEIL_CLI_TOKEN", "")
	t.Setenv("VIDVEIL_OUTPUT_FORMAT", "")
	t.Setenv("VIDVEIL_OUTPUT_COLOR", "")
	t.Setenv("VIDVEIL_SERVER_TIMEOUT", "")
	t.Setenv("VIDVEIL_DEBUG", "")

	cliConfigFilePath = filepath.Join(paths.ConfigDir(), "cli.yml")
	serverAddressFlag = ""
	apiTokenFlag = ""
	tokenFilePath = ""
	outputFormatFlag = ""
	colorFlag = ""
	requestTimeoutSeconds = 0
	debugModeEnabled = false
	debugFlagProvided = false

	defaultTokenFilePath := paths.TokenFile()
	if err := os.MkdirAll(filepath.Dir(defaultTokenFilePath), 0700); err != nil {
		t.Fatalf("creating token dir: %v", err)
	}
	if err := os.WriteFile(defaultTokenFilePath, []byte("default-token\n"), 0644); err != nil {
		t.Fatalf("writing token file: %v", err)
	}

	if err := LoadCLIConfigFromFile(); err != nil {
		t.Fatalf("loading config: %v", err)
	}

	if cliConfig.Server.Token != "default-token" {
		t.Fatalf("token = %q, want %q", cliConfig.Server.Token, "default-token")
	}

	fileInfo, err := os.Stat(defaultTokenFilePath)
	if err != nil {
		t.Fatalf("stating token file: %v", err)
	}
	if fileInfo.Mode().Perm() != 0600 {
		t.Fatalf("token file permissions = %v, want %v", fileInfo.Mode().Perm(), os.FileMode(0600))
	}
	if err := paths.EnsurePathOwnership(defaultTokenFilePath); err != nil {
		t.Fatalf("verifying token ownership: %v", err)
	}
}

func TestWriteCLIDefaultTokenFileUsesRestrictedPermissions(t *testing.T) {
	homeDir := t.TempDir()
	t.Setenv("HOME", homeDir)
	t.Setenv("APPDATA", filepath.Join(homeDir, "AppData", "Roaming"))
	t.Setenv("LOCALAPPDATA", filepath.Join(homeDir, "AppData", "Local"))

	if err := WriteCLIDefaultTokenFile("saved-token"); err != nil {
		t.Fatalf("writing default token file: %v", err)
	}

	defaultTokenFilePath := paths.TokenFile()
	tokenData, err := os.ReadFile(defaultTokenFilePath)
	if err != nil {
		t.Fatalf("reading token file: %v", err)
	}
	if string(tokenData) != "saved-token" {
		t.Fatalf("token file contents = %q, want %q", string(tokenData), "saved-token")
	}

	fileInfo, err := os.Stat(defaultTokenFilePath)
	if err != nil {
		t.Fatalf("stating token file: %v", err)
	}
	if fileInfo.Mode().Perm() != 0600 {
		t.Fatalf("token file permissions = %v, want %v", fileInfo.Mode().Perm(), os.FileMode(0600))
	}
	if err := paths.EnsurePathOwnership(defaultTokenFilePath); err != nil {
		t.Fatalf("verifying token ownership: %v", err)
	}
}

func TestLoadCLIConfigFromFileReadsAuthTokenFromConfig(t *testing.T) {
	homeDir := t.TempDir()
	t.Setenv("HOME", homeDir)
	t.Setenv("APPDATA", filepath.Join(homeDir, "AppData", "Roaming"))
	t.Setenv("LOCALAPPDATA", filepath.Join(homeDir, "AppData", "Local"))
	t.Setenv("VIDVEIL_SERVER_PRIMARY", "")
	t.Setenv("VIDVEIL_SERVER", "")
	t.Setenv("VIDVEIL_TOKEN", "")
	t.Setenv("VIDVEIL_CLI_TOKEN", "")
	t.Setenv("VIDVEIL_OUTPUT_FORMAT", "")
	t.Setenv("VIDVEIL_OUTPUT_COLOR", "")
	t.Setenv("VIDVEIL_SERVER_TIMEOUT", "")
	t.Setenv("VIDVEIL_DEBUG", "")

	cliConfigFilePath = filepath.Join(paths.ConfigDir(), "cli.yml")
	serverAddressFlag = ""
	apiTokenFlag = ""
	tokenFilePath = ""
	outputFormatFlag = ""
	colorFlag = ""
	requestTimeoutSeconds = 0
	debugModeEnabled = false
	debugFlagProvided = false

	configYAML := "server:\n  primary: https://configured.example.com\nauth:\n  token: auth-token\n  token_file: \"\"\n"
	if err := os.MkdirAll(filepath.Dir(cliConfigFilePath), 0700); err != nil {
		t.Fatalf("creating config dir: %v", err)
	}
	if err := os.WriteFile(cliConfigFilePath, []byte(configYAML), 0600); err != nil {
		t.Fatalf("writing config file: %v", err)
	}

	if err := LoadCLIConfigFromFile(); err != nil {
		t.Fatalf("loading config: %v", err)
	}

	if cliConfig.Server.Token != "auth-token" {
		t.Fatalf("token = %q, want %q", cliConfig.Server.Token, "auth-token")
	}
}

func TestLoadCLIConfigFromFileReadsAuthTokenFileFromConfig(t *testing.T) {
	homeDir := t.TempDir()
	t.Setenv("HOME", homeDir)
	t.Setenv("APPDATA", filepath.Join(homeDir, "AppData", "Roaming"))
	t.Setenv("LOCALAPPDATA", filepath.Join(homeDir, "AppData", "Local"))
	t.Setenv("VIDVEIL_SERVER_PRIMARY", "")
	t.Setenv("VIDVEIL_SERVER", "")
	t.Setenv("VIDVEIL_TOKEN", "")
	t.Setenv("VIDVEIL_CLI_TOKEN", "")
	t.Setenv("VIDVEIL_OUTPUT_FORMAT", "")
	t.Setenv("VIDVEIL_OUTPUT_COLOR", "")
	t.Setenv("VIDVEIL_SERVER_TIMEOUT", "")
	t.Setenv("VIDVEIL_DEBUG", "")

	configTokenFilePath := filepath.Join(t.TempDir(), "config-token")
	if err := os.WriteFile(configTokenFilePath, []byte("config-file-token\n"), 0600); err != nil {
		t.Fatalf("writing config token file: %v", err)
	}

	cliConfigFilePath = filepath.Join(paths.ConfigDir(), "cli.yml")
	serverAddressFlag = ""
	apiTokenFlag = ""
	tokenFilePath = ""
	outputFormatFlag = ""
	colorFlag = ""
	requestTimeoutSeconds = 0
	debugModeEnabled = false
	debugFlagProvided = false

	configYAML := "server:\n  primary: https://configured.example.com\nauth:\n  token: auth-token\n  token_file: " + configTokenFilePath + "\n"
	if err := os.MkdirAll(filepath.Dir(cliConfigFilePath), 0700); err != nil {
		t.Fatalf("creating config dir: %v", err)
	}
	if err := os.WriteFile(cliConfigFilePath, []byte(configYAML), 0600); err != nil {
		t.Fatalf("writing config file: %v", err)
	}

	if err := LoadCLIConfigFromFile(); err != nil {
		t.Fatalf("loading config: %v", err)
	}

	if cliConfig.Server.Token != "config-file-token" {
		t.Fatalf("token = %q, want %q", cliConfig.Server.Token, "config-file-token")
	}
}

func TestLoadCLIConfigFromFileReadsOutputVerboseFromConfig(t *testing.T) {
	homeDir := t.TempDir()
	t.Setenv("HOME", homeDir)
	t.Setenv("APPDATA", filepath.Join(homeDir, "AppData", "Roaming"))
	t.Setenv("LOCALAPPDATA", filepath.Join(homeDir, "AppData", "Local"))
	t.Setenv("VIDVEIL_SERVER_PRIMARY", "")
	t.Setenv("VIDVEIL_SERVER", "")
	t.Setenv("VIDVEIL_TOKEN", "")
	t.Setenv("VIDVEIL_CLI_TOKEN", "")
	t.Setenv("VIDVEIL_OUTPUT_FORMAT", "")
	t.Setenv("VIDVEIL_OUTPUT_COLOR", "")
	t.Setenv("VIDVEIL_SERVER_TIMEOUT", "")
	t.Setenv("VIDVEIL_DEBUG", "")

	cliConfigFilePath = filepath.Join(paths.ConfigDir(), "cli.yml")
	serverAddressFlag = ""
	apiTokenFlag = ""
	tokenFilePath = ""
	outputFormatFlag = ""
	colorFlag = ""
	requestTimeoutSeconds = 0
	debugModeEnabled = false
	debugFlagProvided = false

	configYAML := "server:\n  primary: https://configured.example.com\noutput:\n  format: table\n  color: auto\n  pager: auto\n  quiet: false\n  verbose: true\n"
	if err := os.MkdirAll(filepath.Dir(cliConfigFilePath), 0700); err != nil {
		t.Fatalf("creating config dir: %v", err)
	}
	if err := os.WriteFile(cliConfigFilePath, []byte(configYAML), 0600); err != nil {
		t.Fatalf("writing config file: %v", err)
	}

	if err := LoadCLIConfigFromFile(); err != nil {
		t.Fatalf("loading config: %v", err)
	}

	if !cliConfig.Output.Verbose {
		t.Fatal("expected output.verbose config to enable verbose output")
	}
}

func TestLoadCLIConfigFromFileReadsLoggingAndCacheFromConfig(t *testing.T) {
	homeDir := t.TempDir()
	t.Setenv("HOME", homeDir)
	t.Setenv("APPDATA", filepath.Join(homeDir, "AppData", "Roaming"))
	t.Setenv("LOCALAPPDATA", filepath.Join(homeDir, "AppData", "Local"))
	t.Setenv("VIDVEIL_SERVER_PRIMARY", "")
	t.Setenv("VIDVEIL_SERVER", "")
	t.Setenv("VIDVEIL_TOKEN", "")
	t.Setenv("VIDVEIL_CLI_TOKEN", "")
	t.Setenv("VIDVEIL_OUTPUT_FORMAT", "")
	t.Setenv("VIDVEIL_OUTPUT_COLOR", "")
	t.Setenv("VIDVEIL_SERVER_TIMEOUT", "")
	t.Setenv("VIDVEIL_DEBUG", "")

	cliConfigFilePath = filepath.Join(paths.ConfigDir(), "cli.yml")
	serverAddressFlag = ""
	apiTokenFlag = ""
	tokenFilePath = ""
	outputFormatFlag = ""
	colorFlag = ""
	requestTimeoutSeconds = 0
	debugModeEnabled = false
	debugFlagProvided = false

	configYAML := "server:\n  primary: https://configured.example.com\nlogging:\n  level: debug\n  file: /tmp/vidveil-cli.log\n  max_size: 20MB\n  max_files: 7\ncache:\n  enabled: false\n  ttl: 10m\n  max_size: 200MB\n"
	if err := os.MkdirAll(filepath.Dir(cliConfigFilePath), 0700); err != nil {
		t.Fatalf("creating config dir: %v", err)
	}
	if err := os.WriteFile(cliConfigFilePath, []byte(configYAML), 0600); err != nil {
		t.Fatalf("writing config file: %v", err)
	}

	if err := LoadCLIConfigFromFile(); err != nil {
		t.Fatalf("loading config: %v", err)
	}

	if cliConfig.Logging.Level != "debug" {
		t.Fatalf("logging level = %q, want %q", cliConfig.Logging.Level, "debug")
	}
	if cliConfig.Logging.File != "/tmp/vidveil-cli.log" {
		t.Fatalf("logging file = %q, want %q", cliConfig.Logging.File, "/tmp/vidveil-cli.log")
	}
	if cliConfig.Logging.MaxSize != "20MB" {
		t.Fatalf("logging max size = %q, want %q", cliConfig.Logging.MaxSize, "20MB")
	}
	if cliConfig.Logging.MaxFiles != 7 {
		t.Fatalf("logging max files = %d, want %d", cliConfig.Logging.MaxFiles, 7)
	}
	if cliConfig.Cache.Enabled {
		t.Fatal("expected cache.enabled config to disable cache")
	}
	if cliConfig.Cache.TTL != "10m" {
		t.Fatalf("cache ttl = %q, want %q", cliConfig.Cache.TTL, "10m")
	}
	if cliConfig.Cache.MaxSize != "200MB" {
		t.Fatalf("cache max size = %q, want %q", cliConfig.Cache.MaxSize, "200MB")
	}
}

func TestInitializeCLILoggingUsesConfiguredLogFile(t *testing.T) {
	originalCLIConfig := cliConfig
	originalLogWriter := log.Writer()
	originalLogFlags := log.Flags()
	t.Cleanup(func() {
		CloseCLILogging()
		log.SetOutput(originalLogWriter)
		log.SetFlags(originalLogFlags)
		cliConfig = originalCLIConfig
	})

	customLogFilePath := filepath.Join(t.TempDir(), "custom", "vidveil-cli.log")
	cliConfig = &CLIConfig{}
	cliConfig.Logging.File = customLogFilePath

	if err := InitializeCLILogging(); err != nil {
		t.Fatalf("initializing cli logging: %v", err)
	}

	log.Print("configured log entry")
	CloseCLILogging()

	logData, err := os.ReadFile(customLogFilePath)
	if err != nil {
		t.Fatalf("reading configured log file: %v", err)
	}
	if !strings.Contains(string(logData), "configured log entry") {
		t.Fatalf("configured log file missing log entry:\n%s", string(logData))
	}
}

func TestInitializeCLILoggingUsesDefaultLogFile(t *testing.T) {
	homeDir := t.TempDir()
	t.Setenv("HOME", homeDir)
	t.Setenv("APPDATA", filepath.Join(homeDir, "AppData", "Roaming"))
	t.Setenv("LOCALAPPDATA", filepath.Join(homeDir, "AppData", "Local"))

	originalCLIConfig := cliConfig
	originalLogWriter := log.Writer()
	originalLogFlags := log.Flags()
	t.Cleanup(func() {
		CloseCLILogging()
		log.SetOutput(originalLogWriter)
		log.SetFlags(originalLogFlags)
		cliConfig = originalCLIConfig
	})

	cliConfig = &CLIConfig{}

	if err := InitializeCLILogging(); err != nil {
		t.Fatalf("initializing cli logging: %v", err)
	}

	defaultLogFilePath := paths.LogFile()
	fileInfo, err := os.Stat(defaultLogFilePath)
	if err != nil {
		t.Fatalf("stating default log file: %v", err)
	}
	if fileInfo.Mode().Perm() != 0600 {
		t.Fatalf("default log file permissions = %v, want %v", fileInfo.Mode().Perm(), os.FileMode(0600))
	}
	if err := paths.EnsurePathOwnership(defaultLogFilePath); err != nil {
		t.Fatalf("verifying default log ownership: %v", err)
	}
}

func TestExecuteCLICreatesClientDirsAfterParsingFlags(t *testing.T) {
	homeDir := t.TempDir()
	t.Setenv("HOME", homeDir)
	t.Setenv("APPDATA", filepath.Join(homeDir, "AppData", "Roaming"))
	t.Setenv("LOCALAPPDATA", filepath.Join(homeDir, "AppData", "Local"))
	t.Setenv("VIDVEIL_SERVER_PRIMARY", "")
	t.Setenv("VIDVEIL_SERVER", "")
	t.Setenv("VIDVEIL_TOKEN", "")
	t.Setenv("VIDVEIL_CLI_TOKEN", "")
	t.Setenv("VIDVEIL_OUTPUT_FORMAT", "")
	t.Setenv("VIDVEIL_OUTPUT_COLOR", "")
	t.Setenv("VIDVEIL_SERVER_TIMEOUT", "")
	t.Setenv("VIDVEIL_DEBUG", "")

	originalArgs := os.Args
	originalCLIConfigFilePath := cliConfigFilePath
	originalServerAddressFlag := serverAddressFlag
	originalAPITokenFlag := apiTokenFlag
	originalTokenFilePath := tokenFilePath
	originalOutputFormatFlag := outputFormatFlag
	originalColorFlag := colorFlag
	originalRequestTimeoutSeconds := requestTimeoutSeconds
	originalDebugModeEnabled := debugModeEnabled
	originalDebugFlagProvided := debugFlagProvided
	originalOfficialSite := OfficialSite
	t.Cleanup(func() {
		CloseCLILogging()
		os.Args = originalArgs
		cliConfigFilePath = originalCLIConfigFilePath
		serverAddressFlag = originalServerAddressFlag
		apiTokenFlag = originalAPITokenFlag
		tokenFilePath = originalTokenFilePath
		outputFormatFlag = originalOutputFormatFlag
		colorFlag = originalColorFlag
		requestTimeoutSeconds = originalRequestTimeoutSeconds
		debugModeEnabled = originalDebugModeEnabled
		debugFlagProvided = originalDebugFlagProvided
		OfficialSite = originalOfficialSite
	})

	OfficialSite = ""
	cliConfigFilePath = ""
	serverAddressFlag = ""
	apiTokenFlag = ""
	tokenFilePath = ""
	outputFormatFlag = ""
	colorFlag = ""
	requestTimeoutSeconds = 0
	debugModeEnabled = false
	debugFlagProvided = false
	os.Args = []string{"vidveil-cli", "login", "--help"}

	if err := ExecuteCLI(); err != nil {
		t.Fatalf("executing cli: %v", err)
	}

	expectedDirs := []string{
		paths.ConfigDir(),
		paths.DataDir(),
		paths.CacheDir(),
		paths.LogDir(),
	}
	for _, expectedDir := range expectedDirs {
		fileInfo, err := os.Stat(expectedDir)
		if err != nil {
			t.Fatalf("stating client dir %q: %v", expectedDir, err)
		}
		if !fileInfo.IsDir() {
			t.Fatalf("path %q is not a directory", expectedDir)
		}
		if err := paths.EnsurePathOwnership(expectedDir); err != nil {
			t.Fatalf("verifying ownership for %q: %v", expectedDir, err)
		}
	}
}

func TestRunProbeCommandUsesOutputVerboseFromConfig(t *testing.T) {
	originalCLIConfig := cliConfig
	t.Cleanup(func() {
		cliConfig = originalCLIConfig
		probeVerboseMode = false
	})

	cliConfig = &CLIConfig{}
	cliConfig.Output.Verbose = true

	err := RunProbeCommand(nil)
	if err == nil {
		t.Fatal("expected probe command to require engine selection")
	}
	if !probeVerboseMode {
		t.Fatal("expected output.verbose config to enable probe verbose mode")
	}
}

func TestLoadCLIConfigFromFileUsesOfficialSiteDefault(t *testing.T) {
	homeDir := t.TempDir()
	t.Setenv("HOME", homeDir)
	t.Setenv("APPDATA", filepath.Join(homeDir, "AppData", "Roaming"))
	t.Setenv("LOCALAPPDATA", filepath.Join(homeDir, "AppData", "Local"))
	t.Setenv("VIDVEIL_SERVER_PRIMARY", "")
	t.Setenv("VIDVEIL_SERVER", "")
	t.Setenv("VIDVEIL_TOKEN", "")
	t.Setenv("VIDVEIL_CLI_TOKEN", "")
	t.Setenv("VIDVEIL_OUTPUT_FORMAT", "")
	t.Setenv("VIDVEIL_OUTPUT_COLOR", "")
	t.Setenv("VIDVEIL_SERVER_TIMEOUT", "")
	t.Setenv("VIDVEIL_DEBUG", "")

	originalOfficialSite := OfficialSite
	t.Cleanup(func() {
		OfficialSite = originalOfficialSite
	})
	OfficialSite = "https://x.scour.li"

	cliConfigFilePath = filepath.Join(paths.ConfigDir(), "cli.yml")
	serverAddressFlag = ""
	apiTokenFlag = ""
	tokenFilePath = ""
	outputFormatFlag = ""
	colorFlag = ""
	requestTimeoutSeconds = 0
	debugModeEnabled = false
	debugFlagProvided = false

	if err := LoadCLIConfigFromFile(); err != nil {
		t.Fatalf("loading config: %v", err)
	}

	if cliConfig.Server.Address != "https://x.scour.li" {
		t.Fatalf("server address = %q, want %q", cliConfig.Server.Address, "https://x.scour.li")
	}
}

func TestIsServerConfiguredDoesNotTreatOfficialSiteAsSavedServer(t *testing.T) {
	originalCLIConfig := cliConfig
	originalCLIConfigHasSavedServerAddress := cliConfigHasSavedServerAddress
	originalOfficialSite := OfficialSite
	t.Cleanup(func() {
		cliConfig = originalCLIConfig
		cliConfigHasSavedServerAddress = originalCLIConfigHasSavedServerAddress
		OfficialSite = originalOfficialSite
	})

	cliConfig = &CLIConfig{}
	cliConfigHasSavedServerAddress = false
	OfficialSite = "https://x.scour.li"

	if IsServerConfigured() {
		t.Fatal("expected official site default not to count as saved configuration")
	}
}

func TestIsServerConfiguredUsesSavedConfigServer(t *testing.T) {
	originalCLIConfig := cliConfig
	originalCLIConfigHasSavedServerAddress := cliConfigHasSavedServerAddress
	originalOfficialSite := OfficialSite
	t.Cleanup(func() {
		cliConfig = originalCLIConfig
		cliConfigHasSavedServerAddress = originalCLIConfigHasSavedServerAddress
		OfficialSite = originalOfficialSite
	})

	cliConfig = &CLIConfig{}
	cliConfigHasSavedServerAddress = true
	OfficialSite = "https://x.scour.li"

	if !IsServerConfigured() {
		t.Fatal("expected saved config server to count as configured")
	}
}

func TestPrintCLIHelpMessageShowsOfficialSiteDefault(t *testing.T) {
	originalBinaryName := BinaryName
	originalVersion := Version
	originalOfficialSite := OfficialSite
	t.Cleanup(func() {
		BinaryName = originalBinaryName
		Version = originalVersion
		OfficialSite = originalOfficialSite
	})

	BinaryName = "vidveil-cli"
	Version = "dev"
	OfficialSite = "https://x.scour.li"

	originalStdout := os.Stdout
	readPipe, writePipe, err := os.Pipe()
	if err != nil {
		t.Fatalf("creating stdout pipe: %v", err)
	}

	os.Stdout = writePipe
	PrintCLIHelpMessage()
	writePipe.Close()
	os.Stdout = originalStdout

	var outputBuffer bytes.Buffer
	if _, err := outputBuffer.ReadFrom(readPipe); err != nil {
		t.Fatalf("reading help output: %v", err)
	}

	helpOutput := outputBuffer.String()
	if !strings.Contains(helpOutput, "--server URL                  Server URL (default: https://x.scour.li)") {
		t.Fatalf("help output missing official site default:\n%s", helpOutput)
	}
	if !strings.Contains(helpOutput, "--output FORMAT               Output format: json, yaml, csv, table, plain (default: table)") {
		t.Fatalf("help output missing yaml/csv output formats:\n%s", helpOutput)
	}
}

func TestLoadCLIConfigFromFileReadsTUIEnabledFalse(t *testing.T) {
	homeDir := t.TempDir()
	t.Setenv("HOME", homeDir)
	t.Setenv("APPDATA", filepath.Join(homeDir, "AppData", "Roaming"))
	t.Setenv("LOCALAPPDATA", filepath.Join(homeDir, "AppData", "Local"))
	t.Setenv("VIDVEIL_SERVER_PRIMARY", "")
	t.Setenv("VIDVEIL_SERVER", "")
	t.Setenv("VIDVEIL_TOKEN", "")
	t.Setenv("VIDVEIL_CLI_TOKEN", "")
	t.Setenv("VIDVEIL_OUTPUT_FORMAT", "")
	t.Setenv("VIDVEIL_OUTPUT_COLOR", "")
	t.Setenv("VIDVEIL_SERVER_TIMEOUT", "")
	t.Setenv("VIDVEIL_DEBUG", "")

	cliConfigFilePath = filepath.Join(paths.ConfigDir(), "cli.yml")
	serverAddressFlag = ""
	apiTokenFlag = ""
	tokenFilePath = ""
	outputFormatFlag = ""
	colorFlag = ""
	requestTimeoutSeconds = 0
	debugModeEnabled = false
	debugFlagProvided = false

	configYAML := "server:\n  primary: https://configured.example.com\n  timeout: 30s\ntui:\n  enabled: false\n  theme: dark\n  mouse: true\n  unicode: true\n"
	if err := os.MkdirAll(filepath.Dir(cliConfigFilePath), 0700); err != nil {
		t.Fatalf("creating config dir: %v", err)
	}
	if err := os.WriteFile(cliConfigFilePath, []byte(configYAML), 0600); err != nil {
		t.Fatalf("writing config file: %v", err)
	}

	if err := LoadCLIConfigFromFile(); err != nil {
		t.Fatalf("loading config: %v", err)
	}

	if IsCLITUIEnabled() {
		t.Fatal("expected tui.enabled=false to disable interactive TUI autostart")
	}
}

func TestLoadCLIConfigFromFileIgnoresLegacyTUIShowHintsKey(t *testing.T) {
	homeDir := t.TempDir()
	t.Setenv("HOME", homeDir)
	t.Setenv("APPDATA", filepath.Join(homeDir, "AppData", "Roaming"))
	t.Setenv("LOCALAPPDATA", filepath.Join(homeDir, "AppData", "Local"))
	t.Setenv("VIDVEIL_SERVER_PRIMARY", "")
	t.Setenv("VIDVEIL_SERVER", "")
	t.Setenv("VIDVEIL_TOKEN", "")
	t.Setenv("VIDVEIL_CLI_TOKEN", "")
	t.Setenv("VIDVEIL_OUTPUT_FORMAT", "")
	t.Setenv("VIDVEIL_OUTPUT_COLOR", "")
	t.Setenv("VIDVEIL_SERVER_TIMEOUT", "")
	t.Setenv("VIDVEIL_DEBUG", "")

	cliConfigFilePath = filepath.Join(paths.ConfigDir(), "cli.yml")
	serverAddressFlag = ""
	apiTokenFlag = ""
	tokenFilePath = ""
	outputFormatFlag = ""
	colorFlag = ""
	requestTimeoutSeconds = 0
	debugModeEnabled = false
	debugFlagProvided = false

	configYAML := "server:\n  primary: https://configured.example.com\n  timeout: 30s\ntui:\n  enabled: true\n  theme: dark\n  show_hints: true\n"
	if err := os.MkdirAll(filepath.Dir(cliConfigFilePath), 0700); err != nil {
		t.Fatalf("creating config dir: %v", err)
	}
	if err := os.WriteFile(cliConfigFilePath, []byte(configYAML), 0600); err != nil {
		t.Fatalf("writing config file: %v", err)
	}

	if err := LoadCLIConfigFromFile(); err != nil {
		t.Fatalf("loading config: %v", err)
	}

	if !cliConfig.TUI.Mouse {
		t.Fatal("expected legacy tui.show_hints config to leave default tui.mouse enabled")
	}
	if !cliConfig.TUI.Unicode {
		t.Fatal("expected legacy tui.show_hints config to leave default tui.unicode enabled")
	}
}

func TestLoadCLIConfigFromFileReadsDurationTimeoutString(t *testing.T) {
	homeDir := t.TempDir()
	t.Setenv("HOME", homeDir)
	t.Setenv("APPDATA", filepath.Join(homeDir, "AppData", "Roaming"))
	t.Setenv("LOCALAPPDATA", filepath.Join(homeDir, "AppData", "Local"))
	t.Setenv("VIDVEIL_SERVER_PRIMARY", "")
	t.Setenv("VIDVEIL_SERVER", "")
	t.Setenv("VIDVEIL_TOKEN", "")
	t.Setenv("VIDVEIL_CLI_TOKEN", "")
	t.Setenv("VIDVEIL_OUTPUT_FORMAT", "")
	t.Setenv("VIDVEIL_OUTPUT_COLOR", "")
	t.Setenv("VIDVEIL_SERVER_TIMEOUT", "")
	t.Setenv("VIDVEIL_DEBUG", "")

	cliConfigFilePath = filepath.Join(paths.ConfigDir(), "cli.yml")
	serverAddressFlag = ""
	apiTokenFlag = ""
	tokenFilePath = ""
	outputFormatFlag = ""
	colorFlag = ""
	requestTimeoutSeconds = 0
	debugModeEnabled = false
	debugFlagProvided = false

	configYAML := "server:\n  primary: https://configured.example.com\n  timeout: 45s\n"
	if err := os.MkdirAll(filepath.Dir(cliConfigFilePath), 0700); err != nil {
		t.Fatalf("creating config dir: %v", err)
	}
	if err := os.WriteFile(cliConfigFilePath, []byte(configYAML), 0600); err != nil {
		t.Fatalf("writing config file: %v", err)
	}

	if err := LoadCLIConfigFromFile(); err != nil {
		t.Fatalf("loading config: %v", err)
	}

	if cliConfig.Server.Timeout != 45 {
		t.Fatalf("timeout = %d, want %d", cliConfig.Server.Timeout, 45)
	}
}

func TestLoadCLIConfigFromFileReadsDebugFromConfig(t *testing.T) {
	homeDir := t.TempDir()
	t.Setenv("HOME", homeDir)
	t.Setenv("APPDATA", filepath.Join(homeDir, "AppData", "Roaming"))
	t.Setenv("LOCALAPPDATA", filepath.Join(homeDir, "AppData", "Local"))
	t.Setenv("VIDVEIL_SERVER_PRIMARY", "")
	t.Setenv("VIDVEIL_SERVER", "")
	t.Setenv("VIDVEIL_TOKEN", "")
	t.Setenv("VIDVEIL_CLI_TOKEN", "")
	t.Setenv("VIDVEIL_OUTPUT_FORMAT", "")
	t.Setenv("VIDVEIL_OUTPUT_COLOR", "")
	t.Setenv("VIDVEIL_SERVER_TIMEOUT", "")
	t.Setenv("VIDVEIL_DEBUG", "")

	cliConfigFilePath = filepath.Join(paths.ConfigDir(), "cli.yml")
	serverAddressFlag = ""
	apiTokenFlag = ""
	tokenFilePath = ""
	outputFormatFlag = ""
	colorFlag = ""
	requestTimeoutSeconds = 0
	debugModeEnabled = false
	debugFlagProvided = false

	configYAML := "server:\n  primary: https://configured.example.com\n  timeout: 30s\ndebug: true\n"
	if err := os.MkdirAll(filepath.Dir(cliConfigFilePath), 0700); err != nil {
		t.Fatalf("creating config dir: %v", err)
	}
	if err := os.WriteFile(cliConfigFilePath, []byte(configYAML), 0600); err != nil {
		t.Fatalf("writing config file: %v", err)
	}

	if err := LoadCLIConfigFromFile(); err != nil {
		t.Fatalf("loading config: %v", err)
	}

	if !debugModeEnabled {
		t.Fatal("expected debug config to enable debug mode")
	}
}

func TestLoadCLIConfigFromFileReadsServerAdminPathFromConfig(t *testing.T) {
	homeDir := t.TempDir()
	t.Setenv("HOME", homeDir)
	t.Setenv("APPDATA", filepath.Join(homeDir, "AppData", "Roaming"))
	t.Setenv("LOCALAPPDATA", filepath.Join(homeDir, "AppData", "Local"))

	originalCLIConfigFilePath := cliConfigFilePath
	originalServerAddressFlag := serverAddressFlag
	originalAPITokenFlag := apiTokenFlag
	originalTokenFilePath := tokenFilePath
	originalOutputFormatFlag := outputFormatFlag
	originalColorFlag := colorFlag
	originalRequestTimeoutSeconds := requestTimeoutSeconds
	originalDebugModeEnabled := debugModeEnabled
	originalDebugFlagProvided := debugFlagProvided
	originalOfficialSite := OfficialSite
	t.Cleanup(func() {
		cliConfigFilePath = originalCLIConfigFilePath
		serverAddressFlag = originalServerAddressFlag
		apiTokenFlag = originalAPITokenFlag
		tokenFilePath = originalTokenFilePath
		outputFormatFlag = originalOutputFormatFlag
		colorFlag = originalColorFlag
		requestTimeoutSeconds = originalRequestTimeoutSeconds
		debugModeEnabled = originalDebugModeEnabled
		debugFlagProvided = originalDebugFlagProvided
		OfficialSite = originalOfficialSite
	})

	OfficialSite = ""
	cliConfigFilePath = filepath.Join(paths.ConfigDir(), "cli.yml")
	configText := `server:
  primary: "https://configured.example.com"
  admin_path: manage
`
	if err := os.MkdirAll(filepath.Dir(cliConfigFilePath), 0700); err != nil {
		t.Fatalf("creating config directory: %v", err)
	}
	if err := os.WriteFile(cliConfigFilePath, []byte(configText), 0600); err != nil {
		t.Fatalf("writing config file: %v", err)
	}

	if err := LoadCLIConfigFromFile(); err != nil {
		t.Fatalf("loading config: %v", err)
	}

	if cliConfig.Server.AdminPath != "manage" {
		t.Fatalf("server admin path = %q, want %q", cliConfig.Server.AdminPath, "manage")
	}
}

func TestInitAPIClientUsesConfiguredAPIVersion(t *testing.T) {
	originalCLIConfig := cliConfig
	originalAPIClient := apiClient
	t.Cleanup(func() {
		cliConfig = originalCLIConfig
		apiClient = originalAPIClient
	})

	cliConfig = &CLIConfig{}
	cliConfig.Server.Address = "https://configured.example.com"
	cliConfig.Server.Token = "cfg-token"
	cliConfig.Server.Timeout = 30
	cliConfig.Server.APIVersion = "v2"

	InitAPIClient()

	if apiClient.GetAPIBaseURL() != "https://configured.example.com/api/v2" {
		t.Fatalf("api base URL = %q, want %q", apiClient.GetAPIBaseURL(), "https://configured.example.com/api/v2")
	}
}

func TestResolveCLIReachableServerAddressFallsBackToClusterNode(t *testing.T) {
	healthyClusterServer := httptest.NewServer(http.HandlerFunc(func(responseWriter http.ResponseWriter, request *http.Request) {
		if request.URL.Path != "/api/v1/healthz" {
			t.Fatalf("unexpected health path %q", request.URL.Path)
		}
		responseWriter.WriteHeader(http.StatusOK)
	}))
	defer healthyClusterServer.Close()

	originalCLIConfig := cliConfig
	t.Cleanup(func() {
		cliConfig = originalCLIConfig
	})

	cliConfig = &CLIConfig{}
	cliConfig.Server.Address = "http://127.0.0.1:1"
	cliConfig.Server.Cluster = []string{healthyClusterServer.URL}
	cliConfig.Server.APIVersion = "v1"
	cliConfig.Server.Timeout = 1

	if gotServerAddress := ResolveCLIReachableServerAddress(); gotServerAddress != healthyClusterServer.URL {
		t.Fatalf("resolved server address = %q, want %q", gotServerAddress, healthyClusterServer.URL)
	}
}

func TestDiscoverCLIServerConfigMergesAutodiscoverResponse(t *testing.T) {
	discoveryServer := httptest.NewServer(http.HandlerFunc(func(responseWriter http.ResponseWriter, request *http.Request) {
		if request.URL.Path != "/api/autodiscover" {
			t.Fatalf("unexpected autodiscover path %q", request.URL.Path)
		}
		responseWriter.Header().Set("Content-Type", "application/json")
		responseWriter.WriteHeader(http.StatusOK)
		_, _ = responseWriter.Write([]byte(`{"primary":"https://cluster.example.com","cluster":["https://node-a.example.com","https://node-b.example.com"],"api_version":"v2","timeout":45,"retry":5,"retry_delay":2}`))
	}))
	defer discoveryServer.Close()

	discoveryClient := api.NewAPIClient(discoveryServer.URL, "", 1, "v1")
	fileCLIConfig := CLIConfig{}
	fileCLIConfig.Server.Address = "https://configured.example.com"
	fileCLIConfig.Server.APIVersion = "v1"
	fileCLIConfig.Server.Timeout = 30
	fileCLIConfig.Server.Retry = 3
	fileCLIConfig.Server.RetryDelay = 1

	discoveredCLIConfig, err := DiscoverCLIServerConfig(discoveryClient, fileCLIConfig)
	if err != nil {
		t.Fatalf("discovering cli config: %v", err)
	}

	if discoveredCLIConfig.Server.Address != "https://cluster.example.com" {
		t.Fatalf("primary server = %q, want %q", discoveredCLIConfig.Server.Address, "https://cluster.example.com")
	}
	if len(discoveredCLIConfig.Server.Cluster) != 2 {
		t.Fatalf("cluster node count = %d, want %d", len(discoveredCLIConfig.Server.Cluster), 2)
	}
	if discoveredCLIConfig.Server.APIVersion != "v2" {
		t.Fatalf("api version = %q, want %q", discoveredCLIConfig.Server.APIVersion, "v2")
	}
	if discoveredCLIConfig.Server.Timeout != 45 {
		t.Fatalf("timeout = %d, want %d", discoveredCLIConfig.Server.Timeout, 45)
	}
	if discoveredCLIConfig.Server.Retry != 5 {
		t.Fatalf("retry = %d, want %d", discoveredCLIConfig.Server.Retry, 5)
	}
	if discoveredCLIConfig.Server.RetryDelay != 2 {
		t.Fatalf("retry delay = %d, want %d", discoveredCLIConfig.Server.RetryDelay, 2)
	}
}

func TestWriteCLIConfigFileWritesDurationTimeoutString(t *testing.T) {
	homeDir := t.TempDir()
	t.Setenv("HOME", homeDir)
	t.Setenv("APPDATA", filepath.Join(homeDir, "AppData", "Roaming"))
	t.Setenv("LOCALAPPDATA", filepath.Join(homeDir, "AppData", "Local"))

	configFilePath := filepath.Join(paths.ConfigDir(), "cli.yml")
	fileCLIConfig := CLIConfig{}
	fileCLIConfig.Server.Address = "https://configured.example.com"
	fileCLIConfig.Server.Timeout = 45

	if err := WriteCLIConfigFile(fileCLIConfig, configFilePath); err != nil {
		t.Fatalf("writing config file: %v", err)
	}

	configData, err := os.ReadFile(configFilePath)
	if err != nil {
		t.Fatalf("reading config file: %v", err)
	}

	if !strings.Contains(string(configData), "timeout: 45s") {
		t.Fatalf("config file missing duration timeout:\n%s", string(configData))
	}
}

func TestTUIQuestionMarkTogglesShortcutsHelp(t *testing.T) {
	model := CreateInitialTUIModel()

	updatedModel, command := model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'?'}})
	if command != nil {
		t.Fatal("expected no command when toggling shortcuts help")
	}

	tuiModel, ok := updatedModel.(TUIModel)
	if !ok {
		t.Fatalf("updated model type = %T, want TUIModel", updatedModel)
	}
	if !tuiModel.showShortcutsHelp {
		t.Fatal("expected shortcuts help to be shown after pressing ?")
	}

	updatedModel, command = tuiModel.Update(tea.KeyMsg{Type: tea.KeyEsc})
	if command != nil {
		t.Fatal("expected no command when closing shortcuts help")
	}

	tuiModel, ok = updatedModel.(TUIModel)
	if !ok {
		t.Fatalf("updated model type after esc = %T, want TUIModel", updatedModel)
	}
	if tuiModel.showShortcutsHelp {
		t.Fatal("expected shortcuts help to close after pressing esc")
	}
}

func TestTUIViewShowsShortcutsHelp(t *testing.T) {
	model := CreateInitialTUIModel()
	model.showShortcutsHelp = true

	viewOutput := model.View()
	if !strings.Contains(viewOutput, "Shortcuts:") {
		t.Fatalf("view missing shortcuts heading:\n%s", viewOutput)
	}
	if !strings.Contains(viewOutput, "  ?      toggle this help") {
		t.Fatalf("view missing shortcuts help line:\n%s", viewOutput)
	}
}
