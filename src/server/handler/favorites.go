// SPDX-License-Identifier: MIT
// AI.md PART 16: Favorites — server-side storage. Progressive enhancement per
// AI.md PART 16 ("If it works without JavaScript, ship it. Add JavaScript
// only to enhance") and PART 32's cookie-vs-localStorage split (cookies are
// server-read/authoritative — "anything the server reads to render pages";
// localStorage is JS-only, "never load-bearing"). The visitor_id cookie
// carries only an opaque identifier; the favorite rows themselves live in
// server.db, read and written by the server on every request, so the whole
// feature (view/add/remove/clear/export/import) works with JavaScript
// disabled. JS is layered on top only for instant feedback, no reload.
package handler

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
)

// favoritesVisitorCookieName holds an opaque, server-issued visitor
// identifier — never the favorite data itself. Long-lived (1 year) so
// returning anonymous visitors keep their list without an account.
const favoritesVisitorCookieName = "visitor_id"

// Favorite is one server-stored favorite row.
type Favorite struct {
	ID        int64     `json:"id"`
	URL       string    `json:"url"`
	Title     string    `json:"title"`
	Thumbnail string    `json:"thumbnail,omitempty"`
	Source    string    `json:"source,omitempty"`
	CreatedAt time.Time `json:"created_at"`
}

// getOrCreateVisitorID returns the caller's visitor_id, issuing and setting a
// new cookie when absent. Called at the top of every favorites handler so
// GET (list) and POST (mutate) requests share identity consistently.
func (h *SearchHandler) getOrCreateVisitorID(w http.ResponseWriter, r *http.Request) string {
	if c, err := r.Cookie(favoritesVisitorCookieName); err == nil && c.Value != "" {
		return c.Value
	}
	id := newVisitorID()
	sslEnabled := h.appConfig != nil && h.appConfig.Server.SSL.Enabled
	http.SetCookie(w, newSecureCookie(favoritesVisitorCookieName, id, "/", 365*24*60*60, sslEnabled))
	return id
}

// newVisitorID returns a random 32-hex-char opaque identifier — carries no
// personal data, cannot be reversed to derive anything about the visitor.
func newVisitorID() string {
	b := make([]byte, 16)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}

