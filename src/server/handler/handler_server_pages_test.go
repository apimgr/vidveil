// SPDX-License-Identifier: MIT
// AI.md PART 28: Coverage tests for renderServerTemplate main body (handler/server.go lines 90-125).
// Uses os.DirFS("..") to provide the real template filesystem from within the handler package.
package handler

import (
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"github.com/apimgr/vidveil/src/config"
)

// setRealTemplatesFS installs the on-disk template filesystem for tests that exercise
// server page rendering. The handler package resides at src/server/handler/ so the
// parent directory src/server/ is the root of the template tree.
func setRealTemplatesFS(t *testing.T) {
	t.Helper()
	prev := templatesFS
	SetTemplatesFS(os.DirFS(".."))
	t.Cleanup(func() { templatesFS = prev })
}

func newServerHandler(t *testing.T) *ServerHandler {
	t.Helper()
	return NewServerHandler(config.DefaultAppConfig())
}

func TestServerHandler_AboutPage_Returns200(t *testing.T) {
	setRealTemplatesFS(t)
	h := newServerHandler(t)
	req := httptest.NewRequest(http.MethodGet, "/server/about", nil)
	rr := httptest.NewRecorder()
	h.AboutPage(rr, req)
	if rr.Code != http.StatusOK {
		t.Errorf("AboutPage: status=%d want 200; body=%s", rr.Code, rr.Body.String())
	}
	ct := rr.Header().Get("Content-Type")
	if !strings.Contains(ct, "text/html") {
		t.Errorf("AboutPage: Content-Type=%q want text/html", ct)
	}
}

func TestServerHandler_PrivacyPage_Returns200(t *testing.T) {
	setRealTemplatesFS(t)
	h := newServerHandler(t)
	req := httptest.NewRequest(http.MethodGet, "/server/privacy", nil)
	rr := httptest.NewRecorder()
	h.PrivacyPage(rr, req)
	if rr.Code != http.StatusOK {
		t.Errorf("PrivacyPage: status=%d want 200; body=%s", rr.Code, rr.Body.String())
	}
}

func TestServerHandler_ContactPage_GET_Returns200(t *testing.T) {
	setRealTemplatesFS(t)
	h := newServerHandler(t)
	req := httptest.NewRequest(http.MethodGet, "/server/contact", nil)
	rr := httptest.NewRecorder()
	h.ContactPage(rr, req)
	if rr.Code != http.StatusOK {
		t.Errorf("ContactPage GET: status=%d want 200; body=%s", rr.Code, rr.Body.String())
	}
}

func TestRenderServerTemplate_UnknownTemplate_Returns500(t *testing.T) {
	setRealTemplatesFS(t)
	h := newServerHandler(t)
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rr := httptest.NewRecorder()
	h.renderServerTemplate(rr, req, "nonexistent-template", nil)
	if rr.Code != http.StatusInternalServerError {
		t.Errorf("renderServerTemplate with unknown template: status=%d want 500", rr.Code)
	}
}

// ContactPage POST routes to handleContactSubmit and renders a success response.
func TestServerHandler_ContactPage_POST_Returns200(t *testing.T) {
	setRealTemplatesFS(t)
	h := newServerHandler(t)
	req := httptest.NewRequest(http.MethodPost, "/server/contact", strings.NewReader("subject=hello&message=world"))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rr := httptest.NewRecorder()
	h.ContactPage(rr, req)
	if rr.Code != http.StatusOK {
		t.Errorf("ContactPage POST: status=%d want 200; body=%s", rr.Code, rr.Body.String())
	}
}

// HelpPage renders the server help template.
func TestServerHandler_HelpPage_Returns200(t *testing.T) {
	setRealTemplatesFS(t)
	h := newServerHandler(t)
	req := httptest.NewRequest(http.MethodGet, "/server/help", nil)
	rr := httptest.NewRecorder()
	h.HelpPage(rr, req)
	if rr.Code != http.StatusOK {
		t.Errorf("HelpPage: status=%d want 200; body=%s", rr.Code, rr.Body.String())
	}
}

