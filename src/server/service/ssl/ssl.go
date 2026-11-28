// SPDX-License-Identifier: MIT
// AI.md PART 15: SSL/TLS & Let's Encrypt Support
package ssl

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/sha256"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"encoding/pem"
	"fmt"
	"log"
	"math/big"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"golang.org/x/crypto/acme/autocert"

	"github.com/apimgr/vidveil/src/config"
	"github.com/apimgr/vidveil/src/server/service/database"
)

// SSLManager handles SSL/TLS certificates including Let's Encrypt
type SSLManager struct {
	appConfig *config.AppConfig
	db        *database.AppDatabase
	// certPath is the base SSL directory ({config_dir}/ssl or CertPath override)
	certPath string
	// configDir is the config root used to build spec-compliant subdirectory paths
	configDir string
	mu        sync.RWMutex
	// certificate currently loaded into TLS config
	certificate *tls.Certificate
	// token -> keyAuth for HTTP-01 challenges
	httpChallenge map[string]string
	// autocertMgr is the ACME autocert manager (HTTP-01/TLS-ALPN-01)
	autocertMgr *autocert.Manager
	// useAutocert selects the autocert code path for GetTLSConfig/GetHTTPHandler
	useAutocert bool
	// systemCert is true when the loaded cert is under /etc/letsencrypt — never auto-renewed
	systemCert bool
	// userCert is true when the loaded cert is under {config_dir}/ssl/local — never auto-renewed
	userCert bool
}

// SetDatabase enables persistence of DNS provider validation state.
func (m *SSLManager) SetDatabase(db *database.AppDatabase) { m.db = db }

func (m *SSLManager) dnsConfigFingerprint() (string, error) {
	le := m.appConfig.Server.SSL.LetsEncrypt
	payload, err := json.Marshal(struct {
		Provider    string            `json:"provider"`
		Credentials map[string]string `json:"credentials"`
	}{le.DNSProvider, le.DNSCredentials})
	if err != nil {
		return "", err
	}
	digest := sha256.Sum256(payload)
	return hex.EncodeToString(digest[:]), nil
}

func syncDNSConfigStateTx(ctx context.Context, tx *sql.Tx, fingerprint string) error {
	var old string
	err := tx.QueryRowContext(ctx, "SELECT value FROM app_secrets WHERE key = ?", "dns_config_fingerprint").Scan(&old)
	if err != nil && err != sql.ErrNoRows {
		return err
	}
	if old != "" && old != fingerprint {
		if _, err := tx.ExecContext(ctx, "DELETE FROM app_secrets WHERE key = ?", "dns_validated_at"); err != nil {
			return err
		}
	}
	_, err = tx.ExecContext(ctx, "INSERT INTO app_secrets (key, value) VALUES (?, ?) ON CONFLICT(key) DO UPDATE SET value = excluded.value", "dns_config_fingerprint", fingerprint)
	return err
}

func (m *SSLManager) syncDNSConfigState() error {
	if m.db == nil {
		return nil
	}
	fingerprint, err := m.dnsConfigFingerprint()
	if err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	return m.db.Write(ctx, func(tx *sql.Tx) error {
		return syncDNSConfigStateTx(ctx, tx, fingerprint)
	})
}

func (m *SSLManager) recordDNSValidation() error {
	if m.db == nil {
		return nil
	}
	fingerprint, err := m.dnsConfigFingerprint()
	if err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	return m.db.Write(ctx, func(tx *sql.Tx) error {
		if err := syncDNSConfigStateTx(ctx, tx, fingerprint); err != nil {
			return err
		}
		_, err := tx.ExecContext(ctx, "INSERT INTO app_secrets (key, value) VALUES (?, ?) ON CONFLICT(key) DO UPDATE SET value = excluded.value", "dns_validated_at", time.Now().UTC().Format(time.RFC3339))
		return err
	})
}

// CertInfo contains certificate information
type CertInfo struct {
	Domain      string    `json:"domain"`
	Issuer      string    `json:"issuer"`
	NotBefore   time.Time `json:"not_before"`
	NotAfter    time.Time `json:"not_after"`
	DaysLeft    int       `json:"days_left"`
	IsValid     bool      `json:"is_valid"`
	AutoRenew   bool      `json:"auto_renew"`
	ChallenType string    `json:"challenge_type"`
}

