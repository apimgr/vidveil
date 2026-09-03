// SPDX-License-Identifier: MIT
package config

import (
	"os"
	"strings"
	"testing"
)

// TestValidateSEOVerification_AllValid verifies that well-formed codes for
// every provider produce no complaints.
func TestValidateSEOVerification_AllValid(t *testing.T) {
	v := SEOVerificationConfig{
		Google:    "abc_DEF-123",
		Bing:      "ABCDEF0123456789",
		Yandex:    "abcdef0123456789",
		Baidu:     "abcABC123",
		Pinterest: "abcdef0123456789",
		Facebook:  "abc0123456789",
		Custom: []SEOCustomTag{
			{Name: "my-tag_1", Content: "some-content"},
			{Property: "og:verify", Content: "value"},
		},
	}
	bad := validateSEOVerification(v)
	if len(bad) != 0 {
		t.Fatalf("expected no invalid fields, got %v", bad)
	}
}

// TestValidateSEOVerification_AllInvalid verifies each provider rejects a
// value that violates its charset pattern.
func TestValidateSEOVerification_AllInvalid(t *testing.T) {
	v := SEOVerificationConfig{
		Google:    strings.Repeat("a", 44), // too long (>43)
		Bing:      "lowercase-not-hex",
		Yandex:    "UPPERCASE",
		Baidu:     "has space",
		Pinterest: "ZZZ",
		Facebook:  "HasUppercase",
	}
	bad := validateSEOVerification(v)
	want := []string{
		"seo.verification.google",
		"seo.verification.bing",
		"seo.verification.yandex",
		"seo.verification.baidu",
		"seo.verification.pinterest",
		"seo.verification.facebook",
	}
	for _, w := range want {
		found := false
		for _, b := range bad {
			if b == w {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("expected %q in invalid list, got %v", w, bad)
		}
	}
}

// TestValidateSEOVerification_CustomBranches exercises the custom-tag paths:
// missing name/property, bad name charset, and bad content length.
func TestValidateSEOVerification_CustomBranches(t *testing.T) {
	v := SEOVerificationConfig{
		Custom: []SEOCustomTag{
			// neither name nor property
			{Content: "x"},
			// property-only, valid name, empty content
			{Property: "og:key", Content: ""},
			// invalid name charset
			{Name: "bad name!", Content: "ok"},
			// content too long (>256)
			{Name: "good", Content: strings.Repeat("z", 257)},
		},
	}
	bad := validateSEOVerification(v)
	expect := []string{
		"seo.verification.custom[0].name_or_property",
		"seo.verification.custom[1].content",
		"seo.verification.custom[2].name_or_property",
		"seo.verification.custom[3].content",
	}
	for _, e := range expect {
		found := false
		for _, b := range bad {
			if b == e {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("expected %q in %v", e, bad)
		}
	}
}

// TestValidateSEOVerification_EmptySkips confirms empty codes are skipped
// entirely (no validation, no complaints).
func TestValidateSEOVerification_EmptySkips(t *testing.T) {
	if bad := validateSEOVerification(SEOVerificationConfig{}); len(bad) != 0 {
		t.Fatalf("empty config should yield no invalid fields, got %v", bad)
	}
}

// TestGetDisplayHost_ProductionFQDN forces a production FQDN via DOMAIN and
// verifies GetDisplayHost returns it unchanged.
func TestGetDisplayHost_ProductionFQDN(t *testing.T) {
	old, had := os.LookupEnv("DOMAIN")
	t.Cleanup(func() {
		if had {
			os.Setenv("DOMAIN", old)
		} else {
			os.Unsetenv("DOMAIN")
		}
	})
	os.Setenv("DOMAIN", "search.example.com")
	if got := GetDisplayHost(nil); got != "search.example.com" {
		t.Fatalf("expected production FQDN returned verbatim, got %q", got)
	}
}

// TestGetFQDN_DomainWins confirms the DOMAIN override short-circuits.
func TestGetFQDN_DomainWins(t *testing.T) {
	old, had := os.LookupEnv("DOMAIN")
	t.Cleanup(func() {
		if had {
			os.Setenv("DOMAIN", old)
		} else {
			os.Unsetenv("DOMAIN")
		}
	})
	os.Setenv("DOMAIN", "explicit.host.tld")
	if got := GetFQDN(); got != "explicit.host.tld" {
		t.Fatalf("DOMAIN override should win, got %q", got)
	}
}
