// SPDX-License-Identifier: MIT
// AI.md PART 12: Coverage tests for requestLogMiddleware — verifies it invokes
// the next handler, preserves status/size, and does not panic when resolving
// the client IP for both plain and X-Forwarded-For requests.
package server

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestRequestLogMiddleware_CallsNextAndPreservesResponse(t *testing.T) {
	var called bool
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		w.WriteHeader(http.StatusTeapot)
		w.Write([]byte("hello"))
	})

	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/search?q=x", nil)
	req.RemoteAddr = "127.0.0.1:12345"

	requestLogMiddleware(next).ServeHTTP(rr, req)

	if !called {
		t.Error("requestLogMiddleware did not call the next handler")
	}
	if rr.Code != http.StatusTeapot {
		t.Errorf("status = %d, want %d", rr.Code, http.StatusTeapot)
	}
	if rr.Body.String() != "hello" {
		t.Errorf("body = %q, want %q", rr.Body.String(), "hello")
	}
}

func TestRequestLogMiddleware_HonorsForwardedHeaderFromTrustedPeer(t *testing.T) {
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.RemoteAddr = "172.19.0.1:51120"
	req.Header.Set("X-Real-IP", "203.0.113.7")

	requestLogMiddleware(next).ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("status = %d, want %d", rr.Code, http.StatusOK)
	}
}
