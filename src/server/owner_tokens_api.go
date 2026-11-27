// SPDX-License-Identifier: MIT
// Owner token-management API routes per AI.md PART 11.
package server

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/apimgr/vidveil/src/server/handler"
	"github.com/apimgr/vidveil/src/server/service/apitoken"
	"github.com/apimgr/vidveil/src/server/service/urlvar"
	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
)

const ownerTokenRevealCookie = "vidveil_token_reveal_once"

func (s *Server) ownerTokenPrincipal(w http.ResponseWriter, r *http.Request) (apitoken.Principal, bool) {
	w.Header().Set("Cache-Control", "no-store")
	path := r.URL.Path
	sensitive := strings.HasSuffix(path, "/reveal") || strings.HasSuffix(path, "/rotate") || strings.HasSuffix(path, "/restore") || (r.Method == http.MethodDelete && strings.HasSuffix(path, "/tokens/primary"))
	if sensitive && s.tokenActionLimiter != nil && !s.tokenActionLimiter.Allow(urlvar.ResolveClientIP(r)) {
		w.Header().Set("Retry-After", "900")
		handler.SendError(w, "RATE_LIMITED", "Too many token actions; try again later")
		return apitoken.Principal{}, false
	}
	header := r.Header.Get("Authorization")
	token, bearer := bearerToken(strings.TrimSpace(header))
	if !bearer {
		handler.SetBearerChallenge(w, "")
		handler.SendError(w, "UNAUTHORIZED", "Authentication required")
		return apitoken.Principal{}, false
	}
	if s.tokenManager == nil {
		handler.SendError(w, "SERVER_ERROR", "Token management is unavailable")
		return apitoken.Principal{}, false
	}
	principal, err := s.tokenManager.Authenticate(r.Context(), token)
	if err != nil {
		handler.SetBearerChallenge(w, "invalid_token")
		handler.SendError(w, "UNAUTHORIZED", "Invalid token")
		return apitoken.Principal{}, false
	}
	if principal.Kind != "primary" {
		handler.SetBearerChallenge(w, "insufficient_scope")
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

func (s *Server) redirectOwnerTokenReveal(w http.ResponseWriter, r *http.Request, token, id string, primary bool, notice string) {
	var nonce [24]byte
	if _, err := rand.Read(nonce[:]); err != nil {
		http.Error(w, "Could not prepare token reveal", http.StatusInternalServerError)
		return
	}
	key := hex.EncodeToString(nonce[:])
	now := time.Now()
	s.tokenRevealMu.Lock()
	if s.tokenReveals == nil {
		s.tokenReveals = make(map[string]tokenRevealFlash)
	}
	for existing, flash := range s.tokenReveals {
		if !flash.expiresAt.After(now) {
			delete(s.tokenReveals, existing)
		}
	}
	if len(s.tokenReveals) >= 256 {
		for existing := range s.tokenReveals {
			delete(s.tokenReveals, existing)
			break
		}
	}
	s.tokenReveals[key] = tokenRevealFlash{token: token, id: id, primary: primary, expiresAt: now.Add(time.Minute)}
	s.tokenRevealMu.Unlock()
	time.AfterFunc(time.Minute, func() {
		s.tokenRevealMu.Lock()
		if flash, ok := s.tokenReveals[key]; ok && !flash.expiresAt.After(time.Now()) {
			delete(s.tokenReveals, key)
		}
		s.tokenRevealMu.Unlock()
	})
	http.SetCookie(w, &http.Cookie{Name: ownerTokenRevealCookie, Value: key, Path: "/", MaxAge: 60, HttpOnly: true, Secure: true, SameSite: http.SameSiteStrictMode})
	fragment := "#primary"
	if !primary {
		fragment = "#token-" + id
	}
	target := urlvar.BuildURL(r, "/server/preferences/tokens")
	if notice != "" {
		target += "?notice=" + url.QueryEscape(notice)
	}
	http.Redirect(w, r, target+fragment, http.StatusSeeOther)
}

func (s *Server) ownerTokenRevealMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		cookie, err := r.Cookie(ownerTokenRevealCookie)
		if err == nil {
			http.SetCookie(w, &http.Cookie{Name: ownerTokenRevealCookie, Value: "", Path: "/", MaxAge: -1, HttpOnly: true, Secure: true, SameSite: http.SameSiteStrictMode})
			s.tokenRevealMu.Lock()
			flash, found := s.tokenReveals[cookie.Value]
			if found {
				delete(s.tokenReveals, cookie.Value)
			}
			s.tokenRevealMu.Unlock()
			if found && flash.expiresAt.After(time.Now()) {
				r = r.WithContext(handler.WithOwnerTokenReveal(r.Context(), handler.OwnerTokenReveal{ID: flash.id, Primary: flash.primary, Token: flash.token}))
			}
		}
		next.ServeHTTP(w, r)
	})
}

