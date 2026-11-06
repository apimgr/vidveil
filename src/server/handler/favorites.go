// SPDX-License-Identifier: MIT
// AI.md PART 16: Favorites — server-side storage. Progressive enhancement per
// AI.md PART 16 ("If it works without JavaScript, ship it. Add JavaScript
// only to enhance") and PART 32's cookie-vs-localStorage split (cookies are
// server-read/authoritative — "anything the server reads to render pages";
// localStorage is JS-only, "never load-bearing"). The owner-token cookie
// carries the owner credential; the favorite rows themselves live in
// server.db, read and written by the server on every request, so the whole
// feature (view/add/remove/clear/export/import) works with JavaScript
// disabled. JS is layered on top only for instant feedback, no reload.
package handler

import (
	"context"
	"crypto/sha256"
	"crypto/subtle"
	"database/sql"
	"encoding/json"
	"fmt"
	"html"
	"io"
	"log"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/apimgr/vidveil/src/common/i18n"
	"github.com/apimgr/vidveil/src/server/service/apitoken"
	"github.com/apimgr/vidveil/src/server/service/urlvar"
	"github.com/go-chi/chi/v5"
)

// Favorite is one server-stored favorite row.
type Favorite struct {
	ID        int64     `json:"id"`
	URL       string    `json:"url"`
	Title     string    `json:"title"`
	Thumbnail string    `json:"thumbnail,omitempty"`
	Source    string    `json:"source,omitempty"`
	CreatedAt time.Time `json:"created_at"`
}

// ownerID resolves the owner for a bearer token or owner cookie and issues
// a primary token when the caller first creates a resource.
func (h *SearchHandler) ownerID(w http.ResponseWriter, r *http.Request, create bool, apiOnly bool) (string, string) {
	if h.tokenManager == nil {
		log.Printf("owner token manager is unavailable while resolving favorite ownership")
		return "", ""
	}
	if header := r.Header.Get("Authorization"); strings.HasPrefix(header, "Bearer ") {
		raw := strings.TrimSpace(strings.TrimPrefix(header, "Bearer "))
		if h.isOperatorToken(raw) {
			if create {
				return h.issuePrimaryOwner(w, r, apiOnly)
			}
			return "*", ""
		}
		principal, err := h.tokenManager.Authenticate(r.Context(), raw)
		if err == nil {
			return principal.OwnerID, ""
		}
		return "", ""
	}
	if apiOnly {
		if create {
			return h.issuePrimaryOwner(w, r, true)
		}
		return "", ""
	}
	if c, err := r.Cookie(apitoken.OwnerCookieName); err == nil && c.Value != "" {
		principal, authErr := h.tokenManager.Authenticate(r.Context(), c.Value)
		if authErr == nil && principal.Kind == "primary" {
			h.setOwnerCookie(w, c.Value)
			return principal.OwnerID, ""
		}
		http.SetCookie(w, &http.Cookie{Name: apitoken.OwnerCookieName, Value: "", Path: "/", MaxAge: -1, HttpOnly: true, Secure: true, SameSite: http.SameSiteStrictMode})
	}
	legacy, _ := r.Cookie("visitor_id")
	if !create && legacy == nil {
		return "", ""
	}
	ownerID, raw := h.issuePrimaryOwner(w, r, false)
	if ownerID == "" {
		return "", ""
	}
	if legacy != nil && legacy.Value != "" && h.healthDB != nil {
		_, _ = h.healthDB.ExecContext(r.Context(), `UPDATE favorites SET owner_id = ? WHERE visitor_id = ? AND owner_id LIKE 'legacy-favorite-%'`, ownerID, legacy.Value)
		http.SetCookie(w, &http.Cookie{Name: "visitor_id", Value: "", Path: "/", MaxAge: -1, HttpOnly: true, Secure: true, SameSite: http.SameSiteStrictMode})
	}
	return ownerID, raw
}

