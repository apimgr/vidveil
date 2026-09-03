// SPDX-License-Identifier: MIT
// AI.md PART 9: Error Handling & Response Format
// AI.md PART 11: Security - Secure Cookie Handling

package handler

import (
	"encoding/json"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/apimgr/vidveil/src/common/i18n"
	"github.com/apimgr/vidveil/src/config"
	"github.com/apimgr/vidveil/src/server/service/urlvar"
)

// injectLocaleData populates Lang and Dir on template data per AI.md PART 30
// (<html lang="{{.Lang}}" dir="{{.Dir}}">). Locale resolution: ?lang= query,
// "lang" cookie, then the first acceptable Accept-Language tag, otherwise
// the default locale.
func injectLocaleData(r *http.Request, data map[string]interface{}) {
	if data == nil || r == nil {
		return
	}
	if _, ok := data["Lang"]; !ok {
		data["Lang"] = resolveLocale(r)
	}
	if _, ok := data["Dir"]; !ok {
		data["Dir"] = i18n.Direction(data["Lang"].(string))
	}
}

// resolveLocale returns the locale for a request. Prefers the value set by
// i18n.LanguageMiddleware in the request context (primary path); falls back to
// direct header/cookie/query inspection when the middleware is not in the chain.
func resolveLocale(r *http.Request) string {
	if loc := i18n.LocaleFromContext(r.Context()); loc != i18n.DefaultLocale {
		return loc
	}
	if v := strings.TrimSpace(r.URL.Query().Get("lang")); v != "" {
		return strings.ToLower(v)
	}
	if c, err := r.Cookie("lang"); err == nil && c.Value != "" {
		return strings.ToLower(c.Value)
	}
	if al := r.Header.Get("Accept-Language"); al != "" {
		// Accept-Language: en-US,en;q=0.9 → "en-us"
		first := al
		if idx := strings.IndexAny(first, ",;"); idx > 0 {
			first = first[:idx]
		}
		first = strings.TrimSpace(first)
		if first != "" {
			return strings.ToLower(first)
		}
	}
	return i18n.DefaultLocale
}

// newSecureCookie creates a cookie with proper security flags per AI.md PART 11
// The Secure flag is set when sslEnabled is true
func newSecureCookie(name, value, path string, maxAge int, sslEnabled bool) *http.Cookie {
	cookie := &http.Cookie{
		Name:     name,
		Value:    value,
		Path:     path,
		MaxAge:   maxAge,
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
		// Per AI.md PART 11: Secure flag when SSL enabled
		Secure: sslEnabled,
	}
	return cookie
}

// newSecureCookieStrict creates a cookie with SameSite=Strict per AI.md PART 11
// Use for sensitive operations like pending 2FA tokens
func newSecureCookieStrict(name, value, path string, maxAge int, sslEnabled bool) *http.Cookie {
	cookie := &http.Cookie{
		Name:     name,
		Value:    value,
		Path:     path,
		MaxAge:   maxAge,
		HttpOnly: true,
		SameSite: http.SameSiteStrictMode,
		Secure:   sslEnabled,
	}
	return cookie
}

// deleteCookie creates a cookie that deletes an existing cookie
func deleteCookie(name, path string) *http.Cookie {
	return &http.Cookie{
		Name:   name,
		Value:  "",
		Path:   path,
		MaxAge: -1,
	}
}

// APIResponse is the unified response structure per AI.md PART 9
// Details carries optional structured context for validation errors and similar
// (e.g. {"field":"email","rule":"format"}). It is omitted when not needed.
type APIResponse struct {
	OK      bool   `json:"ok"`
	Data    any    `json:"data,omitempty"`
	Error   string `json:"error,omitempty"`
	Message string `json:"message,omitempty"`
	Details any    `json:"details,omitempty"`
}

// SendOK sends a success response per AI.md PART 9
func SendOK(w http.ResponseWriter, data any) {
	w.Header().Set("Content-Type", "application/json")
	// Use MarshalIndent with 2-space indentation per PART 14
	response := APIResponse{OK: true, Data: data}
	output, err := json.MarshalIndent(response, "", "  ")
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte(`{"ok":false,"error":"SERVER_ERROR","message":"Failed to encode response"}`))
		w.Write([]byte("\n"))
		return
	}
	w.Write(output)
	w.Write([]byte("\n"))
}

