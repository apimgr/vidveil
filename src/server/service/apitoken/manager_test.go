// SPDX-License-Identifier: MIT
package apitoken

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/apimgr/vidveil/src/server/service/database"
)

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

func TestIssuePrimaryAndAuthenticate(t *testing.T) {
	schema, err := database.NewSchemaManager(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = schema.Close() }()
	if err := schema.EnsureSchema(); err != nil {
		t.Fatal(err)
	}
	manager, err := NewManager(schema.GetDB(), strings.Repeat("ab", 32))
	if err != nil {
		t.Fatal(err)
	}
	issued, err := manager.IssuePrimary(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(issued.Token, "tok_") || len(issued.Token) != 36 {
		t.Fatalf("issued token format = %q", issued.Token)
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

func TestLinkedTokenLifecycleRequiresPrimary(t *testing.T) {
	schema, err := database.NewSchemaManager(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = schema.Close() }()
	if err := schema.EnsureSchema(); err != nil {
		t.Fatal(err)
	}
	manager, err := NewManager(schema.GetDB(), strings.Repeat("cd", 32))
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
	manager, err := NewManager(schema.GetDB(), strings.Repeat("ab", 32))
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
	if _, err := schema.GetDB().Exec(`INSERT INTO favorites (owner_id, visitor_id, url, title) VALUES (?, ?, ?, ?)`, resourceOwner.OwnerID, "test-visitor", "https://example.test/video", "saved"); err != nil {
		t.Fatal(err)
	}
	old := time.Now().Add(-60 * 24 * time.Hour).Unix()
	if _, err := schema.GetDB().Exec(`UPDATE owners SET last_seen_at = ? WHERE id IN (?, ?)`, old, resourceOwner.OwnerID, idleOwner.OwnerID); err != nil {
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
}
