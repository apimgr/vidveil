// SPDX-License-Identifier: MIT
// AI.md PART 8: URL Variables & Reverse Proxy Headers - unit tests
package urlvars

import (
	"crypto/tls"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

// --- DefaultURLVarsConfig ---

// DefaultURLVarsConfig must return a config with every field set to the
// documented default values so callers that embed it get sane behaviour
// without any extra configuration.
func TestDefaultURLVarsConfigFields(t *testing.T) {
	cfg := DefaultURLVarsConfig()

	if !cfg.Learning {
		t.Error("DefaultURLVarsConfig.Learning = false, want true")
	}
	if cfg.MinSamples != 3 {
		t.Errorf("DefaultURLVarsConfig.MinSamples = %d, want 3", cfg.MinSamples)
	}
	if cfg.SampleWindow != 5*time.Minute {
		t.Errorf("DefaultURLVarsConfig.SampleWindow = %v, want 5m", cfg.SampleWindow)
	}
	if !cfg.LogChanges {
		t.Error("DefaultURLVarsConfig.LogChanges = false, want true")
	}
	if !cfg.LiveReload {
		t.Error("DefaultURLVarsConfig.LiveReload = false, want true")
	}
}

// --- NewURLResolver ---

// NewURLResolver must return a non-nil resolver and store the supplied config.
func TestNewURLResolverNonNil(t *testing.T) {
	cfg := DefaultURLVarsConfig()
	r := NewURLResolver(cfg)
	if r == nil {
		t.Fatal("NewURLResolver returned nil")
	}
	if r.config != cfg {
		t.Error("NewURLResolver: stored config does not match supplied config")
	}
}

// The observations map must be initialised so callers do not panic on first
// call to GetURLVars.
func TestNewURLResolverObservationsInitialised(t *testing.T) {
	r := NewURLResolver(DefaultURLVarsConfig())
	if r.observations == nil {
		t.Error("NewURLResolver: observations map is nil")
	}
}

// --- SetLogger ---

// SetLogger must not panic when called with a valid logger function.
func TestSetLoggerNoPanic(t *testing.T) {
	r := NewURLResolver(DefaultURLVarsConfig())
	r.SetLogger(func(format string, args ...interface{}) {})
}

// SetLogger must not panic when called with nil (clearing the logger).
func TestSetLoggerNilNoPanic(t *testing.T) {
	r := NewURLResolver(DefaultURLVarsConfig())
	r.SetLogger(func(format string, args ...interface{}) {})
	r.SetLogger(nil)
}

// The logger function must actually be invoked when a base-domain change is
// detected, confirming that SetLogger wires it up correctly.
func TestSetLoggerIsInvoked(t *testing.T) {
	cfg := DefaultURLVarsConfig()
	cfg.MinSamples = 1
	r := NewURLResolver(cfg)

	called := false
	r.SetLogger(func(format string, args ...interface{}) {
		called = true
	})

	// Trigger inferPatterns by recording an observation directly.
	r.recordObservation("api.example.com")

	if !called {
		t.Error("logger was never called after a domain change")
	}
}

// --- resolveProto (via GetURLVars) ---

// X-Forwarded-Proto header takes priority and its value is lowercased.
func TestResolveProtoXForwardedProto(t *testing.T) {
	r := NewURLResolver(DefaultURLVarsConfig())
	req := httptest.NewRequest("GET", "/", nil)
	req.Header.Set("X-Forwarded-Proto", "HTTPS")

	proto, _, _ := r.GetURLVars(req)
	if proto != "https" {
		t.Errorf("proto = %q, want %q", proto, "https")
	}
}

// X-Forwarded-Ssl: on must be treated as https.
func TestResolveProtoXForwardedSslOn(t *testing.T) {
	r := NewURLResolver(DefaultURLVarsConfig())
	req := httptest.NewRequest("GET", "/", nil)
	req.Header.Set("X-Forwarded-Ssl", "on")

	proto, _, _ := r.GetURLVars(req)
	if proto != "https" {
		t.Errorf("proto = %q, want https", proto)
	}
}

// X-Forwarded-Ssl values other than "on" must not resolve to https.
func TestResolveProtoXForwardedSslOffIgnored(t *testing.T) {
	r := NewURLResolver(DefaultURLVarsConfig())
	req := httptest.NewRequest("GET", "/", nil)
	req.Header.Set("X-Forwarded-Ssl", "off")

	proto, _, _ := r.GetURLVars(req)
	if proto == "https" {
		t.Errorf("proto = %q with X-Forwarded-Ssl: off, want http", proto)
	}
}

// X-Url-Scheme is the third priority after X-Forwarded-Proto and X-Forwarded-Ssl.
func TestResolveProtoXUrlScheme(t *testing.T) {
	r := NewURLResolver(DefaultURLVarsConfig())
	req := httptest.NewRequest("GET", "/", nil)
	req.Header.Set("X-Url-Scheme", "https")

	proto, _, _ := r.GetURLVars(req)
	if proto != "https" {
		t.Errorf("proto = %q, want https", proto)
	}
}

// req.TLS != nil must resolve to "https" when no proxy headers are present.
func TestResolveProtoTLSConnection(t *testing.T) {
	r := NewURLResolver(DefaultURLVarsConfig())
	req := httptest.NewRequest("GET", "/", nil)
	req.TLS = &tls.ConnectionState{}
	req.Host = "example.com"

	proto, _, _ := r.GetURLVars(req)
	if proto != "https" {
		t.Errorf("proto = %q with req.TLS set, want https", proto)
	}
}

// No proxy headers and no TLS must resolve to the default "http".
func TestResolveProtoDefaultHttp(t *testing.T) {
	r := NewURLResolver(DefaultURLVarsConfig())
	req := httptest.NewRequest("GET", "/", nil)

	proto, _, _ := r.GetURLVars(req)
	if proto != "http" {
		t.Errorf("proto = %q, want http", proto)
	}
}

// X-Forwarded-Proto must beat X-Forwarded-Ssl when both are set.
func TestResolveProtoXForwardedProtoBeatsXForwardedSsl(t *testing.T) {
	r := NewURLResolver(DefaultURLVarsConfig())
	req := httptest.NewRequest("GET", "/", nil)
	req.Header.Set("X-Forwarded-Proto", "http")
	req.Header.Set("X-Forwarded-Ssl", "on")

	proto, _, _ := r.GetURLVars(req)
	if proto != "http" {
		t.Errorf("proto = %q; X-Forwarded-Proto must win over X-Forwarded-Ssl", proto)
	}
}

// --- resolveFQDN (via GetURLVars) ---

// X-Forwarded-Host without a port must be returned verbatim.
func TestResolveFQDNXForwardedHost(t *testing.T) {
	r := NewURLResolver(DefaultURLVarsConfig())
	req := httptest.NewRequest("GET", "/", nil)
	req.Header.Set("X-Forwarded-Host", "example.com")

	_, fqdn, _ := r.GetURLVars(req)
	if fqdn != "example.com" {
		t.Errorf("fqdn = %q, want example.com", fqdn)
	}
}

// X-Forwarded-Host with a port must have the port stripped.
func TestResolveFQDNXForwardedHostStripsPort(t *testing.T) {
	r := NewURLResolver(DefaultURLVarsConfig())
	req := httptest.NewRequest("GET", "/", nil)
	req.Header.Set("X-Forwarded-Host", "example.com:8080")

	_, fqdn, _ := r.GetURLVars(req)
	if fqdn != "example.com" {
		t.Errorf("fqdn = %q with port in X-Forwarded-Host, want example.com", fqdn)
	}
}

// X-Real-Host is used when X-Forwarded-Host is absent.
func TestResolveFQDNXRealHost(t *testing.T) {
	r := NewURLResolver(DefaultURLVarsConfig())
	req := httptest.NewRequest("GET", "/", nil)
	req.Header.Set("X-Real-Host", "real.example.com")

	_, fqdn, _ := r.GetURLVars(req)
	if fqdn != "real.example.com" {
		t.Errorf("fqdn = %q, want real.example.com", fqdn)
	}
}

// X-Original-Host is used when neither X-Forwarded-Host nor X-Real-Host is set.
func TestResolveFQDNXOriginalHost(t *testing.T) {
	r := NewURLResolver(DefaultURLVarsConfig())
	req := httptest.NewRequest("GET", "/", nil)
	req.Header.Set("X-Original-Host", "original.example.com")

	_, fqdn, _ := r.GetURLVars(req)
	if fqdn != "original.example.com" {
		t.Errorf("fqdn = %q, want original.example.com", fqdn)
	}
}

// DOMAIN env var must supply the FQDN when no proxy headers are present.
func TestResolveFQDNDomainEnvVar(t *testing.T) {
	t.Setenv("DOMAIN", "env.example.com")

	r := NewURLResolver(DefaultURLVarsConfig())
	req := httptest.NewRequest("GET", "/", nil)

	_, fqdn, _ := r.GetURLVars(req)
	if fqdn != "env.example.com" {
		t.Errorf("fqdn = %q, want env.example.com", fqdn)
	}
}

// DOMAIN env var with multiple comma-separated entries must return only the first,
// with leading/trailing whitespace trimmed.
func TestResolveFQDNDomainEnvVarMultipleUsesFirst(t *testing.T) {
	t.Setenv("DOMAIN", "  first.example.com , second.example.com")

	r := NewURLResolver(DefaultURLVarsConfig())
	req := httptest.NewRequest("GET", "/", nil)

	_, fqdn, _ := r.GetURLVars(req)
	if fqdn != "first.example.com" {
		t.Errorf("fqdn = %q, want first.example.com", fqdn)
	}
}

// X-Forwarded-Host must beat the DOMAIN env var.
func TestResolveFQDNHeaderBeatsDomainEnv(t *testing.T) {
	t.Setenv("DOMAIN", "env.example.com")

	r := NewURLResolver(DefaultURLVarsConfig())
	req := httptest.NewRequest("GET", "/", nil)
	req.Header.Set("X-Forwarded-Host", "header.example.com")

	_, fqdn, _ := r.GetURLVars(req)
	if fqdn != "header.example.com" {
		t.Errorf("fqdn = %q; X-Forwarded-Host must beat DOMAIN env var", fqdn)
	}
}

// When no proxy headers and no DOMAIN env var are set the result must be
// non-empty (hostname, public IP, or localhost).
func TestResolveFQDNFallbackNonEmpty(t *testing.T) {
	t.Setenv("DOMAIN", "")

	r := NewURLResolver(DefaultURLVarsConfig())
	req := httptest.NewRequest("GET", "/", nil)

	_, fqdn, _ := r.GetURLVars(req)
	if fqdn == "" {
		t.Error("fqdn is empty string; expected at least 'localhost'")
	}
}

// --- resolvePort (via GetURLVars) ---

// X-Forwarded-Port header must be used as the port.
func TestResolvePortXForwardedPort(t *testing.T) {
	r := NewURLResolver(DefaultURLVarsConfig())
	req := httptest.NewRequest("GET", "/", nil)
	req.Header.Set("X-Forwarded-Host", "example.com")
	req.Header.Set("X-Forwarded-Port", "8080")

	_, _, port := r.GetURLVars(req)
	if port != "8080" {
		t.Errorf("port = %q, want 8080", port)
	}
}

// Port 80 must be stripped (returned as empty string).
func TestResolvePortStrips80(t *testing.T) {
	r := NewURLResolver(DefaultURLVarsConfig())
	req := httptest.NewRequest("GET", "/", nil)
	req.Header.Set("X-Forwarded-Host", "example.com")
	req.Header.Set("X-Forwarded-Port", "80")

	_, _, port := r.GetURLVars(req)
	if port != "" {
		t.Errorf("port = %q for port 80, want empty string", port)
	}
}

// Port 443 must be stripped (returned as empty string).
func TestResolvePortStrips443(t *testing.T) {
	r := NewURLResolver(DefaultURLVarsConfig())
	req := httptest.NewRequest("GET", "/", nil)
	req.Header.Set("X-Forwarded-Host", "example.com")
	req.Header.Set("X-Forwarded-Port", "443")

	_, _, port := r.GetURLVars(req)
	if port != "" {
		t.Errorf("port = %q for port 443, want empty string", port)
	}
}

// A non-standard port embedded in the Host header must be extracted.
func TestResolvePortFromHostHeader(t *testing.T) {
	r := NewURLResolver(DefaultURLVarsConfig())
	req := httptest.NewRequest("GET", "/", nil)
	req.Host = "example.com:9090"

	_, _, port := r.GetURLVars(req)
	if port != "9090" {
		t.Errorf("port = %q from Host header, want 9090", port)
	}
}

// When Host header contains port 80 that port must be stripped.
func TestResolvePortFromHostHeaderStrips80(t *testing.T) {
	r := NewURLResolver(DefaultURLVarsConfig())
	req := httptest.NewRequest("GET", "/", nil)
	req.Host = "example.com:80"

	_, _, port := r.GetURLVars(req)
	if port != "" {
		t.Errorf("port = %q from Host header with :80, want empty string", port)
	}
}

// --- extractBaseDomain ---

// Three-part hostname must strip the leading subdomain.
func TestExtractBaseDomainThreeParts(t *testing.T) {
	got := extractBaseDomain("www.example.com")
	if got != "example.com" {
		t.Errorf("extractBaseDomain(www.example.com) = %q, want example.com", got)
	}
}

// Two-part hostname must be returned unchanged.
func TestExtractBaseDomainTwoParts(t *testing.T) {
	got := extractBaseDomain("example.com")
	if got != "example.com" {
		t.Errorf("extractBaseDomain(example.com) = %q, want example.com", got)
	}
}

// Single label (no dots) must be returned unchanged.
func TestExtractBaseDomainSingleLabel(t *testing.T) {
	got := extractBaseDomain("localhost")
	if got != "localhost" {
		t.Errorf("extractBaseDomain(localhost) = %q, want localhost", got)
	}
}

// Four-part hostname returns the last two parts only.
func TestExtractBaseDomainFourParts(t *testing.T) {
	got := extractBaseDomain("sub.example.co.uk")
	if got != "co.uk" {
		t.Errorf("extractBaseDomain(sub.example.co.uk) = %q, want co.uk", got)
	}
}

// Empty string input must not panic and returns empty string.
func TestExtractBaseDomainEmpty(t *testing.T) {
	got := extractBaseDomain("")
	if got != "" {
		t.Errorf("extractBaseDomain('') = %q, want empty string", got)
	}
}

// --- BuildURL ---

// Non-standard port must appear in the URL.
func TestBuildURLWithPort(t *testing.T) {
	r := NewURLResolver(DefaultURLVarsConfig())
	req := httptest.NewRequest("GET", "/path", nil)
	req.Header.Set("X-Forwarded-Proto", "http")
	req.Header.Set("X-Forwarded-Host", "example.com")
	req.Header.Set("X-Forwarded-Port", "8080")

	got := r.BuildURL(req, "/path")
	want := "http://example.com:8080/path"
	if got != want {
		t.Errorf("BuildURL = %q, want %q", got, want)
	}
}

// Port 80 must be omitted from the built URL.
func TestBuildURLPort80Omitted(t *testing.T) {
	r := NewURLResolver(DefaultURLVarsConfig())
	req := httptest.NewRequest("GET", "/page", nil)
	req.Header.Set("X-Forwarded-Proto", "http")
	req.Header.Set("X-Forwarded-Host", "example.com")
	req.Header.Set("X-Forwarded-Port", "80")

	got := r.BuildURL(req, "/page")
	want := "http://example.com/page"
	if got != want {
		t.Errorf("BuildURL (port 80) = %q, want %q", got, want)
	}
}

// Port 443 must be omitted from the built URL.
func TestBuildURLPort443Omitted(t *testing.T) {
	r := NewURLResolver(DefaultURLVarsConfig())
	req := httptest.NewRequest("GET", "/secure", nil)
	req.Header.Set("X-Forwarded-Proto", "https")
	req.Header.Set("X-Forwarded-Host", "example.com")
	req.Header.Set("X-Forwarded-Port", "443")

	got := r.BuildURL(req, "/secure")
	want := "https://example.com/secure"
	if got != want {
		t.Errorf("BuildURL (port 443) = %q, want %q", got, want)
	}
}

// An empty path must produce a URL ending at the FQDN.
func TestBuildURLEmptyPath(t *testing.T) {
	r := NewURLResolver(DefaultURLVarsConfig())
	req := httptest.NewRequest("GET", "/", nil)
	req.Header.Set("X-Forwarded-Proto", "http")
	req.Header.Set("X-Forwarded-Host", "example.com")
	req.Header.Set("X-Forwarded-Port", "80")

	got := r.BuildURL(req, "")
	want := "http://example.com"
	if got != want {
		t.Errorf("BuildURL (empty path) = %q, want %q", got, want)
	}
}

// --- GetBaseDomain / GetWildcardDomain ---

// A freshly created resolver must return empty strings for both.
func TestGetBaseDomainInitiallyEmpty(t *testing.T) {
	r := NewURLResolver(DefaultURLVarsConfig())
	if got := r.GetBaseDomain(); got != "" {
		t.Errorf("GetBaseDomain() initially = %q, want empty string", got)
	}
	if got := r.GetWildcardDomain(); got != "" {
		t.Errorf("GetWildcardDomain() initially = %q, want empty string", got)
	}
}

// After recording enough distinct subdomains of the same base the wildcard
// must be inferred and the base domain set.
func TestGetBaseDomainAndWildcardAfterLearning(t *testing.T) {
	// MinSamples=1 so patterns are inferred after the first unique domain.
	// We need at least 2 distinct subdomains to trigger wildcard detection.
	cfg := DefaultURLVarsConfig()
	cfg.MinSamples = 1
	r := NewURLResolver(cfg)

	// Record three distinct subdomains under the same base.
	r.recordObservation("api.example.com")
	r.recordObservation("www.example.com")
	r.recordObservation("mail.example.com")

	base := r.GetBaseDomain()
	if base != "example.com" {
		t.Errorf("GetBaseDomain() = %q after learning, want example.com", base)
	}

	wildcard := r.GetWildcardDomain()
	if wildcard != "*.example.com" {
		t.Errorf("GetWildcardDomain() = %q after learning, want *.example.com", wildcard)
	}
}

// Fewer than two subdomains of the same base must not produce a wildcard.
func TestGetWildcardDomainNotSetWithOneSubdomain(t *testing.T) {
	cfg := DefaultURLVarsConfig()
	cfg.MinSamples = 1
	r := NewURLResolver(cfg)

	r.recordObservation("api.example.com")

	wildcard := r.GetWildcardDomain()
	if wildcard != "" {
		t.Errorf("GetWildcardDomain() = %q with only one subdomain, want empty string", wildcard)
	}
}

// --- GetAllDomains ---

// Empty DOMAIN env var must return nil.
func TestGetAllDomainsEmptyEnv(t *testing.T) {
	t.Setenv("DOMAIN", "")
	got := GetAllDomains()
	if got != nil {
		t.Errorf("GetAllDomains() = %v, want nil", got)
	}
}

// A single domain must return a one-element slice.
func TestGetAllDomainsSingle(t *testing.T) {
	t.Setenv("DOMAIN", "a.com")
	got := GetAllDomains()
	if len(got) != 1 || got[0] != "a.com" {
		t.Errorf("GetAllDomains() = %v, want [a.com]", got)
	}
}

// Multiple comma-separated domains must all be returned.
func TestGetAllDomainsMultiple(t *testing.T) {
	t.Setenv("DOMAIN", "a.com,b.com")
	got := GetAllDomains()
	if len(got) != 2 || got[0] != "a.com" || got[1] != "b.com" {
		t.Errorf("GetAllDomains() = %v, want [a.com b.com]", got)
	}
}

// Whitespace around domain entries must be trimmed.
func TestGetAllDomainsTrimsWhitespace(t *testing.T) {
	t.Setenv("DOMAIN", "  a.com , b.com  ")
	got := GetAllDomains()
	if len(got) != 2 || got[0] != "a.com" || got[1] != "b.com" {
		t.Errorf("GetAllDomains() = %v, want [a.com b.com]", got)
	}
}

// An entry that is only whitespace must be skipped.
func TestGetAllDomainsSkipsBlankEntries(t *testing.T) {
	t.Setenv("DOMAIN", "a.com,   ,b.com")
	got := GetAllDomains()
	if len(got) != 2 {
		t.Errorf("GetAllDomains() = %v, want exactly 2 entries", got)
	}
}

// --- Middleware ---

// The middleware must set X-Resolved-Proto and X-Resolved-Host on the request.
func TestMiddlewareSetsResolvedProtoAndHost(t *testing.T) {
	r := NewURLResolver(DefaultURLVarsConfig())

	var capturedProto, capturedHost string
	inner := http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		capturedProto = req.Header.Get("X-Resolved-Proto")
		capturedHost = req.Header.Get("X-Resolved-Host")
	})

	handler := r.Middleware(inner)
	req := httptest.NewRequest("GET", "/", nil)
	req.Header.Set("X-Forwarded-Proto", "https")
	req.Header.Set("X-Forwarded-Host", "middleware.example.com")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if capturedProto != "https" {
		t.Errorf("X-Resolved-Proto = %q, want https", capturedProto)
	}
	if capturedHost != "middleware.example.com" {
		t.Errorf("X-Resolved-Host = %q, want middleware.example.com", capturedHost)
	}
}

