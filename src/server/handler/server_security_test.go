// SPDX-License-Identifier: MIT
// AI.md PART 11: Security Reports — handler-level tests for the
// coordinated-disclosure contact-form submission paths (HTML and API) and
// the Affected Component dropdown/"Other" fallback.
package handler

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"net/url"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/apimgr/vidveil/src/common/authpad"
	"github.com/apimgr/vidveil/src/config"
	"github.com/apimgr/vidveil/src/server/service/database"
	"github.com/apimgr/vidveil/src/server/service/email"
	"github.com/apimgr/vidveil/src/server/service/secreport"
	"github.com/apimgr/vidveil/src/server/service/secret"
)

type securityReportEmailCall struct {
	template string
	to       string
	vars     map[string]string
}

type recordingSecurityReportEmailSender struct {
	calls       []securityReportEmailCall
	acknowledge error
	contactErr  error
}

func (s *recordingSecurityReportEmailSender) Send(templateName, to string, vars map[string]string) error {
	copied := make(map[string]string, len(vars))
	for key, value := range vars {
		copied[key] = value
	}
	s.calls = append(s.calls, securityReportEmailCall{template: templateName, to: to, vars: copied})
	if templateName == "security_report_researcher_ack" {
		return s.acknowledge
	}
	return nil
}

func (s *recordingSecurityReportEmailSender) SendRaw(to, subject, body string) error {
	s.calls = append(s.calls, securityReportEmailCall{template: "raw:" + subject, to: to, vars: map[string]string{"body": body}})
	return nil
}

func (s *recordingSecurityReportEmailSender) SendRawAttachment(to, subject, body, filename, contentType string, payload []byte) error {
	s.calls = append(s.calls, securityReportEmailCall{template: "attachment:" + subject, to: to, vars: map[string]string{
		"body": body, "filename": filename, "content_type": contentType, "payload": string(payload),
	}})
	return nil
}

func (s *recordingSecurityReportEmailSender) SendContact(to, replyTo, subject, body string) error {
	s.calls = append(s.calls, securityReportEmailCall{template: "contact", to: to, vars: map[string]string{"reply_to": replyTo, "subject": subject, "body": body}})
	return s.contactErr
}

func TestContactSubmissionSendsOneServerRoutedEmail(t *testing.T) {
	h, _ := newSecurityTestHandler(t)
	h.appConfig.Server.Pages.Contact.Antispam.MinFillSeconds = 0
	h.appConfig.Server.Contact.General.Email = "contact@example.test"
	sender := &recordingSecurityReportEmailSender{}
	h.SetEmailService(sender)
	form := url.Values{"name": {"Alice"}, "email": {"alice@example.test"}, "subject": {"Question"}, "message": {"Hello"}, "type": {"privacy"}, "request_kind": {"export"}}
	rr := submitAndConfirmHTMLContact(t, h, "/server/contact", form)
	if rr.Code != http.StatusOK {
		t.Fatalf("contact submit status=%d body=%s", rr.Code, rr.Body.String())
	}
	if len(sender.calls) != 1 {
		t.Fatalf("sent %d contact emails, want exactly one", len(sender.calls))
	}
	call := sender.calls[0]
	if call.to != "contact@example.test" || call.vars["reply_to"] != "alice@example.test" || call.vars["subject"] != "Question" {
		t.Fatalf("unexpected contact delivery: %#v", call)
	}
}

func TestAPIContactDeliveryFailureReleasesRetryState(t *testing.T) {
	h, _ := newSecurityTestHandler(t)
	h.appConfig.Server.Pages.Contact.Enabled = true
	h.appConfig.Server.Contact.General.Email = "contact@example.test"
	h.appConfig.Server.Pages.Contact.Antispam.ReviewMinSeconds = 0
	sender := &recordingSecurityReportEmailSender{contactErr: errors.New("smtp unavailable")}
	h.SetEmailService(sender)
	for attempt := 0; attempt < 2; attempt++ {
		form := url.Values{"name": {"Alice"}, "email": {"alice@example.test"}, "subject": {"Question"}, "message": {"Please reply"}, "type": {"privacy"}, "request_kind": {"export"}}
		response := submitAPIContactConfirmation(t, h, "/api/v1/server/contact", form)
		if response.Code != http.StatusInternalServerError {
			t.Fatalf("attempt %d status=%d, want 500; body=%s", attempt+1, response.Code, response.Body.String())
		}
	}
	if len(sender.calls) != 2 {
		t.Fatalf("sent %d attempts, want 2", len(sender.calls))
	}
}

