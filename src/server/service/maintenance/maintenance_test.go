// SPDX-License-Identifier: MIT
// Tests for the maintenance package: NewMaintenanceManager, IsMaintenanceMode,
// SetMaintenanceMode, encryptBackup, decryptBackup, and verifySQLiteIntegrity.
package maintenance

import (
	"bytes"
	"testing"
)

// newTestManager creates a MaintenanceManager backed by a temp directory so
// no test writes to real system paths.
func newTestManager(t *testing.T) *MaintenanceManager {
	t.Helper()
	dir := t.TempDir()
	return NewMaintenanceManager(dir, dir, "0.1.0")
}

// ---- NewMaintenanceManager ----

func TestNewMaintenanceManagerReturnsNonNil(t *testing.T) {
	m := newTestManager(t)
	if m == nil {
		t.Fatal("NewMaintenanceManager returned nil")
	}
}

func TestNewMaintenanceManagerVersionStored(t *testing.T) {
	m := newTestManager(t)
	if m.version != "0.1.0" {
		t.Errorf("version = %q, want %q", m.version, "0.1.0")
	}
}

func TestNewMaintenanceManagerPathsNonNil(t *testing.T) {
	m := newTestManager(t)
	if m.paths == nil {
		t.Error("paths must be non-nil after construction")
	}
}

// ---- IsMaintenanceMode ----

func TestIsMaintenanceModeInitiallyFalse(t *testing.T) {
	m := newTestManager(t)
	if m.IsMaintenanceMode() {
		t.Error("IsMaintenanceMode should be false before SetMaintenanceMode is called")
	}
}

// ---- SetMaintenanceMode ----

func TestSetMaintenanceModeEnableReturnsTrueOnCheck(t *testing.T) {
	m := newTestManager(t)
	if err := m.SetMaintenanceMode(true); err != nil {
		t.Fatalf("SetMaintenanceMode(true) error: %v", err)
	}
	if !m.IsMaintenanceMode() {
		t.Error("IsMaintenanceMode should be true after SetMaintenanceMode(true)")
	}
}

func TestSetMaintenanceModeDisableReturnsFalseOnCheck(t *testing.T) {
	m := newTestManager(t)
	if err := m.SetMaintenanceMode(true); err != nil {
		t.Fatalf("SetMaintenanceMode(true) error: %v", err)
	}
	if err := m.SetMaintenanceMode(false); err != nil {
		t.Fatalf("SetMaintenanceMode(false) error: %v", err)
	}
	if m.IsMaintenanceMode() {
		t.Error("IsMaintenanceMode should be false after SetMaintenanceMode(false)")
	}
}

func TestSetMaintenanceModeDisableWhenAlreadyOffNoError(t *testing.T) {
	m := newTestManager(t)
	// Calling disable when already off must not return an error
	if err := m.SetMaintenanceMode(false); err != nil {
		t.Errorf("SetMaintenanceMode(false) on an inactive flag returned error: %v", err)
	}
}

func TestSetMaintenanceModeToggle(t *testing.T) {
	m := newTestManager(t)
	for i, enabled := range []bool{true, false, true, false} {
		if err := m.SetMaintenanceMode(enabled); err != nil {
			t.Fatalf("iteration %d: SetMaintenanceMode(%v) error: %v", i, enabled, err)
		}
		if got := m.IsMaintenanceMode(); got != enabled {
			t.Errorf("iteration %d: IsMaintenanceMode() = %v, want %v", i, got, enabled)
		}
	}
}

// ---- encryptBackup ----

func TestEncryptBackupReturnsDifferentBytes(t *testing.T) {
	m := newTestManager(t)
	data := []byte("hello world backup data")
	enc, err := m.encryptBackup(data, "testpassword")
	if err != nil {
		t.Fatalf("encryptBackup error: %v", err)
	}
	if bytes.Equal(enc, data) {
		t.Error("encrypted output must differ from plaintext")
	}
}

func TestEncryptBackupMinimumLength(t *testing.T) {
	m := newTestManager(t)
	data := []byte("x")
	enc, err := m.encryptBackup(data, "p")
	if err != nil {
		t.Fatalf("encryptBackup error: %v", err)
	}
	// salt(16) + nonce(12) = 28 bytes minimum, plus ciphertext and GCM tag
	if len(enc) < 28 {
		t.Errorf("encrypted length = %d, want >= 28", len(enc))
	}
}

