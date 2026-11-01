// SPDX-License-Identifier: MIT
// Owner token-management API routes per AI.md PART 11.
package server

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"html"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/apimgr/vidveil/src/common/i18n"
	"github.com/apimgr/vidveil/src/server/handler"
	"github.com/apimgr/vidveil/src/server/service/apitoken"
	"github.com/apimgr/vidveil/src/server/service/urlvar"
	"github.com/go-chi/chi/v5"
)

func (s *Server) ownerTokenPrincipal(w http.ResponseWriter, r *http.Request) (apitoken.Principal, bool) {
	w.Header().Set("Cache-Control", "no-store")
	if s.tokenManager == nil {
		handler.SendError(w, "SERVER_ERROR", "Token management is unavailable")
		return apitoken.Principal{}, false
	}
	path := r.URL.Path
	sensitive := strings.HasSuffix(path, "/reveal") || strings.HasSuffix(path, "/rotate") || strings.HasSuffix(path, "/restore")
	if r.Method == http.MethodPost && sensitive && s.tokenActionLimiter != nil && !s.tokenActionLimiter.Allow(urlvar.ResolveClientIP(r)) {
		w.Header().Set("Retry-After", "900")
		handler.SendError(w, "RATE_LIMITED", "Too many token actions; try again later")
		return apitoken.Principal{}, false
	}
	header := r.Header.Get("Authorization")
	if !strings.HasPrefix(header, "Bearer ") || strings.TrimSpace(strings.TrimPrefix(header, "Bearer ")) == "" {
		handler.SendError(w, "FORBIDDEN", "A primary bearer token is required")
		return apitoken.Principal{}, false
	}
	principal, err := s.tokenManager.Authenticate(r.Context(), strings.TrimSpace(strings.TrimPrefix(header, "Bearer ")))
	if err != nil {
		handler.SendError(w, "FORBIDDEN", "A primary bearer token is required")
		return apitoken.Principal{}, false
	}
	if principal.Kind != "primary" {
		handler.SendError(w, "FORBIDDEN", "Token management requires the primary token")
		return apitoken.Principal{}, false
	}
	return principal, true
}

const ownerTokenCookieName = apitoken.OwnerCookieName

func (s *Server) ownerTokenCookieMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if s.tokenManager != nil && !strings.HasPrefix(r.URL.Path, "/api/") {
			if cookie, err := r.Cookie(ownerTokenCookieName); err == nil {
				principal, authErr := s.tokenManager.Authenticate(r.Context(), cookie.Value)
				if authErr == nil && principal.Kind == "primary" {
					s.setOwnerTokenCookie(w, cookie.Value)
				} else {
					http.SetCookie(w, &http.Cookie{Name: ownerTokenCookieName, Value: "", Path: "/", MaxAge: -1, HttpOnly: true, Secure: true, SameSite: http.SameSiteStrictMode})
				}
			}
		}
		next.ServeHTTP(w, r)
	})
}

func (s *Server) ownerTokenWebPrincipal(w http.ResponseWriter, r *http.Request, sensitive bool) (apitoken.Principal, bool) {
	w.Header().Set("Cache-Control", "no-store")
	if sensitive && s.tokenActionLimiter != nil && !s.tokenActionLimiter.Allow(urlvar.ResolveClientIP(r)) {
		w.Header().Set("Retry-After", "900")
		http.Error(w, "Too many token actions; try again later", http.StatusTooManyRequests)
		return apitoken.Principal{}, false
	}
	cookie, err := r.Cookie(apitoken.OwnerCookieName)
	if err != nil || s.tokenManager == nil {
		http.Error(w, "A primary owner token is required", http.StatusForbidden)
		return apitoken.Principal{}, false
	}
	principal, err := s.tokenManager.Authenticate(r.Context(), cookie.Value)
	if err != nil || principal.Kind != "primary" {
		http.SetCookie(w, &http.Cookie{Name: apitoken.OwnerCookieName, Value: "", Path: "/", MaxAge: -1, HttpOnly: true, Secure: true, SameSite: http.SameSiteStrictMode})
		http.Error(w, "A primary owner token is required", http.StatusForbidden)
		return apitoken.Principal{}, false
	}
	return principal, true
}