func (s *Server) ownerTokenWebForget(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	http.SetCookie(w, &http.Cookie{Name: apitoken.OwnerCookieName, Value: "", Path: "/", MaxAge: -1, HttpOnly: true, Secure: true, SameSite: http.SameSiteStrictMode})
	http.Redirect(w, r, urlvar.BuildURL(r, "/server/preferences/tokens")+"#primary", http.StatusSeeOther)
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
	s.redirectOwnerTokenReveal(w, r, token, "", true, "")
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
	s.redirectOwnerTokenReveal(w, r, issued.Token, "", true, "token-rotated")
}

func (s *Server) ownerTokenWebRevokePrimary(w http.ResponseWriter, r *http.Request) {
	principal, ok := s.ownerTokenWebPrincipal(w, r, true)
	if !ok {
		return
	}
	if err := r.ParseForm(); err != nil {
		http.Error(w, "Invalid token revocation form", http.StatusBadRequest)
		return
	}
	counts, err := s.tokenManager.RevokePrimary(r.Context(), principal, r.FormValue("confirm"))
	if errors.Is(err, apitoken.ErrInvalidConfirmation) {
		http.Error(w, "Primary-token confirmation did not match", http.StatusBadRequest)
		return
	}
	if err != nil {
		s.ownerTokenOperationError(w, err)
		return
	}
	if s.logger != nil {
		s.logger.Audit("token.revoked", principal.OwnerID, "owner", urlvar.ResolveClientIP(r), "success", map[string]interface{}{
			"owner_id": principal.OwnerID, "token_prefix": counts.Prefix, "scope": "owner",
			"tokens": counts.Tokens, "resources": counts.Resources, "source": "web",
		})
	}
	http.SetCookie(w, &http.Cookie{Name: apitoken.OwnerCookieName, Value: "", Path: "/", MaxAge: 0, HttpOnly: true, Secure: true, SameSite: http.SameSiteStrictMode})
	w.Header().Set("Clear-Site-Data", `"storage"`)
	http.Redirect(w, r, urlvar.BuildURL(r, "/server/preferences/tokens"), http.StatusSeeOther)
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
	scope := r.FormValue("scope")
	if scope == "" {
		scope = "read-write"
	}
	issued, err := s.tokenManager.CreateLinkedWithScope(r.Context(), principal, r.FormValue("name"), scope, expires, s.appConfig.Server.Tokens.MaxLinked)
	if err != nil {
		s.ownerTokenOperationError(w, err)
		return
	}
	s.auditOwnerToken("token.created", principal.OwnerID, issued.Prefix, r)
	s.redirectOwnerTokenReveal(w, r, issued.Token, issued.TokenID, false, "token-created")
}

