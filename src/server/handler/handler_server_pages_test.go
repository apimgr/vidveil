// SPDX-License-Identifier: MIT
// AI.md PART 28: Coverage tests for renderServerTemplate main body (handler/server.go lines 90-125).
// Uses os.DirFS("..") to provide the real template filesystem from within the handler package.
package handler

import (
	"html/template"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/apimgr/vidveil/src/config"
	"github.com/apimgr/vidveil/src/notify"
)

func simpleCaptchaFormValues(t *testing.T, h *ServerHandler, r *http.Request) string {
	t.Helper()
	id, question := h.newSimpleCaptcha(r)
	parts := strings.Fields(question)
	if len(parts) < 3 {
		t.Fatalf("invalid captcha question %q", question)
	}
	left, err := strconv.Atoi(parts[0])
	if err != nil {
		t.Fatal(err)
	}
	right, err := strconv.Atoi(parts[2])
	if err != nil {
		t.Fatal(err)
	}
	return "captcha_id=" + url.QueryEscape(id) + "&captcha_answer=" + strconv.Itoa(left+right)
}

func addSimpleCaptcha(t *testing.T, h *ServerHandler, form url.Values) {
	t.Helper()
	values, err := url.ParseQuery(simpleCaptchaFormValues(t, h, httptest.NewRequest(http.MethodGet, "/server/contact", nil)))
	if err != nil {
		t.Fatal(err)
	}
	for key, value := range values {
		form.Set(key, value[0])
	}
}

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
	req.Header.Set("User-Agent", "Mozilla/5.0")
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

func TestServerHandler_AboutPage_TextBrowserUsesNoJSTemplate(t *testing.T) {
	setRealTemplatesFS(t)
	h := newServerHandler(t)
	req := httptest.NewRequest(http.MethodGet, "/server/about", nil)
	req.Header.Set("User-Agent", "Lynx/2.9.0")
	response := httptest.NewRecorder()
	h.AboutPage(response, req)
	if response.Code != http.StatusOK {
		t.Fatalf("AboutPage text browser: status=%d body=%s", response.Code, response.Body.String())
	}
	if !strings.Contains(response.Body.String(), `href="/server/privacy"`) || strings.Contains(response.Body.String(), "/static/js/app.js") {
		t.Errorf("text-browser server page must retain navigation without JavaScript: %s", response.Body.String())
	}
}

func TestServerHandler_AboutPage_HTTPToolGetsFormattedText(t *testing.T) {
	setRealTemplatesFS(t)
	h := newServerHandler(t)
	req := httptest.NewRequest(http.MethodGet, "/server/about", nil)
	req.Header.Set("User-Agent", "curl/8.0")
	response := httptest.NewRecorder()
	h.AboutPage(response, req)
	if response.Code != http.StatusOK || !strings.HasPrefix(response.Header().Get("Content-Type"), "text/plain") {
		t.Fatalf("AboutPage HTTP tool: status=%d type=%q", response.Code, response.Header().Get("Content-Type"))
	}
	if strings.Contains(response.Body.String(), "<html") || !strings.Contains(response.Body.String(), "About") {
		t.Errorf("HTTP tool response should be formatted plain text: %s", response.Body.String())
	}
}

func TestHealthz_TextBrowserGetsNoJSTemplate(t *testing.T) {
	setRealTemplatesFS(t)
	h := &SearchHandler{appConfig: config.DefaultAppConfig()}
	req := httptest.NewRequest(http.MethodGet, "/server/healthz", nil)
	req.Header.Set("User-Agent", "Lynx/2.9.0")
	response := httptest.NewRecorder()
	h.renderHealthzHTML(response, req, "healthy", http.StatusOK, "development", "1h", "localhost", time.Now().Format(time.RFC3339), map[string]string{
		"database": "ok", "cache": "ok", "disk": "ok", "scheduler": "ok",
	})
	if response.Code != http.StatusOK || strings.Contains(response.Body.String(), "<script") || !strings.Contains(response.Body.String(), "System Health") {
		t.Fatalf("healthz text-browser response: status=%d body=%s", response.Code, response.Body.String())
	}
}

