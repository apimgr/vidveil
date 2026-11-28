// SPDX-License-Identifier: MIT
package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestMigrateConfigLayoutSplitsAreasAndSecrets(t *testing.T) {
	configDir := t.TempDir()
	configPath := filepath.Join(configDir, "server.yml")
	input := "server:\n  port: \"64900\"\n  branding:\n    title: Example\n  token: operator-secret\n"
	if err := os.WriteFile(configPath, []byte(input), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := migrateConfigLayout(configPath); err != nil {
		t.Fatalf("migrateConfigLayout: %v", err)
	}
	serverData, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(serverData), "Example") || strings.Contains(string(serverData), "operator-secret") {
		t.Fatalf("node config retained moved settings: %s", serverData)
	}
	brandingPath := filepath.Join(configDir, "yaml.d", "branding.yml")
	brandingData, err := os.ReadFile(brandingPath)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(brandingData), "Example") {
		t.Fatalf("branding area missing moved value: %s", brandingData)
	}
	secretsPath := filepath.Join(configDir, "secrets.yml")
	secretsData, err := os.ReadFile(secretsPath)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(secretsData), "operator-secret") {
		t.Fatalf("secrets file missing moved value: %s", secretsData)
	}
	for _, path := range []string{configPath + ".bak", brandingPath, secretsPath} {
		info, err := os.Stat(path)
		if err != nil {
			t.Fatal(err)
		}
		if info.Mode().Perm() != 0600 {
			t.Errorf("%s mode = %04o, want 0600", path, info.Mode().Perm())
		}
	}
}

func TestMigrateConfigLayoutPlacesCLISettingsInMaintenanceArea(t *testing.T) {
	configDir := t.TempDir()
	configPath := filepath.Join(configDir, "server.yml")
	if err := os.WriteFile(configPath, []byte("cli:\n  binary_download:\n    dir: '{data_dir}/cli'\n    require_auth: true\n"), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := migrateConfigLayout(configPath); err != nil {
		t.Fatalf("migrateConfigLayout: %v", err)
	}
	serverData, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(serverData), "binary_download") {
		t.Fatalf("server.yml retained CLI download settings: %s", serverData)
	}
	maintenanceData, err := os.ReadFile(filepath.Join(configDir, "yaml.d", "maintenance.yml"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(maintenanceData), "require_auth: true") {
		t.Fatalf("maintenance area missing CLI download settings: %s", maintenanceData)
	}
}

func TestMigrateConfigLayoutDoesNotCreateDropInWithoutChanges(t *testing.T) {
	configDir := t.TempDir()
	configPath := filepath.Join(configDir, "server.yml")
	if err := os.WriteFile(configPath, []byte("server:\n  port: \"64900\"\n  address: 127.0.0.1\n"), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := migrateConfigLayout(configPath); err != nil {
		t.Fatalf("migrateConfigLayout: %v", err)
	}
	if _, err := os.Stat(filepath.Join(configDir, "yaml.d")); !os.IsNotExist(err) {
		t.Fatalf("yaml.d exists despite no migration: %v", err)
	}
}
