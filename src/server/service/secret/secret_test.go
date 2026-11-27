// SPDX-License-Identifier: MIT
// AI.md PART 28: Unit tests for secrets service

package secret

import (
	"bytes"
	"context"
	"database/sql"
	"github.com/apimgr/vidveil/src/common/dbstore"
	"os"
	"path/filepath"
	"testing"
	"time"

	_ "modernc.org/sqlite"
)

func setupTestDB(t *testing.T) (*sql.DB, func()) {
	t.Helper()

	tmpDir := filepath.Join(os.TempDir(), "apimgr", "vidveil-test-"+t.Name())
	if err := os.MkdirAll(tmpDir, 0755); err != nil {
		t.Fatalf("create temporary database directory: %v", err)
	}
	dbPath := filepath.Join(tmpDir, "test.db")

	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		t.Fatalf("open db: %v", err)
	}

	// Create app_secrets table
	_, err = db.Exec(`CREATE TABLE IF NOT EXISTS app_secrets (
		key TEXT PRIMARY KEY,
		value TEXT NOT NULL,
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
		rotated_at DATETIME,
		expires_at DATETIME,
		previous_value TEXT
	)`)
	if err != nil {
		t.Fatalf("create table: %v", err)
	}

	cleanup := func() {
		if err := db.Close(); err != nil {
			t.Errorf("close test database: %v", err)
		}
		if err := os.RemoveAll(tmpDir); err != nil {
			t.Errorf("remove temporary database directory: %v", err)
		}
	}

	return db, cleanup
}

type rawDBTestStore struct{ db *sql.DB }

type secretTestReader struct{ db *sql.DB }
type secretTestRow struct{ row *sql.Row }

func (r secretTestReader) QueryContext(ctx context.Context, query string, args ...any) (*sql.Rows, error) {
	return r.db.QueryContext(ctx, query, args...)
}
func (r secretTestReader) QueryRowContext(ctx context.Context, query string, args ...any) dbstore.RowScanner {
	return secretTestRow{row: r.db.QueryRowContext(ctx, query, args...)}
}
func (r secretTestReader) PingContext(ctx context.Context) error { return r.db.PingContext(ctx) }
func (r secretTestRow) Scan(dest ...any) error                   { return r.row.Scan(dest...) }

func (s rawDBTestStore) Read(ctx context.Context, fn func(context.Context, dbstore.Reader) error) error {
	return fn(ctx, secretTestReader(s))
}

func (s rawDBTestStore) Write(ctx context.Context, fn func(*sql.Tx) error) error {
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

func testStore(db *sql.DB) Store { return rawDBTestStore{db: db} }

func TestNewManager(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()

	m := NewManager(testStore(db))
	if m == nil {
		t.Fatal("NewManager returned nil")
	}
	if m.db != (rawDBTestStore{db: db}) {
		t.Error("Manager db not set correctly")
	}
}

func TestEnsureSecrets_CreatesAll(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()

	m := NewManager(testStore(db))
	ctx := context.Background()

	if err := m.EnsureSecrets(ctx); err != nil {
		t.Fatalf("EnsureSecrets: %v", err)
	}

	// Verify all secrets exist
	for _, key := range []SecretKey{InstallationSecret, CookieSigningKey, CSRFTokenSecret} {
		secret, err := m.Get(ctx, key)
		if err != nil {
			t.Errorf("Get %s: %v", key, err)
		}
		if len(secret) != 32 {
			t.Errorf("secret %s length = %d, want 32", key, len(secret))
		}
	}
}

func TestEnsureSecrets_Idempotent(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()

	m := NewManager(testStore(db))
	ctx := context.Background()

	// Run twice
	if err := m.EnsureSecrets(ctx); err != nil {
		t.Fatalf("first EnsureSecrets: %v", err)
	}

	// Get value after first run
	first, err := m.Get(ctx, InstallationSecret)
	if err != nil {
		t.Fatalf("Get after first: %v", err)
	}

	if err := m.EnsureSecrets(ctx); err != nil {
		t.Fatalf("second EnsureSecrets: %v", err)
	}

	// Value should be same
	second, err := m.Get(ctx, InstallationSecret)
	if err != nil {
		t.Fatalf("Get after second: %v", err)
	}

	if !bytes.Equal(first, second) {
		t.Error("EnsureSecrets regenerated existing secret")
	}
}

func TestGetInstallationSecret(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()

	m := NewManager(testStore(db))
	ctx := context.Background()

	if err := m.EnsureSecrets(ctx); err != nil {
		t.Fatalf("EnsureSecrets: %v", err)
	}

	secret, err := m.GetInstallationSecret(ctx)
	if err != nil {
		t.Fatalf("GetInstallationSecret: %v", err)
	}
	if len(secret) != 32 {
		t.Errorf("secret length = %d, want 32", len(secret))
	}
}

func TestGet_NotFound(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()

	m := NewManager(testStore(db))
	ctx := context.Background()

	_, err := m.Get(ctx, "nonexistent")
	if err == nil {
		t.Error("Get nonexistent: expected error")
	}
}

func TestRotate(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()

	m := NewManager(testStore(db))
	ctx := context.Background()

	if err := m.EnsureSecrets(ctx); err != nil {
		t.Fatalf("EnsureSecrets: %v", err)
	}

	// Get original value
	original, err := m.Get(ctx, InstallationSecret)
	if err != nil {
		t.Fatalf("Get original: %v", err)
	}

	// Rotate
	if err := m.Rotate(ctx, InstallationSecret); err != nil {
		t.Fatalf("Rotate: %v", err)
	}

	// Get new value
	rotated, err := m.Get(ctx, InstallationSecret)
	if err != nil {
		t.Fatalf("Get rotated: %v", err)
	}

	// Should be different
	if bytes.Equal(original, rotated) {
		t.Error("Rotate did not change secret")
	}
}

func TestRotateImmediatelyInvalidatesPrevious(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()
	m := NewManager(testStore(db))
	ctx := context.Background()
	if err := m.EnsureSecrets(ctx); err != nil {
		t.Fatalf("EnsureSecrets: %v", err)
	}
	previous, err := m.Get(ctx, CSRFTokenSecret)
	if err != nil {
		t.Fatalf("Get CSRF secret: %v", err)
	}
	if err := m.RotateImmediately(ctx, CSRFTokenSecret); err != nil {
		t.Fatalf("RotateImmediately: %v", err)
	}
	valid, err := m.ValidateWithPrevious(ctx, CSRFTokenSecret, previous)
	if err != nil {
		t.Fatalf("ValidateWithPrevious: %v", err)
	}
	if valid {
		t.Fatal("previous CSRF secret remained valid after immediate rotation")
	}
}

func TestRotateIfDue(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()
	m := NewManager(testStore(db))
	ctx := context.Background()
	if err := m.EnsureSecrets(ctx); err != nil {
		t.Fatalf("EnsureSecrets: %v", err)
	}
	if _, err := db.Exec(`UPDATE app_secrets SET created_at = ? WHERE key = ?`, time.Now().Add(-181*24*time.Hour), string(CSRFTokenSecret)); err != nil {
		t.Fatalf("age CSRF secret: %v", err)
	}
	rotated, err := m.RotateIfDue(ctx, CSRFTokenSecret, 180*24*time.Hour)
	if err != nil || !rotated {
		t.Fatalf("RotateIfDue() = %v, %v; want true, nil", rotated, err)
	}
	rotated, err = m.RotateIfDue(ctx, CSRFTokenSecret, 180*24*time.Hour)
	if err != nil || rotated {
		t.Fatalf("RotateIfDue() after rotation = %v, %v; want false, nil", rotated, err)
	}
}

func TestValidateWithPrevious_CurrentValue(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()

	m := NewManager(testStore(db))
	ctx := context.Background()

	if err := m.EnsureSecrets(ctx); err != nil {
		t.Fatalf("EnsureSecrets: %v", err)
	}

	secret, err := m.Get(ctx, InstallationSecret)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}

	valid, err := m.ValidateWithPrevious(ctx, InstallationSecret, secret)
	if err != nil {
		t.Fatalf("ValidateWithPrevious: %v", err)
	}
	if !valid {
		t.Error("current value should validate")
	}
}

