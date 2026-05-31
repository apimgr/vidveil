// SPDX-License-Identifier: MIT
package ratelimit

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/apimgr/vidveil/src/config"
	"github.com/apimgr/vidveil/src/server/service/logging"
)

func TestNew(t *testing.T) {
	limiter := NewRateLimiter(true, 100, 60)

	if limiter == nil {
		t.Fatal("New() returned nil")
	}

	if !limiter.enabled {
		t.Error("Expected limiter to be enabled")
	}

	if limiter.requests != 100 {
		t.Errorf("Expected 100 requests, got %d", limiter.requests)
	}

	expectedWindow := time.Duration(60) * time.Second
	if limiter.window != expectedWindow {
		t.Errorf("Expected %v window, got %v", expectedWindow, limiter.window)
	}
}

func TestNewDisabled(t *testing.T) {
	limiter := NewRateLimiter(false, 100, 60)

	if limiter == nil {
		t.Fatal("New() returned nil")
	}

	if limiter.enabled {
		t.Error("Expected limiter to be disabled")
	}
}

func TestAllow(t *testing.T) {
	// 5 requests per second
	limiter := NewRateLimiter(true, 5, 1)

	ip := "192.168.1.1"

	// First 5 requests should be allowed
	for i := 0; i < 5; i++ {
		if !limiter.Allow(ip) {
			t.Errorf("Request %d should be allowed", i+1)
		}
	}

	// 6th request should be denied
	if limiter.Allow(ip) {
		t.Error("6th request should be denied")
	}
}

func TestAllowDisabled(t *testing.T) {
	limiter := NewRateLimiter(false, 1, 60)

	ip := "192.168.1.1"

	// All requests should be allowed when disabled
	for i := 0; i < 100; i++ {
		if !limiter.Allow(ip) {
			t.Errorf("Request %d should be allowed when limiter disabled", i+1)
		}
	}
}

func TestAllowDifferentIPs(t *testing.T) {
	limiter := NewRateLimiter(true, 2, 60)

	// Different IPs should have separate limits
	if !limiter.Allow("192.168.1.1") {
		t.Error("First IP first request should be allowed")
	}
	if !limiter.Allow("192.168.1.1") {
		t.Error("First IP second request should be allowed")
	}
	if limiter.Allow("192.168.1.1") {
		t.Error("First IP third request should be denied")
	}

	// Second IP should still have full quota
	if !limiter.Allow("192.168.1.2") {
		t.Error("Second IP first request should be allowed")
	}
	if !limiter.Allow("192.168.1.2") {
		t.Error("Second IP second request should be allowed")
	}
}

func TestWindowReset(t *testing.T) {
	// 2 requests per 1 second
	limiter := NewRateLimiter(true, 2, 1)

	ip := "192.168.1.1"

	// Use up quota
	limiter.Allow(ip)
	limiter.Allow(ip)

	if limiter.Allow(ip) {
		t.Error("Should be rate limited")
	}

	// Wait for window to reset
	time.Sleep(1100 * time.Millisecond)

	// Should be allowed again
	if !limiter.Allow(ip) {
		t.Error("Should be allowed after window reset")
	}
}

func TestMiddleware(t *testing.T) {
	limiter := NewRateLimiter(true, 2, 60)

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("OK"))
	})

	middleware := limiter.Middleware(handler)

	// First two requests should succeed
	for i := 0; i < 2; i++ {
		req := httptest.NewRequest("GET", "/test", nil)
		req.RemoteAddr = "192.168.1.1:12345"
		rr := httptest.NewRecorder()

		middleware.ServeHTTP(rr, req)

		if rr.Code != http.StatusOK {
			t.Errorf("Request %d: expected status 200, got %d", i+1, rr.Code)
		}
	}

	// Third request should be rate limited
	req := httptest.NewRequest("GET", "/test", nil)
	req.RemoteAddr = "192.168.1.1:12345"
	rr := httptest.NewRecorder()

	middleware.ServeHTTP(rr, req)

	if rr.Code != http.StatusTooManyRequests {
		t.Errorf("Expected status 429, got %d", rr.Code)
	}

	// Check Retry-After header
	retryAfter := rr.Header().Get("Retry-After")
	if retryAfter == "" {
		t.Error("Missing Retry-After header")
	}
}

