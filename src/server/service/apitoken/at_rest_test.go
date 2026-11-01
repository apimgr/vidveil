// SPDX-License-Identifier: MIT
package apitoken

import (
	"context"
	"database/sql"
	"strings"
	"testing"

	_ "modernc.org/sqlite"
)

func TestEncryptDecryptHash(t *testing.T) {
	key := strings.Repeat("01", 32)
	hash := HashToken("tok_private-value")
	stored, err := EncryptHash(key, hash)
	if err != nil {
		t.Fatal(err)
	}
	if stored == hash || strings.Contains(stored, hash) {
		t.Fatalf("stored digest exposes plaintext hash: %q", stored)
	}
	got, err := DecryptHash(key, stored)
	if err != nil {
		t.Fatal(err)
	}
	if got != hash {
		t.Fatalf("DecryptHash() = %q, want %q", got, hash)
	}
}

func TestEncryptDecryptToken(t *testing.T) {
	key := strings.Repeat("04", 32)
	token := "tok_0123456789abcdefghijkmnpqrstuv"
	stored, err := EncryptToken(key, token)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(stored, token) {
		t.Fatal("encrypted token contains plaintext")
	}
	got, err := DecryptToken(key, stored)
	if err != nil {
		t.Fatal(err)
	}
	if got != token {
		t.Fatalf("DecryptToken() = %q, want original token", got)
	}
	if _, err := DecryptToken(strings.Repeat("05", 32), stored); err == nil {
		t.Fatal("DecryptToken() accepted the wrong key")
	}
}

func TestNormalizeLegacyHashesAndRotation(t *testing.T) {
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = db.Close() }()
	if _, err := db.Exec(`CREATE TABLE api_tokens (id INTEGER PRIMARY KEY, token_hash TEXT NOT NULL)`); err != nil {
		t.Fatal(err)
	}
	hash := HashToken("tok_legacy")
	legacyEncrypted, err := EncryptHash(strings.Repeat("01", 32), HashToken("tok_legacy_encrypted"))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`INSERT INTO api_tokens (token_hash) VALUES (?), (?)`, hash, legacyEncrypted); err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	oldKey := strings.Repeat("02", 32)
	newKey := strings.Repeat("03", 32)
	if err := NormalizeLegacyHashes(ctx, db, strings.Repeat("01", 32)); err != nil {
		t.Fatal(err)
	}
	var first string
	if err := db.QueryRow(`SELECT token_hash FROM api_tokens WHERE id = 1`).Scan(&first); err != nil {
		t.Fatal(err)
	}
	if first != hash {
		t.Fatal("plaintext SHA-256 digest was changed")
	}
	if err := NormalizeLegacyHashes(ctx, db, strings.Repeat("01", 32)); err != nil {
		t.Fatalf("idempotent digest migration: %v", err)
	}
	var second string
	if err := db.QueryRow(`SELECT token_hash FROM api_tokens WHERE id = 2`).Scan(&second); err != nil {
		t.Fatal(err)
	}
	if second != HashToken("tok_legacy_encrypted") {
		t.Fatal("legacy encrypted digest was not normalized")
	}
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		t.Fatal(err)
	}
	if err := ReencryptHashes(ctx, tx, oldKey, newKey); err != nil {
		_ = tx.Rollback()
		t.Fatal(err)
	}
	if err := tx.Commit(); err != nil {
		t.Fatal(err)
	}
	var rotated string
	if err := db.QueryRow(`SELECT token_hash FROM api_tokens WHERE id = 1`).Scan(&rotated); err != nil {
		t.Fatal(err)
	}
	if rotated != hash {
		t.Fatalf("normalized digest = %q, want %q", rotated, hash)
	}
}
