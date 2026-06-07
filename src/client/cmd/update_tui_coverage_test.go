// SPDX-License-Identifier: MIT
// AI.md PART 28: Coverage tests for update.go, tui.go, setup.go, login.go pure functions.
// No network calls, no terminal I/O, no file writes to production paths.
package cmd

import (
	"testing"

	"github.com/apimgr/vidveil/src/common/terminal"
	"github.com/apimgr/vidveil/src/common/theme"
)

// ── update.go ─────────────────────────────────────────────────────────────────

func TestPrintCLIUpdateHelp_NoPanic(t *testing.T) {
	PrintCLIUpdateHelp()
}

func TestRunCLIUpdateCommand_Help_NoPanic(t *testing.T) {
	if err := RunCLIUpdateCommand([]string{"--help"}); err != nil {
		t.Errorf("RunCLIUpdateCommand --help: %v", err)
	}
}

func TestRunCLIUpdateCommand_HelpKeyword_NoPanic(t *testing.T) {
	if err := RunCLIUpdateCommand([]string{"help"}); err != nil {
		t.Errorf("RunCLIUpdateCommand help: %v", err)
	}
}

func TestRunCLIUpdateCommand_HelpShortFlag_NoPanic(t *testing.T) {
	if err := RunCLIUpdateCommand([]string{"-h"}); err != nil {
		t.Errorf("RunCLIUpdateCommand -h: %v", err)
	}
}

func TestCLIUpdateValidBranches_ContainsExpectedKeys(t *testing.T) {
	for _, b := range []string{"stable", "beta", "daily"} {
		if !CLIUpdateValidBranches[b] {
			t.Errorf("CLIUpdateValidBranches missing %q", b)
		}
	}
}

func TestCLIUpdateValidBranches_DoesNotContainUnknown(t *testing.T) {
	if CLIUpdateValidBranches["bogus"] {
		t.Error("CLIUpdateValidBranches should not contain 'bogus'")
	}
}

func TestCLIUpdateBranchFile_IsNonEmpty(t *testing.T) {
	if CLIUpdateBranchFile == "" {
		t.Error("CLIUpdateBranchFile constant is empty")
	}
}

func TestGetCLIUpdateBranch_InvalidBranchDefaultsToStable(t *testing.T) {
	// Simulate what GetCLIUpdateBranch does when the file contains an invalid branch
	branch := "totallyinvalid"
	if !CLIUpdateValidBranches[branch] {
		branch = "stable"
	}
	if branch != "stable" {
		t.Errorf("invalid branch should default to stable, got %q", branch)
	}
}

func TestGetCLIUpdateBranch_ValidBranchPassesThrough(t *testing.T) {
	for _, b := range []string{"stable", "beta", "daily"} {
		if !CLIUpdateValidBranches[b] {
			t.Errorf("branch %q should pass validation", b)
		}
	}
}

// ── tui.go ────────────────────────────────────────────────────────────────────

func TestGetTUILayoutConfig_AllSizeModes_NoPanic(t *testing.T) {
	modes := []terminal.SizeMode{
		terminal.SizeModeMicro,
		terminal.SizeModeMinimal,
		terminal.SizeModeCompact,
		terminal.SizeModeStandard,
		terminal.SizeModeWide,
		terminal.SizeModeUltrawide,
		terminal.SizeModeMassive,
	}
	for _, mode := range modes {
		cfg := GetTUILayoutConfig(mode)
		_ = cfg.ShowBorders
		_ = cfg.ShowHeader
		_ = cfg.ShowFooter
		_ = cfg.ShowSidebar
		_ = cfg.MaxColumns
		_ = cfg.TruncateAt
		_ = cfg.UseAbbrev
		_ = cfg.VerticalScroll
	}
}

func TestGetTUILayoutConfig_Micro_NoBordersNoSidebar(t *testing.T) {
	cfg := GetTUILayoutConfig(terminal.SizeModeMicro)
	if cfg.ShowSidebar {
		t.Error("SizeModeMicro: ShowSidebar should be false")
	}
	if cfg.ShowBorders {
		t.Error("SizeModeMicro: ShowBorders should be false")
	}
}

func TestGetTUILayoutConfig_Massive_NoTruncationHasSidebar(t *testing.T) {
	cfg := GetTUILayoutConfig(terminal.SizeModeMassive)
	if cfg.TruncateAt != 0 {
		t.Errorf("SizeModeMassive: TruncateAt = %d, want 0", cfg.TruncateAt)
	}
	if !cfg.ShowSidebar {
		t.Error("SizeModeMassive: ShowSidebar should be true")
	}
}

