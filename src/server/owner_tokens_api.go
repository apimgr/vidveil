// SPDX-License-Identifier: MIT
// Owner token-management API routes per AI.md PART 11.
package server

import (
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/apimgr/vidveil/src/server/handler"
	"github.com/apimgr/vidveil/src/server/service/apitoken"
	"github.com/apimgr/vidveil/src/server/service/secret"
	"github.com/apimgr/vidveil/src/server/service/urlvar"
	"github.com/go-chi/chi/v5"
)

const ownerTokenRevealCookie = "vidveil_token_reveal_once"

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
	token, bearer := bearerToken(strings.TrimSpace(header))
	if !bearer {
		handler.SetBearerChallenge(w, "")
		handler.SendError(w, "UNAUTHORIZED", "Authentication required")
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

func (s *Server) redirectOwnerTokenReveal(w http.ResponseWriter, r *http.Request, token string, id int64, primary bool, notice string) {
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
		fragment = fmt.Sprintf("#token-%d", id)
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
	s.redirectOwnerTokenReveal(w, r, token, 0, true, "")
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
	s.rotateCSRFSecretAfterAPITokenRegeneration(r)
	s.setOwnerTokenCookie(w, issued.Token)
	s.auditOwnerToken("token.rotated", principal.OwnerID, issued.Prefix, r)
	s.redirectOwnerTokenReveal(w, r, issued.Token, 0, true, "token-rotated")
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
	s.redirectOwnerTokenReveal(w, r, issued.Token, issued.TokenID, false, "token-created")
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
	s.rotateCSRFSecretAfterAPITokenRegeneration(r)
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
	http.Redirect(w, r, urlvar.BuildURL(r, "/server/preferences/tokens")+"?notice=token-renamed#token-"+strconv.FormatInt(id, 10), http.StatusSeeOther)
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
	http.Redirect(w, r, urlvar.BuildURL(r, "/server/preferences/tokens")+"?notice=token-revoked#token-"+strconv.FormatInt(id, 10), http.StatusSeeOther)
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
	s.rotateCSRFSecretAfterAPITokenRegeneration(r)
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
	s.rotateCSRFSecretAfterAPITokenRegeneration(r)
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

func (s *Server) rotateCSRFSecretAfterAPITokenRegeneration(r *http.Request) {
	if s.secretsMgr == nil {
		log.Printf("CSRF token secret rotation after API token regeneration failed: secret manager unavailable")
		return
	}
	if err := s.secretsMgr.RotateImmediately(r.Context(), secret.CSRFTokenSecret); err != nil {
		log.Printf("CSRF token secret rotation after API token regeneration failed: %v", err)
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

func nullInt(value sql.NullInt64) any {
	if value.Valid {
		return value.Int64
	}
	return nil
}