func (s *Server) ownerTokenWebLinkedID(w http.ResponseWriter, r *http.Request, sensitive bool) (apitoken.Principal, string, bool) {
	principal, ok := s.ownerTokenWebPrincipal(w, r, sensitive)
	if !ok {
		return apitoken.Principal{}, "", false
	}
	id := chi.URLParam(r, "id")
	parsed, err := uuid.Parse(id)
	if err != nil || parsed.Version() != 7 {
		http.NotFound(w, r)
		return apitoken.Principal{}, "", false
	}
	return principal, parsed.String(), true
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
	s.redirectOwnerTokenReveal(w, r, token, id, false, "")
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
	s.redirectOwnerTokenReveal(w, r, issued.Token, id, false, "token-rotated")
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
	expires, err := ownerTokenExpiryFromForm(r)
	if err != nil {
		http.Error(w, "Invalid token expiry", http.StatusBadRequest)
		return
	}
	if err := s.tokenManager.UpdateLinked(r.Context(), principal, id, r.FormValue("name"), expires); err != nil {
		s.ownerTokenOperationError(w, err)
		return
	}
	http.Redirect(w, r, urlvar.BuildURL(r, "/server/preferences/tokens")+"?notice=token-renamed#token-"+id, http.StatusSeeOther)
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
	http.Redirect(w, r, urlvar.BuildURL(r, "/server/preferences/tokens")+"?notice=token-revoked#token-"+id, http.StatusSeeOther)
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
	http.Redirect(w, r, urlvar.BuildURL(r, "/server/preferences/tokens")+"#primary", http.StatusSeeOther)
}

// ownerTokenWebIssuePrimary creates the visitor's first anonymous identity.
func (s *Server) ownerTokenWebIssuePrimary(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	if s.tokenActionLimiter != nil && !s.tokenActionLimiter.Allow(urlvar.ResolveClientIP(r)) {
		w.Header().Set("Retry-After", "900")
		handler.SendError(w, "RATE_LIMITED", "Too many token actions")
		return
	}
	if s.tokenManager == nil {
		handler.SendError(w, "SERVER_ERROR", "Token management is unavailable")
		return
	}
	if cookie, err := r.Cookie(ownerTokenCookieName); err == nil {
		_, ownerErr := s.tokenManager.ValidateOnly(r.Context(), cookie.Value)
		if s.matchesOperatorToken(cookie.Value) || ownerErr == nil {
			handler.SendError(w, "CONFLICT", "A valid token is already present")
			return
		}
	}
	issued, err := s.tokenManager.IssuePrimary(r.Context())
	if err != nil {
		handler.SendError(w, "SERVER_ERROR", "Could not issue a primary token")
		return
	}
	s.setOwnerTokenCookie(w, issued.Token)
	if s.logger != nil {
		s.logger.Audit("token.created", issued.OwnerID, "owner", urlvar.ResolveClientIP(r), "success", map[string]interface{}{
			"owner_id": issued.OwnerID, "token_prefix": issued.Prefix, "source": "issue",
		})
	}
	s.redirectOwnerTokenReveal(w, r, issued.Token, "", true, "token-created")
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
		items = append(items, ownerTokenJSON(token.ID, token.Name, "linked", token.Scope, token.Prefix, token.Suffix, token.CreatedAt, token.LastUsed, token.ExpiresAt, status))
	}
	handler.SendOK(w, map[string]any{
		"primary": ownerTokenJSON(primary.ID, "", "primary", "global", primary.Prefix, primary.Suffix, primary.CreatedAt, primary.LastUsed, sql.NullInt64{}, "active"),
		"linked":  items, "max_linked": s.appConfig.Server.Tokens.MaxLinked,
	})
}

func ownerTokenJSON(id, name, kind, scope, prefix, suffix string, created int64, lastUsed, expires sql.NullInt64, status string) map[string]any {
	var lastUsedAt, expiresAt any
	var tokenName any = name
	if kind == "primary" {
		tokenName = nil
	}
	maskedPrefix := strings.SplitN(prefix, "_", 2)[0] + "_..."
	if lastUsed.Valid {
		lastUsedAt = time.Unix(lastUsed.Int64, 0).UTC().Format(time.RFC3339)
	}
	if expires.Valid {
		expiresAt = time.Unix(expires.Int64, 0).UTC().Format(time.RFC3339)
	}
	return map[string]any{
		"id": id, "name": tokenName, "kind": kind, "scope": scope,
		"token_masked": maskedPrefix + suffix,
		"created_at":   time.Unix(created, 0).UTC().Format(time.RFC3339),
		"last_used_at": lastUsedAt, "expires_at": expiresAt, "status": status,
	}
}

