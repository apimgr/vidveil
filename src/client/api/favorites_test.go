// SPDX-License-Identifier: MIT
package api

import (
	"encoding/json"
	"net/http"
	"testing"
)

func TestFavoriteAPIRequests(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/api/v1/favorites", func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer tok-test" {
			t.Errorf("authorization = %q, want bearer token", r.Header.Get("Authorization"))
		}
		switch r.Method {
		case http.MethodGet:
			_, _ = w.Write([]byte(`{"ok":true,"data":{"favorites":[{"id":17,"url":"https://example.test/video","title":"Example"}]}}`))
		case http.MethodPost:
			var request Favorite
			if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
				t.Errorf("decode favorite request: %v", err)
			}
			if request.URL != "https://example.test/video" || request.Title != "Example" {
				t.Errorf("favorite request = %#v", request)
			}
			_, _ = w.Write([]byte(`{"ok":true,"data":{"message":"added"}}`))
		case http.MethodDelete:
			_, _ = w.Write([]byte(`{"ok":true,"data":{"message":"cleared"}}`))
		default:
			t.Errorf("unexpected method %s", r.Method)
			http.Error(w, "unexpected method", http.StatusMethodNotAllowed)
		}
	})
	mux.HandleFunc("/api/v1/favorites/17", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodDelete {
			t.Errorf("remove method = %s", r.Method)
		}
		_, _ = w.Write([]byte(`{"ok":true,"data":{"message":"removed"}}`))
	})
	client, _ := newTestServer(t, mux)

	favorites, err := client.ListFavorites()
	if err != nil || len(favorites) != 1 || favorites[0].ID != 17 {
		t.Fatalf("ListFavorites() = %#v, %v", favorites, err)
	}
	if err := client.AddFavorite(Favorite{URL: "https://example.test/video", Title: "Example"}); err != nil {
		t.Fatalf("AddFavorite(): %v", err)
	}
	if err := client.RemoveFavorite(17); err != nil {
		t.Fatalf("RemoveFavorite(): %v", err)
	}
	if err := client.ClearFavorites(); err != nil {
		t.Fatalf("ClearFavorites(): %v", err)
	}
}
