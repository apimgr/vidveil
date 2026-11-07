// SPDX-License-Identifier: MIT
package server

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/apimgr/vidveil/src/config"
	"github.com/apimgr/vidveil/src/server/handler"
	"github.com/apimgr/vidveil/src/server/service/apitoken"
	"github.com/apimgr/vidveil/src/server/service/database"
	"github.com/apimgr/vidveil/src/server/service/ratelimit"
	"github.com/apimgr/vidveil/src/server/service/secret"
)

func TestOwnerTokenExpiryFormAcceptsChosenDate(t *testing.T) {
	form := url.Values{"expires_preset": {"custom"}, "expires_date": {time.Now().UTC().Add(48 * time.Hour).Format("2006-01-02")}}
	request := httptest.NewRequest(http.MethodPost, "/server/preferences/tokens", strings.NewReader(form.Encode()))
	request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	if err := request.ParseForm(); err != nil {
		t.Fatal(err)
	}
	expires, err := ownerTokenExpiryFromForm(request)
	if err != nil {
		t.Fatalf("chosen date rejected: %v", err)
	}
	if expires == nil || *expires <= time.Now().Unix() {
		t.Fatalf("chosen date expiry = %v, want future timestamp", expires)
	}
}

func TestOwnerTokenExpiryPresetIgnoresStaleCustomDate(t *testing.T) {
	form := url.Values{"expires_preset": {"30d"}, "expires_date": {time.Now().UTC().Add(90 * 24 * time.Hour).Format("2006-01-02")}}
	request := httptest.NewRequest(http.MethodPost, "/server/preferences/tokens", strings.NewReader(form.Encode()))
	request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	if err := request.ParseForm(); err != nil {
		t.Fatal(err)
	}
	expires, err := ownerTokenExpiryFromForm(request)
	if err != nil {
		t.Fatalf("preset expiry rejected: %v", err)
	}
	if expires == nil || time.Until(time.Unix(*expires, 0)) < 29*24*time.Hour || time.Until(time.Unix(*expires, 0)) > 31*24*time.Hour {
		t.Fatalf("preset expiry = %v, want about 30 days", expires)
	}
}

func TestOwnerTokenRevealRedirectsAndIsConsumedOnce(t *testing.T) {
	request := httptest.NewRequest(http.MethodPost, "/server/preferences/token/reveal?lang=es", nil)
	response := httptest.NewRecorder()
	s := &Server{}
	secretToken := "tok_0123456789abcdefghijkmnpqrstuv"
	s.redirectOwnerTokenReveal(response, request, secretToken, 0, true, "")
	if response.Code != http.StatusSeeOther || !strings.Contains(response.Header().Get("Location"), "/server/preferences/tokens#primary") {
		t.Fatalf("reveal redirect = %d %q", response.Code, response.Header().Get("Location"))
	}
	cookies := response.Result().Cookies()
	if len(cookies) != 1 || cookies[0].Name != ownerTokenRevealCookie {
		t.Fatalf("reveal cookie = %#v", cookies)
	}
	request = httptest.NewRequest(http.MethodGet, "/server/preferences/tokens", nil)
	request.AddCookie(cookies[0])
	var got handler.OwnerTokenReveal
	called := 0
	next := http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
		called++
		got, _ = handler.OwnerTokenRevealFromContext(r.Context())
	})
	s.ownerTokenRevealMiddleware(next).ServeHTTP(httptest.NewRecorder(), request)
	if got.Token != secretToken || !got.Primary || called != 1 {
		t.Fatalf("reveal flash = %#v, called %d", got, called)
	}
	got = handler.OwnerTokenReveal{}
	request = httptest.NewRequest(http.MethodGet, "/server/preferences/tokens", nil)
	request.AddCookie(cookies[0])
	s.ownerTokenRevealMiddleware(next).ServeHTTP(httptest.NewRecorder(), request)
	if got.Token != "" || called != 2 {
		t.Fatal("one-time reveal was not consumed")
	}
}

func TestAPITokenRegenerationRotatesCSRFSecret(t *testing.T) {
	schema, err := database.NewSchemaManager(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = schema.Close() }()
	if err := schema.EnsureSchema(); err != nil {
		t.Fatal(err)
	}
	manager := secret.NewManager(schema.GetDB())
	if err := manager.EnsureSecrets(context.Background()); err != nil {
		t.Fatal(err)
	}
	previous, err := manager.Get(context.Background(), secret.CSRFTokenSecret)
	if err != nil {
		t.Fatal(err)
	}
	s := &Server{secretsMgr: manager}
	s.rotateCSRFSecretAfterAPITokenRegeneration(httptest.NewRequest(http.MethodPost, "/", nil))
	valid, err := manager.ValidateWithPrevious(context.Background(), secret.CSRFTokenSecret, previous)
	if err != nil {
		t.Fatal(err)
	}
	if valid {
		t.Fatal("API token regeneration left the previous CSRF secret valid")
	}
}