func (s *Server) linkedTokenJSON(ctx context.Context, primary apitoken.Principal, id string) (map[string]any, error) {
	linked, err := s.tokenManager.ListLinked(ctx, primary)
	if err != nil {
		return nil, err
	}
	now := time.Now().Unix()
	for _, token := range linked {
		if token.ID != id {
			continue
		}
		status := "active"
		if token.RevokedAt.Valid {
			status = "revoked"
		} else if token.ExpiresAt.Valid && token.ExpiresAt.Int64 <= now {
			status = "expired"
		}
		return ownerTokenJSON(token.ID, token.Name, "linked", token.Scope, token.Prefix, token.Suffix, token.CreatedAt, token.LastUsed, token.ExpiresAt, status), nil
	}
	return nil, apitoken.ErrInvalidToken
}

// ownerTokensAPIIssuePrimary issues a new anonymous primary token when the
// caller does not already hold a valid operator or owner credential.
func (s *Server) ownerTokensAPIIssuePrimary(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	if s.tokenActionLimiter != nil && !s.tokenActionLimiter.Allow(urlvar.ResolveClientIP(r)) {
		w.Header().Set("Retry-After", "900")
		handler.SendError(w, "RATE_LIMITED", "Too many token actions")
		return
	}

	if raw, ok := bearerToken(strings.TrimSpace(r.Header.Get("Authorization"))); ok {
		if s.matchesOperatorToken(raw) {
			handler.SendError(w, "CONFLICT", "A valid token is already present")
			return
		}
		if s.tokenManager != nil {
			if _, err := s.tokenManager.ValidateOnly(r.Context(), raw); err == nil {
				handler.SendError(w, "CONFLICT", "A valid token is already present")
				return
			}
		}
	}
	if s.tokenManager == nil {
		handler.SendError(w, "SERVER_ERROR", "Token management is unavailable")
		return
	}
	issued, err := s.tokenManager.IssuePrimary(r.Context())
	if err != nil {
		handler.SendError(w, "SERVER_ERROR", "Could not issue a primary token")
		return
	}
	if s.logger != nil {
		s.logger.Audit("token.created", issued.OwnerID, "owner", urlvar.ResolveClientIP(r), "success", map[string]interface{}{
			"owner_id": issued.OwnerID, "token_prefix": issued.Prefix, "source": "issue",
		})
	}
	w.WriteHeader(http.StatusCreated)
	handler.SendOK(w, map[string]string{"owner_token": issued.Token})
}

