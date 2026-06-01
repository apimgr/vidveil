// SPDX-License-Identifier: MIT
package logging

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/apimgr/vidveil/src/config"
)

// --- generateAuditID ---

func TestGenerateAuditID(t *testing.T) {
	id := generateAuditID()
	if id == "" {
		t.Fatal("generateAuditID() returned empty string")
	}
	if !strings.HasPrefix(id, "audit_") {
		t.Errorf("generateAuditID() = %q, want prefix %q", id, "audit_")
	}
}

// generateAuditID must return unique values across calls
func TestGenerateAuditIDUnique(t *testing.T) {
	a := generateAuditID()
	b := generateAuditID()
	if a == b {
		t.Errorf("generateAuditID() returned duplicate value %q", a)
	}
}

// --- AppLogger methods with in-memory writer ---

// newInMemoryLogger builds an AppLogger that writes all outputs to the given buffer.
func newInMemoryLogger(level Level, buf io.Writer) *AppLogger {
	return &AppLogger{
		level: level,
		outputs: map[string]io.Writer{
			"server":   buf,
			"debug":    buf,
			"access":   buf,
			"security": buf,
		},
		appConfig: config.DefaultAppConfig(),
	}
}

func TestAppLoggerDebugWritesToOutput(t *testing.T) {
	var buf bytes.Buffer
	l := newInMemoryLogger(LevelDebug, &buf)

	l.Debug("debug message", nil)

	if buf.Len() == 0 {
		t.Error("Debug() produced no output")
	}
}

func TestAppLoggerInfoWritesToOutput(t *testing.T) {
	var buf bytes.Buffer
	l := newInMemoryLogger(LevelDebug, &buf)

	l.Info("info message", nil)

	if buf.Len() == 0 {
		t.Error("Info() produced no output")
	}
}

func TestAppLoggerWarnWritesToOutput(t *testing.T) {
	var buf bytes.Buffer
	l := newInMemoryLogger(LevelDebug, &buf)

	l.Warn("warn message", nil)

	if buf.Len() == 0 {
		t.Error("Warn() produced no output")
	}
}

func TestAppLoggerErrorWritesToOutput(t *testing.T) {
	var buf bytes.Buffer
	l := newInMemoryLogger(LevelDebug, &buf)

	l.Error("error message", nil)

	if buf.Len() == 0 {
		t.Error("Error() produced no output")
	}
}

// Debug() must be suppressed when the logger level is higher than LevelDebug
func TestAppLoggerLevelFilteringSuppressesDebug(t *testing.T) {
	var buf bytes.Buffer
	l := newInMemoryLogger(LevelInfo, &buf)

	l.Debug("should be filtered", nil)

	if buf.Len() != 0 {
		t.Errorf("Debug() wrote %d bytes at LevelInfo, expected 0", buf.Len())
	}
}

func TestAppLoggerAccessWritesToOutput(t *testing.T) {
	var buf bytes.Buffer
	l := newInMemoryLogger(LevelDebug, &buf)

	l.Access("GET", "/ping", "127.0.0.1:9000", "go-test/1.0", 200, 5*time.Millisecond)

	if buf.Len() == 0 {
		t.Error("Access() produced no output")
	}
	if !strings.Contains(buf.String(), "GET") {
		t.Errorf("Access() output does not contain method name: %s", buf.String())
	}
}

func TestAppLoggerSecurityWritesToOutput(t *testing.T) {
	var buf bytes.Buffer
	l := newInMemoryLogger(LevelDebug, &buf)

	l.Security("brute_force", "10.0.0.1", map[string]interface{}{"attempts": 5})

	if buf.Len() == 0 {
		t.Error("Security() produced no output")
	}
	if !strings.Contains(buf.String(), "brute_force") {
		t.Errorf("Security() output does not contain event name: %s", buf.String())
	}
}

// --- AppLogger.Audit without "audit" output ---

