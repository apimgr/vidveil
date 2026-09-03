// SPDX-License-Identifier: MIT
package email

import (
	"strings"
	"testing"

	"github.com/apimgr/vidveil/src/config"
)

// newTestEmailService constructs an EmailService without calling GetAppPaths,
// safe for use in tests with no filesystem side effects.
func newTestEmailService(t *testing.T) *EmailService {
	t.Helper()
	cfg := config.DefaultAppConfig()
	return &EmailService{
		appConfig:   cfg,
		templateDir: t.TempDir(),
	}
}

// --- parseTemplate ---

func TestParseTemplate_ValidFormat(t *testing.T) {
	svc := newTestEmailService(t)
	subject, body := svc.parseTemplate("Subject: Hello\n---\nBody text")
	if subject != "Hello" {
		t.Errorf("parseTemplate subject = %q, want %q", subject, "Hello")
	}
	if body != "Body text" {
		t.Errorf("parseTemplate body = %q, want %q", body, "Body text")
	}
}

func TestParseTemplate_NoSeparator(t *testing.T) {
	svc := newTestEmailService(t)
	input := "Subject: Hello\nBody text"
	subject, body := svc.parseTemplate(input)
	if subject != "Notification" {
		t.Errorf("parseTemplate subject = %q, want %q", subject, "Notification")
	}
	if body != input {
		t.Errorf("parseTemplate body = %q, want full input", body)
	}
}

func TestParseTemplate_NoSubjectPrefix(t *testing.T) {
	svc := newTestEmailService(t)
	subject, body := svc.parseTemplate("Not a subject line\n---\nBody")
	if subject != "Notification" {
		t.Errorf("parseTemplate subject = %q, want %q", subject, "Notification")
	}
	if body != "Body" {
		t.Errorf("parseTemplate body = %q, want %q", body, "Body")
	}
}

func TestParseTemplate_WhitespaceTrimmed(t *testing.T) {
	svc := newTestEmailService(t)
	subject, body := svc.parseTemplate("Subject:  Trimmed  \n---\n  Body with spaces  ")
	// Subject: prefix is exactly "Subject: " so leading space becomes part of subject value
	if !strings.Contains(subject, "Trimmed") {
		t.Errorf("parseTemplate subject %q does not contain trimmed value", subject)
	}
	if strings.HasPrefix(body, " ") || strings.HasSuffix(body, " ") {
		t.Errorf("parseTemplate body %q has leading/trailing whitespace", body)
	}
}

func TestParseTemplate_EmptyInput(t *testing.T) {
	svc := newTestEmailService(t)
	subject, body := svc.parseTemplate("")
	if subject != "Notification" {
		t.Errorf("parseTemplate empty input subject = %q, want %q", subject, "Notification")
	}
	if body != "" {
		t.Errorf("parseTemplate empty input body = %q, want empty", body)
	}
}

func TestParseTemplate_MultilineBody(t *testing.T) {
	svc := newTestEmailService(t)
	subject, body := svc.parseTemplate("Subject: Multi\n---\nLine one\nLine two\nLine three")
	if subject != "Multi" {
		t.Errorf("parseTemplate subject = %q, want %q", subject, "Multi")
	}
	if !strings.Contains(body, "Line one") || !strings.Contains(body, "Line three") {
		t.Errorf("parseTemplate body %q missing expected lines", body)
	}
}

// --- applyVars ---

func TestApplyVars_BasicSubstitution(t *testing.T) {
	svc := newTestEmailService(t)
	result := svc.applyVars("{name} visited {site}", map[string]string{
		"name": "Alice",
		"site": "example.com",
	})
	if result != "Alice visited example.com" {
		t.Errorf("applyVars = %q, want %q", result, "Alice visited example.com")
	}
}

func TestApplyVars_MissingVarUnchanged(t *testing.T) {
	svc := newTestEmailService(t)
	input := "Hello {missing}"
	result := svc.applyVars(input, map[string]string{"other": "value"})
	if result != input {
		t.Errorf("applyVars with missing var = %q, want original %q", result, input)
	}
}

func TestApplyVars_EmptyVarsMap(t *testing.T) {
	svc := newTestEmailService(t)
	input := "No {substitution} here"
	result := svc.applyVars(input, map[string]string{})
	if result != input {
		t.Errorf("applyVars empty map = %q, want %q", result, input)
	}
}

func TestApplyVars_NilVarsMap(t *testing.T) {
	svc := newTestEmailService(t)
	input := "No {substitution} here"
	result := svc.applyVars(input, nil)
	if result != input {
		t.Errorf("applyVars nil map = %q, want %q", result, input)
	}
}

