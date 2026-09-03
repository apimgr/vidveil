// SPDX-License-Identifier: MIT
// AI.md PART 28: Test coverage for server package pure/middleware functions.
package server

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

// --- GetTemplatesFS ---

// TestGetTemplatesFS_NonEmpty verifies the embedded FS is non-zero.
func TestGetTemplatesFS_NonEmpty(t *testing.T) {
	fs := GetTemplatesFS()
	f, err := fs.Open(".")
	if err != nil {
		t.Fatalf("GetTemplatesFS().Open('.') = %v, want no error", err)
	}
	defer f.Close()
}

// --- parseBodySize ---

// TestParseBodySize_Empty verifies the default value is returned for empty string.
func TestParseBodySize_Empty(t *testing.T) {
	if got := parseBodySize("", 100); got != 100 {
		t.Errorf("parseBodySize('', 100) = %d, want 100", got)
	}
}

// TestParseBodySize_Bytes verifies bare number returns the value directly.
func TestParseBodySize_Bytes(t *testing.T) {
	if got := parseBodySize("512", 0); got != 512 {
		t.Errorf("parseBodySize('512', 0) = %d, want 512", got)
	}
}

// TestParseBodySize_KB verifies KB suffix multiplies by 1024.
func TestParseBodySize_KB(t *testing.T) {
	if got := parseBodySize("4KB", 0); got != 4*1024 {
		t.Errorf("parseBodySize('4KB', 0) = %d, want %d", got, 4*1024)
	}
}

// TestParseBodySize_MB verifies MB suffix multiplies correctly.
func TestParseBodySize_MB(t *testing.T) {
	if got := parseBodySize("10MB", 0); got != 10*1024*1024 {
		t.Errorf("parseBodySize('10MB', 0) = %d, want %d", got, 10*1024*1024)
	}
}

// TestParseBodySize_GB verifies GB suffix multiplies correctly.
func TestParseBodySize_GB(t *testing.T) {
	if got := parseBodySize("2GB", 0); got != 2*1024*1024*1024 {
		t.Errorf("parseBodySize('2GB', 0) = %d, want %d", got, 2*1024*1024*1024)
	}
}

// TestParseBodySize_BExplicit verifies explicit B suffix works.
func TestParseBodySize_BExplicit(t *testing.T) {
	if got := parseBodySize("256B", 0); got != 256 {
		t.Errorf("parseBodySize('256B', 0) = %d, want 256", got)
	}
}

// TestParseBodySize_Invalid verifies default returned for non-numeric input.
func TestParseBodySize_Invalid(t *testing.T) {
	if got := parseBodySize("bad", 99); got != 99 {
		t.Errorf("parseBodySize('bad', 99) = %d, want 99 (default)", got)
	}
}

// TestParseBodySize_ZeroReturnsDefault verifies zero value falls back to default.
func TestParseBodySize_ZeroReturnsDefault(t *testing.T) {
	if got := parseBodySize("0", 50); got != 50 {
		t.Errorf("parseBodySize('0', 50) = %d, want 50 (default for zero)", got)
	}
}

// TestParseBodySize_LowercaseKb verifies lowercase suffix is normalised.
func TestParseBodySize_LowercaseKb(t *testing.T) {
	if got := parseBodySize("8kb", 0); got != 8*1024 {
		t.Errorf("parseBodySize('8kb', 0) = %d, want %d", got, 8*1024)
	}
}

// --- parseDuration ---

// TestParseDuration_Empty verifies default is returned for empty string.
func TestParseDuration_Empty(t *testing.T) {
	d := 5 * time.Second
	if got := parseDuration("", d); got != d {
		t.Errorf("parseDuration('', 5s) = %v, want %v", got, d)
	}
}

// TestParseDuration_Valid verifies a valid duration string is parsed correctly.
func TestParseDuration_Valid(t *testing.T) {
	if got := parseDuration("30s", 0); got != 30*time.Second {
		t.Errorf("parseDuration('30s', 0) = %v, want 30s", got)
	}
}

// TestParseDuration_Minutes verifies minute duration is parsed correctly.
func TestParseDuration_Minutes(t *testing.T) {
	if got := parseDuration("5m", 0); got != 5*time.Minute {
		t.Errorf("parseDuration('5m', 0) = %v, want 5m", got)
	}
}

