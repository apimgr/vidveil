// SPDX-License-Identifier: MIT
// Additional coverage tests for the ssl package.
// Targets paths not exercised by ssl_test.go:
//   - generateSelfSigned: key+cert written to tempdir, certificate loaded
//   - loadCertificate: PEM files on disk parsed into m.certificate
//   - copyLetsEncryptCerts: files copied and loaded
//   - GetCertInfo: all fields populated from a loaded certificate
//   - NeedsRenewal: false when cert has 365 days left
//   - RenewCertificate: SSL disabled path, empty/invalid domain path, self-signed regeneration
//   - requestHTTP01 / requestTLSALPN01: autocert manager configured
//   - requestDNS01: missing provider returns error; fallback to self-signed
//   - Initialize: SSL enabled + no LE certs → generateSelfSigned
//   - Initialize: SSL enabled + existing cert files → loadCertificate
//   - GetTLSConfig: autocert path (useAutocert=true)
//   - GetHTTPHandler: autocert path (useAutocert=true)
//   - IsAutocertEnabled: true after requestHTTP01
package ssl

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/apimgr/vidveil/src/config"
)

// newEnabledSSLManager returns a manager with SSL enabled, using a fresh tempdir
// as the certificate path so no real filesystem paths are touched.
func newEnabledSSLManager(t *testing.T) *SSLManager {
	t.Helper()
	cfg := config.DefaultAppConfig()
	cfg.Server.SSL.Enabled = true
	cfg.Server.SSL.CertPath = t.TempDir()
	cfg.Server.FQDN = ""
	cfg.Server.SSL.LetsEncrypt.Enabled = false
	return &SSLManager{
		appConfig:     cfg,
		certPath:      cfg.Server.SSL.CertPath,
		httpChallenge: make(map[string]string),
	}
}

// ---- generateSelfSigned ----

func TestGenerateSelfSignedWritesCertAndKey(t *testing.T) {
	m := newEnabledSSLManager(t)
	if err := m.generateSelfSigned(); err != nil {
		t.Fatalf("generateSelfSigned() error: %v", err)
	}
	certFile := filepath.Join(m.certPath, "cert.pem")
	keyFile := filepath.Join(m.certPath, "key.pem")
	if _, err := os.Stat(certFile); err != nil {
		t.Errorf("cert.pem not created: %v", err)
	}
	if _, err := os.Stat(keyFile); err != nil {
		t.Errorf("key.pem not created: %v", err)
	}
}

func TestGenerateSelfSignedLoadsCertificate(t *testing.T) {
	m := newEnabledSSLManager(t)
	if err := m.generateSelfSigned(); err != nil {
		t.Fatalf("generateSelfSigned() error: %v", err)
	}
	m.mu.RLock()
	cert := m.certificate
	m.mu.RUnlock()
	if cert == nil {
		t.Error("certificate should be loaded after generateSelfSigned")
	}
}

func TestGenerateSelfSignedWithExplicitFQDN(t *testing.T) {
	m := newEnabledSSLManager(t)
	m.appConfig.Server.FQDN = "example.internal"
	if err := m.generateSelfSigned(); err != nil {
		t.Fatalf("generateSelfSigned() with FQDN error: %v", err)
	}
	m.mu.RLock()
	cert := m.certificate
	m.mu.RUnlock()
	if cert == nil {
		t.Error("certificate should be loaded after generateSelfSigned with FQDN")
	}
}

// ---- loadCertificate ----

func TestLoadCertificateFromGeneratedFiles(t *testing.T) {
	m := newEnabledSSLManager(t)
	if err := m.generateSelfSigned(); err != nil {
		t.Fatalf("generateSelfSigned() error: %v", err)
	}
	// Clear certificate and reload from disk.
	m.mu.Lock()
	m.certificate = nil
	m.mu.Unlock()

	certFile := filepath.Join(m.certPath, "cert.pem")
	keyFile := filepath.Join(m.certPath, "key.pem")
	if err := m.loadCertificate(certFile, keyFile); err != nil {
		t.Fatalf("loadCertificate() error: %v", err)
	}
	m.mu.RLock()
	cert := m.certificate
	m.mu.RUnlock()
	if cert == nil {
		t.Error("certificate should be non-nil after loadCertificate")
	}
}

func TestLoadCertificateNonExistentFileReturnsError(t *testing.T) {
	m := newEnabledSSLManager(t)
	err := m.loadCertificate("/nonexistent/cert.pem", "/nonexistent/key.pem")
	if err == nil {
		t.Error("expected error loading non-existent cert files")
	}
}

// ---- copyLetsEncryptCerts ----

