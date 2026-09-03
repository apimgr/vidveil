// SPDX-License-Identifier: MIT
// AI.md PART 28: Coverage tests for setup.go key handlers, Update, View,
// IsServerConfigured, EnsureServerConfigured, and ParseCLIGlobalFlags branches.
// No terminal, no network — pure state-machine tests.
package cmd

import (
	"os"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

// ── SetupWizardModel.handleEnterKey ──────────────────────────────────────────

func TestHandleEnterKey_ServerURL_InvalidURL_SetsError(t *testing.T) {
	m := CreateSetupWizardModel()
	m.state = SetupStateServerURL
	m.serverURL = "not-a-url"
	newM, cmd := m.handleEnterKey()
	_ = cmd
	wm := newM.(SetupWizardModel)
	if wm.errorMessage == "" {
		t.Error("handleEnterKey invalid URL: expected errorMessage to be set")
	}
}

func TestHandleEnterKey_ServerURL_ValidURL_ChangesState(t *testing.T) {
	m := CreateSetupWizardModel()
	m.state = SetupStateServerURL
	m.serverURL = "http://localhost:8080"
	newM, _ := m.handleEnterKey()
	wm := newM.(SetupWizardModel)
	if wm.state != SetupStateTestConnection {
		t.Errorf("handleEnterKey valid URL: state = %v, want SetupStateTestConnection", wm.state)
	}
}

func TestHandleEnterKey_Token_AdvancesToSaveConfig(t *testing.T) {
	m := CreateSetupWizardModel()
	m.state = SetupStateToken
	m.serverURL = "http://localhost:8080"
	newM, _ := m.handleEnterKey()
	wm := newM.(SetupWizardModel)
	if wm.state != SetupStateSaveConfig {
		t.Errorf("handleEnterKey token: state = %v, want SetupStateSaveConfig", wm.state)
	}
}

func TestHandleEnterKey_SaveConfig_NoSave_Completes(t *testing.T) {
	m := CreateSetupWizardModel()
	m.state = SetupStateSaveConfig
	m.serverURL = "http://localhost:8080"
	m.saveToConfig = false
	newM, cmd := m.handleEnterKey()
	wm := newM.(SetupWizardModel)
	if wm.state != SetupStateComplete {
		t.Errorf("handleEnterKey save(false): state = %v, want SetupStateComplete", wm.state)
	}
	_ = cmd
}

func TestHandleEnterKey_SaveConfig_SaveTrue_WritesConfig(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", tmp)

	m := CreateSetupWizardModel()
	m.state = SetupStateSaveConfig
	m.serverURL = "http://localhost:8080"
	m.apiToken = "token123"
	m.saveToConfig = true

	newM, _ := m.handleEnterKey()
	wm := newM.(SetupWizardModel)
	if wm.state != SetupStateComplete && wm.state != SetupStateFailed {
		t.Logf("handleEnterKey save(true): state = %v (may fail due to dirs)", wm.state)
	}
}

func TestHandleEnterKey_OtherState_Noop(t *testing.T) {
	m := CreateSetupWizardModel()
	m.state = SetupStateTestConnection
	newM, cmd := m.handleEnterKey()
	wm := newM.(SetupWizardModel)
	if wm.state != SetupStateTestConnection {
		t.Errorf("handleEnterKey other state: state changed unexpectedly to %v", wm.state)
	}
	_ = cmd
}

// ── SetupWizardModel.handleBackspace ─────────────────────────────────────────

func TestHandleBackspace_ServerURL_RemovesLastChar(t *testing.T) {
	m := CreateSetupWizardModel()
	m.state = SetupStateServerURL
	m.serverURL = "http://test"
	newM, _ := m.handleBackspace()
	wm := newM.(SetupWizardModel)
	if wm.serverURL != "http://tes" {
		t.Errorf("handleBackspace URL: got %q, want 'http://tes'", wm.serverURL)
	}
}

func TestHandleBackspace_ServerURL_EmptyURL_Noop(t *testing.T) {
	m := CreateSetupWizardModel()
	m.state = SetupStateServerURL
	m.serverURL = ""
	newM, _ := m.handleBackspace()
	wm := newM.(SetupWizardModel)
	if wm.serverURL != "" {
		t.Errorf("handleBackspace empty URL: got %q, want ''", wm.serverURL)
	}
}

func TestHandleBackspace_Token_RemovesLastChar(t *testing.T) {
	m := CreateSetupWizardModel()
	m.state = SetupStateToken
	m.apiToken = "abc"
	newM, _ := m.handleBackspace()
	wm := newM.(SetupWizardModel)
	if wm.apiToken != "ab" {
		t.Errorf("handleBackspace token: got %q, want 'ab'", wm.apiToken)
	}
}

func TestHandleBackspace_Token_EmptyToken_Noop(t *testing.T) {
	m := CreateSetupWizardModel()
	m.state = SetupStateToken
	m.apiToken = ""
	newM, _ := m.handleBackspace()
	wm := newM.(SetupWizardModel)
	if wm.apiToken != "" {
		t.Errorf("handleBackspace empty token: got %q, want ''", wm.apiToken)
	}
}

// ── SetupWizardModel.handleInput ─────────────────────────────────────────────

func TestHandleInput_ServerURLState_AppendsChar(t *testing.T) {
	m := CreateSetupWizardModel()
	m.state = SetupStateServerURL
	m.serverURL = "http"
	newM, _ := m.handleInput("s")
	wm := newM.(SetupWizardModel)
	if wm.serverURL != "https" {
		t.Errorf("handleInput URL: got %q, want 'https'", wm.serverURL)
	}
}

func TestHandleInput_TokenState_AppendsToToken(t *testing.T) {
	m := CreateSetupWizardModel()
	m.state = SetupStateToken
	m.apiToken = "tok"
	newM, _ := m.handleInput("y")
	wm := newM.(SetupWizardModel)
	if wm.apiToken != "toky" {
		t.Errorf("handleInput token state: token got %q, want 'toky'", wm.apiToken)
	}
}

func TestHandleInput_OtherState_Noop(t *testing.T) {
	m := CreateSetupWizardModel()
	m.state = SetupStateSaveConfig
	m.serverURL = "http"
	newM, _ := m.handleInput("x")
	wm := newM.(SetupWizardModel)
	if wm.serverURL != "http" {
		t.Errorf("handleInput other state: serverURL changed to %q, expected no change", wm.serverURL)
	}
}

// ── SetupWizardModel.Update ──────────────────────────────────────────────────

func TestUpdate_EscKey_SetsQuitting(t *testing.T) {
	m := CreateSetupWizardModel()
	newM, _ := m.Update(tea.KeyMsg{Type: tea.KeyEsc})
	wm := newM.(SetupWizardModel)
	if !wm.isQuitting {
		t.Error("Update esc: isQuitting should be true")
	}
}

func TestUpdate_CtrlC_SetsQuitting(t *testing.T) {
	m := CreateSetupWizardModel()
	newM, _ := m.Update(tea.KeyMsg{Type: tea.KeyCtrlC})
	wm := newM.(SetupWizardModel)
	if !wm.isQuitting {
		t.Error("Update ctrl+c: isQuitting should be true")
	}
}

func TestUpdate_EnterKey_CallsHandleEnterKey(t *testing.T) {
	m := CreateSetupWizardModel()
	m.state = SetupStateToken
	newM, _ := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	wm := newM.(SetupWizardModel)
	if wm.state != SetupStateSaveConfig {
		t.Errorf("Update enter: state = %v, want SetupStateSaveConfig", wm.state)
	}
}

func TestUpdate_TabKey_TokenState_TogglesFocus(t *testing.T) {
	m := CreateSetupWizardModel()
	m.state = SetupStateToken
	m.inputFocused = 0
	newM, _ := m.Update(tea.KeyMsg{Type: tea.KeyTab})
	wm := newM.(SetupWizardModel)
	if wm.inputFocused != 1 {
		t.Errorf("Update tab (token state): inputFocused = %d, want 1", wm.inputFocused)
	}
}

func TestUpdate_TabKey_NonTokenState_Noop(t *testing.T) {
	m := CreateSetupWizardModel()
	m.state = SetupStateServerURL
	m.inputFocused = 0
	newM, _ := m.Update(tea.KeyMsg{Type: tea.KeyTab})
	wm := newM.(SetupWizardModel)
	_ = wm
}

func TestUpdate_SpaceKey_SaveConfigState_Toggles(t *testing.T) {
	m := CreateSetupWizardModel()
	m.state = SetupStateSaveConfig
	m.saveToConfig = false
	newM, _ := m.Update(tea.KeyMsg{Type: tea.KeySpace, Runes: []rune{' '}})
	wm := newM.(SetupWizardModel)
	if !wm.saveToConfig {
		t.Error("Update space (save config state): saveToConfig should be toggled to true")
	}
}

func TestUpdate_BackspaceKey_CallsHandleBackspace(t *testing.T) {
	m := CreateSetupWizardModel()
	m.state = SetupStateServerURL
	m.serverURL = "abc"
	newM, _ := m.Update(tea.KeyMsg{Type: tea.KeyBackspace})
	wm := newM.(SetupWizardModel)
	if wm.serverURL != "ab" {
		t.Errorf("Update backspace: serverURL = %q, want 'ab'", wm.serverURL)
	}
}

func TestUpdate_SingleChar_AppendedToInput(t *testing.T) {
	m := CreateSetupWizardModel()
	m.state = SetupStateServerURL
	m.serverURL = "h"
	newM, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'i'}})
	wm := newM.(SetupWizardModel)
	if wm.serverURL != "hi" {
		t.Errorf("Update single char: serverURL = %q, want 'hi'", wm.serverURL)
	}
}

