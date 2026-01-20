// SPDX-License-Identifier: MIT
// AI.md PART 33: CLI Client - Setup Wizard
package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/apimgr/vidveil/src/client/api"
	"github.com/apimgr/vidveil/src/client/paths"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"gopkg.in/yaml.v3"
)

// Setup wizard constants
// Per AI.md PART 1: No magic numbers - use named constants
const (
	SetupMinURLLength              = 10
	SetupTestConnectionTimeoutSecs = 10
	SetupPasswordMaskChar          = '*'
	SetupDefaultServerURLPrefix    = "https://"
	SetupInputFocusServerURL       = 0
	SetupInputFocusToken           = 1
)

// SetupWizardState represents the current state of the setup wizard
// Per AI.md PART 1: Type names MUST be specific
type SetupWizardState int

const (
	SetupStateServerURL SetupWizardState = iota
	SetupStateTestConnection
	SetupStateToken
	SetupStateSaveConfig
	SetupStateComplete
	SetupStateFailed
)

// SetupWizardModel represents the setup wizard TUI model
// Per AI.md PART 1: Type names MUST be specific - "model" is ambiguous
type SetupWizardModel struct {
	state             SetupWizardState
	serverURL         string
	apiToken          string
	cursorPosition    int
	errorMessage      string
	statusMessage     string
	connectionSuccess bool
	serverVersion     string
	saveToConfig      bool
	isQuitting        bool
	// inputFocused: 0 = server URL, 1 = token
	inputFocused      int
}

// SetupConnectionTestMsg is sent when connection test completes
// Per AI.md PART 1: Type names MUST be specific
type SetupConnectionTestMsg struct {
	success bool
	version string
	err     error
}

// CreateSetupWizardModel creates the initial setup wizard model
// Per AI.md PART 1: Function names MUST reveal intent
func CreateSetupWizardModel() SetupWizardModel {
	return SetupWizardModel{
		state:        SetupStateServerURL,
		serverURL:    SetupDefaultServerURLPrefix,
		saveToConfig: true,
		inputFocused: SetupInputFocusServerURL,
	}
}

// Init implements tea.Model
func (m SetupWizardModel) Init() tea.Cmd {
	return nil
}

// Update implements tea.Model
func (m SetupWizardModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c", "esc":
			m.isQuitting = true
			return m, tea.Quit

		case "enter":
			return m.handleEnterKey()

		case "tab":
			// Toggle between fields
			if m.state == SetupStateToken {
				m.inputFocused = (m.inputFocused + 1) % 2
			}
			return m, nil

		case "backspace":
			return m.handleBackspace()

		case " ":
			// Toggle save to config checkbox
			if m.state == SetupStateSaveConfig {
				m.saveToConfig = !m.saveToConfig
				return m, nil
			}
			// Otherwise add space to input
			return m.handleInput(" ")

		default:
			if len(msg.String()) == 1 {
				return m.handleInput(msg.String())
			}
		}

	case SetupConnectionTestMsg:
		m.state = SetupStateToken
		if msg.success {
			m.connectionSuccess = true
			m.serverVersion = msg.version
			m.statusMessage = fmt.Sprintf("Connected! Server version: %s", msg.version)
			m.errorMessage = ""
		} else {
			m.connectionSuccess = false
			m.errorMessage = fmt.Sprintf("Connection failed: %v", msg.err)
			m.state = SetupStateServerURL
		}
		return m, nil
	}

	return m, nil
}

// handleEnterKey handles the enter key press based on current state
func (m SetupWizardModel) handleEnterKey() (tea.Model, tea.Cmd) {
	switch m.state {
	case SetupStateServerURL:
		// Validate URL
		if !strings.HasPrefix(m.serverURL, "http://") && !strings.HasPrefix(m.serverURL, "https://") {
			m.errorMessage = "URL must start with http:// or https://"
			return m, nil
		}
		if len(m.serverURL) < SetupMinURLLength {
			m.errorMessage = "Please enter a valid server URL"
			return m, nil
		}
		m.errorMessage = ""
		m.state = SetupStateTestConnection
		m.statusMessage = "Testing connection..."
		return m, TestServerConnection(m.serverURL)

	case SetupStateToken:
		// Token is optional, proceed to save
		m.state = SetupStateSaveConfig
		return m, nil

	case SetupStateSaveConfig:
		// Save configuration
		if err := SaveSetupWizardConfig(m.serverURL, m.apiToken, m.saveToConfig); err != nil {
			m.errorMessage = fmt.Sprintf("Failed to save config: %v", err)
			m.state = SetupStateFailed
			return m, nil
		}
		m.state = SetupStateComplete
		return m, tea.Quit
	}

	return m, nil
}