// APIAbout text/plain path returns plain text with name and version.
func TestAPIAbout_TextPlain_ReturnsText(t *testing.T) {
	h := newServerHandler(t)
	req := httptest.NewRequest(http.MethodGet, "/api/v1/server/about", nil)
	req.Header.Set("Accept", "text/plain")
	rr := httptest.NewRecorder()
	h.APIAbout(rr, req)
	if rr.Code != http.StatusOK {
		t.Errorf("APIAbout text: status=%d want 200", rr.Code)
	}
	if !strings.Contains(rr.Header().Get("Content-Type"), "text/plain") {
		t.Errorf("APIAbout text: Content-Type=%q want text/plain", rr.Header().Get("Content-Type"))
	}
	if !strings.Contains(rr.Body.String(), "name:") {
		t.Errorf("APIAbout text: body missing 'name:': %s", rr.Body.String())
	}
}

// APIPrivacy text/plain path returns plain text with policy fields.
func TestAPIPrivacy_TextPlain_ReturnsText(t *testing.T) {
	h := newServerHandler(t)
	req := httptest.NewRequest(http.MethodGet, "/api/v1/server/privacy", nil)
	req.Header.Set("Accept", "text/plain")
	rr := httptest.NewRecorder()
	h.APIPrivacy(rr, req)
	if rr.Code != http.StatusOK {
		t.Errorf("APIPrivacy text: status=%d want 200", rr.Code)
	}
	if !strings.Contains(rr.Header().Get("Content-Type"), "text/plain") {
		t.Errorf("APIPrivacy text: Content-Type=%q want text/plain", rr.Header().Get("Content-Type"))
	}
	if !strings.Contains(rr.Body.String(), "policy_version:") {
		t.Errorf("APIPrivacy text: body missing 'policy_version:': %s", rr.Body.String())
	}
}

// APIHelp text/plain path returns plain text with endpoint list.
func TestAPIHelp_TextPlain_ReturnsText(t *testing.T) {
	h := newServerHandler(t)
	req := httptest.NewRequest(http.MethodGet, "/api/v1/server/help", nil)
	req.Header.Set("Accept", "text/plain")
	rr := httptest.NewRecorder()
	h.APIHelp(rr, req)
	if rr.Code != http.StatusOK {
		t.Errorf("APIHelp text: status=%d want 200", rr.Code)
	}
	if !strings.Contains(rr.Header().Get("Content-Type"), "text/plain") {
		t.Errorf("APIHelp text: Content-Type=%q want text/plain", rr.Header().Get("Content-Type"))
	}
	if !strings.Contains(rr.Body.String(), "search:") {
		t.Errorf("APIHelp text: body missing 'search:': %s", rr.Body.String())
	}
}

// ── RenderErrorPage (real FS success path) ────────────────────────────────────

// RenderErrorPage with a real template filesystem renders HTML and returns the given status.
func TestRenderErrorPage_RealFS_Returns404WithHTML(t *testing.T) {
	setRealTemplatesFS(t)
	cfg := config.DefaultAppConfig()
	h := &SearchHandler{appConfig: cfg}
	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/missing", nil)

	h.RenderErrorPage(rr, req, http.StatusNotFound, "Not Found", "Resource not found")

	if rr.Code != http.StatusNotFound {
		t.Errorf("RenderErrorPage real FS: status=%d want 404", rr.Code)
	}
	ct := rr.Header().Get("Content-Type")
	if !strings.Contains(ct, "text/html") {
		t.Errorf("RenderErrorPage real FS: Content-Type=%q want text/html", ct)
	}
}

// RenderErrorPage with a real template filesystem also covers the 500 code path.
func TestRenderErrorPage_RealFS_Returns500WithHTML(t *testing.T) {
	setRealTemplatesFS(t)
	cfg := config.DefaultAppConfig()
	h := &SearchHandler{appConfig: cfg}
	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/error", nil)

	h.RenderErrorPage(rr, req, http.StatusInternalServerError, "Server Error", "Internal error")

	if rr.Code != http.StatusInternalServerError {
		t.Errorf("RenderErrorPage real FS 500: status=%d want 500", rr.Code)
	}
}
