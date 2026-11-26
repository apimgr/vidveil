// SPDX-License-Identifier: MIT
package server

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/apimgr/vidveil/src/server/service/maintenance"
)

func TestOperatorStatusRouteRequiresServerTokenAndReturnsDiagnostics(t *testing.T) {
	server := newTestServer(t)
	server.appConfig.Server.Token = "status-route-token"
	server.RefreshOperatorTokenHash()
	server.searchHandler.SetMaintenanceStatus(&maintenance.MaintenanceStatus{
		Reason: "database_connection", Diagnostic: "database probe refused connection",
	})
	endpoint := server.appConfig.APIBasePath() + "/status"

	unauthorizedRequest := httptest.NewRequest(http.MethodGet, endpoint, nil)
	unauthorizedResponse := httptest.NewRecorder()
	server.router.ServeHTTP(unauthorizedResponse, unauthorizedRequest)
	if unauthorizedResponse.Code != http.StatusUnauthorized {
		t.Fatalf("unauthorized status = %d, want 401", unauthorizedResponse.Code)
	}

	authorizedRequest := httptest.NewRequest(http.MethodGet, endpoint, nil)
	authorizedRequest.Header.Set("Authorization", "Bearer status-route-token")
	authorizedRequest.Header.Set("Accept", "application/json")
	authorizedResponse := httptest.NewRecorder()
	server.router.ServeHTTP(authorizedResponse, authorizedRequest)
	if authorizedResponse.Code != http.StatusOK || !strings.Contains(authorizedResponse.Body.String(), "database probe refused connection") || !strings.Contains(authorizedResponse.Body.String(), "fix_instructions") {
		t.Fatalf("authorized status = %d, body=%s", authorizedResponse.Code, authorizedResponse.Body.String())
	}
}