func TestMiddlewareXForwardedFor(t *testing.T) {
	limiter := NewRateLimiter(true, 1, 60)

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	middleware := limiter.Middleware(handler)

	// First request with X-Forwarded-For
	req := httptest.NewRequest("GET", "/test", nil)
	req.RemoteAddr = "10.0.0.1:12345"
	req.Header.Set("X-Forwarded-For", "203.0.113.1")
	rr := httptest.NewRecorder()

	middleware.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", rr.Code)
	}

	// Second request from same X-Forwarded-For should be limited
	// Different proxy IP, same client IP
	req2 := httptest.NewRequest("GET", "/test", nil)
	req2.RemoteAddr = "10.0.0.2:12345"
	req2.Header.Set("X-Forwarded-For", "203.0.113.1")
	rr2 := httptest.NewRecorder()

	middleware.ServeHTTP(rr2, req2)
	if rr2.Code != http.StatusTooManyRequests {
		t.Errorf("Expected status 429 (same X-Forwarded-For), got %d", rr2.Code)
	}
}

func TestRateLimitHeaders(t *testing.T) {
	limiter := NewRateLimiter(true, 10, 60)

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	middleware := limiter.Middleware(handler)

	req := httptest.NewRequest("GET", "/test", nil)
	req.RemoteAddr = "192.168.1.1:12345"
	rr := httptest.NewRecorder()

	middleware.ServeHTTP(rr, req)

	// Check rate limit headers — X-RateLimit-Limit is intentionally absent
	// (threshold disclosure, PART 11: must not reveal the exact threshold)
	if rr.Header().Get("X-RateLimit-Limit") != "" {
		t.Error("X-RateLimit-Limit must not be set (threshold disclosure risk per AI.md PART 11)")
	}

	if rr.Header().Get("X-RateLimit-Remaining") == "" {
		t.Error("Missing X-RateLimit-Remaining header")
	}

	if rr.Header().Get("X-RateLimit-Reset") == "" {
		t.Error("Missing X-RateLimit-Reset header")
	}
}

func TestRemaining(t *testing.T) {
	limiter := NewRateLimiter(true, 5, 60)
	ip := "192.168.1.1"

	// Initially should have all requests remaining
	remaining := limiter.Remaining(ip)
	if remaining != 5 {
		t.Errorf("Expected 5 remaining, got %d", remaining)
	}

	// Use one request
	limiter.Allow(ip)
	remaining = limiter.Remaining(ip)
	if remaining != 4 {
		t.Errorf("Expected 4 remaining, got %d", remaining)
	}
}

func TestReset(t *testing.T) {
	limiter := NewRateLimiter(true, 5, 60)
	ip := "192.168.1.1"

	// Make a request to establish a window
	limiter.Allow(ip)

	resetTime := limiter.Reset(ip)

	// Reset should be approximately 60 seconds from now
	expectedReset := time.Now().Add(60 * time.Second)
	diff := resetTime.Sub(expectedReset)

	if diff > 2*time.Second || diff < -2*time.Second {
		t.Errorf("Reset time %v too far from expected %v (diff: %v)", resetTime, expectedReset, diff)
	}
}

func TestSetHeaders(t *testing.T) {
	limiter := NewRateLimiter(true, 10, 60)

	rr := httptest.NewRecorder()
	limiter.SetHeaders(rr, "192.168.1.1")

	// X-RateLimit-Limit must not be set (threshold disclosure, AI.md PART 11)
	if rr.Header().Get("X-RateLimit-Limit") != "" {
		t.Errorf("X-RateLimit-Limit must not be set, got %s", rr.Header().Get("X-RateLimit-Limit"))
	}
	// Remaining and Reset must be present
	if rr.Header().Get("X-RateLimit-Remaining") == "" {
		t.Error("Missing X-RateLimit-Remaining header")
	}
	if rr.Header().Get("X-RateLimit-Reset") == "" {
		t.Error("Missing X-RateLimit-Reset header")
	}
}

