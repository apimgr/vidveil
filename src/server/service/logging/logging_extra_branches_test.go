// SPDX-License-Identifier: MIT
// AI.md PART 28: Coverage tests for uncovered branches in logging.go.
package logging

import (
	"bytes"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/apimgr/vidveil/src/config"
)

// ── RotatingFile.Close with nil file ─────────────────────────────────────────

func TestRotatingFile_Close_NilFile(t *testing.T) {
	rf := &RotatingFile{}
	if err := rf.Close(); err != nil {
		t.Errorf("Close() with nil file: want nil, got %v", err)
	}
}

// ── MaskEmail: domain without a dot ──────────────────────────────────────────

func TestMaskEmail_DomainNoDot(t *testing.T) {
	result := MaskEmail("user@nodomain")
	if result != "u***@***" {
		t.Errorf("MaskEmail(user@nodomain) = %q, want %q", result, "u***@***")
	}
}

// ── MaskIP: IPv6 with fewer than 4 colon-separated parts ─────────────────────

func TestMaskIP_IPv6_ShortParts(t *testing.T) {
	ip := "a:b:c"
	result := MaskIP(ip)
	if result == ip {
		t.Errorf("MaskIP(%q): expected masked result, got unchanged input", ip)
	}
}

// ── SanitizeLogFields: non-string email and ip values ────────────────────────

func TestSanitizeLogFields_NonStringEmail(t *testing.T) {
	result := SanitizeLogFields(map[string]interface{}{
		"email": 42,
	})
	if result["email"] != "***" {
		t.Errorf("SanitizeLogFields non-string email: got %v, want \"***\"", result["email"])
	}
}

func TestSanitizeLogFields_NonStringIP(t *testing.T) {
	result := SanitizeLogFields(map[string]interface{}{
		"ip": true,
	})
	if result["ip"] != "***" {
		t.Errorf("SanitizeLogFields non-string ip: got %v, want \"***\"", result["ip"])
	}
}

// ── auditSeverity: login_failed failure branch ────────────────────────────────

func TestAuditSeverity_LoginFailed_Failure(t *testing.T) {
	sev := auditSeverity("login_failed", "failure")
	if sev != "warn" {
		t.Errorf("auditSeverity(login_failed, failure) = %q, want %q", sev, "warn")
	}
}

func TestAuditSeverity_CSRF_Failure(t *testing.T) {
	sev := auditSeverity("csrf_rejected", "failure")
	if sev != "warn" {
		t.Errorf("auditSeverity(csrf_rejected, failure) = %q, want %q", sev, "warn")
	}
}

// ── responseWriter.Hijack: underlying writer supports Hijack ─────────────────

// hijackableRecorder wraps httptest.ResponseRecorder and implements the custom Hijack signature.
type hijackableRecorder struct {
	*httptest.ResponseRecorder
}

func (h *hijackableRecorder) Hijack() (interface{}, interface{}, error) {
	return nil, nil, nil
}

func TestResponseWriterHijack_Supported(t *testing.T) {
	base := &hijackableRecorder{ResponseRecorder: httptest.NewRecorder()}
	rw := &responseWriter{ResponseWriter: base, status: http.StatusOK}
	_, _, err := rw.Hijack()
	if err != nil {
		t.Errorf("Hijack() on hijackable writer: got error %v, want nil", err)
	}
}

// ── AppLogger.log: json.Marshal failure branch ────────────────────────────────

func TestAppLogger_Log_UnmarshalableFields(t *testing.T) {
	var buf bytes.Buffer
	l := &AppLogger{
		level:     LevelDebug,
		outputs:   map[string]io.Writer{"debug": &buf},
		appConfig: config.DefaultAppConfig(),
	}
	// A channel value cannot be JSON-marshaled; this triggers the err != nil branch in log().
	l.log(LevelDebug, "debug", "test", map[string]interface{}{
		"ch": make(chan int),
	})
	// The log call silently returns; nothing is written to the buffer.
	if buf.Len() != 0 {
		t.Errorf("log() with unmarshalable fields: expected 0 bytes written, got %d", buf.Len())
	}
}
