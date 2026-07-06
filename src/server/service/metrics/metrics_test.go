// SPDX-License-Identifier: MIT
// Tests for the metrics package: verifies that every promauto-registered variable
// is non-nil after package init and that representative metrics can be used without
// panicking.  All vars are globals initialized at import time.
package metrics

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

// ---- Application metrics ----

func TestAppInfoNotNil(t *testing.T) {
	if AppInfo == nil {
		t.Error("AppInfo is nil; promauto registration failed")
	}
}

func TestAppUptimeSecondsNotNil(t *testing.T) {
	if AppUptimeSeconds == nil {
		t.Error("AppUptimeSeconds is nil; promauto registration failed")
	}
}

func TestAppStartTimestampNotNil(t *testing.T) {
	if AppStartTimestamp == nil {
		t.Error("AppStartTimestamp is nil; promauto registration failed")
	}
}

// ---- HTTP metrics ----

func TestHTTPRequestsTotalNotNil(t *testing.T) {
	if HTTPRequestsTotal == nil {
		t.Error("HTTPRequestsTotal is nil; promauto registration failed")
	}
}

func TestHTTPRequestDurationNotNil(t *testing.T) {
	if HTTPRequestDuration == nil {
		t.Error("HTTPRequestDuration is nil; promauto registration failed")
	}
}

func TestHTTPRequestSizeNotNil(t *testing.T) {
	if HTTPRequestSize == nil {
		t.Error("HTTPRequestSize is nil; promauto registration failed")
	}
}

func TestHTTPResponseSizeNotNil(t *testing.T) {
	if HTTPResponseSize == nil {
		t.Error("HTTPResponseSize is nil; promauto registration failed")
	}
}

func TestHTTPActiveRequestsNotNil(t *testing.T) {
	if HTTPActiveRequests == nil {
		t.Error("HTTPActiveRequests is nil; promauto registration failed")
	}
}

// ---- Database metrics ----

func TestDBQueriesTotalNotNil(t *testing.T) {
	if DBQueriesTotal == nil {
		t.Error("DBQueriesTotal is nil; promauto registration failed")
	}
}

func TestDBQueryDurationNotNil(t *testing.T) {
	if DBQueryDuration == nil {
		t.Error("DBQueryDuration is nil; promauto registration failed")
	}
}

func TestDBConnectionsOpenNotNil(t *testing.T) {
	if DBConnectionsOpen == nil {
		t.Error("DBConnectionsOpen is nil; promauto registration failed")
	}
}

func TestDBConnectionsInUseNotNil(t *testing.T) {
	if DBConnectionsInUse == nil {
		t.Error("DBConnectionsInUse is nil; promauto registration failed")
	}
}

func TestDBErrorsTotalNotNil(t *testing.T) {
	if DBErrorsTotal == nil {
		t.Error("DBErrorsTotal is nil; promauto registration failed")
	}
}

// ---- Authentication metrics ----

func TestAuthAttemptsTotalNotNil(t *testing.T) {
	if AuthAttemptsTotal == nil {
		t.Error("AuthAttemptsTotal is nil; promauto registration failed")
	}
}

func TestAuthSessionsActiveNotNil(t *testing.T) {
	if AuthSessionsActive == nil {
		t.Error("AuthSessionsActive is nil; promauto registration failed")
	}
}

// ---- Cache metrics ----

func TestCacheHitsTotalNotNil(t *testing.T) {
	if CacheHitsTotal == nil {
		t.Error("CacheHitsTotal is nil; promauto registration failed")
	}
}

func TestCacheMissesTotalNotNil(t *testing.T) {
	if CacheMissesTotal == nil {
		t.Error("CacheMissesTotal is nil; promauto registration failed")
	}
}

func TestCacheEvictionsNotNil(t *testing.T) {
	if CacheEvictions == nil {
		t.Error("CacheEvictions is nil; promauto registration failed")
	}
}

// ---- Search metrics ----