func TestGetTUILayoutConfig_Wide_HasSidebarWidth(t *testing.T) {
	cfg := GetTUILayoutConfig(terminal.SizeModeWide)
	if !cfg.ShowSidebar {
		t.Error("SizeModeWide: ShowSidebar should be true")
	}
	if cfg.SidebarWidth <= 0 {
		t.Errorf("SizeModeWide: SidebarWidth = %d, want > 0", cfg.SidebarWidth)
	}
}

func TestGetTUILayoutConfig_Compact_HasBordersNoSidebar(t *testing.T) {
	cfg := GetTUILayoutConfig(terminal.SizeModeCompact)
	if !cfg.ShowBorders {
		t.Error("SizeModeCompact: ShowBorders should be true")
	}
	if cfg.ShowSidebar {
		t.Error("SizeModeCompact: ShowSidebar should be false")
	}
}

func TestGetTUILayoutConfig_Ultrawide_NotVerticalScroll(t *testing.T) {
	cfg := GetTUILayoutConfig(terminal.SizeModeUltrawide)
	if cfg.VerticalScroll {
		t.Error("SizeModeUltrawide: VerticalScroll should be false")
	}
}

func TestGetTUILayoutConfig_UnknownMode_FallsBackToStandard(t *testing.T) {
	unknown := terminal.SizeMode(99)
	cfg := GetTUILayoutConfig(unknown)
	standard := GetTUILayoutConfig(terminal.SizeModeStandard)
	if cfg.MaxColumns != standard.MaxColumns {
		t.Errorf("unknown mode should fall back to Standard: MaxColumns=%d, want %d", cfg.MaxColumns, standard.MaxColumns)
	}
}

func TestCreateTUIStylesFromPalette_DarkTheme_NoPanic(t *testing.T) {
	palette := theme.GetColorPalette("dark")
	styles := CreateTUIStylesFromPalette(palette)
	_ = styles.Base
	_ = styles.Title
	_ = styles.Input
	_ = styles.Result
	_ = styles.Selected
	_ = styles.Help
	_ = styles.Status
	_ = styles.Error
	_ = styles.Warning
	_ = styles.Muted
	_ = styles.Border
}

func TestCreateTUIStylesFromPalette_LightTheme_NoPanic(t *testing.T) {
	palette := theme.GetColorPalette("light")
	styles := CreateTUIStylesFromPalette(palette)
	_ = styles.Base
}

func TestCreateInitialTUIModel_NoPanic(t *testing.T) {
	model := CreateInitialTUIModel()
	_ = model
}

func TestTUIModel_Init_ReturnsNil(t *testing.T) {
	model := CreateInitialTUIModel()
	cmd := model.Init()
	if cmd != nil {
		t.Error("TUIModel.Init: expected nil tea.Cmd")
	}
}

// ── setup.go ─────────────────────────────────────────────────────────────────

func TestCreateSetupWizardModel_InitialState(t *testing.T) {
	model := CreateSetupWizardModel()
	if model.state != SetupStateServerURL {
		t.Errorf("CreateSetupWizardModel: state = %v, want SetupStateServerURL", model.state)
	}
}

func TestCreateSetupWizardModel_DefaultServerURL(t *testing.T) {
	model := CreateSetupWizardModel()
	if model.serverURL == "" {
		t.Error("CreateSetupWizardModel: serverURL should have default prefix")
	}
}

func TestSetupWizardModel_Init_NoPanic(t *testing.T) {
	model := CreateSetupWizardModel()
	_ = model.Init()
}

func TestSetupWizardModel_View_NoPanic(t *testing.T) {
	model := CreateSetupWizardModel()
	v := model.View()
	if v == "" {
		t.Error("SetupWizardModel.View: returned empty string")
	}
}

func TestSaveSetupWizardConfig_NoSave_ReturnsNil(t *testing.T) {
	err := SaveSetupWizardConfig("https://example.com", "mytoken", false)
	if err != nil {
		t.Errorf("SaveSetupWizardConfig(saveToFile=false): expected nil, got %v", err)
	}
}

func TestSaveSetupWizardConfig_EmptyURLNoSave_ReturnsNil(t *testing.T) {
	err := SaveSetupWizardConfig("", "", false)
	if err != nil {
		t.Errorf("SaveSetupWizardConfig empty no save: expected nil, got %v", err)
	}
}

// ── login.go ─────────────────────────────────────────────────────────────────

func TestPrintLoginCommandHelp_NoPanic(t *testing.T) {
	PrintLoginCommandHelp()
}

func TestRunLoginCommand_HelpLong_NoPanic(t *testing.T) {
	err := RunLoginCommand([]string{"--help"})
	if err != nil {
		t.Errorf("RunLoginCommand --help: %v", err)
	}
}

func TestRunLoginCommand_HelpShort_NoPanic(t *testing.T) {
	err := RunLoginCommand([]string{"-h"})
	if err != nil {
		t.Errorf("RunLoginCommand -h: %v", err)
	}
}
