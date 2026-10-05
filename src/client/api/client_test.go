// SPDX-License-Identifier: MIT
package api

import (
	"net/http"
	"strings"
	"testing"
)

// --- NewAPIClient ---

// TestNewAPIClient_NonNil verifies the constructor returns a non-nil client.
func TestNewAPIClient_NonNil(t *testing.T) {
	c := NewAPIClient("http://localhost:8080", "test", 30, "v1")
	if c == nil {
		t.Fatal("NewAPIClient returned nil")
	}
}

// TestNewAPIClient_BaseURLStored verifies baseURL is stored as-is.
func TestNewAPIClient_BaseURLStored(t *testing.T) {
	c := NewAPIClient("http://localhost:8080", "test", 30, "v1")
	if c.GetBaseURL() != "http://localhost:8080" {
		t.Errorf("GetBaseURL() = %q, want %q", c.GetBaseURL(), "http://localhost:8080")
	}
}

// TestNewAPIClient_TokenStored verifies the token field is set.
func TestNewAPIClient_TokenStored(t *testing.T) {
	c := NewAPIClient("http://localhost:8080", "mytoken", 30, "v1")
	if c.token != "mytoken" {
		t.Errorf("token = %q, want %q", c.token, "mytoken")
	}
}

// TestNewAPIClient_APIVersionStored verifies the apiVersion field is set.
func TestNewAPIClient_APIVersionStored(t *testing.T) {
	c := NewAPIClient("http://localhost:8080", "test", 30, "v1")
	if c.apiVersion != "v1" {
		t.Errorf("apiVersion = %q, want %q", c.apiVersion, "v1")
	}
}

// TestNewAPIClient_DefaultAPIVersion verifies that an empty apiVersion falls
// back to APIClientDefaultAPIVersion.
func TestNewAPIClient_DefaultAPIVersion(t *testing.T) {
	c := NewAPIClient("http://localhost:8080", "test", 30, "")
	if c.apiVersion != APIClientDefaultAPIVersion {
		t.Errorf("apiVersion = %q, want default %q", c.apiVersion, APIClientDefaultAPIVersion)
	}
}

// TestNewAPIClient_DefaultTimeout verifies that timeout <= 0 uses the default.
func TestNewAPIClient_DefaultTimeout(t *testing.T) {
	c := NewAPIClient("http://localhost:8080", "test", 0, "v1")
	if c == nil {
		t.Fatal("NewAPIClient(timeout=0) returned nil")
	}
	// httpClient is unexported; verify indirectly that construction did not panic.
	if c.httpClient == nil {
		t.Fatal("httpClient is nil after construction with timeout=0")
	}
}

// TestNewAPIClient_NegativeTimeout verifies negative timeout uses the default.
func TestNewAPIClient_NegativeTimeout(t *testing.T) {
	c := NewAPIClient("http://localhost:8080", "test", -1, "v1")
	if c == nil {
		t.Fatal("NewAPIClient(timeout=-1) returned nil")
	}
	if c.httpClient == nil {
		t.Fatal("httpClient is nil after construction with timeout=-1")
	}
}

// TestNewAPIClient_EmptyBaseURL verifies empty baseURL is accepted without
// panicking (server URL is optional at construction time).
func TestNewAPIClient_EmptyBaseURL(t *testing.T) {
	c := NewAPIClient("", "test", 30, "v1")
	if c == nil {
		t.Fatal("NewAPIClient with empty baseURL returned nil")
	}
}

// TestNewAPIClient_EmptyToken verifies an empty token is accepted.
func TestNewAPIClient_EmptyToken(t *testing.T) {
	c := NewAPIClient("http://localhost:8080", "", 30, "v1")
	if c == nil {
		t.Fatal("NewAPIClient with empty token returned nil")
	}
	if c.token != "" {
		t.Errorf("token = %q, want empty string", c.token)
	}
}

// TestNewAPIClient_APIVersionLeadingSlashStripped verifies that a leading slash
// in apiVersion is normalised away.
func TestNewAPIClient_APIVersionLeadingSlashStripped(t *testing.T) {
	c := NewAPIClient("http://localhost:8080", "test", 30, "/v2")
	if strings.HasPrefix(c.apiVersion, "/") {
		t.Errorf("apiVersion = %q still has leading slash", c.apiVersion)
	}
}

// TestNewAPIClient_HTTPClientNonNil verifies the internal http.Client is
// initialised.
func TestNewAPIClient_HTTPClientNonNil(t *testing.T) {
	c := NewAPIClient("http://localhost:8080", "test", 30, "v1")
	if c.httpClient == nil {
		t.Fatal("httpClient is nil after construction")
	}
}