func (s *Server) setOwnerTokenCookie(w http.ResponseWriter, raw string) {
	maxAge := 365 * 24 * 60 * 60
	if s.appConfig != nil {
		maxAge = apitoken.CookieMaxAge(s.appConfig.Server.Tokens.CookieMaxAge)
	}
	http.SetCookie(w, &http.Cookie{Name: apitoken.OwnerCookieName, Value: raw, Path: "/", MaxAge: maxAge, HttpOnly: true, Secure: true, SameSite: http.SameSiteStrictMode})
}

func writeOwnerTokenReveal(w http.ResponseWriter, r *http.Request, token string) {
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	locale := i18n.DetectLocale(r)
	_, _ = fmt.Fprintf(w, `<!doctype html><html lang="%s" dir="%s"><meta charset="utf-8"><title>%s</title><main><h1>%s</h1><p>%s</p><code class="owner-token-value" data-owner-token-value>%s</code><p><button type="button" data-copy-owner-token data-copy-success="%s" data-copy-failed="%s">%s</button> <span role="status" aria-live="polite"></span></p><p><a href="/server/preferences">%s</a></p></main><script src="/static/js/app.js"></script></html>`, html.EscapeString(locale), i18n.Direction(locale), html.EscapeString(i18n.Translate(locale, "owner_tokens.title")), html.EscapeString(i18n.Translate(locale, "owner_tokens.title")), html.EscapeString(i18n.Translate(locale, "owner_tokens.copy_store")), html.EscapeString(token), html.EscapeString(i18n.Translate(locale, "owner_tokens.copy_success")), html.EscapeString(i18n.Translate(locale, "owner_tokens.copy_failed")), html.EscapeString(i18n.Translate(locale, "owner_tokens.copy")), html.EscapeString(i18n.Translate(locale, "owner_tokens.return_preferences")))
}

func (s *Server) ownerTokenWebRevealPrimary(w http.ResponseWriter, r *http.Request) {
	principal, ok := s.ownerTokenWebPrincipal(w, r, true)
	if !ok {
		return
	}
	token, err := s.tokenManager.Reveal(r.Context(), principal, principal.TokenID)
	if err != nil {
		s.ownerTokenOperationError(w, err)
		return
	}
	s.auditOwnerToken("token.revealed", principal.OwnerID, principal.Prefix, r)
	writeOwnerTokenReveal(w, r, token)
}

func (s *Server) ownerTokenWebRotatePrimary(w http.ResponseWriter, r *http.Request) {
	principal, ok := s.ownerTokenWebPrincipal(w, r, true)
	if !ok {
		return
	}
	if err := r.ParseForm(); err != nil || r.FormValue("confirm") != "rotate" {
		http.Error(w, "Primary token rotation confirmation is required", http.StatusBadRequest)
		return
	}
	issued, err := s.tokenManager.Rotate(r.Context(), principal, principal.TokenID)
	if err != nil {
		s.ownerTokenOperationError(w, err)
		return
	}
	s.setOwnerTokenCookie(w, issued.Token)
	s.auditOwnerToken("token.rotated", principal.OwnerID, issued.Prefix, r)
	writeOwnerTokenReveal(w, r, issued.Token)
}

func (s *Server) ownerTokenWebCreateLinked(w http.ResponseWriter, r *http.Request) {
	principal, ok := s.ownerTokenWebPrincipal(w, r, false)
	if !ok {
		return
	}
	if err := r.ParseForm(); err != nil {
		http.Error(w, "Invalid token form", http.StatusBadRequest)
		return
	}
	expires, err := ownerTokenExpiryFromForm(r)
	if err != nil {
		http.Error(w, "Invalid token expiry", http.StatusBadRequest)
		return
	}
	issued, err := s.tokenManager.CreateLinked(r.Context(), principal, r.FormValue("name"), expires, s.appConfig.Server.Tokens.MaxLinked)
	if err != nil {
		s.ownerTokenOperationError(w, err)
		return
	}
	s.auditOwnerToken("token.created", principal.OwnerID, issued.Prefix, r)
	writeOwnerTokenReveal(w, r, issued.Token)
}

func (s *Server) ownerTokenWebLinkedID(w http.ResponseWriter, r *http.Request, sensitive bool) (apitoken.Principal, int64, bool) {
	principal, ok := s.ownerTokenWebPrincipal(w, r, sensitive)
	if !ok {
		return apitoken.Principal{}, 0, false
	}
	id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil || id < 1 {
		http.NotFound(w, r)
		return apitoken.Principal{}, 0, false
	}
	return principal, id, true
}