func TestApplyVars_MultipleOccurrencesOfSameVar(t *testing.T) {
	svc := newTestEmailService(t)
	result := svc.applyVars("{x} and {x} and {x}", map[string]string{"x": "go"})
	if result != "go and go and go" {
		t.Errorf("applyVars multiple occurrences = %q, want %q", result, "go and go and go")
	}
}

func TestApplyVars_EmptyValue(t *testing.T) {
	svc := newTestEmailService(t)
	result := svc.applyVars("before {empty} after", map[string]string{"empty": ""})
	if result != "before  after" {
		t.Errorf("applyVars empty value = %q, want %q", result, "before  after")
	}
}

// --- getGlobalVars ---

func TestGetGlobalVars_RequiredKeys(t *testing.T) {
	svc := newTestEmailService(t)
	vars := svc.getGlobalVars()
	for _, key := range []string{"app_name", "app_url"} {
		if _, ok := vars[key]; !ok {
			t.Errorf("getGlobalVars missing key %q", key)
		}
	}
}

func TestGetGlobalVars_HTTPSchemeWhenSSLDisabled(t *testing.T) {
	svc := newTestEmailService(t)
	svc.appConfig.Server.SSL.Enabled = false
	svc.appConfig.Server.SSL.LetsEncrypt.Enabled = false
	vars := svc.getGlobalVars()
	if !strings.HasPrefix(vars["app_url"], "http://") {
		t.Errorf("app_url = %q, want http:// prefix when SSL disabled", vars["app_url"])
	}
}

func TestGetGlobalVars_HTTPSSchemeWhenSSLEnabled(t *testing.T) {
	svc := newTestEmailService(t)
	svc.appConfig.Server.SSL.Enabled = true
	vars := svc.getGlobalVars()
	if !strings.HasPrefix(vars["app_url"], "https://") {
		t.Errorf("app_url = %q, want https:// prefix when SSL enabled", vars["app_url"])
	}
}

func TestGetGlobalVars_HTTPSSchemeWhenLetsEncryptEnabled(t *testing.T) {
	svc := newTestEmailService(t)
	svc.appConfig.Server.SSL.Enabled = false
	svc.appConfig.Server.SSL.LetsEncrypt.Enabled = true
	vars := svc.getGlobalVars()
	if !strings.HasPrefix(vars["app_url"], "https://") {
		t.Errorf("app_url = %q, want https:// prefix when LetsEncrypt enabled", vars["app_url"])
	}
}

func TestGetGlobalVars_OmitsPort80(t *testing.T) {
	svc := newTestEmailService(t)
	svc.appConfig.Server.SSL.Enabled = false
	svc.appConfig.Server.Port = "80"
	svc.appConfig.Server.FQDN = "example.com"
	vars := svc.getGlobalVars()
	if strings.Contains(vars["app_url"], ":80") {
		t.Errorf("app_url = %q should not include port 80", vars["app_url"])
	}
}

func TestGetGlobalVars_OmitsPort443(t *testing.T) {
	svc := newTestEmailService(t)
	svc.appConfig.Server.SSL.Enabled = true
	svc.appConfig.Server.Port = "443"
	svc.appConfig.Server.FQDN = "example.com"
	vars := svc.getGlobalVars()
	if strings.Contains(vars["app_url"], ":443") {
		t.Errorf("app_url = %q should not include port 443", vars["app_url"])
	}
}

func TestGetGlobalVars_IncludesCustomPort(t *testing.T) {
	svc := newTestEmailService(t)
	svc.appConfig.Server.SSL.Enabled = false
	svc.appConfig.Server.Port = "8080"
	svc.appConfig.Server.FQDN = "example.com"
	vars := svc.getGlobalVars()
	if !strings.Contains(vars["app_url"], ":8080") {
		t.Errorf("app_url = %q should include custom port 8080", vars["app_url"])
	}
}

// --- GetTemplateList ---

func TestGetTemplateList_NonNil(t *testing.T) {
	svc := newTestEmailService(t)
	list := svc.GetTemplateList()
	if list == nil {
		t.Fatal("GetTemplateList() returned nil")
	}
}

func TestGetTemplateList_ContainsKnownTemplates(t *testing.T) {
	svc := newTestEmailService(t)
	list := svc.GetTemplateList()

	found := make(map[string]bool, len(list))
	for _, name := range list {
		found[name] = true
	}

	// These keys exist in defaultTemplates
	for _, name := range []string{
		"security_alert",
		"backup_complete",
		"backup_failed",
		"ssl_expiring",
		"ssl_renewed",
		"scheduler_error",
		"test",
	} {
		if !found[name] {
			t.Errorf("GetTemplateList() missing expected template %q", name)
		}
	}
}

// --- GetTemplate ---

func TestGetTemplate_KnownTemplate(t *testing.T) {
	svc := newTestEmailService(t)
	content, err := svc.GetTemplate("test")
	if err != nil {
		t.Fatalf("GetTemplate(%q) error = %v", "test", err)
	}
	if content == "" {
		t.Errorf("GetTemplate(%q) returned empty content", "test")
	}
}

