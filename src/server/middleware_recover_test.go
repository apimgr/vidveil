// SPDX-License-Identifier: MIT
// AI.md PART 16: Coverage tests for guaranteedRecoverer — verifies it always
// renders a real, content-negotiated error body on panic (never a blank body
// or dropped connection), never double-writes when a handler already sent a
// header before panicking, and correctly re-propagates http.ErrAbortHandler.
package server

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestGuaranteedRecoverer_PassesThroughNormalResponse(t *testing.T) {
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("ok"))
	})

	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/", nil)

	guaranteedRecoverer(next).ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", rr.Code)
	}
	if rr.Body.String() != "ok" {
		t.Errorf("expected body %q, got %q", "ok", rr.Body.String())
	}
}

func TestGuaranteedRecoverer_JSONOnPanicNoPriorWrite(t *testing.T) {
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		panic("boom")
	})

	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/items", nil)

	guaranteedRecoverer(next).ServeHTTP(rr, req)

	if rr.Code != http.StatusInternalServerError {
		t.Errorf("expected 500, got %d", rr.Code)
	}
	if ct := rr.Header().Get("Content-Type"); !strings.Contains(ct, "application/json") {
		t.Errorf("expected JSON content-type for /api/ path, got %q", ct)
	}
	if rr.Body.Len() == 0 {
		t.Fatal("expected non-empty body, got blank body (PART 16 violation)")
	}
	if !strings.Contains(rr.Body.String(), `"ok":false`) {
		t.Errorf("expected canonical error envelope, got %q", rr.Body.String())
	}
}

func TestGuaranteedRecoverer_PlainTextOnPanic(t *testing.T) {
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		panic("boom")
	})

	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/dashboard", nil)
	req.Header.Set("Accept", "text/plain")

	guaranteedRecoverer(next).ServeHTTP(rr, req)

	if rr.Code != http.StatusInternalServerError {
		t.Errorf("expected 500, got %d", rr.Code)
	}
	if ct := rr.Header().Get("Content-Type"); !strings.Contains(ct, "text/plain") {
		t.Errorf("expected text/plain content-type, got %q", ct)
	}
	if rr.Body.Len() == 0 {
		t.Fatal("expected non-empty body, got blank body (PART 16 violation)")
	}
}

func TestGuaranteedRecoverer_HTMLOnPanicDefault(t *testing.T) {
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		panic("boom")
	})

	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/dashboard", nil)
	req.Header.Set("Accept", "text/html")

	guaranteedRecoverer(next).ServeHTTP(rr, req)

	if rr.Code != http.StatusInternalServerError {
		t.Errorf("expected 500, got %d", rr.Code)
	}
	if ct := rr.Header().Get("Content-Type"); !strings.Contains(ct, "text/html") {
		t.Errorf("expected text/html content-type, got %q", ct)
	}
	body := rr.Body.String()
	if body == "" {
		t.Fatal("expected non-empty body, got blank body (PART 16 violation)")
	}
	if strings.Contains(body, "boom") {
		t.Error("panic detail leaked into client response (PART 11 Tier 1 violation)")
	}
}

func TestGuaranteedRecoverer_NoDoubleWriteWhenHeaderAlreadySent(t *testing.T) {
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusAccepted)
		w.Write([]byte("partial"))
		panic("boom after write")
	})

	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/", nil)

	guaranteedRecoverer(next).ServeHTTP(rr, req)

	if rr.Code != http.StatusAccepted {
		t.Errorf("expected original status 202 to survive (no corrupt double-write), got %d", rr.Code)
	}
	if rr.Body.String() != "partial" {
		t.Errorf("expected original body to survive untouched, got %q", rr.Body.String())
	}
}

func TestGuaranteedRecoverer_RepropagatesErrAbortHandler(t *testing.T) {
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		panic(http.ErrAbortHandler)
	})

	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/", nil)

	defer func() {
		rvr := recover()
		if rvr != http.ErrAbortHandler {
			t.Errorf("expected http.ErrAbortHandler to re-panic, got %v", rvr)
		}
	}()

	guaranteedRecoverer(next).ServeHTTP(rr, req)
	t.Fatal("expected panic to propagate for http.ErrAbortHandler")
}

func TestWriteGuaranteedError_NoPanicDetailLeak(t *testing.T) {
	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/x", nil)

	writeGuaranteedError(rr, req, http.StatusInternalServerError)

	if strings.Contains(rr.Body.String(), "runtime") || strings.Contains(rr.Body.String(), "goroutine") {
		t.Error("stack trace detail leaked into client response")
	}
}