func submitAPIContactConfirmation(t *testing.T, h *ServerHandler, path string, form url.Values) *httptest.ResponseRecorder {
	t.Helper()
	h.appConfig.Server.Pages.Contact.Antispam.ReviewMinSeconds = 0
	addSimpleCaptcha(t, h, form)
	form.Set("step", "submit")
	submit := httptest.NewRequest(http.MethodPost, path, strings.NewReader(form.Encode()))
	submit.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	submitResponse := httptest.NewRecorder()
	h.APIContact(submitResponse, submit)
	if submitResponse.Code != http.StatusOK {
		t.Fatalf("review step status=%d body=%s", submitResponse.Code, submitResponse.Body.String())
	}
	var envelope struct {
		Data struct {
			ReviewToken string `json:"review_token"`
		} `json:"data"`
	}
	if err := json.Unmarshal(submitResponse.Body.Bytes(), &envelope); err != nil {
		t.Fatalf("decode review response: %v", err)
	}
	if envelope.Data.ReviewToken == "" {
		t.Fatalf("review response missing token: %s", submitResponse.Body.String())
	}
	form.Set("step", "confirm")
	form.Set("confirmed", "true")
	form.Set("review_token", envelope.Data.ReviewToken)
	confirm := httptest.NewRequest(http.MethodPost, path, strings.NewReader(form.Encode()))
	confirm.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	response := httptest.NewRecorder()
	h.APIContact(response, confirm)
	return response
}

// newSecurityTestHandler wires a ServerHandler with a real tempdir sqlite DB
// (full schema via database.SchemaManager, matching production) and a
// secret.Manager, so the security_id validation and CreateReport paths
// exercise real code rather than mocks.
func newSecurityTestHandler(t *testing.T) (*ServerHandler, string) {
	t.Helper()
	setRealTemplatesFS(t)

	dbPath := t.TempDir() + "/test.db"
	sm, err := database.NewSchemaManager(dbPath)
	if err != nil {
		t.Fatalf("NewSchemaManager: %v", err)
	}
	if err := sm.EnsureSchema(); err != nil {
		t.Fatalf("EnsureSchema: %v", err)
	}
	t.Cleanup(func() { _ = sm.Close() })

	db := sm.GetAppDatabase()
	secretsMgr := secret.NewManager(db)
	if err := secretsMgr.EnsureSecrets(context.Background()); err != nil {
		t.Fatalf("EnsureSecrets: %v", err)
	}

	h := NewServerHandler(config.DefaultAppConfig())
	h.SetDatabase(db)
	h.SetSecretsManager(secretsMgr)
	h.SetConfigDir(t.TempDir())

	secret, err := secretsMgr.GetInstallationSecret(context.Background())
	if err != nil {
		t.Fatalf("GetInstallationSecret: %v", err)
	}
	securityID := secreport.GenerateSecurityID(secret, time.Now())
	return h, securityID
}

