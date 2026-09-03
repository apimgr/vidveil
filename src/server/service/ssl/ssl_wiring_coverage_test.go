// SPDX-License-Identifier: MIT
// AI.md PART 15: Additional coverage for SSL manager construction and the
// certificate-source branches exercised by the TLS serving wiring.
package ssl

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"math/big"
	"net"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/apimgr/vidveil/src/config"
)

// writeCertPair generates a throwaway self-signed cert+key PEM pair into dir
// under the given filenames, creating dir if needed.
func writeCertPair(t *testing.T, dir, certName, keyName, cn string) {
	t.Helper()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("MkdirAll %s: %v", dir, err)
	}
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatalf("GenerateKey: %v", err)
	}
	tmpl := &x509.Certificate{
		SerialNumber: big.NewInt(2),
		Subject:      pkix.Name{CommonName: cn},
		NotBefore:    time.Now().Add(-time.Hour),
		NotAfter:     time.Now().Add(365 * 24 * time.Hour),
		DNSNames:     []string{cn},
		IPAddresses:  []net.IP{net.ParseIP("127.0.0.1")},
	}
	der, err := x509.CreateCertificate(rand.Reader, tmpl, tmpl, &key.PublicKey, key)
	if err != nil {
		t.Fatalf("CreateCertificate: %v", err)
	}
	certPEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der})
	if err := os.WriteFile(filepath.Join(dir, certName), certPEM, 0o644); err != nil {
		t.Fatalf("write cert: %v", err)
	}
	keyDER, err := x509.MarshalECPrivateKey(key)
	if err != nil {
		t.Fatalf("MarshalECPrivateKey: %v", err)
	}
	keyPEM := pem.EncodeToMemory(&pem.Block{Type: "EC PRIVATE KEY", Bytes: keyDER})
	if err := os.WriteFile(filepath.Join(dir, keyName), keyPEM, 0o600); err != nil {
		t.Fatalf("write key: %v", err)
	}
}

// newFQDNManager builds an enabled manager with a fixed FQDN and temp cert/config
// dirs so the app-managed and user-managed certificate priority branches in
// Initialize are exercisable without touching /etc/letsencrypt.
func newFQDNManager(t *testing.T, fqdn string) *SSLManager {
	t.Helper()
	cfg := config.DefaultAppConfig()
	cfg.Server.SSL.Enabled = true
	cfg.Server.FQDN = fqdn
	cfg.Server.SSL.LetsEncrypt.Enabled = false
	return &SSLManager{
		appConfig:     cfg,
		certPath:      t.TempDir(),
		configDir:     t.TempDir(),
		httpChallenge: make(map[string]string),
	}
}

// ---- NewSSLManager ----

func TestNewSSLManager_DefaultCertPathUnderConfigDir(t *testing.T) {
	cfg := config.DefaultAppConfig()
	cfg.Server.SSL.CertPath = ""
	dir := t.TempDir()
	m := NewSSLManager(cfg, dir)
	if m == nil {
		t.Fatal("NewSSLManager returned nil")
	}
	want := filepath.Join(dir, "ssl")
	if m.certPath != want {
		t.Fatalf("certPath = %q, want %q", m.certPath, want)
	}
}

func TestNewSSLManager_ExplicitCertPathHonored(t *testing.T) {
	cfg := config.DefaultAppConfig()
	custom := t.TempDir()
	cfg.Server.SSL.CertPath = custom
	m := NewSSLManager(cfg, t.TempDir())
	if m.certPath != custom {
		t.Fatalf("certPath = %q, want explicit %q", m.certPath, custom)
	}
}

// ---- Initialize: legacy flat cert fallback ----

func TestInitialize_LoadsLegacyFlatCert(t *testing.T) {
	// Generate a self-signed cert into a dir, then a fresh manager pointed at that
	// same dir must load it via the legacy flat-path fallback in Initialize.
	src := newEnabledSSLManager(t)
	if err := src.generateSelfSigned(); err != nil {
		t.Fatalf("generateSelfSigned: %v", err)
	}
	if _, err := os.Stat(filepath.Join(src.certPath, "cert.pem")); err != nil {
		t.Fatalf("expected cert.pem in %s: %v", src.certPath, err)
	}

	cfg := config.DefaultAppConfig()
	cfg.Server.SSL.Enabled = true
	cfg.Server.SSL.CertPath = src.certPath
	cfg.Server.FQDN = ""
	cfg.Server.SSL.LetsEncrypt.Enabled = false
	m := &SSLManager{
		appConfig:     cfg,
		certPath:      src.certPath,
		configDir:     t.TempDir(),
		httpChallenge: make(map[string]string),
	}
	if err := m.Initialize(); err != nil {
		t.Fatalf("Initialize with legacy cert error: %v", err)
	}
	m.mu.RLock()
	loaded := m.certificate
	m.mu.RUnlock()
	if loaded == nil {
		t.Error("Initialize should have loaded the legacy flat certificate")
	}
}

func TestInitialize_DisabledReturnsNilImmediately(t *testing.T) {
	cfg := config.DefaultAppConfig()
	cfg.Server.SSL.Enabled = false
	m := NewSSLManager(cfg, t.TempDir())
	if err := m.Initialize(); err != nil {
		t.Fatalf("Initialize with SSL disabled should be a no-op, got: %v", err)
	}
}

// ---- NeedsRenewal: system/user cert branches ----

func TestNeedsRenewal_SystemCertNeverRenews(t *testing.T) {
	m := newEnabledSSLManager(t)
	m.systemCert = true
	if m.NeedsRenewal() {
		t.Error("system-managed certs must never be flagged for renewal")
	}
}

