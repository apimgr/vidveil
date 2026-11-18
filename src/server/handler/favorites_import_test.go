package handler

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestFavoritesImportRejectsInvalidJSON(t *testing.T) {
	request := httptest.NewRequest(http.MethodPost, "/favorites/import", strings.NewReader("not-json"))
	request.Header.Set("Content-Type", "application/json")
	response := httptest.NewRecorder()

	(&SearchHandler{}).FavoritesImport(response, request)

	if response.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", response.Code, http.StatusBadRequest)
	}
}

func TestDecodeFavoriteImportAcceptsArrayAndObject(t *testing.T) {
	for _, input := range []string{
		`[{"url":"https://example.test/video","title":"Example"}]`,
		`{"favorites":[{"url":"https://example.test/video","title":"Example"}]}`,
	} {
		entries, err := decodeFavoriteImport(strings.NewReader(input))
		if err != nil {
			t.Fatalf("decodeFavoriteImport(%s): %v", input, err)
		}
		if len(entries) != 1 || entries[0].URL != "https://example.test/video" {
			t.Fatalf("decoded entries = %+v", entries)
		}
	}
}

func TestFavoritesExportRequiresOwnerToken(t *testing.T) {
	request := httptest.NewRequest(http.MethodGet, "/favorites/export", nil)
	response := httptest.NewRecorder()

	(&SearchHandler{}).FavoritesExport(response, request)

	if response.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want %d", response.Code, http.StatusForbidden)
	}
}
