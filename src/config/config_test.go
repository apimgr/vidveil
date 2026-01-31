// SPDX-License-Identifier: MIT
package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestDefaultAppConfig(t *testing.T) {
	cfg := DefaultAppConfig()

	if cfg == nil {
		t.Fatal("DefaultAppConfig() returned nil")
	}

	// Check server defaults
	if cfg.Server.Mode != "production" {
		t.Errorf("Expected mode 'production', got '%s'", cfg.Server.Mode)
	}

	if cfg.Server.Branding.Title != "Vidveil" {
		t.Errorf("Expected title 'Vidveil', got '%s'", cfg.Server.Branding.Title)
	}

	if cfg.Server.Address != "[::]" {
		t.Errorf("Expected address '[::]', got '%s'", cfg.Server.Address)
	}

	// Check web defaults
	if cfg.Web.UI.Theme != "dark" {
		t.Errorf("Expected theme 'dark', got '%s'", cfg.Web.UI.Theme)
	}

	// Check search defaults
	if cfg.Search.ResultsPerPage != 50 {
		t.Errorf("Expected 50 results per page, got %d", cfg.Search.ResultsPerPage)
	}
}

func TestParseBool(t *testing.T) {
	tests := []struct {
		input    string
		expected bool
	}{
		// Truthy values
		{"1", true},
		{"yes", true},
		{"YES", true},
		{"true", true},
		{"TRUE", true},
		{"enable", true},
		{"enabled", true},
		{"on", true},
		{"ON", true},
		// Falsy values
		{"0", false},
		{"no", false},
		{"NO", false},
		{"false", false},
		{"FALSE", false},
		{"disable", false},
		{"disabled", false},
		{"off", false},
		{"OFF", false},
		{"", false},
		// Invalid defaults to false
		{"invalid", false},
		{"maybe", false},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result := ParseBool(tt.input)
			if result != tt.expected {
				t.Errorf("ParseBool(%q) = %v, want %v", tt.input, result, tt.expected)
			}
		})
	}
}

func TestNormalizeMode(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"dev", "development"},
		{"DEV", "development"},
		{"development", "development"},
		{"DEVELOPMENT", "development"},
		{"prod", "production"},
		{"PROD", "production"},
		{"production", "production"},
		{"PRODUCTION", "production"},
		{"", "production"},
		{"invalid", "production"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result := NormalizeMode(tt.input)
			if result != tt.expected {
				t.Errorf("NormalizeMode(%q) = %q, want %q", tt.input, result, tt.expected)
			}
		})
	}
}

func TestGetAppPaths(t *testing.T) {
	// Test with custom paths
	customConfig := "/tmp/test-config"
	customData := "/tmp/test-data"

	paths := GetAppPaths(customConfig, customData)

	if paths.Config != customConfig {
		t.Errorf("Expected config path %q, got %q", customConfig, paths.Config)
	}

	if paths.Data != customData {
		t.Errorf("Expected data path %q, got %q", customData, paths.Data)
	}
}

func TestIsValidHost(t *testing.T) {
	tests := []struct {
		host     string
		devMode  bool
		expected bool
	}{
		// Production mode - valid domains
		{"example.com", false, true},
		{"api.example.com", false, true},
		// Production mode - not allowed
		{"localhost", false, false},
		{"test.local", false, false},
		{"192.168.1.1", false, false},
		{"::1", false, false},
		// Development mode - allowed
		{"localhost", true, true},
		{"test.local", true, true},
		{"example.com", true, true},
		// IPs never allowed even in dev
		{"192.168.1.1", true, false},
	}

	for _, tt := range tests {
		name := tt.host
		if tt.devMode {
			name += "_dev"
		}
		t.Run(name, func(t *testing.T) {
			result := IsValidHost(tt.host, tt.devMode)
			if result != tt.expected {
				t.Errorf("IsValidHost(%q, %v) = %v, want %v", tt.host, tt.devMode, result, tt.expected)
			}
		})
	}
}

func TestLoadAndSave(t *testing.T) {
	// Create temp directory
	tmpDir := filepath.Join(os.TempDir(), "apimgr-test", "vidveil-config-test")
	defer os.RemoveAll(tmpDir)

	configDir := filepath.Join(tmpDir, "config")
	dataDir := filepath.Join(tmpDir, "data")

	// LoadAppConfig should create default config
	cfg, configPath, err := LoadAppConfig(configDir, dataDir)
	if err != nil {
		t.Fatalf("LoadAppConfig() error: %v", err)
	}

	if cfg == nil {
		t.Fatal("LoadAppConfig() returned nil config")
	}

	if configPath == "" {
		t.Fatal("LoadAppConfig() returned empty config path")
	}

	// Verify config file was created
	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		t.Errorf("Config file not created at %s", configPath)
	}

	// Modify and save
	cfg.Server.Branding.Title = "Test Title"
	if err := SaveAppConfig(cfg, configPath); err != nil {
		t.Fatalf("SaveAppConfig() error: %v", err)
	}

	// Reload and verify
	cfg2, _, err := LoadAppConfig(configDir, dataDir)
	if err != nil {
		t.Fatalf("LoadAppConfig() after save error: %v", err)
	}

	if cfg2.Server.Branding.Title != "Test Title" {
		t.Errorf("Expected title 'Test Title', got '%s'", cfg2.Server.Branding.Title)
	}
}

func TestGetFQDN(t *testing.T) {
	// Test with DOMAIN env var
	os.Setenv("DOMAIN", "test.example.com")
	defer os.Unsetenv("DOMAIN")

	fqdn := GetFQDN()
	if fqdn != "test.example.com" {
		t.Errorf("Expected FQDN 'test.example.com', got '%s'", fqdn)
	}
}

func TestGetDisplayHost(t *testing.T) {
	cfg := DefaultAppConfig()

	// Set DOMAIN to test
	os.Setenv("DOMAIN", "vidveil.example.com")
	defer os.Unsetenv("DOMAIN")

	host := GetDisplayHost(cfg)

	// Should return the FQDN, not localhost
	if host == "localhost" || host == "127.0.0.1" || host == "0.0.0.0" {
		t.Errorf("GetDisplayHost returned loopback address: %s", host)
	}
}

func TestIsDevelopmentMode(t *testing.T) {
	cfg := DefaultAppConfig()

	cfg.Server.Mode = "production"
	if cfg.IsDevelopmentMode() {
		t.Error("Expected production mode, got development")
	}

	cfg.Server.Mode = "development"
	if !cfg.IsDevelopmentMode() {
		t.Error("Expected development mode, got production")
	}
}
