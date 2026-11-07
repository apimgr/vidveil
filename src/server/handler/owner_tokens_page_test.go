// SPDX-License-Identifier: MIT
package handler

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/apimgr/vidveil/src/server/service/apitoken"
	"github.com/apimgr/vidveil/src/server/service/database"
)

func TestOwnerTokensPageOnlyOffersRestoreWithoutPrimaryCookie(t *testing.T) {
	setRealTemplatesFS(t)
	h := newRenderTestHandler()
	for _, userAgent := range []string{"Mozilla/5.0", "Lynx/2.9"} {
		request := httptest.NewRequest(http.MethodGet, "/server/preferences/tokens", nil)
		request.Header.Set("Accept", "text/html")
		request.Header.Set("User-Agent", userAgent)
		response := httptest.NewRecorder()
		h.OwnerTokensPage(response, request)
		body := response.Body.String()
		if response.Code != http.StatusOK || response.Header().Get("Cache-Control") != "no-store" {
			t.Fatalf("OwnerTokensPage(%q) response = %d, cache-control %q", userAgent, response.Code, response.Header().Get("Cache-Control"))
		}
		if !strings.Contains(body, `action="/server/preferences/token/restore"`) || strings.Contains(body, `action="/server/preferences/token/reveal"`) || strings.Contains(body, "Linked tokens") {
			t.Errorf("OwnerTokensPage(%q) exposed token management without a primary cookie: %s", userAgent, body)
		}
	}
}

func TestOwnerTokensPageMasksPrimaryAndRendersManagementForms(t *testing.T) {
	setRealTemplatesFS(t)
	schema, err := database.NewSchemaManager(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = schema.Close() }()
	if err := schema.EnsureSchema(); err != nil {
		t.Fatal(err)
	}
	manager, err := apitoken.NewManager(schema.GetDB(), strings.Repeat("ab", 32))
	if err != nil {
		t.Fatal(err)
	}
	issued, err := manager.IssuePrimary(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	linked, err := manager.CreateLinked(context.Background(), mustPrimaryPrincipal(t, manager, issued.Token), "automation", nil, 25)
	if err != nil {
		t.Fatal(err)
	}
	h := newRenderTestHandler()
	h.tokenManager = manager
	for _, userAgent := range []string{"Mozilla/5.0", "Lynx/2.9"} {
		request := httptest.NewRequest(http.MethodGet, "/server/preferences/tokens", nil)
		request.Header.Set("Accept", "text/html")
		request.Header.Set("User-Agent", userAgent)
		request.AddCookie(&http.Cookie{Name: apitoken.OwnerCookieName, Value: issued.Token})
		response := httptest.NewRecorder()
		h.OwnerTokensPage(response, request)
		body := response.Body.String()
		for _, marker := range []string{`tok_••••••••••••••••••••••••` + issued.Token[len(issued.Token)-4:], `action="/server/preferences/token/reveal"`, `action="/server/preferences/token/rotate"`, `action="/server/preferences/tokens"`, `automation`, linked.Token[len(linked.Token)-4:], `name="csrf_token"`, `class="copy-value"`, `This browser`, `Forget saved token`, `Expires on`} {
			if !strings.Contains(body, marker) {
				t.Errorf("OwnerTokensPage(%q) omitted %q: %s", userAgent, marker, body)
			}
		}
		if strings.Contains(body, issued.Token) || strings.Contains(body, linked.Token) {
			t.Errorf("OwnerTokensPage(%q) leaked a raw token", userAgent)
		}
	}
}

func mustPrimaryPrincipal(t *testing.T, manager *apitoken.Manager, token string) apitoken.Principal {
	t.Helper()
	principal, err := manager.Authenticate(context.Background(), token)
	if err != nil {
		t.Fatal(err)
	}
	return principal
}