// SendError sends an error response per AI.md PART 9
func SendError(w http.ResponseWriter, code string, message string) {
	status := ErrorCodeToHTTP(code)
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	// Use MarshalIndent with 2-space indentation per PART 14
	response := APIResponse{OK: false, Error: code, Message: message}
	output, err := json.MarshalIndent(response, "", "  ")
	if err != nil {
		w.Write([]byte(`{"ok":false,"error":"SERVER_ERROR","message":"Failed to encode error"}`))
		w.Write([]byte("\n"))
		return
	}
	w.Write(output)
	w.Write([]byte("\n"))
}

// SendNegotiatedError writes an error using the canonical JSON envelope for API
// and JSON clients and a plain-text body for everything else, per AI.md PART 14
// content negotiation. Middleware that runs ahead of routing uses this so an
// /api/** rejection never returns a bare non-canonical body.
func SendNegotiatedError(w http.ResponseWriter, r *http.Request, code string, message string) {
	if strings.HasPrefix(r.URL.Path, "/api/") || strings.Contains(r.Header.Get("Accept"), "application/json") {
		SendError(w, code, message)
		return
	}
	http.Error(w, message, ErrorCodeToHTTP(code))
}

// sendErrorWithDetails sends an error response including the structured details
// object per AI.md PART 14 (e.g. {"field":"email","rule":"format"}).
func sendErrorWithDetails(w http.ResponseWriter, code string, message string, details any) {
	status := ErrorCodeToHTTP(code)
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	response := APIResponse{OK: false, Error: code, Message: message, Details: details}
	output, err := json.MarshalIndent(response, "", "  ")
	if err != nil {
		w.Write([]byte(`{"ok":false,"error":"SERVER_ERROR","message":"Failed to encode error"}`))
		w.Write([]byte("\n"))
		return
	}
	w.Write(output)
	w.Write([]byte("\n"))
}

// ErrorCodeToHTTP maps error codes to HTTP status codes per AI.md PART 9
func ErrorCodeToHTTP(code string) int {
	switch code {
	case "BAD_REQUEST", "VALIDATION_FAILED":
		return 400
	case "UNAUTHORIZED", "TOKEN_EXPIRED", "TOKEN_INVALID", "2FA_REQUIRED", "2FA_INVALID":
		return 401
	case "FORBIDDEN", "ACCOUNT_LOCKED", "CSRF_FAILED":
		return 403
	case "NOT_FOUND":
		return 404
	case "METHOD_NOT_ALLOWED":
		return 405
	case "CONFLICT":
		return 409
	case "RATE_LIMITED":
		return 429
	case "MAINTENANCE":
		return 503
	default:
		return 500
	}
}

// Standard error codes per AI.md PART 9
const (
	CodeBadRequest       = "BAD_REQUEST"
	CodeValidation       = "VALIDATION_FAILED"
	CodeUnauthorized     = "UNAUTHORIZED"
	CodeTokenExpired     = "TOKEN_EXPIRED"
	CodeTokenInvalid     = "TOKEN_INVALID"
	Code2FARequired      = "2FA_REQUIRED"
	Code2FAInvalid       = "2FA_INVALID"
	CodeForbidden        = "FORBIDDEN"
	CodeAccountLocked    = "ACCOUNT_LOCKED"
	CodeNotFound         = "NOT_FOUND"
	CodeMethodNotAllowed = "METHOD_NOT_ALLOWED"
	CodeConflict         = "CONFLICT"
	CodeRateLimited      = "RATE_LIMITED"
	CodeServerError      = "SERVER_ERROR"
	CodeMaintenance      = "MAINTENANCE"
)

// Standard error messages per AI.md PART 9
const (
	MsgBadRequest       = "Invalid request format"
	MsgValidation       = "Validation failed"
	MsgUnauthorized     = "Authentication required"
	MsgTokenExpired     = "Token has expired"
	MsgTokenInvalid     = "Invalid token"
	Msg2FARequired      = "Two-factor authentication required"
	Msg2FAInvalid       = "Invalid 2FA code"
	MsgForbidden        = "Permission denied"
	MsgAccountLocked    = "Account locked"
	MsgNotFound         = "Resource not found"
	MsgMethodNotAllowed = "Method not allowed"
	MsgConflict         = "Resource already exists"
	MsgRateLimited      = "Too many requests"
	MsgServerError      = "Internal server error"
	MsgMaintenance      = "Service unavailable"
)

