// SPDX-License-Identifier: MIT
// AI.md PART 28: Coverage tests for sendEmail, sendTLS, and autodetectSMTP.
// All tests target early-error / connection-refused paths — no real SMTP server needed.
package email

import (
	"testing"

	"github.com/apimgr/vidveil/src/config"
)

// newEnabledEmailService returns an EmailService with Email.Enabled=true.
func newEnabledEmailService(t *testing.T) *EmailService {
	t.Helper()
	appCfg := config.DefaultAppConfig()
	appCfg.Server.Email.Enabled = true
	return NewEmailService(appCfg)
}

// ── sendEmail — "no SMTP host configured" early-return ───────────────────────

func TestSendEmail_NoHostNoAutodetect_ReturnsError(t *testing.T) {
	t.Setenv("SMTP_HOST", "")
	appCfg := config.DefaultAppConfig()
	appCfg.Server.Email.Enabled = true
	appCfg.Server.Email.Host = ""
	appCfg.Server.Email.Autodetect = false
	appCfg.Server.Email.AutodetectHost = nil
	s := NewEmailService(appCfg)
	err := s.sendEmail("to@example.com", "subject", "body")
	if err == nil {
		t.Error("sendEmail(no host, no autodetect): expected error, got nil")
	}
}

// ── sendEmail — autodetect enabled but no reachable host ─────────────────────

func TestSendEmail_AutodetectEnabled_NoReachableHost(t *testing.T) {
	t.Setenv("SMTP_HOST", "")
	appCfg := config.DefaultAppConfig()
	appCfg.Server.Email.Enabled = true
	appCfg.Server.Email.Host = ""
	appCfg.Server.Email.Autodetect = true
	appCfg.Server.Email.AutodetectHost = []string{"127.0.0.1"}
	appCfg.Server.Email.AutodetectPort = []int{1}
	s := NewEmailService(appCfg)

	err := s.sendEmail("to@example.com", "subject", "body")
	if err == nil {
		t.Error("sendEmail(autodetect, no reachable host): expected error, got nil")
	}
}

// ── sendEmail — STARTTLS path, connection refused (covers smtp.SendMail call) ─

func TestSendEmail_StarttlsConnRefused(t *testing.T) {
	appCfg := config.DefaultAppConfig()
	appCfg.Server.Email.Enabled = true
	appCfg.Server.Email.Host = "127.0.0.1"
	appCfg.Server.Email.Port = 1
	appCfg.Server.Email.TLS = "starttls"
	s := NewEmailService(appCfg)

	err := s.sendEmail("to@example.com", "subject", "body")
	if err == nil {
		t.Error("sendEmail STARTTLS conn-refused: expected error, got nil")
	}
}

// ── sendEmail — "none" TLS mode covers the smtp.SendMail branch ──────────────

func TestSendEmail_TLSModeNone_ConnRefused(t *testing.T) {
	appCfg := config.DefaultAppConfig()
	appCfg.Server.Email.Enabled = true
	appCfg.Server.Email.Host = "127.0.0.1"
	appCfg.Server.Email.Port = 1
	appCfg.Server.Email.TLS = "none"
	s := NewEmailService(appCfg)

	err := s.sendEmail("to@example.com", "subject", "body")
	if err == nil {
		t.Error("sendEmail TLS=none conn-refused: expected error, got nil")
	}
}

// ── sendEmail — port 465 → implicit TLS → sendTLS path ───────────────────────

func TestSendEmail_ImplicitTLS_Port465_ConnRefused(t *testing.T) {
	appCfg := config.DefaultAppConfig()
	appCfg.Server.Email.Enabled = true
	appCfg.Server.Email.Host = "127.0.0.1"
	appCfg.Server.Email.Port = 465
	s := NewEmailService(appCfg)

	err := s.sendEmail("to@example.com", "subject", "body")
	if err == nil {
		t.Error("sendEmail port 465 / sendTLS conn-refused: expected error, got nil")
	}
}

// ── sendEmail — explicit tls mode → sendTLS path ─────────────────────────────