// Audit must not panic and must not write when no "audit" output is registered
func TestAppLoggerAuditNoOutput(t *testing.T) {
	var buf bytes.Buffer
	l := &AppLogger{
		level: LevelDebug,
		outputs: map[string]io.Writer{
			"server": &buf,
		},
		appConfig: config.DefaultAppConfig(),
	}

	l.Audit("admin.login", "alice", "admin", "10.0.0.1", "success", nil)

	if buf.Len() != 0 {
		t.Errorf("Audit() with no audit output wrote %d bytes to server output, want 0", buf.Len())
	}
}

// --- AppLogger.Audit with "audit" output ---

func TestAppLoggerAuditWithOutput(t *testing.T) {
	var buf bytes.Buffer
	l := &AppLogger{
		level: LevelDebug,
		outputs: map[string]io.Writer{
			"audit": &buf,
		},
		appConfig: config.DefaultAppConfig(),
	}

	l.Audit("admin.login", "bob", "admin", "192.168.1.1", "success", map[string]interface{}{"extra": "data"})

	if buf.Len() == 0 {
		t.Fatal("Audit() produced no output")
	}

	// Strip trailing newline before unmarshalling
	raw := bytes.TrimRight(buf.Bytes(), "\n")
	var entry AuditEntry
	if err := json.Unmarshal(raw, &entry); err != nil {
		t.Fatalf("Audit() output is not valid JSON: %v — got: %s", err, raw)
	}
	if !strings.HasPrefix(entry.ID, "audit_") {
		t.Errorf("AuditEntry.ID = %q, want prefix %q", entry.ID, "audit_")
	}
	if entry.Time == "" {
		t.Error("AuditEntry.Time is empty")
	}
	if entry.Event != "admin.login" {
		t.Errorf("AuditEntry.Event = %q, want %q", entry.Event, "admin.login")
	}
	if entry.Result != "success" {
		t.Errorf("AuditEntry.Result = %q, want %q", entry.Result, "success")
	}
}

// --- AppLogger.Reopen ---

// Reopen on a logger with no file outputs must not panic
func TestAppLoggerReopenNoFiles(t *testing.T) {
	cfg := config.DefaultAppConfig()
	l, err := NewAppLogger(cfg)
	if err != nil {
		t.Fatalf("NewAppLogger() error: %v", err)
	}
	defer l.Close()

	l.Reopen()
}

// --- RotatingFile.Reopen ---

func TestRotatingFileReopen(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "reopen.log")

	cfg := RotationConfig{
		MaxSize:  "10MB",
		Interval: "",
		Compress: false,
		Keep:     0,
	}

	rf, err := NewRotatingFile(path, cfg)
	if err != nil {
		t.Fatalf("NewRotatingFile() error: %v", err)
	}
	defer rf.Close()

	if err := rf.Reopen(); err != nil {
		t.Fatalf("Reopen() error: %v", err)
	}

	payload := []byte("after reopen\n")
	if _, err := rf.Write(payload); err != nil {
		t.Errorf("Write() after Reopen() error: %v", err)
	}

	if _, err := os.Stat(path); err != nil {
		t.Errorf("log file missing after Reopen(): %v", err)
	}
}

// --- NewAccessLogMiddleware + Handler ---

func TestAccessLogMiddlewareHandler(t *testing.T) {
	var buf bytes.Buffer
	l := newInMemoryLogger(LevelDebug, &buf)

	middleware := NewAccessLogMiddleware(l)

	handlerRan := false
	inner := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		handlerRan = true
		w.WriteHeader(http.StatusOK)
	})

	wrapped := middleware.Handler(inner)

	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	rec := httptest.NewRecorder()
	wrapped.ServeHTTP(rec, req)

	if !handlerRan {
		t.Error("Handler() did not call the underlying handler")
	}
	if rec.Code != http.StatusOK {
		t.Errorf("Handler() response code = %d, want %d", rec.Code, http.StatusOK)
	}
}

// --- responseWriter.WriteHeader ---