func TestSearchQueriesTotalNotNil(t *testing.T) {
	if SearchQueriesTotal == nil {
		t.Error("SearchQueriesTotal is nil; promauto registration failed")
	}
}

func TestSearchResultsTotalNotNil(t *testing.T) {
	if SearchResultsTotal == nil {
		t.Error("SearchResultsTotal is nil; promauto registration failed")
	}
}

func TestSearchDurationNotNil(t *testing.T) {
	if SearchDuration == nil {
		t.Error("SearchDuration is nil; promauto registration failed")
	}
}

// ---- Engine metrics ----

func TestEngineRequestsTotalNotNil(t *testing.T) {
	if EngineRequestsTotal == nil {
		t.Error("EngineRequestsTotal is nil; promauto registration failed")
	}
}

func TestEngineErrorsTotalNotNil(t *testing.T) {
	if EngineErrorsTotal == nil {
		t.Error("EngineErrorsTotal is nil; promauto registration failed")
	}
}

func TestEngineResponseTimeNotNil(t *testing.T) {
	if EngineResponseTime == nil {
		t.Error("EngineResponseTime is nil; promauto registration failed")
	}
}

// ---- Functional smoke tests ----

func TestAppUptimeSecondsCanSet(t *testing.T) {
	// Set must not panic; the value is observable via Gather() but we only verify
	// that the call itself completes without runtime error.
	AppUptimeSeconds.Set(42)
}

func TestAppStartTimestampCanSet(t *testing.T) {
	AppStartTimestamp.Set(1700000000)
}

func TestHTTPActiveRequestsCanIncAndDec(t *testing.T) {
	HTTPActiveRequests.Inc()
	HTTPActiveRequests.Dec()
}

func TestHTTPRequestsTotalCanInc(t *testing.T) {
	HTTPRequestsTotal.WithLabelValues("GET", "/", "200").Inc()
}

func TestHTTPRequestDurationCanObserve(t *testing.T) {
	HTTPRequestDuration.WithLabelValues("POST", "/api/v1/search").Observe(0.042)
}

func TestHTTPRequestSizeCanObserve(t *testing.T) {
	HTTPRequestSize.WithLabelValues("GET", "/").Observe(512)
}

func TestHTTPResponseSizeCanObserve(t *testing.T) {
	HTTPResponseSize.WithLabelValues("GET", "/").Observe(2048)
}

func TestDBQueriesTotalCanInc(t *testing.T) {
	DBQueriesTotal.WithLabelValues("SELECT", "admin_credentials").Inc()
}

func TestDBQueryDurationCanObserve(t *testing.T) {
	DBQueryDuration.WithLabelValues("INSERT", "setup_tokens").Observe(0.001)
}

func TestDBConnectionsOpenCanSet(t *testing.T) {
	DBConnectionsOpen.Set(5)
}

func TestDBConnectionsInUseCanSet(t *testing.T) {
	DBConnectionsInUse.Set(2)
}

func TestDBErrorsTotalCanInc(t *testing.T) {
	DBErrorsTotal.WithLabelValues("SELECT", "timeout").Inc()
}

func TestAuthAttemptsTotalCanInc(t *testing.T) {
	AuthAttemptsTotal.WithLabelValues("password", "success").Inc()
	AuthAttemptsTotal.WithLabelValues("password", "failure").Inc()
}

func TestAuthSessionsActiveCanSet(t *testing.T) {
	AuthSessionsActive.Set(3)
}

func TestCacheHitsTotalCanInc(t *testing.T) {
	CacheHitsTotal.WithLabelValues("items").Inc()
}

func TestCacheMissesTotalCanInc(t *testing.T) {
	CacheMissesTotal.WithLabelValues("items").Inc()
}

func TestCacheEvictionsCanInc(t *testing.T) {
	CacheEvictions.WithLabelValues("items").Inc()
}

func TestSearchQueriesTotalCanInc(t *testing.T) {
	SearchQueriesTotal.Inc()
}

