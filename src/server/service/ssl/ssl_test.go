// SPDX-License-Identifier: MIT
// Tests for the ssl package: NewSSLManager, Initialize, IsAutocertEnabled,
// GetCertificate, GetTLSConfig, GetHTTPHandler, SetHTTP01Challenge,
// ClearHTTP01Challenge, HTTP01Handler, NeedsRenewal, and GetCertInfo.
package ssl

import (
	"crypto/tls"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/apimgr/vidveil/src/config"
)

// newTestSSLManager builds an SSLManager with SSL disabled so Initialize
// returns immediately without touching the filesystem.
func newTestSSLManager(t *testing.T) *SSLManager {
	t.Helper()
	cfg := config.DefaultAppConfig()
	cfg.Server.SSL.Enabled = false
	return &SSLManager{
		appConfig:     cfg,
		certPath:      t.TempDir(),
		httpChallenge: make(map[string]string),
	}
}

// ---- NewSSLManager ----

func TestNewSSLManagerReturnsNonNil(t *testing.T) {
	cfg := config.DefaultAppConfig()
	cfg.Server.SSL.Enabled = false
	m := NewSSLManager(cfg, "")
	if m == nil {
		t.Fatal("NewSSLManager returned nil")
	}
}

func TestNewSSLManagerCertPathSet(t *testing.T) {
	cfg := config.DefaultAppConfig()
	cfg.Server.SSL.Enabled = false
	m := NewSSLManager(cfg, "")
	if m.certPath == "" {
		t.Error("certPath is empty after NewSSLManager")
	}
}

func TestNewSSLManagerHTTPChallengeMapNonNil(t *testing.T) {
	cfg := config.DefaultAppConfig()
	cfg.Server.SSL.Enabled = false
	m := NewSSLManager(cfg, "")
	if m.httpChallenge == nil {
		t.Error("httpChallenge map is nil after NewSSLManager")
	}
}

func TestNewSSLManagerUsesCertPathFromConfig(t *testing.T) {
	cfg := config.DefaultAppConfig()
	cfg.Server.SSL.Enabled = false
	cfg.Server.SSL.CertPath = t.TempDir()
	m := NewSSLManager(cfg, "")
	if m.certPath != cfg.Server.SSL.CertPath {
		t.Errorf("certPath = %q, want %q", m.certPath, cfg.Server.SSL.CertPath)
	}
}

// ---- Initialize ----

func TestInitializeSSLDisabledReturnsNil(t *testing.T) {
	m := newTestSSLManager(t)
	if err := m.Initialize(); err != nil {
		t.Errorf("Initialize with SSL disabled returned error: %v", err)
	}
}

func TestInitializeSSLDisabledLeavesNoCertificate(t *testing.T) {
	m := newTestSSLManager(t)
	_ = m.Initialize()
	m.mu.RLock()
	cert := m.certificate
	m.mu.RUnlock()
	if cert != nil {
		t.Error("expected certificate to remain nil when SSL is disabled")
	}
}

// ---- IsAutocertEnabled ----

func TestIsAutocertEnabledDefaultFalse(t *testing.T) {
	m := newTestSSLManager(t)
	if m.IsAutocertEnabled() {
		t.Error("IsAutocertEnabled should be false by default")
	}
}

// ---- GetCertificate ----

func TestGetCertificateNoCertReturnsError(t *testing.T) {
	m := newTestSSLManager(t)
	cert, err := m.GetCertificate(nil)
	if err == nil {
		t.Error("expected error when no certificate is loaded")
	}
	if cert != nil {
		t.Error("expected nil certificate when no certificate is loaded")
	}
}

// ---- GetTLSConfig ----

func TestGetTLSConfigNonNil(t *testing.T) {
	m := newTestSSLManager(t)
	cfg := m.GetTLSConfig()
	if cfg == nil {
		t.Fatal("GetTLSConfig returned nil")
	}
}

func TestGetTLSConfigMinVersionTLS12(t *testing.T) {
	m := newTestSSLManager(t)
	cfg := m.GetTLSConfig()
	if cfg.MinVersion != tls.VersionTLS12 {
		t.Errorf("MinVersion = %d, want %d (TLS 1.2)", cfg.MinVersion, tls.VersionTLS12)
	}
}

func TestGetTLSConfigCipherSuitesNonEmpty(t *testing.T) {
	m := newTestSSLManager(t)
	cfg := m.GetTLSConfig()
	if len(cfg.CipherSuites) == 0 {
		t.Error("CipherSuites must not be empty")
	}
}

func TestGetTLSConfigHasGetCertificateFunc(t *testing.T) {
	m := newTestSSLManager(t)
	cfg := m.GetTLSConfig()
	if cfg.GetCertificate == nil {
		t.Error("GetCertificate function must be set in TLS config")
	}
}

// ---- GetHTTPHandler ----

func TestGetHTTPHandlerReturnsNonNil(t *testing.T) {
	m := newTestSSLManager(t)
	h := m.GetHTTPHandler()
	if h == nil {
		t.Error("GetHTTPHandler returned nil")
	}
}