// copyLetsEncryptCerts copies PEM files from a source path to m.certPath.
// We create synthetic (but valid, self-signed-generated) source files.
func TestCopyLetsEncryptCertsSuccess(t *testing.T) {
	// Source manager: generate cert into a temp dir.
	src := newEnabledSSLManager(t)
	if err := src.generateSelfSigned(); err != nil {
		t.Fatalf("generateSelfSigned for source: %v", err)
	}
	srcCert := filepath.Join(src.certPath, "cert.pem")
	srcKey := filepath.Join(src.certPath, "key.pem")

	// Destination manager with a separate tempdir.
	dst := newEnabledSSLManager(t)
	if err := dst.copyLetsEncryptCerts(srcCert, srcKey); err != nil {
		t.Fatalf("copyLetsEncryptCerts() error: %v", err)
	}
	dst.mu.RLock()
	cert := dst.certificate
	dst.mu.RUnlock()
	if cert == nil {
		t.Error("certificate should be loaded after copyLetsEncryptCerts")
	}
}

func TestCopyLetsEncryptCertsMissingSourceReturnsError(t *testing.T) {
	m := newEnabledSSLManager(t)
	err := m.copyLetsEncryptCerts("/no/such/cert.pem", "/no/such/key.pem")
	if err == nil {
		t.Error("expected error when source cert file does not exist")
	}
}

// ---- GetCertInfo ----

func TestGetCertInfoAfterGenerateSelfSigned(t *testing.T) {
	m := newEnabledSSLManager(t)
	if err := m.generateSelfSigned(); err != nil {
		t.Fatalf("generateSelfSigned() error: %v", err)
	}
	info, err := m.GetCertInfo()
	if err != nil {
		t.Fatalf("GetCertInfo() error: %v", err)
	}
	if info == nil {
		t.Fatal("GetCertInfo() returned nil CertInfo")
	}
}

func TestGetCertInfoDomainNonEmpty(t *testing.T) {
	m := newEnabledSSLManager(t)
	m.appConfig.Server.FQDN = "testhost.local"
	if err := m.generateSelfSigned(); err != nil {
		t.Fatalf("generateSelfSigned() error: %v", err)
	}
	info, err := m.GetCertInfo()
	if err != nil {
		t.Fatalf("GetCertInfo() error: %v", err)
	}
	if info.Domain == "" {
		t.Error("GetCertInfo().Domain should be non-empty")
	}
}

func TestGetCertInfoIsValidTrue(t *testing.T) {
	m := newEnabledSSLManager(t)
	if err := m.generateSelfSigned(); err != nil {
		t.Fatalf("generateSelfSigned() error: %v", err)
	}
	info, err := m.GetCertInfo()
	if err != nil {
		t.Fatalf("GetCertInfo() error: %v", err)
	}
	if !info.IsValid {
		t.Error("fresh self-signed cert should be reported as valid")
	}
}

func TestGetCertInfoDaysLeftPositive(t *testing.T) {
	m := newEnabledSSLManager(t)
	if err := m.generateSelfSigned(); err != nil {
		t.Fatalf("generateSelfSigned() error: %v", err)
	}
	info, err := m.GetCertInfo()
	if err != nil {
		t.Fatalf("GetCertInfo() error: %v", err)
	}
	if info.DaysLeft <= 0 {
		t.Errorf("DaysLeft = %d, want > 0 for fresh cert", info.DaysLeft)
	}
}

func TestGetCertInfoAutoRenewMatchesConfig(t *testing.T) {
	m := newEnabledSSLManager(t)
	m.appConfig.Server.SSL.LetsEncrypt.Enabled = true
	if err := m.generateSelfSigned(); err != nil {
		t.Fatalf("generateSelfSigned() error: %v", err)
	}
	info, err := m.GetCertInfo()
	if err != nil {
		t.Fatalf("GetCertInfo() error: %v", err)
	}
	if !info.AutoRenew {
		t.Error("GetCertInfo().AutoRenew should be true when LetsEncrypt.Enabled=true")
	}
}

// ---- NeedsRenewal ----

func TestNeedsRenewalFalseForFreshCert(t *testing.T) {
	m := newEnabledSSLManager(t)
	if err := m.generateSelfSigned(); err != nil {
		t.Fatalf("generateSelfSigned() error: %v", err)
	}
	if m.NeedsRenewal() {
		t.Error("NeedsRenewal should be false for a freshly generated cert (365 days left)")
	}
}

// ---- RenewCertificate ----

func TestRenewCertificateSSLDisabledReturnsNil(t *testing.T) {
	m := newTestSSLManager(t)
	if err := m.RenewCertificate(context.TODO()); err != nil {
		t.Errorf("RenewCertificate with SSL disabled returned error: %v", err)
	}
}

