package handler

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/apimgr/vidveil/src/config"
)

func TestBrandingAssetURLUsesOnlyCachedSameOriginAssets(t *testing.T) {
	h := NewSearchHandler(config.DefaultAppConfig(), nil)
	dataDir := t.TempDir()
	h.SetDataDir(dataDir)

	if got := h.brandingAssetURL("https://assets.example.org/logo.png", "logo-200.png"); got != "" {
		t.Fatalf("uncached configured source URL leaked to the page: %q", got)
	}

	cacheDir := filepath.Join(dataDir, "branding")
	if err := os.MkdirAll(cacheDir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(cacheDir, "logo-200.png"), []byte("cached"), 0644); err != nil {
		t.Fatal(err)
	}
	if got := h.brandingAssetURL("/etc/vidveil/logo.png", "logo-200.png"); got != "/branding/logo-200.png" {
		t.Fatalf("cached local source URL = %q, want same-origin cache path", got)
	}
	if got := h.brandingAssetURL("/etc/vidveil/logo.png", "../../server.yml"); got != "" {
		t.Fatalf("unapproved asset path = %q, want empty", got)
	}
}

func TestFaviconAndAppleTouchIconUseCachedCustomAssets(t *testing.T) {
	cfg := config.DefaultAppConfig()
	cfg.Server.Branding.Favicon = "/etc/vidveil/favicon.png"
	h := NewSearchHandler(cfg, nil)
	dataDir := t.TempDir()
	h.SetDataDir(dataDir)
	cacheDir := filepath.Join(dataDir, "branding")
	if err := os.MkdirAll(cacheDir, 0755); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"favicon-32.png", "favicon-180.png"} {
		if err := os.WriteFile(filepath.Join(cacheDir, name), []byte("cached"), 0644); err != nil {
			t.Fatal(err)
		}
	}

	for _, tc := range []struct {
		name    string
		handler func(http.ResponseWriter, *http.Request)
		want    string
	}{
		{"favicon", h.Favicon, "/branding/favicon-32.png"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			w := httptest.NewRecorder()
			tc.handler(w, httptest.NewRequest("GET", "/", nil))
			if got := w.Header().Get("Location"); got != tc.want {
				t.Fatalf("redirect = %q, want %q", got, tc.want)
			}
		})
	}
	w := httptest.NewRecorder()
	h.AppleTouchIcon(w, httptest.NewRequest("GET", "/apple-touch-icon.png", nil))
	if w.Code != http.StatusOK || w.Body.String() != "cached" {
		t.Fatalf("custom apple-touch icon response = %d %q, want 200 with cached image", w.Code, w.Body.String())
	}
}
