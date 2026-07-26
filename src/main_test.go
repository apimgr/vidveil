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
	"os/user"
	"strings"
	"testing"

	"github.com/apimgr/vidveil/src/config"
	"github.com/apimgr/vidveil/src/server/service/system"
	_ "modernc.org/sqlite"
)

// ── loadInstallationSecret / pgpGenerateCore / pgpExportPublicCore ────────────
// AI.md PART 12 GPG Keypair Management. These exercise the error-returning cores
// (no os.Exit) so the generate/export logic is unit-testable.

// TestLoadInstallationSecret verifies a secret is produced and is stable across
// calls against the same data dir (persisted in the DB).
func TestLoadInstallationSecret(t *testing.T) {
	base := t.TempDir()
	dataDir := base + "/data"
	os.MkdirAll(dataDir, 0755)

	secret, err := loadInstallationSecret(dataDir)
	if err != nil {
		t.Fatalf("loadInstallationSecret: %v", err)
	}
	if len(secret) == 0 {
		t.Fatal("empty installation secret")
	}
	secret2, err := loadInstallationSecret(dataDir)
	if err != nil {
		t.Fatalf("loadInstallationSecret second call: %v", err)
	}
	if !bytes.Equal(secret, secret2) {
		t.Fatal("installation secret not stable across calls")
	}
}

// TestPgpGenerateCore_NoSecurityContact verifies generation is refused when no
// security contact email is configured.
func TestPgpGenerateCore_NoSecurityContact(t *testing.T) {
	base := t.TempDir()
	cfgDir := base + "/config"
	dataDir := base + "/data"
	os.MkdirAll(cfgDir, 0755)
	os.MkdirAll(dataDir, 0755)

	cfg, cfgPath, err := config.LoadAppConfig(cfgDir, dataDir)
	if err != nil {
		t.Fatalf("LoadAppConfig: %v", err)
	}
	cfg.Server.Contact.Security.Email = ""
	if err := config.SaveAppConfig(cfg, cfgPath); err != nil {
		t.Fatalf("SaveAppConfig: %v", err)
	}

	_, _, _, _, err = pgpGenerateCore(cfgDir, dataDir)
	if err == nil {
		t.Fatal("expected error when no security contact configured")
	}
	if !strings.Contains(err.Error(), "security contact") {
		t.Errorf("unexpected error: %v", err)
	}
}

// TestPgpExportPublicCore_NoKey verifies exporting before generating fails.
func TestPgpExportPublicCore_NoKey(t *testing.T) {
	base := t.TempDir()
	cfgDir := base + "/config"
	dataDir := base + "/data"
	os.MkdirAll(cfgDir, 0755)
	os.MkdirAll(dataDir, 0755)

	if _, err := pgpExportPublicCore(cfgDir, dataDir, ""); err == nil {
		t.Fatal("expected error exporting public key when none exists")
	}
}

// TestPgpGenerateCore_And_ExportPublic covers the full generate → export flow:
// keypair generation, DB metadata, config flip, and public-key export to both
// stdout (empty outPath) and a file.
func TestPgpGenerateCore_And_ExportPublic(t *testing.T) {
	base := t.TempDir()
	cfgDir := base + "/config"
	dataDir := base + "/data"
	os.MkdirAll(cfgDir, 0755)
	os.MkdirAll(dataDir, 0755)

	cfg, cfgPath, err := config.LoadAppConfig(cfgDir, dataDir)
	if err != nil {
		t.Fatalf("LoadAppConfig: %v", err)
	}
	cfg.Server.Contact.Security.Email = "security@example.com"
	cfg.Server.Branding.Title = "VidVeil"
	if err := config.SaveAppConfig(cfg, cfgPath); err != nil {
		t.Fatalf("SaveAppConfig: %v", err)
	}

	kp, identity, contact, pubPath, err := pgpGenerateCore(cfgDir, dataDir)
	if err != nil {
		t.Fatalf("pgpGenerateCore: %v", err)
	}
	if kp == nil || kp.Fingerprint == "" {
		t.Fatal("expected non-empty keypair")
	}
	if contact != "security@example.com" {
		t.Errorf("contact = %q, want security@example.com", contact)
	}
	if !strings.Contains(identity, "Security") {
		t.Errorf("identity = %q, want it to contain 'Security'", identity)
	}
	if _, err := os.Stat(pubPath); err != nil {
		t.Fatalf("public key not written at %s: %v", pubPath, err)
	}

	pub, err := pgpExportPublicCore(cfgDir, dataDir, "")
	if err != nil {
		t.Fatalf("pgpExportPublicCore stdout: %v", err)
	}
	if !bytes.Contains(pub, []byte("BEGIN PGP PUBLIC KEY BLOCK")) {
		t.Fatal("exported bytes are not an armored public key")
	}

	out := base + "/exported.asc"
	if _, err := pgpExportPublicCore(cfgDir, dataDir, out); err != nil {
		t.Fatalf("pgpExportPublicCore file: %v", err)
	}
	written, err := os.ReadFile(out)
	if err != nil {
		t.Fatalf("read exported: %v", err)
	}
	if !bytes.Equal(written, pub) {
		t.Fatal("file export does not match stdout export")
	}
}