// The middleware must set X-Resolved-BaseURL.
func TestMiddlewareSetsBaseURL(t *testing.T) {
	r := NewURLResolver(DefaultURLVarsConfig())

	var capturedBaseURL string
	inner := http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		capturedBaseURL = req.Header.Get("X-Resolved-BaseURL")
	})

	handler := r.Middleware(inner)
	req := httptest.NewRequest("GET", "/", nil)
	req.Header.Set("X-Forwarded-Proto", "http")
	req.Header.Set("X-Forwarded-Host", "example.com")
	req.Header.Set("X-Forwarded-Port", "80")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if capturedBaseURL != "http://example.com" {
		t.Errorf("X-Resolved-BaseURL = %q, want http://example.com", capturedBaseURL)
	}
}

// The middleware must set X-Resolved-Port when the port is non-standard.
func TestMiddlewareSetsPortWhenNonStandard(t *testing.T) {
	r := NewURLResolver(DefaultURLVarsConfig())

	var capturedPort string
	inner := http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		capturedPort = req.Header.Get("X-Resolved-Port")
	})

	handler := r.Middleware(inner)
	req := httptest.NewRequest("GET", "/", nil)
	req.Header.Set("X-Forwarded-Proto", "http")
	req.Header.Set("X-Forwarded-Host", "example.com")
	req.Header.Set("X-Forwarded-Port", "8080")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if capturedPort != "8080" {
		t.Errorf("X-Resolved-Port = %q, want 8080", capturedPort)
	}
}