func (s *Server) ownerTokenWebRevealLinked(w http.ResponseWriter, r *http.Request) {
	principal, id, ok := s.ownerTokenWebLinkedID(w, r, true)
	if !ok {
		return
	}
	token, err := s.tokenManager.Reveal(r.Context(), principal, id)
	if err != nil {
		s.ownerTokenOperationError(w, err)
		return
	}
	s.auditOwnerToken("token.revealed", principal.OwnerID, token[:12], r)
	writeOwnerTokenReveal(w, r, token)
}

func (s *Server) ownerTokenWebRotateLinked(w http.ResponseWriter, r *http.Request) {
	principal, id, ok := s.ownerTokenWebLinkedID(w, r, true)
	if !ok {
		return
	}
	issued, err := s.tokenManager.Rotate(r.Context(), principal, id)
	if err != nil {
		s.ownerTokenOperationError(w, err)
		return
	}
	s.auditOwnerToken("token.rotated", principal.OwnerID, issued.Prefix, r)
	writeOwnerTokenReveal(w, r, issued.Token)
}

func (s *Server) ownerTokenWebUpdateLinked(w http.ResponseWriter, r *http.Request) {
	principal, id, ok := s.ownerTokenWebLinkedID(w, r, false)
	if !ok {
		return
	}
	if err := r.ParseForm(); err != nil {
		http.Error(w, "Invalid token form", http.StatusBadRequest)
		return
	}
	expires, err := parseOwnerTokenExpiry(r.FormValue("expires"))
	if err != nil {
		http.Error(w, "Invalid token expiry", http.StatusBadRequest)
		return
	}
	if err := s.tokenManager.UpdateLinked(r.Context(), principal, id, r.FormValue("name"), expires); err != nil {
		s.ownerTokenOperationError(w, err)
		return
	}
	http.Redirect(w, r, "/server/preferences", http.StatusSeeOther)
}

func (s *Server) ownerTokenWebRevokeLinked(w http.ResponseWriter, r *http.Request) {
	principal, id, ok := s.ownerTokenWebLinkedID(w, r, false)
	if !ok {
		return
	}
	var prefix string
	if linked, err := s.tokenManager.ListLinked(r.Context(), principal); err == nil {
		for _, item := range linked {
			if item.ID == id {
				prefix = item.Prefix
				break
			}
		}
	}
	if err := s.tokenManager.RevokeLinked(r.Context(), principal, id); err != nil {
		s.ownerTokenOperationError(w, err)
		return
	}
	s.auditOwnerToken("token.revoked", principal.OwnerID, prefix, r)
	http.Redirect(w, r, "/server/preferences", http.StatusSeeOther)
}

func (s *Server) ownerTokenRestore(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	if s.tokenManager == nil {
		handler.SendError(w, "SERVER_ERROR", "Token management is unavailable")
		return
	}
	if s.tokenActionLimiter != nil && !s.tokenActionLimiter.Allow(urlvar.ResolveClientIP(r)) {
		w.Header().Set("Retry-After", "900")
		handler.SendError(w, "RATE_LIMITED", "Too many token actions; try again later")
		return
	}
	if err := r.ParseForm(); err != nil {
		handler.SendError(w, "BAD_REQUEST", "Invalid token form")
		return
	}
	principal, err := s.tokenManager.Authenticate(r.Context(), strings.TrimSpace(r.FormValue("token")))
	if err != nil || principal.Kind != "primary" {
		handler.SendError(w, "FORBIDDEN", "A valid primary token is required")
		return
	}
	s.setOwnerTokenCookie(w, strings.TrimSpace(r.FormValue("token")))
	s.auditOwnerToken("token.restored", principal.OwnerID, principal.Prefix, r)
	http.Redirect(w, r, "/server/preferences", http.StatusSeeOther)
}