func (s *Server) ownerTokensAPICreate(w http.ResponseWriter, r *http.Request) {
	principal, ok := s.ownerTokenPrincipal(w, r)
	if !ok {
		return
	}
	var input struct {
		Name      string `json:"name"`
		Scope     string `json:"scope"`
		Expires   string `json:"expires"`
		ExpiresOn string `json:"expires_on"`
	}
	decoder := json.NewDecoder(io.LimitReader(r.Body, 8<<10))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&input); err != nil {
		handler.SendError(w, "VALIDATION_FAILED", "Invalid token payload")
		return
	}
	if err := decoder.Decode(&struct{}{}); err != io.EOF {
		handler.SendError(w, "VALIDATION_FAILED", "Invalid token payload")
		return
	}
	if len(strings.TrimSpace(input.Name)) < 1 || len(strings.TrimSpace(input.Name)) > 64 {
		handler.SendError(w, "VALIDATION_FAILED", "Token name must be 1 to 64 characters")
		return
	}
	if input.Expires != "" && input.ExpiresOn != "" {
		handler.SendError(w, "VALIDATION_FAILED", "Choose either expires or expires_on")
		return
	}
	expires, err := parseOwnerTokenExpiry(input.Expires)
	if input.ExpiresOn != "" {
		expires, err = parseOwnerTokenDate(input.ExpiresOn)
	}
	if err != nil {
		handler.SendError(w, "VALIDATION_FAILED", "Invalid token expiry")
		return
	}
	if input.Scope == "" {
		input.Scope = "read-write"
	}
	if input.Scope != "read" && input.Scope != "read-write" {
		handler.SendError(w, "VALIDATION_FAILED", "Invalid linked-token scope")
		return
	}
	issued, err := s.tokenManager.CreateLinkedWithScope(r.Context(), principal, input.Name, input.Scope, expires, s.appConfig.Server.Tokens.MaxLinked)
	if err != nil {
		s.ownerTokenOperationError(w, err)
		return
	}
	s.auditOwnerToken("token.created", principal.OwnerID, issued.Prefix, r)
	w.WriteHeader(http.StatusCreated)
	createdAt := time.Now().UTC().Unix()
	var expiresAt sql.NullInt64
	if expires != nil {
		expiresAt = sql.NullInt64{Int64: *expires, Valid: true}
	}
	result := ownerTokenJSON(issued.TokenID, strings.TrimSpace(input.Name), "linked", input.Scope, issued.Prefix, issued.Token[len(issued.Token)-4:], createdAt, sql.NullInt64{}, expiresAt, "active")
	result["linked_token"] = issued.Token
	handler.SendOK(w, result)
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

func (s *Server) ownerTokensAPIRevokePrimary(w http.ResponseWriter, r *http.Request) {
	principal, ok := s.ownerTokenPrincipal(w, r)
	if !ok {
		return
	}
	var input struct {
		Confirm string `json:"confirm"`
	}
	decoder := json.NewDecoder(io.LimitReader(r.Body, 8<<10))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&input); err != nil {
		handler.SendError(w, "VALIDATION_FAILED", "A valid confirmation is required")
		return
	}
	if err := decoder.Decode(&struct{}{}); err != io.EOF {
		handler.SendError(w, "VALIDATION_FAILED", "A valid confirmation is required")
		return
	}
	counts, err := s.tokenManager.RevokePrimary(r.Context(), principal, input.Confirm)
	if err != nil {
		s.ownerTokenOperationError(w, err)
		return
	}
	if s.logger != nil {
		s.logger.Audit("token.revoked", principal.OwnerID, "owner", urlvar.ResolveClientIP(r), "success", map[string]interface{}{
			"owner_id": principal.OwnerID, "token_prefix": counts.Prefix, "scope": "owner",
			"tokens": counts.Tokens, "resources": counts.Resources, "source": "api",
		})
	}
	handler.SendOK(w, map[string]int64{"tokens": counts.Tokens, "resources": counts.Resources})
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
	result, err := s.linkedTokenJSON(r.Context(), principal, id)
	if err != nil {
		s.ownerTokenOperationError(w, err)
		return
	}
	result["linked_token"] = token
	handler.SendOK(w, result)
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
	result, err := s.linkedTokenJSON(r.Context(), principal, id)
	if err != nil {
		s.ownerTokenOperationError(w, err)
		return
	}
	result["linked_token"] = issued.Token
	handler.SendOK(w, result)
}