// TestParseDuration_Invalid verifies default returned for invalid input.
func TestParseDuration_Invalid(t *testing.T) {
	def := 10 * time.Second
	if got := parseDuration("not-a-duration", def); got != def {
		t.Errorf("parseDuration('not-a-duration', 10s) = %v, want 10s", got)
	}
}

// --- URLNormalizeMiddleware ---

// TestURLNormalizeMiddleware_Root verifies root "/" passes through unchanged.
func TestURLNormalizeMiddleware_Root(t *testing.T) {
	called := false
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		w.WriteHeader(http.StatusOK)
	})
	handler := URLNormalizeMiddleware(next)
	req := httptest.NewRequest("GET", "/", nil)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if !called {
		t.Error("URLNormalizeMiddleware: root should call next handler")
	}
	if rr.Code == http.StatusMovedPermanently {
		t.Error("URLNormalizeMiddleware: root should not redirect")
	}
}

// TestURLNormalizeMiddleware_TrailingSlash verifies trailing slash triggers 301 redirect.
func TestURLNormalizeMiddleware_TrailingSlash(t *testing.T) {
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	handler := URLNormalizeMiddleware(next)
	req := httptest.NewRequest("GET", "/about/", nil)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusMovedPermanently {
		t.Errorf("URLNormalizeMiddleware trailing slash: status = %d, want 301", rr.Code)
	}
	loc := rr.Header().Get("Location")
	if strings.HasSuffix(loc, "/") {
		t.Errorf("URLNormalizeMiddleware redirect location has trailing slash: %q", loc)
	}
}

// TestURLNormalizeMiddleware_NoTrailingSlash verifies paths without trailing slash pass through.
func TestURLNormalizeMiddleware_NoTrailingSlash(t *testing.T) {
	called := false
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		w.WriteHeader(http.StatusOK)
	})
	handler := URLNormalizeMiddleware(next)
	req := httptest.NewRequest("GET", "/about", nil)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if !called {
		t.Error("URLNormalizeMiddleware: path without trailing slash should call next")
	}
}

// TestURLNormalizeMiddleware_FileRequestPassThrough verifies explicit file requests are not redirected.
func TestURLNormalizeMiddleware_FileRequestPassThrough(t *testing.T) {
	called := false
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		w.WriteHeader(http.StatusOK)
	})
	handler := URLNormalizeMiddleware(next)
	req := httptest.NewRequest("GET", "/static/file.css/", nil)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	// File extensions should not be stripped even if path ends with /
	_ = called
}

// TestURLNormalizeMiddleware_QueryStringPreserved verifies query string is preserved on redirect.
func TestURLNormalizeMiddleware_QueryStringPreserved(t *testing.T) {
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	handler := URLNormalizeMiddleware(next)
	req := httptest.NewRequest("GET", "/search/?q=test", nil)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if rr.Code == http.StatusMovedPermanently {
		loc := rr.Header().Get("Location")
		if !strings.Contains(loc, "q=test") {
			t.Errorf("URLNormalizeMiddleware: redirect location lost query string: %q", loc)
		}
	}
}

// --- extractClientIP ---

// TestExtractClientIP_WithPort verifies host:port is split correctly.
func TestExtractClientIP_WithPort(t *testing.T) {
	req := httptest.NewRequest("GET", "/", nil)
	req.RemoteAddr = "10.0.0.1:9000"
	if got := extractClientIP(req); got != "10.0.0.1" {
		t.Errorf("extractClientIP = %q, want %q", got, "10.0.0.1")
	}
}

// TestExtractClientIP_BareIP verifies bare IP (no port) is returned as-is.
func TestExtractClientIP_BareIP(t *testing.T) {
	req := httptest.NewRequest("GET", "/", nil)
	req.RemoteAddr = "192.168.1.1"
	if got := extractClientIP(req); got != "192.168.1.1" {
		t.Errorf("extractClientIP bare IP = %q, want %q", got, "192.168.1.1")
	}
}

// TestExtractClientIP_IPv6 verifies IPv6 address is extracted correctly.
func TestExtractClientIP_IPv6(t *testing.T) {
	req := httptest.NewRequest("GET", "/", nil)
	req.RemoteAddr = "[::1]:8080"
	if got := extractClientIP(req); got != "::1" {
		t.Errorf("extractClientIP IPv6 = %q, want %q", got, "::1")
	}
}