func TestNeedsRenewal_UserCertNeverRenews(t *testing.T) {
	m := newEnabledSSLManager(t)
	m.userCert = true
	if m.NeedsRenewal() {
		t.Error("user-managed certs must never be flagged for renewal")
	}
}

// ---- GetCertificate: returns loaded cert ----

func TestGetCertificate_ReturnsLoadedCert(t *testing.T) {
	m := newEnabledSSLManager(t)
	if err := m.generateSelfSigned(); err != nil {
		t.Fatalf("generateSelfSigned: %v", err)
	}
	cert, err := m.GetCertificate(nil)
	if err != nil {
		t.Fatalf("GetCertificate error: %v", err)
	}
	if cert == nil {
		t.Error("GetCertificate returned nil for a loaded certificate")
	}
}

// ---- Initialize: app-managed and user-managed priority branches ----

func TestInitialize_LoadsAppManagedLetsEncryptCert(t *testing.T) {
	// Priority 3: {config_dir}/ssl/letsencrypt/{fqdn}/ fullchain.pem + privkey.pem.
	// Loaded cert must NOT be flagged system or user managed (it auto-renews).
	fqdn := "app.example.test"
	m := newFQDNManager(t, fqdn)
	dir := filepath.Join(m.configDir, "ssl", "letsencrypt", fqdn)
	writeCertPair(t, dir, "fullchain.pem", "privkey.pem", fqdn)

	if err := m.Initialize(); err != nil {
		t.Fatalf("Initialize error: %v", err)
	}
	m.mu.RLock()
	loaded := m.certificate
	m.mu.RUnlock()
	if loaded == nil {
		t.Fatal("Initialize should have loaded the app-managed Let's Encrypt cert")
	}
	if m.systemCert {
		t.Error("app-managed cert must not be flagged systemCert")
	}
	if m.userCert {
		t.Error("app-managed cert must not be flagged userCert")
	}
}

func TestInitialize_LoadsUserManagedLocalCert(t *testing.T) {
	// Priority 4: {config_dir}/ssl/local/{fqdn}/ cert.pem + key.pem sets userCert=true.
	fqdn := "user.example.test"
	m := newFQDNManager(t, fqdn)
	dir := filepath.Join(m.configDir, "ssl", "local", fqdn)
	writeCertPair(t, dir, "cert.pem", "key.pem", fqdn)

	if err := m.Initialize(); err != nil {
		t.Fatalf("Initialize error: %v", err)
	}
	m.mu.RLock()
	loaded := m.certificate
	m.mu.RUnlock()
	if loaded == nil {
		t.Fatal("Initialize should have loaded the user-managed local cert")
	}
	if !m.userCert {
		t.Error("user-managed local cert must set userCert=true")
	}
	if m.NeedsRenewal() {
		t.Error("user-managed cert must never be flagged for renewal")
	}
}

// ---- GetCertInfo ----

func TestGetCertInfo_HappyPath(t *testing.T) {
	m := newEnabledSSLManager(t)
	if err := m.generateSelfSigned(); err != nil {
		t.Fatalf("generateSelfSigned: %v", err)
	}
	info, err := m.GetCertInfo()
	if err != nil {
		t.Fatalf("GetCertInfo error: %v", err)
	}
	if info.Domain == "" {
		t.Error("GetCertInfo returned empty domain")
	}
	if info.DaysLeft <= 0 {
		t.Errorf("GetCertInfo DaysLeft = %d, want > 0 for a fresh cert", info.DaysLeft)
	}
	if !info.IsValid {
		t.Error("a freshly generated cert should report IsValid=true")
	}
}

func TestGetCertInfo_NoCertLoaded(t *testing.T) {
	m := newEnabledSSLManager(t)
	if _, err := m.GetCertInfo(); err == nil {
		t.Error("GetCertInfo should error when no certificate is loaded")
	}
}

// ---- RenewCertificate ----

func TestRenewCertificate_DisabledIsNoOp(t *testing.T) {
	m := newEnabledSSLManager(t)
	m.appConfig.Server.SSL.Enabled = false
	if err := m.RenewCertificate(context.Background()); err != nil {
		t.Fatalf("RenewCertificate with SSL disabled should be a no-op, got: %v", err)
	}
}

func TestRenewCertificate_FreshCertNoOp(t *testing.T) {
	// A freshly generated self-signed cert has ~365 days left, so NeedsRenewal
	// is false and RenewCertificate returns nil without regenerating.
	m := newEnabledSSLManager(t)
	if err := m.generateSelfSigned(); err != nil {
		t.Fatalf("generateSelfSigned: %v", err)
	}
	if err := m.RenewCertificate(context.Background()); err != nil {
		t.Fatalf("RenewCertificate on a fresh cert should be a no-op, got: %v", err)
	}
}

func TestRenewCertificate_RegeneratesSelfSignedWhenNoCert(t *testing.T) {
	// No cert loaded and no valid FQDN: NeedsRenewal is true, IsValidSSLHost is
	// false, so RenewCertificate regenerates a self-signed cert.
	m := newEnabledSSLManager(t)
	m.appConfig.Server.FQDN = ""
	m.appConfig.Server.SSL.LetsEncrypt.Enabled = false
	if err := m.RenewCertificate(context.Background()); err != nil {
		t.Fatalf("RenewCertificate should regenerate self-signed, got: %v", err)
	}
	m.mu.RLock()
	loaded := m.certificate
	m.mu.RUnlock()
	if loaded == nil {
		t.Error("RenewCertificate should have generated a self-signed certificate")
	}
}