// renderResponse renders appropriate response based on client type
// Per AI.md PART 14: Different clients get different formats
func (h *SearchHandler) renderResponse(w http.ResponseWriter, r *http.Request, name string, data map[string]interface{}) {
	h.renderResponseStatus(w, r, name, data, http.StatusOK)
}

// renderResponseStatus is renderResponse with an explicit HTTP status code.
// Needed so overload/error conditions detected after a search runs (e.g. the
// RATE_LIMITED envelope returned by EngineManager.SearchWithOperators when the
// searchSem concurrency guard is saturated, AI.md PART 12 "Rate Limiting") can
// surface as a real 429 instead of always answering 200. The status is set via
// an explicit w.WriteHeader(status) before handing off to each client-type
// branch below, so every branch (JSON, text/plain, no-JS HTML, full HTML)
// honors it without needing its own status parameter.
func (h *SearchHandler) renderResponseStatus(w http.ResponseWriter, r *http.Request, name string, data map[string]interface{}, status int) {
	// 1. Our CLI client - INTERACTIVE, receives JSON, renders own TUI/GUI
	if isOurCliClient(r) {
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		WriteJSON(w, status, data)
		return
	}

	// Inject locale and direction per AI.md PART 30 (<html lang="{{.Lang}}" dir="{{.Dir}}">)
	injectLocaleData(r, data)

	// Inject CSRF token per AI.md PART 16 — server-rendered HTML always provides the
	// token in template data so POST forms can include the hidden input automatically.
	if data["CSRFToken"] == nil {
		data["CSRFToken"] = cSRFTokenFromRequest(r)
	}

	// Inject the resolved theme per AI.md PART 16 — header.tmpl's theme toggle
	// is rendered on every page and needs the current theme regardless of
	// whether the individual handler already set it.
	if data["Theme"] == nil {
		data["Theme"] = h.getRequestTheme(r)
	}

	// Inject the next theme in the cycle per AI.md PART 16 "Theme Cycle
	// Logic" — the toggle form's hidden input always posts this computed
	// value so repeated clicks keep cycling instead of sticking after the
	// first submit.
	if data["NextTheme"] == nil {
		if themeStr, ok := data["Theme"].(string); ok {
			data["NextTheme"] = nextTheme(themeStr)
		} else {
			data["NextTheme"] = nextTheme(h.getRequestTheme(r))
		}
	}

	// Resolved per request via BuildURL (AI.md PART 12) — never frozen at
	// startup/config, so og:url/canonical matches the Host/proto the client
	// actually used, including behind a reverse proxy. Set here (where r is
	// in scope) so renderTemplate's own fallback is only ever hit by direct
	// test calls that construct data maps without going through renderResponse.
	if data["AppURL"] == nil {
		data["AppURL"] = urlvar.BuildURL(r, "")
	}

	// The header's <noscript> theme-switch form (AI.md PART 16) posts to
	// /server/preferences/save with return_to=CurrentPath so a no-JS visitor
	// lands back on the page they switched theme from, not always /server/preferences.
	if data["CurrentPath"] == nil {
		data["CurrentPath"] = r.URL.RequestURI()
	}

	// Consent banner gating per AI.md PART 12/16 — the server renders the
	// banner only when no valid cookie_consent cookie exists (zero-JS flow).
	if data["HasConsentCookie"] == nil {
		data["HasConsentCookie"] = hasConsentCookie(r)
	}

	// Consent banner text/links per AI.md PART 12 "Cookie Consent Banner ->
	// Implementation" (server.privacy.consent.*, server.privacy.data.sold).
	// Fallback defaults here so any handler that builds its own data map
	// without going through server.go's renderTemplate() still gets a
	// correctly populated banner instead of missing template keys.
	if h.appConfig != nil {
		privacy := h.appConfig.Server.Privacy
		if data["ConsentMessage"] == nil {
			data["ConsentMessage"] = privacy.GetConsentMessage()
		}
		if data["ConsentPolicyURL"] == nil {
			data["ConsentPolicyURL"] = privacy.Consent.Policy.URL
		}
		if data["ConsentPolicyText"] == nil {
			data["ConsentPolicyText"] = privacy.Consent.Policy.Text
		}
		if data["ConsentDeclineText"] == nil {
			data["ConsentDeclineText"] = privacy.Consent.Buttons.Decline
		}
		if data["ConsentAcceptText"] == nil {
			data["ConsentAcceptText"] = privacy.Consent.Buttons.Accept
		}
		if data["ConsentPreferencesText"] == nil {
			data["ConsentPreferencesText"] = privacy.Consent.PreferencesText
		}
		if data["ConsentDataSold"] == nil {
			data["ConsentDataSold"] = privacy.Data.Sold
		}
	}

	// Site banner per AI.md PART 16 "Site Banner" / PART 25 "Announcements" —
	// server filters to active, undismissed announcements so the shared
	// footer/banner partial has no flash of a dismissed or expired banner.
	if data["Announcements"] == nil {
		data["Announcements"] = activeAnnouncements(h.appConfig, r)
	}

	accept := r.Header.Get("Accept")

	// 2. Accept: text/plain explicitly requested — per AI.md PART 14 returns formatted text
	//    via HTML2TextConverter (same output as HTTP tools), not raw data strings.
	//    Only applies when text/html is NOT also in the Accept header.
	if strings.Contains(accept, "text/plain") && !strings.Contains(accept, "text/html") {
		html := h.renderSimpleHTML(name, data)
		text := convertHTMLToText(html, 80)
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		w.WriteHeader(status)
		w.Write([]byte(text + "\n"))
		return
	}

	// 3. Text browsers (lynx, w3m, links) - INTERACTIVE, NO JavaScript
	//    Receive server-rendered HTML that works without JS
	if isTextBrowser(r) {
		// Use no-JS templates from template/nojs/ directory per AI.md PART 14
		w.WriteHeader(status)
		h.renderTemplate(w, r, "nojs/"+name, data)
		return
	}

	// 4. HTTP tools (curl, wget) - NON-INTERACTIVE, just dump output
	//    Receive pre-formatted text via HTML2TextConverter
	//    Exception: if Accept header explicitly requests text/html, return HTML
	if isHttpTool(r) && !strings.Contains(accept, "text/html") {
		html := h.renderSimpleHTML(name, data)
		text := convertHTMLToText(html, 80)
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		w.WriteHeader(status)
		w.Write([]byte(text + "\n"))
		return
	}

	// 5. Regular browsers (Chrome, Firefox) - full HTML with JavaScript
	w.WriteHeader(status)
	h.renderTemplate(w, r, name, data)
}