func (s *Server) ownerTokensAPIList(w http.ResponseWriter, r *http.Request) {
	principal, ok := s.ownerTokenPrincipal(w, r)
	if !ok {
		return
	}
	primary, err := s.tokenManager.PrimaryMetadata(r.Context(), principal)
	if err != nil {
		handler.SendError(w, "SERVER_ERROR", "Could not load token details")
		return
	}
	linked, err := s.tokenManager.ListLinked(r.Context(), principal)
	if err != nil {
		handler.SendError(w, "SERVER_ERROR", "Could not load linked tokens")
		return
	}
	items := make([]map[string]any, 0, len(linked))
	now := time.Now().Unix()
	for _, token := range linked {
		status := "active"
		if token.RevokedAt.Valid {
			status = "revoked"
		} else if token.ExpiresAt.Valid && token.ExpiresAt.Int64 <= now {
			status = "expired"
		}
		items = append(items, map[string]any{
			"id": token.ID, "name": token.Name, "prefix": token.Prefix, "suffix": token.Suffix,
			"created_at": token.CreatedAt, "last_used_at": nullInt(token.LastUsed),
			"expires_at": nullInt(token.ExpiresAt), "status": status,
		})
	}
	handler.SendOK(w, map[string]any{
		"primary": map[string]any{"id": primary.ID, "prefix": primary.Prefix, "suffix": primary.Suffix, "created_at": primary.CreatedAt, "last_used_at": nullInt(primary.LastUsed)},
		"linked":  items,
	})
}

func (s *Server) ownerTokensAPICreate(w http.ResponseWriter, r *http.Request) {
	principal, ok := s.ownerTokenPrincipal(w, r)
	if !ok {
		return
	}
	var input struct {
		Name    string `json:"name"`
		Expires string `json:"expires"`
	}
	if err := json.NewDecoder(io.LimitReader(r.Body, 8<<10)).Decode(&input); err != nil {
		handler.SendError(w, "VALIDATION_FAILED", "Invalid token payload")
		return
	}
	expires, err := parseOwnerTokenExpiry(input.Expires)
	if err != nil {
		handler.SendError(w, "VALIDATION_FAILED", "Invalid token expiry")
		return
	}
	issued, err := s.tokenManager.CreateLinked(r.Context(), principal, input.Name, expires, s.appConfig.Server.Tokens.MaxLinked)
	if err != nil {
		s.ownerTokenOperationError(w, err)
		return
	}
	s.auditOwnerToken("token.created", principal.OwnerID, issued.Prefix, r)
	handler.SendOK(w, map[string]any{"id": issued.TokenID, "name": strings.TrimSpace(input.Name), "token": issued.Token, "prefix": issued.Prefix})
}

func (s *Server) ownerTokensAPIRevealPrimary(w http.ResponseWriter, r *http.Request) {
	principal, ok := s.ownerTokenPrincipal(w, r)
	if !ok {
		return
	}
	token, err := s.tokenManager.Reveal(r.Context(), principal, principal.TokenID)
	if err != nil {
		s.ownerTokenOperationError(w, err)
		return
	}
	s.auditOwnerToken("token.revealed", principal.OwnerID, token[:12], r)
	handler.SendOK(w, map[string]string{"token": token})
}

func (s *Server) ownerTokensAPIRotatePrimary(w http.ResponseWriter, r *http.Request) {
	principal, ok := s.ownerTokenPrincipal(w, r)
	if !ok {
		return
	}
	issued, err := s.tokenManager.Rotate(r.Context(), principal, principal.TokenID)
	if err != nil {
		s.ownerTokenOperationError(w, err)
		return
	}
	s.auditOwnerToken("token.rotated", principal.OwnerID, issued.Prefix, r)
	handler.SendOK(w, map[string]string{"token": issued.Token, "prefix": issued.Prefix})
}

func (s *Server) ownerTokensAPIRevealLinked(w http.ResponseWriter, r *http.Request) {
	principal, id, ok := s.ownerTokenRequestID(w, r)
	if !ok {
		return
	}
	token, err := s.tokenManager.Reveal(r.Context(), principal, id)
	if err != nil {
		s.ownerTokenOperationError(w, err)
		return
	}
	s.auditOwnerToken("token.revealed", principal.OwnerID, token[:12], r)
	handler.SendOK(w, map[string]string{"token": token})
}

func (s *Server) ownerTokensAPIRotateLinked(w http.ResponseWriter, r *http.Request) {
	principal, id, ok := s.ownerTokenRequestID(w, r)
	if !ok {
		return
	}
	issued, err := s.tokenManager.Rotate(r.Context(), principal, id)
	if err != nil {
		s.ownerTokenOperationError(w, err)
		return
	}
	s.auditOwnerToken("token.rotated", principal.OwnerID, issued.Prefix, r)
	handler.SendOK(w, map[string]string{"token": issued.Token, "prefix": issued.Prefix})
}

