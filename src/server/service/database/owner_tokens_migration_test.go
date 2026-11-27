// SPDX-License-Identifier: MIT
package database

import (
	"path/filepath"
	"testing"

	"github.com/google/uuid"
)

func TestEnsureSchemaPreservesLegacyTokensAsOwners(t *testing.T) {
	path := filepath.Join(t.TempDir(), "server.db")
	legacy, err := NewSchemaManager(path)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := legacy.db.Exec(`CREATE TABLE api_tokens (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		token_hash TEXT NOT NULL UNIQUE,
		token_prefix TEXT NOT NULL,
		resource_type TEXT NOT NULL,
		resource_id TEXT NOT NULL,
		created_at INTEGER NOT NULL DEFAULT (strftime('%s', 'now')),
		expires_at INTEGER,
		last_used_at INTEGER,
		revoked_at INTEGER,
		revoked_reason TEXT
	)`); err != nil {
		_ = legacy.Close()
		t.Fatal(err)
	}
	if _, err := legacy.db.Exec(`INSERT INTO api_tokens (token_hash, token_prefix, resource_type, resource_id) VALUES ('hash', 'tok_legacy', 'favorite', 'old-resource')`); err != nil {
		_ = legacy.Close()
		t.Fatal(err)
	}
	if err := legacy.Close(); err != nil {
		t.Fatal(err)
	}

	migrated, err := NewSchemaManager(path)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = migrated.Close() }()
	if err := migrated.EnsureSchema(); err != nil {
		t.Fatal(err)
	}
	var id, publicID, ownerID, kind, name, encrypted string
	if err := migrated.db.QueryRow(`SELECT CAST(id AS TEXT), public_id, owner_id, kind, COALESCE(name, ''), token_enc FROM api_tokens WHERE token_prefix = 'tok_legacy'`).Scan(&id, &publicID, &ownerID, &kind, &name, &encrypted); err != nil {
		t.Fatal(err)
	}
	parsedID, err := uuid.Parse(publicID)
	if err != nil || parsedID.Version() != 7 {
		t.Fatalf("migrated token public_id = %q, want UUIDv7", publicID)
	}
	if id != "1" {
		t.Fatalf("legacy integer token ID = %q, want preserved value 1", id)
	}
	if ownerID == "" || kind != "linked" || name != "" || encrypted != "" {
		t.Fatalf("migrated token = owner %q, kind %q, name %q, encrypted %q", ownerID, kind, name, encrypted)
	}
	var owners int
	if err := migrated.db.QueryRow(`SELECT COUNT(*) FROM owners WHERE id = ?`, ownerID).Scan(&owners); err != nil {
		t.Fatal(err)
	}
	if owners != 1 {
		t.Fatalf("legacy owner count = %d, want 1", owners)
	}
	columns, idType, err := apiTokenColumns(t.Context(), migrated.db)
	if err != nil {
		t.Fatal(err)
	}
	if idType != "INTEGER" || !columns["public_id"] || !columns["token_last4"] || !columns["resource_type"] || !columns["resource_id"] {
		t.Fatalf("migrated API-token schema: id type %q, columns %#v", idType, columns)
	}
}

func TestEnsureSchemaPreservesLegacyFavoritesAsOwnedResources(t *testing.T) {
	path := filepath.Join(t.TempDir(), "server.db")
	legacy, err := NewSchemaManager(path)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := legacy.db.Exec(`CREATE TABLE favorites (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		visitor_id TEXT NOT NULL,
		url TEXT NOT NULL,
		title TEXT NOT NULL,
		thumbnail TEXT,
		source TEXT,
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
		UNIQUE(visitor_id, url)
	)`); err != nil {
		_ = legacy.Close()
		t.Fatal(err)
	}
	if _, err := legacy.db.Exec(`INSERT INTO favorites (visitor_id, url, title) VALUES ('old-visitor', 'https://example.test/video', 'Saved video')`); err != nil {
		_ = legacy.Close()
		t.Fatal(err)
	}
	if err := legacy.Close(); err != nil {
		t.Fatal(err)
	}

	migrated, err := NewSchemaManager(path)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = migrated.Close() }()
	if err := migrated.EnsureSchema(); err != nil {
		t.Fatal(err)
	}
	var ownerID, visitorID, url, title string
	if err := migrated.db.QueryRow(`SELECT owner_id, visitor_id, url, title FROM favorites WHERE url = 'https://example.test/video'`).Scan(&ownerID, &visitorID, &url, &title); err != nil {
		t.Fatal(err)
	}
	if ownerID != "system" || visitorID != "old-visitor" || url != "https://example.test/video" || title != "Saved video" {
		t.Fatalf("migrated favorite = owner %q, visitor %q, url %q, title %q", ownerID, visitorID, url, title)
	}
	var owners int
	if err := migrated.db.QueryRow(`SELECT COUNT(*) FROM owners WHERE id = ?`, ownerID).Scan(&owners); err != nil {
		t.Fatal(err)
	}
	if owners != 1 {
		t.Fatalf("legacy favorite owner count = %d, want 1", owners)
	}
}

func TestEnsureSchemaPreservesFavoritesWithSameOwnerAndURL(t *testing.T) {
	path := filepath.Join(t.TempDir(), "server.db")
	schema, err := NewSchemaManager(path)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := schema.db.Exec(`CREATE TABLE owners (id TEXT PRIMARY KEY)`); err != nil {
		_ = schema.Close()
		t.Fatal(err)
	}
	if _, err := schema.db.Exec(`INSERT INTO owners (id) VALUES ('owner-a')`); err != nil {
		_ = schema.Close()
		t.Fatal(err)
	}
	if _, err := schema.db.Exec(`CREATE TABLE favorites (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		owner_id TEXT NOT NULL,
		visitor_id TEXT NOT NULL,
		url TEXT NOT NULL,
		title TEXT NOT NULL,
		thumbnail TEXT,
		source TEXT,
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
		UNIQUE(visitor_id, url)
	)`); err != nil {
		_ = schema.Close()
		t.Fatal(err)
	}
	if _, err := schema.db.Exec(`INSERT INTO favorites (owner_id, visitor_id, url, title) VALUES
		('owner-a', 'legacy-visitor', 'https://example.test/video', 'First'),
		('owner-a', 'owner-a', 'https://example.test/video', 'Duplicate')`); err != nil {
		_ = schema.Close()
		t.Fatal(err)
	}
	if err := schema.Close(); err != nil {
		t.Fatal(err)
	}

	migrated, err := NewSchemaManager(path)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = migrated.Close() }()
	if err := migrated.EnsureSchema(); err != nil {
		t.Fatal(err)
	}
	var count int
	if err := migrated.db.QueryRow(`SELECT COUNT(*) FROM favorites WHERE owner_id = 'owner-a' AND url = 'https://example.test/video'`).Scan(&count); err != nil {
		t.Fatal(err)
	}
	if count != 2 {
		t.Fatalf("favorite count = %d, want both legacy rows preserved", count)
	}
	var legacyVisitorCount int
	if err := migrated.db.QueryRow(`SELECT COUNT(*) FROM favorites WHERE owner_id = 'owner-a' AND url = 'https://example.test/video' AND visitor_id IN ('legacy-visitor', 'owner-a')`).Scan(&legacyVisitorCount); err != nil {
		t.Fatal(err)
	}
	if legacyVisitorCount != 2 {
		t.Fatalf("preserved legacy visitor identifiers = %d, want 2", legacyVisitorCount)
	}
}