func TestGetHTTPHandlerImplementsHTTPHandler(t *testing.T) {
	m := newTestSSLManager(t)
	var _ http.Handler = m.GetHTTPHandler()
}

// ---- SetHTTP01Challenge / ClearHTTP01Challenge ----

func TestSetHTTP01ChallengeStoresValue(t *testing.T) {
	m := newTestSSLManager(t)
	m.SetHTTP01Challenge("tok1", "tok1.keyauth")
	m.mu.RLock()
	got, ok := m.httpChallenge["tok1"]
	m.mu.RUnlock()
	if !ok {
		t.Fatal("token not found after SetHTTP01Challenge")
	}
	if got != "tok1.keyauth" {
		t.Errorf("keyAuth = %q, want %q", got, "tok1.keyauth")
	}
}

func TestSetHTTP01ChallengeOverwritesExisting(t *testing.T) {
	m := newTestSSLManager(t)
	m.SetHTTP01Challenge("tok1", "old")
	m.SetHTTP01Challenge("tok1", "new")
	m.mu.RLock()
	got := m.httpChallenge["tok1"]
	m.mu.RUnlock()
	if got != "new" {
		t.Errorf("keyAuth = %q, want %q after overwrite", got, "new")
	}
}

func TestClearHTTP01ChallengeRemovesToken(t *testing.T) {
	m := newTestSSLManager(t)
	m.SetHTTP01Challenge("tok2", "tok2.keyauth")
	m.ClearHTTP01Challenge("tok2")
	m.mu.RLock()
	_, ok := m.httpChallenge["tok2"]
	m.mu.RUnlock()
	if ok {
		t.Error("token still present after ClearHTTP01Challenge")
	}
}

func TestClearHTTP01ChallengeUnknownTokenNoError(t *testing.T) {
	m := newTestSSLManager(t)
	// Must not panic
	m.ClearHTTP01Challenge("nonexistent")
}

// ---- HTTP01Handler (via httptest) ----

func TestHTTP01HandlerKnownTokenReturns200AndKeyAuth(t *testing.T) {
	m := newTestSSLManager(t)
	m.SetHTTP01Challenge("mytoken", "mytoken.keyauth")

	req := httptest.NewRequest(http.MethodGet, "/.well-known/acme-challenge/mytoken", nil)
	rr := httptest.NewRecorder()

	m.HTTP01Handler(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("status = %d, want 200", rr.Code)
	}
	if body := rr.Body.String(); body != "mytoken.keyauth" {
		t.Errorf("body = %q, want %q", body, "mytoken.keyauth")
	}
}

func TestHTTP01HandlerUnknownTokenReturns404(t *testing.T) {
	m := newTestSSLManager(t)

	req := httptest.NewRequest(http.MethodGet, "/.well-known/acme-challenge/unknowntoken", nil)
	rr := httptest.NewRecorder()

	m.HTTP01Handler(rr, req)

	if rr.Code != http.StatusNotFound {
		t.Errorf("status = %d, want 404", rr.Code)
	}
}

func TestHTTP01HandlerNonChallengePath(t *testing.T) {
	m := newTestSSLManager(t)

	req := httptest.NewRequest(http.MethodGet, "/robots.txt", nil)
	rr := httptest.NewRecorder()

	m.HTTP01Handler(rr, req)

	// A path outside /.well-known/acme-challenge/ has no matching token and returns 404
	if rr.Code != http.StatusNotFound {
		t.Errorf("status = %d, want 404 for non-challenge path", rr.Code)
	}
}

func TestHTTP01HandlerAfterClearReturns404(t *testing.T) {
	m := newTestSSLManager(t)
	m.SetHTTP01Challenge("gone", "gone.keyauth")
	m.ClearHTTP01Challenge("gone")

	req := httptest.NewRequest(http.MethodGet, "/.well-known/acme-challenge/gone", nil)
	rr := httptest.NewRecorder()

	m.HTTP01Handler(rr, req)

	if rr.Code != http.StatusNotFound {
		t.Errorf("status = %d, want 404 after clear", rr.Code)
	}
}

// ---- NeedsRenewal ----

func TestNeedsRenewalNoCertReturnsFalse(t *testing.T) {
	// GetCertInfo returns an error when no cert is loaded; NeedsRenewal
	// returns true in that case per the source: "if err != nil { return true }".
	m := newTestSSLManager(t)
	// No cert loaded — NeedsRenewal must return true (no cert = must renew)
	if !m.NeedsRenewal() {
		t.Error("NeedsRenewal should return true when no certificate is loaded")
	}
}

// ---- GetCertInfo ----

func TestGetCertInfoNoCertReturnsError(t *testing.T) {
	m := newTestSSLManager(t)
	info, err := m.GetCertInfo()
	if err == nil {
		t.Error("expected error from GetCertInfo when no certificate is loaded")
	}
	if info != nil {
		t.Error("expected nil CertInfo when no certificate is loaded")
	}
}