// The middleware must NOT set X-Resolved-Port for port 80.
func TestMiddlewareOmitsPortFor80(t *testing.T) {
	r := NewURLResolver(DefaultURLVarsConfig())

	var capturedPort string
	inner := http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		capturedPort = req.Header.Get("X-Resolved-Port")
	})

	handler := r.Middleware(inner)
	req := httptest.NewRequest("GET", "/", nil)
	req.Header.Set("X-Forwarded-Proto", "http")
	req.Header.Set("X-Forwarded-Host", "example.com")
	req.Header.Set("X-Forwarded-Port", "80")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if capturedPort != "" {
		t.Errorf("X-Resolved-Port = %q for port 80, want empty string", capturedPort)
	}
}

// The middleware must call the next handler.
func TestMiddlewareCallsNext(t *testing.T) {
	r := NewURLResolver(DefaultURLVarsConfig())

	called := false
	inner := http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		called = true
		w.WriteHeader(http.StatusOK)
	})

	handler := r.Middleware(inner)
	req := httptest.NewRequest("GET", "/", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if !called {
		t.Error("Middleware did not call the next handler")
	}
}

// The X-Resolved-BaseURL must include the port when the port is non-standard.
func TestMiddlewareBaseURLIncludesNonStandardPort(t *testing.T) {
	r := NewURLResolver(DefaultURLVarsConfig())

	var capturedBaseURL string
	inner := http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		capturedBaseURL = req.Header.Get("X-Resolved-BaseURL")
	})

	handler := r.Middleware(inner)
	req := httptest.NewRequest("GET", "/", nil)
	req.Header.Set("X-Forwarded-Proto", "http")
	req.Header.Set("X-Forwarded-Host", "example.com")
	req.Header.Set("X-Forwarded-Port", "9000")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	want := "http://example.com:9000"
	if capturedBaseURL != want {
		t.Errorf("X-Resolved-BaseURL = %q, want %q", capturedBaseURL, want)
	}
}

