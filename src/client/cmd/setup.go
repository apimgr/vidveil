// SPDX-License-Identifier: MIT
// AI.md PART 32: CLI Client - Setup Wizard
package cmd

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"unicode/utf8"

	"github.com/apimgr/vidveil/src/client/api"
	paths "github.com/apimgr/vidveil/src/client/paths"
	"github.com/apimgr/vidveil/src/common/i18n"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"golang.org/x/term"
	"gopkg.in/yaml.v3"
)

// Setup wizard constants
// Per AI.md PART 1: No magic numbers - use named constants
const (
	SetupTestConnectionTimeoutSecs = 10
	SetupPasswordMaskChar          = '*'
	SetupDefaultServerURLPrefix    = "https://"
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
	state            SetupWizardState
	serverURL        string
	apiToken         string
	errorMessage     string
	statusMessage    string
	saveToConfig     bool
	setAsDefault     bool
	setupOptionFocus int
	isQuitting       bool
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

		case "tab", "shift+tab":
			if m.state == SetupStateSaveConfig {
				m.setupOptionFocus = (m.setupOptionFocus + 1) % 2
			}
			return m, nil

		case "backspace":
			return m.handleBackspace()

		case " ":
			// Toggle the focused setup persistence option.
			if m.state == SetupStateSaveConfig {
				if m.setupOptionFocus == 0 {
					m.saveToConfig = !m.saveToConfig
				} else {
					m.setAsDefault = !m.setAsDefault
				}
				return m, nil
			}
			// Otherwise add space to input
			return m.handleInput(" ")

		default:
			if len(msg.Runes) > 0 {
				return m.handleInput(string(msg.Runes))
			}
			if input := msg.String(); len(input) == 1 {
				return m.handleInput(input)
			}
		}

	case SetupConnectionTestMsg:
		if msg.success {
			m.statusMessage = i18n.TranslateFormat(resolveCLILanguage(), "cli.setup.connected", "version", msg.version)
			m.errorMessage = ""
			m.state = SetupStateSaveConfig
		} else {
			m.errorMessage = i18n.TranslateFormat(resolveCLILanguage(), "cli.setup.connection_failed", "error", msg.err)
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
		if err := validateCLIServerURL(m.serverURL); err != nil {
			m.errorMessage = i18n.TranslateFormat(resolveCLILanguage(), "cli.setup.invalid_server_url", "error", err)
			return m, nil
		}
		m.errorMessage = ""
		m.state = SetupStateToken
		m.statusMessage = ""
		return m, nil

	case SetupStateToken:
		m.errorMessage = ""
		m.state = SetupStateTestConnection
		m.statusMessage = i18n.Translate(resolveCLILanguage(), "cli.setup.testing_connection")
		return m, testServerConnection(m.serverURL, m.apiToken)

	case SetupStateSaveConfig:
		// Save configuration
		if err := SaveSetupWizardConfig(m.serverURL, m.apiToken, m.saveToConfig, m.setAsDefault); err != nil {
			m.errorMessage = i18n.TranslateFormat(resolveCLILanguage(), "cli.setup.save_failed", "error", err)
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
			_, size := utf8.DecodeLastRuneInString(m.serverURL)
			m.serverURL = m.serverURL[:len(m.serverURL)-size]
		}
	case SetupStateToken:
		if len(m.apiToken) > 0 {
			_, size := utf8.DecodeLastRuneInString(m.apiToken)
			m.apiToken = m.apiToken[:len(m.apiToken)-size]
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
		return i18n.Translate(resolveCLILanguage(), "cli.setup.cancelled") + "\n"
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
	viewBuilder.WriteString(titleStyle.Render("  " + i18n.Translate(resolveCLILanguage(), "cli.setup.title")))
	viewBuilder.WriteString("\n\n")

	// Current state content
	switch m.state {
	case SetupStateServerURL:
		viewBuilder.WriteString(labelStyle.Render("  " + i18n.Translate(resolveCLILanguage(), "cli.setup.no_server") + "\n\n"))
		viewBuilder.WriteString(labelStyle.Render("  " + i18n.Translate(resolveCLILanguage(), "cli.setup.server_url") + ":\n"))
		viewBuilder.WriteString(inputStyle.Render(fmt.Sprintf("  [%s_]\n", m.serverURL)))

	case SetupStateTestConnection:
		viewBuilder.WriteString(labelStyle.Render("  " + i18n.Translate(resolveCLILanguage(), "cli.setup.testing_connection") + "\n"))

	case SetupStateToken:
		viewBuilder.WriteString(labelStyle.Render("  " + i18n.Translate(resolveCLILanguage(), "cli.setup.api_token_optional") + "\n"))
		displayToken := strings.Repeat(string(SetupPasswordMaskChar), utf8.RuneCountInString(m.apiToken))
		if m.apiToken == "" {
			displayToken = ""
		}
		viewBuilder.WriteString(inputStyle.Render(fmt.Sprintf("  [%s_]\n", displayToken)))

	case SetupStateSaveConfig:
		viewBuilder.WriteString(labelStyle.Render("  " + i18n.Translate(resolveCLILanguage(), "cli.setup.configuration") + ":\n\n"))
		fmt.Fprintf(&viewBuilder, "    %s: %s\n", i18n.Translate(resolveCLILanguage(), "cli.setup.server"), m.serverURL)
		if m.apiToken != "" {
			fmt.Fprintf(&viewBuilder, "    %s:  ********\n", i18n.Translate(resolveCLILanguage(), "cli.setup.token"))
		} else {
			fmt.Fprintf(&viewBuilder, "    %s:  %s\n", i18n.Translate(resolveCLILanguage(), "cli.setup.token"), i18n.Translate(resolveCLILanguage(), "cli.setup.none"))
		}
		viewBuilder.WriteString("\n")
		checkbox := "[ ]"
		if m.saveToConfig {
			checkbox = "[x]"
		}
		fmt.Fprintf(&viewBuilder, "  %s %s\n", checkbox, i18n.Translate(resolveCLILanguage(), "gui.save_configuration"))
		defaultCheckbox := "[ ]"
		if m.setAsDefault {
			defaultCheckbox = "[x]"
		}
		fmt.Fprintf(&viewBuilder, "  %s %s\n", defaultCheckbox, i18n.Translate(resolveCLILanguage(), "gui.set_default_server"))
		viewBuilder.WriteString("\n")
		viewBuilder.WriteString(labelStyle.Render("  " + i18n.Translate(resolveCLILanguage(), "cli.setup.option_instructions") + "\n"))

	case SetupStateComplete:
		viewBuilder.WriteString(successStyle.Render("  " + i18n.Translate(resolveCLILanguage(), "cli.setup.complete") + "\n\n"))
		fmt.Fprintf(&viewBuilder, "    %s: %s\n", i18n.Translate(resolveCLILanguage(), "cli.setup.server"), m.serverURL)
		fmt.Fprintf(&viewBuilder, "    %s: %s\n", i18n.Translate(resolveCLILanguage(), "cli.setup.config"), getCLIConfigFilePath())
		if m.apiToken != "" {
			fmt.Fprintf(&viewBuilder, "    %s:  %s\n", i18n.Translate(resolveCLILanguage(), "cli.setup.token_file"), paths.TokenFile())
		}

	case SetupStateFailed:
		viewBuilder.WriteString(errorStyle.Render("  " + i18n.Translate(resolveCLILanguage(), "cli.setup.failed") + "\n"))
	}

	// Error message
	if m.errorMessage != "" {
		viewBuilder.WriteString("\n")
		viewBuilder.WriteString(errorStyle.Render(fmt.Sprintf("  %s: %s\n", i18n.Translate(resolveCLILanguage(), "common.error"), m.errorMessage)))
	}

	// Status message
	if m.statusMessage != "" && m.errorMessage == "" {
		viewBuilder.WriteString("\n")
		viewBuilder.WriteString(successStyle.Render(fmt.Sprintf("  %s\n", m.statusMessage)))
	}

	// Help text
	viewBuilder.WriteString("\n")
	viewBuilder.WriteString(helpStyle.Render("  " + i18n.Translate(resolveCLILanguage(), "cli.setup.navigation_help")))
	viewBuilder.WriteString("\n")

	return viewBuilder.String()
}

// testServerConnection tests connection to the server
// Per AI.md PART 1: Function names MUST reveal intent
func testServerConnection(serverURL, token string) tea.Cmd {
	return func() tea.Msg {
		version, err := validateSetupServerConnection(serverURL, token)
		return SetupConnectionTestMsg{success: err == nil, version: version, err: err}
	}
}

func validateSetupServerConnection(serverURL, token string) (string, error) {
	testClient := api.NewAPIClient(serverURL, token, SetupTestConnectionTimeoutSecs, "v1")
	testClient.SetLanguage(resolveCLILanguage())
	healthy, err := testClient.Health()
	if err != nil {
		return "", err
	}
	if !healthy {
		return "", fmt.Errorf("%s", i18n.Translate(resolveCLILanguage(), "cli.setup.server_unhealthy"))
	}
	if token != "" {
		if _, err := testClient.CheckAuth(); err != nil {
			return "", fmt.Errorf("%s", i18n.TranslateFormat(resolveCLILanguage(), "cli.setup.authentication_failed", "error", err))
		}
	}
	versionResp, err := testClient.GetVersion()
	if err != nil {
		return "", err
	}
	if versionResp == nil || versionResp.Version == "" {
		return "unknown", nil
	}
	return versionResp.Version, nil
}

// RunInlineSetupWizard handles first-run setup when a full-screen TUI cannot fit.
func RunInlineSetupWizard() error {
	if !term.IsTerminal(int(os.Stdin.Fd())) || !term.IsTerminal(int(os.Stdout.Fd())) {
		return NewExitError(ExitConfig, fmt.Errorf("%s", i18n.Translate(resolveCLILanguage(), "cli.setup.noninteractive_error")))
	}
	reader := bufio.NewReader(os.Stdin)
	fmt.Print(i18n.Translate(resolveCLILanguage(), "cli.setup.server_url") + ": ")
	serverURLLine, err := reader.ReadString('\n')
	if err != nil && serverURLLine == "" {
		return fmt.Errorf("%s: %w", i18n.Translate(resolveCLILanguage(), "cli.setup.read_server_url_failed"), err)
	}
	serverURL := strings.TrimSpace(serverURLLine)
	if err := validateCLIServerURL(serverURL); err != nil {
		return NewExitError(ExitConfig, fmt.Errorf("%s: %w", i18n.Translate(resolveCLILanguage(), "cli.setup.invalid_server_url_prefix"), err))
	}
	fmt.Print(i18n.Translate(resolveCLILanguage(), "cli.setup.api_token_optional") + ": ")
	tokenBytes, err := term.ReadPassword(int(os.Stdin.Fd()))
	fmt.Println()
	if err != nil {
		return fmt.Errorf("%s: %w", i18n.Translate(resolveCLILanguage(), "cli.setup.read_token_failed"), err)
	}
	token := strings.TrimSpace(string(tokenBytes))
	saveConfig, err := readSetupConfirmation(reader, i18n.Translate(resolveCLILanguage(), "gui.save_configuration"), true)
	if err != nil {
		return err
	}
	setDefault, err := readSetupConfirmation(reader, i18n.Translate(resolveCLILanguage(), "gui.set_default_server"), false)
	if err != nil {
		return err
	}
	version, err := validateSetupServerConnection(serverURL, token)
	if err != nil {
		return fmt.Errorf("%s: %w", i18n.Translate(resolveCLILanguage(), "cli.setup.connection_failed"), err)
	}
	fmt.Println(i18n.TranslateFormat(resolveCLILanguage(), "cli.setup.connected_version", "version", version))
	if err := SaveSetupWizardConfig(serverURL, token, saveConfig, setDefault); err != nil {
		return err
	}
	return nil
}

func readSetupConfirmation(reader *bufio.Reader, prompt string, defaultYes bool) (bool, error) {
	choice := i18n.Translate(resolveCLILanguage(), "cli.setup.no_default")
	if defaultYes {
		choice = i18n.Translate(resolveCLILanguage(), "cli.setup.yes_default")
	}
	fmt.Printf("%s [%s]: ", prompt, choice)
	answer, err := reader.ReadString('\n')
	if err != nil && answer == "" {
		return false, fmt.Errorf("%s: %w", i18n.Translate(resolveCLILanguage(), "cli.setup.read_choice_failed"), err)
	}
	answer = strings.ToLower(strings.TrimSpace(answer))
	if answer == "" {
		return defaultYes, nil
	}
	return setupConfirmationIsYes(answer, resolveCLILanguage()), nil
}

func setupConfirmationIsYes(answer, locale string) bool {
	if answer == "y" || answer == "yes" {
		return true
	}
	switch strings.ToLower(locale) {
	case "de":
		return answer == "j" || answer == "ja"
	case "es":
		return answer == "s" || answer == "si" || answer == "sí"
	case "fr":
		return answer == "o" || answer == "oui"
	case "it":
		return answer == "s" || answer == "si" || answer == "sì"
	case "ja":
		return answer == "はい"
	case "zh":
		return answer == "是"
	case "ar":
		return answer == "نعم" || answer == "ن"
	default:
		return false
	}
}

// SaveSetupWizardConfig saves the setup wizard configuration
// Per AI.md PART 1: Function names MUST reveal intent
func SaveSetupWizardConfig(serverURL, token string, saveToFile bool, setAsDefault ...bool) error {
	if err := validateCLIServerURL(serverURL); err != nil {
		return fmt.Errorf("%s: %w", i18n.Translate(resolveCLILanguage(), "cli.setup.invalid_server_url_prefix"), err)
	}
	if cliConfig != nil {
		cliConfig.Server.Address = serverURL
		cliConfig.Server.Token = token
		cliConfig.Auth.Token = token
	}
	if !saveToFile {
		return nil
	}

	// Ensure directories exist
	if err := paths.EnsureClientDirs(); err != nil {
		return fmt.Errorf("%s: %w", i18n.Translate(resolveCLILanguage(), "cli.setup.create_config_dirs_failed"), err)
	}

	// Save config file
	configFilePath := getCLIConfigFilePath()
	writeConfig := func(path string) error {
		var fileCLIConfig CLIConfig
		if data, err := os.ReadFile(path); err == nil {
			if err := yaml.Unmarshal(data, &fileCLIConfig); err != nil {
				return fmt.Errorf("%s %s: %w", i18n.Translate(resolveCLILanguage(), "cli.setup.parse_config_failed"), path, err)
			}
		} else if !os.IsNotExist(err) {
			return fmt.Errorf("%s %s: %w", i18n.Translate(resolveCLILanguage(), "cli.setup.read_config_failed"), path, err)
		}
		fileCLIConfig.Server.Address = serverURL
		fileCLIConfig.Auth.Token = token
		if fileCLIConfig.Server.Timeout == 0 {
			fileCLIConfig.Server.Timeout = 30
		}
		if fileCLIConfig.Output.Format == "" {
			fileCLIConfig.Output.Format = "table"
		}
		if fileCLIConfig.Output.Color == "" {
			fileCLIConfig.Output.Color = "auto"
		}
		return writeCLIConfigFile(fileCLIConfig, path)
	}
	if err := writeConfig(configFilePath); err != nil {
		return err
	}
	if len(setAsDefault) > 0 && setAsDefault[0] {
		defaultConfigPath := paths.ResolveConfigFilePath("")
		if filepath.Clean(defaultConfigPath) != filepath.Clean(configFilePath) {
			if err := writeConfig(defaultConfigPath); err != nil {
				return fmt.Errorf("%s: %w", i18n.Translate(resolveCLILanguage(), "cli.setup.save_default_config_failed"), err)
			}
		}
	}
	cliConfigHasSavedServerAddress = true
	return nil
}

// runSetupWizard runs the setup wizard TUI
// Per AI.md PART 32: CLI First-Run Flow
// Per AI.md PART 1: Function names MUST reveal intent
func runSetupWizard() error {
	wizardProgram := tea.NewProgram(CreateSetupWizardModel())
	finalModel, err := wizardProgram.Run()
	if err != nil {
		return err
	}

	model := finalModel.(SetupWizardModel)
	if model.isQuitting && model.state != SetupStateComplete {
		return fmt.Errorf("%s", i18n.Translate(resolveCLILanguage(), "cli.setup.cancelled"))
	}

	return nil
}

// IsServerConfigured checks if a server is configured
// Per AI.md PART 32: Check for config file before showing wizard
// Per AI.md PART 1: Function names MUST reveal intent
func IsServerConfigured() bool {
	// Check config file
	if cliConfigHasSavedServerAddress {
		return true
	}

	// Check environment variable
	if getCLIServerAddressFromEnv() != "" {
		return true
	}

	return false
}

// EnsureServerConfigured ensures a server is configured, running wizard if needed
// Per AI.md PART 32: CLI First-Run Flow
// Per AI.md PART 1: Function names MUST reveal intent
func EnsureServerConfigured() error {
	if IsServerConfigured() {
		return nil
	}
	return runSetupWizard()
}
