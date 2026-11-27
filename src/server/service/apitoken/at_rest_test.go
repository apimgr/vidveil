// SPDX-License-Identifier: MIT
package apitoken

import (
	"context"
	"database/sql"
	"github.com/apimgr/vidveil/src/common/dbstore"
	"strings"
	"testing"

	"github.com/apimgr/vidveil/src/server/service/secret"
	_ "modernc.org/sqlite"
)

type rawTokenTestStore struct{ db *sql.DB }

type tokenTestReader struct{ db *sql.DB }
type tokenTestRow struct{ row *sql.Row }

func (r tokenTestReader) QueryContext(ctx context.Context, query string, args ...any) (*sql.Rows, error) {
	return r.db.QueryContext(ctx, query, args...)
}
func (r tokenTestReader) QueryRowContext(ctx context.Context, query string, args ...any) dbstore.RowScanner {
	return tokenTestRow{row: r.db.QueryRowContext(ctx, query, args...)}
}
func (r tokenTestReader) PingContext(ctx context.Context) error { return r.db.PingContext(ctx) }
func (r tokenTestRow) Scan(dest ...any) error                   { return r.row.Scan(dest...) }

func (s rawTokenTestStore) Read(ctx context.Context, fn func(context.Context, dbstore.Reader) error) error {
	return fn(ctx, tokenTestReader(s))
}

func (s rawTokenTestStore) Write(ctx context.Context, fn func(*sql.Tx) error) error {
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

var _ secret.Store = rawTokenTestStore{}

func TestEncryptDecryptHash(t *testing.T) {
	key := strings.Repeat("01", 32)
	hash := HashToken("own_0123456789abcdefghijkmnpqrstuv")
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
	token := "own_" + strings.Repeat("a", 32)
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

func TestEncryptDecryptLegacyOwnerToken(t *testing.T) {
	key := strings.Repeat("06", 32)
	const token = "tok_aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
	stored, err := EncryptToken(key, token)
	if err != nil {
		t.Fatal(err)
	}
	got, err := DecryptToken(key, stored)
	if err != nil || got != token {
		t.Fatalf("DecryptToken() = (%q, %v), want legacy credential", got, err)
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
	if err := NormalizeLegacyHashes(ctx, rawTokenTestStore{db}, strings.Repeat("01", 32)); err != nil {
		t.Fatal(err)
	}
	var first string
	if err := db.QueryRow(`SELECT token_hash FROM api_tokens WHERE id = 1`).Scan(&first); err != nil {
		t.Fatal(err)
	}
	if first != hash {
		t.Fatal("plaintext SHA-256 digest was changed")
	}
	if err := NormalizeLegacyHashes(ctx, rawTokenTestStore{db}, strings.Repeat("01", 32)); err != nil {
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
