// SPDX-License-Identifier: MIT
// Package apitoken protects SHA-256 API-token digests at rest.
package apitoken

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"regexp"
	"strings"

	"github.com/apimgr/vidveil/src/server/service/secreport"
)

const encryptedHashPrefix = "enc:v1:"

var sha256HashPattern = regexp.MustCompile(`^[0-9a-f]{64}$`)

// EncryptHash encrypts a SHA-256 token digest without ever storing the token.
func EncryptHash(encryptionKey, hash string) (string, error) {
	if !sha256HashPattern.MatchString(hash) {
		return "", fmt.Errorf("invalid API-token SHA-256 digest")
	}
	ciphertext, err := secreport.EncryptAESGCM(encryptionKey, []byte(hash))
	if err != nil {
		return "", err
	}
	return encryptedHashPrefix + base64.RawURLEncoding.EncodeToString(ciphertext), nil
}

// EncryptToken protects a raw owner token for the explicit reveal action.
// Callers must never include the returned ciphertext in ordinary token views.
func EncryptToken(encryptionKey, token string) (string, error) {
	if !strings.HasPrefix(token, "tok_") || len(token) < 5 {
		return "", fmt.Errorf("invalid API token")
	}
	ciphertext, err := secreport.EncryptAESGCM(encryptionKey, []byte(token))
	if err != nil {
		return "", err
	}
	return encryptedHashPrefix + base64.RawURLEncoding.EncodeToString(ciphertext), nil
}

// DecryptToken returns a raw token only to the caller performing an explicit
// reveal operation.
func DecryptToken(encryptionKey, stored string) (string, error) {
	if !strings.HasPrefix(stored, encryptedHashPrefix) {
		return "", fmt.Errorf("invalid encrypted API token")
	}
	ciphertext, err := base64.RawURLEncoding.DecodeString(strings.TrimPrefix(stored, encryptedHashPrefix))
	if err != nil {
		return "", fmt.Errorf("decode encrypted API token: %w", err)
	}
	plaintext, err := secreport.DecryptAESGCM(encryptionKey, ciphertext)
	if err != nil {
		return "", fmt.Errorf("decrypt API token: %w", err)
	}
	token := string(plaintext)
	if !strings.HasPrefix(token, "tok_") || len(token) < 5 {
		return "", fmt.Errorf("invalid decrypted API token")
	}
	return token, nil
}

// DecryptHash accepts legacy plaintext digests so existing databases can be
// upgraded before the server starts using the encrypted representation.
func DecryptHash(encryptionKey, stored string) (string, error) {
	if !strings.HasPrefix(stored, encryptedHashPrefix) {
		if !sha256HashPattern.MatchString(stored) {
			return "", fmt.Errorf("invalid stored API-token SHA-256 digest")
		}
		return stored, nil
	}
	ciphertext, err := base64.RawURLEncoding.DecodeString(strings.TrimPrefix(stored, encryptedHashPrefix))
	if err != nil {
		return "", fmt.Errorf("decode encrypted API-token digest: %w", err)
	}
	plaintext, err := secreport.DecryptAESGCM(encryptionKey, ciphertext)
	if err != nil {
		return "", fmt.Errorf("decrypt API-token digest: %w", err)
	}
	hash := string(plaintext)
	if !sha256HashPattern.MatchString(hash) {
		return "", fmt.Errorf("invalid decrypted API-token SHA-256 digest")
	}
	return hash, nil
}

// NormalizeLegacyHashes converts encrypted legacy digests to the SHA-256
// representation required by the current schema. Plain digests remain intact.
func NormalizeLegacyHashes(ctx context.Context, db *sql.DB, encryptionKey string) error {
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin API-token digest migration: %w", err)
	}
	defer func() { _ = tx.Rollback() }()
	rows, err := tx.QueryContext(ctx, `SELECT id, token_hash FROM api_tokens ORDER BY id`)
	if err != nil {
		return fmt.Errorf("read API-token digests: %w", err)
	}
	type storedHash struct {
		id   int64
		hash string
	}
	var pending []storedHash
	for rows.Next() {
		var item storedHash
		if err := rows.Scan(&item.id, &item.hash); err != nil {
			_ = rows.Close()
			return fmt.Errorf("scan API-token digest: %w", err)
		}
		if !strings.HasPrefix(item.hash, encryptedHashPrefix) {
			if !sha256HashPattern.MatchString(item.hash) {
				_ = rows.Close()
				return fmt.Errorf("validate API-token digest %d: invalid digest", item.id)
			}
			continue
		}
		plain, err := DecryptHash(encryptionKey, item.hash)
		if err != nil {
			_ = rows.Close()
			return fmt.Errorf("decrypt legacy API-token digest %d: %w", item.id, err)
		}
		item.hash = plain
		pending = append(pending, item)
	}
	if err := rows.Err(); err != nil {
		_ = rows.Close()
		return fmt.Errorf("read API-token digests: %w", err)
	}
	if err := rows.Close(); err != nil {
		return fmt.Errorf("close API-token digests: %w", err)
	}
	for _, item := range pending {
		if _, err := tx.ExecContext(ctx, `UPDATE api_tokens SET token_hash = ? WHERE id = ?`, item.hash, item.id); err != nil {
			return fmt.Errorf("store API-token digest %d: %w", item.id, err)
		}
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit API-token digest migration: %w", err)
	}
	return nil
}