func TestAPISecurityReturnsPublicOverviewContract(t *testing.T) {
	t.Setenv("DOMAIN", "example.com")
	h, _ := newSecurityTestHandler(t)
	h.appConfig.Web.Security.ReportURL = "https://github.com/apimgr/vidveil/security/advisories/new"
	h.appConfig.Web.Security.Contact = "mailto:security@example.test"
	h.appConfig.Server.Pages.Contact.Enabled = true
	h.appConfig.Web.Security.Expires = "2030-01-01T00:00:00Z"
	request := httptest.NewRequest(http.MethodGet, "https://example.com/api/v1/server/security", nil)
	request.Host = "example.com"
	response := httptest.NewRecorder()
	h.APISecurity(response, request)
	if response.Code != http.StatusOK {
		t.Fatalf("APISecurity status = %d, want 200: %s", response.Code, response.Body.String())
	}
	var payload struct {
		OK   bool `json:"ok"`
		Data struct {
			Contacts []struct {
				Type  string `json:"type"`
				Value string `json:"value"`
			} `json:"contacts"`
			Expires string `json:"expires"`
			PGP     struct {
				Available   bool    `json:"available"`
				URL         *string `json:"url"`
				Fingerprint *string `json:"fingerprint"`
			} `json:"pgp"`
			PolicyURL string `json:"policy_url"`
			ThanksURL string `json:"thanks_url"`
			ReportURL string `json:"report_url"`
		} `json:"data"`
	}
	if err := json.Unmarshal(response.Body.Bytes(), &payload); err != nil {
		t.Fatalf("decode API security response: %v", err)
	}
	if !payload.OK || len(payload.Data.Contacts) != 3 || payload.Data.Contacts[0].Type != "github" || payload.Data.Contacts[1].Type != "contact_form" || payload.Data.Contacts[2].Type != "email" {
		t.Fatalf("security response contacts = %#v", payload.Data.Contacts)
	}
	if payload.Data.Expires != "2030-01-01T00:00:00Z" || payload.Data.PGP.Available || payload.Data.PGP.URL != nil || payload.Data.PGP.Fingerprint != nil {
		t.Fatalf("security response optional metadata = %#v", payload.Data)
	}
	if payload.Data.PolicyURL != "https://example.com/server/security/policy" || payload.Data.ThanksURL != "https://example.com/server/security/thanks" || payload.Data.ReportURL != "https://example.com/server/contact" {
		t.Fatalf("security response links = %#v", payload.Data)
	}
}

func TestAPISecurityPolicyReturnsMarkdownSource(t *testing.T) {
	h, _ := newSecurityTestHandler(t)
	h.appConfig.Web.Security.PolicyText = "# Operator policy\n\nReport privately."
	response := httptest.NewRecorder()
	h.APISecurityPolicy(response, httptest.NewRequest(http.MethodGet, "/api/v1/server/security/policy", nil))
	var payload struct {
		OK   bool `json:"ok"`
		Data struct {
			Policy string `json:"policy"`
		} `json:"data"`
	}
	if err := json.Unmarshal(response.Body.Bytes(), &payload); err != nil {
		t.Fatalf("decode API security policy: %v", err)
	}
	if response.Code != http.StatusOK || !payload.OK || payload.Data.Policy != h.appConfig.Web.Security.PolicyText {
		t.Fatalf("policy response = %d %#v", response.Code, payload)
	}
}

func TestAPISecurityThanksReturnsEmptyAndOrderedRows(t *testing.T) {
	h, _ := newSecurityTestHandler(t)
	response := httptest.NewRecorder()
	h.APISecurityThanks(response, httptest.NewRequest(http.MethodGet, "/api/v1/server/security/thanks", nil))
	var empty struct {
		OK   bool `json:"ok"`
		Data struct {
			Entries []map[string]string `json:"entries"`
		} `json:"data"`
	}
	if err := json.Unmarshal(response.Body.Bytes(), &empty); err != nil {
		t.Fatalf("decode empty thanks response: %v", err)
	}
	if response.Code != http.StatusOK || !empty.OK || empty.Data.Entries == nil || len(empty.Data.Entries) != 0 {
		t.Fatalf("empty thanks response = %d %#v", response.Code, empty)
	}
	if _, err := h.db.Exec(`INSERT INTO security_thanks (tracking_id, credit, severity, disclosed_on) VALUES (?, ?, ?, ?)`, "sec_0123456789abcdef", "Researcher", "high", "2026-10-15"); err != nil {
		t.Fatalf("insert thanks fixture: %v", err)
	}
	response = httptest.NewRecorder()
	h.APISecurityThanks(response, httptest.NewRequest(http.MethodGet, "/api/v1/server/security/thanks", nil))
	var populated struct {
		Data struct {
			Entries []map[string]string `json:"entries"`
		} `json:"data"`
	}
	if err := json.Unmarshal(response.Body.Bytes(), &populated); err != nil {
		t.Fatalf("decode populated thanks response: %v", err)
	}
	if response.Code != http.StatusOK || len(populated.Data.Entries) != 1 || populated.Data.Entries[0]["credit"] != "Researcher" || populated.Data.Entries[0]["date"] != "2026-10-15" || populated.Data.Entries[0]["severity"] != "high" {
		t.Fatalf("populated thanks response = %d %#v", response.Code, populated)
	}
}

