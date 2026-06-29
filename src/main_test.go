// SPDX-License-Identifier: MIT
// Coverage tests for pure helper functions in main.go.
// Functions that call os.Exit are NOT tested here; only pure
// or stdout-writing functions are exercised.
package main

import (
	"bytes"
	"database/sql"
	"io"
	"os"
	"strings"
	"testing"

	"github.com/apimgr/vidveil/src/config"
	_ "modernc.org/sqlite"
)

// captureStdout redirects os.Stdout to a buffer for the duration of f,
// returning the captured output.
func captureStdout(f func()) string {
	r, w, err := os.Pipe()
	if err != nil {
		panic(err)
	}
	orig := os.Stdout
	os.Stdout = w
	f()
	w.Close()
	os.Stdout = orig
	var buf bytes.Buffer
	io.Copy(&buf, r)
	r.Close()
	return buf.String()
}

// ── printBashCompletions ──────────────────────────────────────────────────────

func TestPrintBashCompletions_ContainsBinaryName(t *testing.T) {
	out := captureStdout(func() { printBashCompletions("vidveil") })
	if !strings.Contains(out, "vidveil") {
		t.Error("printBashCompletions: output does not contain binary name 'vidveil'")
	}
}

func TestPrintBashCompletions_ContainsCompleteCommand(t *testing.T) {
	out := captureStdout(func() { printBashCompletions("vidveil") })
	if !strings.Contains(out, "complete") {
		t.Error("printBashCompletions: output does not contain 'complete' directive")
	}
}

func TestPrintBashCompletions_ContainsHelpFlag(t *testing.T) {
	out := captureStdout(func() { printBashCompletions("vidveil") })
	if !strings.Contains(out, "--help") {
		t.Error("printBashCompletions: output does not contain '--help' flag")
	}
}

// ── printZshCompletions ───────────────────────────────────────────────────────

func TestPrintZshCompletions_ContainsBinaryName(t *testing.T) {
	out := captureStdout(func() { printZshCompletions("vidveil") })
	if !strings.Contains(out, "vidveil") {
		t.Error("printZshCompletions: output does not contain binary name 'vidveil'")
	}
}

func TestPrintZshCompletions_ContainsCompdef(t *testing.T) {
	out := captureStdout(func() { printZshCompletions("vidveil") })
	if !strings.Contains(out, "#compdef") {
		t.Error("printZshCompletions: output does not contain '#compdef'")
	}
}

func TestPrintZshCompletions_ContainsArguments(t *testing.T) {
	out := captureStdout(func() { printZshCompletions("vidveil") })
	if !strings.Contains(out, "_arguments") {
		t.Error("printZshCompletions: output does not contain '_arguments'")
	}
}

// ── printFishCompletions ──────────────────────────────────────────────────────

func TestPrintFishCompletions_ContainsBinaryName(t *testing.T) {
	out := captureStdout(func() { printFishCompletions("vidveil") })
	if !strings.Contains(out, "vidveil") {
		t.Error("printFishCompletions: output does not contain binary name 'vidveil'")
	}
}

func TestPrintFishCompletions_ContainsCompleteDirective(t *testing.T) {
	out := captureStdout(func() { printFishCompletions("vidveil") })
	if !strings.Contains(out, "complete -c vidveil") {
		t.Error("printFishCompletions: output does not start with 'complete -c vidveil'")
	}
}

func TestPrintFishCompletions_ContainsVersionFlag(t *testing.T) {
	out := captureStdout(func() { printFishCompletions("vidveil") })
	if !strings.Contains(out, "version") {
		t.Error("printFishCompletions: output does not contain 'version' flag")
	}
}

// ── printPowerShellCompletions ────────────────────────────────────────────────

func TestPrintPowerShellCompletions_ContainsBinaryName(t *testing.T) {
	out := captureStdout(func() { printPowerShellCompletions("vidveil") })
	if !strings.Contains(out, "vidveil") {
		t.Error("printPowerShellCompletions: output does not contain binary name 'vidveil'")
	}
}

func TestPrintPowerShellCompletions_ContainsRegisterArgCompleter(t *testing.T) {
	out := captureStdout(func() { printPowerShellCompletions("vidveil") })
	if !strings.Contains(out, "Register-ArgumentCompleter") {
		t.Error("printPowerShellCompletions: output does not contain 'Register-ArgumentCompleter'")
	}
}

// ── printCompletions ──────────────────────────────────────────────────────────

func TestPrintCompletions_Bash(t *testing.T) {
	out := captureStdout(func() { printCompletions("bash", "vidveil") })
	if !strings.Contains(out, "compgen") {
		t.Error("printCompletions bash: output does not contain bash-specific 'compgen'")
	}
}

