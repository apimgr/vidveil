// SPDX-License-Identifier: MIT
// AI.md PART 11: Security Reports — unit tests for the security_reports store.
package secreport

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"os"
	"path/filepath"
	"testing"
	"time"

	_ "modernc.org/sqlite"
)

func setupTestDB(t *testing.T) *sql.DB {
	t.Helper()

	tmpDir := filepath.Join(os.TempDir(), "apimgr", "vidveil-test-"+t.Name())
	if err := os.MkdirAll(tmpDir, 0755); err != nil {
		t.Fatalf("mkdir tempdir: %v", err)
	}
	t.Cleanup(func() { os.RemoveAll(tmpDir) })
	dbPath := filepath.Join(tmpDir, "test.db")

	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	t.Cleanup(func() { db.Close() })

	// Schema matches security_reports DDL in
	// src/server/service/database/migrations.go.
	_, err = db.Exec(`CREATE TABLE IF NOT EXISTS security_reports (
		tracking_id TEXT PRIMARY KEY,
		status TEXT NOT NULL DEFAULT 'received',
		severity TEXT NOT NULL,
		component TEXT NOT NULL,
		endpoint TEXT,
		summary TEXT NOT NULL,
		encrypted_body BLOB NOT NULL,
		encryption_method TEXT NOT NULL,
		researcher_email TEXT,
		researcher_gpg_fingerprint TEXT,
		cve_requested INTEGER DEFAULT 0,
		disclosure_window_days INTEGER,
		credit_preference TEXT NOT NULL,
		credit_name TEXT,
		disclosed INTEGER DEFAULT 0,
		app_version TEXT,
		commit_hash TEXT,
		report_token_hash TEXT,
		report_token_last_used_date TEXT,
		maintainer_comments TEXT,
		expected_disclosure_date DATETIME,
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
		updated_at DATETIME DEFAULT CURRENT_TIMESTAMP,
		closed_at DATETIME
	)`)
	if err != nil {
		t.Fatalf("create table: %v", err)
	}
	return db
}

func testEncryptionKeyHex(t *testing.T) string {
	t.Helper()
	key := make([]byte, 32)
	if _, err := rand.Read(key); err != nil {
		t.Fatalf("generate key: %v", err)
	}
	return hex.EncodeToString(key)
}

func TestCreateReport_And_GetReportStatus(t *testing.T) {
	db := setupTestDB(t)
	ctx := context.Background()
	configDir := t.TempDir() // no pgp keypair present -> AES-256-GCM fallback
	keyHex := testEncryptionKeyHex(t)

	sub, err := CreateReport(ctx, db, configDir, keyHex, Input{
		Severity:         "high",
		Component:        "auth",
		Endpoint:         "/api/v1/login",
		Summary:          "auth bypass",
		Body:             []byte("steps to reproduce...\nimpact...\n"),
		ResearcherEmail:  "researcher@example.com",
		CreditPreference: "anonymous",
	})
	if err != nil {
		t.Fatalf("CreateReport: %v", err)
	}
	if sub.TrackingID == "" || len(sub.TrackingID) < 5 || sub.TrackingID[:4] != "sec_" {
		t.Fatalf("CreateReport: unexpected tracking id %q", sub.TrackingID)
	}
	if sub.ReportToken == "" {
		t.Fatalf("CreateReport: expected a non-empty report token")
	}

	status, err := GetReportStatus(ctx, db, sub.TrackingID, sub.ReportToken)
	if err != nil {
		t.Fatalf("GetReportStatus: %v", err)
	}
	if status.Status != StatusReceived {
		t.Fatalf("GetReportStatus: want status %q, got %q", StatusReceived, status.Status)
	}
	if status.TrackingID != sub.TrackingID {
		t.Fatalf("GetReportStatus: want tracking id %q, got %q", sub.TrackingID, status.TrackingID)
	}
}

func TestGetReportStatus_WrongTokenRejected(t *testing.T) {
	db := setupTestDB(t)
	ctx := context.Background()
	configDir := t.TempDir()
	keyHex := testEncryptionKeyHex(t)

	sub, err := CreateReport(ctx, db, configDir, keyHex, Input{
		Severity:         "medium",
		Component:        "billing",
		Summary:          "off-by-one",
		Body:             []byte("body"),
		CreditPreference: "none",
	})
	if err != nil {
		t.Fatalf("CreateReport: %v", err)
	}

	if _, err := GetReportStatus(ctx, db, sub.TrackingID, "wrong-token"); err == nil {
		t.Fatalf("GetReportStatus: expected error for wrong token")
	}
}

func TestGetReportStatus_UnknownTrackingIDRejected(t *testing.T) {
	db := setupTestDB(t)
	ctx := context.Background()

	if _, err := GetReportStatus(ctx, db, "sec_doesnotexist", "any-token"); err == nil {
		t.Fatalf("GetReportStatus: expected error for unknown tracking id")
	}
}

func TestGetReportStatus_ExpiredTokenRejected(t *testing.T) {
	db := setupTestDB(t)
	ctx := context.Background()
	configDir := t.TempDir()
	keyHex := testEncryptionKeyHex(t)

	sub, err := CreateReport(ctx, db, configDir, keyHex, Input{
		Severity:         "low",
		Component:        "docs",
		Summary:          "typo",
		Body:             []byte("body"),
		CreditPreference: "none",
	})
	if err != nil {
		t.Fatalf("CreateReport: %v", err)
	}

	// Close the report more than the 30-day grace period in the past so the
	// one-shot token has expired per AI.md PART 11.
	closedAt := time.Now().Add(-31 * 24 * time.Hour)
	if _, err := db.ExecContext(ctx,
		`UPDATE security_reports SET closed_at = ? WHERE tracking_id = ?`,
		closedAt, sub.TrackingID); err != nil {
		t.Fatalf("set closed_at: %v", err)
	}

	if _, err := GetReportStatus(ctx, db, sub.TrackingID, sub.ReportToken); err == nil {
		t.Fatalf("GetReportStatus: expected error for expired token")
	}
}