// TestPgpRotateCore_NoExistingKey verifies rotation is refused when there is no
// keypair to rotate from.
func TestPgpRotateCore_NoExistingKey(t *testing.T) {
	base := t.TempDir()
	cfgDir := base + "/config"
	dataDir := base + "/data"
	os.MkdirAll(cfgDir, 0755)
	os.MkdirAll(dataDir, 0755)

	cfg, cfgPath, err := config.LoadAppConfig(cfgDir, dataDir)
	if err != nil {
		t.Fatalf("LoadAppConfig: %v", err)
	}
	cfg.Server.Contact.Security.Email = "security@example.com"
	cfg.Server.Branding.Title = "VidVeil"
	if err := config.SaveAppConfig(cfg, cfgPath); err != nil {
		t.Fatalf("SaveAppConfig: %v", err)
	}

	if _, err := pgpRotateCore(cfgDir, dataDir); err == nil {
		t.Fatal("expected error rotating with no existing keypair")
	}
}

// TestPgpRotateCore covers generate → rotate: the new keypair differs from the
// old, the old key is archived, and the rotation timestamp is stamped.
func TestPgpRotateCore(t *testing.T) {
	base := t.TempDir()
	cfgDir := base + "/config"
	dataDir := base + "/data"
	os.MkdirAll(cfgDir, 0755)
	os.MkdirAll(dataDir, 0755)

	cfg, cfgPath, err := config.LoadAppConfig(cfgDir, dataDir)
	if err != nil {
		t.Fatalf("LoadAppConfig: %v", err)
	}
	cfg.Server.Contact.Security.Email = "security@example.com"
	cfg.Server.Branding.Title = "VidVeil"
	if err := config.SaveAppConfig(cfg, cfgPath); err != nil {
		t.Fatalf("SaveAppConfig: %v", err)
	}

	oldKP, _, _, _, err := pgpGenerateCore(cfgDir, dataDir)
	if err != nil {
		t.Fatalf("pgpGenerateCore: %v", err)
	}

	res, err := pgpRotateCore(cfgDir, dataDir)
	if err != nil {
		t.Fatalf("pgpRotateCore: %v", err)
	}
	if res.newFingerprint == "" || res.newFingerprint == oldKP.Fingerprint {
		t.Fatalf("expected a distinct new fingerprint, got %q (old %q)", res.newFingerprint, oldKP.Fingerprint)
	}
	if res.oldFingerprint != oldKP.Fingerprint {
		t.Errorf("old fingerprint = %q, want %q", res.oldFingerprint, oldKP.Fingerprint)
	}
	if _, err := os.Stat(res.archiveDir); err != nil {
		t.Fatalf("archive dir missing: %v", err)
	}
	if _, err := os.Stat(res.pubKeyPath); err != nil {
		t.Fatalf("new public key missing: %v", err)
	}
}

// TestPgpGenerateAndExportPublic_Wrappers exercises the success paths of the
// thin pgpGenerate / pgpExportPublic CLI wrappers (no os.Exit on success).
func TestPgpGenerateAndExportPublic_Wrappers(t *testing.T) {
	base := t.TempDir()
	cfgDir := base + "/config"
	dataDir := base + "/data"
	os.MkdirAll(cfgDir, 0755)
	os.MkdirAll(dataDir, 0755)

	cfg, cfgPath, err := config.LoadAppConfig(cfgDir, dataDir)
	if err != nil {
		t.Fatalf("LoadAppConfig: %v", err)
	}
	cfg.Server.Contact.Security.Email = "security@example.com"
	cfg.Server.Branding.Title = "VidVeil"
	if err := config.SaveAppConfig(cfg, cfgPath); err != nil {
		t.Fatalf("SaveAppConfig: %v", err)
	}

	out := captureStdout(func() { pgpGenerate(cfgDir, dataDir) })
	if !strings.Contains(out, "Fingerprint:") {
		t.Errorf("pgpGenerate output missing fingerprint line: %q", out)
	}

	dest := base + "/pub.asc"
	out = captureStdout(func() { pgpExportPublic(cfgDir, dataDir, dest) })
	if !strings.Contains(out, dest) {
		t.Errorf("pgpExportPublic output missing path: %q", out)
	}
	if _, err := os.Stat(dest); err != nil {
		t.Fatalf("exported public key not written: %v", err)
	}
}