// NewSSLManager creates a new SSL manager.
// configDir is the OS-appropriate config root (e.g. /etc/apimgr/vidveil on Linux).
// Pass "" to auto-detect via GetAppPaths.
func NewSSLManager(appConfig *config.AppConfig, configDir string) *SSLManager {
	if configDir == "" {
		paths := config.GetAppPaths("", "")
		configDir = paths.Config
	}

	certPath := appConfig.Server.SSL.CertPath
	if certPath == "" {
		// Per AI.md PART 15: cert base dir is {config_dir}/ssl/
		certPath = filepath.Join(configDir, "ssl")
	}

	return &SSLManager{
		appConfig:     appConfig,
		certPath:      certPath,
		configDir:     configDir,
		httpChallenge: make(map[string]string),
	}
}

// Initialize sets up SSL if enabled.
// Certificate lookup follows AI.md PART 15 priority order:
//  1. /etc/letsencrypt/live/domain/    (literal "domain" dir — system certbot setup)
//  2. /etc/letsencrypt/live/{fqdn}/    (FQDN-named system certbot dir)
//  3. {config_dir}/ssl/letsencrypt/{fqdn}/  (app-managed, auto-renews at 7 days)
//  4. {config_dir}/ssl/local/{fqdn}/   (user-managed, no auto-renewal)
func (m *SSLManager) Initialize() error {
	if !m.appConfig.Server.SSL.Enabled {
		return nil
	}
	if m.appConfig.Server.SSL.LetsEncrypt.Enabled {
		if err := m.syncDNSConfigState(); err != nil {
			return fmt.Errorf("sync DNS provider state: %w", err)
		}
	}
	if m.appConfig.Server.SSL.LetsEncrypt.Enabled && strings.EqualFold(m.appConfig.Server.SSL.LetsEncrypt.Challenge, "dns-01") {
		// Validation logs the failure; existing certificates can still be loaded.
		_ = m.ValidateDNSProvider()
	}

	fqdn := m.appConfig.Server.FQDN

	if fqdn != "" {
		// Priority 1: /etc/letsencrypt/live/domain/ (literal "domain" directory)
		p1 := "/etc/letsencrypt/live/domain"
		if certExists(p1, "fullchain.pem", "privkey.pem") {
			if err := m.loadCertificate(
				filepath.Join(p1, "fullchain.pem"),
				filepath.Join(p1, "privkey.pem"),
			); err == nil {
				m.systemCert = true
				return nil
			}
		}

		// Priority 2: /etc/letsencrypt/live/{fqdn}/
		p2 := filepath.Join("/etc/letsencrypt/live", fqdn)
		if certExists(p2, "fullchain.pem", "privkey.pem") {
			if err := m.loadCertificate(
				filepath.Join(p2, "fullchain.pem"),
				filepath.Join(p2, "privkey.pem"),
			); err == nil {
				m.systemCert = true
				return nil
			}
		}

		// Priority 3: {config_dir}/ssl/letsencrypt/{fqdn}/ (app manages, auto-renews)
		p3 := filepath.Join(m.configDir, "ssl", "letsencrypt", fqdn)
		if certExists(p3, "fullchain.pem", "privkey.pem") {
			if err := m.loadCertificate(
				filepath.Join(p3, "fullchain.pem"),
				filepath.Join(p3, "privkey.pem"),
			); err == nil {
				return nil
			}
		}

		// Priority 4: {config_dir}/ssl/local/{fqdn}/ (user manages, no auto-renewal)
		p4 := filepath.Join(m.configDir, "ssl", "local", fqdn)
		if certExists(p4, "cert.pem", "key.pem") {
			if err := m.loadCertificate(
				filepath.Join(p4, "cert.pem"),
				filepath.Join(p4, "key.pem"),
			); err == nil {
				m.userCert = true
				return nil
			}
		}
	}

	// Fallback for no-FQDN or when none of the 4 paths have a cert:
	// check legacy flat path (used by self-signed certs generated on previous runs)
	legacyCert := filepath.Join(m.certPath, "cert.pem")
	legacyKey := filepath.Join(m.certPath, "key.pem")
	if certExists(m.certPath, "cert.pem", "key.pem") {
		if err := m.loadCertificate(legacyCert, legacyKey); err == nil {
			return nil
		}
	}

	// No existing cert found — request via Let's Encrypt or generate self-signed
	if m.appConfig.Server.SSL.LetsEncrypt.Enabled && fqdn != "" {
		if config.IsValidSSLHost(fqdn) {
			return m.RequestCertificate(fqdn)
		}
		log.Printf("WARN: domain %q is not valid for Let's Encrypt; using a self-signed certificate", fqdn)
	}

	return m.generateSelfSigned()
}