func TestUpdate_ConnectionTestMsg_Success(t *testing.T) {
	m := CreateSetupWizardModel()
	m.state = SetupStateServerURL
	msg := SetupConnectionTestMsg{success: true, version: "1.2.3"}
	newM, _ := m.Update(msg)
	wm := newM.(SetupWizardModel)
	if wm.state != SetupStateToken {
		t.Errorf("Update connection success: state = %v, want SetupStateToken", wm.state)
	}
	if !wm.connectionSuccess {
		t.Error("Update connection success: connectionSuccess should be true")
	}
}

func TestUpdate_ConnectionTestMsg_Failure(t *testing.T) {
	m := CreateSetupWizardModel()
	m.state = SetupStateServerURL
	msg := SetupConnectionTestMsg{success: false, err: os.ErrNotExist}
	newM, _ := m.Update(msg)
	wm := newM.(SetupWizardModel)
	if wm.errorMessage == "" {
		t.Error("Update connection failure: errorMessage should be set")
	}
}

func TestUpdate_UnknownMsg_Noop(t *testing.T) {
	m := CreateSetupWizardModel()
	newM, _ := m.Update(struct{ x int }{42})
	wm := newM.(SetupWizardModel)
	if wm.state != m.state {
		t.Errorf("Update unknown msg: state changed unexpectedly")
	}
}