func TestEncryptBackupTwoCallsDifferentOutput(t *testing.T) {
	m := newTestManager(t)
	data := []byte("same data every time")
	enc1, err1 := m.encryptBackup(data, "password")
	enc2, err2 := m.encryptBackup(data, "password")
	if err1 != nil || err2 != nil {
		t.Fatalf("encryptBackup errors: %v, %v", err1, err2)
	}
	if bytes.Equal(enc1, enc2) {
		t.Error("two encryptions of the same data must produce different ciphertext (random salt/nonce)")
	}
}

// ---- decryptBackup ----

func TestDecryptBackupRoundTrip(t *testing.T) {
	m := newTestManager(t)
	original := []byte("round-trip payload 1234567890")
	enc, err := m.encryptBackup(original, "secret")
	if err != nil {
		t.Fatalf("encryptBackup error: %v", err)
	}
	dec, err := m.decryptBackup(enc, "secret")
	if err != nil {
		t.Fatalf("decryptBackup error: %v", err)
	}
	if !bytes.Equal(dec, original) {
		t.Errorf("decrypted = %q, want %q", dec, original)
	}
}

func TestDecryptBackupWrongPasswordReturnsError(t *testing.T) {
	m := newTestManager(t)
	enc, err := m.encryptBackup([]byte("data"), "correctpass")
	if err != nil {
		t.Fatalf("encryptBackup error: %v", err)
	}
	_, err = m.decryptBackup(enc, "wrongpass")
	if err == nil {
		t.Error("expected error when decrypting with wrong password")
	}
}

func TestDecryptBackupDataTooShortReturnsError(t *testing.T) {
	m := newTestManager(t)
	// Fewer than 28 bytes (salt+nonce minimum)
	_, err := m.decryptBackup(make([]byte, 10), "anypassword")
	if err == nil {
		t.Error("expected error for data shorter than 28 bytes")
	}
}

func TestDecryptBackupEmptyDataReturnsError(t *testing.T) {
	m := newTestManager(t)
	_, err := m.decryptBackup([]byte{}, "password")
	if err == nil {
		t.Error("expected error for empty encrypted data")
	}
}

func TestDecryptBackupExactly27BytesReturnsError(t *testing.T) {
	m := newTestManager(t)
	// 27 bytes is one byte under the minimum (salt=16, nonce=12 → 28)
	_, err := m.decryptBackup(make([]byte, 27), "password")
	if err == nil {
		t.Error("expected error for 27-byte input (below minimum 28)")
	}
}

// ---- verifySQLiteIntegrity ----

func TestVerifySQLiteIntegrityEmptyBytesReturnsError(t *testing.T) {
	if err := verifySQLiteIntegrity([]byte{}); err == nil {
		t.Error("expected error for empty byte slice")
	}
}

func TestVerifySQLiteIntegrityNonSQLiteDataReturnsError(t *testing.T) {
	if err := verifySQLiteIntegrity([]byte("this is not a database file at all")); err == nil {
		t.Error("expected error for non-SQLite data")
	}
}

func TestVerifySQLiteIntegrityTooSmallReturnsError(t *testing.T) {
	// Only 5 bytes — well under the 16-byte SQLite magic header
	if err := verifySQLiteIntegrity([]byte("SQL")); err == nil {
		t.Error("expected error for data shorter than SQLite magic header")
	}
}

func TestVerifySQLiteIntegrityWrongMagicReturnsError(t *testing.T) {
	// Exactly 16 bytes but wrong content
	if err := verifySQLiteIntegrity([]byte("SQLite format 4\x00")); err == nil {
		t.Error("expected error for data with wrong SQLite magic header")
	}
}

func TestVerifySQLiteIntegrityValidMagicReturnsNil(t *testing.T) {
	// Valid SQLite magic header followed by padding to satisfy length check
	magic := make([]byte, 100)
	copy(magic, "SQLite format 3\x00")
	if err := verifySQLiteIntegrity(magic); err != nil {
		t.Errorf("expected nil error for valid SQLite magic header: %v", err)
	}
}