// certExists returns true when both files exist and are readable under dir.
func certExists(dir, certFile, keyFile string) bool {
	if _, err := os.Stat(filepath.Join(dir, certFile)); err != nil {
		return false
	}
	_, err := os.Stat(filepath.Join(dir, keyFile))
	return err == nil
}

// loadCertificate loads an existing certificate
func (m *SSLManager) loadCertificate(certFile, keyFile string) error {
	cert, err := tls.LoadX509KeyPair(certFile, keyFile)
	if err != nil {
		return fmt.Errorf("failed to load certificate: %w", err)
	}
	if _, err := validateCertificate(&cert, m.appConfig.Server.FQDN); err != nil {
		return err
	}
	m.mu.Lock()
	m.certificate = &cert
	m.mu.Unlock()

	return nil
}

func validateCertificate(cert *tls.Certificate, hostname string) (*x509.Certificate, error) {
	if cert == nil || len(cert.Certificate) == 0 {
		return nil, fmt.Errorf("failed to load certificate: empty certificate chain")
	}
	leaf, err := x509.ParseCertificate(cert.Certificate[0])
	if err != nil {
		return nil, fmt.Errorf("failed to parse certificate: %w", err)
	}
	now := time.Now()
	if now.Before(leaf.NotBefore) || !now.Before(leaf.NotAfter) {
		return nil, fmt.Errorf("certificate is not currently valid")
	}
	if hostname = strings.TrimSpace(hostname); hostname != "" {
		if err := leaf.VerifyHostname(hostname); err != nil && !certificateCommonNameMatches(leaf.Subject.CommonName, hostname) {
			return nil, fmt.Errorf("certificate does not match configured hostname")
		}
	}
	cert.Leaf = leaf
	return leaf, nil
}

func certificateCommonNameMatches(commonName, hostname string) bool {
	commonName = strings.TrimSuffix(strings.ToLower(strings.TrimSpace(commonName)), ".")
	hostname = strings.TrimSuffix(strings.ToLower(strings.TrimSpace(hostname)), ".")
	if commonName == hostname {
		return true
	}
	if !strings.HasPrefix(commonName, "*.") || strings.Count(commonName, ".") != strings.Count(hostname, ".") {
		return false
	}
	return strings.HasSuffix(hostname, commonName[1:])
}

// generateSelfSigned generates a self-signed certificate
func (m *SSLManager) generateSelfSigned() error {
	privateKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return fmt.Errorf("failed to generate private key: %w", err)
	}

	serialNumber, err := rand.Int(rand.Reader, new(big.Int).Lsh(big.NewInt(1), 128))
	if err != nil {
		return fmt.Errorf("failed to generate serial number: %w", err)
	}

	domain := m.appConfig.Server.FQDN
	if domain == "" {
		domain, _ = os.Hostname()
	}

	template := x509.Certificate{
		SerialNumber: serialNumber,
		Subject: pkix.Name{
			Organization: []string{"Vidveil Self-Signed"},
			CommonName:   domain,
		},
		NotBefore:             time.Now(),
		NotAfter:              time.Now().Add(365 * 24 * time.Hour),
		KeyUsage:              x509.KeyUsageKeyEncipherment | x509.KeyUsageDigitalSignature,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		BasicConstraintsValid: true,
		DNSNames:              []string{domain, "localhost"},
		IPAddresses:           []net.IP{net.ParseIP("127.0.0.1"), net.ParseIP("::1")},
	}

	certDER, err := x509.CreateCertificate(rand.Reader, &template, &template, &privateKey.PublicKey, privateKey)
	if err != nil {
		return fmt.Errorf("failed to create certificate: %w", err)
	}

	// Save certificate
	certFile := filepath.Join(m.certPath, "cert.pem")
	certOut, err := os.Create(certFile)
	if err != nil {
		return fmt.Errorf("failed to create cert file: %w", err)
	}
	if err := pem.Encode(certOut, &pem.Block{Type: "CERTIFICATE", Bytes: certDER}); err != nil {
		_ = certOut.Close()
		return fmt.Errorf("write certificate: %w", err)
	}
	if err := certOut.Close(); err != nil {
		return fmt.Errorf("close certificate: %w", err)
	}

	// Save private key
	keyFile := filepath.Join(m.certPath, "key.pem")
	keyOut, err := os.OpenFile(keyFile, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0600)
	if err != nil {
		return fmt.Errorf("failed to create key file: %w", err)
	}
	keyBytes, err := x509.MarshalECPrivateKey(privateKey)
	if err != nil {
		return fmt.Errorf("failed to marshal private key: %w", err)
	}
	if err := pem.Encode(keyOut, &pem.Block{Type: "EC PRIVATE KEY", Bytes: keyBytes}); err != nil {
		_ = keyOut.Close()
		return fmt.Errorf("write private key: %w", err)
	}
	if err := keyOut.Close(); err != nil {
		return fmt.Errorf("close private key: %w", err)
	}

	return m.loadCertificate(certFile, keyFile)
}

