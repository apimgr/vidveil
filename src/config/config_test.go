// SPDX-License-Identifier: MIT
package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestDefault(t *testing.T) {
	cfg := Default()

	if cfg == nil {
		t.Fatal("Default() returned nil")
	}

	// Check server defaults
	if cfg.Server.Mode != "production" {
		t.Errorf("Expected mode 'production', got '%s'", cfg.Server.Mode)
	}

	if cfg.Server.Title != "Vidveil" {
		t.Errorf("Expected title 'Vidveil', got '%s'", cfg.Server.Title)
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

func TestGetPaths(t *testing.T) {
	// Test with custom paths
	customConfig := "/tmp/test-config"
	customData := "/tmp/test-data"

	paths := GetPaths(customConfig, customData)

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
		// Production mode
		{"example.com", false, true},
		{"api.example.com", false, true},
		{"localhost", false, false},        // Not allowed in prod
		{"test.local", false, false},       // Dev TLD
		{"192.168.1.1", false, false},      // IP not allowed
		{"::1", false, false},              // IPv6 loopback
		// Development mode
		{"localhost", true, true},          // Allowed in dev
		{"test.local", true, true},         // Dev TLD allowed
		{"example.com", true, true},
		{"192.168.1.1", true, false},       // IPs never allowed
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

	// Load should create default config
	cfg, configPath, err := Load(configDir, dataDir)
	if err != nil {
		t.Fatalf("Load() error: %v", err)
	}

	if cfg == nil {
		t.Fatal("Load() returned nil config")
	}

	if configPath == "" {
		t.Fatal("Load() returned empty config path")
	}

	// Verify config file was created
	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		t.Errorf("Config file not created at %s", configPath)
	}

	// Modify and save
	cfg.Server.Title = "Test Title"
	if err := Save(cfg, configPath); err != nil {
		t.Fatalf("Save() error: %v", err)
	}

	// Reload and verify
	cfg2, _, err := Load(configDir, dataDir)
	if err != nil {
		t.Fatalf("Load() after save error: %v", err)
	}

	if cfg2.Server.Title != "Test Title" {
		t.Errorf("Expected title 'Test Title', got '%s'", cfg2.Server.Title)
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
	cfg := Default()

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
	cfg := Default()

	cfg.Server.Mode = "production"
	if cfg.IsDevelopmentMode() {
		t.Error("Expected production mode, got development")
	}

	cfg.Server.Mode = "development"
	if !cfg.IsDevelopmentMode() {
		t.Error("Expected development mode, got production")
	}
}
