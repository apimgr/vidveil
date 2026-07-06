// SPDX-License-Identifier: MIT
// AI.md PART 15: DNS-01 challenge implementation via go-acme/lego.
// Supports: cloudflare, route53, digitalocean, godaddy, namecheap, rfc2136.
package ssl

import (
	"crypto"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/go-acme/lego/v4/certcrypto"
	"github.com/go-acme/lego/v4/certificate"
	"github.com/go-acme/lego/v4/challenge"
	"github.com/go-acme/lego/v4/challenge/dns01"
	"github.com/go-acme/lego/v4/lego"
	"github.com/go-acme/lego/v4/providers/dns/cloudflare"
	"github.com/go-acme/lego/v4/providers/dns/digitalocean"
	"github.com/go-acme/lego/v4/providers/dns/godaddy"
	"github.com/go-acme/lego/v4/providers/dns/namecheap"
	"github.com/go-acme/lego/v4/providers/dns/rfc2136"
	"github.com/go-acme/lego/v4/providers/dns/route53"
	"github.com/go-acme/lego/v4/registration"
)

// legoUser implements registration.User for lego certificate requests.
type legoUser struct {
	email        string
	registration *registration.Resource
	key          crypto.PrivateKey
}

func (u *legoUser) GetEmail() string                        { return u.email }
func (u *legoUser) GetRegistration() *registration.Resource { return u.registration }
func (u *legoUser) GetPrivateKey() crypto.PrivateKey        { return u.key }

// requestDNS01 requests a Let's Encrypt certificate using DNS-01 challenge via lego.
// Provider is selected from server.ssl.letsencrypt.dns_provider_type.
// Credentials are provided as JSON in server.ssl.letsencrypt.dns_provider_key.
func (m *SSLManager) requestDNS01(domain string) error {
	providerType := m.appConfig.Server.SSL.LetsEncrypt.DNSProviderType
	if providerType == "" {
		return fmt.Errorf("DNS-01: dns_provider_type must be set in server.yml (e.g. cloudflare, route53, digitalocean)")
	}

	email := m.appConfig.Server.SSL.LetsEncrypt.Email
	if email == "" {
		email = "admin@" + domain
	}

	credsJSON := m.appConfig.Server.SSL.LetsEncrypt.DNSProviderKey

	// Generate account key for ACME registration
	accountKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return fmt.Errorf("DNS-01: failed to generate account key: %w", err)
	}

	user := &legoUser{email: email, key: accountKey}

	cfg := lego.NewConfig(user)
	cfg.Certificate.KeyType = certcrypto.RSA2048

	client, err := lego.NewClient(cfg)
	if err != nil {
		return fmt.Errorf("DNS-01: failed to create lego client: %w", err)
	}

	// Build DNS provider from type and credentials
	provider, err := buildDNSProvider(providerType, credsJSON)
	if err != nil {
		return fmt.Errorf("DNS-01: %w", err)
	}

	if err := client.Challenge.SetDNS01Provider(provider,
		dns01.AddRecursiveNameservers([]string{"8.8.8.8:53", "1.1.1.1:53"}),
	); err != nil {
		return fmt.Errorf("DNS-01: failed to set provider: %w", err)
	}

	// Register ACME account
	reg, err := client.Registration.Register(registration.RegisterOptions{TermsOfServiceAgreed: true})
	if err != nil {
		return fmt.Errorf("DNS-01: ACME registration failed: %w", err)
	}
	user.registration = reg

	// Request certificate
	request := certificate.ObtainRequest{
		Domains: []string{domain},
		Bundle:  true,
	}
	certs, err := client.Certificate.Obtain(request)
	if err != nil {
		return fmt.Errorf("DNS-01: certificate obtain failed: %w", err)
	}

	// Write certificate and key to {config_dir}/ssl/letsencrypt/{domain}/ per AI.md PART 15.
	// This is the app-managed path; the scheduler auto-renews it 7 days before expiry.
	leDir := filepath.Join(m.configDir, "ssl", "letsencrypt", domain)
	if err := os.MkdirAll(leDir, 0o700); err != nil {
		return fmt.Errorf("DNS-01: failed to create cert dir: %w", err)
	}
	certFile := filepath.Join(leDir, "fullchain.pem")
	keyFile := filepath.Join(leDir, "privkey.pem")

	if err := os.WriteFile(certFile, certs.Certificate, 0o600); err != nil {
		return fmt.Errorf("DNS-01: failed to write cert: %w", err)
	}
	if err := os.WriteFile(keyFile, certs.PrivateKey, 0o600); err != nil {
		return fmt.Errorf("DNS-01: failed to write key: %w", err)
	}

	// Parse and store the certificate
	tlsCert, err := tls.X509KeyPair(certs.Certificate, certs.PrivateKey)
	if err != nil {
		return fmt.Errorf("DNS-01: failed to parse certificate: %w", err)
	}

	leaf, err := x509.ParseCertificate(tlsCert.Certificate[0])
	if err != nil {
		return fmt.Errorf("DNS-01: failed to parse leaf certificate: %w", err)
	}
	tlsCert.Leaf = leaf

	m.mu.Lock()
	m.certificate = &tlsCert
	m.mu.Unlock()

	fmt.Printf("DNS-01: certificate obtained for %s (expires %s)\n",
		domain, leaf.NotAfter.Format(time.RFC1123))

	return nil
}

// buildDNSProvider constructs a lego DNS provider from type name and JSON credentials.
// credsJSON is a JSON object whose fields correspond to the provider's environment variables.
// If credsJSON is empty, the provider reads credentials from environment variables.
func buildDNSProvider(providerType, credsJSON string) (challenge.Provider, error) {
	// Apply credentials as environment variables if provided
	if credsJSON != "" {
		if err := applyCredsFromJSON(credsJSON); err != nil {
			return nil, fmt.Errorf("failed to parse dns_provider_key JSON: %w", err)
		}
	}

	switch providerType {
	case "cloudflare":
		p, err := cloudflare.NewDNSProvider()
		return p, err

	case "route53":
		p, err := route53.NewDNSProvider()
		return p, err

	case "digitalocean":
		p, err := digitalocean.NewDNSProvider()
		return p, err

	case "godaddy":
		p, err := godaddy.NewDNSProvider()
		return p, err

	case "namecheap":
		p, err := namecheap.NewDNSProvider()
		return p, err

	case "rfc2136":
		p, err := rfc2136.NewDNSProvider()
		return p, err

	default:
		return nil, fmt.Errorf("unsupported dns_provider_type %q — supported: cloudflare, route53, digitalocean, godaddy, namecheap, rfc2136", providerType)
	}
}

// applyCredsFromJSON sets environment variables from a JSON credentials map.
// Keys become env var names (upper-cased with _ separator as needed by each provider).
func applyCredsFromJSON(credsJSON string) error {
	var creds map[string]string
	if err := json.Unmarshal([]byte(credsJSON), &creds); err != nil {
		return err
	}
	for k, v := range creds {
		if err := os.Setenv(k, v); err != nil {
			return fmt.Errorf("failed to set env %s: %w", k, err)
		}
	}
	return nil
}