func TestValidateWithPrevious_PreviousValue(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()

	m := NewManager(testStore(db))
	ctx := context.Background()

	if err := m.EnsureSecrets(ctx); err != nil {
		t.Fatalf("EnsureSecrets: %v", err)
	}

	// Get original value
	original, err := m.Get(ctx, InstallationSecret)
	if err != nil {
		t.Fatalf("Get original: %v", err)
	}

	// Rotate
	if err := m.Rotate(ctx, InstallationSecret); err != nil {
		t.Fatalf("Rotate: %v", err)
	}

	// Original should still validate (within 7-day window)
	valid, err := m.ValidateWithPrevious(ctx, InstallationSecret, original)
	if err != nil {
		t.Fatalf("ValidateWithPrevious: %v", err)
	}
	if !valid {
		t.Error("previous value should validate within window")
	}
}

func TestValidateWithPrevious_InvalidValue(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()

	m := NewManager(testStore(db))
	ctx := context.Background()

	if err := m.EnsureSecrets(ctx); err != nil {
		t.Fatalf("EnsureSecrets: %v", err)
	}

	for _, invalid := range [][]byte{make([]byte, 32), []byte("short"), make([]byte, 33)} {
		valid, err := m.ValidateWithPrevious(ctx, InstallationSecret, invalid)
		if err != nil {
			t.Fatalf("ValidateWithPrevious: %v", err)
		}
		if valid {
			t.Errorf("invalid value of length %d should not validate", len(invalid))
		}
	}
}

func TestGenerateSecretBytes(t *testing.T) {
	secretBytes, err := generateSecretBytes()
	if err != nil {
		t.Fatalf("generateSecretBytes: %v", err)
	}
	if len(secretBytes) != 32 {
		t.Errorf("length = %d, want 32", len(secretBytes))
	}

	// Generate another - should be different
	secretBytes2, err := generateSecretBytes()
	if err != nil {
		t.Fatalf("generateSecretBytes 2: %v", err)
	}
	if bytes.Equal(secretBytes, secretBytes2) {
		t.Error("two generated secrets should differ")
	}
}

func TestSecretKeyConstants(t *testing.T) {
	if InstallationSecret != "installation_secret" {
		t.Errorf("InstallationSecret = %q", InstallationSecret)
	}
	if CookieSigningKey != "cookie_signing_key" {
		t.Errorf("CookieSigningKey = %q", CookieSigningKey)
	}
	if CSRFTokenSecret != "csrf_token_secret" {
		t.Errorf("CSRFTokenSecret = %q", CSRFTokenSecret)
	}
}
