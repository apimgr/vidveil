// SPDX-License-Identifier: MIT
package server

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestRestoreAPIRouteRequiresOperatorBearerToken(t *testing.T) {
	s := newTestServer(t)
	s.appConfig.Server.Token = "restore-route-token"
	s.RefreshOperatorTokenHash()
	endpoint := s.appConfig.APIBasePath() + "/server/restore"

	unauthorized := httptest.NewRequest(http.MethodPost, endpoint, nil)
	unauthorized.Header.Set("Authorization", "Bearer wrong-token")
	unauthorizedRec := httptest.NewRecorder()
	s.router.ServeHTTP(unauthorizedRec, unauthorized)
	if unauthorizedRec.Code != http.StatusUnauthorized {
		t.Fatalf("wrong token status = %d, want 401; body=%s", unauthorizedRec.Code, unauthorizedRec.Body.String())
	}

	authorized := httptest.NewRequest(http.MethodPost, endpoint, nil)
	authorized.Header.Set("Authorization", "Bearer restore-route-token")
	authorizedRec := httptest.NewRecorder()
	s.router.ServeHTTP(authorizedRec, authorized)
	if authorizedRec.Code != http.StatusBadRequest {
		t.Fatalf("valid token with missing upload status = %d, want 400; body=%s", authorizedRec.Code, authorizedRec.Body.String())
	}
}