func TestGetTemplate_UnknownTemplateReturnsError(t *testing.T) {
	svc := newTestEmailService(t)
	_, err := svc.GetTemplate("nonexistent_template_xyz")
	if err == nil {
		t.Error("GetTemplate with unknown name should return error, got nil")
	}
}

func TestGetTemplate_AllKnownTemplatesReadable(t *testing.T) {
	svc := newTestEmailService(t)
	for _, name := range []string{
		"security_alert",
		"backup_complete",
		"backup_failed",
		"ssl_expiring",
		"ssl_renewed",
		"scheduler_error",
		"test",
	} {
		content, err := svc.GetTemplate(name)
		if err != nil {
			t.Errorf("GetTemplate(%q) error = %v", name, err)
		}
		if content == "" {
			t.Errorf("GetTemplate(%q) returned empty content", name)
		}
	}
}

// --- SaveTemplate / IsCustomTemplate / ResetTemplate ---

func TestSaveTemplate_CreatesFile(t *testing.T) {
	svc := newTestEmailService(t)
	if err := svc.SaveTemplate("mytest", "Subject: Hi\n---\nHello"); err != nil {
		t.Fatalf("SaveTemplate error = %v", err)
	}
	if !svc.IsCustomTemplate("mytest") {
		t.Error("IsCustomTemplate should be true after SaveTemplate")
	}
}

func TestIsCustomTemplate_FalseForNonexistent(t *testing.T) {
	svc := newTestEmailService(t)
	if svc.IsCustomTemplate("no_such_template") {
		t.Error("IsCustomTemplate should be false for nonexistent template")
	}
}

func TestIsCustomTemplate_FalseForDefaultOnly(t *testing.T) {
	svc := newTestEmailService(t)
	// "test" exists as a default but has not been saved as custom
	if svc.IsCustomTemplate("test") {
		t.Error("IsCustomTemplate should be false when only a default exists")
	}
}

func TestResetTemplate_RemovesCustomFile(t *testing.T) {
	svc := newTestEmailService(t)
	if err := svc.SaveTemplate("reset_me", "content"); err != nil {
		t.Fatalf("SaveTemplate error = %v", err)
	}
	if err := svc.ResetTemplate("reset_me"); err != nil {
		t.Fatalf("ResetTemplate error = %v", err)
	}
	if svc.IsCustomTemplate("reset_me") {
		t.Error("IsCustomTemplate should be false after ResetTemplate")
	}
}

func TestResetTemplate_NonexistentIsNoOp(t *testing.T) {
	svc := newTestEmailService(t)
	if err := svc.ResetTemplate("never_existed"); err != nil {
		t.Errorf("ResetTemplate on nonexistent should not error, got %v", err)
	}
}

func TestResetTemplate_IdempotentDoubleReset(t *testing.T) {
	svc := newTestEmailService(t)
	if err := svc.SaveTemplate("double", "body"); err != nil {
		t.Fatalf("SaveTemplate error = %v", err)
	}
	if err := svc.ResetTemplate("double"); err != nil {
		t.Fatalf("first ResetTemplate error = %v", err)
	}
	if err := svc.ResetTemplate("double"); err != nil {
		t.Errorf("second ResetTemplate should be no-op, got %v", err)
	}
}

func TestSaveTemplate_CustomOverridesDefault(t *testing.T) {
	svc := newTestEmailService(t)
	customContent := "Subject: Custom\n---\nCustom body"
	if err := svc.SaveTemplate("test", customContent); err != nil {
		t.Fatalf("SaveTemplate error = %v", err)
	}
	got, err := svc.GetTemplate("test")
	if err != nil {
		t.Fatalf("GetTemplate after save error = %v", err)
	}
	if got != customContent {
		t.Errorf("GetTemplate returned %q, want custom content %q", got, customContent)
	}
}

func TestResetTemplate_FallsBackToDefault(t *testing.T) {
	svc := newTestEmailService(t)
	original, err := svc.GetTemplate("test")
	if err != nil {
		t.Fatalf("GetTemplate original error = %v", err)
	}

	if err := svc.SaveTemplate("test", "Subject: Override\n---\nOverride body"); err != nil {
		t.Fatalf("SaveTemplate error = %v", err)
	}
	if err := svc.ResetTemplate("test"); err != nil {
		t.Fatalf("ResetTemplate error = %v", err)
	}

	restored, err := svc.GetTemplate("test")
	if err != nil {
		t.Fatalf("GetTemplate after reset error = %v", err)
	}
	if restored != original {
		t.Errorf("GetTemplate after reset = %q, want original %q", restored, original)
	}
}
