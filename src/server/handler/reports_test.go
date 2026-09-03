// SPDX-License-Identifier: MIT
// AI.md PART 11: tests for the browser report ingestion endpoints, added
// alongside reports.go per the project's same-pass-test rule.
package handler

import (
	"net/http/httptest"
	"strings"
	"testing"
)

func TestReportsCSP_LegacyReport_Returns204(t *testing.T) {
	h := NewServerHandler(nil)
	body := `{"csp-report":{"document-uri":"https://x/y?token=secret","violated-directive":"script-src","blocked-uri":"https://evil/x"}}`
	req := httptest.NewRequest("POST", "/api/v1/server/reports/csp", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/csp-report")
	rr := httptest.NewRecorder()

	h.ReportsCSP(rr, req)

	if rr.Code != 204 {
		t.Fatalf("status = %d, want 204", rr.Code)
	}
	if rr.Body.Len() != 0 {
		t.Fatalf("body must be empty (no user content echoed), got %q", rr.Body.String())
	}
}

func TestReportsNEL_ReportsJSON_Returns204(t *testing.T) {
	h := NewServerHandler(nil)
	body := `[{"type":"network-error","url":"https://x/y?sid=abc","body":{"phase":"connection","type":"tcp.reset"}}]`
	req := httptest.NewRequest("POST", "/api/v1/server/reports/nel", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/reports+json")
	rr := httptest.NewRecorder()

	h.ReportsNEL(rr, req)

	if rr.Code != 204 {
		t.Fatalf("status = %d, want 204", rr.Code)
	}
}

func TestReportsDefault_MalformedBody_Returns204(t *testing.T) {
	h := NewServerHandler(nil)
	req := httptest.NewRequest("POST", "/api/v1/server/reports/default", strings.NewReader("{not json"))
	req.Header.Set("Content-Type", "application/reports+json")
	rr := httptest.NewRecorder()

	h.ReportsDefault(rr, req)

	if rr.Code != 204 {
		t.Fatalf("status = %d, want 204", rr.Code)
	}
}

func TestSummarizeReport_StripsURLQueryStrings(t *testing.T) {
	body := []byte(`{"csp-report":{"document-uri":"https://host/path?token=SECRET&sid=99","blocked-uri":"https://evil/x?k=v"}}`)
	fields := summarizeReport("application/csp-report", body)

	csp, ok := fields["csp_report"].(map[string]interface{})
	if !ok {
		t.Fatalf("expected csp_report object, got %T", fields["csp_report"])
	}
	doc, _ := csp["document-uri"].(string)
	if strings.Contains(doc, "token") || strings.Contains(doc, "SECRET") || strings.Contains(doc, "?") {
		t.Fatalf("document-uri query not stripped: %q", doc)
	}
	if doc != "https://host/path" {
		t.Fatalf("document-uri = %q, want https://host/path", doc)
	}
	blocked, _ := csp["blocked-uri"].(string)
	if strings.Contains(blocked, "?") {
		t.Fatalf("blocked-uri query not stripped: %q", blocked)
	}
}

func TestSummarizeReport_ReportsJSONArrayURLsStripped(t *testing.T) {
	body := []byte(`[{"type":"csp-violation","url":"https://host/p?x=1","body":{"documentURL":"https://host/d?tok=zzz","blockedURL":"https://e/b"}}]`)
	fields := summarizeReport("application/reports+json", body)

	arr, ok := fields["report"].([]interface{})
	if !ok || len(arr) == 0 {
		t.Fatalf("expected report array, got %T", fields["report"])
	}
	entry := arr[0].(map[string]interface{})
	if u, _ := entry["url"].(string); strings.Contains(u, "?") {
		t.Fatalf("top-level url query not stripped: %q", u)
	}
	inner := entry["body"].(map[string]interface{})
	if u, _ := inner["documentURL"].(string); strings.Contains(u, "tok") || strings.Contains(u, "?") {
		t.Fatalf("nested documentURL query not stripped: %q", u)
	}
}

func TestSummarizeReport_EmptyBody(t *testing.T) {
	fields := summarizeReport("application/reports+json", nil)
	if fields["report"] != "empty" {
		t.Fatalf("empty body: report = %v, want empty", fields["report"])
	}
}

func TestNormalizeReportContentType(t *testing.T) {
	if got := normalizeReportContentType("application/reports+json; charset=utf-8"); got != "application/reports+json" {
		t.Fatalf("got %q, want application/reports+json", got)
	}
}