// listFavorites reads all favorites for a visitor, newest first.
func (h *SearchHandler) listFavorites(visitorID string) ([]Favorite, error) {
	if h.healthDB == nil {
		return nil, sql.ErrConnDone
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	rows, err := h.healthDB.QueryContext(ctx,
		`SELECT id, url, title, thumbnail, source, created_at FROM favorites WHERE visitor_id = ? ORDER BY created_at DESC`,
		visitorID)
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

// addFavorite inserts a favorite for a visitor. Duplicate url (per the
// UNIQUE(visitor_id, url) constraint) is treated as success — favoriting an
// already-favorited video is idempotent, not an error.
func (h *SearchHandler) addFavorite(visitorID, url, title, thumbnail, source string) error {
	if h.healthDB == nil {
		return sql.ErrConnDone
	}
	if url == "" || title == "" {
		return sql.ErrNoRows
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	_, err := h.healthDB.ExecContext(ctx,
		`INSERT OR IGNORE INTO favorites (visitor_id, url, title, thumbnail, source) VALUES (?, ?, ?, ?, ?)`,
		visitorID, url, title, thumbnail, source)
	return err
}

// removeFavorite deletes one favorite by id, scoped to the visitor so one
// visitor can never delete another visitor's row via a guessed id.
func (h *SearchHandler) removeFavorite(visitorID string, id int64) error {
	if h.healthDB == nil {
		return sql.ErrConnDone
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	_, err := h.healthDB.ExecContext(ctx,
		`DELETE FROM favorites WHERE id = ? AND visitor_id = ?`, id, visitorID)
	return err
}

// removeFavoriteByURL deletes one favorite by URL, scoped to the visitor —
// used by the no-JS favorite-toggle form on the search results page, which
// only knows the video URL, not the row id.
func (h *SearchHandler) removeFavoriteByURL(visitorID, url string) error {
	if h.healthDB == nil {
		return sql.ErrConnDone
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	_, err := h.healthDB.ExecContext(ctx,
		`DELETE FROM favorites WHERE url = ? AND visitor_id = ?`, url, visitorID)
	return err
}

// clearFavorites deletes every favorite belonging to a visitor.
func (h *SearchHandler) clearFavorites(visitorID string) error {
	if h.healthDB == nil {
		return sql.ErrConnDone
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	_, err := h.healthDB.ExecContext(ctx, `DELETE FROM favorites WHERE visitor_id = ?`, visitorID)
	return err
}

// FavoritesPage handles GET /favorites — the no-JS-first favorites list.
// Server queries and renders the actual data (AI.md PART 16: "the server
// does the work, the client displays the result"); JS enhances the same
// markup afterward via the JSON data island for instant add/remove feedback.
func (h *SearchHandler) FavoritesPage(w http.ResponseWriter, r *http.Request) {
	visitorID := h.getOrCreateVisitorID(w, r)
	favs, err := h.listFavorites(visitorID)
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

	visitorID := h.getOrCreateVisitorID(w, r)

	var opErr error
	switch r.FormValue("_method") {
	case "DELETE":
		if idStr := r.FormValue("id"); idStr != "" {
			if id, err := strconv.ParseInt(idStr, 10, 64); err == nil {
				opErr = h.removeFavorite(visitorID, id)
			}
		} else if url := r.FormValue("url"); url != "" {
			opErr = h.removeFavoriteByURL(visitorID, url)
		} else {
			opErr = h.clearFavorites(visitorID)
		}
	default:
		opErr = h.addFavorite(visitorID, r.FormValue("url"), r.FormValue("title"), r.FormValue("thumbnail"), r.FormValue("source"))
	}

	if detectResponseFormat(r) == "application/json" || isOurCliClient(r) {
		if opErr != nil {
			SendError(w, "SERVER_ERROR", "failed to update favorites")
			return
		}
		SendOK(w, map[string]interface{}{"message": "ok"})
		return
	}

	// The favorite-toggle form on the search results page (and any other
	// non-/favorites page) submits a same-origin relative "redirect" field so
	// the visitor lands back where they were, not on /favorites. Only a
	// same-origin relative path is ever honored (open-redirect guard).
	redirectTo := "/favorites"
	if rt := r.FormValue("redirect"); rt != "" && strings.HasPrefix(rt, "/") && !strings.HasPrefix(rt, "//") {
		redirectTo = rt
	}
	http.Redirect(w, r, redirectTo, http.StatusFound)
}

// FavoritesExport handles GET /favorites/export — downloads the visitor's
// favorites as a JSON file, matching the shape FavoritesImport accepts.
func (h *SearchHandler) FavoritesExport(w http.ResponseWriter, r *http.Request) {
	visitorID := h.getOrCreateVisitorID(w, r)
	favs, err := h.listFavorites(visitorID)
	if err != nil {
		favs = nil
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
	visitorID := h.getOrCreateVisitorID(w, r)

	var entries []importFavoriteEntry
	if file, _, err := r.FormFile("file"); err == nil {
		defer func() { _ = file.Close() }()
		_ = json.NewDecoder(file).Decode(&entries)
	} else {
		_ = json.NewDecoder(r.Body).Decode(&entries)
	}

	for _, e := range entries {
		_ = h.addFavorite(visitorID, e.URL, e.Title, e.Thumbnail, e.Source)
	}

	if detectResponseFormat(r) == "application/json" || isOurCliClient(r) {
		SendOK(w, map[string]interface{}{"imported": len(entries)})
		return
	}
	http.Redirect(w, r, "/favorites", http.StatusFound)
}

// FavoritesAPIList handles GET /api/{api_version}/favorites.
func (h *SearchHandler) FavoritesAPIList(w http.ResponseWriter, r *http.Request) {
	visitorID := h.getOrCreateVisitorID(w, r)
	favs, err := h.listFavorites(visitorID)
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
	visitorID := h.getOrCreateVisitorID(w, r)
	if err := h.addFavorite(visitorID, in.URL, in.Title, in.Thumbnail, in.Source); err != nil {
		SendError(w, "VALIDATION_FAILED", "url and title are required")
		return
	}
	SendOK(w, map[string]interface{}{"message": "added"})
}

// FavoritesAPIRemove handles DELETE /api/{api_version}/favorites/{id}.
func (h *SearchHandler) FavoritesAPIRemove(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		SendError(w, "BAD_REQUEST", "invalid id")
		return
	}
	visitorID := h.getOrCreateVisitorID(w, r)
	if err := h.removeFavorite(visitorID, id); err != nil {
		SendError(w, "SERVER_ERROR", "failed to remove favorite")
		return
	}
	SendOK(w, map[string]interface{}{"message": "removed"})
}

// FavoritesAPIClear handles DELETE /api/{api_version}/favorites.
func (h *SearchHandler) FavoritesAPIClear(w http.ResponseWriter, r *http.Request) {
	visitorID := h.getOrCreateVisitorID(w, r)
	if err := h.clearFavorites(visitorID); err != nil {
		SendError(w, "SERVER_ERROR", "failed to clear favorites")
		return
	}
	SendOK(w, map[string]interface{}{"message": "cleared"})
}
