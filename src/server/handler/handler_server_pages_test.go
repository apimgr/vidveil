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