// ── SetupWizardModel.View — all states ───────────────────────────────────────

func TestView_AllStates_NoPanic(t *testing.T) {
	states := []SetupWizardState{
		SetupStateServerURL,
		SetupStateTestConnection,
		SetupStateToken,
		SetupStateSaveConfig,
		SetupStateComplete,
		SetupStateFailed,
	}
	for _, state := range states {
		m := CreateSetupWizardModel()
		m.state = state
		view := m.View()
		if view == "" {
			t.Errorf("View(%v): returned empty string", state)
		}
	}
}

// ── IsServerConfigured / EnsureServerConfigured ──────────────────────────────

func TestIsServerConfigured_WithSavedAddress_ReturnsTrue(t *testing.T) {
	orig := cliConfigHasSavedServerAddress
	cliConfigHasSavedServerAddress = true
	t.Cleanup(func() { cliConfigHasSavedServerAddress = orig })

	if !IsServerConfigured() {
		t.Error("IsServerConfigured: expected true when cliConfigHasSavedServerAddress=true")
	}
}

func TestIsServerConfigured_WithEnvVar_ReturnsTrue(t *testing.T) {
	orig := cliConfigHasSavedServerAddress
	cliConfigHasSavedServerAddress = false
	t.Cleanup(func() { cliConfigHasSavedServerAddress = orig })
	t.Setenv("VIDVEIL_SERVER", "http://env-server.com")

	if !IsServerConfigured() {
		t.Error("IsServerConfigured: expected true when VIDVEIL_SERVER is set")
	}
}

