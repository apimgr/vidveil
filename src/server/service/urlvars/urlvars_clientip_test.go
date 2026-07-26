// SPDX-License-Identifier: MIT
// AI.md PART 12 "Client IP Detection": deterministic coverage for the client-IP
// resolution priority chain and the trusted-proxy gate that guards it.
package urlvars

import (
	"net/http"
	"testing"

	"github.com/apimgr/vidveil/src/config"
)

// newClientIPResolver builds a resolver whose trusted-proxy set includes the
// default private/loopback ranges. When extraCIDRs is non-empty an AppConfig is
// attached so the additional-CIDR branch of isTrustedProxy is exercised.
func newClientIPResolver(t *testing.T, extraCIDRs ...string) *URLResolver {
	t.Helper()
	r := NewURLResolver(DefaultURLVarsConfig())
	if len(extraCIDRs) > 0 {
		cfg := config.DefaultAppConfig()
		cfg.Server.TrustedProxies.Additional = extraCIDRs
		r.SetAppConfig(cfg)
	}
	return r
}

func reqWith(remoteAddr string, headers map[string]string) *http.Request {
	req := &http.Request{
		RemoteAddr: remoteAddr,
		Header:     http.Header{},
	}
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	return req
}

func TestResolveClientIP_TrustedPeerHeaderPriority(t *testing.T) {
	// From a trusted (loopback) peer, each header wins in priority order.
	cases := []struct {
		name    string
		headers map[string]string
		want    string
	}{
		{
			name:    "cf-connecting-ip wins over all",
			headers: map[string]string{"CF-Connecting-IP": "1.1.1.1", "True-Client-IP": "2.2.2.2", "X-Real-IP": "3.3.3.3", "X-Forwarded-For": "4.4.4.4", "X-Client-IP": "5.5.5.5"},
			want:    "1.1.1.1",
		},
		{
			name:    "true-client-ip wins when no cf",
			headers: map[string]string{"True-Client-IP": "2.2.2.2", "X-Real-IP": "3.3.3.3"},
			want:    "2.2.2.2",
		},
		{
			name:    "x-real-ip wins when no cf/true",
			headers: map[string]string{"X-Real-IP": "3.3.3.3", "X-Forwarded-For": "4.4.4.4"},
			want:    "3.3.3.3",
		},
		{
			name:    "x-forwarded-for leftmost entry",
			headers: map[string]string{"X-Forwarded-For": "4.4.4.4, 10.0.0.1, 10.0.0.2"},
			want:    "4.4.4.4",
		},
		{
			name:    "x-forwarded-for single entry",
			headers: map[string]string{"X-Forwarded-For": "  4.4.4.4  "},
			want:    "4.4.4.4",
		},
		{
			name:    "x-client-ip lowest priority",
			headers: map[string]string{"X-Client-IP": "5.5.5.5"},
			want:    "5.5.5.5",
		},
	}
	r := newClientIPResolver(t)
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := r.resolveClientIP(reqWith("127.0.0.1:5000", tc.headers))
			if got != tc.want {
				t.Errorf("resolveClientIP = %q, want %q", got, tc.want)
			}
		})
	}
}

func TestResolveClientIP_UntrustedPeerIgnoresHeaders(t *testing.T) {
	// A public (untrusted) peer must have its spoofed headers ignored; resolution
	// falls straight through to the RemoteAddr host.
	r := newClientIPResolver(t)
	got := r.resolveClientIP(reqWith("8.8.8.8:4321", map[string]string{
		"CF-Connecting-IP": "1.1.1.1",
		"X-Forwarded-For":  "4.4.4.4",
	}))
	if got != "8.8.8.8" {
		t.Errorf("resolveClientIP = %q, want peer 8.8.8.8 (headers must be ignored)", got)
	}
}

func TestResolveClientIP_NoHeadersFallsBackToPeer(t *testing.T) {
	r := newClientIPResolver(t)
	got := r.resolveClientIP(reqWith("192.168.1.50:9000", nil))
	if got != "192.168.1.50" {
		t.Errorf("resolveClientIP = %q, want 192.168.1.50", got)
	}
}

func TestResolveClientIP_RemoteAddrWithoutPort(t *testing.T) {
	// SplitHostPort fails on a bare address; the whole RemoteAddr is returned.
	r := newClientIPResolver(t)
	got := r.resolveClientIP(reqWith("203.0.113.7", nil))
	if got != "203.0.113.7" {
		t.Errorf("resolveClientIP = %q, want raw 203.0.113.7", got)
	}
}

func TestResolveClientIP_AdditionalCIDRTrusted(t *testing.T) {
	// A peer inside an operator-configured additional CIDR is trusted, so its
	// forwarded header is honored.
	r := newClientIPResolver(t, "203.0.113.0/24")
	got := r.resolveClientIP(reqWith("203.0.113.9:7000", map[string]string{"X-Real-IP": "9.9.9.9"}))
	if got != "9.9.9.9" {
		t.Errorf("resolveClientIP = %q, want honored header 9.9.9.9", got)
	}
}

func TestResolveClientIP_AdditionalSingleIPTrusted(t *testing.T) {
	// A bare IP (no /mask) in Additional matches only that exact peer.
	r := newClientIPResolver(t, "198.51.100.5")
	got := r.resolveClientIP(reqWith("198.51.100.5:7000", map[string]string{"X-Real-IP": "7.7.7.7"}))
	if got != "7.7.7.7" {
		t.Errorf("resolveClientIP = %q, want honored header 7.7.7.7", got)
	}
	// A different peer not in the set is untrusted.
	got = r.resolveClientIP(reqWith("198.51.100.6:7000", map[string]string{"X-Real-IP": "7.7.7.7"}))
	if got != "198.51.100.6" {
		t.Errorf("resolveClientIP = %q, want peer 198.51.100.6", got)
	}
}

func TestIsTrustedProxy_Ranges(t *testing.T) {
	r := newClientIPResolver(t)
	trusted := []string{"127.0.0.1:80", "10.1.2.3:80", "192.168.0.1:80", "172.16.5.5:80", "169.254.1.1:80", "::1", "[fe80::1]:80"}
	for _, addr := range trusted {
		if !r.isTrustedProxy(addr) {
			t.Errorf("isTrustedProxy(%q) = false, want true", addr)
		}
	}
	untrusted := []string{"8.8.8.8:80", "1.2.3.4", "not-an-ip", ""}
	for _, addr := range untrusted {
		if r.isTrustedProxy(addr) {
			t.Errorf("isTrustedProxy(%q) = true, want false", addr)
		}
	}
}

func TestResolveClientIP_Global(t *testing.T) {
	// The package-level ResolveClientIP uses the global resolver; a loopback peer
	// is always trusted, so its forwarded header is honored.
	got := ResolveClientIP(reqWith("127.0.0.1:1234", map[string]string{"X-Real-IP": "6.6.6.6"}))
	if got != "6.6.6.6" {
		t.Errorf("ResolveClientIP = %q, want 6.6.6.6", got)
	}
}
