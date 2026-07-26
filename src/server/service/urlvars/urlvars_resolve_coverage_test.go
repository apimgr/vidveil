// SPDX-License-Identifier: MIT
// AI.md PART 12: deterministic coverage for the trusted-proxy header priority
// branches of resolvePathPrefix and resolveFQDN. A loopback RemoteAddr is a
// trusted peer, so the proxy-header branches are reached without any network.
package urlvars

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/apimgr/vidveil/src/config"
)

func newResolver(t *testing.T) *URLResolver {
	t.Helper()
	r := NewURLResolver(URLVarsConfig{})
	r.SetAppConfig(config.DefaultAppConfig())
	return r
}

func trustedReq(method, target string) *http.Request {
	req := httptest.NewRequest(method, target, nil)
	req.RemoteAddr = "127.0.0.1:5555" // loopback => trusted proxy
	return req
}

// TestResolvePathPrefix covers each trusted-proxy header, the config BaseURL
// branch, and the "/" default.
func TestResolvePathPrefix(t *testing.T) {
	cases := []struct {
		name   string
		header string
		value  string
		want   string
	}{
		{"forwarded-prefix", "X-Forwarded-Prefix", "/app/", "/app"},
		{"forwarded-path", "X-Forwarded-Path", "sub", "/sub"},
		{"script-name", "X-Script-Name", "/wsgi", "/wsgi"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			r := newResolver(t)
			req := trustedReq(http.MethodGet, "/")
			req.Header.Set(tc.header, tc.value)
			if got := r.resolvePathPrefix(req); got != tc.want {
				t.Errorf("%s: got %q want %q", tc.name, got, tc.want)
			}
		})
	}

	// Config BaseURL branch (no proxy headers).
	r := newResolver(t)
	r.appCfg.Server.BaseURL = "/base"
	if got := r.resolvePathPrefix(trustedReq(http.MethodGet, "/")); got != "/base" {
		t.Errorf("config BaseURL: got %q want /base", got)
	}

	// Default branch: no headers, no BaseURL, no BASEURL env.
	r2 := newResolver(t)
	r2.appCfg.Server.BaseURL = ""
	if got := r2.resolvePathPrefix(trustedReq(http.MethodGet, "/")); got != "/" {
		t.Errorf("default: got %q want /", got)
	}
}

// TestResolveFQDN covers the Tor priority, the trusted-proxy host headers
// (with and without a port), and returns a non-empty value on the fallthrough.
func TestResolveFQDN(t *testing.T) {
	// Tor priority 0.
	rTor := newResolver(t)
	rTor.appCfg.Server.Tor.OnionAddress = "abcdef.onion"
	reqTor := trustedReq(http.MethodGet, "/")
	reqTor.Host = "abcdef.onion"
	if got := rTor.resolveFQDN(reqTor); got != "abcdef.onion" {
		t.Errorf("tor: got %q want abcdef.onion", got)
	}

	// X-Forwarded-Host with a port is stripped to the host.
	r := newResolver(t)
	req := trustedReq(http.MethodGet, "/")
	req.Header.Set("X-Forwarded-Host", "proxy.example.com:8443")
	if got := r.resolveFQDN(req); got != "proxy.example.com" {
		t.Errorf("x-forwarded-host: got %q want proxy.example.com", got)
	}

	// X-Real-Host without a port is returned verbatim.
	r2 := newResolver(t)
	req2 := trustedReq(http.MethodGet, "/")
	req2.Header.Set("X-Real-Host", "real.example.com")
	if got := r2.resolveFQDN(req2); got != "real.example.com" {
		t.Errorf("x-real-host: got %q want real.example.com", got)
	}

	// X-Original-Host branch.
	r3 := newResolver(t)
	req3 := trustedReq(http.MethodGet, "/")
	req3.Header.Set("X-Original-Host", "orig.example.com")
	if got := r3.resolveFQDN(req3); got != "orig.example.com" {
		t.Errorf("x-original-host: got %q want orig.example.com", got)
	}

	// Fallthrough (no proxy headers, untrusted peer): must be non-empty.
	r4 := newResolver(t)
	req4 := httptest.NewRequest(http.MethodGet, "/", nil)
	req4.RemoteAddr = "8.8.8.8:1234" // not trusted
	if got := r4.resolveFQDN(req4); got == "" {
		t.Error("fallthrough resolveFQDN returned empty")
	}
}