// hasConsentCookie reports whether the request carries a non-empty
// cookie_consent cookie (AI.md PART 12 — server skips rendering the banner).
func hasConsentCookie(r *http.Request) bool {
	c, err := r.Cookie("cookie_consent")
	return err == nil && c.Value != ""
}

// BannerAnnouncement is the per-request template view of an active,
// undismissed site-wide announcement (AI.md PART 16 "Site Banner").
type BannerAnnouncement struct {
	ID          string
	Type        string
	Title       string
	Message     string
	Dismissible bool
}

// dismissedAnnouncementIDs parses the dismissed_announcements cookie
// (comma-separated ids, AI.md PART 16 "Site Banner" -> Storage) into a set.
func dismissedAnnouncementIDs(r *http.Request) map[string]bool {
	c, err := r.Cookie("dismissed_announcements")
	if err != nil || c.Value == "" {
		return nil
	}
	value := c.Value
	if unescaped, err := url.QueryUnescape(value); err == nil {
		value = unescaped
	}
	ids := make(map[string]bool)
	for _, id := range strings.Split(value, ",") {
		id = strings.TrimSpace(id)
		if id != "" {
			ids[id] = true
		}
	}
	return ids
}

// activeAnnouncements returns the site-wide announcements to render for this
// request, in config order, per AI.md PART 16 "Site Banner" / PART 25
// "Announcements": disabled/empty config = no banner; each message's
// start/end window (ISO 8601 UTC) gates visibility; ids present in the
// dismissed_announcements cookie are skipped server-side (no flash of a
// dismissed banner, works with zero JS).
func activeAnnouncements(cfg *config.AppConfig, r *http.Request) []BannerAnnouncement {
	if cfg == nil || !cfg.Web.Announcements.Enabled || len(cfg.Web.Announcements.Messages) == 0 {
		return nil
	}
	dismissed := dismissedAnnouncementIDs(r)
	now := time.Now().UTC()
	var active []BannerAnnouncement
	for _, a := range cfg.Web.Announcements.Messages {
		if a.ID == "" || dismissed[a.ID] {
			continue
		}
		if a.Start != "" {
			if start, err := time.Parse(time.RFC3339, a.Start); err == nil && now.Before(start) {
				continue
			}
		}
		if a.End != "" {
			if end, err := time.Parse(time.RFC3339, a.End); err == nil && now.After(end) {
				continue
			}
		}
		active = append(active, BannerAnnouncement{
			ID:          a.ID,
			Type:        a.Type,
			Title:       a.Title,
			Message:     a.Message,
			Dismissible: a.Dismissible,
		})
	}
	return active
}

