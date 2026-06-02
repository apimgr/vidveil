// SPDX-License-Identifier: MIT
// Coverage tests for standalone utility functions in the handler package:
// formatUptime, formatDuration, containsInsensitive, toLowerSimple,
// indexOfString, csvEscape, BuildDateTime, isLoopbackRequest,
// ServerMetrics active-connection counters, and GetAnalyticsSummary.
package handler

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

// ── formatUptime ─────────────────────────────────────────────────────────────

func TestFormatUptime_MinutesOnly(t *testing.T) {
	got := formatUptime(45 * time.Minute)
	if got != "45m" {
		t.Errorf("formatUptime(45m) = %q, want %q", got, "45m")
	}
}

func TestFormatUptime_HoursAndMinutes(t *testing.T) {
	got := formatUptime(2*time.Hour + 30*time.Minute)
	if got != "2h 30m" {
		t.Errorf("formatUptime(2h30m) = %q, want %q", got, "2h 30m")
	}
}

func TestFormatUptime_DaysHoursMinutes(t *testing.T) {
	got := formatUptime(3*24*time.Hour + 5*time.Hour + 10*time.Minute)
	if got != "3d 5h 10m" {
		t.Errorf("formatUptime(3d5h10m) = %q, want %q", got, "3d 5h 10m")
	}
}

// ── formatDuration ───────────────────────────────────────────────────────────

func TestFormatDuration_MinutesOnly(t *testing.T) {
	got := formatDuration(15 * time.Minute)
	if got != "15m" {
		t.Errorf("formatDuration(15m) = %q, want %q", got, "15m")
	}
}

func TestFormatDuration_HoursAndMinutes(t *testing.T) {
	got := formatDuration(1*time.Hour + 20*time.Minute)
	if got != "1h 20m" {
		t.Errorf("formatDuration(1h20m) = %q, want %q", got, "1h 20m")
	}
}

func TestFormatDuration_DaysHoursMinutes(t *testing.T) {
	got := formatDuration(2*24*time.Hour + 3*time.Hour + 4*time.Minute)
	if got != "2d 3h 4m" {
		t.Errorf("formatDuration(2d3h4m) = %q, want %q", got, "2d 3h 4m")
	}
}

// ── toLowerSimple ─────────────────────────────────────────────────────────────

func TestToLowerSimple_AllUpper(t *testing.T) {
	if got := toLowerSimple("HELLO"); got != "hello" {
		t.Errorf("toLowerSimple(%q) = %q, want %q", "HELLO", got, "hello")
	}
}

func TestToLowerSimple_Mixed(t *testing.T) {
	if got := toLowerSimple("HeLLo"); got != "hello" {
		t.Errorf("toLowerSimple(%q) = %q, want %q", "HeLLo", got, "hello")
	}
}

func TestToLowerSimple_AlreadyLower(t *testing.T) {
	if got := toLowerSimple("hello"); got != "hello" {
		t.Errorf("toLowerSimple(%q) = %q, want %q", "hello", got, "hello")
	}
}

func TestToLowerSimple_Empty(t *testing.T) {
	if got := toLowerSimple(""); got != "" {
		t.Errorf("toLowerSimple(%q) = %q, want %q", "", got, "")
	}
}

// ── indexOfString ─────────────────────────────────────────────────────────────

func TestIndexOfString_Found(t *testing.T) {
	if got := indexOfString("hello world", "world"); got != 6 {
		t.Errorf("indexOfString found = %d, want 6", got)
	}
}

func TestIndexOfString_NotFound(t *testing.T) {
	if got := indexOfString("hello", "xyz"); got != -1 {
		t.Errorf("indexOfString not-found = %d, want -1", got)
	}
}

func TestIndexOfString_EmptySubstr(t *testing.T) {
	if got := indexOfString("hello", ""); got != 0 {
		t.Errorf("indexOfString empty-substr = %d, want 0", got)
	}
}

func TestIndexOfString_AtStart(t *testing.T) {
	if got := indexOfString("abc", "a"); got != 0 {
		t.Errorf("indexOfString at-start = %d, want 0", got)
	}
}

// ── containsInsensitive ───────────────────────────────────────────────────────

func TestContainsInsensitive_CaseMatch(t *testing.T) {
	if !containsInsensitive("Hello World", "world") {
		t.Error("containsInsensitive: should find 'world' in 'Hello World'")
	}
}

func TestContainsInsensitive_UpperMatch(t *testing.T) {
	if !containsInsensitive("hello world", "HELLO") {
		t.Error("containsInsensitive: should find 'HELLO' in 'hello world'")
	}
}

func TestContainsInsensitive_NotFound(t *testing.T) {
	if containsInsensitive("hello world", "xyz") {
		t.Error("containsInsensitive: should not find 'xyz' in 'hello world'")
	}
}

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

// ── parseLogLine ─────────────────────────────────────────────────────────────

func TestParseLogLine_ShortLine(t *testing.T) {
	h := &AdminHandler{}
	entry := h.parseLogLine("short")
	if entry.Message == "" {
		t.Error("parseLogLine: short line should produce non-empty message")
	}
}

func TestParseLogLine_WithTimestampAndInfo(t *testing.T) {
	h := &AdminHandler{}
	entry := h.parseLogLine("2026-01-01 10:00:00 INFO  test message")
	if entry.Timestamp == "" {
		t.Error("parseLogLine: timestamp should be extracted")
	}
}

func TestParseLogLine_DebugLevel(t *testing.T) {
	h := &AdminHandler{}
	entry := h.parseLogLine("2026-01-01 10:00:00 DEBUG debug details")
	if entry.Level != "DEBUG" {
		t.Errorf("parseLogLine: level = %q, want %q", entry.Level, "DEBUG")
	}
}

func TestParseLogLine_WarnLevel(t *testing.T) {
	h := &AdminHandler{}
	entry := h.parseLogLine("2026-01-01 10:00:00 WARN  warning text")
	if entry.Level != "WARN " {
		t.Errorf("parseLogLine: level = %q, want %q", entry.Level, "WARN ")
	}
}

func TestParseLogLine_ErrorLevel(t *testing.T) {
	h := &AdminHandler{}
	entry := h.parseLogLine("2026-01-01 10:00:00 ERROR something failed")
	if entry.Level != "ERROR" {
		t.Errorf("parseLogLine: level = %q, want %q", entry.Level, "ERROR")
	}
}

func TestParseLogLine_EmptyLine(t *testing.T) {
	h := &AdminHandler{}
	entry := h.parseLogLine("")
	if entry.Level == "" {
		t.Error("parseLogLine: empty line should produce a default level")
	}
}
