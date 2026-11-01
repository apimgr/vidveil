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

func TestFavoritesExportRequiresOwnerToken(t *testing.T) {
	request := httptest.NewRequest(http.MethodGet, "/favorites/export", nil)
	response := httptest.NewRecorder()

	(&SearchHandler{}).FavoritesExport(response, request)

	if response.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want %d", response.Code, http.StatusForbidden)
	}
}