func TestResponseWriterWriteHeader(t *testing.T) {
	rec := httptest.NewRecorder()
	rw := &responseWriter{ResponseWriter: rec, status: http.StatusOK}

	rw.WriteHeader(http.StatusNotFound)

	if rw.status != http.StatusNotFound {
		t.Errorf("responseWriter.status = %d, want %d", rw.status, http.StatusNotFound)
	}
	if rec.Code != http.StatusNotFound {
		t.Errorf("underlying ResponseWriter code = %d, want %d", rec.Code, http.StatusNotFound)
	}
}

// --- responseWriter.Hijack ---

// Hijack must return a non-nil error when the underlying ResponseWriter does not
// implement http.Hijacker (httptest.ResponseRecorder does not).
func TestResponseWriterHijackUnsupported(t *testing.T) {
	rec := httptest.NewRecorder()
	rw := &responseWriter{ResponseWriter: rec, status: http.StatusOK}

	_, _, err := rw.Hijack()
	if err == nil {
		t.Error("Hijack() expected error for non-Hijacker underlying writer, got nil")
	}
}

// --- needsRotation for time-based intervals ---

// needsRotation returns false when maxSize is 0 and interval is RotationNone
func TestNeedsRotationNoneInterval(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "none.log")

	rf, err := NewRotatingFile(path, RotationConfig{MaxSize: "0B", Interval: ""})
	if err != nil {
		t.Fatalf("NewRotatingFile() error: %v", err)
	}
	defer rf.Close()

	rf.maxSize = 0
	rf.interval = RotationNone
	rf.currentSize = 0

	if rf.needsRotation() {
		t.Error("needsRotation() = true for RotationNone with maxSize 0, want false")
	}
}

func TestNeedsRotationHourly(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "hourly.log")

	rf, err := NewRotatingFile(path, RotationConfig{MaxSize: "50MB", Interval: "hourly"})
	if err != nil {
		t.Fatalf("NewRotatingFile() error: %v", err)
	}
	defer rf.Close()

	rf.interval = RotationHourly
	rf.lastRotation = time.Now().Add(-2 * time.Hour)
	rf.currentSize = 0
	rf.maxSize = 50 * 1024 * 1024

	if !rf.needsRotation() {
		t.Error("needsRotation() = false for Hourly interval 2 hours ago, want true")
	}
}

func TestNeedsRotationDaily(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "daily.log")

	rf, err := NewRotatingFile(path, RotationConfig{MaxSize: "50MB", Interval: "daily"})
	if err != nil {
		t.Fatalf("NewRotatingFile() error: %v", err)
	}
	defer rf.Close()

	rf.interval = RotationDaily
	rf.lastRotation = time.Now().Add(-48 * time.Hour)
	rf.currentSize = 0
	rf.maxSize = 50 * 1024 * 1024

	if !rf.needsRotation() {
		t.Error("needsRotation() = false for Daily interval 48 hours ago, want true")
	}
}

func TestNeedsRotationWeekly(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "weekly.log")

	rf, err := NewRotatingFile(path, RotationConfig{MaxSize: "50MB", Interval: "weekly"})
	if err != nil {
		t.Fatalf("NewRotatingFile() error: %v", err)
	}
	defer rf.Close()

	rf.interval = RotationWeekly
	rf.lastRotation = time.Now().Add(-8 * 24 * time.Hour)
	rf.currentSize = 0
	rf.maxSize = 50 * 1024 * 1024

	if !rf.needsRotation() {
		t.Error("needsRotation() = false for Weekly interval 8 days ago, want true")
	}
}

func TestNeedsRotationMonthly(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "monthly.log")

	rf, err := NewRotatingFile(path, RotationConfig{MaxSize: "50MB", Interval: "monthly"})
	if err != nil {
		t.Fatalf("NewRotatingFile() error: %v", err)
	}
	defer rf.Close()

	rf.interval = RotationMonthly
	rf.lastRotation = time.Now().Add(-32 * 24 * time.Hour)
	rf.currentSize = 0
	rf.maxSize = 50 * 1024 * 1024

	if !rf.needsRotation() {
		t.Error("needsRotation() = false for Monthly interval 32 days ago, want true")
	}
}
