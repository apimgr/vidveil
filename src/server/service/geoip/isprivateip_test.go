// SPDX-License-Identifier: MIT
// AI.md PART 19: deterministic coverage for private/internal IP detection,
// which gates whether an address is exempt from country blocking.
package geoip

import (
	"net"
	"testing"
)

func TestIsPrivateIP(t *testing.T) {
	cases := []struct {
		name string
		ip   string
		want bool
	}{
		{"nil", "", false},
		{"loopback v4", "127.0.0.1", true},
		{"loopback v6", "::1", true},
		{"link-local v4", "169.254.10.1", true},
		{"link-local v6", "fe80::1", true},
		{"rfc1918 10/8", "10.1.2.3", true},
		{"rfc1918 172.16/12", "172.16.5.5", true},
		{"rfc1918 192.168/16", "192.168.1.1", true},
		{"rfc4193 fc00::/7", "fc00::abcd", true},
		{"public v4", "8.8.8.8", false},
		{"public v4 near-172", "172.32.0.1", false},
		{"public v6", "2606:4700:4700::1111", false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var ip net.IP
			if tc.ip != "" {
				ip = net.ParseIP(tc.ip)
			}
			if got := isPrivateIP(ip); got != tc.want {
				t.Errorf("isPrivateIP(%q) = %v, want %v", tc.ip, got, tc.want)
			}
		})
	}
}