func TestValidSecurityIDAcceptsPreviousInstallationSecretDuringGrace(t *testing.T) {
	h, previousID := newSecurityTestHandler(t)
	if err := h.secretsMgr.Rotate(context.Background(), secret.InstallationSecret); err != nil {
		t.Fatalf("rotate installation secret: %v", err)
	}
	if !h.validSecurityID(httptest.NewRequest(http.MethodGet, "/", nil), previousID) {
		t.Fatal("security_id signed with the previous installation secret should remain valid during the grace period")
	}
}

func TestValidSecurityIDRejectsExpiredPreviousInstallationSecret(t *testing.T) {
	h, previousID := newSecurityTestHandler(t)
	if err := h.secretsMgr.Rotate(context.Background(), secret.InstallationSecret); err != nil {
		t.Fatalf("rotate installation secret: %v", err)
	}
	if _, err := h.db.Exec(`UPDATE app_secrets SET expires_at = ? WHERE key = ?`, time.Now().Add(-time.Hour), string(secret.InstallationSecret)); err != nil {
		t.Fatalf("expire previous installation secret: %v", err)
	}
	if h.validSecurityID(httptest.NewRequest(http.MethodGet, "/", nil), previousID) {
		t.Fatal("security_id signed with an expired previous installation secret must be rejected")
	}
}

// ContactPage GET with a valid security_id switches to security mode and
// exposes the Affected Component dropdown options.
func TestServerHandler_ContactPage_SecurityMode_ShowsComponentDropdown(t *testing.T) {
	h, securityID := newSecurityTestHandler(t)
	req := httptest.NewRequest(http.MethodGet, "/server/contact?security_id="+securityID, nil)
	req.Header.Set("User-Agent", "Mozilla/5.0")
	rr := httptest.NewRecorder()
	h.ContactPage(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("ContactPage security mode: status=%d want 200; body=%s", rr.Code, rr.Body.String())
	}
	body := rr.Body.String()
	if !strings.Contains(body, `name="component"`) || !strings.Contains(body, "<select") {
		t.Errorf("ContactPage security mode: expected a component <select>, got body=%s", body)
	}
	if !strings.Contains(body, `value="other"`) {
		t.Errorf("ContactPage security mode: expected an Other option, got body=%s", body)
	}
	if !strings.Contains(body, `name="component_other"`) {
		t.Errorf("ContactPage security mode: expected a component_other free-text field, got body=%s", body)
	}
}

func securityFormValues(component, componentOther string) url.Values {
	v := url.Values{}
	v.Set("name", "Security Researcher")
	v.Set("email", "researcher@example.com")
	v.Set("component", component)
	if componentOther != "" {
		v.Set("component_other", componentOther)
	}
	v.Set("severity", "high")
	v.Set("summary", "test summary")
	v.Set("steps", "1. do a thing")
	v.Set("impact", "denial of service")
	v.Set("credit_preference", "anonymous")
	v.Set("disclosure_agreement", "1")
	return v
}

func TestSecurityContactValidationRejectsInvalidEnumsAndEmail(t *testing.T) {
	for _, tc := range []struct {
		name  string
		field string
		value string
		want  string
	}{
		{name: "severity", field: "severity", value: "urgent", want: "severity"},
		{name: "credit preference", field: "credit_preference", value: "public", want: "credit_preference"},
		{name: "email", field: "email", value: "not-an-email", want: "email"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			form := securityFormValues("API", "")
			form.Set(tc.field, tc.value)
			request := httptest.NewRequest(http.MethodPost, "/server/contact", strings.NewReader(form.Encode()))
			request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
			if got := securityContactValidationError(request); got != tc.want {
				t.Errorf("securityContactValidationError() = %q, want %q", got, tc.want)
			}
		})
	}
}

