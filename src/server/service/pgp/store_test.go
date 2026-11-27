package pgp

import (
	"context"
	"database/sql"
	"github.com/apimgr/vidveil/src/common/dbstore"
	"path/filepath"
	"testing"
	"time"

	"github.com/apimgr/vidveil/src/server/service/database"
)

// newTestDB opens an in-memory SQLite database with the pgp_keypair schema.
func newTestDB(t *testing.T) *database.AppDatabase {
	t.Helper()
	db, err := database.NewAppDatabase(database.DatabaseConfig{Driver: database.DriverSQLite, Path: filepath.Join(t.TempDir(), "test.db")})
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	_, err = pgpTestExec(db, `CREATE TABLE pgp_keypair (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		fingerprint TEXT NOT NULL,
		created_at DATETIME NOT NULL,
		expires_at DATETIME NOT NULL,
		last_rotated_at DATETIME,
		keyservers_published TEXT DEFAULT '[]',
		revoked INTEGER DEFAULT 0
	)`)
	if err != nil {
		t.Fatalf("create table: %v", err)
	}
	return db
}

func pgpTestExec(db *database.AppDatabase, query string, args ...any) (sql.Result, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	var result sql.Result
	err := db.Write(ctx, func(tx *sql.Tx) error {
		var err error
		result, err = tx.ExecContext(ctx, query, args...)
		return err
	})
	return result, err
}

type pgpTestRow struct {
	db    *database.AppDatabase
	query string
	args  []any
}

func (r pgpTestRow) Scan(dest ...any) error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	return r.db.Read(ctx, func(readCtx context.Context, readDB dbstore.Reader) error {
		return readDB.QueryRowContext(readCtx, r.query, r.args...).Scan(dest...)
	})
}

func pgpTestQueryRow(db *database.AppDatabase, query string, args ...any) pgpTestRow {
	return pgpTestRow{db: db, query: query, args: args}
}

// TestSaveAndGetKeypairMeta verifies metadata persists and reads back.
func TestSaveAndGetKeypairMeta(t *testing.T) {
	db := newTestDB(t)

	if meta, err := GetKeypairMeta(db); err != nil || meta != nil {
		t.Fatalf("expected (nil,nil) on empty table, got (%v,%v)", meta, err)
	}

	created := time.Now().UTC().Truncate(time.Second)
	kp := &Keypair{
		Fingerprint: "ABCDEF0123456789",
		CreatedAt:   created,
		ExpiresAt:   created.Add(DefaultValidity),
	}
	if err := SaveKeypairMeta(db, kp); err != nil {
		t.Fatalf("SaveKeypairMeta: %v", err)
	}

	meta, err := GetKeypairMeta(db)
	if err != nil {
		t.Fatalf("GetKeypairMeta: %v", err)
	}
	if meta == nil {
		t.Fatal("expected metadata, got nil")
	}
	if meta.Fingerprint != kp.Fingerprint {
		t.Fatalf("fingerprint = %q, want %q", meta.Fingerprint, kp.Fingerprint)
	}
	if meta.Revoked {
		t.Fatal("expected revoked=false")
	}
	if meta.LastRotatedAt != nil {
		t.Fatal("expected LastRotatedAt nil")
	}
	if len(meta.KeyserversPublished) != 0 {
		t.Fatalf("expected no keyservers, got %v", meta.KeyserversPublished)
	}
}

// TestSaveKeypairMetaNoTable verifies SaveKeypairMeta surfaces DB errors when
// the schema is missing.
func TestSaveKeypairMetaNoTable(t *testing.T) {
	db, err := database.NewAppDatabase(database.DatabaseConfig{Driver: database.DriverSQLite, Path: filepath.Join(t.TempDir(), "missing.db")})
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	created := time.Now().UTC()
	kp := &Keypair{Fingerprint: "AAAA", CreatedAt: created, ExpiresAt: created.Add(time.Hour)}
	if err := SaveKeypairMeta(db, kp); err == nil {
		t.Fatal("expected error saving to db without pgp_keypair table")
	}
}

// TestGetKeypairMetaRotatedAndPublished verifies the rotated timestamp,
// keyservers list, and revoked flag decode correctly.
func TestGetKeypairMetaRotatedAndPublished(t *testing.T) {
	db := newTestDB(t)
	created := time.Now().UTC().Truncate(time.Second)
	rotated := created.Add(time.Hour)
	published := `[{"url":"https://keys.openpgp.org","published_at":"2026-07-25T00:00:00Z"}]`

	_, err := pgpTestExec(db,
		`INSERT INTO pgp_keypair (fingerprint, created_at, expires_at, last_rotated_at, keyservers_published, revoked)
		 VALUES (?, ?, ?, ?, ?, 1)`,
		"CCCC", created, created.Add(DefaultValidity), rotated, published,
	)
	if err != nil {
		t.Fatalf("insert: %v", err)
	}

	meta, err := GetKeypairMeta(db)
	if err != nil {
		t.Fatalf("GetKeypairMeta: %v", err)
	}
	if meta.LastRotatedAt == nil || !meta.LastRotatedAt.Equal(rotated) {
		t.Fatalf("LastRotatedAt = %v, want %v", meta.LastRotatedAt, rotated)
	}
	if !meta.Revoked {
		t.Fatal("expected revoked=true")
	}
	if len(meta.KeyserversPublished) != 1 || meta.KeyserversPublished[0].URL != "https://keys.openpgp.org" {
		t.Fatalf("keyservers = %v", meta.KeyserversPublished)
	}
}