// --- GlobalResolver ---

// GlobalResolver must return a non-nil resolver.
func TestGlobalResolverNonNil(t *testing.T) {
	if GlobalResolver() == nil {
		t.Error("GlobalResolver() returned nil")
	}
}

// GlobalResolver must return the same instance on every call (singleton).
func TestGlobalResolverSameInstance(t *testing.T) {
	first := GlobalResolver()
	second := GlobalResolver()
	if first != second {
		t.Error("GlobalResolver() returned different instances on successive calls")
	}
}

// --- Package-level convenience functions ---

// Package-level GetURLVars must delegate to GlobalResolver and return sensible values.
func TestPackageGetURLVars(t *testing.T) {
	req := httptest.NewRequest("GET", "/", nil)
	req.Header.Set("X-Forwarded-Proto", "https")
	req.Header.Set("X-Forwarded-Host", "pkg.example.com")
	req.Header.Set("X-Forwarded-Port", "443")

	proto, fqdn, port := GetURLVars(req)
	if proto != "https" {
		t.Errorf("GetURLVars proto = %q, want https", proto)
	}
	if fqdn != "pkg.example.com" {
		t.Errorf("GetURLVars fqdn = %q, want pkg.example.com", fqdn)
	}
	if port != "" {
		t.Errorf("GetURLVars port = %q for port 443, want empty string", port)
	}
}