func TestServerHandler_DPOPage_FallsBackToGeneralContact(t *testing.T) {
	setRealTemplatesFS(t)
	cfg := config.DefaultAppConfig()
	cfg.Server.Contact.Admin.Email = "admin@example.invalid"
	cfg.Server.Contact.General.Email = "privacy@example.invalid"
	h := NewServerHandler(cfg)
	req := httptest.NewRequest(http.MethodGet, "/server/dpo", nil)
	req.Header.Set("User-Agent", "Mozilla/5.0")
	rr := httptest.NewRecorder()
	h.DPOPage(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("DPOPage: status=%d want 200; body=%s", rr.Code, rr.Body.String())
	}
	if !strings.Contains(rr.Body.String(), "privacy@example.invalid") || strings.Contains(rr.Body.String(), "admin@example.invalid") {
		t.Errorf("DPOPage did not apply the configured fallback safely: %s", rr.Body.String())
	}
}

func TestServerHandler_DPOPageWithoutContactShowsOnlyContactForm(t *testing.T) {
	setRealTemplatesFS(t)
	cfg := config.DefaultAppConfig()
	cfg.Server.Contact.Admin.Email = "admin@example.invalid"
	cfg.Server.Contact.General.Email = ""
	h := NewServerHandler(cfg)
	req := httptest.NewRequest(http.MethodGet, "/server/dpo", nil)
	req.Header.Set("User-Agent", "Mozilla/5.0")
	rr := httptest.NewRecorder()
	h.DPOPage(rr, req)
	if rr.Code != http.StatusOK || !strings.Contains(rr.Body.String(), "/server/contact") {
		t.Errorf("DPOPage without contact: status=%d body=%s", rr.Code, rr.Body.String())
	}
	if strings.Contains(rr.Body.String(), "admin@example.invalid") {
		t.Errorf("DPOPage exposed the admin address: %s", rr.Body.String())
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

func TestTrackingJavaScriptRequiresConsent(t *testing.T) {
	cfg := config.DefaultAppConfig()
	cfg.Server.Tracking = config.TrackingConfig{Type: "google", ID: "G-ABC123"}
	h := NewServerHandler(cfg)
	req := httptest.NewRequest(http.MethodGet, "/server/tracking.js", nil)
	blocked := httptest.NewRecorder()
	h.TrackingJavaScript(blocked, req)
	if blocked.Code != http.StatusNotFound || blocked.Header().Get("Cache-Control") != "no-cache" {
		t.Fatalf("tracking without consent: status=%d cache=%q", blocked.Code, blocked.Header().Get("Cache-Control"))
	}

	req.AddCookie(&http.Cookie{Name: "cookie_consent", Value: url.QueryEscape(`{"analytics":true}`)})
	allowed := httptest.NewRecorder()
	h.TrackingJavaScript(allowed, req)
	if allowed.Code != http.StatusOK || !strings.Contains(allowed.Header().Get("Content-Type"), "application/javascript") || !strings.Contains(allowed.Body.String(), "G-ABC123") {
		t.Fatalf("tracking with consent: status=%d type=%q body=%q", allowed.Code, allowed.Header().Get("Content-Type"), allowed.Body.String())
	}
}

func TestSimpleCaptchaIsSingleUse(t *testing.T) {
	h := newServerHandler(t)
	request := httptest.NewRequest(http.MethodGet, "/server/contact", nil)
	id, question := h.newSimpleCaptcha(request)
	if id == "" || question == "" {
		t.Fatal("newSimpleCaptcha returned an empty challenge")
	}
	parts := strings.Split(question, " ")
	if len(parts) != 5 {
		t.Fatalf("unexpected challenge question %q", question)
	}
	answer := ""
	left, leftErr := strconv.Atoi(parts[0])
	right, rightErr := strconv.Atoi(parts[2])
	if leftErr != nil || rightErr != nil {
		t.Fatalf("invalid arithmetic challenge %q", question)
	}
	answer = strconv.Itoa(left + right)
	form := url.Values{"captcha_id": {id}, "captcha_answer": {answer}}
	first := httptest.NewRequest(http.MethodPost, "/server/contact", strings.NewReader(form.Encode()))
	first.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	if !h.verifyCaptcha(first) {
		t.Fatal("valid captcha answer was rejected")
	}
	second := httptest.NewRequest(http.MethodPost, "/server/contact", strings.NewReader(form.Encode()))
	second.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	if h.verifyCaptcha(second) {
		t.Fatal("captcha challenge was accepted more than once")
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

	form := simpleCaptchaFormValues(t, h, httptest.NewRequest(http.MethodGet, "/server/contact", nil)) + "&name=Alice&email=a%40b.com&subject=hello&message=world"
	req := httptest.NewRequest(http.MethodPost, "/server/contact", strings.NewReader(form))
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

	form := simpleCaptchaFormValues(t, h, httptest.NewRequest(http.MethodGet, "/server/contact", nil)) + "&subject=hello"
	req := httptest.NewRequest(http.MethodPost, "/server/contact", strings.NewReader(form))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rr := httptest.NewRecorder()
	h.ContactPage(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("ContactSubmit invalid: status=%d want 200; body=%s", rr.Code, rr.Body.String())
	}
	if !strings.Contains(rr.Body.String(), "name, a valid email address, a subject, and a message") {
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
	req.Header.Set("User-Agent", "Mozilla/5.0")
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

func TestAPIDPO_ReturnsConfiguredContactAndPrivacyURL(t *testing.T) {
	cfg := config.DefaultAppConfig()
	cfg.Server.Contact.DPO = config.DPOContactConfig{Name: "Privacy Officer", Email: "dpo@example.invalid"}
	h := NewServerHandler(cfg)
	req := httptest.NewRequest(http.MethodGet, "/api/v1/server/dpo", nil)
	req.Header.Set("Accept", "application/json")
	rr := httptest.NewRecorder()
	h.APIDPO(rr, req)
	if rr.Code != http.StatusOK || !strings.Contains(rr.Body.String(), `"name": "Privacy Officer"`) || !strings.Contains(rr.Body.String(), `"email": "dpo@example.invalid"`) || !strings.Contains(rr.Body.String(), `"privacy_url":`) {
		t.Errorf("APIDPO response: status=%d body=%s", rr.Code, rr.Body.String())
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
		"SearchTime":      "42 seconds",
		"SearchSessionID": "test-session",
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
		"PrevURL":         "/search?page=1&q=test",
		"NextURL":         "/search?page=2&q=test",
		"HasMore":         true,
		"InfiniteScroll":  false,
		"FavoriteURLs":    map[string]bool{},
		"CurrentPath":     "/search?q=test",
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
	if !strings.Contains(body, `href="/search?page=2&amp;q=test"`) {
		t.Errorf("search render: server-rendered next-page link missing")
	}
	if !strings.Contains(body, `data-search-session="test-session"`) {
		t.Errorf("search render: session token for infinite-scroll dedup missing")
	}
	if !strings.Contains(body, `id="search-time-container">in 42 seconds</span>`) {
		t.Errorf("search render: duration should be rendered in human-readable units; body=%s", body)
	}
	// 5. The results themselves must not be gated inside a <noscript> block
	//    (a page-wide <noscript> fallback for an unrelated control, such as
	//    the header's no-JS theme toggle, is legitimate progressive
	//    enhancement and must not trip this check).
	if gridIdx, dataIdx := strings.Index(body, `id="video-grid"`), strings.Index(body, `id="search-results-data"`); gridIdx == -1 || dataIdx == -1 {
		t.Errorf("search render: could not locate video-grid/search-results-data to check <noscript> gating; body=%s", body)
	} else {
		start := gridIdx
		if dataIdx < start {
			start = dataIdx
		}
		end := gridIdx
		if dataIdx > end {
			end = dataIdx
		}
		if strings.Contains(body[start:end], "<noscript>") {
			t.Errorf("search render: results must not be confined to <noscript>")
		}
	}

	// Infinite scroll is an enhancement. A visitor with JavaScript disabled
	// still needs a server-rendered way to reach the next result page.
	data["InfiniteScroll"] = true
	noJSRecorder := httptest.NewRecorder()
	h.renderResponse(noJSRecorder, req, "search", data)
	noJSBody := noJSRecorder.Body.String()
	if !strings.Contains(noJSBody, `<noscript>`) || !strings.Contains(noJSBody, `rel="next"`) || !strings.Contains(noJSBody, `href="/search?page=2&amp;q=test"`) {
		t.Errorf("infinite-scroll preference must retain no-JS next-page navigation; body=%s", noJSBody)
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
