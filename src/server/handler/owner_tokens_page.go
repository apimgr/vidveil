// SPDX-License-Identifier: MIT
package handler

import (
	"database/sql"
	"net/http"
	"time"

	"github.com/apimgr/vidveil/src/server/service/apitoken"
)

type ownerTokenPageItem struct {
	ID       int64
	Name     string
	Suffix   string
	Created  string
	LastUsed string
	Expires  string
	Status   string
}

// OwnerTokensPage renders token management separately from the single
// preferences form. Without a valid primary owner cookie, only restoration is
// available.
func (h *SearchHandler) OwnerTokensPage(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	data := map[string]interface{}{
		"Title":         "Owner tokens - " + h.appConfig.Server.Branding.Title,
		"ActiveNav":     "preferences",
		"TokensEnabled": false,
		"PrimaryToken":  ownerTokenPageItem{},
		"LinkedTokens":  []ownerTokenPageItem{},
	}
	if h.tokenManager != nil {
		if cookie, err := r.Cookie(apitoken.OwnerCookieName); err == nil {
			if principal, authErr := h.tokenManager.Authenticate(r.Context(), cookie.Value); authErr == nil && principal.Kind == "primary" {
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
					items = append(items, ownerTokenPageItem{
						ID: token.ID, Name: token.Name, Suffix: token.Suffix,
						Created:  time.Unix(token.CreatedAt, 0).UTC().Format(time.RFC3339),
						LastUsed: formatOwnerTokenTime(token.LastUsed), Expires: expires, Status: status,
					})
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
