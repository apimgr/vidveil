// SPDX-License-Identifier: MIT
// AI.md PART 11: Reporting API (Modern + Legacy) — browser-emitted report
// ingestion endpoints. CSP violations, Network Error Logging (NEL), and the
// generic Reporting-API batch all POST here. These back the Reporting-Endpoints,
// Report-To, and NEL response headers set in the security-headers middleware.
package handler

import (
	"encoding/json"
	"io"
	"net/http"
	"net/url"
	"strings"
)

// maxReportBodyBytes bounds each report POST so a hostile client cannot flood
// memory with an oversized body. Browser reports are small (a few KB); 64 KiB
// leaves generous headroom while capping abuse.
const maxReportBodyBytes = 64 * 1024

// ReportsDefault handles POST /api/v1/server/reports/default — the Reporting API
// (modern Reporting-Endpoints + legacy Report-To) batch endpoint. Browsers post
// an array of reports (CSP, deprecation, intervention, crash) as
// application/reports+json.
func (h *ServerHandler) ReportsDefault(w http.ResponseWriter, r *http.Request) {
	h.ingestReport(w, r, "security.report")
}

// ReportsNEL handles POST /api/v1/server/reports/nel — Network Error Logging.
// Browsers post application/reports+json describing TLS/DNS/TCP/HTTP failures.
func (h *ServerHandler) ReportsNEL(w http.ResponseWriter, r *http.Request) {
	h.ingestReport(w, r, "security.nel_report")
}

// ReportsCSP handles POST /api/v1/server/reports/csp — Content-Security-Policy
// violation reports. Accepts both the legacy application/csp-report shape and
// the modern application/reports+json shape.
func (h *ServerHandler) ReportsCSP(w http.ResponseWriter, r *http.Request) {
	h.ingestReport(w, r, "security.csp_violation")
}

// ingestReport reads a bounded browser report body, records a sanitized summary
// to security.log, and always answers 204 No Content. Per AI.md PART 11 the
// response body NEVER echoes user-controlled fields (Tier 2 visibility), the
// report is rate-limited per-IP by the global rate-limit middleware, and URL
// fields pass through the Output Sanitization Pipeline (query strings stripped)
// before they are logged.
func (h *ServerHandler) ingestReport(w http.ResponseWriter, r *http.Request, event string) {
	// Bound the body regardless of the declared Content-Length so a lying or
	// chunked request cannot exhaust memory.
	body, _ := io.ReadAll(io.LimitReader(http.MaxBytesReader(w, r.Body, maxReportBodyBytes), maxReportBodyBytes))
	_ = r.Body.Close()

	if h.logger != nil {
		fields := summarizeReport(r.Header.Get("Content-Type"), body)
		fields["content_type"] = normalizeReportContentType(r.Header.Get("Content-Type"))
		h.logger.Security(event, getClientIP(r), fields)
	}

	// Keep the browser happy and echo nothing back.
	w.WriteHeader(http.StatusNoContent)
}

// normalizeReportContentType trims parameters (charset, boundary) so the logged
// content type is a stable, low-cardinality tag.
func normalizeReportContentType(ct string) string {
	if i := strings.IndexByte(ct, ';'); i >= 0 {
		ct = ct[:i]
	}
	return strings.TrimSpace(strings.ToLower(ct))
}

// summarizeReport parses a browser report body best-effort and returns a small,
// sanitized field map suitable for security.log. Parsing never fails the
// request: an unparseable body yields a "malformed" marker plus the byte size.
func summarizeReport(contentType string, body []byte) map[string]interface{} {
	fields := map[string]interface{}{
		"bytes": len(body),
	}
	if len(body) == 0 {
		fields["report"] = "empty"
		return fields
	}

	var parsed interface{}
	if err := json.Unmarshal(body, &parsed); err != nil {
		// Do not log raw untrusted bytes back; only note that it was malformed.
		fields["report"] = "malformed"
		return fields
	}

	sanitizeReportURLs(parsed)

	// Legacy CSP reports arrive as {"csp-report": {...}} — surface the object so
	// the violated directive and (sanitized) document URI show up in the log.
	if obj, ok := parsed.(map[string]interface{}); ok {
		if csp, ok := obj["csp-report"]; ok {
			fields["csp_report"] = csp
			return fields
		}
	}

	fields["report"] = parsed
	return fields
}

// sanitizeReportURLs walks a decoded JSON report in place and strips the query
// string from any string value stored under a key that names a URL. Report
// bodies can carry request URLs that leak session tokens or search terms in
// their query params (AI.md PART 11 → Output Sanitization Pipeline); dropping
// the query keeps the useful path/host while discarding the sensitive tail.
func sanitizeReportURLs(v interface{}) {
	switch node := v.(type) {
	case map[string]interface{}:
		for k, child := range node {
			if s, ok := child.(string); ok && isURLKey(k) {
				node[k] = stripURLQuery(s)
				continue
			}
			sanitizeReportURLs(child)
		}
	case []interface{}:
		for _, child := range node {
			sanitizeReportURLs(child)
		}
	}
}

// isURLKey reports whether a JSON key names a URL-bearing field. Covers the
// hyphen/camel variants used across the CSP-report and Reporting-API shapes
// (document-uri, blocked-uri, documentURL, blockedURL, referrer, source_file…).
func isURLKey(key string) bool {
	k := strings.ToLower(key)
	return strings.Contains(k, "uri") ||
		strings.Contains(k, "url") ||
		k == "referrer" ||
		k == "source_file" ||
		k == "sourcefile"
}

// stripURLQuery removes the query and userinfo from a URL string, preserving
// scheme/host/path. Non-URL strings are returned unchanged.
func stripURLQuery(raw string) string {
	parsed, err := url.Parse(raw)
	if err != nil || parsed.Scheme == "" && parsed.Host == "" && !strings.Contains(raw, "/") {
		return raw
	}
	parsed.RawQuery = ""
	parsed.Fragment = ""
	parsed.User = nil
	return parsed.String()
}