func TestSendEmail_ExplicitTLSMode_ConnRefused(t *testing.T) {
	appCfg := config.DefaultAppConfig()
	appCfg.Server.Email.Enabled = true
	appCfg.Server.Email.Host = "127.0.0.1"
	appCfg.Server.Email.Port = 1
	appCfg.Server.Email.TLS = "tls"
	s := NewEmailService(appCfg)

	err := s.sendEmail("to@example.com", "subject", "body")
	if err == nil {
		t.Error("sendEmail TLS=tls conn-refused: expected error, got nil")
	}
}

// ── sendEmail with auth (username set) ───────────────────────────────────────

func TestSendEmail_WithUsername_ConnRefused(t *testing.T) {
	appCfg := config.DefaultAppConfig()
	appCfg.Server.Email.Enabled = true
	appCfg.Server.Email.Host = "127.0.0.1"
	appCfg.Server.Email.Port = 1
	appCfg.Server.Email.Username = "user@example.com"
	appCfg.Server.Email.Password = "pass"
	appCfg.Server.Email.TLS = "starttls"
	s := NewEmailService(appCfg)

	err := s.sendEmail("to@example.com", "subject", "body")
	if err == nil {
		t.Error("sendEmail with auth conn-refused: expected error, got nil")
	}
}

// ── sendEmail — from-name path (covers "Name <email>" formatting) ─────────────

func TestSendEmail_WithFromName_ConnRefused(t *testing.T) {
	appCfg := config.DefaultAppConfig()
	appCfg.Server.Email.Enabled = true
	appCfg.Server.Email.Host = "127.0.0.1"
	appCfg.Server.Email.Port = 1
	appCfg.Server.Email.TLS = "starttls"
	appCfg.Server.Email.FromName = "VidVeil"
	appCfg.Server.Email.FromEmail = "no-reply@example.com"
	s := NewEmailService(appCfg)

	err := s.sendEmail("to@example.com", "subject", "body")
	if err == nil {
		t.Error("sendEmail with from-name conn-refused: expected error, got nil")
	}
}

// ── sendTLS directly: conn refused at tls.Dial ───────────────────────────────

func TestSendTLS_ConnRefused(t *testing.T) {
	appCfg := config.DefaultAppConfig()
	s := NewEmailService(appCfg)

	err := s.sendTLS("127.0.0.1:1", "127.0.0.1", nil, "from@example.com", "to@example.com", []byte("msg"))
	if err == nil {
		t.Error("sendTLS conn-refused: expected error, got nil")
	}
}

// ── autodetectSMTP (method): call via sendEmail with empty custom host lists ──

func TestAutodetectSMTP_ViaServiceMethod(t *testing.T) {
	appCfg := config.DefaultAppConfig()
	appCfg.Server.Email.Enabled = true
	appCfg.Server.Email.Autodetect = true
	appCfg.Server.Email.AutodetectHost = []string{"127.0.0.1"}
	appCfg.Server.Email.AutodetectPort = []int{1}
	s := NewEmailService(appCfg)

	err := s.sendEmail("to@example.com", "subj", "body")
	if err == nil {
		t.Error("autodetectSMTP (conn refused): expected error, got nil")
	}
}

// ── Send — full template flow (covers parseTemplate, applyVars, getGlobalVars) ──

func TestSend_EnabledValidTemplate_CoversFlow(t *testing.T) {
	t.Setenv("SMTP_HOST", "")
	appCfg := config.DefaultAppConfig()
	appCfg.Server.Email.Enabled = true
	appCfg.Server.Email.Host = ""
	appCfg.Server.Email.Autodetect = false
	s := NewEmailService(appCfg)

	err := s.Send("test", "to@example.com", map[string]string{"name": "Tester"})
	if err == nil {
		t.Error("Send(no SMTP host): expected error, got nil")
	}
}

func TestSend_EnabledValidTemplate_WithVars_CoversApplyVars(t *testing.T) {
	t.Setenv("SMTP_HOST", "")
	appCfg := config.DefaultAppConfig()
	appCfg.Server.Email.Enabled = true
	appCfg.Server.Email.Host = ""
	appCfg.Server.Email.Autodetect = false
	s := NewEmailService(appCfg)

	err := s.Send("security_alert", "to@example.com", map[string]string{"event": "test event"})
	if err == nil {
		t.Error("Send security_alert (no SMTP host): expected error, got nil")
	}
}
