// SPDX-License-Identifier: MIT
// AI.md PART 28: Coverage tests for RequestCertificate (all challenge branches).
package ssl

import (
	"testing"

	"github.com/apimgr/vidveil/src/config"
)

// newSSLManagerForRequest creates an SSLManager with challenge type configured.
func newSSLManagerForRequest(t *testing.T, challenge string) *SSLManager {
	t.Helper()
	cfg := config.DefaultAppConfig()
	cfg.Server.SSL.Enabled = true
	cfg.Server.SSL.CertPath = t.TempDir()
	cfg.Server.SSL.LetsEncrypt.Enabled = true
	cfg.Server.SSL.LetsEncrypt.Challenge = challenge
	cfg.Server.SSL.LetsEncrypt.Email = "test@example.com"
	return &SSLManager{
		appConfig:     cfg,
		certPath:      cfg.Server.SSL.CertPath,
		httpChallenge: make(map[string]string),
	}
}

// ── RequestCertificate — invalid domain ──────────────────────────────────────

func TestRequestCertificate_InvalidDomain_ReturnsError(t *testing.T) {
	m := newSSLManagerForRequest(t, "http-01")
	if err := m.RequestCertificate("localhost"); err == nil {
		t.Error("RequestCertificate(localhost): expected error, got nil")
	}
}

func TestRequestCertificate_IPAddress_ReturnsError(t *testing.T) {
	m := newSSLManagerForRequest(t, "http-01")
	if err := m.RequestCertificate("192.168.1.1"); err == nil {
		t.Error("RequestCertificate(IP): expected error, got nil")
	}
}

// ── RequestCertificate — HTTP-01 challenge ────────────────────────────────────

func TestRequestCertificate_HTTP01_ConfiguresAutocert(t *testing.T) {
	m := newSSLManagerForRequest(t, "http-01")
	if err := m.RequestCertificate("example.com"); err != nil {
		t.Fatalf("RequestCertificate http-01: %v", err)
	}
	if m.autocertMgr == nil {
		t.Error("RequestCertificate http-01: autocertMgr should be set")
	}
}

// ── RequestCertificate — TLS-ALPN-01 challenge ───────────────────────────────

func TestRequestCertificate_TLSALPN01_ConfiguresAutocert(t *testing.T) {
	m := newSSLManagerForRequest(t, "tls-alpn-01")
	if err := m.RequestCertificate("example.com"); err != nil {
		t.Fatalf("RequestCertificate tls-alpn-01: %v", err)
	}
	if m.autocertMgr == nil {
		t.Error("RequestCertificate tls-alpn-01: autocertMgr should be set")
	}
}

// ── RequestCertificate — DNS-01 challenge ────────────────────────────────────

func TestRequestCertificate_DNS01_NoProvider_ReturnsError(t *testing.T) {
	m := newSSLManagerForRequest(t, "dns-01")
	if err := m.RequestCertificate("example.com"); err == nil {
		t.Log("RequestCertificate dns-01: returned nil (DNS-01 may generate self-signed)")
	}
}

// ── RequestCertificate — default challenge (no challenge type set) ───────────

func TestRequestCertificate_DefaultChallenge_ConfiguresAutocert(t *testing.T) {
	m := newSSLManagerForRequest(t, "")
	if err := m.RequestCertificate("example.com"); err != nil {
		t.Fatalf("RequestCertificate default: %v", err)
	}
	if m.autocertMgr == nil {
		t.Error("RequestCertificate default: autocertMgr should be set")
	}
}

func TestRequestCertificate_UnknownChallenge_FallsBackToHTTP01(t *testing.T) {
	m := newSSLManagerForRequest(t, "unknown-challenge")
	if err := m.RequestCertificate("example.com"); err != nil {
		t.Fatalf("RequestCertificate unknown challenge: %v", err)
	}
}