// handleBackspace handles backspace key
func (m SetupWizardModel) handleBackspace() (tea.Model, tea.Cmd) {
	switch m.state {
	case SetupStateServerURL:
		if len(m.serverURL) > 0 {
			m.serverURL = m.serverURL[:len(m.serverURL)-1]
		}
	case SetupStateToken:
		if len(m.apiToken) > 0 {
			m.apiToken = m.apiToken[:len(m.apiToken)-1]
		}
	}
	return m, nil
}

// handleInput handles text input
func (m SetupWizardModel) handleInput(char string) (tea.Model, tea.Cmd) {
	switch m.state {
	case SetupStateServerURL:
		m.serverURL += char
	case SetupStateToken:
		m.apiToken += char
	}
	return m, nil
}

// View implements tea.Model
func (m SetupWizardModel) View() string {
	if m.isQuitting {
		return "Setup cancelled.\n"
	}

	// Styles
	titleStyle := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#00BFFF"))
	inputStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#FFFFFF"))
	labelStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#888888"))
	errorStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#FF5555"))
	successStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#55FF55"))
	helpStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#666666"))

	var viewBuilder strings.Builder

	// Header
	viewBuilder.WriteString("\n")
	viewBuilder.WriteString(titleStyle.Render("  VidVeil CLI Setup Wizard"))
	viewBuilder.WriteString("\n\n")

	// Current state content
	switch m.state {
	case SetupStateServerURL:
		viewBuilder.WriteString(labelStyle.Render("  No server configured. Let's set one up!\n\n"))
		viewBuilder.WriteString(labelStyle.Render("  Server URL:\n"))
		viewBuilder.WriteString(inputStyle.Render(fmt.Sprintf("  [%s_]\n", m.serverURL)))

	case SetupStateTestConnection:
		viewBuilder.WriteString(labelStyle.Render("  Testing connection...\n"))

	case SetupStateToken:
		if m.connectionSuccess {
			viewBuilder.WriteString(successStyle.Render(fmt.Sprintf("  Connected to server (v%s)\n\n", m.serverVersion)))
		}
		viewBuilder.WriteString(labelStyle.Render("  API Token (optional, press Enter to skip):\n"))
		displayToken := strings.Repeat(string(SetupPasswordMaskChar), len(m.apiToken))
		if m.apiToken == "" {
			displayToken = ""
		}
		viewBuilder.WriteString(inputStyle.Render(fmt.Sprintf("  [%s_]\n", displayToken)))

	case SetupStateSaveConfig:
		viewBuilder.WriteString(labelStyle.Render("  Configuration:\n\n"))
		viewBuilder.WriteString(fmt.Sprintf("    Server: %s\n", m.serverURL))
		if m.apiToken != "" {
			viewBuilder.WriteString("    Token:  ********\n")
		} else {
			viewBuilder.WriteString("    Token:  (none)\n")
		}
		viewBuilder.WriteString("\n")
		checkbox := "[ ]"
		if m.saveToConfig {
			checkbox = "[x]"
		}
		viewBuilder.WriteString(fmt.Sprintf("  %s Save to configuration file\n", checkbox))
		viewBuilder.WriteString("\n")
		viewBuilder.WriteString(labelStyle.Render("  Press Enter to save, Esc to cancel\n"))

	case SetupStateComplete:
		viewBuilder.WriteString(successStyle.Render("  Setup complete!\n\n"))
		viewBuilder.WriteString(fmt.Sprintf("    Server: %s\n", m.serverURL))
		viewBuilder.WriteString(fmt.Sprintf("    Config: %s\n", paths.ConfigFile()))
		if m.apiToken != "" {
			viewBuilder.WriteString(fmt.Sprintf("    Token:  %s\n", paths.TokenFile()))
		}

	case SetupStateFailed:
		viewBuilder.WriteString(errorStyle.Render("  Setup failed!\n"))
	}

	// Error message
	if m.errorMessage != "" {
		viewBuilder.WriteString("\n")
		viewBuilder.WriteString(errorStyle.Render(fmt.Sprintf("  Error: %s\n", m.errorMessage)))
	}

	// Status message
	if m.statusMessage != "" && m.errorMessage == "" {
		viewBuilder.WriteString("\n")
		viewBuilder.WriteString(successStyle.Render(fmt.Sprintf("  %s\n", m.statusMessage)))
	}

	// Help text
	viewBuilder.WriteString("\n")
	viewBuilder.WriteString(helpStyle.Render("  Esc: cancel | Enter: continue"))
	viewBuilder.WriteString("\n")

	return viewBuilder.String()
}

