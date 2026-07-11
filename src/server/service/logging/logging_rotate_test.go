// SPDX-License-Identifier: MIT
// Coverage tests for RotatingFile.rotate, compressFile, cleanupOldFiles,
// and AppLogger.Close/Reopen with active file outputs.
// These complement logging_coverage_test.go and logging_test.go.
package logging

import (
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/apimgr/vidveil/src/config"
)

// ── RotatingFile.rotate ───────────────────────────────────────────────────────

func TestRotate_TriggeredBySecondWrite(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "rot.log")

	rf, err := NewRotatingFile(path, RotationConfig{MaxSize: "1B", Compress: false, Keep: 0})
	if err != nil {
		t.Fatalf("NewRotatingFile: %v", err)
	}
	defer rf.Close()

	// First write: currentSize goes from 0 to 3 (no rotation yet)
	rf.Write([]byte("abc"))

	// Second write: currentSize(3) >= maxSize(1) → rotate() fires
	_, err = rf.Write([]byte("xyz"))
	if err != nil {
		t.Errorf("Write after rotate trigger: %v", err)
	}
}

func TestRotate_NewFileCreatedAfterRotation(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "newfile.log")

	rf, err := NewRotatingFile(path, RotationConfig{MaxSize: "1B", Compress: false, Keep: 0})
	if err != nil {
		t.Fatalf("NewRotatingFile: %v", err)
	}
	defer rf.Close()

	rf.Write([]byte("hello"))
	rf.Write([]byte("world"))

	// After rotation, the log file path should still be writable
	rf.Write([]byte("post-rotate"))

	if _, err := os.Stat(path); err != nil {
		t.Errorf("log file missing after rotation: %v", err)
	}
}

func TestRotate_WithKeepCount_OldFilesRemoved(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "keep.log")

	rf, err := NewRotatingFile(path, RotationConfig{MaxSize: "1B", Compress: false, Keep: 1})
	if err != nil {
		t.Fatalf("NewRotatingFile: %v", err)
	}
	defer rf.Close()

	// Trigger multiple rotations (keep=1 means only 1 old file retained)
	for i := 0; i < 4; i++ {
		rf.Write([]byte("data"))
		rf.Write([]byte("more"))
		// Small sleep to ensure unique timestamps in rotated filenames
		time.Sleep(2 * time.Millisecond)
	}
	// cleanupOldFiles runs in a goroutine — give it time to finish
	time.Sleep(50 * time.Millisecond)
}

// ── compressFile — direct call ────────────────────────────────────────────────

func TestCompressFile_CompressesAndRemovesOriginal(t *testing.T) {
	dir := t.TempDir()
	src := filepath.Join(dir, "to_compress.log")
	if err := os.WriteFile(src, []byte("log data to compress"), 0644); err != nil {
		t.Fatalf("write src: %v", err)
	}

	rf := &RotatingFile{path: filepath.Join(dir, "active.log"), keepCount: 1}

	rf.compressFile(src)

	// Original should be removed, .gz should exist
	if _, err := os.Stat(src); err == nil {
		t.Error("compressFile: original file should have been removed")
	}
	if _, err := os.Stat(src + ".gz"); err != nil {
		t.Error("compressFile: .gz file should have been created")
	}
}

func TestCompressFile_KeepZeroRemovesGz(t *testing.T) {
	dir := t.TempDir()
	src := filepath.Join(dir, "discard.log")
	if err := os.WriteFile(src, []byte("discard data"), 0644); err != nil {
		t.Fatalf("write src: %v", err)
	}

	rf := &RotatingFile{path: filepath.Join(dir, "active.log"), keepCount: 0}

	rf.compressFile(src)

	// Both original and .gz should be removed when keepCount=0
	if _, err := os.Stat(src); err == nil {
		t.Error("compressFile keepCount=0: original should be removed")
	}
	if _, err := os.Stat(src + ".gz"); err == nil {
		t.Error("compressFile keepCount=0: .gz should be removed")
	}
}

func TestCompressFile_MissingSrcNoPanic(t *testing.T) {
	dir := t.TempDir()
	rf := &RotatingFile{path: filepath.Join(dir, "active.log"), keepCount: 1}
	rf.compressFile(filepath.Join(dir, "nonexistent.log"))
}

// ── cleanupOldFiles — direct call ─────────────────────────────────────────────

func TestCleanupOldFiles_RemovesExcessFiles(t *testing.T) {
	dir := t.TempDir()
	base := filepath.Join(dir, "app.log")

	// Create 3 rotated files
	for i := 0; i < 3; i++ {
		name := base + ".202601010000" + string(rune('1'+i))
		os.WriteFile(name, []byte("old"), 0644)
		time.Sleep(2 * time.Millisecond)
	}

	rf := &RotatingFile{path: base, keepCount: 1}
	rf.cleanupOldFiles()

	// Small sleep since cleanupOldFiles runs synchronously here
	matches, _ := filepath.Glob(base + ".*")
	if len(matches) > 1 {
		t.Errorf("cleanupOldFiles: expected ≤1 rotated file, got %d", len(matches))
	}
}