// ReencryptHashes normalizes legacy encrypted digests in the caller's transaction.
func ReencryptHashes(ctx context.Context, tx *sql.Tx, oldKey, newKey string) error {
	rows, err := tx.QueryContext(ctx, `SELECT id, token_hash FROM api_tokens ORDER BY id`)
	if err != nil {
		return fmt.Errorf("read API-token digests for rotation: %w", err)
	}
	type storedHash struct {
		id   int64
		hash string
	}
	var hashes []storedHash
	for rows.Next() {
		var item storedHash
		if err := rows.Scan(&item.id, &item.hash); err != nil {
			_ = rows.Close()
			return fmt.Errorf("scan API-token digest for rotation: %w", err)
		}
		hashes = append(hashes, item)
	}
	if err := rows.Err(); err != nil {
		_ = rows.Close()
		return fmt.Errorf("read API-token digests for rotation: %w", err)
	}
	if err := rows.Close(); err != nil {
		return fmt.Errorf("close API-token digests for rotation: %w", err)
	}
	for _, item := range hashes {
		hash, err := DecryptHash(oldKey, item.hash)
		if err != nil {
			return fmt.Errorf("decrypt API-token digest %d for rotation: %w", item.id, err)
		}
		if _, err := tx.ExecContext(ctx, `UPDATE api_tokens SET token_hash = ? WHERE id = ?`, hash, item.id); err != nil {
			return fmt.Errorf("store API-token digest %d: %w", item.id, err)
		}
	}
	return nil
}

// ReencryptTokens rotates the encrypted raw-token copies in the caller's
// transaction. Empty values belong to legacy tokens and remain unrevealable.
func ReencryptTokens(ctx context.Context, tx *sql.Tx, oldKey, newKey string) error {
	rows, err := tx.QueryContext(ctx, `SELECT id, token_enc FROM api_tokens WHERE token_enc <> '' ORDER BY id`)
	if err != nil {
		return fmt.Errorf("read encrypted API tokens for rotation: %w", err)
	}
	type storedToken struct {
		id    int64
		token string
	}
	var tokens []storedToken
	for rows.Next() {
		var item storedToken
		if err := rows.Scan(&item.id, &item.token); err != nil {
			_ = rows.Close()
			return fmt.Errorf("scan encrypted API token for rotation: %w", err)
		}
		tokens = append(tokens, item)
	}
	if err := rows.Err(); err != nil {
		_ = rows.Close()
		return fmt.Errorf("read encrypted API tokens for rotation: %w", err)
	}
	if err := rows.Close(); err != nil {
		return fmt.Errorf("close encrypted API tokens for rotation: %w", err)
	}
	for _, item := range tokens {
		plain, err := DecryptToken(oldKey, item.token)
		if err != nil {
			return fmt.Errorf("decrypt API token %d for rotation: %w", item.id, err)
		}
		value, err := EncryptToken(newKey, plain)
		if err != nil {
			return fmt.Errorf("encrypt API token %d for rotation: %w", item.id, err)
		}
		if _, err := tx.ExecContext(ctx, `UPDATE api_tokens SET token_enc = ? WHERE id = ?`, value, item.id); err != nil {
			return fmt.Errorf("store rotated API token %d: %w", item.id, err)
		}
	}
	return nil
}

// HashToken computes the SHA-256 digest representation before at-rest encryption.
func HashToken(token string) string {
	digest := sha256.Sum256([]byte(token))
	return hex.EncodeToString(digest[:])
}