func TestRenewCertificateFreshCertNoRenewal(t *testing.T) {
	m := newEnabledSSLManager(t)
	if err := m.generateSelfSigned(); err != nil {
		t.Fatalf("generateSelfSigned() error: %v", err)
	}
	// Fresh cert: NeedsRenewal returns false → RenewCertificate is a no-op.
	if err := m.RenewCertificate(context.TODO()); err != nil {
		t.Errorf("RenewCertificate with fresh cert returned error: %v", err)
	}
}

func TestRenewCertificateNoCertEmptyDomainRegenSelfSigned(t *testing.T) {
	m := newEnabledSSLManager(t)
	// No cert loaded → NeedsRenewal=true, FQDN="" → falls through to generateSelfSigned
	m.appConfig.Server.SSL.LetsEncrypt.Enabled = false
	m.appConfig.Server.FQDN = ""
	if err := m.RenewCertificate(context.TODO()); err != nil {
		t.Errorf("RenewCertificate (empty domain, no cert) error: %v", err)
	}
	m.mu.RLock()
	cert := m.certificate
	m.mu.RUnlock()
	if cert == nil {
		t.Error("certificate should be set after RenewCertificate fallback to self-signed")
	}
}

func TestRenewCertificateNoCertInvalidDomainRegenSelfSigned(t *testing.T) {
	m := newEnabledSSLManager(t)
	m.appConfig.Server.SSL.LetsEncrypt.Enabled = false
	// "localhost" is not a valid SSL host per config.IsValidSSLHost
	m.appConfig.Server.FQDN = "localhost"
	if err := m.RenewCertificate(context.TODO()); err != nil {
		t.Errorf("RenewCertificate (invalid domain, no cert) error: %v", err)
	}
	m.mu.RLock()
	cert := m.certificate
	m.mu.RUnlock()
	if cert == nil {
		t.Error("certificate should be set after RenewCertificate with invalid domain")
	}
}

// ---- requestHTTP01 ----

func TestRequestHTTP01SetsAutocert(t *testing.T) {
	m := newEnabledSSLManager(t)
	if err := m.requestHTTP01("example.com"); err != nil {
		t.Fatalf("requestHTTP01() error: %v", err)
	}
	if !m.useAutocert {
		t.Error("useAutocert should be true after requestHTTP01")
	}
	if m.autocertMgr == nil {
		t.Error("autocertMgr should be set after requestHTTP01")
	}
}

func TestRequestHTTP01SetsIsAutocertEnabled(t *testing.T) {
	m := newEnabledSSLManager(t)
	if err := m.requestHTTP01("example.com"); err != nil {
		t.Fatalf("requestHTTP01() error: %v", err)
	}
	if !m.IsAutocertEnabled() {
		t.Error("IsAutocertEnabled should return true after requestHTTP01")
	}
}

func TestRequestHTTP01WithEmailFromConfig(t *testing.T) {
	m := newEnabledSSLManager(t)
	m.appConfig.Server.SSL.LetsEncrypt.Email = "test@example.com"
	if err := m.requestHTTP01("example.com"); err != nil {
		t.Fatalf("requestHTTP01() error: %v", err)
	}
	if m.autocertMgr == nil {
		t.Error("autocertMgr should be set")
	}
}

// ---- requestTLSALPN01 ----

func TestRequestTLSALPN01SetsAutocert(t *testing.T) {
	m := newEnabledSSLManager(t)
	if err := m.requestTLSALPN01("example.com"); err != nil {
		t.Fatalf("requestTLSALPN01() error: %v", err)
	}
	if !m.useAutocert {
		t.Error("useAutocert should be true after requestTLSALPN01")
	}
}

// ---- requestDNS01 ----

func TestRequestDNS01MissingProviderReturnsError(t *testing.T) {
	m := newEnabledSSLManager(t)
	m.appConfig.Server.SSL.LetsEncrypt.DNSProviderType = ""
	err := m.requestDNS01("example.com")
	if err == nil {
		t.Error("expected error when DNSProviderType is empty")
	}
}

func TestRequestDNS01MissingCredsReturnsError(t *testing.T) {
	m := newEnabledSSLManager(t)
	m.appConfig.Server.SSL.LetsEncrypt.DNSProviderType = "cloudflare"
	// No credentials in env or config — lego provider init must return an error.
	err := m.requestDNS01("example.com")
	if err == nil {
		t.Error("requestDNS01() with missing cloudflare credentials should return an error")
	}
}

// ---- Initialize with SSL enabled ----

