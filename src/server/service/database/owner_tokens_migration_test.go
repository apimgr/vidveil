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
	var ownerID, url, title string
	if err := migrated.GetDB().QueryRow(`SELECT owner_id, url, title FROM favorites WHERE visitor_id = 'old-visitor'`).Scan(&ownerID, &url, &title); err != nil {
		t.Fatal(err)
	}
	if ownerID != "legacy-favorite-old-visitor" || url != "https://example.test/video" || title != "Saved video" {
		t.Fatalf("migrated favorite = owner %q, url %q, title %q", ownerID, url, title)
	}
	var owners int
	if err := migrated.GetDB().QueryRow(`SELECT COUNT(*) FROM owners WHERE id = ?`, ownerID).Scan(&owners); err != nil {
		t.Fatal(err)
	}
	if owners != 1 {
		t.Fatalf("legacy favorite owner count = %d, want 1", owners)
	}
}
