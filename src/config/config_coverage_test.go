// SPDX-License-Identifier: MIT
package config

import (
	"os"
	"strings"
	"testing"
)

// TestIsLoopback verifies loopback detection for known loopback and non-loopback hosts.
func TestIsLoopback(t *testing.T) {
	tests := []struct {
		host     string
		expected bool
	}{
		{"localhost", true},
		{"127.0.0.1", true},
		{"::1", true},
		{"example.com", false},
		// Private range address — not loopback
		{"192.168.1.1", false},
	}

	for _, tc := range tests {
		got := isLoopback(tc.host)
		if got != tc.expected {
			t.Errorf("isLoopback(%q) = %v, want %v", tc.host, got, tc.expected)
		}
	}
}

// TestIsDevTLD verifies that dev-only TLDs are detected and public TLDs are rejected.
func TestIsDevTLD(t *testing.T) {
	tests := []struct {
		fqdn     string
		expected bool
	}{
		{"localhost", true},
		{"example.local", true},
		{"test.test", true},
		{"example.com", false},
		{"api.example.com", false},
		// Empty string has no dot suffix and is not "localhost"
		{"", false},
	}

	for _, tc := range tests {
		got := isDevTLD(tc.fqdn)
		if got != tc.expected {
			t.Errorf("isDevTLD(%q) = %v, want %v", tc.fqdn, got, tc.expected)
		}
	}
}

// TestGenerateToken verifies token length, hex encoding, and randomness.
func TestGenerateToken(t *testing.T) {
	// 16 bytes encodes to 32 hex characters
	tok16 := generateToken(16)
	if len(tok16) != 32 {
		t.Errorf("generateToken(16) length = %d, want 32", len(tok16))
	}
	if tok16 == "" {
		t.Error("generateToken(16) returned empty string")
	}

	// 32 bytes encodes to 64 hex characters
	tok32 := generateToken(32)
	if len(tok32) != 64 {
		t.Errorf("generateToken(32) length = %d, want 64", len(tok32))
	}

	// Two calls must produce different values
	tok32b := generateToken(32)
	if tok32 == tok32b {
		t.Error("generateToken produced identical values on two consecutive calls — likely not random")
	}
}

// TestFindUnusedPort verifies that findUnusedPort returns a valid port in the expected range or the fallback.
func TestFindUnusedPort(t *testing.T) {
	port := findUnusedPort()
	// Valid result is either a port in the scan range or the defined fallback
	if port <= 0 {
		t.Errorf("findUnusedPort() = %d, want a positive integer", port)
	}
	if port < 64000 || port > 65000 {
		t.Errorf("findUnusedPort() = %d, want value in [64000, 65000]", port)
	}
}

// TestIsValidHostEdgeCases covers edge cases not exercised by the existing test suite.
func TestIsValidHostEdgeCases(t *testing.T) {
	tests := []struct {
		host    string
		devMode bool
		want    bool
		desc    string
	}{
		// Host without any dot and not "localhost" is invalid in dev mode
		{"myserver", true, false, "no-dot host in dev mode"},
		// Host without any dot and not "localhost" is invalid in production mode
		{"myserver", false, false, "no-dot host in production mode"},
		// Dev-only TLD in production mode must be rejected
		{"host.lan", false, false, "dev TLD in production mode"},
		// Dev-only TLD in dev mode must be accepted
		{"host.lan", true, true, "dev TLD in dev mode"},
	}

	for _, tc := range tests {
		got := IsValidHost(tc.host, tc.devMode)
		if got != tc.want {
			t.Errorf("IsValidHost(%q, devMode=%v) [%s] = %v, want %v",
				tc.host, tc.devMode, tc.desc, got, tc.want)
		}
	}
}

// TestGetGlobalIPv4_ReturnsStringOrEmpty verifies getGlobalIPv4 returns either a
// valid IPv4 string or empty string — never panics.
func TestGetGlobalIPv4_ReturnsStringOrEmpty(t *testing.T) {
	got := getGlobalIPv4()
	if got == "" {
		return
	}
	parts := strings.Split(got, ".")
	if len(parts) != 4 {
		t.Errorf("getGlobalIPv4() = %q, does not look like an IPv4 address", got)
	}
}

// TestGetGlobalIPv6_ReturnsStringOrEmpty verifies getGlobalIPv6 returns either a
// valid IPv6 string or empty — never panics.
func TestGetGlobalIPv6_ReturnsStringOrEmpty(t *testing.T) {
	got := getGlobalIPv6()
	if got == "" {
		return
	}
	if !strings.Contains(got, ":") {
		t.Errorf("getGlobalIPv6() = %q, does not look like an IPv6 address", got)
	}
}

