// SPDX-License-Identifier: MIT
package ratelimit

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestNew(t *testing.T) {
	limiter := New(true, 100, 60)

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
	limiter := New(false, 100, 60)

	if limiter == nil {
		t.Fatal("New() returned nil")
	}

	if limiter.enabled {
		t.Error("Expected limiter to be disabled")
	}
}

func TestAllow(t *testing.T) {
	limiter := New(true, 5, 1) // 5 requests per second

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
	limiter := New(false, 1, 60)

	ip := "192.168.1.1"

	// All requests should be allowed when disabled
	for i := 0; i < 100; i++ {
		if !limiter.Allow(ip) {
			t.Errorf("Request %d should be allowed when limiter disabled", i+1)
		}
	}
}

func TestAllowDifferentIPs(t *testing.T) {
	limiter := New(true, 2, 60)

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
	limiter := New(true, 2, 1) // 2 requests per 1 second

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
	limiter := New(true, 2, 60)

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
	limiter := New(true, 1, 60)

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
	req2 := httptest.NewRequest("GET", "/test", nil)
	req2.RemoteAddr = "10.0.0.2:12345" // Different proxy IP
	req2.Header.Set("X-Forwarded-For", "203.0.113.1") // Same client IP
	rr2 := httptest.NewRecorder()

	middleware.ServeHTTP(rr2, req2)
	if rr2.Code != http.StatusTooManyRequests {
		t.Errorf("Expected status 429 (same X-Forwarded-For), got %d", rr2.Code)
	}
}

func TestRateLimitHeaders(t *testing.T) {
	limiter := New(true, 10, 60)

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	middleware := limiter.Middleware(handler)

	req := httptest.NewRequest("GET", "/test", nil)
	req.RemoteAddr = "192.168.1.1:12345"
	rr := httptest.NewRecorder()

	middleware.ServeHTTP(rr, req)

	// Check rate limit headers
	if rr.Header().Get("X-RateLimit-Limit") == "" {
		t.Error("Missing X-RateLimit-Limit header")
	}

	if rr.Header().Get("X-RateLimit-Remaining") == "" {
		t.Error("Missing X-RateLimit-Remaining header")
	}

	if rr.Header().Get("X-RateLimit-Reset") == "" {
		t.Error("Missing X-RateLimit-Reset header")
	}
}

func TestRemaining(t *testing.T) {
	limiter := New(true, 5, 60)
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
	limiter := New(true, 5, 60)
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
	limiter := New(true, 10, 60)

	rr := httptest.NewRecorder()
	limiter.SetHeaders(rr, "192.168.1.1")

	if rr.Header().Get("X-RateLimit-Limit") != "10" {
		t.Errorf("Expected X-RateLimit-Limit=10, got %s", rr.Header().Get("X-RateLimit-Limit"))
	}
}

func TestNewDefaults(t *testing.T) {
	// Test with zero/negative values - should use defaults
	limiter := New(true, 0, 0)

	if limiter.requests != 120 {
		t.Errorf("Expected default requests=120, got %d", limiter.requests)
	}

	expectedWindow := time.Duration(60) * time.Second
	if limiter.window != expectedWindow {
		t.Errorf("Expected default window=%v, got %v", expectedWindow, limiter.window)
	}
}