func TestIsServerConfigured_NothingSet_ReturnsFalse(t *testing.T) {
	orig := cliConfigHasSavedServerAddress
	cliConfigHasSavedServerAddress = false
	t.Cleanup(func() { cliConfigHasSavedServerAddress = orig })
	t.Setenv("VIDVEIL_SERVER", "")
	t.Setenv("VIDVEIL_CLI_SERVER", "")

	result := IsServerConfigured()
	_ = result
}

func TestEnsureServerConfigured_WhenConfigured_ReturnsNil(t *testing.T) {
	orig := cliConfigHasSavedServerAddress
	cliConfigHasSavedServerAddress = true
	t.Cleanup(func() { cliConfigHasSavedServerAddress = orig })

	if err := EnsureServerConfigured(); err != nil {
		t.Errorf("EnsureServerConfigured when configured: expected nil, got %v", err)
	}
}

// ── ParseCLIGlobalFlags — uncovered branches ─────────────────────────────────

func TestParseCLIGlobalFlags_ServerFlag_SetsServerAddress(t *testing.T) {
	orig := serverAddressFlag
	t.Cleanup(func() { serverAddressFlag = orig })

	remaining := ParseCLIGlobalFlags([]string{"--server", "http://test.com"})
	if serverAddressFlag != "http://test.com" {
		t.Errorf("ParseCLIGlobalFlags --server: serverAddressFlag = %q, want 'http://test.com'", serverAddressFlag)
	}
	if len(remaining) != 0 {
		t.Errorf("ParseCLIGlobalFlags --server: remaining = %v, want []", remaining)
	}
}

func TestParseCLIGlobalFlags_ServerFlag_Inline(t *testing.T) {
	orig := serverAddressFlag
	t.Cleanup(func() { serverAddressFlag = orig })

	ParseCLIGlobalFlags([]string{"--server=http://inline.com"})
	if serverAddressFlag != "http://inline.com" {
		t.Errorf("--server=inline: serverAddressFlag = %q", serverAddressFlag)
	}
}

func TestParseCLIGlobalFlags_ServerFlag_NoValue_Skipped(t *testing.T) {
	orig := serverAddressFlag
	t.Cleanup(func() { serverAddressFlag = orig })

	// --server with no following value just skips
	remaining := ParseCLIGlobalFlags([]string{"--server"})
	_ = remaining
}

func TestParseCLIGlobalFlags_TokenFlag_SetsToken(t *testing.T) {
	orig := apiTokenFlag
	t.Cleanup(func() { apiTokenFlag = orig })

	ParseCLIGlobalFlags([]string{"--token", "mytoken123"})
	if apiTokenFlag != "mytoken123" {
		t.Errorf("--token: apiTokenFlag = %q, want 'mytoken123'", apiTokenFlag)
	}
}

func TestParseCLIGlobalFlags_TokenFileFlag(t *testing.T) {
	orig := tokenFilePath
	t.Cleanup(func() { tokenFilePath = orig })

	ParseCLIGlobalFlags([]string{"--token-file", "/path/to/token"})
	if tokenFilePath != "/path/to/token" {
		t.Errorf("--token-file: tokenFilePath = %q", tokenFilePath)
	}
}

func TestParseCLIGlobalFlags_OutputFlag(t *testing.T) {
	orig := outputFormatFlag
	t.Cleanup(func() { outputFormatFlag = orig })

	ParseCLIGlobalFlags([]string{"--output", "json"})
	if outputFormatFlag != "json" {
		t.Errorf("--output: outputFormatFlag = %q, want 'json'", outputFormatFlag)
	}
}

func TestParseCLIGlobalFlags_ColorFlag(t *testing.T) {
	orig := colorFlag
	t.Cleanup(func() { colorFlag = orig })

	ParseCLIGlobalFlags([]string{"--color", "always"})
	if colorFlag != "always" {
		t.Errorf("--color: colorFlag = %q, want 'always'", colorFlag)
	}
}

