// SPDX-License-Identifier: MIT
// AI.md PART 15: DNS-01 challenge implementation via go-acme/lego.
// ALL lego DNS providers are supported; the provider is selected by name at
// runtime and its credential fields are read from the process environment.
package ssl

import (
	"context"
	"crypto"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"fmt"
	"log"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/go-acme/lego/v4/certcrypto"
	"github.com/go-acme/lego/v4/certificate"
	"github.com/go-acme/lego/v4/challenge"
	"github.com/go-acme/lego/v4/challenge/dns01"
	"github.com/go-acme/lego/v4/lego"
	"github.com/go-acme/lego/v4/providers/dns"
	"github.com/go-acme/lego/v4/registration"
)

// legoUser implements registration.User for lego certificate requests.
var dnsProviderEnvMu sync.Mutex

type dnsSavedEnv struct {
	value string
	set   bool
}

type legoUser struct {
	email        string
	registration *registration.Resource
	key          crypto.PrivateKey
}

func (u *legoUser) GetEmail() string                        { return u.email }
func (u *legoUser) GetRegistration() *registration.Resource { return u.registration }
func (u *legoUser) GetPrivateKey() crypto.PrivateKey        { return u.key }

// requestDNS01 requests a Let's Encrypt certificate using DNS-01 challenge via lego.
// Provider and credentials come from server.ssl.letsencrypt.dns_provider and
// server.ssl.letsencrypt.dns_credentials.
func (m *SSLManager) requestDNS01(domain string) error {
	providerType := m.appConfig.Server.SSL.LetsEncrypt.DNSProvider
	if providerType == "" {
		return fmt.Errorf("DNS-01: dns_provider must be set in server.yml (e.g. cloudflare, route53, digitalocean)")
	}

	email := m.appConfig.Server.SSL.LetsEncrypt.Email
	if email == "" {
		email = "admin@" + domain
	}

	creds := m.appConfig.Server.SSL.LetsEncrypt.DNSCredentials

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
	provider, err := buildDNSProvider(providerType, creds)
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

	tlsCert, err := tls.X509KeyPair(certs.Certificate, certs.PrivateKey)
	if err != nil {
		return fmt.Errorf("DNS-01: failed to parse certificate: %w", err)
	}
	leaf, err := validateCertificate(&tlsCert, domain)
	if err != nil {
		return fmt.Errorf("DNS-01: invalid issued certificate: %w", err)
	}
	// Managed certificates use the same staged pair replacement and rollback
	// behavior as autocert so a failed renewal cannot leave mismatched files.
	cache := newManagedACMECache(m.configDir, domain)
	cacheValue := append(append([]byte(nil), certs.PrivateKey...), certs.Certificate...)
	if err := cache.Put(context.Background(), domain, cacheValue); err != nil {
		return fmt.Errorf("DNS-01: failed to persist managed certificate pair: %w", err)
	}

	m.mu.Lock()
	m.certificate = &tlsCert
	m.mu.Unlock()
	if err := m.recordDNSValidation(); err != nil {
		return fmt.Errorf("certificate issued but DNS validation state could not be stored: %w", err)
	}

	log.Printf("INFO: DNS-01 certificate obtained for %s (expires %s)",
		domain, leaf.NotAfter.Format(time.RFC1123))

	return nil
}

// ValidateDNSProvider constructs the configured lego provider without creating
// or changing DNS records.
func (m *SSLManager) ValidateDNSProvider() error {
	providerName := m.appConfig.Server.SSL.LetsEncrypt.DNSProvider
	if providerName == "" {
		return fmt.Errorf("dns_provider must be set")
	}
	_, err := buildDNSProvider(providerName, m.appConfig.Server.SSL.LetsEncrypt.DNSCredentials)
	if err != nil {
		log.Printf("ERROR: DNS-01 provider validation failed: %v", err)
	}
	return err
}

// buildDNSProvider scopes credentials to lego's provider constructor and restores
// the caller's environment before returning. lego providers capture credentials
// during construction, so they remain usable without retaining environment values.
func buildDNSProvider(providerName string, credentials map[string]string) (challenge.Provider, error) {
	if strings.TrimSpace(providerName) == "" {
		return nil, fmt.Errorf("dns_provider must be set")
	}
	dnsProviderEnvMu.Lock()
	defer dnsProviderEnvMu.Unlock()
	saved := make(map[string]dnsSavedEnv, len(credentials))
	for name, value := range credentials {
		old, set := os.LookupEnv(name)
		saved[name] = dnsSavedEnv{value: old, set: set}
		if err := os.Setenv(name, value); err != nil {
			restoreDNSEnvironment(saved)
			return nil, fmt.Errorf("cannot set environment variable %s", name)
		}
	}
	provider, err := dns.NewDNSChallengeProviderByName(providerName)
	restoreDNSEnvironment(saved)
	if err != nil {
		message := err.Error()
		for _, value := range credentials {
			if value != "" {
				message = strings.ReplaceAll(message, value, "[redacted]")
			}
		}
		return nil, fmt.Errorf("dns_provider %q: %s", providerName, message)
	}
	return provider, nil
}

func restoreDNSEnvironment(saved map[string]dnsSavedEnv) {
	for name, old := range saved {
		if old.set {
			_ = os.Setenv(name, old.value)
		} else {
			_ = os.Unsetenv(name)
		}
	}
}
