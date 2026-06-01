// SPDX-License-Identifier: MIT
package config

import (
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