func TestOwnerTokensAPIRequiresPrimaryBearerAndMasksList(t *testing.T) {
	schema, err := database.NewSchemaManager(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = schema.Close() }()
	if err := schema.EnsureSchema(); err != nil {
		t.Fatal(err)
	}
	manager, err := apitoken.NewManager(schema.GetDB(), strings.Repeat("ef", 32))
	if err != nil {
		t.Fatal(err)
	}
	issued, err := manager.IssuePrimary(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	s := &Server{appConfig: config.DefaultAppConfig(), tokenManager: manager}
	s.tokenActionLimiter = ratelimit.NewRateLimiter(true, 1, 900)
	defer s.tokenActionLimiter.Stop()
	for attempt, want := range []int{http.StatusUnauthorized, http.StatusTooManyRequests} {
		request := httptest.NewRequest(http.MethodPost, "/api/v1/server/preferences/tokens/primary/rotate", nil)
		response := httptest.NewRecorder()
		s.ownerTokenPrincipal(response, request)
		if response.Code != want {
			t.Fatalf("sensitive token attempt %d status = %d, want %d", attempt+1, response.Code, want)
		}
		if attempt == 0 && response.Header().Get("WWW-Authenticate") != `Bearer realm="vidveil"` {
			t.Fatalf("missing token challenge = %q", response.Header().Get("WWW-Authenticate"))
		}
	}

	withoutBearer := httptest.NewRequest(http.MethodGet, "/api/v1/server/preferences/tokens", nil)
	withoutBearer.AddCookie(&http.Cookie{Name: "vidveil_owner_token_example", Value: issued.Token})
	denied := httptest.NewRecorder()
	s.ownerTokensAPIList(denied, withoutBearer)
	if denied.Code != http.StatusUnauthorized || denied.Header().Get("WWW-Authenticate") != `Bearer realm="vidveil"` || strings.Contains(denied.Body.String(), issued.Token) {
		t.Fatalf("cookie-only API response = %d %q", denied.Code, denied.Body.String())
	}
	invalidRequest := httptest.NewRequest(http.MethodGet, "/api/v1/server/preferences/tokens", nil)
	invalidRequest.Header.Set("Authorization", "Bearer invalid-token")
	invalidResponse := httptest.NewRecorder()
	s.ownerTokensAPIList(invalidResponse, invalidRequest)
	if invalidResponse.Code != http.StatusUnauthorized || invalidResponse.Header().Get("WWW-Authenticate") != `Bearer realm="vidveil", error="invalid_token"` {
		t.Fatalf("invalid token response = %d %q", invalidResponse.Code, invalidResponse.Header().Get("WWW-Authenticate"))
	}

	request := httptest.NewRequest(http.MethodGet, "/api/v1/server/preferences/tokens", nil)
	request.Header.Set("Authorization", "Bearer "+issued.Token)
	response := httptest.NewRecorder()
	s.ownerTokensAPIList(response, request)
	if response.Code != http.StatusOK {
		t.Fatalf("primary token list status = %d: %s", response.Code, response.Body.String())
	}
	if strings.Contains(response.Body.String(), issued.Token) {
		t.Fatal("token list leaked the raw primary token")
	}

	create := httptest.NewRequest(http.MethodPost, "/api/v1/server/preferences/tokens", strings.NewReader(`{"name":"automation","expires":"30d"}`))
	create.Header.Set("Authorization", "Bearer "+issued.Token)
	create.Header.Set("Content-Type", "application/json")
	created := httptest.NewRecorder()
	s.ownerTokensAPICreate(created, create)
	if created.Code != http.StatusOK {
		t.Fatalf("linked-token creation status = %d: %s", created.Code, created.Body.String())
	}
	var result struct {
		Data struct {
			Token string `json:"token"`
		} `json:"data"`
	}
	if err := json.Unmarshal(created.Body.Bytes(), &result); err != nil {
		t.Fatal(err)
	}
	if result.Data.Token == "" {
		t.Fatal("linked-token creation did not return the issued token")
	}
	linkedRequest := httptest.NewRequest(http.MethodGet, "/api/v1/server/preferences/tokens", nil)
	linkedRequest.Header.Set("Authorization", "Bearer "+result.Data.Token)
	linkedDenied := httptest.NewRecorder()
	s.ownerTokensAPIList(linkedDenied, linkedRequest)
	if linkedDenied.Code != http.StatusForbidden || linkedDenied.Header().Get("WWW-Authenticate") != `Bearer error="insufficient_scope"` {
		t.Fatalf("linked-token management status = %d, want forbidden", linkedDenied.Code)
	}
	primaryList := httptest.NewRequest(http.MethodGet, "/api/v1/server/preferences/tokens", nil)
	primaryList.Header.Set("Authorization", "Bearer "+issued.Token)
	listed := httptest.NewRecorder()
	s.ownerTokensAPIList(listed, primaryList)
	if listed.Code != http.StatusOK || strings.Contains(listed.Body.String(), result.Data.Token) {
		t.Fatalf("primary token list = %d %q", listed.Code, listed.Body.String())
	}
}

func TestOwnerTokenRestoreSetsStrictPrimaryCookie(t *testing.T) {
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
	s := &Server{appConfig: config.DefaultAppConfig(), tokenManager: manager}
	request := httptest.NewRequest(http.MethodPost, "/server/preferences/token/restore", strings.NewReader("token="+issued.Token))
	request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	response := httptest.NewRecorder()
	s.ownerTokenRestore(response, request)
	if response.Code != http.StatusSeeOther {
		t.Fatalf("restore status = %d: %s", response.Code, response.Body.String())
	}
	cookies := response.Result().Cookies()
	if len(cookies) != 1 {
		t.Fatalf("restore cookies = %d, want one", len(cookies))
	}
	cookie := cookies[0]
	if cookie.Name != ownerTokenCookieName || cookie.Value != issued.Token || !cookie.HttpOnly || !cookie.Secure || cookie.SameSite != http.SameSiteStrictMode || cookie.Path != "/" {
		t.Fatalf("restored cookie does not match required owner-cookie attributes: %#v", cookie)
	}
}
