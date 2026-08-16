// SPDX-License-Identifier: MIT
// AI.md PART 11: Security Reports — handler-level tests for the
// coordinated-disclosure contact-form submission paths (HTML and API) and
// the Affected Component dropdown/"Other" fallback.
package handler

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/apimgr/vidveil/src/config"
	"github.com/apimgr/vidveil/src/server/service/database"
	"github.com/apimgr/vidveil/src/server/service/secreport"
	"github.com/apimgr/vidveil/src/server/service/secret"
)

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
	t.Cleanup(func() { sm.Close() })

	db := sm.GetDB()
	secretsMgr := secret.NewManager(db)
	if err := secretsMgr.EnsureSecrets(context.Background()); err != nil {
		t.Fatalf("EnsureSecrets: %v", err)
	}

	h := NewServerHandler(config.DefaultAppConfig())
	h.SetDB(db)
	h.SetSecretsManager(secretsMgr)
	h.SetConfigDir(t.TempDir())

	secret, err := secretsMgr.GetInstallationSecret(context.Background())
	if err != nil {
		t.Fatalf("GetInstallationSecret: %v", err)
	}
	securityID := secreport.GenerateSecurityID(secret, time.Now())
	return h, securityID
}

// ContactPage GET with a valid security_id switches to security mode and
// exposes the Affected Component dropdown options.
func TestServerHandler_ContactPage_SecurityMode_ShowsComponentDropdown(t *testing.T) {
	h, securityID := newSecurityTestHandler(t)
	req := httptest.NewRequest(http.MethodGet, "/server/contact?security_id="+securityID, nil)
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

// handleSecurityReportSubmit accepts a dropdown-selected component value directly.
func TestHandleSecurityReportSubmit_DropdownComponent_Succeeds(t *testing.T) {
	h, securityID := newSecurityTestHandler(t)
	form := securityFormValues("API", "")
	req := httptest.NewRequest(http.MethodPost, "/server/contact?security_id="+securityID, strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rr := httptest.NewRecorder()
	h.ContactPage(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("handleSecurityReportSubmit dropdown: status=%d want 200; body=%s", rr.Code, rr.Body.String())
	}
	if !strings.Contains(rr.Body.String(), "Tracking ID") {
		t.Errorf("handleSecurityReportSubmit dropdown: expected success message with tracking id, got body=%s", rr.Body.String())
	}
}

// handleSecurityReportSubmit falls back to component_other when "other" is selected.
func TestHandleSecurityReportSubmit_OtherComponent_UsesFreeTextFallback(t *testing.T) {
	h, securityID := newSecurityTestHandler(t)
	form := securityFormValues("other", "Custom Engine Plugin")
	req := httptest.NewRequest(http.MethodPost, "/server/contact?security_id="+securityID, strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rr := httptest.NewRecorder()
	h.ContactPage(rr, req)
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
	req := httptest.NewRequest(http.MethodPost, "/server/contact?security_id="+securityID, strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rr := httptest.NewRecorder()
	h.ContactPage(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("status=%d want 200; body=%s", rr.Code, rr.Body.String())
	}
	if !strings.Contains(rr.Body.String(), "complete all required fields") {
		t.Errorf("expected required-fields error message, got body=%s", rr.Body.String())
	}
}

// apiSecurityReportSubmit (JSON API path) also resolves "other" via component_other.
func TestAPIContact_SecurityMode_OtherComponent_UsesFreeTextFallback(t *testing.T) {
	h, securityID := newSecurityTestHandler(t)
	form := securityFormValues("other", "Bespoke Backend Service")
	req := httptest.NewRequest(http.MethodPost, "/api/v1/server/contact?security_id="+securityID, strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rr := httptest.NewRecorder()
	h.APIContact(rr, req)
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
	h, _ := newSecurityTestHandler(t)

	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("tracking_id", "sec_doesnotexist")
	req := httptest.NewRequest(http.MethodGet, "/server/security/report/sec_doesnotexist?token=x", nil)
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))
	rr := httptest.NewRecorder()
	h.SecurityReportStatusPage(rr, req)
	if rr.Code != http.StatusNotFound {
		t.Errorf("unknown tracking id: status=%d want 404", rr.Code)
	}

	sub, err := secreport.CreateReport(context.Background(), h.db, h.configDir, h.appConfig.Server.Security.EncryptionKey, secreport.Input{
		Severity:         "low",
		Component:        "API",
		Summary:          "test",
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
	h.SecurityReportStatusPage(rr, req)
	if rr.Code != http.StatusOK {
		t.Errorf("valid tracking id/token: status=%d want 200; body=%s", rr.Code, rr.Body.String())
	}
}
