// SPDX-License-Identifier: MIT
// Coverage tests for email.go paths not covered by email_test.go.
// Targets: NewEmailService, effectiveEmailConfig, Send (disabled path).
package email

import (
	"testing"

	"github.com/apimgr/vidveil/src/config"
)

// ── NewEmailService ──────────────────────────────────────────────────────────

func TestNewEmailService_ReturnsNonNil(t *testing.T) {
	cfg := config.DefaultAppConfig()
	svc := NewEmailService(cfg)
	if svc == nil {
		t.Fatal("NewEmailService() returned nil")
	}
}

func TestNewEmailService_HasAppConfig(t *testing.T) {
	cfg := config.DefaultAppConfig()
	svc := NewEmailService(cfg)
	if svc.appConfig == nil {
		t.Error("NewEmailService: appConfig is nil")
	}
}

func TestNewEmailService_TemplateDirNonEmpty(t *testing.T) {
	cfg := config.DefaultAppConfig()
	svc := NewEmailService(cfg)
	if svc.templateDir == "" {
		t.Error("NewEmailService: templateDir is empty")
	}
}

// ── Send — disabled path ────────────────────────────────────────────────────

func TestSend_DisabledReturnsError(t *testing.T) {
	cfg := config.DefaultAppConfig()
	cfg.Server.Notifications.Email.Enabled = false
	svc := &EmailService{appConfig: cfg, templateDir: t.TempDir()}
	err := svc.Send("security_alert", "user@example.com", nil)
	if err == nil {
		t.Error("Send with email disabled: expected error, got nil")
	}
}

// ── effectiveEmailConfig — config values ────────────────────────────────────

func TestEffectiveEmailConfig_ReturnsConfigValues(t *testing.T) {
	cfg := config.DefaultAppConfig()
	cfg.Server.Notifications.Email.SMTP.Host = "smtp.example.com"
	cfg.Server.Notifications.Email.SMTP.Port = 587
	cfg.Server.Notifications.Email.SMTP.Username = "user"
	cfg.Server.Notifications.Email.SMTP.Password = "pass"
	cfg.Server.Notifications.Email.SMTP.TLS = "starttls"
	cfg.Server.Notifications.Email.From.Email = "from@example.com"
	cfg.Server.Notifications.Email.From.Name = "Example App"
	svc := &EmailService{appConfig: cfg, templateDir: t.TempDir()}

	host, port, username, password, fromAddr, fromName, tlsMode := svc.effectiveEmailConfig()

	if host != "smtp.example.com" {
		t.Errorf("host = %q, want smtp.example.com", host)
	}
	if port != 587 {
		t.Errorf("port = %d, want 587", port)
	}
	if username != "user" {
		t.Errorf("username = %q, want user", username)
	}
	if password != "pass" {
		t.Errorf("password = %q, want pass", password)
	}
	if fromAddr != "from@example.com" {
		t.Errorf("fromAddr = %q, want from@example.com", fromAddr)
	}
	if fromName != "Example App" {
		t.Errorf("fromName = %q, want Example App", fromName)
	}
	if tlsMode != "starttls" {
		t.Errorf("tlsMode = %q, want starttls", tlsMode)
	}
}

func TestEffectiveEmailConfig_EnvVarOverridesHost(t *testing.T) {
	t.Setenv("SMTP_HOST", "envhost.example.com")
	cfg := config.DefaultAppConfig()
	cfg.Server.Notifications.Email.SMTP.Host = "config-host.example.com"
	svc := &EmailService{appConfig: cfg, templateDir: t.TempDir()}

	host, _, _, _, _, _, _ := svc.effectiveEmailConfig()
	if host != "envhost.example.com" {
		t.Errorf("SMTP_HOST env override: host = %q, want envhost.example.com", host)
	}
}

func TestEffectiveEmailConfig_EnvVarOverridesPort(t *testing.T) {
	t.Setenv("SMTP_PORT", "2525")
	cfg := config.DefaultAppConfig()
	cfg.Server.Notifications.Email.SMTP.Port = 25
	svc := &EmailService{appConfig: cfg, templateDir: t.TempDir()}

	_, port, _, _, _, _, _ := svc.effectiveEmailConfig()
	if port != 2525 {
		t.Errorf("SMTP_PORT env override: port = %d, want 2525", port)
	}
}