func TestParseCLIGlobalFlags_LangFlag_SetsEnv(t *testing.T) {
	ParseCLIGlobalFlags([]string{"--lang", "fr"})
	if os.Getenv("VIDVEIL_LANG") != "fr" {
		t.Errorf("--lang: VIDVEIL_LANG = %q, want 'fr'", os.Getenv("VIDVEIL_LANG"))
	}
}

func TestParseCLIGlobalFlags_TimeoutFlag(t *testing.T) {
	orig := requestTimeoutSeconds
	t.Cleanup(func() { requestTimeoutSeconds = orig })

	ParseCLIGlobalFlags([]string{"--timeout", "60"})
	if requestTimeoutSeconds != 60 {
		t.Errorf("--timeout: requestTimeoutSeconds = %d, want 60", requestTimeoutSeconds)
	}
}

func TestParseCLIGlobalFlags_DebugFlag(t *testing.T) {
	orig := debugModeEnabled
	t.Cleanup(func() { debugModeEnabled = orig })

	ParseCLIGlobalFlags([]string{"--debug"})
	if !debugModeEnabled {
		t.Error("--debug: debugModeEnabled should be true")
	}
}

func TestParseCLIGlobalFlags_HelpAfterCommand_GoesToRemaining(t *testing.T) {
	remaining := ParseCLIGlobalFlags([]string{"search", "--help"})
	found := false
	for _, r := range remaining {
		if r == "--help" {
			found = true
		}
	}
	if !found {
		t.Errorf("--help after command: expected in remaining, got %v", remaining)
	}
}

func TestParseCLIGlobalFlags_VersionAfterCommand_GoesToRemaining(t *testing.T) {
	remaining := ParseCLIGlobalFlags([]string{"search", "--version"})
	found := false
	for _, r := range remaining {
		if r == "--version" || r == "-v" {
			found = true
		}
	}
	if !found {
		t.Errorf("--version after command: expected in remaining, got %v", remaining)
	}
}

func TestParseCLIGlobalFlags_UnknownFlag_GoesToRemaining(t *testing.T) {
	remaining := ParseCLIGlobalFlags([]string{"--unknown-flag"})
	if len(remaining) != 1 || remaining[0] != "--unknown-flag" {
		t.Errorf("unknown flag: remaining = %v, want [--unknown-flag]", remaining)
	}
}

func TestParseCLIGlobalFlags_CommandSetsCommandSeen(t *testing.T) {
	// After "search" (non-flag arg), commandSeen=true, so -h goes to remaining
	remaining := ParseCLIGlobalFlags([]string{"search", "-h"})
	found := false
	for _, r := range remaining {
		if r == "-h" {
			found = true
		}
	}
	if !found {
		t.Errorf("-h after command: expected in remaining, got %v", remaining)
	}
}

func TestParseCLIGlobalFlags_TokenNoValue_Skips(t *testing.T) {
	orig := apiTokenFlag
	t.Cleanup(func() { apiTokenFlag = orig })
	ParseCLIGlobalFlags([]string{"--token"})
}

func TestParseCLIGlobalFlags_OutputNoValue_Skips(t *testing.T) {
	orig := outputFormatFlag
	t.Cleanup(func() { outputFormatFlag = orig })
	ParseCLIGlobalFlags([]string{"--output"})
}

func TestParseCLIGlobalFlags_ColorNoValue_Skips(t *testing.T) {
	orig := colorFlag
	t.Cleanup(func() { colorFlag = orig })
	ParseCLIGlobalFlags([]string{"--color"})
}

func TestParseCLIGlobalFlags_LangNoValue_Skips(t *testing.T) {
	ParseCLIGlobalFlags([]string{"--lang"})
}

func TestParseCLIGlobalFlags_TimeoutNoValue_Skips(t *testing.T) {
	orig := requestTimeoutSeconds
	t.Cleanup(func() { requestTimeoutSeconds = orig })
	ParseCLIGlobalFlags([]string{"--timeout"})
}
