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

func TestGetAppPathsUsesEnvOverrides(t *testing.T) {
	t.Setenv("CONFIG_DIR", "/tmp/env-config")
	t.Setenv("DATA_DIR", "/tmp/env-data")
	t.Setenv("CACHE_DIR", "/tmp/env-cache")
	t.Setenv("LOG_DIR", "/tmp/env-log")
	t.Setenv("BACKUP_DIR", "/tmp/env-backup")

	paths := GetAppPaths("", "")

	if paths.Config != "/tmp/env-config" {
		t.Errorf("Expected config path from env, got %q", paths.Config)
	}

	if paths.Data != "/tmp/env-data" {
		t.Errorf("Expected data path from env, got %q", paths.Data)
	}

	if paths.Cache != "/tmp/env-cache" {
		t.Errorf("Expected cache path from env, got %q", paths.Cache)
	}

	if paths.Log != "/tmp/env-log" {
		t.Errorf("Expected log path from env, got %q", paths.Log)
	}

	if paths.Backup != "/tmp/env-backup" {
		t.Errorf("Expected backup path from env, got %q", paths.Backup)
	}
}

func TestGetDatabaseDirUsesEnvOverride(t *testing.T) {
	t.Setenv("DATABASE_DIR", "/tmp/env-sqlite")

	dbDir := GetDatabaseDir("/tmp/env-data")
	if dbDir != "/tmp/env-sqlite" {
		t.Errorf("Expected database dir from env, got %q", dbDir)
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

// TestParseBoolWithDefault covers truthy input, falsy input, empty string (uses default), and invalid input.
func TestParseBoolWithDefault(t *testing.T) {
	tests := []struct {
		input      string
		defaultVal bool
		wantVal    bool
		wantErr    bool
	}{
		{"yes", false, true, false},
		{"true", false, true, false},
		{"1", false, true, false},
		{"on", false, true, false},
		{"no", true, false, false},
		{"false", true, false, false},
		{"0", true, false, false},
		{"off", true, false, false},
		// Empty string returns the default value
		{"", true, true, false},
		{"", false, false, false},
		// Invalid value returns false and an error
		{"maybe", false, false, true},
		{"invalid", true, false, true},
	}

	for _, tt := range tests {
		name := tt.input
		if name == "" {
			name = "(empty)"
		}
		t.Run(name, func(t *testing.T) {
			got, err := ParseBoolWithDefault(tt.input, tt.defaultVal)
			if (err != nil) != tt.wantErr {
				t.Errorf("ParseBoolWithDefault(%q, %v) error = %v, wantErr %v", tt.input, tt.defaultVal, err, tt.wantErr)
			}
			if got != tt.wantVal {
				t.Errorf("ParseBoolWithDefault(%q, %v) = %v, want %v", tt.input, tt.defaultVal, got, tt.wantVal)
			}
		})
	}
}

// TestMustParseBool verifies correct value on valid input and panic on invalid input.
func TestMustParseBool(t *testing.T) {
	if got := MustParseBool("yes", false); got != true {
		t.Errorf("MustParseBool(\"yes\", false) = %v, want true", got)
	}
	if got := MustParseBool("no", true); got != false {
		t.Errorf("MustParseBool(\"no\", true) = %v, want false", got)
	}
	// Empty string returns the default without panic
	if got := MustParseBool("", true); got != true {
		t.Errorf("MustParseBool(\"\", true) = %v, want true", got)
	}

	// Invalid input must panic
	defer func() {
		if r := recover(); r == nil {
			t.Error("MustParseBool with invalid input did not panic")
		}
	}()
	MustParseBool("maybe", false)
}

// TestIsFalsy verifies that falsy strings return true and truthy/empty/invalid return false.
func TestIsFalsy(t *testing.T) {
	falsy := []string{"0", "no", "false", "off", "disable", "disabled", "nope", "nah"}
	for _, s := range falsy {
		if !IsFalsy(s) {
			t.Errorf("IsFalsy(%q) = false, want true", s)
		}
	}

	notFalsy := []string{"yes", "true", "1", "on", "", "invalid"}
	for _, s := range notFalsy {
		if IsFalsy(s) {
			t.Errorf("IsFalsy(%q) = true, want false", s)
		}
	}
}

// TestIsValidBool verifies truthy and falsy strings are valid; empty and invalid are not.
func TestIsValidBool(t *testing.T) {
	valid := []string{"yes", "no", "true", "false", "1", "0", "on", "off", "enable", "disable"}
	for _, s := range valid {
		if !IsValidBool(s) {
			t.Errorf("IsValidBool(%q) = false, want true", s)
		}
	}

	invalid := []string{"", "maybe", "invalid", "yesno"}
	for _, s := range invalid {
		if IsValidBool(s) {
			t.Errorf("IsValidBool(%q) = true, want false", s)
		}
	}
}

// TestParseBoolEnv covers env set to truthy, falsy, unset, and invalid values.
func TestParseBoolEnv(t *testing.T) {
	// Env set to truthy value
	t.Setenv("TEST_BOOL_ENV_TRUTHY", "yes")
	if got := ParseBoolEnv("TEST_BOOL_ENV_TRUTHY", false); got != true {
		t.Errorf("ParseBoolEnv truthy: got %v, want true", got)
	}

	// Env set to falsy value
	t.Setenv("TEST_BOOL_ENV_FALSY", "no")
	if got := ParseBoolEnv("TEST_BOOL_ENV_FALSY", true); got != false {
		t.Errorf("ParseBoolEnv falsy: got %v, want false", got)
	}

	// Env unset — returns default
	if got := ParseBoolEnv("TEST_BOOL_ENV_UNSET_XYZ", true); got != true {
		t.Errorf("ParseBoolEnv unset default true: got %v, want true", got)
	}
	if got := ParseBoolEnv("TEST_BOOL_ENV_UNSET_XYZ", false); got != false {
		t.Errorf("ParseBoolEnv unset default false: got %v, want false", got)
	}

	// Env set to invalid value — returns default
	t.Setenv("TEST_BOOL_ENV_INVALID", "maybe")
	if got := ParseBoolEnv("TEST_BOOL_ENV_INVALID", true); got != true {
		t.Errorf("ParseBoolEnv invalid: got %v, want default true", got)
	}
}

// TestIsRunningInContainer just calls the function to ensure no panic. The result
// depends on the runtime environment and is not asserted.
func TestIsRunningInContainer(t *testing.T) {
	_ = IsRunningInContainer()
}

// TestIsProductionMode verifies development mode returns false and production returns true.
func TestIsProductionMode(t *testing.T) {
	cfg := DefaultAppConfig()

	cfg.Server.Mode = "development"
	if cfg.IsProductionMode() {
		t.Error("Expected IsProductionMode false in development mode, got true")
	}

	cfg.Server.Mode = "dev"
	if cfg.IsProductionMode() {
		t.Error("Expected IsProductionMode false for mode 'dev', got true")
	}

	cfg.Server.Mode = "production"
	if !cfg.IsProductionMode() {
		t.Error("Expected IsProductionMode true in production mode, got false")
	}
}

// TestIsValidSSLHost verifies that SSL host validation always uses production rules.
func TestIsValidSSLHost(t *testing.T) {
	if !IsValidSSLHost("example.com") {
		t.Error("IsValidSSLHost(\"example.com\") = false, want true")
	}
	if IsValidSSLHost("localhost") {
		t.Error("IsValidSSLHost(\"localhost\") = true, want false")
	}
	if IsValidSSLHost("192.168.1.1") {
		t.Error("IsValidSSLHost(\"192.168.1.1\") = true, want false")
	}
	if IsValidSSLHost("test.local") {
		t.Error("IsValidSSLHost(\"test.local\") = true, want false")
	}
}

// TestAdminURLPrefix verifies the /server/{admin_path} prefix is returned.
func TestAdminURLPrefix(t *testing.T) {
	cfg := DefaultAppConfig()

	cfg.Server.Admin.Path = "admin"
	if got := cfg.AdminURLPrefix(); got != "/server/admin" {
		t.Errorf("AdminURLPrefix with 'admin': got %q, want %q", got, "/server/admin")
	}

	// Empty path falls back to "admin"
	cfg.Server.Admin.Path = ""
	if got := cfg.AdminURLPrefix(); got != "/server/admin" {
		t.Errorf("AdminURLPrefix with empty path: got %q, want %q", got, "/server/admin")
	}

	cfg.Server.Admin.Path = "myadmin"
	if got := cfg.AdminURLPrefix(); got != "/server/myadmin" {
		t.Errorf("AdminURLPrefix with 'myadmin': got %q, want %q", got, "/server/myadmin")
	}
}

// TestAdminAPIPrefix verifies the canonical admin API prefix matches AdminURLPrefix.
func TestAdminAPIPrefix(t *testing.T) {
	cfg := DefaultAppConfig()

	cfg.Server.Admin.Path = "admin"
	if got := cfg.AdminAPIPrefix(); got != "/server/admin" {
		t.Errorf("AdminAPIPrefix with 'admin': got %q, want %q", got, "/server/admin")
	}

	cfg.Server.Admin.Path = ""
	if got := cfg.AdminAPIPrefix(); got != "/server/admin" {
		t.Errorf("AdminAPIPrefix with empty path: got %q, want %q", got, "/server/admin")
	}

	cfg.Server.Admin.Path = "myadmin"
	if got := cfg.AdminAPIPrefix(); got != "/server/myadmin" {
		t.Errorf("AdminAPIPrefix with 'myadmin': got %q, want %q", got, "/server/myadmin")
	}
}

// TestGetPublicURL covers FQDN set, address empty, and address 0.0.0.0.
func TestGetPublicURL(t *testing.T) {
	cfg := DefaultAppConfig()
	cfg.Server.Port = "8080"

	// FQDN set — uses https
	cfg.Server.FQDN = "example.com"
	if got := cfg.GetPublicURL(); got != "https://example.com" {
		t.Errorf("GetPublicURL with FQDN: got %q, want %q", got, "https://example.com")
	}

	// FQDN empty, Address empty — falls back to localhost
	cfg.Server.FQDN = ""
	cfg.Server.Address = ""
	got := cfg.GetPublicURL()
	if got != "http://localhost:8080" {
		t.Errorf("GetPublicURL with empty address: got %q, want %q", got, "http://localhost:8080")
	}

	// Address 0.0.0.0 — also falls back to localhost
	cfg.Server.Address = "0.0.0.0"
	got = cfg.GetPublicURL()
	if got != "http://localhost:8080" {
		t.Errorf("GetPublicURL with 0.0.0.0: got %q, want %q", got, "http://localhost:8080")
	}
}

// TestGetFQDNWithEnv verifies that the DOMAIN env var is respected.
func TestGetFQDNWithEnv(t *testing.T) {
	t.Setenv("DOMAIN", "env.example.com")
	if got := GetFQDN(); got != "env.example.com" {
		t.Errorf("GetFQDN with DOMAIN env: got %q, want %q", got, "env.example.com")
	}
}

// TestGetFQDNWithoutEnv verifies that GetFQDN returns a non-empty string when DOMAIN is unset.
func TestGetFQDNWithoutEnv(t *testing.T) {
	os.Unsetenv("DOMAIN")
	got := GetFQDN()
	if got == "" {
		t.Error("GetFQDN without DOMAIN env returned empty string")
	}
}

// TestNewWatcherNonexistentPath verifies that NewWatcher succeeds and sets lastMod=0 for a missing file.
func TestNewWatcherNonexistentPath(t *testing.T) {
	cfg := DefaultAppConfig()
	w := NewWatcher("/nonexistent/path/that/does/not/exist.yml", cfg)
	if w == nil {
		t.Fatal("NewWatcher returned nil for nonexistent path")
	}
	if w.lastMod != 0 {
		t.Errorf("NewWatcher nonexistent path: lastMod = %d, want 0", w.lastMod)
	}
}

// TestNewWatcherExistingFile verifies that NewWatcher captures the correct lastMod for an existing file.
func TestNewWatcherExistingFile(t *testing.T) {
	tmp, err := os.CreateTemp(t.TempDir(), "watcher-*.yml")
	if err != nil {
		t.Fatalf("failed to create temp file: %v", err)
	}
	tmp.Close()

	info, err := os.Stat(tmp.Name())
	if err != nil {
		t.Fatalf("failed to stat temp file: %v", err)
	}
	wantMod := info.ModTime().UnixNano()

	cfg := DefaultAppConfig()
	w := NewWatcher(tmp.Name(), cfg)
	if w == nil {
		t.Fatal("NewWatcher returned nil for existing file")
	}
	if w.lastMod != wantMod {
		t.Errorf("NewWatcher existing file: lastMod = %d, want %d", w.lastMod, wantMod)
	}
}

// TestOnReloadRegistersCallback verifies that each OnReload call appends a callback.
func TestOnReloadRegistersCallback(t *testing.T) {
	cfg := DefaultAppConfig()
	w := NewWatcher("/nonexistent", cfg)

	if len(w.callbacks) != 0 {
		t.Errorf("New watcher should have 0 callbacks, got %d", len(w.callbacks))
	}

	w.OnReload(func(_ *AppConfig) {})
	if len(w.callbacks) != 1 {
		t.Errorf("After first OnReload: expected 1 callback, got %d", len(w.callbacks))
	}

	w.OnReload(func(_ *AppConfig) {})
	if len(w.callbacks) != 2 {
		t.Errorf("After second OnReload: expected 2 callbacks, got %d", len(w.callbacks))
	}
}

// TestWatcherStartStop verifies Start/Stop do not panic and the goroutine exits cleanly.
func TestWatcherStartStop(t *testing.T) {
	cfg := DefaultAppConfig()
	w := NewWatcher("/nonexistent", cfg)
	w.Start()
	w.Stop()
}

// TestWatcherReload verifies that Reload on a valid saved config file returns nil.
func TestWatcherReload(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "server.yml")

	cfg := DefaultAppConfig()
	if err := SaveAppConfig(cfg, configPath); err != nil {
		t.Fatalf("SaveAppConfig failed: %v", err)
	}

	w := NewWatcher(configPath, cfg)
	if err := w.Reload(); err != nil {
		t.Errorf("Reload() returned error: %v", err)
	}
}

// TestUserAgentString verifies that String() on a UserAgentConfig produces a non-empty value.
func TestUserAgentString(t *testing.T) {
	cases := []UserAgentConfig{
		{OS: "windows", Browser: "chrome", BrowserVersion: "120"},
		{OS: "macos", Browser: "edge", BrowserVersion: "120"},
		{OS: "linux", Browser: "firefox", BrowserVersion: "120"},
		// Default OS/browser (empty fields)
		{},
	}

	for _, ua := range cases {
		got := ua.String()
		if got == "" {
			t.Errorf("UserAgentConfig%+v.String() returned empty string", ua)
		}
	}
}

// TestSecChUa verifies that SecChUa returns empty for Firefox and non-empty for Chrome/Edge.
func TestSecChUa(t *testing.T) {
	firefoxUA := UserAgentConfig{Browser: "firefox", BrowserVersion: "120"}
	if got := firefoxUA.SecChUa(); got != "" {
		t.Errorf("SecChUa for firefox: got %q, want empty string", got)
	}

	chromeUA := UserAgentConfig{Browser: "chrome", BrowserVersion: "120"}
	if got := chromeUA.SecChUa(); got == "" {
		t.Error("SecChUa for chrome returned empty string, want non-empty")
	}

	edgeUA := UserAgentConfig{Browser: "edge", BrowserVersion: "120"}
	if got := edgeUA.SecChUa(); got == "" {
		t.Error("SecChUa for edge returned empty string, want non-empty")
	}
}

// TestSecChUaPlatform verifies each OS maps to the correct platform string.
func TestSecChUaPlatform(t *testing.T) {
	cases := []struct {
		os   string
		want string
	}{
		{"windows", `"Windows"`},
		{"macos", `"macOS"`},
		{"linux", `"Linux"`},
		// Unknown OS defaults to Windows
		{"", `"Windows"`},
	}

	for _, tc := range cases {
		ua := UserAgentConfig{OS: tc.os}
		if got := ua.SecChUaPlatform(); got != tc.want {
			t.Errorf("SecChUaPlatform(%q) = %q, want %q", tc.os, got, tc.want)
		}
	}
}

// TestValidateSEOVerification_ValidCodes verifies that correct verification codes pass validation.
func TestValidateSEOVerification_ValidCodes(t *testing.T) {
	v := SEOVerificationConfig{
		Google:    "abc123_-XYZ",
		Bing:      "ABCDEF0123456789",
		Yandex:    "abcdef01234567890",
		Baidu:     "abc123XYZ",
		Pinterest: "abcdef0123456789",
		Facebook:  "abc123",
	}
	bad := validateSEOVerification(v)
	if len(bad) != 0 {
		t.Errorf("validateSEOVerification() with valid codes returned errors: %v", bad)
	}
}

// TestValidateSEOVerification_InvalidCodes verifies that malformed codes are flagged.
func TestValidateSEOVerification_InvalidCodes(t *testing.T) {
	v := SEOVerificationConfig{
		// Bing must be uppercase hex; lowercase is invalid
		Bing: "abcdef",
		// Yandex must be lowercase hex; uppercase is invalid
		Yandex: "ABCDEF",
	}
	bad := validateSEOVerification(v)
	if len(bad) != 2 {
		t.Errorf("validateSEOVerification() expected 2 bad fields, got %d: %v", len(bad), bad)
	}
}

// TestValidateSEOVerification_EmptyCodes verifies that empty codes are skipped (no error).
func TestValidateSEOVerification_EmptyCodes(t *testing.T) {
	v := SEOVerificationConfig{}
	bad := validateSEOVerification(v)
	if len(bad) != 0 {
		t.Errorf("validateSEOVerification() with empty config returned errors: %v", bad)
	}
}

// TestValidateSEOVerification_CustomTag verifies custom tag validation.
func TestValidateSEOVerification_CustomTag(t *testing.T) {
	v := SEOVerificationConfig{
		Custom: []SEOCustomTag{
			{Name: "my-tag", Content: "valid-content"},
		},
	}
	bad := validateSEOVerification(v)
	if len(bad) != 0 {
		t.Errorf("validateSEOVerification() with valid custom tag returned errors: %v", bad)
	}

	v2 := SEOVerificationConfig{
		Custom: []SEOCustomTag{
			// Missing both name and property is invalid
			{Content: "some-content"},
		},
	}
	bad2 := validateSEOVerification(v2)
	if len(bad2) == 0 {
		t.Error("validateSEOVerification() with empty name/property should return errors")
	}
}

// TestSEOVerifyPattern tests the pattern-matching helper.
func TestSEOVerifyPattern(t *testing.T) {
	tests := []struct {
		pattern string
		value   string
		want    bool
	}{
		{`^[a-z0-9]+$`, "abc123", true},
		{`^[a-z0-9]+$`, "ABC123", false},
		{`^[a-zA-Z0-9_-]{1,43}$`, "Valid_Code-123", true},
		{`^[A-F0-9]{1,32}$`, "DEADBEEF", true},
		{`^[A-F0-9]{1,32}$`, "deadbeef", false},
	}
	for _, tc := range tests {
		got := seoVerifyPattern(tc.pattern, tc.value)
		if got != tc.want {
			t.Errorf("seoVerifyPattern(%q, %q) = %v, want %v", tc.pattern, tc.value, got, tc.want)
		}
	}
}

// TestIsChromiumBased verifies that only Firefox is not Chromium-based.
func TestIsChromiumBased(t *testing.T) {
	firefox := UserAgentConfig{Browser: "firefox"}
	if firefox.IsChromiumBased() {
		t.Error("IsChromiumBased() for firefox = true, want false")
	}

	chrome := UserAgentConfig{Browser: "chrome"}
	if !chrome.IsChromiumBased() {
		t.Error("IsChromiumBased() for chrome = false, want true")
	}

	edge := UserAgentConfig{Browser: "edge"}
	if !edge.IsChromiumBased() {
		t.Error("IsChromiumBased() for edge = false, want true")
	}

	// Empty browser defaults to chrome path (not firefox)
	empty := UserAgentConfig{}
	if !empty.IsChromiumBased() {
		t.Error("IsChromiumBased() for empty browser = false, want true")
	}
}
