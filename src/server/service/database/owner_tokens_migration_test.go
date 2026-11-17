// SPDX-License-Identifier: MIT
package database

import (
	"path/filepath"
	"testing"
)

func TestEnsureSchemaPreservesLegacyTokensAsOwners(t *testing.T) {
	path := filepath.Join(t.TempDir(), "server.db")
	legacy, err := NewSchemaManager(path)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := legacy.GetDB().Exec(`CREATE TABLE api_tokens (
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
	if _, err := legacy.GetDB().Exec(`INSERT INTO api_tokens (token_hash, token_prefix, resource_type, resource_id) VALUES ('hash', 'tok_legacy', 'favorite', 'old-resource')`); err != nil {
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
	var ownerID, kind, name, encrypted string
	if err := migrated.GetDB().QueryRow(`SELECT owner_id, kind, COALESCE(name, ''), token_enc FROM api_tokens WHERE token_prefix = 'tok_legacy'`).Scan(&ownerID, &kind, &name, &encrypted); err != nil {
		t.Fatal(err)
	}
	if ownerID == "" || kind != "linked" || name != "" || encrypted != "" {
		t.Fatalf("migrated token = owner %q, kind %q, name %q, encrypted %q", ownerID, kind, name, encrypted)
	}
	var owners int
	if err := migrated.GetDB().QueryRow(`SELECT COUNT(*) FROM owners WHERE id = ?`, ownerID).Scan(&owners); err != nil {
		t.Fatal(err)
	}
	if owners != 1 {
		t.Fatalf("legacy owner count = %d, want 1", owners)
	}
}

func TestEnsureSchemaPreservesLegacyFavoritesAsOwnedResources(t *testing.T) {
	path := filepath.Join(t.TempDir(), "server.db")
	legacy, err := NewSchemaManager(path)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := legacy.GetDB().Exec(`CREATE TABLE favorites (
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
	if _, err := legacy.GetDB().Exec(`INSERT INTO favorites (visitor_id, url, title) VALUES ('old-visitor', 'https://example.test/video', 'Saved video')`); err != nil {
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
	if err := migrated.GetDB().QueryRow(`SELECT owner_id, visitor_id, url, title FROM favorites WHERE url = 'https://example.test/video'`).Scan(&ownerID, &visitorID, &url, &title); err != nil {
		t.Fatal(err)
	}
	if ownerID != "legacy-favorite-old-visitor" || visitorID != ownerID || url != "https://example.test/video" || title != "Saved video" {
		t.Fatalf("migrated favorite = owner %q, visitor %q, url %q, title %q", ownerID, visitorID, url, title)
	}
	var owners int
	if err := migrated.GetDB().QueryRow(`SELECT COUNT(*) FROM owners WHERE id = ?`, ownerID).Scan(&owners); err != nil {
		t.Fatal(err)
	}
	if owners != 1 {
		t.Fatalf("legacy favorite owner count = %d, want 1", owners)
	}
}

func TestEnsureSchemaDeduplicatesFavoritesPerOwner(t *testing.T) {
	path := filepath.Join(t.TempDir(), "server.db")
	schema, err := NewSchemaManager(path)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := schema.GetDB().Exec(`CREATE TABLE owners (id TEXT PRIMARY KEY)`); err != nil {
		_ = schema.Close()
		t.Fatal(err)
	}
	if _, err := schema.GetDB().Exec(`INSERT INTO owners (id) VALUES ('owner-a')`); err != nil {
		_ = schema.Close()
		t.Fatal(err)
	}
	if _, err := schema.GetDB().Exec(`CREATE TABLE favorites (
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
	if _, err := schema.GetDB().Exec(`INSERT INTO favorites (owner_id, visitor_id, url, title) VALUES
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
	if err := migrated.GetDB().QueryRow(`SELECT COUNT(*) FROM favorites WHERE owner_id = 'owner-a' AND url = 'https://example.test/video'`).Scan(&count); err != nil {
		t.Fatal(err)
	}
	if count != 1 {
		t.Fatalf("favorite count = %d, want one per owner and URL", count)
	}
	var visitorID string
	if err := migrated.GetDB().QueryRow(`SELECT visitor_id FROM favorites WHERE owner_id = 'owner-a' AND url = 'https://example.test/video'`).Scan(&visitorID); err != nil {
		t.Fatal(err)
	}
	if visitorID != "owner-a" {
		t.Fatalf("canonical favorite visitor ID = %q, want owner-a", visitorID)
	}
}
