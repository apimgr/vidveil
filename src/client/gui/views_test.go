// SPDX-License-Identifier: MIT
// AI.md PART 32: GUI command-to-screen coverage.
//go:build gui && !freebsd && !netbsd && !openbsd

package gui

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"reflect"
	"testing"
)

func TestGUIHasOneScreenForEveryServerFacingCLICommandInHelpOrder(t *testing.T) {
	want := []string{"search", "engines", "bangs", "probe", "health", "favorites", "login", "update", "setup"}
	if !reflect.DeepEqual(guiCommandScreens, want) {
		t.Fatalf("GUI command screens = %v, want CLI help order %v", guiCommandScreens, want)
	}
}

func TestGUIUsesConfiguredLocaleForLabels(t *testing.T) {
	session := &guiSession{cfg: &Config{Language: "es"}}
	if got := session.translate("screen.search"); got != "Buscar" {
		t.Fatalf("Spanish search label = %q, want Buscar", got)
	}
}

func TestGUISetupChecksServerAndOnlyChecksConfiguredToken(t *testing.T) {
	tokenChecks := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/v1/server/healthz":
			w.WriteHeader(http.StatusOK)
		case "/api/v1/version":
			_, _ = fmt.Fprint(w, `{"ok":true,"version":"1.2.3"}`)
		case "/api/v1/server/auth/check":
			tokenChecks++
			if r.Header.Get("Authorization") != "Bearer valid-token" {
				http.Error(w, "unauthorized", http.StatusUnauthorized)
				return
			}
			_, _ = fmt.Fprint(w, `{"ok":true,"data":{"kind":"server"}}`)
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	session := &guiSession{cfg: &Config{Language: "en", APIVersion: "v1"}}
	_, version, err := session.validateServerConnection(server.URL, "")
	if err != nil || version == nil || version.Version != "1.2.3" {
		t.Fatalf("connection without token: version=%#v err=%v", version, err)
	}
	if tokenChecks != 0 {
		t.Fatalf("auth check count without token = %d, want 0", tokenChecks)
	}
	if _, _, err := session.validateServerConnection(server.URL, "valid-token"); err != nil {
		t.Fatalf("connection with valid token: %v", err)
	}
	if tokenChecks != 1 {
		t.Fatalf("auth check count with token = %d, want 1", tokenChecks)
	}
}