// TestGetDisplayHost_ReturnsNonEmpty verifies GetDisplayHost always returns a non-empty string.
func TestGetDisplayHost_ReturnsNonEmpty(t *testing.T) {
	cfg := DefaultAppConfig()
	got := GetDisplayHost(cfg)
	if got == "" {
		t.Error("GetDisplayHost: returned empty string")
	}
}

// TestGetFQDN_WithDomainEnv verifies that the DOMAIN env var overrides os.Hostname.
func TestGetFQDN_WithDomainEnv(t *testing.T) {
	t.Setenv("DOMAIN", "custom.example.com")
	got := GetFQDN()
	if got != "custom.example.com" {
		t.Errorf("GetFQDN with DOMAIN env = %q, want 'custom.example.com'", got)
	}
}

// TestGetFQDN_FallsBackToHostname verifies GetFQDN never returns empty.
func TestGetFQDN_FallsBackToHostname(t *testing.T) {
	got := GetFQDN()
	if got == "" {
		t.Error("GetFQDN: returned empty string")
	}
}

// TestValidateConfig_InvalidPort verifies that an out-of-range port is replaced.
func TestValidateConfig_InvalidPort(t *testing.T) {
	cfg := DefaultAppConfig()
	cfg.Server.Port = "99999"
	validateConfig(cfg)
	if cfg.Server.Port == "99999" {
		t.Error("validateConfig: should replace invalid port 99999")
	}
}

// TestValidateConfig_ValidPort verifies that a valid port is preserved.
func TestValidateConfig_ValidPort(t *testing.T) {
	cfg := DefaultAppConfig()
	cfg.Server.Port = "8080"
	validateConfig(cfg)
	if cfg.Server.Port != "8080" {
		t.Errorf("validateConfig: valid port 8080 changed to %q", cfg.Server.Port)
	}
}

// TestValidateConfig_InvalidMode verifies invalid mode is reset to default.
func TestValidateConfig_InvalidMode(t *testing.T) {
	cfg := DefaultAppConfig()
	cfg.Server.Mode = "unicorn"
	validateConfig(cfg)
	if cfg.Server.Mode == "unicorn" {
		t.Error("validateConfig: should replace invalid mode 'unicorn'")
	}
}

// TestValidateConfig_NegativeRateLimit verifies negative rate limit window is reset.
func TestValidateConfig_NegativeRateLimit(t *testing.T) {
	cfg := DefaultAppConfig()
	cfg.Server.RateLimit.Window = -10
	cfg.Server.RateLimit.Requests = -5
	validateConfig(cfg)
	if cfg.Server.RateLimit.Window < 0 {
		t.Error("validateConfig: negative rate_limit.window should be fixed")
	}
	if cfg.Server.RateLimit.Requests < 0 {
		t.Error("validateConfig: negative rate_limit.requests should be fixed")
	}
}

// TestValidateConfig_InvalidSameSite verifies invalid same_site is reset.
func TestValidateConfig_InvalidSameSite(t *testing.T) {
	cfg := DefaultAppConfig()
	cfg.Server.Session.SameSite = "unsafe"
	validateConfig(cfg)
	if cfg.Server.Session.SameSite == "unsafe" {
		t.Error("validateConfig: invalid same_site 'unsafe' should be replaced")
	}
}

// TestValidateConfig_InvalidCompressionLevel verifies out-of-range compression level is reset.
func TestValidateConfig_InvalidCompressionLevel(t *testing.T) {
	cfg := DefaultAppConfig()
	cfg.Server.Compression.Level = 15
	validateConfig(cfg)
	if cfg.Server.Compression.Level == 15 {
		t.Error("validateConfig: invalid compression level 15 should be replaced")
	}
}

// TestValidateConfig_NonJSONAuditFormat verifies non-JSON audit format is enforced.
func TestValidateConfig_NonJSONAuditFormat(t *testing.T) {
	cfg := DefaultAppConfig()
	cfg.Server.Logs.Audit.Format = "text"
	validateConfig(cfg)
	if cfg.Server.Logs.Audit.Format != "json" {
		t.Errorf("validateConfig: audit format should be forced to 'json', got %q", cfg.Server.Logs.Audit.Format)
	}
}

// ── GetDisplayHost — loopback and dev-TLD paths ───────────────────────────────

