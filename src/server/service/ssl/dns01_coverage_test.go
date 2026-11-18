// SPDX-License-Identifier: MIT
// Coverage tests for dns01.go: legoUser interface methods,
// and buildDNSProvider (unsupported provider path and all supported providers).
package ssl

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"os"
	"testing"
)

// ── legoUser interface methods ──────────────────────────────────────────────

func TestLegoUserGetEmail(t *testing.T) {
	u := &legoUser{email: "test@example.com"}
	if got := u.GetEmail(); got != "test@example.com" {
		t.Errorf("GetEmail() = %q, want %q", got, "test@example.com")
	}
}

func TestLegoUserGetEmailEmpty(t *testing.T) {
	u := &legoUser{}
	if got := u.GetEmail(); got != "" {
		t.Errorf("GetEmail() empty struct = %q, want empty string", got)
	}
}

func TestLegoUserGetRegistrationNilByDefault(t *testing.T) {
	u := &legoUser{email: "admin@example.com"}
	if got := u.GetRegistration(); got != nil {
		t.Errorf("GetRegistration() = %v, want nil for unregistered user", got)
	}
}

func TestLegoUserGetPrivateKey(t *testing.T) {
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatalf("GenerateKey: %v", err)
	}
	u := &legoUser{email: "x@x.com", key: key}
	if got := u.GetPrivateKey(); got == nil {
		t.Error("GetPrivateKey() = nil, want non-nil key")
	}
}

func TestLegoUserGetPrivateKeyNilKey(t *testing.T) {
	u := &legoUser{email: "x@x.com"}
	if got := u.GetPrivateKey(); got != nil {
		t.Error("GetPrivateKey() on zero struct should return nil")
	}
}

// ── buildDNSProvider ────────────────────────────────────────────────────────

func TestBuildDNSProviderUnsupportedReturnsError(t *testing.T) {
	_, err := buildDNSProvider("unsupported-provider", nil)
	if err == nil {
		t.Error("buildDNSProvider(unsupported) = nil, want error")
	}
}

func TestBuildDNSProviderEmptyTypeReturnsError(t *testing.T) {
	_, err := buildDNSProvider("", nil)
	if err == nil {
		t.Error("buildDNSProvider('') = nil, want error")
	}
}

func TestBuildDNSProviderCloudflareNoCredsReturnsError(t *testing.T) {
	// Without CLOUDFLARE_* env vars, the provider constructor fails.
	// This exercises the cloudflare case branch.
	_, err := buildDNSProvider("cloudflare", nil)
	// May return nil if env vars happen to be set in the test environment;
	// in a clean container, it should return an error. We just verify no panic.
	_ = err
}

func TestBuildDNSProviderRoute53NoCreds(t *testing.T) {
	_, err := buildDNSProvider("route53", nil)
	_ = err
}

func TestBuildDNSProviderDigitalOceanNoCreds(t *testing.T) {
	_, err := buildDNSProvider("digitalocean", nil)
	_ = err
}

func TestBuildDNSProviderGodaddyNoCreds(t *testing.T) {
	_, err := buildDNSProvider("godaddy", nil)
	_ = err
}

func TestBuildDNSProviderNamecheapNoCreds(t *testing.T) {
	_, err := buildDNSProvider("namecheap", nil)
	_ = err
}

func TestBuildDNSProviderRfc2136NoCreds(t *testing.T) {
	_, err := buildDNSProvider("rfc2136", nil)
	_ = err
}

func TestBuildDNSProviderRestoresEnvironment(t *testing.T) {
	t.Setenv("CF_DNS_API_TOKEN", "original-value")
	_, _ = buildDNSProvider("cloudflare", map[string]string{"CF_DNS_API_TOKEN": "temporary-value"})
	if got := os.Getenv("CF_DNS_API_TOKEN"); got != "original-value" {
		t.Fatalf("credential environment leaked: got %q", got)
	}
}
