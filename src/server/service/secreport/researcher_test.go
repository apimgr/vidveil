// SPDX-License-Identifier: MIT
// AI.md PART 11: Security Reports — unit tests for researcher GPG key resolution.
package secreport

import (
	"strings"
	"testing"
)

func TestResolveResearcherKey_EmptyReturnsNil(t *testing.T) {
	key, err := ResolveResearcherKey("")
	if err != nil {
		t.Fatalf("ResolveResearcherKey: unexpected error %v", err)
	}
	if key != nil {
		t.Fatalf("ResolveResearcherKey: want nil key for empty input, got %q", key)
	}

	key, err = ResolveResearcherKey("   ")
	if err != nil {
		t.Fatalf("ResolveResearcherKey: unexpected error %v", err)
	}
	if key != nil {
		t.Fatalf("ResolveResearcherKey: want nil key for whitespace-only input, got %q", key)
	}
}

func TestResolveResearcherKey_PastedBlockPassthrough(t *testing.T) {
	block := "-----BEGIN PGP PUBLIC KEY BLOCK-----\n...\n-----END PGP PUBLIC KEY BLOCK-----"
	key, err := ResolveResearcherKey(block)
	if err != nil {
		t.Fatalf("ResolveResearcherKey: unexpected error %v", err)
	}
	if string(key) != block {
		t.Fatalf("ResolveResearcherKey: want passthrough of pasted block, got %q", key)
	}
}

func TestResolveResearcherKey_RejectsNonHTTPS(t *testing.T) {
	if _, err := ResolveResearcherKey("http://keys.openpgp.org/vks/v1/by-fingerprint/ABC"); err == nil {
		t.Fatalf("ResolveResearcherKey: expected error for non-https URL")
	}
}

func TestResolveResearcherKey_RejectsMalformedURL(t *testing.T) {
	if _, err := ResolveResearcherKey("ht!tp://[not a url"); err == nil {
		t.Fatalf("ResolveResearcherKey: expected error for malformed URL")
	}
}

func TestResolveResearcherKey_RejectsNonAllowlistedHost(t *testing.T) {
	// Not a pasted key and not one of the allowlisted keyserver hostnames —
	// must be rejected before any network fetch is attempted (SSRF guard).
	_, err := ResolveResearcherKey("https://evil.example.com/steal-internal-metadata")
	if err == nil {
		t.Fatalf("ResolveResearcherKey: expected error for non-allowlisted host")
	}
	if !strings.Contains(err.Error(), "not an allowlisted keyserver") {
		t.Fatalf("ResolveResearcherKey: unexpected error message %q", err.Error())
	}
}