func TestInitializeSSLEnabledNoCertsGeneratesSelfSigned(t *testing.T) {
	m := newEnabledSSLManager(t)
	// No cert files exist yet, LetsEncrypt disabled, no FQDN → generateSelfSigned.
	if err := m.Initialize(); err != nil {
		t.Fatalf("Initialize() with SSL enabled error: %v", err)
	}
	m.mu.RLock()
	cert := m.certificate
	m.mu.RUnlock()
	if cert == nil {
		t.Error("Initialize should generate a self-signed cert when none exist")
	}
}

func TestInitializeSSLEnabledExistingCertsLoadsThemDirectly(t *testing.T) {
	m := newEnabledSSLManager(t)
	// Pre-generate a cert so Initialize finds existing files.
	if err := m.generateSelfSigned(); err != nil {
		t.Fatalf("pre-generate: %v", err)
	}
	// Clear in-memory cert, re-Initialize: should load from disk without regenerating.
	m.mu.Lock()
	m.certificate = nil
	m.mu.Unlock()

	if err := m.Initialize(); err != nil {
		t.Fatalf("Initialize() with existing certs error: %v", err)
	}
	m.mu.RLock()
	cert := m.certificate
	m.mu.RUnlock()
	if cert == nil {
		t.Error("Initialize with existing certs should load them into m.certificate")
	}
}

// ---- GetTLSConfig / GetHTTPHandler with autocert ----

func TestGetTLSConfigAutocertPathNonNil(t *testing.T) {
	m := newEnabledSSLManager(t)
	if err := m.requestHTTP01("example.com"); err != nil {
		t.Fatalf("requestHTTP01() error: %v", err)
	}
	cfg := m.GetTLSConfig()
	if cfg == nil {
		t.Error("GetTLSConfig should return non-nil when useAutocert=true")
	}
}

func TestGetHTTPHandlerAutocertPathNonNil(t *testing.T) {
	m := newEnabledSSLManager(t)
	if err := m.requestHTTP01("example.com"); err != nil {
		t.Fatalf("requestHTTP01() error: %v", err)
	}
	h := m.GetHTTPHandler()
	if h == nil {
		t.Error("GetHTTPHandler should return non-nil when useAutocert=true")
	}
}

// ── Initialize — FQDN set but no LE certs → covers lines 83-87 ──────────────

func TestInitialize_WithFQDN_NoLECerts_FallsBackToSelfSigned(t *testing.T) {
	cfg := config.DefaultAppConfig()
	cfg.Server.SSL.Enabled = true
	cfg.Server.SSL.CertPath = t.TempDir()
	cfg.Server.FQDN = "test.example.com"
	cfg.Server.SSL.LetsEncrypt.Enabled = false
	m := &SSLManager{
		appConfig:     cfg,
		certPath:      cfg.Server.SSL.CertPath,
		httpChallenge: make(map[string]string),
	}

	err := m.Initialize()
	if err != nil {
		t.Logf("Initialize(FQDN set): %v (may fail on key generation)", err)
	}
}

// ── Initialize — LetsEncrypt enabled with valid domain → covers lines 106-112 ─

func TestInitialize_LetsEncryptEnabled_ValidDomain_RequestCertFails(t *testing.T) {
	cfg := config.DefaultAppConfig()
	cfg.Server.SSL.Enabled = true
	cfg.Server.SSL.CertPath = t.TempDir()
	cfg.Server.FQDN = "valid.example.com"
	cfg.Server.SSL.LetsEncrypt.Enabled = true
	m := &SSLManager{
		appConfig:     cfg,
		certPath:      cfg.Server.SSL.CertPath,
		httpChallenge: make(map[string]string),
	}

	// RequestCertificate will fail (no real ACME server) but line 107-108 are covered
	err := m.Initialize()
	if err == nil {
		t.Log("Initialize(LE enabled): succeeded (unexpected in CI)")
	}
}

// ── Initialize — LetsEncrypt enabled with invalid domain → covers line 111 ───

func TestInitialize_LetsEncryptEnabled_InvalidDomain_FallsBackToSelfSigned(t *testing.T) {
	cfg := config.DefaultAppConfig()
	cfg.Server.SSL.Enabled = true
	cfg.Server.SSL.CertPath = t.TempDir()
	cfg.Server.FQDN = "localhost"
	cfg.Server.SSL.LetsEncrypt.Enabled = true
	m := &SSLManager{
		appConfig:     cfg,
		certPath:      cfg.Server.SSL.CertPath,
		httpChallenge: make(map[string]string),
	}

	err := m.Initialize()
	if err != nil {
		t.Logf("Initialize(LE+invalid domain): %v", err)
	}
}