// Package-level BuildURL must delegate to GlobalResolver.
func TestPackageBuildURL(t *testing.T) {
	req := httptest.NewRequest("GET", "/", nil)
	req.Header.Set("X-Forwarded-Proto", "http")
	req.Header.Set("X-Forwarded-Host", "pkg.example.com")
	req.Header.Set("X-Forwarded-Port", "80")

	got := BuildURL(req, "/test")
	want := "http://pkg.example.com/test"
	if got != want {
		t.Errorf("BuildURL = %q, want %q", got, want)
	}
}

// Package-level GetBaseDomain must not panic and must return a string.
func TestPackageGetBaseDomain(t *testing.T) {
	// Just ensure the function delegates without panicking; the value depends
	// on prior observations recorded against the global resolver.
	_ = GetBaseDomain()
}

// Package-level GetWildcardDomain must not panic and must return a string.
func TestPackageGetWildcardDomain(t *testing.T) {
	_ = GetWildcardDomain()
}

// --- Learning / recordObservation idempotency ---

// Calling GetURLVars multiple times with the same FQDN must not panic.
func TestGetURLVarsIdempotent(t *testing.T) {
	r := NewURLResolver(DefaultURLVarsConfig())
	req := httptest.NewRequest("GET", "/", nil)
	req.Header.Set("X-Forwarded-Host", "idempotent.example.com")

	for i := 0; i < 10; i++ {
		proto, fqdn, _ := r.GetURLVars(req)
		if proto == "" {
			t.Errorf("call %d: proto is empty", i)
		}
		if fqdn != "idempotent.example.com" {
			t.Errorf("call %d: fqdn = %q, want idempotent.example.com", i, fqdn)
		}
	}
}

