// SPDX-License-Identifier: MIT
// AI.md PART 11: wrapped previous encryption-key rotation state.
package secret

import (
	"context"
	"database/sql"
	"encoding/base64"
	"fmt"
	"time"

	"github.com/apimgr/vidveil/src/server/service/secreport"
)

const (
	EncryptionKeyPrevious           = "encryption_key_previous"
	EncryptionKeyPreviousUntil      = "encryption_key_previous_until"
	EncryptionKeyRotationPending    = "encryption_key_rotation_pending"
	EncryptionKeyCurrentFingerprint = secreport.EncryptionKeyCurrentFingerprint
)

// StorePreviousEncryptionKey stores the previous AES key wrapped by the new
// key. The transaction is supplied by the caller so both rows are atomic.
func StorePreviousEncryptionKey(ctx context.Context, tx *sql.Tx, previousKey, currentKey string, until time.Time) error {
	wrapped, err := secreport.EncryptAESGCM(currentKey, []byte(previousKey))
	if err != nil {
		return fmt.Errorf("wrap previous encryption key: %w", err)
	}
	value := base64.StdEncoding.EncodeToString(wrapped)
	pendingCiphertext, err := secreport.EncryptAESGCM(previousKey, []byte(currentKey))
	if err != nil {
		return fmt.Errorf("protect pending encryption key: %w", err)
	}
	for name, item := range map[string]string{
		EncryptionKeyPrevious:           value,
		EncryptionKeyPreviousUntil:      until.UTC().Format(time.RFC3339Nano),
		EncryptionKeyRotationPending:    base64.StdEncoding.EncodeToString(pendingCiphertext),
		EncryptionKeyCurrentFingerprint: secreport.EncryptionKeyFingerprint(currentKey),
	} {
		if _, err := tx.ExecContext(ctx, `INSERT INTO app_secrets (key,value,created_at) VALUES (?,?,?) ON CONFLICT(key) DO UPDATE SET value=excluded.value,created_at=excluded.created_at`, name, item, time.Now().UTC()); err != nil {
			return fmt.Errorf("store %s: %w", name, err)
		}
	}
	return nil
}

// SetCurrentEncryptionKeyMarker records the committed active key fingerprint
// in the same transaction as key rotation metadata.
func SetCurrentEncryptionKeyMarker(ctx context.Context, tx *sql.Tx, key string) error {
	_, err := tx.ExecContext(ctx, `INSERT INTO app_secrets (key,value,created_at) VALUES (?,?,?) ON CONFLICT(key) DO UPDATE SET value=excluded.value,created_at=excluded.created_at`, EncryptionKeyCurrentFingerprint, secreport.EncryptionKeyFingerprint(key), time.Now().UTC())
	if err != nil {
		return fmt.Errorf("store current encryption-key marker: %w", err)
	}
	return nil
}

// EnsureCurrentEncryptionKeyMarker initializes the fingerprint on first
// startup and rejects a config key that does not match the committed DB key.
func EnsureCurrentEncryptionKeyMarker(ctx context.Context, db *sql.DB, key string) error {
	var current string
	err := db.QueryRowContext(ctx, `SELECT value FROM app_secrets WHERE key = ?`, EncryptionKeyCurrentFingerprint).Scan(&current)
	if err == sql.ErrNoRows {
		tx, err := db.BeginTx(ctx, nil)
		if err != nil {
			return err
		}
		defer func() { _ = tx.Rollback() }()
		if err := SetCurrentEncryptionKeyMarker(ctx, tx, key); err != nil {
			return err
		}
		if err := tx.Commit(); err != nil {
			return err
		}
		return nil
	}
	if err != nil {
		return fmt.Errorf("read current encryption-key marker: %w", err)
	}
	if current != secreport.EncryptionKeyFingerprint(key) {
		return fmt.Errorf("configured encryption key does not match the committed database key")
	}
	return nil
}