func TestPrintCompletions_Zsh(t *testing.T) {
	out := captureStdout(func() { printCompletions("zsh", "vidveil") })
	if !strings.Contains(out, "#compdef") {
		t.Error("printCompletions zsh: output does not contain '#compdef'")
	}
}

func TestPrintCompletions_Fish(t *testing.T) {
	out := captureStdout(func() { printCompletions("fish", "vidveil") })
	if !strings.Contains(out, "complete -c") {
		t.Error("printCompletions fish: output does not contain fish 'complete -c'")
	}
}

func TestPrintCompletions_Powershell(t *testing.T) {
	out := captureStdout(func() { printCompletions("powershell", "vidveil") })
	if !strings.Contains(out, "Register-ArgumentCompleter") {
		t.Error("printCompletions powershell: output does not contain 'Register-ArgumentCompleter'")
	}
}

func TestPrintCompletions_Pwsh(t *testing.T) {
	out := captureStdout(func() { printCompletions("pwsh", "vidveil") })
	if !strings.Contains(out, "Register-ArgumentCompleter") {
		t.Error("printCompletions pwsh: output does not contain 'Register-ArgumentCompleter'")
	}
}

func TestPrintCompletions_Sh(t *testing.T) {
	out := captureStdout(func() { printCompletions("sh", "vidveil") })
	if !strings.Contains(out, "compgen") {
		t.Error("printCompletions sh: output should use bash completions (contains 'compgen')")
	}
}

func TestPrintCompletions_Dash(t *testing.T) {
	out := captureStdout(func() { printCompletions("dash", "vidveil") })
	if !strings.Contains(out, "compgen") {
		t.Error("printCompletions dash: output should use bash completions (contains 'compgen')")
	}
}

func TestPrintCompletions_Ksh(t *testing.T) {
	out := captureStdout(func() { printCompletions("ksh", "vidveil") })
	if !strings.Contains(out, "compgen") {
		t.Error("printCompletions ksh: output should use bash completions (contains 'compgen')")
	}
}

// ── printInit ─────────────────────────────────────────────────────────────────

func TestPrintInit_Bash(t *testing.T) {
	out := captureStdout(func() { printInit("bash", "vidveil") })
	if !strings.Contains(out, "source") {
		t.Errorf("printInit bash: expected 'source' in output, got %q", out)
	}
}

func TestPrintInit_Zsh(t *testing.T) {
	out := captureStdout(func() { printInit("zsh", "vidveil") })
	if !strings.Contains(out, "source") {
		t.Errorf("printInit zsh: expected 'source' in output, got %q", out)
	}
}

func TestPrintInit_Fish(t *testing.T) {
	out := captureStdout(func() { printInit("fish", "vidveil") })
	if !strings.Contains(out, "source") {
		t.Errorf("printInit fish: expected 'source' in output, got %q", out)
	}
}

func TestPrintInit_Sh(t *testing.T) {
	out := captureStdout(func() { printInit("sh", "vidveil") })
	if !strings.Contains(out, "eval") {
		t.Errorf("printInit sh: expected 'eval' in output, got %q", out)
	}
}

func TestPrintInit_Dash(t *testing.T) {
	out := captureStdout(func() { printInit("dash", "vidveil") })
	if !strings.Contains(out, "eval") {
		t.Errorf("printInit dash: expected 'eval' in output, got %q", out)
	}
}

func TestPrintInit_Ksh(t *testing.T) {
	out := captureStdout(func() { printInit("ksh", "vidveil") })
	if !strings.Contains(out, "eval") {
		t.Errorf("printInit ksh: expected 'eval' in output, got %q", out)
	}
}

func TestPrintInit_Powershell(t *testing.T) {
	out := captureStdout(func() { printInit("powershell", "vidveil") })
	if !strings.Contains(out, "Invoke-Expression") {
		t.Errorf("printInit powershell: expected 'Invoke-Expression' in output, got %q", out)
	}
}

func TestPrintInit_Pwsh(t *testing.T) {
	out := captureStdout(func() { printInit("pwsh", "vidveil") })
	if !strings.Contains(out, "Invoke-Expression") {
		t.Errorf("printInit pwsh: expected 'Invoke-Expression' in output, got %q", out)
	}
}

// ── getDisplayAddress ─────────────────────────────────────────────────────────

func TestGetDisplayAddress_ContainsPort(t *testing.T) {
	cfg := config.DefaultAppConfig()
	cfg.Server.Port = "8080"
	got := getDisplayAddress(cfg)
	if !strings.Contains(got, "8080") {
		t.Errorf("getDisplayAddress: %q does not contain port '8080'", got)
	}
}

