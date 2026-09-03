// SPDX-License-Identifier: MIT
// AI.md PART 28: Coverage tests for openLibSQL driver selection. sql.Open for
// the libsql driver succeeds without a real server (driver registration only);
// actual queries are not made.
package database

import (
	"testing"
)

// ── openLibSQL ────────────────────────────────────────────────────────────────

func TestNewAppDatabase_LibSQL_EmptyURL_ReturnsError(t *testing.T) {
	cfg := DatabaseConfig{Driver: DriverLibSQL}
	_, err := NewAppDatabase(cfg)
	if err == nil {
		t.Fatal("NewAppDatabase libsql without URL: expected error, got nil")
	}
}

func TestNewAppDatabase_LibSQL_Opens(t *testing.T) {
	cfg := DatabaseConfig{
		Driver: DriverLibSQL,
		URL:    "libsql://example.turso.io",
	}
	db, err := NewAppDatabase(cfg)
	if err != nil {
		t.Fatalf("NewAppDatabase libsql: unexpected error: %v", err)
	}
	defer db.Close()
	if db.Driver() != DriverLibSQL {
		t.Errorf("Driver() = %v, want %v", db.Driver(), DriverLibSQL)
	}
}

func TestNewAppDatabase_LibSQL_TursoAlias(t *testing.T) {
	cfg := DatabaseConfig{
		Driver: "turso",
		URL:    "libsql://example.turso.io",
	}
	db, err := NewAppDatabase(cfg)
	if err != nil {
		t.Fatalf("NewAppDatabase turso alias: %v", err)
	}
	defer db.Close()
	if db.Driver() != DriverLibSQL {
		t.Errorf("Driver() = %v, want %v", db.Driver(), DriverLibSQL)
	}
}

func TestNewAppDatabase_LibSQL_WithToken_Opens(t *testing.T) {
	// Token is appended as ?authToken= when the URL has no query string
	cfg := DatabaseConfig{
		Driver: DriverLibSQL,
		URL:    "libsql://example.turso.io",
		Token:  "test-token",
	}
	db, err := NewAppDatabase(cfg)
	if err != nil {
		t.Fatalf("NewAppDatabase libsql with token: %v", err)
	}
	defer db.Close()
}

func TestNewAppDatabase_LibSQL_TokenWithExistingQuery_Opens(t *testing.T) {
	// Token is appended as &authToken= when the URL already has a query string
	cfg := DatabaseConfig{
		Driver: DriverLibSQL,
		URL:    "libsql://example.turso.io?tls=1",
		Token:  "test-token",
	}
	db, err := NewAppDatabase(cfg)
	if err != nil {
		t.Fatalf("NewAppDatabase libsql token with query: %v", err)
	}
	defer db.Close()
}

func TestNewAppDatabase_LibSQL_TokenAlreadyInURL_Opens(t *testing.T) {
	// Token is NOT re-appended when authToken= is already present in the URL
	cfg := DatabaseConfig{
		Driver: DriverLibSQL,
		URL:    "libsql://example.turso.io?authToken=existing",
		Token:  "ignored-token",
	}
	db, err := NewAppDatabase(cfg)
	if err != nil {
		t.Fatalf("NewAppDatabase libsql token in URL: %v", err)
	}
	defer db.Close()
}

func TestNewAppDatabase_UnknownDriver_ReturnsError(t *testing.T) {
	cfg := DatabaseConfig{Driver: "oracle"}
	_, err := NewAppDatabase(cfg)
	if err == nil {
		t.Error("NewAppDatabase(oracle): expected error, got nil")
	}
}