// TestPrintComplianceReport_Disabled verifies the disabled-mode branch renders
// when no regulatory standards are enabled.
func TestPrintComplianceReport_Disabled(t *testing.T) {
	cfg := config.DefaultAppConfig()
	out := captureStdout(func() { printComplianceReport(cfg) })
	if !strings.Contains(out, "Compliance Report") {
		t.Errorf("missing report header: %q", out)
	}
	if !strings.Contains(out, "DISABLED") {
		t.Errorf("expected disabled mode, got: %q", out)
	}
}

// TestPrintComplianceReport_Enabled verifies the enabled-mode branch lists the
// active standard and the data-subject-request controls it activates.
func TestPrintComplianceReport_Enabled(t *testing.T) {
	cfg := config.DefaultAppConfig()
	cfg.Server.Compliance.GDPR = true
	out := captureStdout(func() { printComplianceReport(cfg) })
	if !strings.Contains(out, "ENABLED") {
		t.Errorf("expected enabled mode, got: %q", out)
	}
	if !strings.Contains(out, "GDPR") {
		t.Errorf("expected GDPR listed, got: %q", out)
	}
	if !strings.Contains(out, "data export") {
		t.Errorf("expected data-subject controls, got: %q", out)
	}
}

// TestAuthorizeViaOperatorToken_NonServiceUser verifies that a non-service,
// non-root user is rejected outright before any token prompt (AI.md PART 5).
func TestAuthorizeViaOperatorToken_NonServiceUser(t *testing.T) {
	base := t.TempDir()
	err := authorizeViaOperatorToken(base+"/config", base+"/data", "confirm: ")
	if err == nil {
		t.Fatal("expected rejection for non-service user")
	}
	if !strings.Contains(err.Error(), "administrator authorization") {
		t.Errorf("unexpected error: %v", err)
	}
}

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
		handleMaintenanceCommand("setup", "", "", "")
	})
	if out == "" {
		t.Error("handleMaintenanceCommand setup: empty output")
	}
}

func TestHandleMaintenanceCommand_Setup_ContainsServerYML(t *testing.T) {
	out := captureStdout(func() {
		handleMaintenanceCommand("setup", "", "", "")
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
		handleMaintenanceCommand("backup", "", cfgDir, dataDir)
	})
}