// handleSecurityReportSubmit accepts a dropdown-selected component value directly.
func TestHandleSecurityReportSubmit_DropdownComponent_Succeeds(t *testing.T) {
	h, securityID := newSecurityTestHandler(t)
	h.SetEmailService(email.NewEmailService(h.appConfig))
	form := securityFormValues("API", "")
	rr := submitAndConfirmHTMLContact(t, h, "/server/contact?security_id="+securityID, form)
	if rr.Code != http.StatusOK {
		t.Fatalf("handleSecurityReportSubmit dropdown: status=%d want 200; body=%s", rr.Code, rr.Body.String())
	}
	if !strings.Contains(rr.Body.String(), "Tracking ID") {
		t.Errorf("handleSecurityReportSubmit dropdown: expected success message with tracking id, got body=%s", rr.Body.String())
	}
	if strings.Contains(rr.Body.String(), "Check your email") {
		t.Errorf("failed acknowledgment must not promise email delivery, got body=%s", rr.Body.String())
	}
}

func TestSecurityReportSendsSingleMaintainerNoticeAndOptionalAcknowledgment(t *testing.T) {
	for _, tc := range []struct {
		name        string
		acknowledge error
		wantEmail   bool
	}{
		{name: "acknowledgment sent", wantEmail: true},
		{name: "acknowledgment failure is nonfatal", acknowledge: errors.New("SMTP unavailable")},
	} {
		t.Run(tc.name, func(t *testing.T) {
			h, securityID := newSecurityTestHandler(t)
			h.appConfig.Server.Contact.Security.Email = "security@example.com"
			sender := &recordingSecurityReportEmailSender{acknowledge: tc.acknowledge}
			h.SetEmailService(sender)
			form := securityFormValues("API", "")
			rr := submitAndConfirmHTMLContact(t, h, "/server/contact?security_id="+securityID, form)
			if rr.Code != http.StatusOK {
				t.Fatalf("security report submit status=%d, want 200; body=%s", rr.Code, rr.Body.String())
			}
			if len(sender.calls) != 2 {
				t.Fatalf("email attempts = %d, want one maintainer notice and one acknowledgment", len(sender.calls))
			}
			if !strings.HasPrefix(sender.calls[0].template, "attachment:") || sender.calls[0].to != "security@example.com" {
				t.Errorf("maintainer email = %+v", sender.calls[0])
			}
			if !strings.Contains(sender.calls[0].vars["body"], "AES-256-GCM ciphertext") || strings.Contains(sender.calls[0].vars["body"], "Steps to reproduce") || sender.calls[0].vars["payload"] == "" {
				t.Errorf("maintainer notice must carry only the warning and encrypted attachment: %+v", sender.calls[0])
			}
			ack := sender.calls[1]
			if ack.template != "security_report_researcher_ack" || ack.to != "researcher@example.com" {
				t.Errorf("researcher acknowledgment = %+v", ack)
			}
			if len(ack.vars) != 2 || ack.vars["tracking_id"] == "" || !strings.Contains(ack.vars["report_status_url"], "/server/security/report/") {
				t.Errorf("acknowledgment variables must contain only tracking_id and status URL: %+v", ack.vars)
			}
			if got := strings.Contains(rr.Body.String(), "Check your email"); got != tc.wantEmail {
				t.Errorf("email delivery message present = %t, want %t; body=%s", got, tc.wantEmail, rr.Body.String())
			}
		})
	}
}

