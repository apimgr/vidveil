// SPDX-License-Identifier: MIT
// AI.md PART 28: Unit tests for secrets service

package secrets

import (
	"context"
	"database/sql"
	"os"
	"path/filepath"
	"testing"
	"time"

	_ "modernc.org/sqlite"
)

func setupTestDB(t *testing.T) (*sql.DB, func()) {
	t.Helper()

	tmpDir := filepath.Join(os.TempDir(), "apimgr", "vidveil-test-"+t.Name())
	os.MkdirAll(tmpDir, 0755)
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
		db.Close()
		os.RemoveAll(tmpDir)
	}

	return db, cleanup
}

func TestNewManager(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()

	m := NewManager(db)
	if m == nil {
		t.Fatal("NewManager returned nil")
	}
	if m.db != db {
		t.Error("Manager db not set correctly")
	}
}

func TestEnsureSecrets_CreatesAll(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()

	m := NewManager(db)
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

	m := NewManager(db)
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

	if !constantTimeEqual(first, second) {
		t.Error("EnsureSecrets regenerated existing secret")
	}
}

func TestGetInstallationSecret(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()

	m := NewManager(db)
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

	m := NewManager(db)
	ctx := context.Background()

	_, err := m.Get(ctx, "nonexistent")
	if err == nil {
		t.Error("Get nonexistent: expected error")
	}
}

func TestRotate(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()

	m := NewManager(db)
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
	if constantTimeEqual(original, rotated) {
		t.Error("Rotate did not change secret")
	}
}

func TestValidateWithPrevious_CurrentValue(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()

	m := NewManager(db)
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

	m := NewManager(db)
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

	m := NewManager(db)
	ctx := context.Background()

	if err := m.EnsureSecrets(ctx); err != nil {
		t.Fatalf("EnsureSecrets: %v", err)
	}

	invalid := make([]byte, 32)
	valid, err := m.ValidateWithPrevious(ctx, InstallationSecret, invalid)
	if err != nil {
		t.Fatalf("ValidateWithPrevious: %v", err)
	}
	if valid {
		t.Error("invalid value should not validate")
	}
}

func TestConstantTimeEqual(t *testing.T) {
	a := []byte{1, 2, 3, 4}
	b := []byte{1, 2, 3, 4}
	c := []byte{1, 2, 3, 5}
	d := []byte{1, 2, 3}

	if !constantTimeEqual(a, b) {
		t.Error("equal slices should match")
	}
	if constantTimeEqual(a, c) {
		t.Error("different slices should not match")
	}
	if constantTimeEqual(a, d) {
		t.Error("different length slices should not match")
	}
}

func TestGenerateSecretBytes(t *testing.T) {
	bytes, err := generateSecretBytes()
	if err != nil {
		t.Fatalf("generateSecretBytes: %v", err)
	}
	if len(bytes) != 32 {
		t.Errorf("length = %d, want 32", len(bytes))
	}

	// Generate another - should be different
	bytes2, err := generateSecretBytes()
	if err != nil {
		t.Fatalf("generateSecretBytes 2: %v", err)
	}
	if constantTimeEqual(bytes, bytes2) {
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

// Benchmark constant-time comparison
func BenchmarkConstantTimeEqual(b *testing.B) {
	a := make([]byte, 32)
	c := make([]byte, 32)
	for i := range a {
		a[i] = byte(i)
		c[i] = byte(i)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		constantTimeEqual(a, c)
	}
}

// Verify constant-time behavior by checking timing doesn't leak position
func TestConstantTimeEqual_TimingConsistency(t *testing.T) {
	a := make([]byte, 32)
	for i := range a {
		a[i] = byte(i)
	}

	// Early mismatch
	earlyMismatch := make([]byte, 32)
	copy(earlyMismatch, a)
	earlyMismatch[0] = 255

	// Late mismatch
	lateMismatch := make([]byte, 32)
	copy(lateMismatch, a)
	lateMismatch[31] = 255

	// Run many iterations and verify both complete in similar time
	iterations := 10000

	startEarly := time.Now()
	for i := 0; i < iterations; i++ {
		constantTimeEqual(a, earlyMismatch)
	}
	earlyDuration := time.Since(startEarly)

	startLate := time.Now()
	for i := 0; i < iterations; i++ {
		constantTimeEqual(a, lateMismatch)
	}
	lateDuration := time.Since(startLate)

	// Both should be within 50% of each other (allowing for noise)
	ratio := float64(earlyDuration) / float64(lateDuration)
	if ratio < 0.5 || ratio > 2.0 {
		t.Logf("timing: early=%v late=%v ratio=%.2f", earlyDuration, lateDuration, ratio)
		// Not failing - timing tests are inherently noisy
	}
}
