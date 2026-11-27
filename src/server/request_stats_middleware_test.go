// SPDX-License-Identifier: MIT
package server

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/apimgr/vidveil/src/config"
	"github.com/apimgr/vidveil/src/server/handler"
)

func TestRequestStatsMiddlewareCountsRequestsAndExcludesHealthMetrics(t *testing.T) {
	metrics := handler.NewMetrics(config.DefaultAppConfig(), nil)
	next := requestStatsMiddleware(metrics)(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}))
	for _, path := range []string{"/search", "/server/healthz", "/api/v1/server/healthz", "/server/metrics"} {
		recorder := httptest.NewRecorder()
		next.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, path, nil))
		if recorder.Code != http.StatusNoContent {
			t.Fatalf("%s status = %d, want %d", path, recorder.Code, http.StatusNoContent)
		}
	}
	if got := metrics.GetRequestsTotal(); got != 1 {
		t.Errorf("request total = %d, want 1", got)
	}
	if got := metrics.GetRequests24h(); got != 1 {
		t.Errorf("request 24h = %d, want 1", got)
	}
	if got := metrics.GetActiveConnections(); got != 0 {
		t.Errorf("active connections after requests = %d, want 0", got)
	}
}