// ImportLegacyPreviousEncryptionKey wraps and removes the plaintext key row
// written by the earlier VidVeil rotation implementation.
func ImportLegacyPreviousEncryptionKey(ctx context.Context, db *sql.DB, currentKey string) error {
	exists, err := PreviousEncryptionKeyExists(ctx, db)
	if err != nil {
		return err
	}
	if exists {
		if _, err := db.ExecContext(ctx, `DELETE FROM settings WHERE key = ?`, "security.previous_encryption_key"); err != nil {
			return fmt.Errorf("remove plaintext previous encryption key: %w", err)
		}
		return nil
	}
	var previous string
	var updatedAt time.Time
	err = db.QueryRowContext(ctx, `SELECT value, updated_at FROM settings WHERE key = ?`, "security.previous_encryption_key").Scan(&previous, &updatedAt)
	if err == sql.ErrNoRows {
		return nil
	}
	if err != nil {
		return fmt.Errorf("read legacy previous encryption key: %w", err)
	}
	until := updatedAt.UTC().Add(30 * 24 * time.Hour)
	wrapped, err := secreport.EncryptAESGCM(currentKey, []byte(previous))
	if err != nil {
		return fmt.Errorf("wrap legacy previous encryption key: %w", err)
	}
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()
	for name, value := range map[string]string{
		EncryptionKeyPrevious:      base64.StdEncoding.EncodeToString(wrapped),
		EncryptionKeyPreviousUntil: until.Format(time.RFC3339Nano),
	} {
		if _, err := tx.ExecContext(ctx, `INSERT INTO app_secrets (key,value,created_at) VALUES (?,?,?) ON CONFLICT(key) DO UPDATE SET value=excluded.value,created_at=excluded.created_at`, name, value, time.Now().UTC()); err != nil {
			return fmt.Errorf("store imported %s: %w", name, err)
		}
	}
	if _, err := tx.ExecContext(ctx, `DELETE FROM settings WHERE key = ?`, "security.previous_encryption_key"); err != nil {
		return fmt.Errorf("remove plaintext previous encryption key: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit imported encryption-key state: %w", err)
	}
	return nil
}

// LoadPendingEncryptionKey recovers the new key if the process stopped after
// persisting rotation state but before atomically switching secrets.yml.
func LoadPendingEncryptionKey(ctx context.Context, db *sql.DB, configuredKey string, now time.Time) (string, bool, error) {
	var encoded string
	err := db.QueryRowContext(ctx, `SELECT value FROM app_secrets WHERE key = ?`, EncryptionKeyRotationPending).Scan(&encoded)
	if err == sql.ErrNoRows {
		return "", false, nil
	}
	if err != nil {
		return "", false, fmt.Errorf("read pending encryption key: %w", err)
	}
	ciphertext, err := base64.StdEncoding.DecodeString(encoded)
	if err != nil {
		return "", false, fmt.Errorf("decode pending encryption key: %w", err)
	}
	if candidate, err := secreport.DecryptAESGCM(configuredKey, ciphertext); err == nil && len(candidate) == 64 {
		return string(candidate), true, nil
	}
	previous, ok, err := LoadPreviousEncryptionKey(ctx, db, configuredKey, now)
	if err != nil {
		return "", false, err
	}
	if !ok {
		return "", false, fmt.Errorf("cannot recover pending encryption-key rotation")
	}
	candidate, err := secreport.DecryptAESGCM(previous, ciphertext)
	if err != nil || len(candidate) != 64 {
		return "", false, fmt.Errorf("cannot recover pending encryption-key rotation")
	}
	return string(candidate), true, nil
}