func TestNewDefaults(t *testing.T) {
	// Test with zero/negative values - should use defaults per AI.md PART 1
	limiter := NewRateLimiter(true, 0, 0)

	if limiter.requests != 100 {
		t.Errorf("Expected default requests=100 (per PART 1), got %d", limiter.requests)
	}

	expectedWindow := time.Duration(60) * time.Second
	if limiter.window != expectedWindow {
		t.Errorf("Expected default window=%v, got %v", expectedWindow, limiter.window)
	}
}

// TestEndpointLimiters tests endpoint-specific rate limiting per AI.md PART 1
func TestEndpointLimiters(t *testing.T) {
	el := NewEndpointLimiters(true)

	// Verify all endpoint types have limiters
	endpoints := []string{
		EndpointLogin,
		EndpointPasswordReset,
		EndpointAPIAuth,
		EndpointAPIUnauth,
		EndpointFileUpload,
		EndpointDefault,
	}

	for _, ep := range endpoints {
		l := el.Get(ep)
		if l == nil {
			t.Errorf("Endpoint %s should have a limiter", ep)
		}
	}
}

// TestEndpointLimiterDefaults verifies default limits per AI.md PART 1
func TestEndpointLimiterDefaults(t *testing.T) {
	el := NewEndpointLimiters(true)

	tests := []struct {
		endpoint         string
		expectedRequests int
		expectedWindow   time.Duration
	}{
		{EndpointLogin, 5, 15 * time.Minute},
		{EndpointPasswordReset, 3, time.Hour},
		{EndpointAPIAuth, 100, time.Minute},
		{EndpointAPIUnauth, 20, time.Minute},
		{EndpointFileUpload, 10, time.Hour},
	}

	for _, tt := range tests {
		l := el.Get(tt.endpoint)
		if l.requests != tt.expectedRequests {
			t.Errorf("%s: expected %d requests, got %d", tt.endpoint, tt.expectedRequests, l.requests)
		}
		if l.window != tt.expectedWindow {
			t.Errorf("%s: expected %v window, got %v", tt.endpoint, tt.expectedWindow, l.window)
		}
	}
}

// TestEndpointLimiterLoginLimit tests login rate limit (5 per 15 min per PART 1)
func TestEndpointLimiterLoginLimit(t *testing.T) {
	el := NewEndpointLimiters(true)
	ip := "192.168.1.1"

	// First 5 login attempts should be allowed
	for i := 0; i < 5; i++ {
		if !el.AllowLogin(ip) {
			t.Errorf("Login attempt %d should be allowed", i+1)
		}
	}

	// 6th login attempt should be denied
	if el.AllowLogin(ip) {
		t.Error("6th login attempt should be denied per PART 1")
	}
}

// TestEndpointLimiterAPIAuthLimit tests authenticated API limit (100 per min per PART 1)
func TestEndpointLimiterAPIAuthLimit(t *testing.T) {
	el := NewEndpointLimiters(true)
	ip := "192.168.1.1"

	// All 100 requests should be allowed
	for i := 0; i < 100; i++ {
		if !el.AllowAPIAuth(ip) {
			t.Errorf("API auth request %d should be allowed", i+1)
		}
	}

	// 101st request should be denied
	if el.AllowAPIAuth(ip) {
		t.Error("101st API auth request should be denied per PART 1")
	}
}

// TestEndpointLimiterAPIUnauthLimit tests unauthenticated API limit (20 per min per PART 1)
func TestEndpointLimiterAPIUnauthLimit(t *testing.T) {
	el := NewEndpointLimiters(true)
	ip := "192.168.1.1"

	// First 20 requests should be allowed
	for i := 0; i < 20; i++ {
		if !el.AllowAPIUnauth(ip) {
			t.Errorf("API unauth request %d should be allowed", i+1)
		}
	}

	// 21st request should be denied
	if el.AllowAPIUnauth(ip) {
		t.Error("21st API unauth request should be denied per PART 1")
	}
}

// TestEndpointLimiterIndependence tests that different endpoints have independent limits
func TestEndpointLimiterIndependence(t *testing.T) {
	el := NewEndpointLimiters(true)
	ip := "192.168.1.1"

	// Exhaust login limit
	for i := 0; i < 5; i++ {
		el.AllowLogin(ip)
	}
	if el.AllowLogin(ip) {
		t.Error("Login should be rate limited")
	}

	// API auth should still work (different limiter)
	if !el.AllowAPIAuth(ip) {
		t.Error("API auth should be allowed (independent limiter)")
	}
}