func TestSecurityReportDoesNotSendEmailBeforeCaptchaPasses(t *testing.T) {
	h, securityID := newSecurityTestHandler(t)
	sender := &recordingSecurityReportEmailSender{}
	h.SetEmailService(sender)
	form := securityFormValues("API", "")
	req := httptest.NewRequest(http.MethodPost, "/server/contact?security_id="+securityID, strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	addContactOriginEvidence(req)
	rr := httptest.NewRecorder()
	h.ContactPage(rr, req)
	if len(sender.calls) != 0 {
		t.Fatalf("email attempts before captcha passed = %d, want 0", len(sender.calls))
	}
	if strings.Contains(rr.Body.String(), "Tracking ID") {
		t.Fatalf("captcha failure unexpectedly created a report: %s", rr.Body.String())
	}
}

// handleSecurityReportSubmit falls back to component_other when "other" is selected.
func TestHandleSecurityReportSubmit_OtherComponent_UsesFreeTextFallback(t *testing.T) {
	h, securityID := newSecurityTestHandler(t)
	form := securityFormValues("other", "Custom Engine Plugin")
	rr := submitAndConfirmHTMLContact(t, h, "/server/contact?security_id="+securityID, form)
	if rr.Code != http.StatusOK {
		t.Fatalf("handleSecurityReportSubmit other: status=%d want 200; body=%s", rr.Code, rr.Body.String())
	}

	var component string
	if err := h.db.QueryRow(`SELECT component FROM security_reports ORDER BY created_at DESC LIMIT 1`).Scan(&component); err != nil {
		t.Fatalf("query stored component: %v", err)
	}
	if component != "Custom Engine Plugin" {
		t.Errorf("stored component = %q, want %q (free-text fallback)", component, "Custom Engine Plugin")
	}
}

// handleSecurityReportSubmit rejects the submission when "other" is selected
// but no free-text value is supplied.
func TestHandleSecurityReportSubmit_OtherComponentEmpty_Rejected(t *testing.T) {
	h, securityID := newSecurityTestHandler(t)
	form := securityFormValues("other", "")
	addSimpleCaptcha(t, h, form)
	req := httptest.NewRequest(http.MethodPost, "/server/contact?security_id="+securityID, strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	addContactOriginEvidence(req)
	rr := httptest.NewRecorder()
	h.ContactPage(rr, req)
	if rr.Code != http.StatusBadRequest {
		t.Fatalf("status=%d want 400; body=%s", rr.Code, rr.Body.String())
	}
	if !strings.Contains(rr.Body.String(), "complete all required fields") {
		t.Errorf("expected required-fields error message, got body=%s", rr.Body.String())
	}
}

// apiSecurityReportSubmit (JSON API path) also resolves "other" via component_other.
func TestAPIContact_SecurityMode_OtherComponent_UsesFreeTextFallback(t *testing.T) {
	h, securityID := newSecurityTestHandler(t)
	form := securityFormValues("other", "Bespoke Backend Service")
	rr := submitAPIContactConfirmation(t, h, "/api/v1/server/contact?security_id="+securityID, form)
	if rr.Code != http.StatusOK {
		t.Fatalf("APIContact security mode: status=%d want 200; body=%s", rr.Code, rr.Body.String())
	}
	if !strings.Contains(rr.Body.String(), "tracking_id") {
		t.Errorf("APIContact security mode: expected tracking_id in response, got body=%s", rr.Body.String())
	}

	var component string
	if err := h.db.QueryRow(`SELECT component FROM security_reports ORDER BY created_at DESC LIMIT 1`).Scan(&component); err != nil {
		t.Fatalf("query stored component: %v", err)
	}
	if component != "Bespoke Backend Service" {
		t.Errorf("stored component = %q, want %q (free-text fallback)", component, "Bespoke Backend Service")
	}
}

// SecurityPage, SecurityPolicyPage, SecurityThanksPage all render successfully.
func TestSecurityPages_Render200(t *testing.T) {
	setRealTemplatesFS(t)
	h := newServerHandler(t)

	for _, tc := range []struct {
		name string
		fn   func(http.ResponseWriter, *http.Request)
		path string
	}{
		{"SecurityPage", h.SecurityPage, "/server/security"},
		{"SecurityPolicyPage", h.SecurityPolicyPage, "/server/security/policy"},
		{"SecurityThanksPage", h.SecurityThanksPage, "/server/security/thanks"},
	} {
		req := httptest.NewRequest(http.MethodGet, tc.path, nil)
		rr := httptest.NewRecorder()
		tc.fn(rr, req)
		if rr.Code != http.StatusOK {
			t.Errorf("%s: status=%d want 200; body=%s", tc.name, rr.Code, rr.Body.String())
		}
	}
}

// SecurityReportStatusPage returns 404 without a valid tracking id/token, and
// 200 with a real one created via the store.
func TestSecurityReportStatusPage(t *testing.T) {
	h, securityID := newSecurityTestHandler(t)

	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("tracking_id", "sec_doesnotexist")
	req := httptest.NewRequest(http.MethodGet, "/server/security/report/sec_doesnotexist?token=x", nil)
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))
	rr := httptest.NewRecorder()
	start := time.Now()
	h.SecurityReportStatusPage(rr, req)
	if elapsed := time.Since(start); elapsed < authpad.Floor {
		t.Errorf("unknown tracking ID response took %s, want at least %s", elapsed, authpad.Floor)
	}
	if rr.Code != http.StatusNotFound {
		t.Errorf("unknown tracking id: status=%d want 404", rr.Code)
	}

	idWindow, validSecurityID := h.securityIDWindow(httptest.NewRequest(http.MethodPost, "/server/contact", nil), securityID)
	if !validSecurityID {
		t.Fatal("test security ID failed validation")
	}
	sub, err := secreport.CreateReport(context.Background(), h.db, h.configDir, h.appConfig.Server.Security.EncryptionKey, secreport.Input{
		IDWindow:         idWindow,
		Severity:         "low",
		Component:        "API",
		Body:             []byte("body"),
		CreditPreference: "none",
	})
	if err != nil {
		t.Fatalf("CreateReport: %v", err)
	}

	rctx = chi.NewRouteContext()
	rctx.URLParams.Add("tracking_id", sub.TrackingID)
	req = httptest.NewRequest(http.MethodGet, "/server/security/report/"+sub.TrackingID+"?token="+sub.ReportToken, nil)
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))
	rr = httptest.NewRecorder()
	start = time.Now()
	h.SecurityReportStatusPage(rr, req)
	if elapsed := time.Since(start); elapsed < authpad.Floor {
		t.Errorf("valid status response took %s, want at least %s", elapsed, authpad.Floor)
	}
	if rr.Code != http.StatusOK {
		t.Errorf("valid tracking id/token: status=%d want 200; body=%s", rr.Code, rr.Body.String())
	}
}