// RequestCertificate requests a certificate from Let's Encrypt
// Supports HTTP-01, TLS-ALPN-01, and DNS-01 challenges per AI.md PART 15
func (m *SSLManager) RequestCertificate(domain string) error {
	if !config.IsValidSSLHost(domain) {
		return fmt.Errorf("invalid domain for SSL: %s", domain)
	}

	challenge := strings.ToLower(m.appConfig.Server.SSL.LetsEncrypt.Challenge)
	if challenge == "dns-01" {
		if err := m.syncDNSConfigState(); err != nil {
			return fmt.Errorf("sync DNS provider state: %w", err)
		}
		if err := m.ValidateDNSProvider(); err != nil {
			return fmt.Errorf("DNS-01 provider validation failed: %w", err)
		}
	}

	switch challenge {
	case "http-01":
		return m.requestHTTP01(domain)
	case "tls-alpn-01":
		return m.requestTLSALPN01(domain)
	case "dns-01":
		return m.requestDNS01(domain)
	default:
		// Default to HTTP-01
		return m.requestHTTP01(domain)
	}
}

// requestHTTP01 requests a certificate using HTTP-01 challenge via autocert
func (m *SSLManager) requestHTTP01(domain string) error {
	// HTTP-01 challenge requires port 80 to be accessible
	log.Printf("INFO: requesting Let's Encrypt certificate for %s using HTTP-01 challenge", domain)

	// Get email for Let's Encrypt registration
	email := m.appConfig.Server.SSL.LetsEncrypt.Email
	if email == "" {
		email = "admin@" + domain
	}

	// Create autocert manager for HTTP-01 challenge
	m.autocertMgr = &autocert.Manager{
		Prompt:     autocert.AcceptTOS,
		Cache:      newManagedACMECache(m.configDir, domain),
		HostPolicy: autocert.HostWhitelist(domain),
		Email:      email,
	}
	m.useAutocert = true

	log.Printf("INFO: Let's Encrypt HTTP-01 manager configured for %s; certificate issuance begins on the first TLS handshake", domain)

	return nil
}

// requestTLSALPN01 requests a certificate using TLS-ALPN-01 challenge
// This challenge is handled automatically by autocert when using TLS
func (m *SSLManager) requestTLSALPN01(domain string) error {
	log.Printf("INFO: requesting Let's Encrypt certificate for %s using TLS-ALPN-01 challenge", domain)

	// Get email for Let's Encrypt registration
	email := m.appConfig.Server.SSL.LetsEncrypt.Email
	if email == "" {
		email = "admin@" + domain
	}

	// TLS-ALPN-01 is the default challenge type in autocert
	// It works by responding to the challenge on the TLS port directly
	m.autocertMgr = &autocert.Manager{
		Prompt:     autocert.AcceptTOS,
		Cache:      newManagedACMECache(m.configDir, domain),
		HostPolicy: autocert.HostWhitelist(domain),
		Email:      email,
	}
	m.useAutocert = true

	log.Printf("INFO: Let's Encrypt TLS-ALPN-01 manager configured for %s; certificate issuance begins on the first TLS handshake", domain)

	return nil
}

// GetCertificate returns the current certificate for TLS config
func (m *SSLManager) GetCertificate(hello *tls.ClientHelloInfo) (*tls.Certificate, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if m.certificate == nil {
		return nil, fmt.Errorf("no certificate available")
	}

	return m.certificate, nil
}

// GetTLSConfig returns a TLS configuration
func (m *SSLManager) GetTLSConfig() *tls.Config {
	// If using autocert, use its TLS config
	if m.useAutocert && m.autocertMgr != nil {
		return m.autocertMgr.TLSConfig()
	}

	return &tls.Config{
		GetCertificate: m.GetCertificate,
		MinVersion:     tls.VersionTLS12,
		CipherSuites: []uint16{
			tls.TLS_ECDHE_ECDSA_WITH_AES_256_GCM_SHA384,
			tls.TLS_ECDHE_RSA_WITH_AES_256_GCM_SHA384,
			tls.TLS_ECDHE_ECDSA_WITH_AES_128_GCM_SHA256,
			tls.TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256,
		},
	}
}

