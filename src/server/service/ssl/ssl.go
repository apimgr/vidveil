// SPDX-License-Identifier: MIT
// AI.md PART 15: SSL/TLS & Let's Encrypt Support
package ssl

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"fmt"
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
)

// SSLManager handles SSL/TLS certificates including Let's Encrypt
type SSLManager struct {
	appConfig   *config.AppConfig
	certPath    string
	mu          sync.RWMutex
	certificate *tls.Certificate
	// token -> keyAuth for HTTP-01
	httpChallenge map[string]string
	// For Let's Encrypt ACME
	autocertMgr *autocert.Manager
	// Whether to use autocert for certificate management
	useAutocert bool
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

// NewSSLManager creates a new SSL manager
func NewSSLManager(appConfig *config.AppConfig) *SSLManager {
	certPath := appConfig.Server.SSL.CertPath
	if certPath == "" {
		paths := config.GetAppPaths("", "")
		certPath = filepath.Join(paths.Config, "ssl", "certs")
	}

	return &SSLManager{
		appConfig:     appConfig,
		certPath:      certPath,
		httpChallenge: make(map[string]string),
	}
}

// Initialize sets up SSL if enabled
func (m *SSLManager) Initialize() error {
	if !m.appConfig.Server.SSL.Enabled {
		return nil
	}

	// Ensure cert directory exists
	if err := os.MkdirAll(m.certPath, 0700); err != nil {
		return fmt.Errorf("failed to create cert directory: %w", err)
	}

	// Check for existing Let's Encrypt certs first (per AI.md PART 8)
	letsEncryptPath := "/etc/letsencrypt/live"
	domain := m.appConfig.Server.FQDN
	if domain != "" {
		leCertPath := filepath.Join(letsEncryptPath, domain, "fullchain.pem")
		leKeyPath := filepath.Join(letsEncryptPath, domain, "privkey.pem")
		if _, err := os.Stat(leCertPath); err == nil {
			if _, err := os.Stat(leKeyPath); err == nil {
				// Copy Let's Encrypt certs to our path
				return m.copyLetsEncryptCerts(leCertPath, leKeyPath)
			}
		}
	}

	// Check for existing certs in our path
	certFile := filepath.Join(m.certPath, "cert.pem")
	keyFile := filepath.Join(m.certPath, "key.pem")

	if _, err := os.Stat(certFile); err == nil {
		if _, err := os.Stat(keyFile); err == nil {
			return m.loadCertificate(certFile, keyFile)
		}
	}

	// If Let's Encrypt is enabled and we have a valid domain, request cert
	if m.appConfig.Server.SSL.LetsEncrypt.Enabled && domain != "" {
		if config.IsValidSSLHost(domain) {
			return m.RequestCertificate(domain)
		}
		// Invalid domain for SSL - log warning and use self-signed
		fmt.Printf("Warning: Domain '%s' is not valid for Let's Encrypt. Using self-signed certificate.\n", domain)
	}

	// Generate self-signed certificate as fallback
	return m.generateSelfSigned()
}

// copyLetsEncryptCerts copies certs from Let's Encrypt directory
func (m *SSLManager) copyLetsEncryptCerts(certPath, keyPath string) error {
	destCert := filepath.Join(m.certPath, "cert.pem")
	destKey := filepath.Join(m.certPath, "key.pem")

	// Read and copy cert
	certData, err := os.ReadFile(certPath)
	if err != nil {
		return fmt.Errorf("failed to read Let's Encrypt cert: %w", err)
	}
	if err := os.WriteFile(destCert, certData, 0644); err != nil {
		return fmt.Errorf("failed to write cert: %w", err)
	}

	// Read and copy key
	keyData, err := os.ReadFile(keyPath)
	if err != nil {
		return fmt.Errorf("failed to read Let's Encrypt key: %w", err)
	}
	if err := os.WriteFile(destKey, keyData, 0600); err != nil {
		return fmt.Errorf("failed to write key: %w", err)
	}

	return m.loadCertificate(destCert, destKey)
}

// loadCertificate loads an existing certificate
func (m *SSLManager) loadCertificate(certFile, keyFile string) error {
	cert, err := tls.LoadX509KeyPair(certFile, keyFile)
	if err != nil {
		return fmt.Errorf("failed to load certificate: %w", err)
	}

	m.mu.Lock()
	m.certificate = &cert
	m.mu.Unlock()

	return nil
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
	pem.Encode(certOut, &pem.Block{Type: "CERTIFICATE", Bytes: certDER})
	certOut.Close()

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
	pem.Encode(keyOut, &pem.Block{Type: "EC PRIVATE KEY", Bytes: keyBytes})
	keyOut.Close()

	return m.loadCertificate(certFile, keyFile)
}

// RequestCertificate requests a certificate from Let's Encrypt
// Supports HTTP-01, TLS-ALPN-01, and DNS-01 challenges per AI.md PART 8
func (m *SSLManager) RequestCertificate(domain string) error {
	if !config.IsValidSSLHost(domain) {
		return fmt.Errorf("invalid domain for SSL: %s", domain)
	}

	challenge := strings.ToLower(m.appConfig.Server.SSL.LetsEncrypt.Challenge)

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
	fmt.Printf("Requesting Let's Encrypt certificate for %s using HTTP-01 challenge\n", domain)

	// Get email for Let's Encrypt registration
	email := m.appConfig.Server.SSL.LetsEncrypt.Email
	if email == "" {
		email = "admin@" + domain
	}

	// Create autocert manager for HTTP-01 challenge
	m.autocertMgr = &autocert.Manager{
		Prompt:     autocert.AcceptTOS,
		Cache:      autocert.DirCache(m.certPath),
		HostPolicy: autocert.HostWhitelist(domain),
		Email:      email,
	}
	m.useAutocert = true

	fmt.Printf("Let's Encrypt autocert manager configured for domain: %s\n", domain)
	fmt.Println("Note: Certificate will be obtained on first TLS handshake.")
	fmt.Println("Ensure port 80 is accessible for HTTP-01 challenge verification.")

	return nil
}

// requestTLSALPN01 requests a certificate using TLS-ALPN-01 challenge
// This challenge is handled automatically by autocert when using TLS
func (m *SSLManager) requestTLSALPN01(domain string) error {
	fmt.Printf("Requesting Let's Encrypt certificate for %s using TLS-ALPN-01 challenge\n", domain)

	// Get email for Let's Encrypt registration
	email := m.appConfig.Server.SSL.LetsEncrypt.Email
	if email == "" {
		email = "admin@" + domain
	}

	// TLS-ALPN-01 is the default challenge type in autocert
	// It works by responding to the challenge on the TLS port directly
	m.autocertMgr = &autocert.Manager{
		Prompt:     autocert.AcceptTOS,
		Cache:      autocert.DirCache(m.certPath),
		HostPolicy: autocert.HostWhitelist(domain),
		Email:      email,
	}
	m.useAutocert = true

	fmt.Printf("Let's Encrypt autocert manager configured for domain: %s (TLS-ALPN-01)\n", domain)
	fmt.Println("Note: Certificate will be obtained on first TLS handshake.")
	fmt.Println("Ensure port 443 is accessible for TLS-ALPN-01 challenge verification.")

	return nil
}

// requestDNS01 requests a certificate using DNS-01 challenge
// DNS-01 requires external DNS provider integration which is not directly supported by autocert
// This implementation falls back to self-signed but can use external certbot-generated certs
func (m *SSLManager) requestDNS01(domain string) error {
	fmt.Printf("Requesting Let's Encrypt certificate for %s using DNS-01 challenge\n", domain)

	providerType := m.appConfig.Server.SSL.LetsEncrypt.DNSProviderType
	if providerType == "" {
		return fmt.Errorf("DNS-01 challenge requires dns_provider_type to be set")
	}

	// Check if certbot/external tool has generated certs
	letsEncryptPath := "/etc/letsencrypt/live"
	leCertPath := filepath.Join(letsEncryptPath, domain, "fullchain.pem")
	leKeyPath := filepath.Join(letsEncryptPath, domain, "privkey.pem")

	if _, err := os.Stat(leCertPath); err == nil {
		if _, err := os.Stat(leKeyPath); err == nil {
			fmt.Printf("Found Let's Encrypt certificates for %s generated via DNS-01\n", domain)
			return m.copyLetsEncryptCerts(leCertPath, leKeyPath)
		}
	}

	fmt.Printf("DNS-01 challenge for provider '%s' requires external certificate generation.\n", providerType)
	fmt.Println("Options:")
	fmt.Println("  1. Use certbot with DNS plugin: certbot certonly --dns-cloudflare -d " + domain)
	fmt.Println("  2. Use HTTP-01 or TLS-ALPN-01 challenge instead")
	fmt.Println("Falling back to self-signed certificate.")
	return m.generateSelfSigned()
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

// NeedsRenewal checks if the certificate needs renewal (< 30 days)
func (m *SSLManager) NeedsRenewal() bool {
	info, err := m.GetCertInfo()
	if err != nil {
		return true
	}
	return info.DaysLeft < 30
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
		return m.RequestCertificate(domain)
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
	w.Write([]byte(keyAuth))
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