// TestNewAPIClient_DefaultUserAgent verifies the default user-agent is set to
// "vidveil-cli/dev" before SetUserAgent is called.
func TestNewAPIClient_DefaultUserAgent(t *testing.T) {
	c := NewAPIClient("http://localhost:8080", "test", 30, "v1")
	if !strings.HasPrefix(c.userAgent, "vidveil-cli/") {
		t.Errorf("userAgent = %q, want 'vidveil-cli/' prefix", c.userAgent)
	}
}

// --- SetUserAgent ---

// TestSetUserAgent_NoPanic verifies SetUserAgent does not panic.
func TestSetUserAgent_NoPanic(t *testing.T) {
	defer func() {
		if r := recover(); r != nil {
			t.Errorf("SetUserAgent panicked: %v", r)
		}
	}()
	c := NewAPIClient("http://localhost:8080", "test", 30, "v1")
	c.SetUserAgent("1.2.3")
}

// TestSetUserAgent_FormatsCorrectly verifies the user-agent string is built as
// "vidveil-cli/<version>".
func TestSetUserAgent_FormatsCorrectly(t *testing.T) {
	c := NewAPIClient("http://localhost:8080", "test", 30, "v1")
	c.SetUserAgent("1.2.3")
	want := "vidveil-cli/1.2.3"
	if c.userAgent != want {
		t.Errorf("userAgent = %q, want %q", c.userAgent, want)
	}
}

// TestSetUserAgent_EmptyVersion verifies that an empty version string does not
// panic and produces a well-formed (though minimal) user-agent.
func TestSetUserAgent_EmptyVersion(t *testing.T) {
	c := NewAPIClient("http://localhost:8080", "test", 30, "v1")
	c.SetUserAgent("")
	if c.userAgent == "" {
		t.Error("userAgent should not be empty after SetUserAgent('')")
	}
}

// TestSetUserAgent_OverwritesPreviousValue verifies repeated calls replace the
// stored value rather than append.
func TestSetUserAgent_OverwritesPreviousValue(t *testing.T) {
	c := NewAPIClient("http://localhost:8080", "test", 30, "v1")
	c.SetUserAgent("1.0.0")
	c.SetUserAgent("2.0.0")
	want := "vidveil-cli/2.0.0"
	if c.userAgent != want {
		t.Errorf("userAgent = %q, want %q after second SetUserAgent", c.userAgent, want)
	}
}

// --- GetBaseURL ---

// TestGetBaseURL_ReturnsStoredValue verifies the base URL is returned unchanged.
func TestGetBaseURL_ReturnsStoredValue(t *testing.T) {
	c := NewAPIClient("http://localhost:8080", "test", 30, "v1")
	if c.GetBaseURL() != "http://localhost:8080" {
		t.Errorf("GetBaseURL() = %q, want %q", c.GetBaseURL(), "http://localhost:8080")
	}
}

// TestGetBaseURL_NoTrailingSlash verifies that a URL passed without a trailing
// slash is returned without one.
func TestGetBaseURL_NoTrailingSlash(t *testing.T) {
	c := NewAPIClient("http://localhost:8080", "test", 30, "v1")
	if strings.HasSuffix(c.GetBaseURL(), "/") {
		t.Errorf("GetBaseURL() = %q has unexpected trailing slash", c.GetBaseURL())
	}
}

// TestGetBaseURL_EmptyWhenNotSet verifies an empty-string base URL is returned
// as-is (the client is valid but unresolvable until a URL is set externally).
func TestGetBaseURL_EmptyWhenNotSet(t *testing.T) {
	c := NewAPIClient("", "test", 30, "v1")
	if c.GetBaseURL() != "" {
		t.Errorf("GetBaseURL() = %q, want empty string", c.GetBaseURL())
	}
}

// --- GetAPIBaseURL ---

// TestGetAPIBaseURL_ContainsBaseURL verifies the base URL prefix is present.
func TestGetAPIBaseURL_ContainsBaseURL(t *testing.T) {
	c := NewAPIClient("http://localhost:8080", "test", 30, "v1")
	apiBase := c.GetAPIBaseURL()
	if !strings.HasPrefix(apiBase, "http://localhost:8080") {
		t.Errorf("GetAPIBaseURL() = %q, want prefix 'http://localhost:8080'", apiBase)
	}
}

// TestGetAPIBaseURL_ContainsVersionSegment verifies the API version is embedded
// in the URL path.
func TestGetAPIBaseURL_ContainsVersionSegment(t *testing.T) {
	c := NewAPIClient("http://localhost:8080", "test", 30, "v1")
	apiBase := c.GetAPIBaseURL()
	if !strings.Contains(apiBase, "v1") {
		t.Errorf("GetAPIBaseURL() = %q does not contain version segment 'v1'", apiBase)
	}
}

// TestGetAPIBaseURL_Format verifies the full expected path structure.
func TestGetAPIBaseURL_Format(t *testing.T) {
	c := NewAPIClient("http://localhost:8080", "test", 30, "v1")
	want := "http://localhost:8080/api/v1"
	if c.GetAPIBaseURL() != want {
		t.Errorf("GetAPIBaseURL() = %q, want %q", c.GetAPIBaseURL(), want)
	}
}

