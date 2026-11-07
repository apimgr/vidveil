// SPDX-License-Identifier: MIT
package handler

import (
	"context"
	"database/sql"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/apimgr/vidveil/src/common/i18n"
	"github.com/apimgr/vidveil/src/server/service/apitoken"
	"github.com/apimgr/vidveil/src/server/service/urlvar"
)

type ownerTokenPageItem struct {
	ID        int64
	Name      string
	Suffix    string
	Created   string
	LastUsed  string
	Expires   string
	Status    string
	Revealed  string
	RevealURL string
	RotateURL string
	UpdateURL string
	RevokeURL string
}

type OwnerTokenReveal struct {
	ID      int64
	Primary bool
	Token   string
}

type ownerTokenRevealContextKey struct{}

func ownerTokenPathPrefix(r *http.Request) string {
	prefix := urlvar.GetPathPrefix(r)
	if prefix == "/" {
		return ""
	}
	return strings.TrimRight(prefix, "/")
}

func WithOwnerTokenReveal(ctx context.Context, reveal OwnerTokenReveal) context.Context {
	return context.WithValue(ctx, ownerTokenRevealContextKey{}, reveal)
}

func OwnerTokenRevealFromContext(ctx context.Context) (OwnerTokenReveal, bool) {
	reveal, ok := ctx.Value(ownerTokenRevealContextKey{}).(OwnerTokenReveal)
	return reveal, ok
}

// OwnerTokensPage renders token management separately from the single
// preferences form. Without a valid primary owner cookie, only restoration is
// available.
func (h *SearchHandler) OwnerTokensPage(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	data := map[string]interface{}{
		"Title":         "Owner tokens - " + h.appConfig.Server.Branding.Title,
		"ActiveNav":     "preferences",
		"NoJSTemplate":  "owner-tokens",
		"TokensEnabled": false,
		"PrimaryToken":  ownerTokenPageItem{},
		"LinkedTokens":  []ownerTokenPageItem{},
		"PathPrefix":    ownerTokenPathPrefix(r),
	}
	noticeKeys := map[string]string{
		"token-created": "owner_tokens.notice_created",
		"token-rotated": "owner_tokens.notice_rotated",
		"token-renamed": "owner_tokens.notice_renamed",
		"token-revoked": "owner_tokens.notice_revoked",
	}
	if key, ok := noticeKeys[r.URL.Query().Get("notice")]; ok {
		data["Notice"] = i18n.Translate(i18n.DetectLocale(r), key)
	}
	if h.tokenManager != nil {
		if cookie, err := r.Cookie(apitoken.OwnerCookieName); err == nil {
			if principal, authErr := h.tokenManager.Authenticate(r.Context(), cookie.Value); authErr == nil && principal.Kind == "primary" {
				reveal, _ := r.Context().Value(ownerTokenRevealContextKey{}).(OwnerTokenReveal)
				primary, metaErr := h.tokenManager.PrimaryMetadata(r.Context(), principal)
				if metaErr != nil {
					writeGuaranteedErrorPage(w, r, http.StatusInternalServerError)
					return
				}
				linked, listErr := h.tokenManager.ListLinked(r.Context(), principal)
				if listErr != nil {
					writeGuaranteedErrorPage(w, r, http.StatusInternalServerError)
					return
				}
				data["TokensEnabled"] = true
				data["PrimaryToken"] = ownerTokenPageItem{
					Suffix:   primary.Suffix,
					Created:  time.Unix(primary.CreatedAt, 0).UTC().Format(time.RFC3339),
					LastUsed: formatOwnerTokenTime(primary.LastUsed),
				}
				if reveal.Primary {
					item := data["PrimaryToken"].(ownerTokenPageItem)
					item.Revealed = reveal.Token
					data["PrimaryToken"] = item
				}
				items := make([]ownerTokenPageItem, 0, len(linked))
				now := time.Now().Unix()
				for _, token := range linked {
					status, expires := "active", "never"
					if token.RevokedAt.Valid {
						status = "revoked"
					} else if token.ExpiresAt.Valid {
						expires = time.Unix(token.ExpiresAt.Int64, 0).UTC().Format("2006-01-02")
						if token.ExpiresAt.Int64 <= now {
							status = "expired"
						}
					}
					item := ownerTokenPageItem{
						ID: token.ID, Name: token.Name, Suffix: token.Suffix,
						Created:  time.Unix(token.CreatedAt, 0).UTC().Format(time.RFC3339),
						LastUsed: formatOwnerTokenTime(token.LastUsed), Expires: expires, Status: status,
						RevealURL: fmt.Sprintf("%s/server/preferences/tokens/%d/reveal", ownerTokenPathPrefix(r), token.ID),
						RotateURL: fmt.Sprintf("%s/server/preferences/tokens/%d/rotate", ownerTokenPathPrefix(r), token.ID),
						UpdateURL: fmt.Sprintf("%s/server/preferences/tokens/%d/update", ownerTokenPathPrefix(r), token.ID),
						RevokeURL: fmt.Sprintf("%s/server/preferences/tokens/%d/revoke", ownerTokenPathPrefix(r), token.ID),
					}
					if !reveal.Primary && reveal.ID == token.ID {
						item.Revealed = reveal.Token
					}
					items = append(items, item)
				}
				data["LinkedTokens"] = items
			}
		}
	}
	h.renderResponse(w, r, "owner-tokens", data)
}

func formatOwnerTokenTime(value sql.NullInt64) string {
	if !value.Valid {
		return ""
	}
	return time.Unix(value.Int64, 0).UTC().Format(time.RFC3339)
}