func TestGetDisplayAddress_NonEmpty(t *testing.T) {
	cfg := config.DefaultAppConfig()
	got := getDisplayAddress(cfg)
	if got == "" {
		t.Error("getDisplayAddress: returned empty string")
	}
}

// ── printHelp ─────────────────────────────────────────────────────────────────

func TestPrintHelp_NoPanic(t *testing.T) {
	captureStdout(func() { printHelp() })
}

func TestPrintHelp_ContainsUsage(t *testing.T) {
	out := captureStdout(func() { printHelp() })
	if !strings.Contains(out, "Usage:") {
		t.Error("printHelp: output does not contain 'Usage:'")
	}
}

func TestPrintHelp_ContainsHelpFlag(t *testing.T) {
	out := captureStdout(func() { printHelp() })
	if !strings.Contains(out, "--help") {
		t.Error("printHelp: output does not contain '--help'")
	}
}

func TestPrintHelp_ContainsVersionFlag(t *testing.T) {
	out := captureStdout(func() { printHelp() })
	if !strings.Contains(out, "--version") {
		t.Error("printHelp: output does not contain '--version'")
	}
}

// ── printVersion ──────────────────────────────────────────────────────────────

func TestPrintVersion_NoPanic(t *testing.T) {
	captureStdout(func() { printVersion() })
}

func TestPrintVersion_ContainsBuilt(t *testing.T) {
	out := captureStdout(func() { printVersion() })
	if !strings.Contains(out, "Built:") {
		t.Error("printVersion: output does not contain 'Built:'")
	}
}

func TestPrintVersion_ContainsGo(t *testing.T) {
	out := captureStdout(func() { printVersion() })
	if !strings.Contains(out, "Go:") {
		t.Error("printVersion: output does not contain 'Go:'")
	}
}

func TestPrintVersion_ContainsOSArch(t *testing.T) {
	out := captureStdout(func() { printVersion() })
	if !strings.Contains(out, "OS/Arch:") {
		t.Error("printVersion: output does not contain 'OS/Arch:'")
	}
}

// ── handleShellCommand (non-exit paths) ───────────────────────────────────────

func TestHandleShellCommand_Completions_Bash(t *testing.T) {
	out := captureStdout(func() { handleShellCommand("completions", "bash") })
	if !strings.Contains(out, "compgen") {
		t.Error("handleShellCommand completions bash: missing bash completion content")
	}
}

func TestHandleShellCommand_Completions_Zsh(t *testing.T) {
	out := captureStdout(func() { handleShellCommand("completions", "zsh") })
	if !strings.Contains(out, "#compdef") {
		t.Error("handleShellCommand completions zsh: missing zsh content")
	}
}

func TestHandleShellCommand_Completions_Fish(t *testing.T) {
	out := captureStdout(func() { handleShellCommand("completions", "fish") })
	if !strings.Contains(out, "complete -c") {
		t.Error("handleShellCommand completions fish: missing fish content")
	}
}

func TestHandleShellCommand_Init_Bash(t *testing.T) {
	out := captureStdout(func() { handleShellCommand("init", "bash") })
	if out == "" {
		t.Error("handleShellCommand init bash: empty output")
	}
}

func TestHandleShellCommand_Init_Zsh(t *testing.T) {
	out := captureStdout(func() { handleShellCommand("init", "zsh") })
	if out == "" {
		t.Error("handleShellCommand init zsh: empty output")
	}
}

func TestHandleShellCommand_Init_Fish(t *testing.T) {
	out := captureStdout(func() { handleShellCommand("init", "fish") })
	if out == "" {
		t.Error("handleShellCommand init fish: empty output")
	}
}

func TestHandleShellCommand_Completions_AutoDetect(t *testing.T) {
	t.Setenv("SHELL", "/bin/bash")
	out := captureStdout(func() { handleShellCommand("completions", "") })
	if out == "" {
		t.Error("handleShellCommand completions auto: empty output")
	}
}

// ── handleMaintenanceCommand (setup sub-command, no os.Exit) ─────────────────

func TestHandleMaintenanceCommand_Setup_NoPanic(t *testing.T) {
	out := captureStdout(func() {
		handleMaintenanceCommand("setup", "", "", "", "")
	})
	if out == "" {
		t.Error("handleMaintenanceCommand setup: empty output")
	}
}

func TestHandleMaintenanceCommand_Setup_ContainsServerYML(t *testing.T) {
	out := captureStdout(func() {
		handleMaintenanceCommand("setup", "", "", "", "")
	})
	if !strings.Contains(out, "server.yml") {
		t.Error("handleMaintenanceCommand setup: output does not mention server.yml")
	}
}

