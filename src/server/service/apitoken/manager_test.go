// SPDX-License-Identifier: MIT
package apitoken

import (
	"context"
	"database/sql"
	"fmt"
	"github.com/apimgr/vidveil/src/common/dbstore"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/apimgr/vidveil/src/server/service/database"
	"github.com/google/uuid"
)

func storeExec(ctx context.Context, store *database.AppDatabase, query string, args ...any) error {
	return store.Write(ctx, func(tx *sql.Tx) error {
		_, err := tx.ExecContext(ctx, query, args...)
		return err
	})
}

func storeQueryRow(ctx context.Context, store *database.AppDatabase, query string, args []any, dest ...any) error {
	return store.Read(ctx, func(readCtx context.Context, db dbstore.Reader) error {
		return db.QueryRowContext(readCtx, query, args...).Scan(dest...)
	})
}

func TestCookieMaxAgeParsesConfiguredUnits(t *testing.T) {
	for value, want := range map[string]int{"": 365 * 24 * 60 * 60, "1y": 365 * 24 * 60 * 60, "30d": 30 * 24 * 60 * 60, "12h": 12 * 60 * 60, "invalid": 365 * 24 * 60 * 60} {
		if got := CookieMaxAge(value); got != want {
			t.Errorf("CookieMaxAge(%q) = %d, want %d", value, got, want)
		}
	}
	if got := CookieMaxAge("0d"); got != int((365 * 24 * time.Hour).Seconds()) {
		t.Errorf("CookieMaxAge(0d) = %d, want default lifetime", got)
	}
}