// Client detection helpers per AI.md PART 14
func isOurCliClient(r *http.Request) bool {
	ua := r.Header.Get("User-Agent")
	// Check for vidveil-cli/ prefix
	return len(ua) >= 12 && ua[:12] == "vidveil-cli/"
}

func isTextBrowser(r *http.Request) bool {
	ua := r.Header.Get("User-Agent")
	if len(ua) == 0 {
		return false
	}
	// Lowercase the UA for prefix matching (ASCII only — UA values are ASCII)
	ual := strings.ToLower(ua)
	// Check for all known text/terminal browser signatures
	return strings.HasPrefix(ual, "lynx") ||
		strings.HasPrefix(ual, "w3m") ||
		strings.HasPrefix(ual, "links") ||
		strings.HasPrefix(ual, "elinks") ||
		strings.HasPrefix(ual, "browsh/") ||
		strings.HasPrefix(ual, "carbonyl/") ||
		strings.HasPrefix(ual, "netsurf")
}

func isHttpTool(r *http.Request) bool {
	ua := r.Header.Get("User-Agent")
	if ua == "" {
		// Empty UA = likely HTTP tool
		return true
	}
	if len(ua) < 4 {
		return false
	}
	ual := ""
	for i := 0; i < len(ua) && i < 7; i++ {
		c := ua[i]
		if c >= 'A' && c <= 'Z' {
			c = c + 32
		}
		ual += string(c)
	}
	return (len(ual) >= 4 && ual[:4] == "curl") ||
		(len(ual) >= 4 && ual[:4] == "wget") ||
		(len(ual) >= 6 && ual[:6] == "httpie")
}

// renderSimpleHTML creates basic HTML for HTTP tools (to be converted to text)
func (h *SearchHandler) renderSimpleHTML(name string, data map[string]interface{}) string {
	html := "<html><body>"

	switch name {
	case "home":
		html += "<h1>VidVeil - Privacy-Respecting Video Search</h1>"
		html += "<p>Search across 51 adult video sites without tracking.</p>"
		html += "<p>Enter a search query or use bang shortcuts like !ph for PornHub.</p>"
		html += "<hr>"
		html += "<h2>Features</h2>"
		html += "<ul>"
		html += "<li>42 video search engines</li>"
		html += "<li>No tracking or logging</li>"
		html += "<li>SSE streaming results</li>"
		html += "<li>Thumbnail proxy</li>"
		html += "<li>Bang shortcuts (!ph, !rt, !xv, etc.)</li>"
		html += "</ul>"
	case "about":
		html += "<h1>About VidVeil</h1>"
		html += "<p>VidVeil is a privacy-respecting meta search engine for adult video content.</p>"
		html += "<h2>Key Features</h2>"
		html += "<ul>"
		html += "<li>No tracking or logging</li>"
		html += "<li>42 search engines</li>"
		html += "<li>SSE streaming results</li>"
		html += "<li>Thumbnail proxy</li>"
		html += "<li>Built-in Tor support</li>"
		html += "<li>Single static binary</li>"
		html += "</ul>"
	case "privacy":
		html += "<h1>Privacy Policy</h1>"
		html += "<p>VidVeil does not track, log, or store any user data.</p>"
		html += "<h2>What We Don't Collect</h2>"
		html += "<ul>"
		html += "<li>No search history</li>"
		html += "<li>No IP addresses</li>"
		html += "<li>No cookies (except essential)</li>"
		html += "<li>No analytics</li>"
		html += "<li>No third-party tracking</li>"
		html += "</ul>"
	case "favorites":
		html += "<h1>Favorites</h1>"
		html += "<p>Favorites are stored locally in your browser (localStorage).</p>"
		html += "<p>Visit /favorites in a browser to view and manage your saved items.</p>"
	case "preferences":
		html += "<h1>Preferences</h1>"
		html += "<p>Customize your search experience.</p>"
		html += "<h2>Available Settings</h2>"
		html += "<ul>"
		html += "<li>Theme (light/dark/auto)</li>"
		html += "<li>Enable/disable engines</li>"
		html += "<li>Results per page</li>"
		html += "<li>Safe search</li>"
		html += "</ul>"
	case "search":
		html += "<h1>Search Results</h1>"
		if query, ok := data["query"].(string); ok {
			html += "<p>Results for: " + htmlEscape(query) + "</p>"
		}
		if results, ok := data["results"].([]interface{}); ok {
			html += "<p>Found " + intToString(len(results)) + " results</p>"
		}
	case "age-verify":
		html += "<h1>Age Verification</h1>"
		html += "<p>You must be 18 or older to use this service.</p>"
		html += "<p>By continuing, you confirm you are of legal age.</p>"
	case "content-restricted":
		html += "<h1>Content Notice</h1>"
		if msg, ok := data["Message"].(string); ok {
			html += "<p>" + htmlEscape(msg) + "</p>"
		}
		if region, ok := data["Region"].(string); ok && region != "" {
			html += "<p>Your detected location: " + htmlEscape(region) + "</p>"
		}
		html += "<p>By continuing, you acknowledge you understand the legal implications.</p>"
	case "content-blocked":
		html += "<h1>Access Restricted</h1>"
		if msg, ok := data["Message"].(string); ok {
			html += "<p>" + htmlEscape(msg) + "</p>"
		}
		if region, ok := data["Region"].(string); ok && region != "" {
			html += "<p>Your detected location: " + htmlEscape(region) + "</p>"
		}
		html += "<p>Access to this service is not available in your region.</p>"
	}

	html += "</body></html>"
	return html
}

