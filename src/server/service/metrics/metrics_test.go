// SPDX-License-Identifier: MIT
// Tests for the metrics package: verifies that every promauto-registered variable
// is non-nil after package init and that representative metrics can be used without
// panicking.  All vars are globals initialized at import time.
package metrics

import (
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
	CacheHitsTotal.Inc()
}

func TestCacheMissesTotalCanInc(t *testing.T) {
	CacheMissesTotal.Inc()
}

func TestCacheEvictionsCanInc(t *testing.T) {
	CacheEvictions.Inc()
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
