// SPDX-License-Identifier: MIT
// AI.md PART 28: Coverage tests for renderServerTemplate main body (handler/server.go lines 90-125).
// Uses os.DirFS("..") to provide the real template filesystem from within the handler package.
package handler

import (
	"html/template"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/apimgr/vidveil/src/config"
	"github.com/apimgr/vidveil/src/notify"
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

// A valid contact submission dispatches to the general role via the wired
// notify.Dispatcher.
func TestServerHandler_ContactSubmit_DispatchesGeneral(t *testing.T) {
	setRealTemplatesFS(t)

	received := make(chan struct{}, 1)
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		select {
		case received <- struct{}{}:
		default:
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer ts.Close()

	h := newServerHandler(t)
	contact := &config.ContactConfig{
		General: config.ContactRoleConfig{Webhooks: map[string]string{"generic": ts.URL}},
	}
	h.SetNotifyDispatcher(notify.New(contact, "vidveil", "1.0.0", ts.URL))

	req := httptest.NewRequest(http.MethodPost, "/server/contact",
		strings.NewReader("email=a@b.com&subject=hello&message=world"))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rr := httptest.NewRecorder()
	h.ContactPage(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("ContactSubmit: status=%d want 200; body=%s", rr.Code, rr.Body.String())
	}
	if !strings.Contains(rr.Body.String(), "Thank you") {
		t.Errorf("ContactSubmit: expected success message; body=%s", rr.Body.String())
	}
	select {
	case <-received:
	case <-time.After(3 * time.Second):
		t.Error("ContactSubmit: webhook never dispatched")
	}
}

// A submission missing the required message re-renders with an error and does
// not dispatch.
func TestServerHandler_ContactSubmit_MissingFieldsError(t *testing.T) {
	setRealTemplatesFS(t)
	h := newServerHandler(t)

	dispatched := make(chan struct{}, 1)
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		select {
		case dispatched <- struct{}{}:
		default:
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer ts.Close()
	contact := &config.ContactConfig{
		General: config.ContactRoleConfig{Webhooks: map[string]string{"generic": ts.URL}},
	}
	h.SetNotifyDispatcher(notify.New(contact, "vidveil", "1.0.0", ts.URL))

	req := httptest.NewRequest(http.MethodPost, "/server/contact",
		strings.NewReader("subject=hello"))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rr := httptest.NewRecorder()
	h.ContactPage(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("ContactSubmit invalid: status=%d want 200; body=%s", rr.Code, rr.Body.String())
	}
	if !strings.Contains(rr.Body.String(), "subject and a message") {
		t.Errorf("ContactSubmit invalid: expected validation error; body=%s", rr.Body.String())
	}
	select {
	case <-dispatched:
		t.Error("ContactSubmit invalid: must not dispatch on validation failure")
	case <-time.After(300 * time.Millisecond):
	}
}

// A filled honeypot field is treated as spam: no dispatch, but a normal success
// page is returned so the bot cannot detect the rejection.
func TestServerHandler_ContactSubmit_HoneypotSilentlySucceeds(t *testing.T) {
	setRealTemplatesFS(t)
	h := newServerHandler(t)

	dispatched := make(chan struct{}, 1)
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		select {
		case dispatched <- struct{}{}:
		default:
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer ts.Close()
	contact := &config.ContactConfig{
		General: config.ContactRoleConfig{Webhooks: map[string]string{"generic": ts.URL}},
	}
	h.SetNotifyDispatcher(notify.New(contact, "vidveil", "1.0.0", ts.URL))

	req := httptest.NewRequest(http.MethodPost, "/server/contact",
		strings.NewReader("subject=hello&message=world&contact_hp=iamabot"))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rr := httptest.NewRecorder()
	h.ContactPage(rr, req)
	if !strings.Contains(rr.Body.String(), "Thank you") {
		t.Errorf("Honeypot: expected success page; body=%s", rr.Body.String())
	}
	select {
	case <-dispatched:
		t.Error("Honeypot: must not dispatch spam")
	case <-time.After(300 * time.Millisecond):
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

// TermsPage renders the server terms-of-service template.
func TestServerHandler_TermsPage_Returns200(t *testing.T) {
	setRealTemplatesFS(t)
	h := newServerHandler(t)
	req := httptest.NewRequest(http.MethodGet, "/server/terms", nil)
	rr := httptest.NewRecorder()
	h.TermsPage(rr, req)
	if rr.Code != http.StatusOK {
		t.Errorf("TermsPage: status=%d want 200; body=%s", rr.Code, rr.Body.String())
	}
	if ct := rr.Header().Get("Content-Type"); !strings.Contains(ct, "text/html") {
		t.Errorf("TermsPage: Content-Type=%q want text/html", ct)
	}
	if !strings.Contains(rr.Body.String(), "Terms of Service") {
		t.Errorf("TermsPage: body missing 'Terms of Service'")
	}
}

// APITerms JSON path returns the terms document with sections.
func TestAPITerms_JSON_ReturnsSections(t *testing.T) {
	h := newServerHandler(t)
	req := httptest.NewRequest(http.MethodGet, "/api/v1/server/terms", nil)
	rr := httptest.NewRecorder()
	h.APITerms(rr, req)
	if rr.Code != http.StatusOK {
		t.Errorf("APITerms JSON: status=%d want 200", rr.Code)
	}
	body := rr.Body.String()
	if !strings.Contains(body, "terms_version") || !strings.Contains(body, "acceptable_use") {
		t.Errorf("APITerms JSON: body missing expected fields: %s", body)
	}
}

// APITerms text/plain path returns plain text with terms fields.
func TestAPITerms_TextPlain_ReturnsText(t *testing.T) {
	h := newServerHandler(t)
	req := httptest.NewRequest(http.MethodGet, "/api/v1/server/terms", nil)
	req.Header.Set("Accept", "text/plain")
	rr := httptest.NewRecorder()
	h.APITerms(rr, req)
	if rr.Code != http.StatusOK {
		t.Errorf("APITerms text: status=%d want 200", rr.Code)
	}
	if !strings.Contains(rr.Header().Get("Content-Type"), "text/plain") {
		t.Errorf("APITerms text: Content-Type=%q want text/plain", rr.Header().Get("Content-Type"))
	}
	if !strings.Contains(rr.Body.String(), "terms_version:") {
		t.Errorf("APITerms text: body missing 'terms_version:': %s", rr.Body.String())
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

// ── Search page: no-JS / progressive-enhancement render ───────────────────────

// The browser search page MUST render real results into the VISIBLE grid (not
// hidden behind a spinner or confined to <noscript>) so it works without
// JavaScript, and MUST embed the results as an inline JSON payload the JS client
// hydrates from — never gating the first result set on the SSE endpoint. Per
// AI.md PART 14: "JavaScript enhances, it does not enable."
func TestSearchPage_NoJS_RendersResultsInVisibleGrid(t *testing.T) {
	setRealTemplatesFS(t)
	h := &SearchHandler{appConfig: config.DefaultAppConfig()}

	req := httptest.NewRequest(http.MethodGet, "/search?q=test", nil)
	req.Header.Set("User-Agent", "Mozilla/5.0")
	req.Header.Set("Accept", "text/html")
	rr := httptest.NewRecorder()

	data := map[string]interface{}{
		"Title":       "test - VidVeil",
		"Query":       "test",
		"SearchQuery": "test",
		"ResultsJSON": template.JS(`[{"url":"https://example.com/v","title":"Sample Video","source":"testsrc"}]`),
		"Results": []map[string]interface{}{
			{
				"URL":       "https://example.com/v",
				"Title":     "Sample Video",
				"Thumbnail": "https://cdn.example.com/x.jpg",
				"Duration":  "10:00",
				"Source":    "testsrc",
				"Views":     "1000",
			},
		},
		"SearchTime":      int64(42),
		"RelatedSearches": []string{},
		"SpellSuggestion": "",
		"EnginesParam":    "",
		"OpenNewTab":      true,
		"Theme":           "dark",
		"Version":         "test",
		"BuildDateTime":   "2026-01-01",
		"Page":            1,
		"PrevPage":        0,
		"NextPage":        2,
		"HasMore":         true,
		"InfiniteScroll":  false,
	}
	h.renderResponse(rr, req, "search", data)

	if rr.Code != http.StatusOK {
		t.Fatalf("search render: status=%d want 200; body=%s", rr.Code, rr.Body.String())
	}
	body := rr.Body.String()

	// 1. The server-computed result is visible in the grid (no-JS core content).
	if !strings.Contains(body, `id="video-grid"`) || !strings.Contains(body, "Sample Video") {
		t.Errorf("search render: visible #video-grid missing server result; body=%s", body)
	}
	if !strings.Contains(body, "video-card") {
		t.Errorf("search render: no .video-card rendered server-side")
	}
	// 2. The inline JSON payload is present so JS hydrates without a second search.
	if !strings.Contains(body, `id="search-results-data"`) {
		t.Errorf("search render: inline JSON hydration payload missing")
	}
	// 3. The "Connecting to engines" spinner is NOT the default first paint.
	if !strings.Contains(body, `id="initial-loading"`) || !strings.Contains(body, "initial-loading hidden") {
		t.Errorf("search render: initial-loading spinner must start hidden; body=%s", body)
	}
	// 4. The result count is server-rendered (works without JS).
	if !strings.Contains(body, `<span id="result-count">1</span>`) {
		t.Errorf("search render: server-rendered result count missing")
	}
	// 5. There must be no <noscript> gating of the results any longer.
	if strings.Contains(body, "<noscript>") {
		t.Errorf("search render: results must not be confined to <noscript>")
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