// TestEndpointLimiterDisabled tests that disabled limiters allow all requests
func TestEndpointLimiterDisabled(t *testing.T) {
	el := NewEndpointLimiters(false)
	ip := "192.168.1.1"

	// Should allow unlimited requests when disabled
	for i := 0; i < 100; i++ {
		if !el.AllowLogin(ip) {
			t.Errorf("Login %d should be allowed when disabled", i+1)
		}
	}
}

// newTestLogger creates a minimal AppLogger with all file outputs disabled.
// This is safe to use in tests — no files are opened.
func newTestLogger(t *testing.T) *logging.AppLogger {
	t.Helper()
	cfg := &config.AppConfig{}
	cfg.Server.Logs.Level = "info"
	logger, err := logging.NewAppLogger(cfg)
	if err != nil {
		t.Fatalf("newTestLogger: %v", err)
	}
	return logger
}

// TestRateLimiterSetLogger covers the RateLimiter.SetLogger path (0% coverage).
func TestRateLimiterSetLogger(t *testing.T) {
	limiter := NewRateLimiter(true, 5, 60)
	logger := newTestLogger(t)
	limiter.SetLogger(logger)
	if limiter.logger != logger {
		t.Error("SetLogger did not store the logger")
	}
}

// TestRateLimiterSetLoggerNil confirms nil is accepted without panic.
func TestRateLimiterSetLoggerNil(t *testing.T) {
	limiter := NewRateLimiter(true, 5, 60)
	limiter.SetLogger(nil)
	if limiter.logger != nil {
		t.Error("SetLogger(nil) should store nil")
	}
}

// TestEndpointLimitersSetLogger covers EndpointLimiters.SetLogger (0% coverage).
// It verifies the logger is propagated to every child limiter.
func TestEndpointLimitersSetLogger(t *testing.T) {
	el := NewEndpointLimiters(true)
	logger := newTestLogger(t)
	el.SetLogger(logger)

	if el.logger != logger {
		t.Error("EndpointLimiters.SetLogger did not store the logger")
	}
	for name, l := range el.limiters {
		if l.logger != logger {
			t.Errorf("child limiter %q did not receive the logger", name)
		}
	}
}

// TestEndpointLimitersGetUnknownFallsBackToDefault covers the fallback branch in
// EndpointLimiters.Get when an unknown endpoint key is requested (currently 80%).
func TestEndpointLimitersGetUnknownFallsBackToDefault(t *testing.T) {
	el := NewEndpointLimiters(true)
	got := el.Get("nonexistent_endpoint_xyz")
	want := el.Get(EndpointDefault)
	if got != want {
		t.Error("Get with unknown endpoint should return the default limiter")
	}
}

// TestEndpointLimiterAllowPasswordReset covers AllowPasswordReset (0% coverage).
// Limit per AI.md PART 12: 3 per hour.
func TestEndpointLimiterAllowPasswordReset(t *testing.T) {
	el := NewEndpointLimiters(true)
	ip := "10.0.0.1"

	for i := 0; i < 3; i++ {
		if !el.AllowPasswordReset(ip) {
			t.Errorf("password reset attempt %d should be allowed", i+1)
		}
	}

	if el.AllowPasswordReset(ip) {
		t.Error("4th password reset attempt should be denied per PART 12")
	}
}

// TestEndpointLimiterAllowFileUpload covers AllowFileUpload (0% coverage).
// Limit per AI.md PART 12: 10 per hour.
func TestEndpointLimiterAllowFileUpload(t *testing.T) {
	el := NewEndpointLimiters(true)
	ip := "10.0.0.2"

	for i := 0; i < 10; i++ {
		if !el.AllowFileUpload(ip) {
			t.Errorf("file upload attempt %d should be allowed", i+1)
		}
	}

	if el.AllowFileUpload(ip) {
		t.Error("11th file upload should be denied per PART 12")
	}
}