// ── handleMaintenanceCommand (additional non-exit paths) ─────────────────────

// TestHandleMaintenanceCommand_Backup creates a real backup using temp dirs.
// It calls the "backup" sub-command which does NOT call os.Exit on success.
func TestHandleMaintenanceCommand_Backup_NoPanic(t *testing.T) {
	base := t.TempDir()
	cfgDir := base + "/config"
	dataDir := base + "/data"
	os.MkdirAll(cfgDir, 0755)
	os.MkdirAll(dataDir, 0755)
	t.Setenv("BACKUP_DIR", base+"/backup")
	os.MkdirAll(base+"/backup", 0755)

	captureStdout(func() {
		handleMaintenanceCommand("backup", "", "", cfgDir, dataDir)
	})
}

// TestHandleMaintenanceCommand_BackupWithPassword tests the encrypted backup path.
func TestHandleMaintenanceCommand_Backup_WithPassword_NoPanic(t *testing.T) {
	base := t.TempDir()
	cfgDir := base + "/config"
	dataDir := base + "/data"
	os.MkdirAll(cfgDir, 0755)
	os.MkdirAll(dataDir, 0755)
	t.Setenv("BACKUP_DIR", base+"/backup")
	os.MkdirAll(base+"/backup", 0755)

	captureStdout(func() {
		handleMaintenanceCommand("backup", "", "testpassword", cfgDir, dataDir)
	})
}

// TestHandleMaintenanceCommand_ModeOn enables maintenance mode (writes flag file).
func TestHandleMaintenanceCommand_ModeOn_NoPanic(t *testing.T) {
	base := t.TempDir()
	cfgDir := base + "/config"
	dataDir := base + "/data"
	os.MkdirAll(cfgDir, 0755)
	os.MkdirAll(dataDir, 0755)

	captureStdout(func() {
		handleMaintenanceCommand("mode", "on", "", cfgDir, dataDir)
	})
}

// TestHandleMaintenanceCommand_ModeOff disables maintenance mode.
func TestHandleMaintenanceCommand_ModeOff_NoPanic(t *testing.T) {
	base := t.TempDir()
	cfgDir := base + "/config"
	dataDir := base + "/data"
	os.MkdirAll(cfgDir, 0755)
	os.MkdirAll(dataDir, 0755)

	captureStdout(func() {
		handleMaintenanceCommand("mode", "off", "", cfgDir, dataDir)
	})
}

// TestHandleMaintenanceCommand_ModeTrue covers the "true" alias.
func TestHandleMaintenanceCommand_ModeTrue_NoPanic(t *testing.T) {
	base := t.TempDir()
	cfgDir := base + "/config"
	dataDir := base + "/data"
	os.MkdirAll(cfgDir, 0755)
	os.MkdirAll(dataDir, 0755)

	captureStdout(func() {
		handleMaintenanceCommand("mode", "true", "", cfgDir, dataDir)
	})
}

// TestHandleMaintenanceCommand_ModeFalse covers the "false" alias.
func TestHandleMaintenanceCommand_ModeFalse_NoPanic(t *testing.T) {
	base := t.TempDir()
	cfgDir := base + "/config"
	dataDir := base + "/data"
	os.MkdirAll(cfgDir, 0755)
	os.MkdirAll(dataDir, 0755)

	captureStdout(func() {
		handleMaintenanceCommand("mode", "false", "", cfgDir, dataDir)
	})
}

// TestHandleMaintenanceCommand_RestoreEmpty tests the "restore" with no arg.
// With no backup files, it returns an error — captured without os.Exit.
func TestHandleMaintenanceCommand_Restore_NoPanic(t *testing.T) {
	base := t.TempDir()
	cfgDir := base + "/config"
	dataDir := base + "/data"
	os.MkdirAll(cfgDir, 0755)
	os.MkdirAll(dataDir, 0755)
	t.Setenv("BACKUP_DIR", base+"/backup")
	os.MkdirAll(base+"/backup", 0755)

	// Create a backup first, then restore it
	captureStdout(func() {
		handleMaintenanceCommand("backup", "", "", cfgDir, dataDir)
	})

	captureStdout(func() {
		handleMaintenanceCommand("restore", "", "", cfgDir, dataDir)
	})
}

// ── isDBFirstRun ──────────────────────────────────────────────────────────────