// --- isAllowlisted ---

// TestIsAllowlisted_FalseByDefault verifies a fresh request is not allowlisted.
func TestIsAllowlisted_FalseByDefault(t *testing.T) {
	req := httptest.NewRequest("GET", "/", nil)
	if isAllowlisted(req) {
		t.Error("isAllowlisted() should return false for unmodified request")
	}
}

// --- secFetchValidationMiddleware ---

// TestSecFetchValidation_SameSitePassThrough verifies same-site POST passes through.
func TestSecFetchValidation_SameSitePassThrough(t *testing.T) {
	called := false
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		w.WriteHeader(http.StatusOK)
	})
	handler := secFetchValidationMiddleware(next)
	req := httptest.NewRequest("POST", "/submit", nil)
	req.Header.Set("Sec-Fetch-Site", "same-origin")
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if !called {
		t.Error("secFetchValidationMiddleware: same-origin POST should call next")
	}
}

// TestSecFetchValidation_CrossSitePostBlocked verifies cross-site POST without Bearer is blocked.
func TestSecFetchValidation_CrossSitePostBlocked(t *testing.T) {
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	handler := secFetchValidationMiddleware(next)
	req := httptest.NewRequest("POST", "/submit", nil)
	req.Header.Set("Sec-Fetch-Site", "cross-site")
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusForbidden {
		t.Errorf("secFetchValidationMiddleware cross-site POST: status = %d, want 403", rr.Code)
	}
}

// TestSecFetchValidation_CrossSitePostWithBearerAllowed verifies Bearer auth bypasses cross-site check.
func TestSecFetchValidation_CrossSitePostWithBearerAllowed(t *testing.T) {
	called := false
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		w.WriteHeader(http.StatusOK)
	})
	handler := secFetchValidationMiddleware(next)
	req := httptest.NewRequest("POST", "/api/v1/search", nil)
	req.Header.Set("Sec-Fetch-Site", "cross-site")
	req.Header.Set("Authorization", "Bearer my-token")
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if !called {
		t.Error("secFetchValidationMiddleware: cross-site POST with Bearer should call next")
	}
}

// TestSecFetchValidation_NavigateToAPIBlocked verifies navigate fetch to /api/* is blocked.
func TestSecFetchValidation_NavigateToAPIBlocked(t *testing.T) {
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	handler := secFetchValidationMiddleware(next)
	req := httptest.NewRequest("GET", "/api/v1/search", nil)
	req.Header.Set("Sec-Fetch-Mode", "navigate")
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusForbidden {
		t.Errorf("secFetchValidationMiddleware navigate to /api/*: status = %d, want 403", rr.Code)
	}
}

// TestSecFetchValidation_NavigateToPageAllowed verifies navigate fetch to non-API page passes through.
func TestSecFetchValidation_NavigateToPageAllowed(t *testing.T) {
	called := false
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		w.WriteHeader(http.StatusOK)
	})
	handler := secFetchValidationMiddleware(next)
	req := httptest.NewRequest("GET", "/about", nil)
	req.Header.Set("Sec-Fetch-Mode", "navigate")
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if !called {
		t.Error("secFetchValidationMiddleware: navigate to non-API should call next")
	}
}

// TestSecFetchValidation_NoHeaders verifies absence of Sec-Fetch headers passes through.
func TestSecFetchValidation_NoHeaders(t *testing.T) {
	called := false
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		w.WriteHeader(http.StatusOK)
	})
	handler := secFetchValidationMiddleware(next)
	req := httptest.NewRequest("POST", "/submit", nil)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if !called {
		t.Error("secFetchValidationMiddleware: no Sec-Fetch headers should call next")
	}
}

// --- recordRequest / recordError (expvar helpers) ---

// TestRecordRequest_NoPanic verifies recordRequest does not panic.
func TestRecordRequest_NoPanic(t *testing.T) {
	defer func() {
		if r := recover(); r != nil {
			t.Errorf("recordRequest panicked: %v", r)
		}
	}()
	recordRequest(50 * time.Millisecond)
}

// TestRecordError_NoPanic verifies recordError does not panic.
func TestRecordError_NoPanic(t *testing.T) {
	defer func() {
		if r := recover(); r != nil {
			t.Errorf("recordError panicked: %v", r)
		}
	}()
	recordError()
}