// TestSetLastRotated verifies the rotation timestamp is stamped on the live row.
func TestSetLastRotated(t *testing.T) {
	db := newTestDB(t)
	created := time.Now().UTC().Truncate(time.Second)
	kp := &Keypair{Fingerprint: "AAAA", CreatedAt: created, ExpiresAt: created.Add(DefaultValidity)}
	if err := SaveKeypairMeta(db, kp); err != nil {
		t.Fatalf("SaveKeypairMeta: %v", err)
	}

	rotated := created.Add(48 * time.Hour)
	if err := SetLastRotated(db, rotated); err != nil {
		t.Fatalf("SetLastRotated: %v", err)
	}
	meta, err := GetKeypairMeta(db)
	if err != nil {
		t.Fatalf("GetKeypairMeta: %v", err)
	}
	if meta.LastRotatedAt == nil || !meta.LastRotatedAt.Equal(rotated) {
		t.Fatalf("LastRotatedAt = %v, want %v", meta.LastRotatedAt, rotated)
	}
}

// TestSetLastRotatedNoRow verifies an error when there is no keypair to stamp.
func TestSetLastRotatedNoRow(t *testing.T) {
	db := newTestDB(t)
	if err := SetLastRotated(db, time.Now()); err == nil {
		t.Fatal("expected error stamping empty pgp_keypair table")
	}
}

// TestMarkRevoked verifies the live keypair row is flagged revoked while the
// fingerprint is retained for audit history (AI.md PART 12 "revoked").
func TestMarkRevoked(t *testing.T) {
	db := newTestDB(t)
	created := time.Now().UTC().Truncate(time.Second)
	kp := &Keypair{Fingerprint: "REVOKEME", CreatedAt: created, ExpiresAt: created.Add(DefaultValidity)}
	if err := SaveKeypairMeta(db, kp); err != nil {
		t.Fatalf("SaveKeypairMeta: %v", err)
	}
	if err := MarkRevoked(db); err != nil {
		t.Fatalf("MarkRevoked: %v", err)
	}
	meta, err := GetKeypairMeta(db)
	if err != nil {
		t.Fatalf("GetKeypairMeta: %v", err)
	}
	if !meta.Revoked {
		t.Fatal("expected revoked=true after MarkRevoked")
	}
	if meta.Fingerprint != "REVOKEME" {
		t.Fatalf("fingerprint = %q, want REVOKEME", meta.Fingerprint)
	}
}

// TestMarkRevokedNoRow verifies an error when there is no keypair to revoke.
func TestMarkRevokedNoRow(t *testing.T) {
	db := newTestDB(t)
	if err := MarkRevoked(db); err == nil {
		t.Fatal("expected error revoking empty pgp_keypair table")
	}
}

// TestMarkRevokedNoTable verifies the underlying Exec error surfaces when the
// pgp_keypair table is missing entirely.
func TestMarkRevokedNoTable(t *testing.T) {
	db, err := database.NewAppDatabase(database.DatabaseConfig{Driver: database.DriverSQLite, Path: filepath.Join(t.TempDir(), "missing.db")})
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	if err := MarkRevoked(db); err == nil {
		t.Fatal("expected error revoking with no pgp_keypair table")
	}
}

// TestSaveKeypairMetaReplaces verifies a second save replaces the prior row so
// only the live keypair remains.
func TestSaveKeypairMetaReplaces(t *testing.T) {
	db := newTestDB(t)
	created := time.Now().UTC().Truncate(time.Second)

	first := &Keypair{Fingerprint: "AAAA", CreatedAt: created, ExpiresAt: created.Add(time.Hour)}
	second := &Keypair{Fingerprint: "BBBB", CreatedAt: created, ExpiresAt: created.Add(time.Hour)}
	if err := SaveKeypairMeta(db, first); err != nil {
		t.Fatalf("save first: %v", err)
	}
	if err := SaveKeypairMeta(db, second); err != nil {
		t.Fatalf("save second: %v", err)
	}

	var count int
	if err := pgpTestQueryRow(db, `SELECT COUNT(*) FROM pgp_keypair`).Scan(&count); err != nil {
		t.Fatalf("count: %v", err)
	}
	if count != 1 {
		t.Fatalf("expected 1 row, got %d", count)
	}
	meta, err := GetKeypairMeta(db)
	if err != nil {
		t.Fatalf("GetKeypairMeta: %v", err)
	}
	if meta.Fingerprint != "BBBB" {
		t.Fatalf("fingerprint = %q, want BBBB", meta.Fingerprint)
	}
}