func TestManagerUsesPublicIDsForLegacyIntegerTokenKeys(t *testing.T) {
	path := filepath.Join(t.TempDir(), "server.db")
	legacy, err := database.NewSchemaManager(path)
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	if err := storeExec(ctx, legacy.GetAppDatabase(), `CREATE TABLE api_tokens (
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
	if err := legacy.EnsureSchema(); err != nil {
		_ = legacy.Close()
		t.Fatal(err)
	}
	manager, err := NewManager(legacy.GetAppDatabase(), strings.Repeat("ab", 32))
	if err != nil {
		_ = legacy.Close()
		t.Fatal(err)
	}
	issued, err := manager.IssuePrimary(context.Background())
	if err != nil {
		_ = legacy.Close()
		t.Fatal(err)
	}
	principal, err := manager.Authenticate(context.Background(), issued.Token)
	if err != nil {
		_ = legacy.Close()
		t.Fatal(err)
	}
	if principal.TokenID != issued.TokenID || principal.TokenID == "1" {
		_ = legacy.Close()
		t.Fatalf("legacy token identity = %q, want public ID %q", principal.TokenID, issued.TokenID)
	}
	var idType string
	if err := storeQueryRow(ctx, legacy.GetAppDatabase(), `SELECT type FROM pragma_table_info('api_tokens') WHERE name = 'id'`, nil, &idType); err != nil {
		_ = legacy.Close()
		t.Fatal(err)
	}
	if idType != "INTEGER" {
		_ = legacy.Close()
		t.Fatalf("legacy api_tokens.id type = %q, want INTEGER preserved", idType)
	}
	if err := legacy.Close(); err != nil {
		t.Fatal(err)
	}
}

func TestIssuePrimaryAndAuthenticate(t *testing.T) {
	schema, err := database.NewSchemaManager(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = schema.Close() }()
	if err := schema.EnsureSchema(); err != nil {
		t.Fatal(err)
	}
	manager, err := NewManager(schema.GetAppDatabase(), strings.Repeat("ab", 32))
	if err != nil {
		t.Fatal(err)
	}
	issued, err := manager.IssuePrimary(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(issued.Token, "own_") || len(issued.Token) != 36 {
		t.Fatalf("issued token format = %q", issued.Token)
	}
	tokenID, err := uuid.Parse(issued.TokenID)
	if err != nil || tokenID.Version() != 7 {
		t.Fatalf("issued token ID = %q, want UUIDv7 (parse error: %v)", issued.TokenID, err)
	}
	principal, err := manager.Authenticate(context.Background(), issued.Token)
	if err != nil {
		t.Fatal(err)
	}
	if principal.OwnerID != issued.OwnerID || principal.TokenID != issued.TokenID || principal.Kind != "primary" {
		t.Fatalf("principal = %+v, want issued owner and primary token", principal)
	}
	if _, err := manager.Authenticate(context.Background(), "tok_invalid"); err == nil {
		t.Fatal("Authenticate accepted an unknown token")
	}
}

func TestAuthenticateRejectsTokensAssignedToSystemOwner(t *testing.T) {
	schema, err := database.NewSchemaManager(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = schema.Close() }()
	if err := schema.EnsureSchema(); err != nil {
		t.Fatal(err)
	}
	manager, err := NewManager(schema.GetAppDatabase(), strings.Repeat("ab", 32))
	if err != nil {
		t.Fatal(err)
	}
	issued, err := manager.IssuePrimary(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if err := storeExec(context.Background(), schema.GetAppDatabase(), `UPDATE api_tokens SET owner_id = 'system' WHERE id = ?`, issued.TokenID); err != nil {
		t.Fatal(err)
	}
	if _, err := manager.Authenticate(context.Background(), issued.Token); err != ErrInvalidToken {
		t.Fatalf("system-owned token authentication error = %v, want invalid token", err)
	}
}

func TestAuthenticateAcceptsLegacyOwnerTokenPrefix(t *testing.T) {
	schema, err := database.NewSchemaManager(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = schema.Close() }()
	if err := schema.EnsureSchema(); err != nil {
		t.Fatal(err)
	}
	manager, err := NewManager(schema.GetAppDatabase(), strings.Repeat("ef", 32))
	if err != nil {
		t.Fatal(err)
	}
	issued, err := manager.IssuePrimary(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	legacyToken := "tok_0123456789abcdefghijkmnpqrstuvWX"
	hash, encrypted, err := manager.encrypt(legacyToken)
	if err != nil {
		t.Fatal(err)
	}
	if err := storeExec(context.Background(), schema.GetAppDatabase(), `UPDATE api_tokens SET token_hash = ?, token_prefix = ?, token_last4 = ?, token_enc = ? WHERE id = ?`, hash, legacyToken[:12], legacyToken[len(legacyToken)-4:], encrypted, issued.TokenID); err != nil {
		t.Fatal(err)
	}
	principal, err := manager.Authenticate(context.Background(), legacyToken)
	if err != nil {
		t.Fatalf("Authenticate rejected legacy token: %v", err)
	}
	if principal.OwnerID != issued.OwnerID || principal.TokenID != issued.TokenID {
		t.Fatalf("legacy token principal = %+v, want issued owner and token", principal)
	}
}

func TestLinkedTokenLifecycleRequiresPrimary(t *testing.T) {
	schema, err := database.NewSchemaManager(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = schema.Close() }()
	if err := schema.EnsureSchema(); err != nil {
		t.Fatal(err)
	}
	manager, err := NewManager(schema.GetAppDatabase(), strings.Repeat("cd", 32))
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	primaryToken, err := manager.IssuePrimary(ctx)
	if err != nil {
		t.Fatal(err)
	}
	primary, err := manager.Authenticate(ctx, primaryToken.Token)
	if err != nil {
		t.Fatal(err)
	}
	linkedToken, err := manager.CreateLinked(ctx, primary, "automation", nil, 25)
	if err != nil {
		t.Fatal(err)
	}
	linked, err := manager.Authenticate(ctx, linkedToken.Token)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := manager.CreateLinked(ctx, linked, "nested", nil, 25); err != ErrManagementForbidden {
		t.Fatalf("linked token management error = %v, want forbidden", err)
	}
	list, err := manager.ListLinked(ctx, primary)
	if err != nil {
		t.Fatal(err)
	}
	if len(list) != 1 || list[0].Name != "automation" || list[0].Suffix != linkedToken.Token[len(linkedToken.Token)-4:] {
		t.Fatalf("linked token list = %+v", list)
	}
	rotated, err := manager.Rotate(ctx, primary, primary.TokenID)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := manager.Authenticate(ctx, primaryToken.Token); err != ErrInvalidToken {
		t.Fatalf("old primary authentication error = %v, want invalid token", err)
	}
	if _, err := manager.Authenticate(ctx, linkedToken.Token); err != nil {
		t.Fatalf("linked token did not survive primary rotation: %v", err)
	}
	if _, err := manager.Authenticate(ctx, rotated.Token); err != nil {
		t.Fatalf("new primary token failed authentication: %v", err)
	}
	rotatedPrincipal, err := manager.Authenticate(ctx, rotated.Token)
	if err != nil {
		t.Fatal(err)
	}
	if err := manager.RevokeLinked(ctx, rotatedPrincipal, linkedToken.TokenID); err != nil {
		t.Fatal(err)
	}
	if _, err := manager.Authenticate(ctx, linkedToken.Token); err != ErrInvalidToken {
		t.Fatalf("revoked linked token authentication error = %v, want invalid token", err)
	}
	if _, err := manager.CreateLinked(ctx, rotatedPrincipal, "AUTOMATION", nil, 25); err != ErrDuplicateTokenName {
		t.Fatalf("reused revoked token name error = %v, want duplicate-name error", err)
	}
}

func TestPurgeIdleOwnersKeepsOwnersWithResources(t *testing.T) {
	schema, err := database.NewSchemaManager(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = schema.Close() }()
	if err := schema.EnsureSchema(); err != nil {
		t.Fatal(err)
	}
	manager, err := NewManager(schema.GetAppDatabase(), strings.Repeat("ab", 32))
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	resourceOwner, err := manager.IssuePrimary(ctx)
	if err != nil {
		t.Fatal(err)
	}
	idleOwner, err := manager.IssuePrimary(ctx)
	if err != nil {
		t.Fatal(err)
	}
	idlePrincipal, err := manager.Authenticate(ctx, idleOwner.Token)
	if err != nil {
		t.Fatal(err)
	}
	linked, err := manager.CreateLinked(ctx, idlePrincipal, "automation", nil, 25)
	if err != nil {
		t.Fatal(err)
	}
	if err := storeExec(ctx, schema.GetAppDatabase(), `INSERT INTO favorites (owner_id, visitor_id, url, title) VALUES (?, ?, ?, ?)`, resourceOwner.OwnerID, "test-visitor", "https://example.test/video", "saved"); err != nil {
		t.Fatal(err)
	}
	old := time.Now().Add(-60 * 24 * time.Hour).Unix()
	if err := storeExec(ctx, schema.GetAppDatabase(), `UPDATE owners SET last_seen_at = ? WHERE id IN (?, ?, 'system')`, old, resourceOwner.OwnerID, idleOwner.OwnerID); err != nil {
		t.Fatal(err)
	}
	if removed, err := manager.PurgeIdleOwners(ctx, 30); err != nil || removed != 1 {
		t.Fatalf("PurgeIdleOwners() = %d, %v; want 1 owner purged", removed, err)
	}
	if _, err := manager.Authenticate(ctx, resourceOwner.Token); err != nil {
		t.Fatalf("owner with a favorite was purged: %v", err)
	}
	if _, err := manager.Authenticate(ctx, idleOwner.Token); err != ErrInvalidToken {
		t.Fatalf("idle primary authentication error = %v, want invalid token", err)
	}
	if _, err := manager.Authenticate(ctx, linked.Token); err != ErrInvalidToken {
		t.Fatalf("idle linked authentication error = %v, want invalid token", err)
	}
	var systemOwnerCount int
	if err := storeQueryRow(ctx, schema.GetAppDatabase(), `SELECT COUNT(*) FROM owners WHERE id = 'system'`, nil, &systemOwnerCount); err != nil {
		t.Fatal(err)
	}
	if systemOwnerCount != 1 {
		t.Fatalf("reserved system owner count = %d, want 1 after idle purge", systemOwnerCount)
	}
}

func TestPurgeIdleOwnersDeletesLargeSetsInBoundedBatches(t *testing.T) {
	schema, err := database.NewSchemaManager(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = schema.Close() }()
	if err := schema.EnsureSchema(); err != nil {
		t.Fatal(err)
	}
	manager, err := NewManager(schema.GetAppDatabase(), strings.Repeat("ab", 32))
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	old := time.Now().Add(-60 * 24 * time.Hour).Unix()
	const ownerCount = 501
	if err := schema.GetAppDatabase().Write(ctx, func(tx *sql.Tx) error {
		for i := 0; i < ownerCount; i++ {
			id := fmt.Sprintf("idle-owner-%04d", i)
			if _, err := tx.ExecContext(ctx, `INSERT INTO owners (id, last_seen_at) VALUES (?, ?)`, id, old); err != nil {
				return err
			}
			if _, err := tx.ExecContext(ctx, `INSERT INTO api_tokens (id, owner_id, kind, scope, token_hash, token_prefix, token_last4, token_enc) VALUES (?, ?, 'primary', 'global', ?, 'own_test', 'XXXX', X'00')`, "token-"+id, id, "hash-"+id); err != nil {
				return err
			}
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	removed, err := manager.PurgeIdleOwners(ctx, 30)
	if err != nil || removed != ownerCount {
		t.Fatalf("PurgeIdleOwners() = %d, %v; want %d owners purged", removed, err, ownerCount)
	}
}

func TestRevokePrimaryConfirmsAndDeletesOwnerResourcesAndTokens(t *testing.T) {
	schema, err := database.NewSchemaManager(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = schema.Close() }()
	if err := schema.EnsureSchema(); err != nil {
		t.Fatal(err)
	}
	manager, err := NewManager(schema.GetAppDatabase(), strings.Repeat("ef", 32))
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	issued, err := manager.IssuePrimary(ctx)
	if err != nil {
		t.Fatal(err)
	}
	primary, err := manager.Authenticate(ctx, issued.Token)
	if err != nil {
		t.Fatal(err)
	}
	linked, err := manager.CreateLinked(ctx, primary, "automation", nil, 25)
	if err != nil {
		t.Fatal(err)
	}
	if err := storeExec(ctx, schema.GetAppDatabase(), `INSERT INTO favorites (owner_id, visitor_id, url, title) VALUES (?, ?, ?, ?)`, issued.OwnerID, "owner-test", "https://example.test/video", "saved"); err != nil {
		t.Fatal(err)
	}

	if _, err := manager.RevokePrimary(ctx, primary, "wrong"); err != ErrInvalidConfirmation {
		t.Fatalf("wrong confirmation error = %v, want ErrInvalidConfirmation", err)
	}
	var tokens, resources int
	if err := storeQueryRow(ctx, schema.GetAppDatabase(), `SELECT COUNT(*) FROM api_tokens WHERE owner_id = ?`, []any{issued.OwnerID}, &tokens); err != nil {
		t.Fatal(err)
	}
	if err := storeQueryRow(ctx, schema.GetAppDatabase(), `SELECT COUNT(*) FROM favorites WHERE owner_id = ?`, []any{issued.OwnerID}, &resources); err != nil {
		t.Fatal(err)
	}
	if tokens != 2 || resources != 1 {
		t.Fatalf("failed confirmation changed owner data: tokens=%d resources=%d", tokens, resources)
	}

	counts, err := manager.RevokePrimary(ctx, primary, issued.Token[len(issued.Token)-4:])
	if err != nil {
		t.Fatalf("RevokePrimary: %v", err)
	}
	if counts.Tokens != 2 || counts.Resources != 1 || counts.Prefix != primary.Prefix {
		t.Fatalf("deletion counts = %+v, want two tokens and one resource", counts)
	}
	if err := storeQueryRow(ctx, schema.GetAppDatabase(), `SELECT COUNT(*) FROM api_tokens WHERE owner_id = ?`, []any{issued.OwnerID}, &tokens); err != nil {
		t.Fatal(err)
	}
	if err := storeQueryRow(ctx, schema.GetAppDatabase(), `SELECT COUNT(*) FROM favorites WHERE owner_id = ?`, []any{issued.OwnerID}, &resources); err != nil {
		t.Fatal(err)
	}
	if tokens != 0 || resources != 0 {
		t.Fatalf("owner data remains after revocation: tokens=%d resources=%d", tokens, resources)
	}
	if _, err := manager.Authenticate(ctx, issued.Token); err != ErrInvalidToken {
		t.Fatalf("deleted primary authentication error = %v, want ErrInvalidToken", err)
	}
	if _, err := manager.Authenticate(ctx, linked.Token); err != ErrInvalidToken {
		t.Fatalf("deleted linked-token authentication error = %v, want ErrInvalidToken", err)
	}
}