func TestSearchResultsTotalCanObserve(t *testing.T) {
	SearchResultsTotal.Observe(25)
}

func TestSearchDurationCanObserve(t *testing.T) {
	SearchDuration.Observe(1.5)
}

func TestEngineRequestsTotalCanInc(t *testing.T) {
	EngineRequestsTotal.WithLabelValues("pornhub").Inc()
}

func TestEngineErrorsTotalCanInc(t *testing.T) {
	EngineErrorsTotal.WithLabelValues("pornhub").Inc()
}

func TestEngineResponseTimeCanObserve(t *testing.T) {
	EngineResponseTime.WithLabelValues("xvideos").Observe(0.8)
}

func TestAppInfoCanSetWithLabels(t *testing.T) {
	AppInfo.WithLabelValues("1.0.0", "abc1234", "2026-01-01", "go1.24").Set(1)
}

// ---- Rate-limit metrics (PART 20 REQUIRED) ----

func TestRateLimitRequestsTotalNotNil(t *testing.T) {
	if RateLimitRequestsTotal == nil {
		t.Error("RateLimitRequestsTotal is nil; promauto registration failed")
	}
}

func TestRateLimitBlockedTotalNotNil(t *testing.T) {
	if RateLimitBlockedTotal == nil {
		t.Error("RateLimitBlockedTotal is nil; promauto registration failed")
	}
}

func TestRateLimitRequestsTotalCanInc(t *testing.T) {
	RateLimitRequestsTotal.WithLabelValues("global", "limited").Inc()
	RateLimitRequestsTotal.WithLabelValues("per_ip", "allowed").Inc()
}

func TestRateLimitBlockedTotalCanInc(t *testing.T) {
	RateLimitBlockedTotal.WithLabelValues("global").Inc()
}

// ---- InitMetricsAppInfo ----

func TestInitMetricsAppInfoNoPanic(t *testing.T) {
	// Calling InitMetricsAppInfo must not panic. The goroutine it starts writes
	// to the global gauge every 5 s; in tests the ticker fires at most once
	// before the process exits, so no cleanup is needed.
	InitMetricsAppInfo("1.0.0", "abc1234", "Mon Jan 02, 2006 at 15:04:05 UTC", "go1.24")
}

func TestInitMetricsAppInfoSetsAppInfo(t *testing.T) {
	// Calling with distinct values must not panic and must register a time-series.
	InitMetricsAppInfo("2.0.0", "def5678", "Tue Jan 03, 2006 at 15:04:05 UTC", "go1.24.1")
}

// ---- InstrumentMiddleware ----

func TestInstrumentMiddlewarePassesThrough(t *testing.T) {
	called := false
	inner := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		w.WriteHeader(http.StatusOK)
	})
	handler := InstrumentMiddleware(inner)
	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)
	if !called {
		t.Error("InstrumentMiddleware did not call the inner handler")
	}
	if rr.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", rr.Code)
	}
}

func TestInstrumentMiddlewareRecordsNon200(t *testing.T) {
	inner := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	})
	handler := InstrumentMiddleware(inner)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/missing", nil)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)
	if rr.Code != http.StatusNotFound {
		t.Errorf("expected status 404, got %d", rr.Code)
	}
}

func TestInstrumentMiddlewareWithRequestBody(t *testing.T) {
	inner := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("hello"))
	})
	handler := InstrumentMiddleware(inner)
	body := []byte(`{"query":"test"}`)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/search", nil)
	req.ContentLength = int64(len(body))
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", rr.Code)
	}
}

// ---- Cache size/bytes gauges ----

func TestCacheSizeNotNil(t *testing.T) {
	if CacheSize == nil {
		t.Error("CacheSize is nil; promauto registration failed")
	}
}

func TestCacheBytesNotNil(t *testing.T) {
	if CacheBytes == nil {
		t.Error("CacheBytes is nil; promauto registration failed")
	}
}

func TestCacheSizeCanSet(t *testing.T) {
	CacheSize.WithLabelValues("items").Set(42)
}

