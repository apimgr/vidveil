// SPDX-License-Identifier: MIT
package handler

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/apimgr/vidveil/src/server/service/apitoken"
	"github.com/apimgr/vidveil/src/server/service/database"
)

func TestFavoritesIssueOwnerTokenAndScopeFavorites(t *testing.T) {
	schema, err := database.NewSchemaManager(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = schema.Close() }()
	if err := schema.EnsureSchema(); err != nil {
		t.Fatal(err)
	}
	manager, err := apitoken.NewManager(schema.GetDB(), strings.Repeat("cd", 32))
	if err != nil {
		t.Fatal(err)
	}
	h := newAPITestHandler()
	h.SetHealthDB(schema.GetDB())
	h.SetOwnerTokenManager(manager)

	form := url.Values{"url": {"https://example.test/video"}, "title": {"Saved video"}}
	request := httptest.NewRequest(http.MethodPost, "/favorites", strings.NewReader(form.Encode()))
	request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	response := httptest.NewRecorder()
	h.FavoritesSave(response, request)
	if response.Code != http.StatusOK || !strings.Contains(response.Body.String(), "Save your owner token") {
		t.Fatalf("favorite creation response = %d %q", response.Code, response.Body.String())
	}
	if !strings.Contains(response.Body.String(), `data-owner-token-value`) || !strings.Contains(response.Body.String(), `data-copy-owner-token`) || !strings.Contains(response.Body.String(), `/static/js/app.js`) {
		t.Fatalf("favorite creation page omitted the token copy control or its enhancement: %s", response.Body.String())
	}
	var ownerCookie *http.Cookie
	for _, cookie := range response.Result().Cookies() {
		if cookie.Name == apitoken.OwnerCookieName {
			ownerCookie = cookie
		}
	}
	if ownerCookie == nil || !ownerCookie.HttpOnly || !ownerCookie.Secure || ownerCookie.SameSite != http.SameSiteStrictMode {
		t.Fatalf("favorite creation owner cookie = %#v", ownerCookie)
	}
	if _, err := manager.Authenticate(context.Background(), ownerCookie.Value); err != nil {
		t.Fatalf("issued cookie token is invalid: %v", err)
	}
	preferencesRequest := httptest.NewRequest(http.MethodGet, "/server/preferences", nil)
	preferencesRequest.AddCookie(ownerCookie)
	preferences := h.preferencePageData(httptest.NewRecorder(), preferencesRequest)
	if preferences["OwnerTokenEnabled"] != true {
		t.Fatalf("preferences did not recognize the owner token: %#v", preferences["OwnerTokenEnabled"])
	}
	linkedExpiry := time.Now().Add(30 * 24 * time.Hour).Unix()
	if _, err := manager.CreateLinked(context.Background(), mustAuthenticateOwner(t, manager, ownerCookie.Value), "automation", &linkedExpiry, 25); err != nil {
		t.Fatalf("create linked token for preferences: %v", err)
	}
	setRealTemplatesFS(t)
	preferencesPage := httptest.NewRequest(http.MethodGet, "/server/preferences", nil)
	preferencesPage.Header.Set("Accept", "text/html")
	preferencesPage.AddCookie(ownerCookie)
	preferencesResponse := httptest.NewRecorder()
	h.PreferencesPage(preferencesResponse, preferencesPage)
	preferencesHTML := preferencesResponse.Body.String()
	if preferencesResponse.Code != http.StatusOK {
		t.Fatalf("preferences page status = %d: %s", preferencesResponse.Code, preferencesHTML)
	}
	if !strings.Contains(preferencesHTML, "tok_••••••••••••"+ownerCookie.Value[len(ownerCookie.Value)-4:]) || strings.Contains(preferencesHTML, ownerCookie.Value[:12]) {
		t.Fatalf("preferences page does not mask token to final four characters: %s", preferencesHTML)
	}
	if !strings.Contains(preferencesHTML, "automation") || !strings.Contains(preferencesHTML, "Created") || !strings.Contains(preferencesHTML, "Expires") {
		t.Fatalf("preferences page omitted linked-token metadata: %s", preferencesHTML)
	}

	list := httptest.NewRequest(http.MethodGet, "/favorites", nil)
	list.Header.Set("Accept", "application/json")
	list.AddCookie(ownerCookie)
	listed := httptest.NewRecorder()
	h.FavoritesPage(listed, list)
	if listed.Code != http.StatusOK || !strings.Contains(listed.Body.String(), "Saved video") {
		t.Fatalf("owner favorite list = %d %q", listed.Code, listed.Body.String())
	}
	setRealTemplatesFS(t)
	webList := httptest.NewRequest(http.MethodGet, "/favorites", nil)
	webList.Header.Set("Accept", "text/html")
	webList.AddCookie(ownerCookie)
	webListed := httptest.NewRecorder()
	h.FavoritesPage(webListed, webList)
	if webListed.Code != http.StatusOK || !strings.Contains(webListed.Body.String(), "Saved video") {
		t.Fatalf("server-rendered owner favorite list = %d %q", webListed.Code, webListed.Body.String())
	}

	other := httptest.NewRequest(http.MethodGet, "/favorites", nil)
	other.Header.Set("Accept", "application/json")
	private := httptest.NewRecorder()
	h.FavoritesPage(private, other)
	if private.Code != http.StatusOK || strings.Contains(private.Body.String(), "Saved video") {
		t.Fatalf("unowned favorite list = %d %q", private.Code, private.Body.String())
	}
	operator := httptest.NewRequest(http.MethodGet, "/api/v1/favorites", nil)
	operator.Header.Set("Authorization", "Bearer "+h.appConfig.Server.Token)
	global := httptest.NewRecorder()
	h.FavoritesAPIList(global, operator)
	if global.Code != http.StatusOK || !strings.Contains(global.Body.String(), "Saved video") {
		t.Fatalf("operator favorite list = %d %q", global.Code, global.Body.String())
	}
}

func mustAuthenticateOwner(t *testing.T, manager *apitoken.Manager, token string) apitoken.Principal {
	t.Helper()
	principal, err := manager.Authenticate(context.Background(), token)
	if err != nil {
		t.Fatalf("authenticate owner token: %v", err)
	}
	return principal
}