// convertHTMLToText uses the full HTML2TextConverter
func convertHTMLToText(html string, width int) string {
	// Simple conversion for now
	text := html

	// H1 with box drawing
	text = replaceAll(text, "<h1>", "\n"+repeatStr("═", width)+"\n")
	text = replaceAll(text, "</h1>", "\n"+repeatStr("═", width)+"\n\n")

	// H2 with line
	text = replaceAll(text, "<h2>", "\n─── ")
	text = replaceAll(text, "</h2>", " ───\n\n")

	// Paragraphs
	text = replaceAll(text, "<p>", "")
	text = replaceAll(text, "</p>", "\n\n")

	// Lists
	text = replaceAll(text, "<ul>", "\n")
	text = replaceAll(text, "</ul>", "\n")
	text = replaceAll(text, "<li>", "  • ")
	text = replaceAll(text, "</li>", "\n")

	// HR
	text = replaceAll(text, "<hr>", "\n"+repeatStr("─", width)+"\n\n")

	// Strip remaining tags
	text = replaceAll(text, "<html>", "")
	text = replaceAll(text, "</html>", "")
	text = replaceAll(text, "<body>", "")
	text = replaceAll(text, "</body>", "")

	return text
}

// Helper functions
func htmlEscape(s string) string {
	result := ""
	for i := 0; i < len(s); i++ {
		c := s[i]
		switch c {
		case '<':
			result += "&lt;"
		case '>':
			result += "&gt;"
		case '&':
			result += "&amp;"
		case '"':
			result += "&quot;"
		default:
			result += string(c)
		}
	}
	return result
}

func intToString(n int) string {
	if n == 0 {
		return "0"
	}
	negative := n < 0
	if negative {
		n = -n
	}
	digits := ""
	for n > 0 {
		digits = string(rune('0'+(n%10))) + digits
		n /= 10
	}
	if negative {
		digits = "-" + digits
	}
	return digits
}

func repeatStr(s string, n int) string {
	result := ""
	for i := 0; i < n; i++ {
		result += s
	}
	return result
}

func replaceAll(s, old, new string) string {
	result := ""
	for {
		i := indexOf(s, old)
		if i == -1 {
			result += s
			break
		}
		result += s[:i] + new
		s = s[i+len(old):]
	}
	return result
}

func indexOf(s, substr string) int {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return i
		}
	}
	return -1
}
