// SPDX-License-Identifier: MIT
// AI.md PART 28: Coverage tests for uncovered LoadAppConfig and validateConfig branches.
package config

import (
	"os"
	"path/filepath"
	"testing"
)

// ── LoadAppConfig: YAML parse error ──────────────────────────────────────────

func TestLoadAppConfig_InvalidYAML_ReturnsError(t *testing.T) {
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "server.yml")
	if err := os.WriteFile(cfgPath, []byte(":\nbroken: [\nyaml"), 0644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	_, _, err := LoadAppConfig(dir, t.TempDir())
	if err == nil {
		t.Error("LoadAppConfig with invalid YAML: expected error, got nil")
	}
}

// ── LoadAppConfig: DATABASE_DIR env overrides sqlite dir ─────────────────────

func TestLoadAppConfig_DatabaseDirEnv_OverridesDefault(t *testing.T) {
	dir := t.TempDir()
	dataDir := t.TempDir()
	dbOverride := t.TempDir()

	// Create a valid minimal config file so LoadAppConfig reads it (not the default-save path).
	cfg := DefaultAppConfig()
	cfg.Server.Database.SQLite.Dir = ""
	if err := SaveAppConfig(cfg, filepath.Join(dir, "server.yml")); err != nil {
		t.Fatalf("SaveAppConfig: %v", err)
	}

	t.Setenv("DATABASE_DIR", dbOverride)

	loaded, _, err := LoadAppConfig(dir, dataDir)
	if err != nil {
		t.Fatalf("LoadAppConfig: %v", err)
	}
	if loaded.Server.Database.SQLite.Dir != GetDatabaseDir(GetAppPaths(dir, dataDir).Data) {
		t.Errorf("DATABASE_DIR env: SQLite.Dir = %q, want computed dbDir", loaded.Server.Database.SQLite.Dir)
	}
}

// ── LoadAppConfig: server.yaml → server.yml migration ────────────────────────

func TestLoadAppConfig_YamlToYmlMigration(t *testing.T) {
	dir := t.TempDir()
	dataDir := t.TempDir()

	// Create server.yaml but NOT server.yml — triggers the rename migration.
	cfg := DefaultAppConfig()
	yamlPath := filepath.Join(dir, "server.yaml")
	if err := SaveAppConfig(cfg, yamlPath); err != nil {
		t.Fatalf("SaveAppConfig to server.yaml: %v", err)
	}

	loaded, cfgPath, err := LoadAppConfig(dir, dataDir)
	if err != nil {
		t.Fatalf("LoadAppConfig after yaml→yml migration: %v", err)
	}
	if loaded == nil {
		t.Fatal("LoadAppConfig returned nil config")
	}
	// After migration the returned path should end in server.yml.
	if filepath.Base(cfgPath) != "server.yml" {
		t.Errorf("migrated config path: got %q, want ...server.yml", cfgPath)
	}
}

// ── validateConfig: SSL + LetsEncrypt + no email ─────────────────────────────

func TestValidateConfig_SSLLetsEncryptNoEmail(t *testing.T) {
	cfg := DefaultAppConfig()
	cfg.Server.SSL.Enabled = true
	cfg.Server.SSL.LetsEncrypt.Enabled = true
	cfg.Server.SSL.LetsEncrypt.Email = ""
	// Must not panic; warning is written to stderr.
	validateConfig(cfg)
}

// ── validateConfig: port list with trailing comma (empty port entry) ──────────

func TestValidateConfig_PortListWithEmptyEntry(t *testing.T) {
	cfg := DefaultAppConfig()
	// Trailing comma produces an empty string token in the split.
	cfg.Server.Port = "8080,"
	validateConfig(cfg)
	// Port should remain unchanged since 8080 is valid.
	if cfg.Server.Port == "" {
		t.Error("validateConfig: Port should not be cleared for valid port with trailing comma")
	}
}
