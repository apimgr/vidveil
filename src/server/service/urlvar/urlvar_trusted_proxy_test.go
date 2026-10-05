// SPDX-License-Identifier: MIT
// Tests for the trusted-proxy set required by AI.md PART 12, including the
// "/24 as the configured listen address" containerized sidecar rule.
package urlvar

import (
	"net"
	"testing"

	"github.com/apimgr/vidveil/src/config"
)

// TestSameSubnetAsListen covers the AI.md PART 12 trusted-proxy rule that trusts
// the reverse-proxy sidecar address sharing a /24 with the listen address.
func TestSameSubnetAsListen(t *testing.T) {
	tests := []struct {
		name    string
		peer    string
		listen  string
		trusted bool
	}{
		{"same /24", "10.0.0.7", "10.0.0.1", true},
		{"different /24", "10.0.1.7", "10.0.0.1", false},
		{"same address", "10.0.0.1", "10.0.0.1", true},
		{"unrelated public", "203.0.113.9", "10.0.0.1", false},
		{"wildcard v4 bind trusts nothing", "10.0.0.7", "0.0.0.0", false},
		{"wildcard v6 bind trusts nothing", "fd00::5", "[::]", false},
		{"empty listen address", "10.0.0.7", "", false},
		{"unparseable listen address", "10.0.0.7", "not-an-ip", false},
		{"v4 peer vs v6 listen", "10.0.0.7", "fd00::1", false},
		{"same v6 /64", "fd00::9", "fd00::1", true},
		{"bracketed v6 listen address", "fd00::9", "[fd00::1]", true},
		{"different v6 /64", "fd00:1::9", "fd00::1", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ip := net.ParseIP(tt.peer)
			if ip == nil {
				t.Fatalf("test peer %q is not a valid IP", tt.peer)
			}
			if got := sameSubnetAsListen(ip, tt.listen); got != tt.trusted {
				t.Errorf("sameSubnetAsListen(%q, %q) = %v, want %v", tt.peer, tt.listen, got, tt.trusted)
			}
		})
	}
}

// TestIsTrustedProxyListenSubnet verifies the resolver honors a concrete listen
// address and ignores a wildcard bind, with no additional CIDRs configured.
func TestIsTrustedProxyListenSubnet(t *testing.T) {
	tests := []struct {
		name    string
		listen  string
		peer    string
		trusted bool
	}{
		{"sidecar in listen /24", "10.0.0.1", "10.0.0.7:5000", true},
		// A routable peer: RFC1918 peers are trusted by the private-range rule
		// regardless of the listen address, so they cannot test the /24 rule.
		{"public peer outside listen /24", "10.0.0.1", "203.0.113.7:5000", false},
		{"wildcard bind, peer not otherwise trusted", "0.0.0.0", "8.8.8.8:5000", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := NewURLResolver(DefaultURLVarsConfig())
			cfg := config.DefaultAppConfig()
			cfg.Server.Address = tt.listen
			r.SetAppConfig(cfg)

			if got := r.isTrustedProxy(tt.peer); got != tt.trusted {
				t.Errorf("isTrustedProxy(%q) with listen %q = %v, want %v", tt.peer, tt.listen, got, tt.trusted)
			}
		})
	}
}