func TestEffectiveEmailConfig_InvalidPortEnvIgnored(t *testing.T) {
	t.Setenv("SMTP_PORT", "notanumber")
	cfg := config.DefaultAppConfig()
	cfg.Server.Notifications.Email.SMTP.Port = 587
	svc := &EmailService{appConfig: cfg, templateDir: t.TempDir()}

	_, port, _, _, _, _, _ := svc.effectiveEmailConfig()
	if port != 587 {
		t.Errorf("invalid SMTP_PORT: port = %d, want config value 587", port)
	}
}

func TestEffectiveEmailConfig_EnvVarOverridesUsername(t *testing.T) {
	t.Setenv("SMTP_USERNAME", "envuser")
	cfg := config.DefaultAppConfig()
	cfg.Server.Notifications.Email.SMTP.Username = "cfguser"
	svc := &EmailService{appConfig: cfg, templateDir: t.TempDir()}

	_, _, username, _, _, _, _ := svc.effectiveEmailConfig()
	if username != "envuser" {
		t.Errorf("SMTP_USERNAME env override: username = %q, want envuser", username)
	}
}

func TestEffectiveEmailConfig_EnvVarOverridesPassword(t *testing.T) {
	t.Setenv("SMTP_PASSWORD", "envpass")
	cfg := config.DefaultAppConfig()
	cfg.Server.Notifications.Email.SMTP.Password = "cfgpass"
	svc := &EmailService{appConfig: cfg, templateDir: t.TempDir()}

	_, _, _, password, _, _, _ := svc.effectiveEmailConfig()
	if password != "envpass" {
		t.Errorf("SMTP_PASSWORD env override: password = %q, want envpass", password)
	}
}

func TestEffectiveEmailConfig_EnvVarOverridesTLS(t *testing.T) {
	t.Setenv("SMTP_TLS", "ssl")
	cfg := config.DefaultAppConfig()
	cfg.Server.Notifications.Email.SMTP.TLS = "none"
	svc := &EmailService{appConfig: cfg, templateDir: t.TempDir()}

	_, _, _, _, _, _, tlsMode := svc.effectiveEmailConfig()
	if tlsMode != "ssl" {
		t.Errorf("SMTP_TLS env override: tlsMode = %q, want ssl", tlsMode)
	}
}

func TestEffectiveEmailConfig_EnvVarOverridesFromEmail(t *testing.T) {
	t.Setenv("SMTP_FROM_EMAIL", "env-from@example.com")
	cfg := config.DefaultAppConfig()
	cfg.Server.Notifications.Email.From.Email = "cfg-from@example.com"
	svc := &EmailService{appConfig: cfg, templateDir: t.TempDir()}

	_, _, _, _, fromAddr, _, _ := svc.effectiveEmailConfig()
	if fromAddr != "env-from@example.com" {
		t.Errorf("SMTP_FROM_EMAIL env override: fromAddr = %q, want env-from@example.com", fromAddr)
	}
}

func TestEffectiveEmailConfig_EnvVarOverridesFromName(t *testing.T) {
	t.Setenv("SMTP_FROM_NAME", "Env App Name")
	cfg := config.DefaultAppConfig()
	cfg.Server.Notifications.Email.From.Name = "Config App Name"
	svc := &EmailService{appConfig: cfg, templateDir: t.TempDir()}

	_, _, _, _, _, fromName, _ := svc.effectiveEmailConfig()
	if fromName != "Env App Name" {
		t.Errorf("SMTP_FROM_NAME env override: fromName = %q, want Env App Name", fromName)
	}
}

func TestEffectiveEmailConfig_FallsBackFromAddrToFQDN(t *testing.T) {
	cfg := config.DefaultAppConfig()
	cfg.Server.Notifications.Email.From.Email = ""
	cfg.Server.FQDN = "myserver.example.com"
	svc := &EmailService{appConfig: cfg, templateDir: t.TempDir()}

	_, _, _, _, fromAddr, _, _ := svc.effectiveEmailConfig()
	if fromAddr != "no-reply@myserver.example.com" {
		t.Errorf("FQDN fallback: fromAddr = %q, want no-reply@myserver.example.com", fromAddr)
	}
}

func TestEffectiveEmailConfig_FallsBackFromNameToBrandingTitle(t *testing.T) {
	cfg := config.DefaultAppConfig()
	cfg.Server.Notifications.Email.From.Name = ""
	cfg.Server.Branding.Title = "My Branding Title"
	svc := &EmailService{appConfig: cfg, templateDir: t.TempDir()}

	_, _, _, _, _, fromName, _ := svc.effectiveEmailConfig()
	if fromName != "My Branding Title" {
		t.Errorf("branding title fallback: fromName = %q, want My Branding Title", fromName)
	}
}