// TestHandleMaintenanceCommand_BackupWithPassword covers the backup path when
// stdin has no data (EOF): the interactive encryption prompt reads as "no" per
// AI.md PART 21 (no --password flag), so this exercises the plain-backup branch.
func TestHandleMaintenanceCommand_Backup_WithPassword_NoPanic(t *testing.T) {
	base := t.TempDir()
	cfgDir := base + "/config"
	dataDir := base + "/data"
	os.MkdirAll(cfgDir, 0755)
	os.MkdirAll(dataDir, 0755)
	t.Setenv("BACKUP_DIR", base+"/backup")
	os.MkdirAll(base+"/backup", 0755)

	captureStdout(func() {
		handleMaintenanceCommand("backup", "", cfgDir, dataDir)
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
		handleMaintenanceCommand("mode", "on", cfgDir, dataDir)
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
		handleMaintenanceCommand("mode", "off", cfgDir, dataDir)
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
		handleMaintenanceCommand("mode", "true", cfgDir, dataDir)
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
		handleMaintenanceCommand("mode", "false", cfgDir, dataDir)
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
		handleMaintenanceCommand("backup", "", cfgDir, dataDir)
	})

	captureStdout(func() {
		handleMaintenanceCommand("restore", "", cfgDir, dataDir)
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

// ── authorizeRestore / authorizeSensitiveOperation / isDatabaseEmpty ──────────
// Per AI.md PART 5 "Sensitive Operations". These tests run as root inside the
// Docker build container, so only the empty-database and root-allowed paths
// are exercised; the operator-token and non-root-rejected paths require a
// non-root/service-user process and are covered by manual/integration testing.

func TestIsDatabaseEmpty_NoDBFile_ReturnsTrue(t *testing.T) {
	base := t.TempDir()
	cfgDir := base + "/config"
	dataDir := base + "/data"
	os.MkdirAll(cfgDir, 0755)
	os.MkdirAll(dataDir, 0755)

	if !isDatabaseEmpty(cfgDir, dataDir) {
		t.Error("isDatabaseEmpty with no db file: expected true")
	}
}

func TestIsDatabaseEmpty_WithSettingsRow_ReturnsFalse(t *testing.T) {
	base := t.TempDir()
	cfgDir := base + "/config"
	dataDir := base + "/data"
	os.MkdirAll(cfgDir, 0755)
	dbDir := dataDir + "/db"
	os.MkdirAll(dbDir, 0755)

	db, err := sql.Open("sqlite", dbDir+"/server.db")
	if err != nil {
		t.Fatal("sql.Open:", err)
	}
	if _, err := db.Exec("CREATE TABLE settings (key TEXT, value TEXT)"); err != nil {
		t.Fatal("CREATE TABLE:", err)
	}
	if _, err := db.Exec("INSERT INTO settings VALUES ('key', 'val')"); err != nil {
		t.Fatal("INSERT:", err)
	}
	db.Close()

	if isDatabaseEmpty(cfgDir, dataDir) {
		t.Error("isDatabaseEmpty with settings rows: expected false")
	}
}

func TestAuthorizeRestore_EmptyDatabase_Allowed(t *testing.T) {
	base := t.TempDir()
	cfgDir := base + "/config"
	dataDir := base + "/data"
	os.MkdirAll(cfgDir, 0755)
	os.MkdirAll(dataDir, 0755)

	if err := authorizeRestore(cfgDir, dataDir); err != nil {
		t.Errorf("authorizeRestore on empty database: expected nil, got %v", err)
	}
}

func TestAuthorizeRestore_NonEmptyDatabase_AsRoot_Allowed(t *testing.T) {
	if !system.IsRunningAsRoot() {
		t.Skip("requires root (test container runs as root)")
	}
	base := t.TempDir()
	cfgDir := base + "/config"
	dataDir := base + "/data"
	os.MkdirAll(cfgDir, 0755)
	dbDir := dataDir + "/db"
	os.MkdirAll(dbDir, 0755)

	db, err := sql.Open("sqlite", dbDir+"/server.db")
	if err != nil {
		t.Fatal("sql.Open:", err)
	}
	if _, err := db.Exec("CREATE TABLE settings (key TEXT, value TEXT)"); err != nil {
		t.Fatal("CREATE TABLE:", err)
	}
	if _, err := db.Exec("INSERT INTO settings VALUES ('key', 'val')"); err != nil {
		t.Fatal("INSERT:", err)
	}
	db.Close()

	captureStdout(func() {
		if err := authorizeRestore(cfgDir, dataDir); err != nil {
			t.Errorf("authorizeRestore as root on non-empty database: expected nil, got %v", err)
		}
	})
}

func TestAuthorizeSensitiveOperation_AsRoot_Allowed(t *testing.T) {
	if !system.IsRunningAsRoot() {
		t.Skip("requires root (test container runs as root)")
	}
	base := t.TempDir()
	cfgDir := base + "/config"
	dataDir := base + "/data"
	os.MkdirAll(cfgDir, 0755)
	os.MkdirAll(dataDir, 0755)

	captureStdout(func() {
		if err := authorizeSensitiveOperation(cfgDir, dataDir); err != nil {
			t.Errorf("authorizeSensitiveOperation as root: expected nil, got %v", err)
		}
	})
}

func TestAuthorizeViaOperatorToken_NonServiceUser_Rejected(t *testing.T) {
	base := t.TempDir()
	cfgDir := base + "/config"
	dataDir := base + "/data"
	os.MkdirAll(cfgDir, 0755)
	os.MkdirAll(dataDir, 0755)

	// The test process never runs as the "vidveil" service user (it runs as
	// whatever CI/dev account invoked `go test`, typically root), so this
	// exercises the "any other user -> rejected" branch of AI.md PART 5
	// "Sensitive Operations" -> Restore/Mode-change authorization flow.
	currentUser, err := user.Current()
	if err != nil {
		t.Skip("user.Current unavailable in this environment")
	}
	if currentUser.Username == "vidveil" {
		t.Skip("test process is running as the vidveil service user")
	}

	err = authorizeViaOperatorToken(cfgDir, dataDir, "Enter operator token to confirm: ")
	if err == nil {
		t.Fatal("authorizeViaOperatorToken as non-service-user: expected error, got nil")
	}
	if !strings.Contains(err.Error(), "requires administrator authorization") {
		t.Errorf("authorizeViaOperatorToken as non-service-user: unexpected error message: %v", err)
	}
}
