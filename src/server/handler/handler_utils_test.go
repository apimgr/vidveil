// SPDX-License-Identifier: MIT
// Coverage tests for standalone utility functions in the handler package:
// csvEscape, BuildDateTime, isLoopbackRequest,
// ServerMetrics active-connection counters, and GetAnalyticsSummary.
package handler

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

// ── csvEscape ────────────────────────────────────────────────────────────────

func TestCsvEscape_Simple(t *testing.T) {
	got := csvEscape("hello")
	if got != `"hello"` {
		t.Errorf("csvEscape simple = %q, want %q", got, `"hello"`)
	}
}

func TestCsvEscape_WithInternalQuote(t *testing.T) {
	got := csvEscape(`say "hi"`)
	if got != `"say ""hi"""` {
		t.Errorf("csvEscape with quote = %q, want %q", got, `"say ""hi"""`)
	}
}

func TestCsvEscape_Empty(t *testing.T) {
	got := csvEscape("")
	if got != `""` {
		t.Errorf("csvEscape empty = %q, want %q", got, `""`)
	}
}

// ── BuildDateTime ─────────────────────────────────────────────────────────────

func TestBuildDateTime_EmptyReturnUnknown(t *testing.T) {
	got := BuildDateTime()
	if got == "" {
		t.Error("BuildDateTime: should never return empty string")
	}
}

// ── isLoopbackRequest ─────────────────────────────────────────────────────────

func TestIsLoopbackRequest_Localhost(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.RemoteAddr = "127.0.0.1:12345"
	if !isLoopbackRequest(req) {
		t.Error("isLoopbackRequest: 127.0.0.1 should be loopback")
	}
}

func TestIsLoopbackRequest_IPv6Loopback(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.RemoteAddr = "[::1]:12345"
	if !isLoopbackRequest(req) {
		t.Error("isLoopbackRequest: ::1 should be loopback")
	}
}

func TestIsLoopbackRequest_ExternalIP(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.RemoteAddr = "203.0.113.1:12345"
	if isLoopbackRequest(req) {
		t.Error("isLoopbackRequest: 203.0.113.1 should not be loopback")
	}
}

func TestIsLoopbackRequest_InvalidAddr(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.RemoteAddr = "notanip"
	if isLoopbackRequest(req) {
		t.Error("isLoopbackRequest: invalid addr should return false")
	}
}

// newTestMetrics constructs a ServerMetrics suitable for unit tests — no appConfig
// or engineMgr required since the methods under test use only atomic counters.
func newTestMetrics() *ServerMetrics {
	return &ServerMetrics{
		startTime:   time.Now(),
		requests24h: newSlidingWindowCounter(),
		searches24h: newSlidingWindowCounter(),
	}
}

// ── ServerMetrics active connections ─────────────────────────────────────────

func TestServerMetrics_ActiveConnections_InitialZero(t *testing.T) {
	m := newTestMetrics()
	if got := m.GetActiveConnections(); got != 0 {
		t.Errorf("initial active connections = %d, want 0", got)
	}
}

func TestServerMetrics_ActiveConnections_Increment(t *testing.T) {
	m := newTestMetrics()
	m.IncrementActiveConnections()
	m.IncrementActiveConnections()
	if got := m.GetActiveConnections(); got != 2 {
		t.Errorf("active connections after 2 increments = %d, want 2", got)
	}
}

func TestServerMetrics_ActiveConnections_Decrement(t *testing.T) {
	m := newTestMetrics()
	m.IncrementActiveConnections()
	m.IncrementActiveConnections()
	m.DecrementActiveConnections()
	if got := m.GetActiveConnections(); got != 1 {
		t.Errorf("active connections after 2 inc + 1 dec = %d, want 1", got)
	}
}

// ── ServerMetrics GetAnalyticsSummary ────────────────────────────────────────

func TestServerMetrics_GetAnalyticsSummary_ZeroState(t *testing.T) {
	m := newTestMetrics()
	s := m.GetAnalyticsSummary()
	if s.SearchesTotal != 0 {
		t.Errorf("GetAnalyticsSummary zero: SearchesTotal = %d, want 0", s.SearchesTotal)
	}
	if s.CacheHitPct != 0 {
		t.Errorf("GetAnalyticsSummary zero: CacheHitPct = %v, want 0", s.CacheHitPct)
	}
}

func TestServerMetrics_GetAnalyticsSummary_AfterSearches(t *testing.T) {
	m := newTestMetrics()
	m.IncrementSearches()
	m.IncrementSearches()
	m.IncrementCacheHits()
	s := m.GetAnalyticsSummary()
	if s.SearchesTotal != 2 {
		t.Errorf("GetAnalyticsSummary searches: SearchesTotal = %d, want 2", s.SearchesTotal)
	}
	if s.CacheHitsTotal != 1 {
		t.Errorf("GetAnalyticsSummary searches: CacheHitsTotal = %d, want 1", s.CacheHitsTotal)
	}
}

func TestServerMetrics_GetAnalyticsSummary_CacheHitPct(t *testing.T) {
	m := newTestMetrics()
	m.IncrementSearches()
	m.IncrementSearches()
	m.IncrementCacheHits()
	m.IncrementCacheHits()
	s := m.GetAnalyticsSummary()
	if s.CacheHitPct != 100.0 {
		t.Errorf("CacheHitPct = %v, want 100.0", s.CacheHitPct)
	}
}