func (h *SearchHandler) isOperatorToken(raw string) bool {
	if h.appConfig == nil || h.appConfig.Server.Token == "" || raw == "" {
		return false
	}
	want := sha256.Sum256([]byte(h.appConfig.Server.Token))
	got := sha256.Sum256([]byte(raw))
	return subtle.ConstantTimeCompare(want[:], got[:]) == 1
}

func ownerIdentityFailureCode(r *http.Request) string {
	if strings.HasPrefix(strings.ToLower(strings.TrimSpace(r.Header.Get("Authorization"))), "bearer ") {
		return "UNAUTHORIZED"
	}
	return "SERVER_ERROR"
}

func sendFavoriteBearerFailure(w http.ResponseWriter, r *http.Request) {
	header := strings.TrimSpace(r.Header.Get("Authorization"))
	if strings.HasPrefix(strings.ToLower(header), "bearer ") && strings.TrimSpace(header[len("Bearer "):]) != "" {
		SetBearerChallenge(w, "invalid_token")
	} else {
		SetBearerChallenge(w, "")
	}
	SendError(w, "UNAUTHORIZED", "Authentication required")
}

func (h *SearchHandler) issuePrimaryOwner(w http.ResponseWriter, r *http.Request, apiOnly bool) (string, string) {
	issued, err := h.tokenManager.IssuePrimary(r.Context())
	if err != nil {
		log.Printf("owner token issuance failed: %v", err)
		return "", ""
	}
	if h.ownerAudit != nil {
		h.ownerAudit.Audit("token.created", issued.OwnerID, "owner", urlvar.ResolveClientIP(r), "success", map[string]interface{}{"owner_id": issued.OwnerID, "token_prefix": issued.Prefix})
	}
	if !apiOnly {
		h.setOwnerCookie(w, issued.Token)
	}
	return issued.OwnerID, issued.Token
}

func (h *SearchHandler) setOwnerCookie(w http.ResponseWriter, token string) {
	maxAge := apitoken.CookieMaxAge(h.appConfig.Server.Tokens.CookieMaxAge)
	http.SetCookie(w, &http.Cookie{Name: apitoken.OwnerCookieName, Value: token, Path: "/", MaxAge: maxAge, HttpOnly: true, Secure: true, SameSite: http.SameSiteStrictMode})
}