// "localhost" must not be recorded as an observation (privacy / noise reduction).
func TestLocalhostNotRecorded(t *testing.T) {
	cfg := DefaultURLVarsConfig()
	cfg.MinSamples = 1
	r := NewURLResolver(cfg)

	// Drive enough requests that localhost would trigger pattern inference if
	// it were being recorded.
	for i := 0; i < 5; i++ {
		req := httptest.NewRequest("GET", "/", nil)
		r.GetURLVars(req)
	}

	r.mu.RLock()
	_, found := r.observations["localhost"]
	r.mu.RUnlock()

	if found {
		t.Error("'localhost' was recorded as an observation; it must be excluded")
	}
}

// --- Learning disabled ---

// Recording the same domain twice must increment the count, not duplicate the entry.
func TestRecordObservationDuplicateIncrementsCount(t *testing.T) {
	cfg := DefaultURLVarsConfig()
	cfg.MinSamples = 10
	r := NewURLResolver(cfg)

	r.recordObservation("repeat.example.com")
	r.recordObservation("repeat.example.com")

	r.mu.RLock()
	obs, ok := r.observations["repeat.example.com"]
	r.mu.RUnlock()

	if !ok {
		t.Fatal("observation for repeat.example.com not found")
	}
	if obs.count != 2 {
		t.Errorf("obs.count = %d after two recordings, want 2", obs.count)
	}
}