// LoadPreviousEncryptionKey returns the grace-period key only while it is
// valid. A partial or malformed pair is an error so callers fail closed.
func LoadPreviousEncryptionKey(ctx context.Context, db *sql.DB, currentKey string, now time.Time) (string, bool, error) {
	var wrapped, untilText string
	err := db.QueryRowContext(ctx, `SELECT value FROM app_secrets WHERE key = ?`, EncryptionKeyPrevious).Scan(&wrapped)
	if err == sql.ErrNoRows {
		var expiryCount int
		if checkErr := db.QueryRowContext(ctx, `SELECT COUNT(*) FROM app_secrets WHERE key = ?`, EncryptionKeyPreviousUntil).Scan(&expiryCount); checkErr != nil {
			return "", false, checkErr
		}
		if expiryCount != 0 {
			return "", false, fmt.Errorf("previous encryption-key expiry exists without a wrapped key")
		}
		return "", false, nil
	}
	if err != nil {
		return "", false, fmt.Errorf("read previous encryption key: %w", err)
	}
	if err := db.QueryRowContext(ctx, `SELECT value FROM app_secrets WHERE key = ?`, EncryptionKeyPreviousUntil).Scan(&untilText); err != nil {
		return "", false, fmt.Errorf("read previous encryption-key expiry: %w", err)
	}
	until, err := time.Parse(time.RFC3339Nano, untilText)
	if err != nil {
		return "", false, fmt.Errorf("parse previous encryption-key expiry: %w", err)
	}
	if now.UTC().After(until.UTC()) {
		return "", false, nil
	}
	ciphertext, err := base64.StdEncoding.DecodeString(wrapped)
	if err != nil {
		return "", false, fmt.Errorf("decode wrapped previous encryption key: %w", err)
	}
	plain, err := secreport.DecryptAESGCM(currentKey, ciphertext)
	if err != nil {
		return "", false, fmt.Errorf("unwrap previous encryption key: %w", err)
	}
	if len(plain) != 64 {
		return "", false, fmt.Errorf("wrapped previous encryption key has invalid length")
	}
	return string(plain), true, nil
}

// DeletePreviousEncryptionKey removes both rotation-state rows.
func DeletePreviousEncryptionKey(ctx context.Context, tx *sql.Tx) error {
	for _, key := range []string{EncryptionKeyPrevious, EncryptionKeyPreviousUntil, EncryptionKeyRotationPending} {
		if _, err := tx.ExecContext(ctx, `DELETE FROM app_secrets WHERE key = ?`, key); err != nil {
			return fmt.Errorf("delete %s: %w", key, err)
		}
	}
	return nil
}

// DeletePendingEncryptionKey clears only the crash-recovery marker after all
// dependent rows have been confirmed readable with the current key.
func DeletePendingEncryptionKey(ctx context.Context, tx *sql.Tx) error {
	if _, err := tx.ExecContext(ctx, `DELETE FROM app_secrets WHERE key = ?`, EncryptionKeyRotationPending); err != nil {
		return fmt.Errorf("delete pending encryption-key marker: %w", err)
	}
	return nil
}

// PreviousEncryptionKeyExists reports whether either grace-period marker is
// present, including incomplete/corrupt state that must block a new rotation.
func PreviousEncryptionKeyExists(ctx context.Context, db *sql.DB) (bool, error) {
	var count int
	if err := db.QueryRowContext(ctx, `SELECT COUNT(*) FROM app_secrets WHERE key IN (?,?)`, EncryptionKeyPrevious, EncryptionKeyPreviousUntil).Scan(&count); err != nil {
		return false, fmt.Errorf("inspect previous encryption-key state: %w", err)
	}
	return count > 0, nil
}

// RetirePreviousEncryptionKey removes an expired previous key and its expiry.
func RetirePreviousEncryptionKey(ctx context.Context, db *sql.DB, now time.Time) (bool, error) {
	var previousCount int
	if err := db.QueryRowContext(ctx, `SELECT COUNT(*) FROM app_secrets WHERE key = ?`, EncryptionKeyPrevious).Scan(&previousCount); err != nil {
		return false, fmt.Errorf("inspect previous encryption key before retirement: %w", err)
	}
	if previousCount == 0 {
		return false, nil
	}
	var untilText string
	err := db.QueryRowContext(ctx, `SELECT value FROM app_secrets WHERE key = ?`, EncryptionKeyPreviousUntil).Scan(&untilText)
	if err != nil {
		return false, fmt.Errorf("read previous encryption-key expiry: %w", err)
	}
	until, err := time.Parse(time.RFC3339Nano, untilText)
	if err != nil {
		return false, fmt.Errorf("parse previous encryption-key expiry: %w", err)
	}
	if now.UTC().Before(until.UTC()) {
		return false, nil
	}
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return false, err
	}
	defer func() { _ = tx.Rollback() }()
	if err := DeletePreviousEncryptionKey(ctx, tx); err != nil {
		return false, err
	}
	if err := tx.Commit(); err != nil {
		return false, err
	}
	return true, nil
}
