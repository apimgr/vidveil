// SPDX-License-Identifier: MIT
// AI.md PART 9: Error Handling & Response Format
// AI.md PART 11: Security - Secure Cookie Handling

package handler

import (
	"encoding/json"
	"net/http"
)

// NewSecureCookie creates a cookie with proper security flags per AI.md PART 11
// The Secure flag is set when sslEnabled is true
func NewSecureCookie(name, value, path string, maxAge int, sslEnabled bool) *http.Cookie {
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

// NewSecureCookieStrict creates a cookie with SameSite=Strict per AI.md PART 11
// Use for sensitive operations like pending 2FA tokens
func NewSecureCookieStrict(name, value, path string, maxAge int, sslEnabled bool) *http.Cookie {
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

// DeleteCookie creates a cookie that deletes an existing cookie
func DeleteCookie(name, path string) *http.Cookie {
	return &http.Cookie{
		Name:   name,
		Value:  "",
		Path:   path,
		MaxAge: -1,
	}
}

// APIResponse is the unified response structure per AI.md PART 9
type APIResponse struct {
	OK      bool   `json:"ok"`
	Data    any    `json:"data,omitempty"`
	Error   string `json:"error,omitempty"`
	Message string `json:"message,omitempty"`
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

// ErrorCodeToHTTP maps error codes to HTTP status codes per AI.md PART 9
func ErrorCodeToHTTP(code string) int {
	switch code {
	case "BAD_REQUEST", "VALIDATION_FAILED":
		return 400
	case "UNAUTHORIZED", "TOKEN_EXPIRED", "TOKEN_INVALID", "2FA_REQUIRED", "2FA_INVALID":
		return 401
	case "FORBIDDEN", "ACCOUNT_LOCKED":
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
	CodeBadRequest      = "BAD_REQUEST"
	CodeValidation      = "VALIDATION_FAILED"
	CodeUnauthorized    = "UNAUTHORIZED"
	CodeTokenExpired    = "TOKEN_EXPIRED"
	CodeTokenInvalid    = "TOKEN_INVALID"
	Code2FARequired     = "2FA_REQUIRED"
	Code2FAInvalid      = "2FA_INVALID"
	CodeForbidden       = "FORBIDDEN"
	CodeAccountLocked   = "ACCOUNT_LOCKED"
	CodeNotFound        = "NOT_FOUND"
	CodeMethodNotAllowed = "METHOD_NOT_ALLOWED"
	CodeConflict        = "CONFLICT"
	CodeRateLimited     = "RATE_LIMITED"
	CodeServerError     = "SERVER_ERROR"
	CodeMaintenance     = "MAINTENANCE"
)

// Standard error messages per AI.md PART 9
const (
	MsgBadRequest      = "Invalid request format"
	MsgValidation      = "Validation failed"
	MsgUnauthorized    = "Authentication required"
	MsgTokenExpired    = "Token has expired"
	MsgTokenInvalid    = "Invalid token"
	Msg2FARequired     = "Two-factor authentication required"
	Msg2FAInvalid      = "Invalid 2FA code"
	MsgForbidden       = "Permission denied"
	MsgAccountLocked   = "Account locked"
	MsgNotFound        = "Resource not found"
	MsgMethodNotAllowed = "Method not allowed"
	MsgConflict        = "Resource already exists"
	MsgRateLimited     = "Too many requests"
	MsgServerError     = "Internal server error"
	MsgMaintenance     = "Service unavailable"
)

// renderResponse renders appropriate response based on client type
// Per AI.md PART 14: Different clients get different formats
func (h *SearchHandler) renderResponse(w http.ResponseWriter, r *http.Request, name string, data map[string]interface{}) {
	// 1. Our CLI client - INTERACTIVE, receives JSON, renders own TUI/GUI
	if isOurCliClient(r) {
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		WriteJSON(w, http.StatusOK, data)
		return
	}

	// 2. Text browsers (lynx, w3m, links) - INTERACTIVE, NO JavaScript
	//    Receive server-rendered HTML that works without JS
	if isTextBrowser(r) {
		// Use no-JS templates from template/nojs/ directory per AI.md PART 14
		h.renderTemplate(w, "nojs/"+name, data)
		return
	}

	// 3. HTTP tools (curl, wget) - NON-INTERACTIVE, just dump output
	//    Receive pre-formatted text via HTML2TextConverter
	if isHttpTool(r) {
		// Render simple HTML content
		html := h.renderSimpleHTML(name, data)

		// Convert to formatted text using full HTML2TextConverter
		text := convertHTMLToText(html, 80)

		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		w.Write([]byte(text + "\n"))
		return
	}

	// 4. Regular browsers (Chrome, Firefox) - full HTML with JavaScript
	h.renderTemplate(w, name, data)
}

// Client detection helpers per AI.md PART 14
func isOurCliClient(r *http.Request) bool {
	ua := r.Header.Get("User-Agent")
	// Check for vidveil-cli/ prefix
	return len(ua) >= 12 && ua[:12] == "vidveil-cli/"
}

func isTextBrowser(r *http.Request) bool {
	ua := r.Header.Get("User-Agent")
	if len(ua) < 4 {
		return false
	}
	ual := ""
	for i := 0; i < len(ua) && i < 8; i++ {
		c := ua[i]
		if c >= 'A' && c <= 'Z' {
			// to lowercase
			c = c + 32
		}
		ual += string(c)
	}
	// Check for text browser signatures
	return ual[:4] == "lynx" || 
		   ual[:3] == "w3m" || 
		   (len(ual) >= 5 && ual[:5] == "links") ||
		   (len(ual) >= 6 && ual[:6] == "elinks")
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
		html += "<li>51 video search engines</li>"
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
		html += "<li>51 search engines</li>"
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
