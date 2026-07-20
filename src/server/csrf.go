// SPDX-License-Identifier: MIT
// CSRF middleware — double-submit cookie pattern per AI.md PART 16 → CSRF Protection.
//
// Token lifecycle:
//   - On each request, if no csrf_token cookie exists (or it is invalid), a new token
//     is generated and set as a SameSite=Strict, HttpOnly=false cookie.
//   - On POST/PUT/PATCH/DELETE: the cookie value is compared against the value in the
//     X-CSRF-Token header (or the csrf_token form field). Mismatch → 403 Forbidden.
//   - Bypass conditions (any one bypasses validation):
//   - Method is GET, HEAD, or OPTIONS
//   - Authorization: Bearer … or X-API-Token header is present
//   - Request is a WebSocket upgrade
//   - Path matches an exempt_paths glob in config
//   - No session cookie present (public/unauthenticated request)
//
// Cookie posture:
//   - csrf_token: SameSite=Strict, HttpOnly=false (form JS must read it), Secure per config
//   - session cookie: SameSite=Strict per AI.md PART 16 → Cookie Posture
package server

import (
	"context"
	"crypto/rand"
	"crypto/subtle"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"path/filepath"
	"strings"

	"github.com/apimgr/vidveil/src/config"
	"github.com/apimgr/vidveil/src/server/handler"
	"github.com/apimgr/vidveil/src/server/service/logging"
)

// newCSRFMiddleware returns a middleware that implements the double-submit cookie
// pattern for CSRF protection per AI.md PART 16 → CSRF Protection.
func newCSRFMiddleware(cfg config.CSRFConfig, sessionCookieName string, logger *logging.AppLogger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Determine whether the Secure flag should be set on the CSRF cookie.
			secureCookie := csrfSecureFlag(cfg.Secure, r)

			// Read the existing CSRF cookie value (may be empty).
			existingToken := ""
			if c, err := r.Cookie(cfg.CookieName); err == nil {
				existingToken = c.Value
			}

			// Validate the token — only when all bypass conditions are absent.
			if !csrfBypass(cfg, sessionCookieName, r) {
				if existingToken == "" {
					csrfDeny(w, r, "token_absent", r.URL.Path, logger)
					return
				}
				// Accept token from header or form field (double-submit pattern).
				submitted := r.Header.Get(cfg.HeaderName)
				if submitted == "" {
					// Parse form to get the field value (r.ParseForm is idempotent).
					_ = r.ParseForm()
					submitted = r.FormValue(cfg.CookieName)
				}
				// Constant-time comparison per AI.md PART 11 (CSRF tokens are credentials).
				if submitted == "" || subtle.ConstantTimeCompare([]byte(submitted), []byte(existingToken)) != 1 {
					csrfDeny(w, r, "token_mismatch", r.URL.Path, logger)
					return
				}
			}

			// Ensure a CSRF cookie is always present on the response.
			// Refresh only if missing — do not rotate on every request.
			token := existingToken
			if token == "" {
				token = csrfGenToken(cfg.TokenLength)
				http.SetCookie(w, &http.Cookie{
					Name:     cfg.CookieName,
					Value:    token,
					Path:     "/",
					MaxAge:   0, // session-scoped (no persistent CSRF cookies)
					Secure:   secureCookie,
					HttpOnly: false, // forms must read this value
					SameSite: http.SameSiteStrictMode,
				})
			}

			// Store the token in the request context so templates can embed it.
			// Uses handler.CSRFTokenKey so the handler package can read it without
			// creating a circular import (server already imports handler).
			ctx := context.WithValue(r.Context(), handler.CSRFTokenKey, token)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// csrfBypass returns true when CSRF validation should be skipped for this request.
func csrfBypass(cfg config.CSRFConfig, sessionCookieName string, r *http.Request) bool {
	// Safe methods never mutate state.
	switch r.Method {
	case http.MethodGet, http.MethodHead, http.MethodOptions:
		return true
	}

	// Bearer / API token callers authenticate via credential, not cookie.
	if strings.HasPrefix(r.Header.Get("Authorization"), "Bearer ") ||
		r.Header.Get("X-API-Token") != "" {
		return true
	}

	// WebSocket upgrade: auth happens at the connection level.
	if strings.EqualFold(r.Header.Get("Upgrade"), "websocket") {
		return true
	}

	// Public (unauthenticated) request: no session cookie → no cookie to forge.
	if _, err := r.Cookie(sessionCookieName); err != nil {
		return true
	}

	// Operator-declared exempt paths (OAuth callbacks, webhook receivers).
	for _, pattern := range cfg.ExemptPaths {
		if csrfPathMatches(pattern, r.URL.Path) {
			return true
		}
	}

	return false
}

// csrfPathMatches checks whether path matches the given glob pattern.
// Uses filepath.Match semantics; leading /api/{api_version}/ wildcards are honoured.
func csrfPathMatches(pattern, urlPath string) bool {
	matched, err := filepath.Match(pattern, urlPath)
	if err != nil {
		return false
	}
	return matched
}

// csrfDeny writes a 403 Forbidden with the canonical CSRF error body per PART 14
// and logs the failure to the security log per AI.md PART 11.
func csrfDeny(w http.ResponseWriter, r *http.Request, reason, endpoint string, logger *logging.AppLogger) {
	if logger != nil {
		logger.Security("security.csrf_failure", r.RemoteAddr, map[string]interface{}{
			"endpoint": endpoint,
			"method":   r.Method,
			"reason":   reason,
		})
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusForbidden)
	_ = json.NewEncoder(w).Encode(map[string]interface{}{
		"ok":      false,
		"error":   "CSRF_FAILED",
		"message": "CSRF token validation failed",
	})
}

// csrfGenToken generates a random hex-encoded CSRF token of tokenLength bytes.
func csrfGenToken(tokenLength int) string {
	if tokenLength <= 0 {
		tokenLength = 32
	}
	b := make([]byte, tokenLength)
	if _, err := rand.Read(b); err != nil {
		// Fallback: generate a shorter token to avoid panicking.
		b = make([]byte, 16)
		_, _ = rand.Read(b)
	}
	return hex.EncodeToString(b)
}

// csrfSecureFlag resolves the CSRF cookie Secure flag from the "auto"|"true"|"false" config value.
func csrfSecureFlag(setting string, r *http.Request) bool {
	switch strings.ToLower(setting) {
	case "true":
		return true
	case "false":
		return false
	default:
		// "auto": set Secure when the request arrived over HTTPS.
		return r.TLS != nil || r.Header.Get("X-Forwarded-Proto") == "https"
	}
}