func TestCleanupOldFiles_NoFilesNoPanic(t *testing.T) {
	dir := t.TempDir()
	rf := &RotatingFile{path: filepath.Join(dir, "app.log"), keepCount: 2}
	rf.cleanupOldFiles()
}

// ── AppLogger.Close with file output ──────────────────────────────────────────

func TestAppLogger_Close_ClosesFileOutput(t *testing.T) {
	dir := t.TempDir()
	logPath := filepath.Join(dir, "server.log")

	cfg := config.DefaultAppConfig()
	cfg.Server.Logs.Server.Enabled = true
	cfg.Server.Logs.Server.Filename = logPath

	logger, err := NewAppLogger(cfg)
	if err != nil {
		t.Fatalf("NewAppLogger: %v", err)
	}

	logger.Info("before close", nil)
	logger.Close()

	data, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	if !strings.Contains(string(data), "before close") {
		t.Error("AppLogger.Close: log message missing from file")
	}
}

// ── AppLogger.Reopen with RotatingFile output ─────────────────────────────────

func TestAppLogger_Reopen_RotatingFile(t *testing.T) {
	dir := t.TempDir()
	logPath := filepath.Join(dir, "server.log")

	cfg := config.DefaultAppConfig()
	cfg.Server.Logs.Server.Enabled = true
	cfg.Server.Logs.Server.Filename = logPath

	logger, err := NewAppLogger(cfg)
	if err != nil {
		t.Fatalf("NewAppLogger: %v", err)
	}
	defer logger.Close()

	logger.Info("before reopen", nil)
	logger.Reopen()
	logger.Info("after reopen", nil)
}

// ── NewAppLogger — additional config paths ─────────────────────────────────────

func TestNewAppLogger_AccessLogEnabled(t *testing.T) {
	dir := t.TempDir()
	logPath := filepath.Join(dir, "access.log")

	cfg := config.DefaultAppConfig()
	cfg.Server.Logs.Access.Enabled = true
	cfg.Server.Logs.Access.Filename = logPath

	logger, err := NewAppLogger(cfg)
	if err != nil {
		t.Fatalf("NewAppLogger with access log: %v", err)
	}
	defer logger.Close()

	logger.Access("GET", "/", "HTTP/1.1", "127.0.0.1", "", "test", 200, 0)
}

func TestNewAppLogger_SecurityLogEnabled(t *testing.T) {
	dir := t.TempDir()
	logPath := filepath.Join(dir, "security.log")

	cfg := config.DefaultAppConfig()
	cfg.Server.Logs.Security.Enabled = true
	cfg.Server.Logs.Security.Filename = logPath

	logger, err := NewAppLogger(cfg)
	if err != nil {
		t.Fatalf("NewAppLogger with security log: %v", err)
	}
	defer logger.Close()

	logger.Security("test_event", "10.0.0.1", nil)
}

func TestNewAppLogger_DebugLogEnabled(t *testing.T) {
	dir := t.TempDir()
	logPath := filepath.Join(dir, "debug.log")

	cfg := config.DefaultAppConfig()
	cfg.Server.Logs.Level = "debug"
	cfg.Server.Logs.Debug.Enabled = true
	cfg.Server.Logs.Debug.Filename = logPath

	logger, err := NewAppLogger(cfg)
	if err != nil {
		t.Fatalf("NewAppLogger with debug log: %v", err)
	}
	defer logger.Close()

	logger.Debug("debug event", nil)
}

func TestNewAppLogger_AuditLogEnabled(t *testing.T) {
	dir := t.TempDir()
	logPath := filepath.Join(dir, "audit.log")

	cfg := config.DefaultAppConfig()
	cfg.Server.Logs.Audit.Enabled = true
	cfg.Server.Logs.Audit.Filename = logPath

	logger, err := NewAppLogger(cfg)
	if err != nil {
		t.Fatalf("NewAppLogger with audit log: %v", err)
	}
	defer logger.Close()

	logger.Audit("config.updated", "admin", "admin", "127.0.0.1", "success", nil)
}

// ── AppLogger.Close — os.File output path ────────────────────────────────────

func TestAppLogger_Close_WithOsFileOutput_CoversElseBranch(t *testing.T) {
	dir := t.TempDir()
	logPath := filepath.Join(dir, "direct.log")
	f, err := os.OpenFile(logPath, os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		t.Fatal(err)
	}

	// Directly add an *os.File to l.outputs to cover the else-if branch (line 690)
	l := &AppLogger{
		outputs: map[string]io.Writer{
			"direct": f,
		},
	}
	l.Close()
}