// listFavorites reads all favorites for a visitor, newest first.
func (h *SearchHandler) listFavorites(ownerID string) ([]Favorite, error) {
	if h.healthDB == nil || ownerID == "" {
		return nil, sql.ErrConnDone
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	query := `SELECT id, url, title, thumbnail, source, created_at FROM favorites WHERE owner_id = ? ORDER BY created_at DESC`
	args := []any{ownerID}
	if ownerID == "*" {
		query = `SELECT id, url, title, thumbnail, source, created_at FROM favorites ORDER BY created_at DESC`
		args = nil
	}
	rows, err := h.healthDB.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	var out []Favorite
	for rows.Next() {
		var f Favorite
		var thumb, source sql.NullString
		if err := rows.Scan(&f.ID, &f.URL, &f.Title, &thumb, &source, &f.CreatedAt); err != nil {
			return nil, err
		}
		f.Thumbnail = thumb.String
		f.Source = source.String
		out = append(out, f)
	}
	return out, rows.Err()
}

// addFavorite inserts a favorite for an owner. Duplicate url (per the
// UNIQUE(visitor_id, url) compatibility constraint) is treated as success — favoriting an
// already-favorited video is idempotent, not an error.
func (h *SearchHandler) addFavorite(ownerID, url, title, thumbnail, source string) error {
	if h.healthDB == nil || ownerID == "" {
		return sql.ErrConnDone
	}
	if url == "" || title == "" {
		return sql.ErrNoRows
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	_, err := h.healthDB.ExecContext(ctx,
		`INSERT OR IGNORE INTO favorites (owner_id, visitor_id, url, title, thumbnail, source) VALUES (?, ?, ?, ?, ?, ?)`,
		ownerID, ownerID, url, title, thumbnail, source)
	return err
}

// removeFavorite deletes one favorite by id, scoped to its owner.
func (h *SearchHandler) removeFavorite(ownerID string, id int64) error {
	if h.healthDB == nil || ownerID == "" {
		return sql.ErrConnDone
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	query := `DELETE FROM favorites WHERE id = ? AND owner_id = ?`
	args := []any{id, ownerID}
	if ownerID == "*" {
		query = `DELETE FROM favorites WHERE id = ?`
		args = []any{id}
	}
	_, err := h.healthDB.ExecContext(ctx, query, args...)
	return err
}

// removeFavoriteByURL deletes one favorite by URL, scoped to the visitor —
// used by the no-JS favorite-toggle form on the search results page, which
// only knows the video URL, not the row id.
func (h *SearchHandler) removeFavoriteByURL(ownerID, url string) error {
	if h.healthDB == nil || ownerID == "" {
		return sql.ErrConnDone
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	query := `DELETE FROM favorites WHERE url = ? AND owner_id = ?`
	args := []any{url, ownerID}
	if ownerID == "*" {
		query = `DELETE FROM favorites WHERE url = ?`
		args = []any{url}
	}
	_, err := h.healthDB.ExecContext(ctx, query, args...)
	return err
}

// clearFavorites deletes every favorite belonging to an owner.
func (h *SearchHandler) clearFavorites(ownerID string) error {
	if h.healthDB == nil || ownerID == "" {
		return sql.ErrConnDone
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	query := `DELETE FROM favorites WHERE owner_id = ?`
	args := []any{ownerID}
	if ownerID == "*" {
		query = `DELETE FROM favorites`
		args = nil
	}
	_, err := h.healthDB.ExecContext(ctx, query, args...)
	return err
}

// FavoritesPage handles GET /favorites — the no-JS-first favorites list.
// Server queries and renders the actual data (AI.md PART 16: "the server
// does the work, the client displays the result"); JS enhances the same
// markup afterward via the JSON data island for instant add/remove feedback.
func (h *SearchHandler) FavoritesPage(w http.ResponseWriter, r *http.Request) {
	ownerID, _ := h.ownerID(w, r, false, false)
	favs, err := h.listFavorites(ownerID)
	if err != nil {
		favs = nil
	}

	switch detectResponseFormat(r) {
	case "application/json":
		SendOK(w, map[string]interface{}{"favorites": favs})

	default:
		h.renderResponse(w, r, "favorites", map[string]interface{}{
			"Title":         "Favorites - " + h.appConfig.Server.Branding.Title,
			"Theme":         h.getRequestTheme(r),
			"ActiveNav":     "favorites",
			"Favorites":     favs,
			"BuildDateTime": BuildDateTime(),
		})
	}
}

// FavoritesSave handles POST /favorites — the single no-JS mutation endpoint
// for add/remove/clear, dispatched via the `_method` hidden field per
// frontend-rules.md ("HTML forms with `_method` hidden field for
// PUT/PATCH/DELETE"). No `_method` field means "add". Always redirects back
// to /favorites (POST-redirect-GET) so no-JS clients get a fresh server
// render, and returns JSON for JS/API callers.
func (h *SearchHandler) FavoritesSave(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Redirect(w, r, "/favorites", http.StatusFound)
		return
	}
	if err := r.ParseForm(); err != nil {
		SendError(w, "BAD_REQUEST", "invalid form data")
		return
	}

	ownerID, newToken := h.ownerID(w, r, r.FormValue("_method") != "DELETE", false)
	if ownerID == "" {
		if detectResponseFormat(r) != "application/json" && !isOurCliClient(r) {
			redirectTo := safeLocalRedirect(r.FormValue("redirect"))
			if redirectTo == "/" {
				redirectTo = "/favorites"
			}
			http.Redirect(w, r, redirectTo, http.StatusFound)
			return
		}
		SendError(w, "SERVER_ERROR", "failed to establish owner identity")
		return
	}

	var opErr error
	switch r.FormValue("_method") {
	case "DELETE":
		if idStr := r.FormValue("id"); idStr != "" {
			if id, err := strconv.ParseInt(idStr, 10, 64); err == nil {
				opErr = h.removeFavorite(ownerID, id)
			}
		} else if url := r.FormValue("url"); url != "" {
			opErr = h.removeFavoriteByURL(ownerID, url)
		} else {
			opErr = h.clearFavorites(ownerID)
		}
	default:
		opErr = h.addFavorite(ownerID, r.FormValue("url"), r.FormValue("title"), r.FormValue("thumbnail"), r.FormValue("source"))
	}

	if detectResponseFormat(r) == "application/json" || isOurCliClient(r) {
		if opErr != nil {
			SendError(w, "SERVER_ERROR", "failed to update favorites")
			return
		}
		result := map[string]interface{}{"message": "ok"}
		if newToken != "" {
			result["owner_token"] = newToken
		}
		SendOK(w, result)
		return
	}
	if newToken != "" {
		w.Header().Set("Cache-Control", "no-store")
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		locale := i18n.DetectLocale(r)
		_, _ = fmt.Fprintf(w, `<!doctype html><html lang="%s" dir="%s"><meta charset="utf-8"><title>%s</title><main><h1>%s</h1><p>%s</p><code class="owner-token-value" data-owner-token-value>%s</code><p><button type="button" data-copy-owner-token data-copy-success="%s" data-copy-failed="%s">%s</button> <span role="status" aria-live="polite"></span></p><p>%s</p><p><a href="/favorites">%s</a></p></main><script src="/static/js/app.js"></script></html>`, html.EscapeString(locale), i18n.Direction(locale), html.EscapeString(i18n.Translate(locale, "owner_tokens.created_title")), html.EscapeString(i18n.Translate(locale, "owner_tokens.created_title")), html.EscapeString(i18n.Translate(locale, "owner_tokens.created_description")), html.EscapeString(newToken), html.EscapeString(i18n.Translate(locale, "owner_tokens.copy_success")), html.EscapeString(i18n.Translate(locale, "owner_tokens.copy_failed")), html.EscapeString(i18n.Translate(locale, "owner_tokens.copy")), html.EscapeString(i18n.Translate(locale, "owner_tokens.token_viewable")), html.EscapeString(i18n.Translate(locale, "owner_tokens.continue_favorites")))
		return
	}

	// The favorite-toggle form on the search results page (and any other
	// non-/favorites page) submits a same-origin relative "redirect" field so
	// the visitor lands back where they were, not on /favorites. Only a
	// same-origin relative path is ever honored (open-redirect guard);
	// safeLocalRedirect also rejects protocol-relative "//host" and "/\host".
	redirectTo := "/favorites"
	if rt := safeLocalRedirect(r.FormValue("redirect")); rt != "/" {
		redirectTo = rt
	}
	http.Redirect(w, r, redirectTo, http.StatusFound)
}

// FavoritesExport handles GET /favorites/export — downloads the visitor's
// favorites as a JSON file, matching the shape FavoritesImport accepts.
func (h *SearchHandler) FavoritesExport(w http.ResponseWriter, r *http.Request) {
	ownerID, _ := h.ownerID(w, r, false, false)
	if ownerID == "" {
		SendError(w, "FORBIDDEN", "A valid owner token is required")
		return
	}
	favs, err := h.listFavorites(ownerID)
	if err != nil {
		SendNegotiatedError(w, r, "SERVER_ERROR", "failed to export favorites")
		return
	}
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("Content-Disposition", `attachment; filename="favorites.json"`)
	WriteJSON(w, http.StatusOK, favs)
}

// importFavoriteEntry is the shape accepted by FavoritesImport, matching
// FavoritesExport's output and Favorite's JSON tags.
type importFavoriteEntry struct {
	URL       string `json:"url"`
	Title     string `json:"title"`
	Thumbnail string `json:"thumbnail"`
	Source    string `json:"source"`
}

// FavoritesImport handles POST /favorites/import — restores favorites from a
// previously exported JSON file (multipart form field "file") or a raw JSON
// body for API callers. Existing favorites are kept; imported entries are
// added alongside them (duplicates by url are ignored per the INSERT OR
// IGNORE in addFavorite).
func (h *SearchHandler) FavoritesImport(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Redirect(w, r, "/favorites", http.StatusFound)
		return
	}
	var reader io.Reader = r.Body
	var uploadedFile io.ReadCloser
	if strings.HasPrefix(r.Header.Get("Content-Type"), "multipart/form-data") {
		file, _, err := r.FormFile("file")
		if err != nil {
			SendNegotiatedError(w, r, "BAD_REQUEST", "favorites file is required")
			return
		}
		uploadedFile = file
		reader = file
	}

	var entries []importFavoriteEntry
	decoder := json.NewDecoder(reader)
	decodeErr := decoder.Decode(&entries)
	if uploadedFile != nil {
		if err := uploadedFile.Close(); decodeErr == nil && err != nil {
			decodeErr = err
		}
	}
	if decodeErr != nil {
		SendNegotiatedError(w, r, "BAD_REQUEST", "invalid favorites JSON")
		return
	}
	var trailing any
	if err := decoder.Decode(&trailing); err != io.EOF {
		SendNegotiatedError(w, r, "BAD_REQUEST", "favorites JSON must contain one array")
		return
	}
	for _, entry := range entries {
		if entry.URL == "" || entry.Title == "" {
			SendNegotiatedError(w, r, "VALIDATION_FAILED", "each favorite requires a URL and title")
			return
		}
	}
	apiOnly := strings.HasPrefix(r.URL.Path, "/api/")
	ownerID, ownerToken := h.ownerID(w, r, true, apiOnly)
	if ownerID == "" {
		SendNegotiatedError(w, r, ownerIdentityFailureCode(r), "A valid owner token is required")
		return
	}
	if h.healthDB == nil {
		SendNegotiatedError(w, r, "SERVER_ERROR", "failed to import favorites")
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()
	tx, err := h.healthDB.BeginTx(ctx, nil)
	if err != nil {
		SendNegotiatedError(w, r, "SERVER_ERROR", "failed to import favorites")
		return
	}
	defer func() { _ = tx.Rollback() }()
	for _, entry := range entries {
		if _, err := tx.ExecContext(ctx,
			`INSERT OR IGNORE INTO favorites (owner_id, visitor_id, url, title, thumbnail, source) VALUES (?, ?, ?, ?, ?, ?)`,
			ownerID, ownerID, entry.URL, entry.Title, entry.Thumbnail, entry.Source); err != nil {
			SendNegotiatedError(w, r, "SERVER_ERROR", "failed to import favorites")
			return
		}
	}
	if err := tx.Commit(); err != nil {
		SendNegotiatedError(w, r, "SERVER_ERROR", "failed to import favorites")
		return
	}

	if detectResponseFormat(r) == "application/json" || isOurCliClient(r) {
		result := map[string]interface{}{"imported": len(entries)}
		if ownerToken != "" {
			result["owner_token"] = ownerToken
		}
		SendOK(w, result)
		return
	}
	if ownerToken != "" {
		w.Header().Set("Cache-Control", "no-store")
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		locale := i18n.DetectLocale(r)
		_, _ = fmt.Fprintf(w, `<!doctype html><html lang="%s" dir="%s"><meta charset="utf-8"><title>%s</title><main><h1>%s</h1><p>%s</p><code class="owner-token-value" data-owner-token-value>%s</code><p><button type="button" data-copy-owner-token data-copy-success="%s" data-copy-failed="%s">%s</button> <span role="status" aria-live="polite"></span></p><p><a href="/favorites">%s</a></p></main><script src="/static/js/app.js"></script></html>`, html.EscapeString(locale), i18n.Direction(locale), html.EscapeString(i18n.Translate(locale, "owner_tokens.created_title")), html.EscapeString(i18n.Translate(locale, "owner_tokens.created_title")), html.EscapeString(i18n.Translate(locale, "owner_tokens.imported_description")), html.EscapeString(ownerToken), html.EscapeString(i18n.Translate(locale, "owner_tokens.copy_success")), html.EscapeString(i18n.Translate(locale, "owner_tokens.copy_failed")), html.EscapeString(i18n.Translate(locale, "owner_tokens.copy")), html.EscapeString(i18n.Translate(locale, "owner_tokens.continue_favorites")))
		return
	}
	http.Redirect(w, r, "/favorites", http.StatusFound)
}

// FavoritesAPIList handles GET /api/{api_version}/favorites.
func (h *SearchHandler) FavoritesAPIList(w http.ResponseWriter, r *http.Request) {
	ownerID, _ := h.ownerID(w, r, false, true)
	if ownerID == "" {
		sendFavoriteBearerFailure(w, r)
		return
	}
	favs, err := h.listFavorites(ownerID)
	if err != nil {
		SendError(w, "SERVER_ERROR", "failed to load favorites")
		return
	}
	SendOK(w, map[string]interface{}{"favorites": favs})
}

// FavoritesAPIAdd handles POST /api/{api_version}/favorites.
func (h *SearchHandler) FavoritesAPIAdd(w http.ResponseWriter, r *http.Request) {
	var in importFavoriteEntry
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
		SendError(w, "BAD_REQUEST", "invalid JSON body")
		return
	}
	if strings.TrimSpace(in.URL) == "" || strings.TrimSpace(in.Title) == "" {
		SendError(w, "VALIDATION_FAILED", "url and title are required")
		return
	}
	ownerID, raw := h.ownerID(w, r, true, true)
	if ownerID == "" {
		if ownerIdentityFailureCode(r) == "UNAUTHORIZED" {
			sendFavoriteBearerFailure(w, r)
			return
		}
		SendError(w, ownerIdentityFailureCode(r), "failed to establish owner identity")
		return
	}
	if err := h.addFavorite(ownerID, in.URL, in.Title, in.Thumbnail, in.Source); err != nil {
		SendError(w, "VALIDATION_FAILED", "url and title are required")
		return
	}
	result := map[string]interface{}{"message": "added"}
	if raw != "" {
		result["owner_token"] = raw
	}
	SendOK(w, result)
}

// FavoritesAPIRemove handles DELETE /api/{api_version}/favorites/{id}.
func (h *SearchHandler) FavoritesAPIRemove(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		SendError(w, "BAD_REQUEST", "invalid id")
		return
	}
	ownerID, _ := h.ownerID(w, r, false, true)
	if ownerID == "" {
		sendFavoriteBearerFailure(w, r)
		return
	}
	if err := h.removeFavorite(ownerID, id); err != nil {
		SendError(w, "SERVER_ERROR", "failed to remove favorite")
		return
	}
	SendOK(w, map[string]interface{}{"message": "removed"})
}

// FavoritesAPIClear handles DELETE /api/{api_version}/favorites.
func (h *SearchHandler) FavoritesAPIClear(w http.ResponseWriter, r *http.Request) {
	ownerID, _ := h.ownerID(w, r, false, true)
	if ownerID == "" {
		sendFavoriteBearerFailure(w, r)
		return
	}
	if err := h.clearFavorites(ownerID); err != nil {
		SendError(w, "SERVER_ERROR", "failed to clear favorites")
		return
	}
	SendOK(w, map[string]interface{}{"message": "cleared"})
}
