// SPDX-License-Identifier: MIT
// Coverage tests for the tor CLI helpers in tor_cli.go.
// Functions requiring a running server or a Tor process are exercised
// only through their error/fallback paths.
package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// ── torDirs ───────────────────────────────────────────────────────────────────

func TestTorDirs_JoinsTorAndSite(t *testing.T) {
	base := t.TempDir()
	torDir, siteDir := torDirs(filepath.Join(base, "config"), filepath.Join(base, "data"))
	if torDir != filepath.Join(base, "data", "tor") {
		t.Errorf("torDirs: torDir = %q, want %q", torDir, filepath.Join(base, "data", "tor"))
	}
	if siteDir != filepath.Join(torDir, "site") {
		t.Errorf("torDirs: siteDir = %q, want %q", siteDir, filepath.Join(torDir, "site"))
	}
}

// ── readHostnameFile ──────────────────────────────────────────────────────────

func TestReadHostnameFile_Missing(t *testing.T) {
	if got := readHostnameFile(t.TempDir()); got != "" {
		t.Errorf("readHostnameFile on missing file = %q, want empty", got)
	}
}

func TestReadHostnameFile_TrimsWhitespace(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "hostname"), []byte("abcd1234.onion\n"), 0600); err != nil {
		t.Fatal(err)
	}
	if got := readHostnameFile(dir); got != "abcd1234.onion" {
		t.Errorf("readHostnameFile = %q, want %q", got, "abcd1234.onion")
	}
}

// ── printTorHelp ──────────────────────────────────────────────────────────────

func TestPrintTorHelp_ListsAllSubcommands(t *testing.T) {
	out := captureStdout(printTorHelp)
	// Per AI.md PART 31 CLI table: all seven operations must be documented
	for _, cmd := range []string{"tor status", "tor validate", "tor restart", "tor regenerate", "tor vanity start", "tor vanity apply", "tor import-keys"} {
		if !strings.Contains(out, cmd) {
			t.Errorf("printTorHelp: missing %q", cmd)
		}
	}
}

// ── handleTorCommand ──────────────────────────────────────────────────────────

func TestHandleTorCommand_NoArgs(t *testing.T) {
	var code int
	captureStdout(func() { code = handleTorCommand(nil, "", "") })
	if code != 1 {
		t.Errorf("handleTorCommand(nil) = %d, want 1", code)
	}
}

func TestHandleTorCommand_Help(t *testing.T) {
	var code int
	out := captureStdout(func() { code = handleTorCommand([]string{"help"}, "", "") })
	if code != 0 {
		t.Errorf("handleTorCommand(help) = %d, want 0", code)
	}
	if !strings.Contains(out, "Tor Hidden Service Commands") {
		t.Error("handleTorCommand(help): missing help header")
	}
}

func TestHandleTorCommand_Unknown(t *testing.T) {
	var code int
	captureStdout(func() { code = handleTorCommand([]string{"bogus"}, "", "") })
	if code != 1 {
		t.Errorf("handleTorCommand(bogus) = %d, want 1", code)
	}
}

func TestHandleTorCommand_VanityMissingSubcommand(t *testing.T) {
	if code := handleTorCommand([]string{"vanity"}, "", ""); code != 1 {
		t.Errorf("handleTorCommand(vanity) = %d, want 1", code)
	}
}

func TestHandleTorCommand_VanityStartMissingPrefix(t *testing.T) {
	if code := handleTorCommand([]string{"vanity", "start"}, "", ""); code != 1 {
		t.Errorf("handleTorCommand(vanity start) = %d, want 1", code)
	}
}

func TestHandleTorCommand_VanityUnknown(t *testing.T) {
	if code := handleTorCommand([]string{"vanity", "bogus"}, "", ""); code != 1 {
		t.Errorf("handleTorCommand(vanity bogus) = %d, want 1", code)
	}
}

func TestHandleTorCommand_ImportKeysMissingPath(t *testing.T) {
	if code := handleTorCommand([]string{"import-keys"}, "", ""); code != 1 {
		t.Errorf("handleTorCommand(import-keys) = %d, want 1", code)
	}
}

// ── torStatus (server not running, disk fallback) ─────────────────────────────

func TestTorStatus_ServerStoppedWithHostname(t *testing.T) {
	base := t.TempDir()
	configDir := filepath.Join(base, "config")
	dataDir := filepath.Join(base, "data")
	siteDir := filepath.Join(dataDir, "tor", "site")
	if err := os.MkdirAll(siteDir, 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(siteDir, "hostname"), []byte("teststatus.onion\n"), 0600); err != nil {
		t.Fatal(err)
	}

	var code int
	out := captureStdout(func() { code = torStatus(configDir, dataDir) })
	if code != 0 {
		t.Errorf("torStatus = %d, want 0", code)
	}
	if !strings.Contains(out, "teststatus.onion") {
		t.Errorf("torStatus: missing stored address, got %q", out)
	}
}

func TestTorStatus_ServerStoppedNoKeys(t *testing.T) {
	base := t.TempDir()
	var code int
	out := captureStdout(func() {
		code = torStatus(filepath.Join(base, "config"), filepath.Join(base, "data"))
	})
	if code != 0 {
		t.Errorf("torStatus = %d, want 0", code)
	}
	if !strings.Contains(out, "server not running") {
		t.Errorf("torStatus: missing stopped notice, got %q", out)
	}
}

