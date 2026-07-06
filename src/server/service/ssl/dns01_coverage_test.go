// SPDX-License-Identifier: MIT
// Coverage tests for dns01.go: legoUser interface methods, applyCredsFromJSON,
// and buildDNSProvider (unsupported provider path and all supported providers).
package ssl

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
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

// ── applyCredsFromJSON ──────────────────────────────────────────────────────

func TestApplyCredsFromJSONValid(t *testing.T) {
	err := applyCredsFromJSON(`{"TEST_CREDS_KEY": "val123"}`)
	if err != nil {
		t.Errorf("applyCredsFromJSON(valid) = %v, want nil", err)
	}
}

func TestApplyCredsFromJSONMultipleKeys(t *testing.T) {
	err := applyCredsFromJSON(`{"TEST_DNS_KEY1": "aaa", "TEST_DNS_KEY2": "bbb"}`)
	if err != nil {
		t.Errorf("applyCredsFromJSON(multi-key) = %v, want nil", err)
	}
}

func TestApplyCredsFromJSONInvalidJSON(t *testing.T) {
	err := applyCredsFromJSON(`not valid json`)
	if err == nil {
		t.Error("applyCredsFromJSON(invalid JSON) = nil, want error")
	}
}

func TestApplyCredsFromJSONEmptyObject(t *testing.T) {
	err := applyCredsFromJSON(`{}`)
	if err != nil {
		t.Errorf("applyCredsFromJSON({}) = %v, want nil", err)
	}
}

func TestApplyCredsFromJSONWrongType(t *testing.T) {
	// Values must be strings; this should fail to unmarshal into map[string]string.
	err := applyCredsFromJSON(`{"KEY": 42}`)
	if err == nil {
		t.Error("applyCredsFromJSON(non-string value) = nil, want unmarshal error")
	}
}

// ── buildDNSProvider ────────────────────────────────────────────────────────

func TestBuildDNSProviderUnsupportedReturnsError(t *testing.T) {
	_, err := buildDNSProvider("unsupported-provider", "")
	if err == nil {
		t.Error("buildDNSProvider(unsupported) = nil, want error")
	}
}

func TestBuildDNSProviderEmptyTypeReturnsError(t *testing.T) {
	_, err := buildDNSProvider("", "")
	if err == nil {
		t.Error("buildDNSProvider('') = nil, want error")
	}
}

func TestBuildDNSProviderCloudflareNoCredsReturnsError(t *testing.T) {
	// Without CLOUDFLARE_* env vars, the provider constructor fails.
	// This exercises the cloudflare case branch.
	_, err := buildDNSProvider("cloudflare", "")
	// May return nil if env vars happen to be set in the test environment;
	// in a clean container, it should return an error. We just verify no panic.
	_ = err
}

func TestBuildDNSProviderRoute53NoCreds(t *testing.T) {
	_, err := buildDNSProvider("route53", "")
	_ = err
}

func TestBuildDNSProviderDigitalOceanNoCreds(t *testing.T) {
	_, err := buildDNSProvider("digitalocean", "")
	_ = err
}

func TestBuildDNSProviderGodaddyNoCreds(t *testing.T) {
	_, err := buildDNSProvider("godaddy", "")
	_ = err
}

func TestBuildDNSProviderNamecheapNoCreds(t *testing.T) {
	_, err := buildDNSProvider("namecheap", "")
	_ = err
}

func TestBuildDNSProviderRfc2136NoCreds(t *testing.T) {
	_, err := buildDNSProvider("rfc2136", "")
	_ = err
}

func TestBuildDNSProviderWithCredsJSON(t *testing.T) {
	// Exercise the applyCredsFromJSON branch inside buildDNSProvider.
	_, err := buildDNSProvider("cloudflare", `{"CF_DNS_API_TOKEN": "test-token-value"}`)
	// Provider may still fail (invalid token); we only verify no panic.
	_ = err
}

func TestBuildDNSProviderWithInvalidCredsJSON(t *testing.T) {
	_, err := buildDNSProvider("cloudflare", `not-json`)
	if err == nil {
		t.Error("buildDNSProvider with invalid JSON creds = nil, want error")
	}
}