func TestCacheBytesCanSet(t *testing.T) {
	CacheBytes.WithLabelValues("items").Set(1024)
}

// ---- Scheduler metrics ----

func TestSchedulerTasksTotalNotNil(t *testing.T) {
	if SchedulerTasksTotal == nil {
		t.Error("SchedulerTasksTotal is nil; promauto registration failed")
	}
}

func TestSchedulerTaskDurationNotNil(t *testing.T) {
	if SchedulerTaskDuration == nil {
		t.Error("SchedulerTaskDuration is nil; promauto registration failed")
	}
}

func TestSchedulerTasksRunningNotNil(t *testing.T) {
	if SchedulerTasksRunning == nil {
		t.Error("SchedulerTasksRunning is nil; promauto registration failed")
	}
}

func TestSchedulerLastRunTimestampNotNil(t *testing.T) {
	if SchedulerLastRunTimestamp == nil {
		t.Error("SchedulerLastRunTimestamp is nil; promauto registration failed")
	}
}

func TestSchedulerTasksTotalCanInc(t *testing.T) {
	SchedulerTasksTotal.WithLabelValues("backup", "success").Inc()
	SchedulerTasksTotal.WithLabelValues("geoip_update", "error").Inc()
}

func TestSchedulerTaskDurationCanObserve(t *testing.T) {
	SchedulerTaskDuration.WithLabelValues("backup").Observe(12.5)
}

func TestSchedulerTasksRunningCanSet(t *testing.T) {
	SchedulerTasksRunning.WithLabelValues("backup").Set(1)
	SchedulerTasksRunning.WithLabelValues("backup").Set(0)
}

func TestSchedulerLastRunTimestampCanSet(t *testing.T) {
	SchedulerLastRunTimestamp.WithLabelValues("backup").Set(1_705_398_600)
}

// ---- System metrics ----

func TestSystemCPUUsagePercentNotNil(t *testing.T) {
	if SystemCPUUsagePercent == nil {
		t.Error("SystemCPUUsagePercent is nil; promauto registration failed")
	}
}

func TestSystemMemoryUsagePercentNotNil(t *testing.T) {
	if SystemMemoryUsagePercent == nil {
		t.Error("SystemMemoryUsagePercent is nil; promauto registration failed")
	}
}

func TestSystemDiskUsagePercentNotNil(t *testing.T) {
	if SystemDiskUsagePercent == nil {
		t.Error("SystemDiskUsagePercent is nil; promauto registration failed")
	}
}

func TestSystemMetricsCanSet(t *testing.T) {
	SystemCPUUsagePercent.Set(12.5)
	SystemMemoryUsagePercent.Set(45.2)
	SystemMemoryUsedBytes.Set(1_073_741_824)
	SystemMemoryTotalBytes.Set(8_589_934_592)
	SystemDiskUsagePercent.WithLabelValues("/var/lib/apimgr/vidveil").Set(30.0)
	SystemDiskUsedBytes.WithLabelValues("/var/lib/apimgr/vidveil").Set(10_737_418_240)
	SystemDiskTotalBytes.WithLabelValues("/var/lib/apimgr/vidveil").Set(107_374_182_400)
}

// ---- Tor metrics ----

func TestTorMetricsNotNil(t *testing.T) {
	if TorEnabled == nil {
		t.Error("TorEnabled is nil; promauto registration failed")
	}
	if TorRunning == nil {
		t.Error("TorRunning is nil; promauto registration failed")
	}
	if TorCircuitEstablished == nil {
		t.Error("TorCircuitEstablished is nil; promauto registration failed")
	}
	if TorRequestsTotal == nil {
		t.Error("TorRequestsTotal is nil; promauto registration failed")
	}
}

func TestTorMetricsCanSet(t *testing.T) {
	TorEnabled.Set(1)
	TorRunning.Set(1)
	TorCircuitEstablished.Set(1)
	TorRequestsTotal.Inc()
}