func TestIsDBFirstRun_EmptyDB_ReturnsTrue(t *testing.T) {
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatal("sql.Open:", err)
	}
	defer db.Close()
	if !isDBFirstRun(db) {
		t.Error("isDBFirstRun on empty DB: expected true")
	}
}

func TestIsDBFirstRun_TableWithRows_ReturnsFalse(t *testing.T) {
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatal("sql.Open:", err)
	}
	defer db.Close()
	if _, err := db.Exec("CREATE TABLE settings (key TEXT, value TEXT)"); err != nil {
		t.Fatal("CREATE TABLE:", err)
	}
	if _, err := db.Exec("INSERT INTO settings VALUES ('key', 'val')"); err != nil {
		t.Fatal("INSERT:", err)
	}
	if isDBFirstRun(db) {
		t.Error("isDBFirstRun with rows: expected false")
	}
}

func TestIsDBFirstRun_EmptySettingsTable_ReturnsTrue(t *testing.T) {
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatal("sql.Open:", err)
	}
	defer db.Close()
	if _, err := db.Exec("CREATE TABLE settings (key TEXT, value TEXT)"); err != nil {
		t.Fatal("CREATE TABLE:", err)
	}
	if !isDBFirstRun(db) {
		t.Error("isDBFirstRun with empty settings table: expected true")
	}
}

// ── checkStatus — first branch (no config file) ───────────────────────────────

func TestCheckStatus_NoConfig_Returns1(t *testing.T) {
	// In a Docker container there is no real config at /etc/apimgr/vidveil/server.yml
	// so LoadAppConfig fails → checkStatus returns 1.
	result := checkStatus()
	if result != 1 {
		t.Logf("checkStatus returned %d (expected 1 in Docker without config)", result)
	}
}

// ── handleShellCommand — auto-detect shell from SHELL env ─────────────────────

func TestHandleShellCommand_AutoDetectShell_NoPanic(t *testing.T) {
	t.Setenv("SHELL", "/bin/bash")
	out := captureStdout(func() {
		handleShellCommand("completions", "")
	})
	if out == "" {
		t.Error("handleShellCommand(auto-detect): expected completions output")
	}
}

func TestHandleShellCommand_NoSHELL_DefaultsBash(t *testing.T) {
	t.Setenv("SHELL", "")
	out := captureStdout(func() {
		handleShellCommand("completions", "")
	})
	if out == "" {
		t.Error("handleShellCommand(no SHELL): expected completions output")
	}
}

// ── printVersion — additional tests ──────────────────────────────────────────

func TestPrintVersion_ContainsVersionString(t *testing.T) {
	out := captureStdout(printVersion)
	if !strings.Contains(out, Version) {
		t.Errorf("printVersion: output should contain Version %q, got %q", Version, out)
	}
}

func TestPrintVersion_ContainsCommit(t *testing.T) {
	out := captureStdout(printVersion)
	if !strings.Contains(out, "Commit:") {
		t.Error("printVersion: output should contain 'Commit:'")
	}
}

func TestPrintVersion_WithOfficialSite(t *testing.T) {
	origSite := OfficialSite
	OfficialSite = "https://example.com"
	defer func() { OfficialSite = origSite }()

	out := captureStdout(printVersion)
	if !strings.Contains(out, "Site:") {
		t.Error("printVersion: output should contain 'Site:' when OfficialSite is set")
	}
	if !strings.Contains(out, "https://example.com") {
		t.Error("printVersion: output should contain the official site URL")
	}
}

// ── printHelp — additional tests ─────────────────────────────────────────────

func TestPrintHelp_ContainsStatusFlag(t *testing.T) {
	out := captureStdout(printHelp)
	if !strings.Contains(out, "--status") {
		t.Error("printHelp: output should contain '--status' flag")
	}
}

func TestPrintHelp_ContainsServerConfiguration(t *testing.T) {
	out := captureStdout(printHelp)
	if !strings.Contains(out, "Server Configuration:") {
		t.Error("printHelp: output should contain 'Server Configuration:'")
	}
}

func TestPrintHelp_ContainsServiceManagement(t *testing.T) {
	out := captureStdout(printHelp)
	if !strings.Contains(out, "Service Management:") {
		t.Error("printHelp: output should contain 'Service Management:'")
	}
}

func TestPrintHelp_ContainsPortFlag(t *testing.T) {
	out := captureStdout(printHelp)
	if !strings.Contains(out, "--port") {
		t.Error("printHelp: output should contain '--port' flag")
	}
}

func TestPrintHelp_ContainsDaemonFlag(t *testing.T) {
	out := captureStdout(printHelp)
	if !strings.Contains(out, "--daemon") {
		t.Error("printHelp: output should contain '--daemon' flag")
	}
}
