// SPDX-License-Identifier: MIT
// AI.md PART 28: Coverage tests for /server/about, /server/privacy, /server/contact page routes.
// These hit the renderServerTemplate main body (lines 90-125 of handler/server.go).
package server

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestServerPage_About_Returns200(t *testing.T) {
	s := newTestServer(t)
	req := httptest.NewRequest(http.MethodGet, "/server/about", nil)
	rr := httptest.NewRecorder()
	s.router.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Errorf("GET /server/about: status=%d want 200; body=%s", rr.Code, rr.Body.String())
	}
	ct := rr.Header().Get("Content-Type")
	if !strings.Contains(ct, "text/html") {
		t.Errorf("GET /server/about: Content-Type=%q want text/html", ct)
	}
	for _, expected := range []string{
		"IP Geolocation by DB-IP",
		"Country and ASN data licensed CC BY 4.0 by the Number Resource Organization (NRO).",
	} {
		if !strings.Contains(rr.Body.String(), expected) {
			t.Errorf("GET /server/about: missing required attribution %q", expected)
		}
	}
}

func TestServerPage_Privacy_Returns200(t *testing.T) {
	s := newTestServer(t)
	req := httptest.NewRequest(http.MethodGet, "/server/privacy", nil)
	rr := httptest.NewRecorder()
	s.router.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Errorf("GET /server/privacy: status=%d want 200; body=%s", rr.Code, rr.Body.String())
	}
}

func TestServerPage_Help_UsesCurrentProjectGuidance(t *testing.T) {
	s := newTestServer(t)
	req := httptest.NewRequest(http.MethodGet, "/server/help", nil)
	rr := httptest.NewRecorder()
	s.router.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("GET /server/help: status=%d want 200; body=%s", rr.Code, rr.Body.String())
	}
	body := rr.Body.String()
	if !strings.Contains(body, "Preferences are stored in browser cookies") {
		t.Error("GET /server/help: missing accurate browser preference storage guidance")
	}
	if strings.Contains(body, "example.com/thumb.jpg") {
		t.Error("GET /server/help: contains a generic thumbnail proxy URL")
	}
}

func TestServerPage_Contact_Returns200(t *testing.T) {
	s := newTestServer(t)
	req := httptest.NewRequest(http.MethodGet, "/server/contact", nil)
	rr := httptest.NewRecorder()
	s.router.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Errorf("GET /server/contact: status=%d want 200; body=%s", rr.Code, rr.Body.String())
	}
}