// TestServerConnection tests connection to the server
// Per AI.md PART 1: Function names MUST reveal intent
func TestServerConnection(serverURL string) tea.Cmd {
	return func() tea.Msg {
		// Create temporary client for testing
		testClient := api.NewAPIClient(serverURL, "", SetupTestConnectionTimeoutSecs)

		// Check health endpoint
		healthy, err := testClient.Health()
		if err != nil {
			return SetupConnectionTestMsg{success: false, err: err}
		}
		if !healthy {
			return SetupConnectionTestMsg{success: false, err: fmt.Errorf("server returned unhealthy status")}
		}

		// Get version
		versionResp, err := testClient.GetVersion()
		version := "unknown"
		if err == nil && versionResp != nil {
			version = versionResp.Version
		}

		return SetupConnectionTestMsg{success: true, version: version}
	}
}

// SaveSetupWizardConfig saves the setup wizard configuration
// Per AI.md PART 1: Function names MUST reveal intent
func SaveSetupWizardConfig(serverURL, token string, saveToFile bool) error {
	if !saveToFile {
		return nil
	}

	// Ensure directories exist
	if err := paths.EnsureClientDirs(); err != nil {
		return fmt.Errorf("creating directories: %w", err)
	}

	// Save config file
	configFilePath := paths.ConfigFile()
	configDirPath := filepath.Dir(configFilePath)

	if err := os.MkdirAll(configDirPath, 0700); err != nil {
		return fmt.Errorf("creating config directory: %w", err)
	}

	// Load existing config or create new
	var fileCLIConfig CLIConfig
	if data, err := os.ReadFile(configFilePath); err == nil {
		// Ignore unmarshal errors - start fresh if config is invalid
		_ = yaml.Unmarshal(data, &fileCLIConfig)
	}

	// Update server address
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
	content := "# VidVeil CLI Configuration\n# See: vidveil-cli --help\n\n" + string(data)
	if err := os.WriteFile(configFilePath, []byte(content), 0600); err != nil {
		return fmt.Errorf("writing config file: %w", err)
	}

	// Save token to separate file if provided
	if token != "" {
		tokenFilePath := paths.TokenFile()
		tokenDirPath := filepath.Dir(tokenFilePath)

		if err := os.MkdirAll(tokenDirPath, 0700); err != nil {
			return fmt.Errorf("creating token directory: %w", err)
		}

		if err := os.WriteFile(tokenFilePath, []byte(token), 0600); err != nil {
			return fmt.Errorf("writing token file: %w", err)
		}
	}

	return nil
}

// RunSetupWizard runs the setup wizard TUI
// Per AI.md PART 33: CLI First-Run Flow
// Per AI.md PART 1: Function names MUST reveal intent
func RunSetupWizard() error {
	wizardProgram := tea.NewProgram(CreateSetupWizardModel())
	finalModel, err := wizardProgram.Run()
	if err != nil {
		return err
	}

	model := finalModel.(SetupWizardModel)
	if model.isQuitting && model.state != SetupStateComplete {
		return fmt.Errorf("setup cancelled")
	}

	return nil
}

// IsServerConfigured checks if a server is configured
// Per AI.md PART 33: Check for config file before showing wizard
// Per AI.md PART 1: Function names MUST reveal intent
func IsServerConfigured() bool {
	// Check config file
	if cliConfig != nil && cliConfig.Server.Address != "" {
		return true
	}

	// Check environment variable
	if os.Getenv("VIDVEIL_SERVER") != "" {
		return true
	}

	return false
}

// EnsureServerConfigured ensures a server is configured, running wizard if needed
// Per AI.md PART 33: CLI First-Run Flow
// Per AI.md PART 1: Function names MUST reveal intent
func EnsureServerConfigured() error {
	if IsServerConfigured() {
		return nil
	}
	return RunSetupWizard()
}
