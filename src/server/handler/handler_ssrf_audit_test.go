// SPDX-License-Identifier: MIT
// Tests for SSRF hardening helpers added during the project audit.
package handler

import (
	"context"
	"net"
	"net/http"
	"net/http/httptest"
	"testing"
)

// TestIsPrivateIP verifies the CIDR-based private/reserved address check.
func TestIsPrivateIP(t *testing.T) {
	cases := []struct {
		ip   string
		want bool
	}{
		{"127.0.0.1", true},
		{"10.1.2.3", true},
		{"172.16.5.5", true},
		{"192.168.1.1", true},
		{"169.254.1.1", true},
		{"::1", true},
		{"fc00::1", true},
		{"fe80::1", true},
		{"8.8.8.8", false},
		{"1.1.1.1", false},
		{"93.184.216.34", false},
	}
	for _, c := range cases {
		ip := net.ParseIP(c.ip)
		if got := isPrivateIP(ip); got != c.want {
			t.Errorf("isPrivateIP(%s) = %v, want %v", c.ip, got, c.want)
		}
	}
	// nil IP must be treated as private (fail closed).
	if !isPrivateIP(nil) {
		t.Error("isPrivateIP(nil) = false, want true (fail closed)")
	}
}

// TestSSRFCheckRedirect verifies redirect-time SSRF protection.
func TestSSRFCheckRedirect(t *testing.T) {
	mkReq := func(rawurl string) *http.Request {
		req, err := http.NewRequest(http.MethodGet, rawurl, nil)
		if err != nil {
			t.Fatalf("NewRequest(%s): %v", rawurl, err)
		}
		return req
	}

	// Public host, first hop: allowed.
	if err := ssrfCheckRedirect(mkReq("https://example.com/a"), nil); err != nil {
		t.Errorf("public redirect blocked: %v", err)
	}

	// Private host: blocked.
	if err := ssrfCheckRedirect(mkReq("http://127.0.0.1/a"), nil); err == nil {
		t.Error("redirect to loopback should be blocked")
	}

	// Unsupported scheme: blocked.
	if err := ssrfCheckRedirect(mkReq("file:///etc/passwd"), nil); err == nil {
		t.Error("redirect to file:// should be blocked")
	}

	// Too many redirects: blocked.
	via := make([]*http.Request, 5)
	if err := ssrfCheckRedirect(mkReq("https://example.com/a"), via); err == nil {
		t.Error("redirect chain of 5 should be blocked")
	}
}

// TestGetAPIResponseFormatOriginalPath verifies .txt detection uses the
// pre-strip original path stored in the request context.
func TestGetAPIResponseFormatOriginalPath(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/api/v1/engines", nil)
	ctx := context.WithValue(req.Context(), OriginalPathKey, "/api/v1/engines.txt")
	req = req.WithContext(ctx)
	if got := getAPIResponseFormat(req); got != "text" {
		t.Errorf("getAPIResponseFormat with .txt original path = %q, want text", got)
	}
}