// GetHTTPHandler returns the HTTP handler for ACME challenges
// This should be used to handle HTTP-01 challenges on port 80
func (m *SSLManager) GetHTTPHandler() http.Handler {
	if m.useAutocert && m.autocertMgr != nil {
		return m.autocertMgr.HTTPHandler(nil)
	}
	return http.HandlerFunc(m.HTTP01Handler)
}

// IsAutocertEnabled returns whether autocert is being used
func (m *SSLManager) IsAutocertEnabled() bool {
	return m.useAutocert
}

// GetCertInfo returns information about the current certificate
func (m *SSLManager) GetCertInfo() (*CertInfo, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if m.certificate == nil {
		return nil, fmt.Errorf("no certificate loaded")
	}

	if len(m.certificate.Certificate) == 0 {
		return nil, fmt.Errorf("empty certificate chain")
	}

	cert, err := x509.ParseCertificate(m.certificate.Certificate[0])
	if err != nil {
		return nil, fmt.Errorf("failed to parse certificate: %w", err)
	}

	daysLeft := int(time.Until(cert.NotAfter).Hours() / 24)

	domain := ""
	if len(cert.DNSNames) > 0 {
		domain = cert.DNSNames[0]
	} else {
		domain = cert.Subject.CommonName
	}

	return &CertInfo{
		Domain:      domain,
		Issuer:      cert.Issuer.CommonName,
		NotBefore:   cert.NotBefore,
		NotAfter:    cert.NotAfter,
		DaysLeft:    daysLeft,
		IsValid:     time.Now().Before(cert.NotAfter) && time.Now().After(cert.NotBefore),
		AutoRenew:   m.appConfig.Server.SSL.LetsEncrypt.Enabled,
		ChallenType: m.appConfig.Server.SSL.LetsEncrypt.Challenge,
	}, nil
}

// NeedsRenewal returns true when the app-managed cert expires within 30 days.
// Per AI.md PART 15: only {config_dir}/ssl/letsencrypt/{fqdn}/ certs are auto-renewed.
// System certs (/etc/letsencrypt/live/**) and user certs (ssl/local/**) are never renewed by the app.
func (m *SSLManager) NeedsRenewal() bool {
	// System certs and user-managed certs are never renewed by the app
	if m.systemCert || m.userCert {
		return false
	}
	info, err := m.GetCertInfo()
	if err != nil {
		// No cert loaded — attempt renewal to get one
		return true
	}
	return time.Until(info.NotAfter) <= 30*24*time.Hour
}

// RenewCertificate renews the certificate if needed
func (m *SSLManager) RenewCertificate(ctx context.Context) error {
	if !m.appConfig.Server.SSL.Enabled {
		return nil
	}

	if !m.NeedsRenewal() {
		return nil
	}

	domain := m.appConfig.Server.FQDN
	if domain == "" || !config.IsValidSSLHost(domain) {
		// Can't renew with Let's Encrypt, regenerate self-signed
		return m.generateSelfSigned()
	}

	if m.appConfig.Server.SSL.LetsEncrypt.Enabled {
		if err := m.RequestCertificate(domain); err != nil {
			return err
		}
		if m.useAutocert && m.autocertMgr != nil {
			cert, err := m.autocertMgr.GetCertificate(&tls.ClientHelloInfo{ServerName: domain})
			if err != nil {
				return fmt.Errorf("renew Let's Encrypt certificate: %w", err)
			}
			m.mu.Lock()
			m.certificate = cert
			m.mu.Unlock()
		}
		return nil
	}

	return m.generateSelfSigned()
}

// HTTP01Handler handles HTTP-01 ACME challenges
func (m *SSLManager) HTTP01Handler(w http.ResponseWriter, r *http.Request) {
	token := strings.TrimPrefix(r.URL.Path, "/.well-known/acme-challenge/")

	m.mu.RLock()
	keyAuth, ok := m.httpChallenge[token]
	m.mu.RUnlock()

	if !ok {
		http.NotFound(w, r)
		return
	}

	w.Header().Set("Content-Type", "text/plain")
	_, _ = w.Write([]byte(keyAuth))
}

// SetHTTP01Challenge sets an HTTP-01 challenge response
func (m *SSLManager) SetHTTP01Challenge(token, keyAuth string) {
	m.mu.Lock()
	m.httpChallenge[token] = keyAuth
	m.mu.Unlock()
}

// ClearHTTP01Challenge clears an HTTP-01 challenge
func (m *SSLManager) ClearHTTP01Challenge(token string) {
	m.mu.Lock()
	delete(m.httpChallenge, token)
	m.mu.Unlock()
}