func (s *Server) ownerTokensAPIUpdateLinked(w http.ResponseWriter, r *http.Request) {
	principal, id, ok := s.ownerTokenRequestID(w, r)
	if !ok {
		return
	}
	var input struct {
		Name      *string `json:"name"`
		Expires   *string `json:"expires"`
		ExpiresOn *string `json:"expires_on"`
	}
	decoder := json.NewDecoder(io.LimitReader(r.Body, 8<<10))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&input); err != nil {
		handler.SendError(w, "VALIDATION_FAILED", "Invalid token payload")
		return
	}
	if err := decoder.Decode(&struct{}{}); err != io.EOF {
		handler.SendError(w, "VALIDATION_FAILED", "Invalid token payload")
		return
	}
	if input.Expires != nil && input.ExpiresOn != nil {
		handler.SendError(w, "VALIDATION_FAILED", "Choose either expires or expires_on")
		return
	}
	current, err := s.linkedTokenJSON(r.Context(), principal, id)
	if err != nil {
		s.ownerTokenOperationError(w, err)
		return
	}
	name := current["name"].(string)
	if input.Name != nil {
		name = *input.Name
	}
	if len(strings.TrimSpace(name)) < 1 || len(strings.TrimSpace(name)) > 64 {
		handler.SendError(w, "VALIDATION_FAILED", "Token name must be 1 to 64 characters")
		return
	}
	var expires *int64
	if input.Expires != nil || input.ExpiresOn != nil {
		value := ""
		if input.Expires != nil {
			value = *input.Expires
			expires, err = parseOwnerTokenExpiry(value)
		} else {
			expires, err = parseOwnerTokenDate(*input.ExpiresOn)
		}
	} else if currentExpiry, ok := current["expires_at"].(string); ok {
		parsed, parseErr := time.Parse(time.RFC3339, currentExpiry)
		if parseErr != nil {
			err = parseErr
		} else {
			value := parsed.Unix()
			expires = &value
		}
	}
	if err != nil {
		handler.SendError(w, "VALIDATION_FAILED", "Invalid token expiry")
		return
	}
	if err := s.tokenManager.UpdateLinked(r.Context(), principal, id, name, expires); err != nil {
		s.ownerTokenOperationError(w, err)
		return
	}
	result, err := s.linkedTokenJSON(r.Context(), principal, id)
	if err != nil {
		s.ownerTokenOperationError(w, err)
		return
	}
	handler.SendOK(w, result)
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
	handler.SendOK(w, map[string]string{"id": id, "status": "revoked"})
}

func (s *Server) ownerTokenRequestID(w http.ResponseWriter, r *http.Request) (apitoken.Principal, string, bool) {
	principal, ok := s.ownerTokenPrincipal(w, r)
	if !ok {
		return apitoken.Principal{}, "", false
	}
	id := chi.URLParam(r, "id")
	parsed, err := uuid.Parse(id)
	if err != nil || parsed.Version() != 7 {
		handler.SendError(w, "NOT_FOUND", "Token not found")
		return apitoken.Principal{}, "", false
	}
	return principal, parsed.String(), true
}

func (s *Server) ownerTokenOperationError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, apitoken.ErrManagementForbidden):
		handler.SendError(w, "FORBIDDEN", "Token management requires the primary token")
	case errors.Is(err, apitoken.ErrInvalidToken):
		handler.SendError(w, "NOT_FOUND", "Token not found")
	case errors.Is(err, apitoken.ErrLinkedTokenLimit), errors.Is(err, apitoken.ErrDuplicateTokenName):
		handler.SendError(w, "CONFLICT", err.Error())
	case errors.Is(err, apitoken.ErrInvalidConfirmation):
		handler.SendError(w, "VALIDATION_FAILED", "Primary-token confirmation did not match")
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
		return parseOwnerTokenDate(value)
	}
}

func parseOwnerTokenDate(value string) (*int64, error) {
	date, err := time.Parse("2006-01-02", value)
	if err != nil {
		return nil, err
	}
	today := time.Now().UTC().Format("2006-01-02")
	if value < today {
		return nil, errors.New("expiry date must be today or later in UTC")
	}
	expires := date.UTC().Add(24 * time.Hour).Unix()
	return &expires, nil
}

func ownerTokenExpiryFromForm(r *http.Request) (*int64, error) {
	if preset := r.FormValue("expires_preset"); preset != "" {
		if preset == "custom" {
			date := strings.TrimSpace(r.FormValue("expires_date"))
			if date == "" {
				return nil, errors.New("custom expiry date is required")
			}
			return parseOwnerTokenExpiry(date)
		}
		return parseOwnerTokenExpiry(preset)
	}
	if date := strings.TrimSpace(r.FormValue("expires_date")); date != "" {
		return parseOwnerTokenExpiry(date)
	}
	return parseOwnerTokenExpiry(r.FormValue("expires"))
}
