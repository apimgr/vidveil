// SPDX-License-Identifier: MIT
// AI.md PART 11: Security Reports — unit tests for the security_reports store.
package secreport

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"fmt"
	"github.com/apimgr/vidveil/src/common/dbstore"
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
	t.Cleanup(func() {
		if err := os.RemoveAll(tmpDir); err != nil {
			t.Errorf("remove temporary database directory: %v", err)
		}
	})
	dbPath := filepath.Join(tmpDir, "test.db")

	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	t.Cleanup(func() {
		if err := db.Close(); err != nil {
			t.Errorf("close test database: %v", err)
		}
	})

	// Schema matches security_reports DDL in
	// src/server/service/database/migrations.go.
	_, err = db.Exec(`CREATE TABLE IF NOT EXISTS security_reports (
		tracking_id VARCHAR(32) PRIMARY KEY,
		id_window BIGINT NOT NULL,
		severity VARCHAR(16) NOT NULL,
		component VARCHAR(255) NOT NULL,
		status VARCHAR(16) NOT NULL DEFAULT 'received',
		body_enc BLOB NOT NULL,
		body_alg VARCHAR(16) NOT NULL,
		contact_enc BLOB,
		credit_pref VARCHAR(8) NOT NULL DEFAULT 'no',
		credit_name VARCHAR(100),
		researcher_comment TEXT NOT NULL DEFAULT '',
		disclosure_date DATE,
		cve_requested BOOLEAN NOT NULL DEFAULT FALSE,
		token_hash VARCHAR(64) NOT NULL,
		token_last_used DATE,
		created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
		updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
		closed_at TIMESTAMP
	);
	CREATE TABLE security_thanks (tracking_id VARCHAR(32) PRIMARY KEY, credit VARCHAR(120) NOT NULL, severity VARCHAR(16) NOT NULL, disclosed_on DATE NOT NULL)`)
	if err != nil {
		t.Fatalf("create table: %v", err)
	}
	if _, err := db.Exec(`CREATE TABLE app_secrets (key TEXT PRIMARY KEY, value TEXT NOT NULL, created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP)`); err != nil {
		t.Fatalf("create app secrets table: %v", err)
	}
	return db
}

type rawReportTestStore struct{ db *sql.DB }

type reportTestReader struct{ db *sql.DB }
type reportTestRow struct{ row *sql.Row }

func (r reportTestReader) QueryContext(ctx context.Context, query string, args ...any) (*sql.Rows, error) {
	return r.db.QueryContext(ctx, query, args...)
}
func (r reportTestReader) QueryRowContext(ctx context.Context, query string, args ...any) dbstore.RowScanner {
	return reportTestRow{row: r.db.QueryRowContext(ctx, query, args...)}
}
func (r reportTestReader) PingContext(ctx context.Context) error { return r.db.PingContext(ctx) }
func (r reportTestRow) Scan(dest ...any) error                   { return r.row.Scan(dest...) }

func (s rawReportTestStore) Read(ctx context.Context, fn func(context.Context, dbstore.Reader) error) error {
	return fn(ctx, reportTestReader(s))
}

func (s rawReportTestStore) Write(ctx context.Context, fn func(*sql.Tx) error) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	if err := fn(tx); err != nil {
		_ = tx.Rollback()
		return err
	}
	return tx.Commit()
}