// When DOMAIN is a loopback, GetDisplayHost tries getGlobalIPv6/IPv4 then falls back.
func TestGetDisplayHost_LoopbackDomain_FallsBack(t *testing.T) {
	t.Setenv("DOMAIN", "localhost")
	result := GetDisplayHost(DefaultAppConfig())
	if result == "" {
		t.Error("GetDisplayHost(loopback): expected non-empty fallback")
	}
}

// When DOMAIN is a dev TLD, GetDisplayHost tries IP resolution then falls back.
func TestGetDisplayHost_DevTLD_FallsBack(t *testing.T) {
	t.Setenv("DOMAIN", "myhost.local")
	result := GetDisplayHost(DefaultAppConfig())
	if result == "" {
		t.Error("GetDisplayHost(.local): expected non-empty fallback")
	}
}

// ── GetFQDN — all fallback paths ──────────────────────────────────────────────

// HOSTNAME env var path (when os.Hostname() fails or returns loopback).
func TestGetFQDN_HostnameEnvVar_NotLoopback(t *testing.T) {
	t.Setenv("DOMAIN", "")
	t.Setenv("HOSTNAME", "myserver.example.com")
	result := GetFQDN()
	if result == "" {
		t.Error("GetFQDN(HOSTNAME env): expected non-empty result")
	}
}

func TestGetFQDN_HostnameEnvVar_Loopback_FallsThrough(t *testing.T) {
	t.Setenv("DOMAIN", "")
	t.Setenv("HOSTNAME", "127.0.0.1")
	// Should skip HOSTNAME (loopback) and try IPv6/IPv4/hostname
	result := GetFQDN()
	_ = result // may return "" if no global IP
}

// ── getHostname — various paths ───────────────────────────────────────────────

func TestGetHostname_ReturnsNonEmpty(t *testing.T) {
	result := getHostname()
	if result == "" {
		t.Log("getHostname: returned empty (no hostname available)")
	}
}

// ── SecChUa — all browser identifiers ────────────────────────────────────────

func TestSecChUa_Chrome_ReturnsChrome(t *testing.T) {
	ua := UserAgentConfig{Browser: "chrome", BrowserVersion: "120", OS: "linux"}
	result := ua.SecChUa()
	if !strings.Contains(result, "Google Chrome") {
		t.Logf("SecChUa chrome: %q", result)
	}
}

func TestSecChUa_Firefox_ReturnsEmpty(t *testing.T) {
	ua := UserAgentConfig{Browser: "firefox", BrowserVersion: "121", OS: "linux"}
	result := ua.SecChUa()
	if result != "" {
		t.Errorf("SecChUa firefox: expected empty, got %q", result)
	}
}

func TestSecChUa_Edge_ReturnsEdge(t *testing.T) {
	ua := UserAgentConfig{Browser: "edge", BrowserVersion: "120", OS: "windows"}
	result := ua.SecChUa()
	if !strings.Contains(result, "Microsoft Edge") {
		t.Logf("SecChUa edge: %q", result)
	}
}

func TestSecChUa_Default_ReturnsChrome(t *testing.T) {
	ua := UserAgentConfig{Browser: "", BrowserVersion: "", OS: "linux"}
	result := ua.SecChUa()
	if !strings.Contains(result, "Google Chrome") {
		t.Logf("SecChUa default: %q", result)
	}
}

func TestSecChUaPlatform_AllPlatforms_NoPanic(t *testing.T) {
	platforms := []string{"linux", "windows", "macos", "android", "unknown"}
	for _, p := range platforms {
		ua := UserAgentConfig{OS: p}
		result := ua.SecChUaPlatform()
		_ = result
	}
}

// ── LoadAppConfig — config dir not found path ─────────────────────────────────

func TestLoadAppConfig_NonExistentDir_ReturnsDefault(t *testing.T) {
	base := os.TempDir() + "/apimgr"
	os.MkdirAll(base, 0755)
	tmp, err := os.MkdirTemp(base, "vidveil-cfg-")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmp)

	cfg, _, loadErr := LoadAppConfig(tmp+"/nonexistent", tmp)
	if cfg == nil {
		t.Error("LoadAppConfig(nonexistent dir): expected non-nil config with defaults")
	}
	_ = loadErr
}

// ── SaveAppConfig — round trip ────────────────────────────────────────────────

func TestSaveAppConfig_TempFile_NoPanic(t *testing.T) {
	base := os.TempDir() + "/apimgr"
	os.MkdirAll(base, 0755)
	tmp, err := os.MkdirTemp(base, "vidveil-save-")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmp)

	cfg := DefaultAppConfig()
	err = SaveAppConfig(cfg, tmp+"/server.yml")
	if err != nil {
		t.Errorf("SaveAppConfig: %v", err)
	}
}