// TestRemainingUnknownIP covers the "not ok" branch in Remaining (line 207-209).
// When the IP has never made a request, the map lookup fails and the full
// allowance must be returned.
func TestRemainingUnknownIP(t *testing.T) {
	limiter := NewRateLimiter(true, 7, 60)
	remaining := limiter.Remaining("never-seen-ip")
	if remaining != 7 {
		t.Errorf("expected 7 remaining for unknown IP, got %d", remaining)
	}
}

// TestRemainingDisabled covers the disabled-limiter path in Remaining.
func TestRemainingDisabled(t *testing.T) {
	limiter := NewRateLimiter(false, 7, 60)
	remaining := limiter.Remaining("any-ip")
	if remaining != 7 {
		t.Errorf("expected 7 remaining when disabled, got %d", remaining)
	}
}

// TestResetUnknownIP covers the "!ok" branch in Reset (returns time.Now()).
func TestResetUnknownIP(t *testing.T) {
	limiter := NewRateLimiter(true, 5, 60)
	before := time.Now()
	reset := limiter.Reset("never-seen-ip-for-reset")
	after := time.Now()

	if reset.Before(before) || reset.After(after.Add(time.Second)) {
		t.Errorf("Reset for unknown IP should be approximately now, got %v", reset)
	}
}

// TestResetEmptyTimestamps covers the "len(client.timestamps)==0" branch in Reset.
// We create a client entry via Allow, then wait for its window to expire and call
// Allow again so the slice gets pruned to zero before Reset is called.
func TestResetEmptyTimestamps(t *testing.T) {
	// 1-second window so the timestamp becomes stale quickly.
	limiter := NewRateLimiter(true, 2, 1)
	ip := "reset-empty-test-ip"

	limiter.Allow(ip)
	limiter.Allow(ip)

	// Wait until the window expires so the next Allow() prunes timestamps to zero
	// and still adds a fresh one — but we need the slice empty case.
	// Instead, directly manipulate the client: clear its timestamps.
	limiter.mu.RLock()
	client := limiter.clients[ip]
	limiter.mu.RUnlock()

	client.mu.Lock()
	client.timestamps = client.timestamps[:0]
	client.mu.Unlock()

	// Now Reset should take the "len==0" path inside the already-locked block.
	// The outer check "!ok || len(client.timestamps)==0" fires because we cleared it.
	before := time.Now()
	reset := limiter.Reset(ip)
	after := time.Now()

	if reset.Before(before) || reset.After(after.Add(time.Second)) {
		t.Errorf("Reset with empty timestamps should return approximately now, got %v", reset)
	}
}

// TestResetDisabled covers the disabled path in Reset.
func TestResetDisabled(t *testing.T) {
	limiter := NewRateLimiter(false, 5, 60)
	before := time.Now()
	reset := limiter.Reset("any-ip")
	after := time.Now()

	if reset.Before(before) || reset.After(after.Add(time.Second)) {
		t.Errorf("Reset when disabled should return approximately now, got %v", reset)
	}
}

// TestMiddlewareXForwardedForWithComma covers the comma-splitting branch in
// Middleware (currently 80%). When X-Forwarded-For contains "clientIP, proxyIP",
// only the first IP before the comma should be used for rate limiting.
func TestMiddlewareXForwardedForWithComma(t *testing.T) {
	limiter := NewRateLimiter(true, 1, 60)

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	middleware := limiter.Middleware(handler)

	// First request: comma-delimited X-Forwarded-For
	req := httptest.NewRequest("GET", "/test", nil)
	req.RemoteAddr = "10.0.0.1:9000"
	req.Header.Set("X-Forwarded-For", "203.0.113.5, 10.0.0.1")
	rr := httptest.NewRecorder()
	middleware.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Errorf("first request: expected 200, got %d", rr.Code)
	}

	// Second request with same first IP in the comma list — should be rate limited
	req2 := httptest.NewRequest("GET", "/test", nil)
	req2.RemoteAddr = "10.0.0.2:9000"
	req2.Header.Set("X-Forwarded-For", "203.0.113.5, 10.0.0.2")
	rr2 := httptest.NewRecorder()
	middleware.ServeHTTP(rr2, req2)
	if rr2.Code != http.StatusTooManyRequests {
		t.Errorf("second request same client IP: expected 429, got %d", rr2.Code)
	}
}

