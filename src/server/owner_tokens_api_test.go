// SPDX-License-Identifier: MIT
package server

import (
	"context"
	"database/sql"
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
	secretToken := "own_0123456789abcdefghijkmnpqrstuv"
	s.redirectOwnerTokenReveal(response, request, secretToken, "", true, "")
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

func TestOperatorTokenRegenerationRotatesCSRFSecret(t *testing.T) {
	schema, err := database.NewSchemaManager(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = schema.Close() }()
	if err := schema.EnsureSchema(); err != nil {
		t.Fatal(err)
	}
	manager := secret.NewManager(schema.GetAppDatabase())
	if err := manager.EnsureSecrets(context.Background()); err != nil {
		t.Fatal(err)
	}
	previous, err := manager.Get(context.Background(), secret.CSRFTokenSecret)
	if err != nil {
		t.Fatal(err)
	}
	s := &Server{secretsMgr: manager}
	if err := s.RotateCSRFSecretAfterOperatorTokenChange(context.Background()); err != nil {
		t.Fatal(err)
	}
	valid, err := manager.ValidateWithPrevious(context.Background(), secret.CSRFTokenSecret, previous)
	if err != nil {
		t.Fatal(err)
	}
	if valid {
		t.Fatal("operator token regeneration left the previous CSRF secret valid")
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
	manager, err := apitoken.NewManager(schema.GetAppDatabase(), strings.Repeat("ef", 32))
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

	create := httptest.NewRequest(http.MethodPost, "/api/v1/server/preferences/tokens", strings.NewReader(`{"name":"automation","scope":"read","expires":"30d"}`))
	create.Header.Set("Authorization", "Bearer "+issued.Token)
	create.Header.Set("Content-Type", "application/json")
	created := httptest.NewRecorder()
	s.ownerTokensAPICreate(created, create)
	if created.Code != http.StatusCreated {
		t.Fatalf("linked-token creation status = %d: %s", created.Code, created.Body.String())
	}
	var result struct {
		Data struct {
			LinkedToken string `json:"linked_token"`
			Scope       string `json:"scope"`
			Masked      string `json:"token_masked"`
		} `json:"data"`
	}
	if err := json.Unmarshal(created.Body.Bytes(), &result); err != nil {
		t.Fatal(err)
	}
	if result.Data.LinkedToken == "" || result.Data.Scope != "read" || !strings.Contains(result.Data.Masked, "...") {
		t.Fatal("linked-token creation did not return the issued token")
	}
	linkedRequest := httptest.NewRequest(http.MethodGet, "/api/v1/server/preferences/tokens", nil)
	linkedRequest.Header.Set("Authorization", "Bearer "+result.Data.LinkedToken)
	linkedDenied := httptest.NewRecorder()
	s.ownerTokensAPIList(linkedDenied, linkedRequest)
	if linkedDenied.Code != http.StatusForbidden || linkedDenied.Header().Get("WWW-Authenticate") != `Bearer error="insufficient_scope"` {
		t.Fatalf("linked-token management status = %d, want forbidden", linkedDenied.Code)
	}
	primaryList := httptest.NewRequest(http.MethodGet, "/api/v1/server/preferences/tokens", nil)
	primaryList.Header.Set("Authorization", "Bearer "+issued.Token)
	listed := httptest.NewRecorder()
	s.ownerTokensAPIList(listed, primaryList)
	if listed.Code != http.StatusOK || strings.Contains(listed.Body.String(), result.Data.LinkedToken) {
		t.Fatalf("primary token list = %d %q", listed.Code, listed.Body.String())
	}
}

func TestOwnerTokensAPIIssuePrimaryAndRejectsExistingCredentials(t *testing.T) {
	schema, err := database.NewSchemaManager(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = schema.Close() }()
	if err := schema.EnsureSchema(); err != nil {
		t.Fatal(err)
	}
	manager, err := apitoken.NewManager(schema.GetAppDatabase(), strings.Repeat("cd", 32))
	if err != nil {
		t.Fatal(err)
	}
	operatorToken := "adm_" + strings.Repeat("a", 32)
	s := &Server{appConfig: config.DefaultAppConfig(), tokenManager: manager}
	s.appConfig.Server.Token = operatorToken
	s.RefreshOperatorTokenHash()
	s.tokenActionLimiter = ratelimit.NewRateLimiter(true, 10, 900)
	defer s.tokenActionLimiter.Stop()

	request := httptest.NewRequest(http.MethodPost, "/api/v1/server/preferences/tokens/primary", nil)
	response := httptest.NewRecorder()
	s.ownerTokensAPIIssuePrimary(response, request)
	if response.Code != http.StatusCreated {
		t.Fatalf("primary issue status = %d: %s", response.Code, response.Body.String())
	}
	var result struct {
		Data struct {
			OwnerToken string `json:"owner_token"`
		} `json:"data"`
	}
	if err := json.Unmarshal(response.Body.Bytes(), &result); err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(result.Data.OwnerToken, "own_") {
		t.Fatalf("issued owner token has invalid format: %q", result.Data.OwnerToken)
	}

	for name, token := range map[string]string{"owner": result.Data.OwnerToken, "operator": operatorToken} {
		request := httptest.NewRequest(http.MethodPost, "/api/v1/server/preferences/tokens/primary", nil)
		request.Header.Set("Authorization", "Bearer "+token)
		response := httptest.NewRecorder()
		s.ownerTokensAPIIssuePrimary(response, request)
		if response.Code != http.StatusConflict || strings.Contains(response.Body.String(), result.Data.OwnerToken) {
			t.Errorf("%s credential issue response = %d %q, want conflict without token disclosure", name, response.Code, response.Body.String())
		}
	}
}

func TestOwnerTokenWebIssuePrimarySetsCookieAndRejectsExistingIdentity(t *testing.T) {
	schema, err := database.NewSchemaManager(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = schema.Close() }()
	if err := schema.EnsureSchema(); err != nil {
		t.Fatal(err)
	}
	manager, err := apitoken.NewManager(schema.GetAppDatabase(), strings.Repeat("de", 32))
	if err != nil {
		t.Fatal(err)
	}
	s := &Server{appConfig: config.DefaultAppConfig(), tokenManager: manager}
	s.tokenActionLimiter = ratelimit.NewRateLimiter(true, 10, 900)
	defer s.tokenActionLimiter.Stop()

	request := httptest.NewRequest(http.MethodPost, "/server/preferences/token/create", nil)
	response := httptest.NewRecorder()
	s.ownerTokenWebIssuePrimary(response, request)
	if response.Code != http.StatusSeeOther {
		t.Fatalf("web primary issue status = %d: %s", response.Code, response.Body.String())
	}
	var ownerCookie *http.Cookie
	for _, cookie := range response.Result().Cookies() {
		if cookie.Name == ownerTokenCookieName {
			ownerCookie = cookie
		}
	}
	if ownerCookie == nil || ownerCookie.Value == "" || !ownerCookie.HttpOnly || !ownerCookie.Secure || ownerCookie.SameSite != http.SameSiteStrictMode {
		t.Fatalf("web issue did not set the required owner cookie: %#v", ownerCookie)
	}
	if _, err := manager.ValidateOnly(context.Background(), ownerCookie.Value); err != nil {
		t.Fatalf("issued web token is invalid: %v", err)
	}

	request = httptest.NewRequest(http.MethodPost, "/server/preferences/token/create", nil)
	request.AddCookie(ownerCookie)
	response = httptest.NewRecorder()
	s.ownerTokenWebIssuePrimary(response, request)
	if response.Code != http.StatusConflict {
		t.Fatalf("web issue with existing identity status = %d, want %d", response.Code, http.StatusConflict)
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
	manager, err := apitoken.NewManager(schema.GetAppDatabase(), strings.Repeat("ab", 32))
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

func TestOperatorRouteDistinguishesValidOwnerToken(t *testing.T) {
	schema, err := database.NewSchemaManager(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = schema.Close() }()
	if err := schema.EnsureSchema(); err != nil {
		t.Fatal(err)
	}
	manager, err := apitoken.NewManager(schema.GetAppDatabase(), strings.Repeat("12", 32))
	if err != nil {
		t.Fatal(err)
	}
	issued, err := manager.IssuePrimary(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	s := &Server{appConfig: config.DefaultAppConfig(), tokenManager: manager}
	s.appConfig.Server.Token = "operator-test-token"
	s.RefreshOperatorTokenHash()
	called := false
	route := s.requireOperatorToken(http.HandlerFunc(func(http.ResponseWriter, *http.Request) { called = true }))
	request := httptest.NewRequest(http.MethodGet, "/api/v1/status", nil)
	request.Header.Set("Authorization", "Bearer "+issued.Token)
	response := httptest.NewRecorder()
	route.ServeHTTP(response, request)
	if response.Code != http.StatusForbidden || response.Header().Get("WWW-Authenticate") != `Bearer error="insufficient_scope"` || called {
		t.Fatalf("valid owner token response = %d, challenge %q, route called %t", response.Code, response.Header().Get("WWW-Authenticate"), called)
	}
}

func TestOwnerTokensAPIRevokePrimaryRequiresConfirmationAndDeletesOwner(t *testing.T) {
	schema, err := database.NewSchemaManager(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = schema.Close() }()
	if err := schema.EnsureSchema(); err != nil {
		t.Fatal(err)
	}
	manager, err := apitoken.NewManager(schema.GetAppDatabase(), strings.Repeat("ac", 32))
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	issued, err := manager.IssuePrimary(ctx)
	if err != nil {
		t.Fatal(err)
	}
	primary, err := manager.Authenticate(ctx, issued.Token)
	if err != nil {
		t.Fatal(err)
	}
	linked, err := manager.CreateLinked(ctx, primary, "automation", nil, 25)
	if err != nil {
		t.Fatal(err)
	}
	if err := schema.GetAppDatabase().Write(ctx, func(tx *sql.Tx) error {
		_, err := tx.ExecContext(ctx, `INSERT INTO favorites (owner_id, visitor_id, url, title) VALUES (?, ?, ?, ?)`, issued.OwnerID, "owner-revoke-test", "https://example.test/video", "saved")
		return err
	}); err != nil {
		t.Fatal(err)
	}
	s := &Server{appConfig: config.DefaultAppConfig(), tokenManager: manager}
	s.tokenActionLimiter = ratelimit.NewRateLimiter(true, 10, 900)
	defer s.tokenActionLimiter.Stop()

	request := httptest.NewRequest(http.MethodDelete, "/api/v1/server/preferences/tokens/primary", strings.NewReader(`{"confirm":"wrong"}`))
	request.Header.Set("Authorization", "Bearer "+issued.Token)
	response := httptest.NewRecorder()
	s.ownerTokensAPIRevokePrimary(response, request)
	if response.Code != http.StatusBadRequest {
		t.Fatalf("wrong confirmation status = %d, want 400: %s", response.Code, response.Body.String())
	}
	if _, err := manager.Authenticate(ctx, issued.Token); err != nil {
		t.Fatalf("wrong confirmation deleted primary token: %v", err)
	}

	linkedRequest := httptest.NewRequest(http.MethodDelete, "/api/v1/server/preferences/tokens/primary", strings.NewReader(`{"confirm":"`+issued.Token[len(issued.Token)-4:]+`"}`))
	linkedRequest.Header.Set("Authorization", "Bearer "+linked.Token)
	linkedResponse := httptest.NewRecorder()
	s.ownerTokensAPIRevokePrimary(linkedResponse, linkedRequest)
	if linkedResponse.Code != http.StatusForbidden {
		t.Fatalf("linked-token primary revocation status = %d, want 403: %s", linkedResponse.Code, linkedResponse.Body.String())
	}

	request = httptest.NewRequest(http.MethodDelete, "/api/v1/server/preferences/tokens/primary", strings.NewReader(`{"confirm":"`+issued.Token[len(issued.Token)-4:]+`"}`))
	request.Header.Set("Authorization", "Bearer "+issued.Token)
	response = httptest.NewRecorder()
	s.ownerTokensAPIRevokePrimary(response, request)
	if response.Code != http.StatusOK {
		t.Fatalf("primary revocation status = %d, want 200: %s", response.Code, response.Body.String())
	}
	var result struct {
		OK   bool `json:"ok"`
		Data struct {
			Tokens    int64 `json:"tokens"`
			Resources int64 `json:"resources"`
		} `json:"data"`
	}
	if err := json.Unmarshal(response.Body.Bytes(), &result); err != nil {
		t.Fatal(err)
	}
	if !result.OK || result.Data.Tokens != 2 || result.Data.Resources != 1 {
		t.Fatalf("primary revocation response = %+v, want two tokens and one resource", result)
	}
	if _, err := manager.Authenticate(ctx, issued.Token); err != apitoken.ErrInvalidToken {
		t.Fatalf("deleted primary authentication error = %v, want invalid token", err)
	}
}

func TestOwnerTokensAPIRevokePrimaryConsumesStrictLimitOnEveryAttempt(t *testing.T) {
	s := &Server{appConfig: config.DefaultAppConfig()}
	s.tokenActionLimiter = ratelimit.NewRateLimiter(true, 1, 900)
	defer s.tokenActionLimiter.Stop()
	for index, want := range []int{http.StatusUnauthorized, http.StatusTooManyRequests} {
		request := httptest.NewRequest(http.MethodDelete, "/api/v1/server/preferences/tokens/primary", strings.NewReader(`{}`))
		response := httptest.NewRecorder()
		s.ownerTokenPrincipal(response, request)
		if response.Code != want {
			t.Fatalf("revoke attempt %d status = %d, want %d", index+1, response.Code, want)
		}
	}
}

func TestOwnerTokenWebRevokePrimaryConfirmsClearsCredentialsAndRedirects(t *testing.T) {
	schema, err := database.NewSchemaManager(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = schema.Close() }()
	if err := schema.EnsureSchema(); err != nil {
		t.Fatal(err)
	}
	manager, err := apitoken.NewManager(schema.GetAppDatabase(), strings.Repeat("ac", 32))
	if err != nil {
		t.Fatal(err)
	}
	issued, err := manager.IssuePrimary(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	s := &Server{appConfig: config.DefaultAppConfig(), tokenManager: manager}
	s.tokenActionLimiter = ratelimit.NewRateLimiter(true, 10, 900)
	defer s.tokenActionLimiter.Stop()

	request := httptest.NewRequest(http.MethodPost, "/server/preferences/token/revoke", strings.NewReader("confirm=wrong"))
	request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	request.AddCookie(&http.Cookie{Name: ownerTokenCookieName, Value: issued.Token})
	response := httptest.NewRecorder()
	s.ownerTokenWebRevokePrimary(response, request)
	if response.Code != http.StatusBadRequest {
		t.Fatalf("wrong web confirmation status = %d, want 400", response.Code)
	}
	if _, err := manager.Authenticate(context.Background(), issued.Token); err != nil {
		t.Fatalf("wrong web confirmation deleted primary token: %v", err)
	}

	request = httptest.NewRequest(http.MethodPost, "/server/preferences/token/revoke", strings.NewReader("confirm="+issued.Token[len(issued.Token)-4:]))
	request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	request.AddCookie(&http.Cookie{Name: ownerTokenCookieName, Value: issued.Token})
	response = httptest.NewRecorder()
	s.ownerTokenWebRevokePrimary(response, request)
	location, err := url.Parse(response.Header().Get("Location"))
	if err != nil || response.Code != http.StatusSeeOther || location.Path != "/server/preferences/tokens" {
		t.Fatalf("web revocation redirect = %d %q, want 303 to token page", response.Code, response.Header().Get("Location"))
	}
	if response.Header().Get("Clear-Site-Data") != `"storage"` {
		t.Fatalf("web revocation Clear-Site-Data = %q, want storage only", response.Header().Get("Clear-Site-Data"))
	}
	var cleared *http.Cookie
	for _, cookie := range response.Result().Cookies() {
		if cookie.Name == ownerTokenCookieName {
			cleared = cookie
			break
		}
	}
	if cleared == nil || cleared.Value != "" || cleared.MaxAge != 0 || !cleared.HttpOnly || !cleared.Secure || cleared.SameSite != http.SameSiteStrictMode {
		t.Fatalf("web revocation did not clear the primary cookie with matching attributes: %#v", cleared)
	}
	if _, err := manager.Authenticate(context.Background(), issued.Token); err != apitoken.ErrInvalidToken {
		t.Fatalf("deleted primary authentication error = %v, want invalid token", err)
	}
}
