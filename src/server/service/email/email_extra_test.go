// SPDX-License-Identifier: MIT
// Additional coverage tests for email.go paths not covered by email_test.go
// or email_coverage_test.go:
// AutodetectSMTP (no-host path), TestSMTPConfig (empty host + refused port),
// getGatewayIP (invoke-only), SendTest disabled path.
package email

import (
	"testing"

	"github.com/apimgr/vidveil/src/config"
)

// ── AutodetectSMTP ────────────────────────────────────────────────────────────

// TestAutodetectSMTP_NoReachableHost verifies that when all supplied hosts
// are unreachable, AutodetectSMTP returns ("", 0).
func TestAutodetectSMTP_NoReachableHost(t *testing.T) {
	hosts := []string{"192.0.2.1", "192.0.2.2"}
	ports := []int{1}
	host, port := AutodetectSMTP(hosts, ports)
	if host != "" {
		t.Errorf("AutodetectSMTP unreachable: host = %q, want empty", host)
	}
	if port != 0 {
		t.Errorf("AutodetectSMTP unreachable: port = %d, want 0", port)
	}
}

// TestAutodetectSMTP_EmptyListsNoReachableHost verifies the default-host
// code path runs without panic (default hosts are not guaranteed reachable
// in CI; we just assert no panic and valid return types).
func TestAutodetectSMTP_EmptyListsNoPanic(t *testing.T) {
	host, port := AutodetectSMTP(nil, nil)
	_ = host
	_ = port
}

// TestAutodetectSMTP_CustomPortsOnly verifies that when custom hosts are
// supplied together with custom ports and nothing responds, we get ("", 0).
func TestAutodetectSMTP_CustomPortsOnly(t *testing.T) {
	host, port := AutodetectSMTP([]string{"192.0.2.99"}, []int{65534})
	if host != "" || port != 0 {
		t.Errorf("AutodetectSMTP custom unreachable: got (%q, %d), want (\"\", 0)", host, port)
	}
}

// ── TestSMTPConfig ───────────────────────────────────────────────────────────

// TestTestSMTPConfig_EmptyHostReturnsError verifies that an empty host
// is rejected without making a network call.
func TestTestSMTPConfig_EmptyHostReturnsError(t *testing.T) {
	err := TestSMTPConfig("", 587)
	if err == nil {
		t.Error("TestSMTPConfig empty host: expected error, got nil")
	}
}

// TestTestSMTPConfig_RefusedPortReturnsError verifies that a refused
// connection (port 1 on loopback) is reported as an error.
func TestTestSMTPConfig_RefusedPortReturnsError(t *testing.T) {
	err := TestSMTPConfig("127.0.0.1", 1)
	if err == nil {
		t.Error("TestSMTPConfig refused port: expected error, got nil")
	}
}

// ── getGatewayIP ─────────────────────────────────────────────────────────────

// TestGetGatewayIP_NoPanic verifies that getGatewayIP does not panic and
// returns a string (may be empty if the UDP probe fails in the test env).
func TestGetGatewayIP_NoPanic(t *testing.T) {
	defer func() {
		if r := recover(); r != nil {
			t.Errorf("getGatewayIP() panicked: %v", r)
		}
	}()
	ip := getGatewayIP()
	_ = ip
}

// TestGetGatewayIP_ReturnsStringOrEmpty verifies the return type and that
// if non-empty the returned string looks like an IP address.
func TestGetGatewayIP_ReturnsStringOrEmpty(t *testing.T) {
	ip := getGatewayIP()
	if ip == "" {
		return
	}
	if len(ip) < 7 {
		t.Errorf("getGatewayIP() = %q, too short to be an IPv4 address", ip)
	}
}

// ── SendTest ──────────────────────────────────────────────────────────────────

// TestSendTest_DisabledReturnsError verifies that SendTest proxies through
// Send and returns an error when email is disabled.
func TestSendTest_DisabledReturnsError(t *testing.T) {
	cfg := config.DefaultAppConfig()
	cfg.Server.Email.Enabled = false
	svc := &EmailService{appConfig: cfg, templateDir: t.TempDir()}
	if err := svc.SendTest("admin@example.com"); err == nil {
		t.Error("SendTest with email disabled: expected error, got nil")
	}
}

// ── Send — unknown template ───────────────────────────────────────────────────

// TestSend_UnknownTemplateReturnsError verifies that Send returns an error
// when the requested template does not exist.
func TestSend_UnknownTemplateReturnsError(t *testing.T) {
	cfg := config.DefaultAppConfig()
	cfg.Server.Email.Enabled = true
	cfg.Server.Email.Host = "smtp.example.com"
	cfg.Server.Email.Port = 587
	svc := &EmailService{appConfig: cfg, templateDir: t.TempDir()}
	err := svc.Send("nonexistent_template_xyz", "user@example.com", nil)
	if err == nil {
		t.Error("Send with unknown template: expected error, got nil")
	}
}