func (s *Server) ownerTokensAPIUpdateLinked(w http.ResponseWriter, r *http.Request) {
	principal, id, ok := s.ownerTokenRequestID(w, r)
	if !ok {
		return
	}
	var input struct {
		Name    string `json:"name"`
		Expires string `json:"expires"`
	}
	if err := json.NewDecoder(io.LimitReader(r.Body, 8<<10)).Decode(&input); err != nil {
		handler.SendError(w, "VALIDATION_FAILED", "Invalid token payload")
		return
	}
	expires, err := parseOwnerTokenExpiry(input.Expires)
	if err != nil {
		handler.SendError(w, "VALIDATION_FAILED", "Invalid token expiry")
		return
	}
	if err := s.tokenManager.UpdateLinked(r.Context(), principal, id, input.Name, expires); err != nil {
		s.ownerTokenOperationError(w, err)
		return
	}
	handler.SendOK(w, map[string]bool{"updated": true})
}

func (s *Server) ownerTokensAPIRevokeLinked(w http.ResponseWriter, r *http.Request) {
	principal, id, ok := s.ownerTokenRequestID(w, r)
	if !ok {
		return
	}
	prefix := ""
	if linked, err := s.tokenManager.ListLinked(r.Context(), principal); err == nil {
		for _, token := range linked {
			if token.ID == id {
				prefix = token.Prefix
				break
			}
		}
	}
	if err := s.tokenManager.RevokeLinked(r.Context(), principal, id); err != nil {
		s.ownerTokenOperationError(w, err)
		return
	}
	s.auditOwnerToken("token.revoked", principal.OwnerID, prefix, r)
	handler.SendOK(w, map[string]bool{"revoked": true})
}

func (s *Server) ownerTokenRequestID(w http.ResponseWriter, r *http.Request) (apitoken.Principal, int64, bool) {
	principal, ok := s.ownerTokenPrincipal(w, r)
	if !ok {
		return apitoken.Principal{}, 0, false
	}
	id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil || id < 1 {
		handler.SendError(w, "NOT_FOUND", "Token not found")
		return apitoken.Principal{}, 0, false
	}
	return principal, id, true
}

func (s *Server) ownerTokenOperationError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, apitoken.ErrManagementForbidden):
		handler.SendError(w, "FORBIDDEN", "Token management requires the primary token")
	case errors.Is(err, apitoken.ErrInvalidToken):
		handler.SendError(w, "NOT_FOUND", "Token not found")
	case errors.Is(err, apitoken.ErrLinkedTokenLimit), errors.Is(err, apitoken.ErrDuplicateTokenName):
		handler.SendError(w, "VALIDATION_FAILED", err.Error())
	default:
		handler.SendError(w, "SERVER_ERROR", "Token operation failed")
	}
}

func (s *Server) auditOwnerToken(event, ownerID, prefix string, r *http.Request) {
	if s.logger == nil {
		return
	}
	s.logger.Audit(event, ownerID, "owner", urlvar.ResolveClientIP(r), "success", map[string]interface{}{"owner_id": ownerID, "token_prefix": prefix})
}

func parseOwnerTokenExpiry(value string) (*int64, error) {
	switch value {
	case "", "never":
		return nil, nil
	case "7d", "30d", "90d", "1y":
		duration := map[string]time.Duration{"7d": 7 * 24 * time.Hour, "30d": 30 * 24 * time.Hour, "90d": 90 * 24 * time.Hour, "1y": 365 * 24 * time.Hour}[value]
		expires := time.Now().Add(duration).Unix()
		return &expires, nil
	default:
		date, err := time.Parse("2006-01-02", value)
		if err != nil {
			return nil, err
		}
		date = date.Add(24*time.Hour - time.Second)
		expires := date.Unix()
		if expires <= time.Now().Unix() {
			return nil, errors.New("expiry must be in the future")
		}
		return &expires, nil
	}
}

func ownerTokenExpiryFromForm(r *http.Request) (*int64, error) {
	if date := strings.TrimSpace(r.FormValue("expires_date")); date != "" {
		return parseOwnerTokenExpiry(date)
	}
	if preset := r.FormValue("expires_preset"); preset != "" {
		return parseOwnerTokenExpiry(preset)
	}
	return parseOwnerTokenExpiry(r.FormValue("expires"))
}

func nullInt(value sql.NullInt64) any {
	if value.Valid {
		return value.Int64
	}
	return nil
}