func reportTestStore(db *sql.DB) Store { return rawReportTestStore{db: db} }

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
	idWindow := windowIndex(time.Now())

	sub, err := CreateReport(ctx, reportTestStore(db), configDir, keyHex, Input{
		Severity:         "high",
		IDWindow:         idWindow,
		Component:        "auth",
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

	status, err := GetReportStatus(ctx, reportTestStore(db), sub.TrackingID, sub.ReportToken)
	if err != nil {
		t.Fatalf("GetReportStatus: %v", err)
	}
	if status.Status != StatusReceived {
		t.Fatalf("GetReportStatus: want status %q, got %q", StatusReceived, status.Status)
	}
	if status.TrackingID != sub.TrackingID {
		t.Fatalf("GetReportStatus: want tracking id %q, got %q", sub.TrackingID, status.TrackingID)
	}
	if _, err := GetReportStatus(ctx, reportTestStore(db), sub.TrackingID, sub.ReportToken); err == nil {
		t.Fatal("GetReportStatus: expected same-day token reuse to be rejected")
	}
}

func TestGetReportStatus_WrongTokenRejected(t *testing.T) {
	db := setupTestDB(t)
	ctx := context.Background()
	configDir := t.TempDir()
	keyHex := testEncryptionKeyHex(t)

	sub, err := CreateReport(ctx, reportTestStore(db), configDir, keyHex, Input{
		Severity:         "medium",
		IDWindow:         windowIndex(time.Now()),
		Component:        "billing",
		Body:             []byte("body"),
		CreditPreference: "none",
	})
	if err != nil {
		t.Fatalf("CreateReport: %v", err)
	}

	if _, err := GetReportStatus(ctx, reportTestStore(db), sub.TrackingID, "wrong-token"); err == nil {
		t.Fatalf("GetReportStatus: expected error for wrong token")
	}
}

func TestGetReportStatus_UnknownTrackingIDRejected(t *testing.T) {
	db := setupTestDB(t)
	ctx := context.Background()

	if _, err := GetReportStatus(ctx, reportTestStore(db), "sec_doesnotexist", "any-token"); err == nil {
		t.Fatalf("GetReportStatus: expected error for unknown tracking id")
	}
}

func TestGetReportStatus_ExpiredTokenRejected(t *testing.T) {
	db := setupTestDB(t)
	ctx := context.Background()
	configDir := t.TempDir()
	keyHex := testEncryptionKeyHex(t)

	sub, err := CreateReport(ctx, reportTestStore(db), configDir, keyHex, Input{
		Severity:         "low",
		IDWindow:         windowIndex(time.Now()),
		Component:        "docs",
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

	if _, err := GetReportStatus(ctx, reportTestStore(db), sub.TrackingID, sub.ReportToken); err == nil {
		t.Fatalf("GetReportStatus: expected error for expired token")
	}
}

func TestDisclosureCreatesPersistentThanksAndForwardOnlyLifecycle(t *testing.T) {
	db := setupTestDB(t)
	ctx := context.Background()
	now := time.Date(2026, 11, 24, 3, 0, 0, 0, time.UTC)
	sub, err := CreateReport(ctx, reportTestStore(db), t.TempDir(), testEncryptionKeyHex(t), Input{
		IDWindow: windowIndex(now), Severity: "high", Component: "parser",
		Body: []byte("private vulnerability details"), CreditPreference: "anon",
	})
	if err != nil {
		t.Fatalf("CreateReport: %v", err)
	}
	for _, status := range []string{StatusTriaged, StatusConfirmed, StatusPatching, StatusDisclosed} {
		if _, err := UpdateReport(ctx, reportTestStore(db), sub.TrackingID, UpdateInput{Status: &status}, now); err != nil {
			t.Fatalf("advance report to %s: %v", status, err)
		}
	}
	if _, err := UpdateReport(ctx, reportTestStore(db), sub.TrackingID, UpdateInput{Status: ptrStatus(StatusPatching)}, now); err == nil {
		t.Fatal("expected disclosed report to reject backward transition")
	}
	entries, err := ListThanks(ctx, reportTestStore(db))
	if err != nil || len(entries) != 1 {
		t.Fatalf("ListThanks = %#v, %v; want one entry", entries, err)
	}
	if entries[0].Credit != "Anonymous Researcher #1" || entries[0].Severity != "high" || entries[0].DisclosedOn != "2026-11-24" {
		t.Fatalf("unexpected thanks entry: %#v", entries[0])
	}
	if _, err := CleanupClosedReports(ctx, reportTestStore(db), 365, now.AddDate(0, 0, 366)); err != nil {
		t.Fatalf("CleanupClosedReports: %v", err)
	}
	entries, err = ListThanks(ctx, reportTestStore(db))
	if err != nil || len(entries) != 1 {
		t.Fatalf("thanks entry did not survive report retention: %#v, %v", entries, err)
	}
}

func ptrStatus(value string) *string { return &value }

func TestCleanupClosedReportsDeletesInBoundedBatches(t *testing.T) {
	db := setupTestDB(t)
	ctx := context.Background()
	now := time.Date(2026, 11, 27, 0, 0, 0, 0, time.UTC)
	const reportCount = 1001
	for i := 0; i < reportCount; i++ {
		_, err := db.ExecContext(ctx, `INSERT INTO security_reports
			(tracking_id, id_window, severity, component, body_enc, body_alg, token_hash, created_at, updated_at, closed_at)
			VALUES (?, 1, 'low', 'test', X'00', 'aes-256-gcm', ?, ?, ?, ?)`,
			fmt.Sprintf("report-%04d", i), fmt.Sprintf("hash-%04d", i), now.AddDate(0, 0, -400), now.AddDate(0, 0, -400), now.AddDate(0, 0, -400))
		if err != nil {
			t.Fatalf("insert report %d: %v", i, err)
		}
	}
	deleted, err := CleanupClosedReports(ctx, reportTestStore(db), 365, now)
	if err != nil || deleted != reportCount {
		t.Fatalf("CleanupClosedReports() = %d, %v; want %d reports deleted", deleted, err, reportCount)
	}
}
