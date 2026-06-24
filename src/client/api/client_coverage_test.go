// SPDX-License-Identifier: MIT
// Coverage tests for APIClient HTTP methods using httptest.NewServer.
// Targets: Search, GetVersion, Autodiscover, Health, FetchURLResponseBytes, get.
package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

// newTestServer starts an httptest.Server with the given handler and returns
// a client pointing at it. Caller must call ts.Close() via t.Cleanup.
func newTestServer(t *testing.T, mux *http.ServeMux) (*APIClient, *httptest.Server) {
	t.Helper()
	ts := httptest.NewServer(mux)
	t.Cleanup(ts.Close)
	return NewAPIClient(ts.URL, "tok-test", 5, "v1"), ts
}

// ── Search ───────────────────────────────────────────────────────────────────

func TestSearch_ReturnsResults(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/api/v1/search", func(w http.ResponseWriter, r *http.Request) {
		resp := SearchResponse{Ok: true, Query: "cats", Count: 1, Results: []SearchResult{
			{Title: "Cats Video", URL: "https://example.com/cats"},
		}}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	})
	client, _ := newTestServer(t, mux)

	result, err := client.Search("cats", 1, 10, nil)
	if err != nil {
		t.Fatalf("Search: %v", err)
	}
	if result.Query != "cats" {
		t.Errorf("Search: query = %q, want cats", result.Query)
	}
	if len(result.Results) != 1 {
		t.Errorf("Search: result count = %d, want 1", len(result.Results))
	}
}

func TestSearch_WithEngines(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/api/v1/search", func(w http.ResponseWriter, r *http.Request) {
		engines := r.URL.Query()["engines"]
		if len(engines) == 0 {
			http.Error(w, "missing engines", 400)
			return
		}
		resp := SearchResponse{Ok: true, Query: r.URL.Query().Get("q")}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	})
	client, _ := newTestServer(t, mux)

	result, err := client.Search("dogs", 0, 0, []string{"youtube", "vimeo"})
	if err != nil {
		t.Fatalf("Search with engines: %v", err)
	}
	if !result.Ok {
		t.Error("Search with engines: expected Ok=true")
	}
}

func TestSearch_ServerError(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/api/v1/search", func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "server error", 500)
	})
	client, _ := newTestServer(t, mux)

	_, err := client.Search("fail", 0, 0, nil)
	if err == nil {
		t.Error("Search 500: expected error, got nil")
	}
}

// ── GetVersion ───────────────────────────────────────────────────────────────

func TestGetVersion_ReturnsVersion(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/api/v1/version", func(w http.ResponseWriter, r *http.Request) {
		resp := VersionResponse{Ok: true, Version: "1.2.3", Commit: "abc", Built: "2026-01-01"}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	})
	client, _ := newTestServer(t, mux)

	ver, err := client.GetVersion()
	if err != nil {
		t.Fatalf("GetVersion: %v", err)
	}
	if ver.Version != "1.2.3" {
		t.Errorf("GetVersion: version = %q, want 1.2.3", ver.Version)
	}
}

func TestGetVersion_ServerError(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/api/v1/version", func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "not found", 404)
	})
	client, _ := newTestServer(t, mux)

	_, err := client.GetVersion()
	if err == nil {
		t.Error("GetVersion 404: expected error, got nil")
	}
}

// ── Autodiscover ─────────────────────────────────────────────────────────────

func TestAutodiscover_ReturnsConfig(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/api/autodiscover", func(w http.ResponseWriter, r *http.Request) {
		resp := AutodiscoverResponse{Primary: "http://node1:8080", APIVersion: "v1", Timeout: 30}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	})
	client, _ := newTestServer(t, mux)

	ad, err := client.Autodiscover()
	if err != nil {
		t.Fatalf("Autodiscover: %v", err)
	}
	if ad.APIVersion != "v1" {
		t.Errorf("Autodiscover: APIVersion = %q, want v1", ad.APIVersion)
	}
}

func TestAutodiscover_ServerError(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/api/autodiscover", func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "unavailable", http.StatusServiceUnavailable)
	})
	client, _ := newTestServer(t, mux)

	_, err := client.Autodiscover()
	if err == nil {
		t.Error("Autodiscover 503: expected error, got nil")
	}
}

// ── Health ───────────────────────────────────────────────────────────────────

func TestHealth_Returns200WhenUp(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/api/v1/healthz", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	client, _ := newTestServer(t, mux)

	healthy, err := client.Health()
	if err != nil {
		t.Fatalf("Health: %v", err)
	}
	if !healthy {
		t.Error("Health: expected true for 200 response")
	}
}

func TestHealth_Returns503WhenDown(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/api/v1/healthz", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusServiceUnavailable)
	})
	client, _ := newTestServer(t, mux)

	healthy, err := client.Health()
	if err != nil {
		t.Fatalf("Health 503: %v", err)
	}
	if healthy {
		t.Error("Health: expected false for 503 response")
	}
}

// ── FetchURLResponseBytes ─────────────────────────────────────────────────────

func TestFetchURLResponseBytes_ReturnsBody(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/some/path", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"key":"value"}`))
	})
	client, ts := newTestServer(t, mux)

	body, err := client.FetchURLResponseBytes(ts.URL + "/some/path")
	if err != nil {
		t.Fatalf("FetchURLResponseBytes: %v", err)
	}
	if string(body) != `{"key":"value"}` {
		t.Errorf("FetchURLResponseBytes: body = %q, want {\"key\":\"value\"}", string(body))
	}
}

func TestFetchURLResponseBytes_WithToken(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/secure", func(w http.ResponseWriter, r *http.Request) {
		auth := r.Header.Get("Authorization")
		if auth != "Bearer tok-test" {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		w.Write([]byte(`"ok"`))
	})
	client, ts := newTestServer(t, mux)

	_, err := client.FetchURLResponseBytes(ts.URL + "/secure")
	if err != nil {
		t.Fatalf("FetchURLResponseBytes with token: %v", err)
	}
}

func TestFetchURLResponseBytes_404ReturnsError(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/notfound", func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "not found", 404)
	})
	client, ts := newTestServer(t, mux)

	_, err := client.FetchURLResponseBytes(ts.URL + "/notfound")
	if err == nil {
		t.Error("FetchURLResponseBytes 404: expected error, got nil")
	}
}

// ── get — error paths ────────────────────────────────────────────────────────

func TestGet_InvalidJSONReturnsError(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/api/v1/version", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{invalid json`))
	})
	client, _ := newTestServer(t, mux)

	_, err := client.GetVersion()
	if err == nil {
		t.Error("get with invalid JSON: expected error, got nil")
	}
}

func TestGet_SendsAuthorizationHeader(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/api/v1/version", func(w http.ResponseWriter, r *http.Request) {
		auth := r.Header.Get("Authorization")
		if auth == "" {
			http.Error(w, "no auth", http.StatusUnauthorized)
			return
		}
		resp := VersionResponse{Ok: true, Version: "1.0.0"}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	})
	client, _ := newTestServer(t, mux)

	_, err := client.GetVersion()
	if err != nil {
		t.Fatalf("get auth header test: %v", err)
	}
}