// TestGetAPIBaseURL_AlternateVersion verifies a non-default version is
// correctly embedded.
func TestGetAPIBaseURL_AlternateVersion(t *testing.T) {
	c := NewAPIClient("http://example.com", "test", 30, "v2")
	want := "http://example.com/api/v2"
	if c.GetAPIBaseURL() != want {
		t.Errorf("GetAPIBaseURL() = %q, want %q", c.GetAPIBaseURL(), want)
	}
}

// --- Exported constants ---

// TestConstants_Defaults verifies the exported default constants have the
// expected zero / non-zero values so callers can rely on them.
func TestConstants_Defaults(t *testing.T) {
	if APIClientDefaultAPIVersion == "" {
		t.Error("APIClientDefaultAPIVersion must not be empty")
	}
	if APIClientDefaultTimeoutSeconds <= 0 {
		t.Errorf("APIClientDefaultTimeoutSeconds = %d, must be positive", APIClientDefaultTimeoutSeconds)
	}
}

// --- Accept-Language header ---

// TestNormalizeAcceptLanguage_Accepted verifies valid BCP 47-style tags survive
// normalization with surrounding whitespace trimmed.
func TestNormalizeAcceptLanguage_Accepted(t *testing.T) {
	tests := []struct {
		in   string
		want string
	}{
		{"en", "en"},
		{"de-DE", "de-DE"},
		{"pt_BR", "pt_BR"},
		{"  fr-CA  ", "fr-CA"},
		{"zh-Hans-CN", "zh-Hans-CN"},
	}
	for _, tt := range tests {
		if got := normalizeAcceptLanguage(tt.in); got != tt.want {
			t.Errorf("normalizeAcceptLanguage(%q) = %q, want %q", tt.in, got, tt.want)
		}
	}
}

// TestNormalizeAcceptLanguage_Rejected verifies header-injection attempts and
// otherwise invalid values are dropped so no header is sent.
func TestNormalizeAcceptLanguage_Rejected(t *testing.T) {
	tests := []string{
		"",
		"   ",
		"en\r\nX-Injected: 1",
		"en\nX-Injected",
		"en,de",
		"en;q=0.8",
		"en;us",
		"../etc/passwd",
		strings.Repeat("a", 36),
	}
	for _, in := range tests {
		if got := normalizeAcceptLanguage(in); got != "" {
			t.Errorf("normalizeAcceptLanguage(%q) = %q, want empty", in, got)
		}
	}
}

// TestSetLanguage_DefaultEmpty verifies a fresh client sends no Accept-Language
// header unless one is configured.
func TestSetLanguage_DefaultEmpty(t *testing.T) {
	if got := (&APIClient{}).language; got != "" {
		t.Errorf("language = %q, want empty", got)
	}
}

// TestSetLanguage_Valid verifies a valid tag is stored.
func TestSetLanguage_Valid(t *testing.T) {
	c := &APIClient{}
	c.SetLanguage("fr-CA")
	if c.language != "fr-CA" {
		t.Errorf("language = %q, want %q", c.language, "fr-CA")
	}
}

// TestSetLanguage_Invalid verifies an invalid tag is discarded rather than
// stored, leaving the previous valid value in place.
func TestSetLanguage_Invalid(t *testing.T) {
	c := &APIClient{}
	c.SetLanguage("de")
	c.SetLanguage("en\r\nX-Injected: 1")
	if c.language != "de" {
		t.Errorf("language = %q, want %q (invalid value should be ignored)", c.language, "de")
	}
}

// TestApplyCommonHeaders verifies User-Agent is always sent and
// Accept-Language is sent only when a language is configured.
func TestApplyCommonHeaders(t *testing.T) {
	c := &APIClient{}
	req, err := http.NewRequest("GET", "http://example.com", nil)
	if err != nil {
		t.Fatalf("NewRequest: %v", err)
	}
	c.applyCommonHeaders(req)
	if got := req.Header.Get("User-Agent"); got != "" {
		t.Errorf("User-Agent = %q, want empty (unset client)", got)
	}
	if _, ok := req.Header["Accept-Language"]; ok {
		t.Error("Accept-Language must be absent when no language is configured")
	}

	c.userAgent = "vidveil-cli/1.2.3"
	c.SetLanguage("es")
	req2, err := http.NewRequest("GET", "http://example.com", nil)
	if err != nil {
		t.Fatalf("NewRequest: %v", err)
	}
	c.applyCommonHeaders(req2)
	if got := req2.Header.Get("User-Agent"); got != "vidveil-cli/1.2.3" {
		t.Errorf("User-Agent = %q, want %q", got, "vidveil-cli/1.2.3")
	}
	if got := req2.Header.Get("Accept-Language"); got != "es" {
		t.Errorf("Accept-Language = %q, want %q", got, "es")
	}
}
