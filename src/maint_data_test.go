// SPDX-License-Identifier: MIT
package main

import (
	"encoding/hex"
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"

	"github.com/apimgr/vidveil/src/server/service/database"
	"github.com/google/uuid"
)

func newDataTestDB(t *testing.T) *database.SchemaManager {
	t.Helper()
	mgr, err := database.NewSchemaManager(filepath.Join(t.TempDir(), "server.db"))
	if err != nil {
		t.Fatalf("NewSchemaManager: %v", err)
	}
	t.Cleanup(func() { _ = mgr.Close() })
	if err := mgr.EnsureSchema(); err != nil {
		t.Fatalf("EnsureSchema: %v", err)
	}
	return mgr
}

func TestReadOwnerFavoritesIsScopedToOwner(t *testing.T) {
	mgr := newDataTestDB(t)
	db := mgr.GetDB()
	for _, owner := range []string{"owner-a", "owner-b"} {
		if _, err := db.Exec(`INSERT INTO owners (id) VALUES (?)`, owner); err != nil {
			t.Fatal(err)
		}
		if _, err := db.Exec(`INSERT INTO favorites (owner_id, visitor_id, url, title) VALUES (?, ?, ?, ?)`, owner, owner, "https://example.test/"+owner, owner); err != nil {
			t.Fatal(err)
		}
	}
	rows, err := readOwnerFavorites(db, "owner-a")
	if err != nil {
		t.Fatalf("readOwnerFavorites: %v", err)
	}
	if len(rows) != 1 || rows[0]["title"] != "owner-a" {
		t.Fatalf("owner favorites = %#v, want only owner-a's favorite", rows)
	}
	if _, exists := rows[0]["owner_id"]; exists {
		t.Fatal("favorite export contains an implementation-only owner_id field")
	}
}

func TestResolveOwnerIDByUniqueTokenPrefix(t *testing.T) {
	mgr := newDataTestDB(t)
	db := mgr.GetDB()
	for _, owner := range []string{"owner-a", "owner-b"} {
		if _, err := db.Exec(`INSERT INTO owners (id) VALUES (?)`, owner); err != nil {
			t.Fatal(err)
		}
	}
	for _, token := range []struct{ owner, prefix string }{{"owner-a", "own_abc123"}, {"owner-b", "own_def456"}} {
		id, err := uuid.NewV7()
		if err != nil {
			t.Fatal(err)
		}
		hashPrefix := token.owner[len(token.owner)-1:]
		if _, err := db.Exec(`INSERT INTO api_tokens (id, owner_id, kind, token_hash, token_prefix, token_last4, token_enc) VALUES (?, ?, 'linked', ?, ?, '', '')`, id.String(), token.owner, strings.Repeat(hashPrefix, 64), token.prefix); err != nil {
			t.Fatal(err)
		}
	}
	got, err := resolveOwnerID(db, "own_abc")
	if err != nil || got != "owner-a" {
		t.Fatalf("resolveOwnerID() = (%q, %v), want (owner-a, nil)", got, err)
	}
	if _, err := resolveOwnerID(db, "own_"); err == nil {
		t.Fatal("ambiguous token prefix unexpectedly resolved")
	}
}

func TestOwnerTokenExportContainsNoCredentialMaterial(t *testing.T) {
	mgr := newDataTestDB(t)
	db := mgr.GetDB()
	if _, err := db.Exec(`INSERT INTO owners (id) VALUES ('owner-a')`); err != nil {
		t.Fatal(err)
	}
	id, err := uuid.NewV7()
	if err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`INSERT INTO api_tokens (id, owner_id, kind, token_hash, token_prefix, token_last4, token_enc) VALUES (?, 'owner-a', 'linked', ?, 'own_abc123', '1234', 'secret-token')`, id.String(), strings.Repeat("b", 64)); err != nil {
		t.Fatal(err)
	}
	tokens, err := readOwnerTokens(db, "owner-a")
	if err != nil {
		t.Fatalf("readOwnerTokens: %v", err)
	}
	encoded, err := json.Marshal(tokens)
	if err != nil {
		t.Fatal(err)
	}
	for _, secret := range []string{"secret-hash", "secret-token", "token_hash", "token_enc"} {
		if strings.Contains(string(encoded), secret) {
			t.Errorf("token export contains sensitive value %q: %s", secret, encoded)
		}
	}
}

func TestDeleteOwnerResourcesOnlyDeletesSelectedOwner(t *testing.T) {
	mgr := newDataTestDB(t)
	db := mgr.GetDB()
	for _, owner := range []string{"owner-a", "owner-b"} {
		if _, err := db.Exec(`INSERT INTO owners (id) VALUES (?)`, owner); err != nil {
			t.Fatal(err)
		}
		if _, err := db.Exec(`INSERT INTO favorites (owner_id, visitor_id, url, title) VALUES (?, ?, ?, ?)`, owner, owner, "https://example.test/"+owner, owner); err != nil {
			t.Fatal(err)
		}
	}
	tx, err := db.Begin()
	if err != nil {
		t.Fatal(err)
	}
	count, err := deleteOwnerResources(t.Context(), tx, "owner-a")
	if err != nil {
		t.Fatal(err)
	}
	if count != 1 {
		t.Fatalf("deleted count = %d, want 1", count)
	}
	if err := tx.Commit(); err != nil {
		t.Fatal(err)
	}
	var remaining int
	if err := db.QueryRow(`SELECT COUNT(*) FROM favorites`).Scan(&remaining); err != nil {
		t.Fatal(err)
	}
	if remaining != 1 {
		t.Fatalf("remaining favorites = %d, want owner's other row to remain", remaining)
	}
}

func TestNewDataRequestIDIsUniqueHex(t *testing.T) {
	first, err := newDataRequestID()
	if err != nil {
		t.Fatalf("newDataRequestID: %v", err)
	}
	second, err := newDataRequestID()
	if err != nil {
		t.Fatalf("newDataRequestID: %v", err)
	}
	if len(first) != 32 {
		t.Errorf("request ID length = %d, want 32 hex characters", len(first))
	}
	if _, err := hex.DecodeString(first); err != nil {
		t.Errorf("request ID %q is not hex: %v", first, err)
	}
	if first == second {
		t.Error("consecutive request IDs are equal")
	}
}

func TestExportTableQueryOnlyAllowsOwnerResources(t *testing.T) {
	query, ok := exportTableQuery("favorites")
	if !ok || !strings.Contains(query, "WHERE owner_id = ?") {
		t.Fatalf("favorites query = (%q, %t), want owner-scoped query", query, ok)
	}
	if _, ok := exportTableQuery("api_tokens"); ok {
		t.Fatal("token table must not be exported through generic resource queries")
	}
}