// TestMiddlewareRateLimitedWithLogger covers the logger branch inside the
// rate-limited path of Middleware (currently 80%).
func TestMiddlewareRateLimitedWithLogger(t *testing.T) {
	limiter := NewRateLimiter(true, 1, 60)
	limiter.SetLogger(newTestLogger(t))

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	middleware := limiter.Middleware(handler)

	// Use X-Real-IP so the same logical IP is recorded on both requests,
	// regardless of the ephemeral port in RemoteAddr.
	clientIP := "192.0.2.99"

	// Consume the one allowed request
	req := httptest.NewRequest("GET", "/api/v1/resource", nil)
	req.RemoteAddr = "10.0.0.1:1234"
	req.Header.Set("X-Real-IP", clientIP)
	rr := httptest.NewRecorder()
	middleware.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Errorf("first request should be OK, got %d", rr.Code)
	}

	// Second request triggers rate limit — logger.Security() must be called without panic
	req2 := httptest.NewRequest("GET", "/api/v1/resource", nil)
	req2.RemoteAddr = "10.0.0.1:1235"
	req2.Header.Set("X-Real-IP", clientIP)
	rr2 := httptest.NewRecorder()
	middleware.ServeHTTP(rr2, req2)
	if rr2.Code != http.StatusTooManyRequests {
		t.Errorf("second request should be 429, got %d", rr2.Code)
	}
}

// TestItoa covers all branches in the local itoa helper (currently 80%).
// The negative number branch is the missing case.
func TestItoa(t *testing.T) {
	tests := []struct {
		input    int
		expected string
	}{
		{0, "0"},
		{1, "1"},
		{42, "42"},
		{-1, "-1"},
		{-100, "-100"},
		{1000000, "1000000"},
	}

	for _, tt := range tests {
		got := itoa(tt.input)
		if got != tt.expected {
			t.Errorf("itoa(%d) = %q, want %q", tt.input, got, tt.expected)
		}
	}
}

// TestCleanupPrunesStaleClients verifies the cleanup goroutine's pruning logic
// by injecting stale entries directly and exercising the same path cleanup()
// uses, via a freshly started limiter whose ticker we cannot easily advance.
// We verify the observable postcondition: after the cleanup window passes,
// Remaining() still returns the full quota (stale timestamps are pruned on the
// Allow/Remaining call path), which indirectly confirms the data structures
// behave correctly. Direct goroutine coverage of cleanup() remains partial
// because the 5-minute ticker cannot be advanced without refactoring the
// production code to accept an injectable clock.
func TestCleanupPrunesStaleClients(t *testing.T) {
	// 1-second window, 2 requests allowed
	limiter := NewRateLimiter(true, 2, 1)
	ip := "cleanup-test-ip"

	// Use up the quota
	limiter.Allow(ip)
	limiter.Allow(ip)
	if limiter.Allow(ip) {
		t.Fatal("third request should be denied")
	}

	// Wait until the window expires
	time.Sleep(1100 * time.Millisecond)

	// After the window expires, Allow prunes timestamps; quota is full again
	if !limiter.Allow(ip) {
		t.Error("request after window expiry should be allowed (stale timestamps pruned)")
	}

	// Inject a fake client with old timestamps to simulate what cleanup() removes
	oldTime := time.Now().Add(-10 * time.Minute)
	limiter.mu.Lock()
	limiter.clients["stale-ip"] = &clientInfo{
		timestamps: []time.Time{oldTime, oldTime},
	}
	limiter.mu.Unlock()

	// Verify stale entry exists
	limiter.mu.RLock()
	_, exists := limiter.clients["stale-ip"]
	limiter.mu.RUnlock()
	if !exists {
		t.Fatal("stale-ip should be present before cleanup")
	}

	// Remaining() prunes within the sliding window per-IP but does NOT delete
	// the map entry — that is cleanup()'s job. Confirm Remaining returns full
	// quota for the stale client (all timestamps outside the window).
	remaining := limiter.Remaining("stale-ip")
	if remaining != 2 {
		t.Errorf("stale client should show full remaining quota, got %d", remaining)
	}
}