func addContactOriginEvidence(req *http.Request) {
	req.Header.Set("Accept", "text/html")
	req.Header.Set("User-Agent", "Mozilla/5.0")
	req.Header.Set("Origin", contactRequestOrigin(req))
	req.Header.Set("Sec-Fetch-Site", "same-origin")
}

func submitAndConfirmHTMLContact(t *testing.T, h *ServerHandler, path string, form url.Values) *httptest.ResponseRecorder {
	t.Helper()
	h.appConfig.Server.Pages.Contact.Antispam.ReviewMinSeconds = 0
	addSimpleCaptcha(t, h, form)
	form.Set("step", "submit")
	submit := httptest.NewRequest(http.MethodPost, path, strings.NewReader(form.Encode()))
	submit.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	addContactOriginEvidence(submit)
	submitResponse := httptest.NewRecorder()
	h.ContactPage(submitResponse, submit)
	if submitResponse.Code != http.StatusOK {
		t.Fatalf("review step status=%d body=%s", submitResponse.Code, submitResponse.Body.String())
	}
	reviewToken := regexp.MustCompile(`name="review_token" value="([^"]+)"`).FindSubmatch(submitResponse.Body.Bytes())
	if len(reviewToken) != 2 {
		t.Fatalf("review response did not contain a token: %s", submitResponse.Body.String())
	}
	form.Set("step", "confirm")
	form.Set("confirmed", "true")
	form.Set("review_token", string(reviewToken[1]))
	confirm := httptest.NewRequest(http.MethodPost, path, strings.NewReader(form.Encode()))
	confirm.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	addContactOriginEvidence(confirm)
	response := httptest.NewRecorder()
	h.ContactPage(response, confirm)
	return response
}