// When Learning is disabled no observations must be stored.
func TestLearningDisabledSkipsObservations(t *testing.T) {
	cfg := DefaultURLVarsConfig()
	cfg.Learning = false
	r := NewURLResolver(cfg)

	req := httptest.NewRequest("GET", "/", nil)
	req.Header.Set("X-Forwarded-Host", "learn.example.com")

	for i := 0; i < 5; i++ {
		r.GetURLVars(req)
	}

	r.mu.RLock()
	count := len(r.observations)
	r.mu.RUnlock()

	if count != 0 {
		t.Errorf("observations len = %d with Learning=false, want 0", count)
	}
}

// ── resolveFQDN — port-stripping paths for X-Forwarded-Host etc. ─────────────

func TestResolveFQDN_XForwardedHostWithPort_StripsPort(t *testing.T) {
	resolver := NewURLResolver(DefaultURLVarsConfig())
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("X-Forwarded-Host", "example.com:8443")

	fqdn := resolver.resolveFQDN(req)
	if fqdn != "example.com" {
		t.Errorf("resolveFQDN(X-Forwarded-Host:port): got %q, want example.com", fqdn)
	}
}

func TestResolveFQDN_XRealHostWithPort_StripsPort(t *testing.T) {
	resolver := NewURLResolver(DefaultURLVarsConfig())
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("X-Real-Host", "real.example.com:9000")

	fqdn := resolver.resolveFQDN(req)
	if fqdn != "real.example.com" {
		t.Errorf("resolveFQDN(X-Real-Host:port): got %q, want real.example.com", fqdn)
	}
}

func TestResolveFQDN_XOriginalHostWithPort_StripsPort(t *testing.T) {
	resolver := NewURLResolver(DefaultURLVarsConfig())
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("X-Original-Host", "orig.example.com:7777")

	fqdn := resolver.resolveFQDN(req)
	if fqdn != "orig.example.com" {
		t.Errorf("resolveFQDN(X-Original-Host:port): got %q, want orig.example.com", fqdn)
	}
}

// ── getPublicIP — basic smoke test ────────────────────────────────────────────

func TestGetPublicIP_ReturnsStringOrEmpty(t *testing.T) {
	ip := getPublicIP()
	// May be empty in Docker/CI; just verify it doesn't panic
	_ = ip
}