// ── torValidate ───────────────────────────────────────────────────────────────

func TestTorValidate_FreshDirs(t *testing.T) {
	base := t.TempDir()
	var code int
	out := captureStdout(func() {
		code = torValidate(filepath.Join(base, "config"), filepath.Join(base, "data"))
	})
	if code != 0 {
		t.Errorf("torValidate = %d, want 0 (missing tor binary is a warning, not an error)", code)
	}
	if !strings.Contains(out, "Config:") {
		t.Errorf("torValidate: missing config line, got %q", out)
	}
}

func TestTorValidate_SecretKeyWithoutHostname(t *testing.T) {
	base := t.TempDir()
	configDir := filepath.Join(base, "config")
	dataDir := filepath.Join(base, "data")
	siteDir := filepath.Join(dataDir, "tor", "site")
	if err := os.MkdirAll(siteDir, 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(siteDir, "hs_ed25519_secret_key"), []byte("stub"), 0600); err != nil {
		t.Fatal(err)
	}

	var code int
	captureStdout(func() { code = torValidate(configDir, dataDir) })
	if code != 1 {
		t.Errorf("torValidate with orphaned secret key = %d, want 1", code)
	}
}

// ── torRestart / signalTorProcess ─────────────────────────────────────────────

func TestTorRestart_NoPIDFile(t *testing.T) {
	base := t.TempDir()
	var code int
	captureStdout(func() {
		code = torRestart(filepath.Join(base, "config"), filepath.Join(base, "data"))
	})
	if code != 1 {
		t.Errorf("torRestart without tor.pid = %d, want 1", code)
	}
}

func TestSignalTorProcess_InvalidPID(t *testing.T) {
	torDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(torDir, "tor.pid"), []byte("not-a-pid\n"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := signalTorProcess(torDir); err == nil {
		t.Error("signalTorProcess with invalid pid: expected error")
	}
}

// ── torRegenerate ─────────────────────────────────────────────────────────────

func TestTorRegenerate_GeneratesNewAddress(t *testing.T) {
	base := t.TempDir()
	configDir := filepath.Join(base, "config")
	dataDir := filepath.Join(base, "data")

	var code int
	out := captureStdout(func() { code = torRegenerate(configDir, dataDir) })
	if code != 0 {
		t.Fatalf("torRegenerate = %d, want 0", code)
	}
	if !strings.Contains(out, ".onion") {
		t.Errorf("torRegenerate: missing .onion address, got %q", out)
	}

	siteDir := filepath.Join(dataDir, "tor", "site")
	hostname := readHostnameFile(siteDir)
	if !strings.HasSuffix(hostname, ".onion") {
		t.Errorf("torRegenerate: hostname file = %q, want .onion suffix", hostname)
	}
	if _, err := os.Stat(filepath.Join(siteDir, "hs_ed25519_secret_key")); err != nil {
		t.Error("torRegenerate: secret key not written")
	}
}

// ── torImportKeys ─────────────────────────────────────────────────────────────

func TestTorImportKeys_MissingFile(t *testing.T) {
	base := t.TempDir()
	var code int
	captureStdout(func() {
		code = torImportKeys(filepath.Join(base, "config"), filepath.Join(base, "data"), filepath.Join(base, "nope.key"))
	})
	if code != 1 {
		t.Errorf("torImportKeys with missing file = %d, want 1", code)
	}
}

func TestTorImportKeys_RoundTrip(t *testing.T) {
	base := t.TempDir()
	configDir := filepath.Join(base, "config")
	dataDir := filepath.Join(base, "data")

	// Generate keys first, then export and re-import them
	var code int
	captureStdout(func() { code = torRegenerate(configDir, dataDir) })
	if code != 0 {
		t.Fatal("torRegenerate failed")
	}

	siteDir := filepath.Join(dataDir, "tor", "site")
	originalHostname := readHostnameFile(siteDir)
	keyData, err := os.ReadFile(filepath.Join(siteDir, "hs_ed25519_secret_key"))
	if err != nil {
		t.Fatal(err)
	}
	exported := filepath.Join(base, "exported.key")
	if err := os.WriteFile(exported, keyData, 0600); err != nil {
		t.Fatal(err)
	}

	// Re-import must reproduce the same address
	captureStdout(func() { code = torImportKeys(configDir, dataDir, exported) })
	if code != 0 {
		t.Fatalf("torImportKeys = %d, want 0", code)
	}
	if got := readHostnameFile(siteDir); got != originalHostname {
		t.Errorf("torImportKeys: hostname = %q, want %q", got, originalHostname)
	}
}

// ── torVanityApply ────────────────────────────────────────────────────────────

func TestTorVanityApply_NoPending(t *testing.T) {
	base := t.TempDir()
	var code int
	captureStdout(func() {
		code = torVanityApply(filepath.Join(base, "config"), filepath.Join(base, "data"))
	})
	if code != 1 {
		t.Errorf("torVanityApply without pending keys = %d, want 1", code)
	}
}
