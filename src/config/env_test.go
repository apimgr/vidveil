// SPDX-License-Identifier: MIT
package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestFirstRunEnvironmentIsSeededAndThenIgnored(t *testing.T) {
	configDir := t.TempDir()
	dataDir := t.TempDir()
	t.Setenv("APPLICATION_NAME", "First Run Name")
	t.Setenv("APPLICATION_TAGLINE", "First Run Tagline")
	t.Setenv("DATABASE_URL", "libsql://db.invalid?authToken=database-credential")
	t.Setenv("DATABASE_DRIVER", "libsql")
	t.Setenv("CACHE_URL", "valkey://cache-user:cache-credential@cache.invalid:6379/0")
	t.Setenv("VIDVEIL_SERVER_PORT", "64999")

	first, _, err := LoadAppConfig(configDir, dataDir)
	if err != nil {
		t.Fatalf("load first-run config: %v", err)
	}
	if first.Server.Branding.Title != "First Run Name" || first.Server.Branding.Tagline != "First Run Tagline" {
		t.Fatal("first-run branding values were not loaded")
	}
	if first.Server.Database.Driver != "libsql" || first.Server.Database.URL != "libsql://db.invalid?authToken=database-credential" {
		t.Fatal("first-run database connection was not loaded")
	}
	if first.Server.Cache.Type != "valkey" || first.Server.Cache.URL != "valkey://cache-user:cache-credential@cache.invalid:6379/0" {
		t.Fatal("first-run cache connection was not loaded")
	}
	if first.Server.Port == "64999" {
		t.Fatal("unlisted VIDVEIL_SERVER_PORT override changed server.port")
	}

	serverData, err := os.ReadFile(filepath.Join(configDir, "server.yml"))
	if err != nil {
		t.Fatalf("read server config: %v", err)
	}
	secretsData, err := os.ReadFile(filepath.Join(configDir, "secrets.yml"))
	if err != nil {
		t.Fatalf("read secrets config: %v", err)
	}
	brandingData, err := os.ReadFile(filepath.Join(configDir, "yaml.d", "branding.yml"))
	if err != nil {
		t.Fatalf("read branding drop-in: %v", err)
	}
	if strings.Contains(string(serverData), "database-credential") || strings.Contains(string(serverData), "cache-credential") {
		t.Fatal("connection credentials were written to server.yml")
	}
	if !strings.Contains(string(secretsData), "database-credential") || !strings.Contains(string(secretsData), "cache-credential") {
		t.Fatal("connection credentials were not persisted to secrets.yml")
	}
	if !strings.Contains(string(brandingData), "First Run Name") || !strings.Contains(string(brandingData), "First Run Tagline") {
		t.Fatal("first-run branding values were not persisted")
	}

	t.Setenv("APPLICATION_NAME", "Changed Environment Name")
	t.Setenv("APPLICATION_TAGLINE", "Changed Environment Tagline")
	t.Setenv("DATABASE_URL", "libsql://other.invalid?authToken=changed")
	t.Setenv("DATABASE_DRIVER", "sqlite")
	t.Setenv("CACHE_URL", "redis://other.invalid:6379/1")
	second, _, err := LoadAppConfig(configDir, dataDir)
	if err != nil {
		t.Fatalf("reload persisted config: %v", err)
	}
	if second.Server.Branding.Title != "First Run Name" || second.Server.Branding.Tagline != "First Run Tagline" {
		t.Fatal("init-only branding environment variables replaced persisted values")
	}
	if second.Server.Database.Driver != "libsql" || second.Server.Database.URL != first.Server.Database.URL {
		t.Fatal("init-only database environment variables replaced persisted values")
	}
	if second.Server.Cache.Type != "valkey" || second.Server.Cache.URL != first.Server.Cache.URL {
		t.Fatal("init-only cache environment variables replaced persisted values")
	}
}

func TestRuntimeEnvironmentOverridesOnlyI2PSetting(t *testing.T) {
	t.Setenv("I2P_ENABLED", "yes")
	t.Setenv("VIDVEIL_SERVER_PORT", "64999")
	cfg := DefaultAppConfig()
	cfg.Server.Port = "64998"
	applyRuntimeEnvironmentOverrides(cfg)
	if !cfg.Server.I2P.Enabled {
		t.Fatal("I2P_ENABLED did not enable I2P")
	}
	if cfg.Server.Port != "64998" {
		t.Fatal("unlisted environment variable changed server.port")
	}
}
