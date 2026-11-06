// SPDX-License-Identifier: MIT
package server

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/apimgr/vidveil/src/config"
)

func TestCORSMiddlewareOriginPolicies(t *testing.T) {
	tests := []struct {
		name            string
		cors            config.CORSConfig
		wantOrigin      string
		wantCredentials string
		wantPreflight   bool
	}{
		{
			name:       "wildcard disables credentials",
			cors:       config.CORSConfig{AllowedOrigins: []string{"*"}, AllowCredentials: true, MaxAge: 86400},
			wantOrigin: "*",
		},
		{
			name:       "explicit origin allows configured credentials",
			cors:       config.CORSConfig{AllowedOrigins: []string{"https://app.example.com"}, AllowCredentials: true, MaxAge: 86400},
			wantOrigin: "https://app.example.com", wantCredentials: "true",
		},
		{
			name: "empty sentinel disables CORS",
			cors: config.CORSConfig{AllowedOrigins: []string{""}, AllowCredentials: true, MaxAge: 86400},
		},
		{
			name:       "preflight exposes supported authorization headers",
			cors:       config.CORSConfig{AllowedOrigins: []string{"https://app.example.com"}, AllowCredentials: true, MaxAge: 86400},
			wantOrigin: "https://app.example.com", wantCredentials: "true", wantPreflight: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := &Server{appConfig: &config.AppConfig{Server: config.ServerConfig{CORS: tt.cors}}}
			called := false
			handler := s.corsMiddleware(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				called = true
				w.WriteHeader(http.StatusNoContent)
			}))
			method := http.MethodGet
			if tt.wantPreflight {
				method = http.MethodOptions
			}
			req := httptest.NewRequest(method, "/api/v1/search", nil)
			req.Header.Set("Origin", "https://app.example.com")
			if tt.wantPreflight {
				req.Header.Set("Access-Control-Request-Method", "POST")
				req.Header.Set("Access-Control-Request-Headers", "authorization,x-csrf-token")
			}
			res := httptest.NewRecorder()
			handler.ServeHTTP(res, req)

			if got := res.Header().Get("Access-Control-Allow-Origin"); got != tt.wantOrigin {
				t.Errorf("Allow-Origin = %q, want %q", got, tt.wantOrigin)
			}
			if got := res.Header().Get("Access-Control-Allow-Credentials"); got != tt.wantCredentials {
				t.Errorf("Allow-Credentials = %q, want %q", got, tt.wantCredentials)
			}
			if tt.wantPreflight {
				if res.Code != http.StatusNoContent {
					t.Errorf("preflight status = %d, want %d", res.Code, http.StatusNoContent)
				}
				if got := strings.ToLower(res.Header().Get("Access-Control-Allow-Headers")); !strings.Contains(got, "authorization") || !strings.Contains(got, "x-csrf-token") || strings.Contains(got, "x-api-key") {
					t.Errorf("preflight allowed headers = %q, want Authorization and X-CSRF-Token without alternate credentials", got)
				}
			} else if tt.wantOrigin == "" && !called {
				t.Error("disabled CORS should pass through to the route")
			}
		})
	}
}

func TestAppendUniqueOriginRejectsUnsafeValues(t *testing.T) {
	got := appendUniqueOrigin(nil, "https://example.com/path")
	if len(got) != 0 {
		t.Fatalf("unsafe origin was accepted: %#v", got)
	}
	got = appendUniqueOrigin(got, "https://example.com")
	got = appendUniqueOrigin(got, "https://example.com")
	if len(got) != 1 || got[0] != "https://example.com" {
		t.Fatalf("valid origin or de-duplication failed: %#v", got)
	}
	got = appendUniqueOrigin(got, "HTTPS://EXAMPLE.COM:443/")
	if len(got) != 1 {
		t.Fatalf("equivalent origin was not normalized: %#v", got)
	}
}

func TestCORSMiddlewareLearnsDOMAINOrigins(t *testing.T) {
	t.Setenv("DOMAIN", "one.example.com, two.example.com")
	s := &Server{appConfig: &config.AppConfig{Server: config.ServerConfig{CORS: config.CORSConfig{
		AllowedOrigins:   []string{},
		AllowCredentials: true,
		MaxAge:           86400,
	}}}}
	req := httptest.NewRequest(http.MethodGet, "/api/v1/search", nil)
	req.Header.Set("Origin", "https://two.example.com")
	res := httptest.NewRecorder()
	s.corsMiddleware(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	})).ServeHTTP(res, req)
	if got := res.Header().Get("Access-Control-Allow-Origin"); got != "https://two.example.com" {
		t.Fatalf("learned DOMAIN origin = %q, want https://two.example.com", got)
	}
	if got := res.Header().Get("Access-Control-Allow-Credentials"); got != "true" {
		t.Fatalf("learned-origin credentials = %q, want true", got)
	}
}

func TestCORSMiddlewareRestrictsTorOrigins(t *testing.T) {
	t.Setenv("DOMAIN", "clearnet.example.com")
	s := &Server{appConfig: &config.AppConfig{Server: config.ServerConfig{
		CORS: config.CORSConfig{AllowedOrigins: []string{"https://clearnet.example.com"}, AllowCredentials: true, MaxAge: 86400},
		Tor:  config.TorConfig{OnionAddress: "abc123.onion"},
	}}}
	req := httptest.NewRequest(http.MethodGet, "http://abc123.onion/api/v1/search", nil)
	req.Header.Set("Origin", "http://abc123.onion")
	res := httptest.NewRecorder()
	s.corsMiddleware(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	})).ServeHTTP(res, req)
	if got := res.Header().Get("Access-Control-Allow-Origin"); got != "http://abc123.onion" {
		t.Fatalf("Tor Allow-Origin = %q, want onion origin only", got)
	}
	if got := res.Header().Get("Access-Control-Allow-Origin"); strings.Contains(got, "clearnet.example.com") {
		t.Fatalf("Tor response exposed clearnet origin %q", got)
	}
}
