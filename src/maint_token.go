// SPDX-License-Identifier: MIT
// AI.md "API Tokens (server-generated resource owner tokens)" -> "Operator
// revocation": token lifecycle is operator-only through the CLI.
package main

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"text/tabwriter"
	"time"

	"github.com/apimgr/vidveil/src/common/terminal"
)

// handleMaintenanceToken implements --maintenance token list|revoke <prefix>.
func handleMaintenanceToken(arg, configDir, dataDir string) error {
	fields := strings.Fields(arg)
	binaryName := filepath.Base(os.Args[0])
	usage := fmt.Sprintf("Usage: %s --maintenance token list\n       %s --maintenance token revoke <prefix>",
		binaryName, binaryName)
	if len(fields) == 0 {
		return fmt.Errorf("missing token action\n   %s", usage)
	}
	switch fields[0] {
	case "list":
		if len(fields) != 1 {
			return fmt.Errorf("token list takes no arguments\n   %s", usage)
		}
	case "revoke":
		if len(fields) != 2 {
			return fmt.Errorf("token revoke requires exactly one token prefix\n   %s", usage)
		}
	default:
		return fmt.Errorf("unknown token action %q\n   %s", fields[0], usage)
	}

	// Both actions are operator-only per AI.md "Token revocation is operator-only
	// via CLI" — no empty-database exception applies.
	if err := authorizeSensitiveOperation(configDir, dataDir); err != nil {
		return err
	}

	mgr, err := openSecretDB(dataDir)
	if err != nil {
		return fmt.Errorf("open database: %w", err)
	}
	defer func() { _ = mgr.Close() }()

	if fields[0] == "list" {
		return listAPITokens(mgr.GetDB())
	}
	return revokeAPIToken(mgr.GetDB(), fields[1])
}

// listAPITokens prints metadata and status without exposing token values.
func listAPITokens(db *sql.DB) error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	rows, err := db.QueryContext(ctx, `
		SELECT token_prefix,
		       kind,
	       COALESCE(owner_id, ''),
	       COALESCE(name, ''),
		       created_at,
		       expires_at,
		       last_used_at,
		       revoked_at
		FROM api_tokens
		ORDER BY created_at DESC`)
	if err != nil {
		return fmt.Errorf("query tokens: %w", err)
	}
	defer func() { _ = rows.Close() }()

	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	defer func() { _ = w.Flush() }()
	_, _ = fmt.Fprintln(w, "PREFIX\tKIND\tOWNER\tNAME\tCREATED\tEXPIRES\tLAST USED\tSTATUS")

	count := 0
	for rows.Next() {
		var prefix, kind, ownerID, name string
		var createdAt int64
		var expiresAt, lastUsedAt, revokedAt sql.NullInt64
		if err := rows.Scan(&prefix, &kind, &ownerID, &name, &createdAt,
			&expiresAt, &lastUsedAt, &revokedAt); err != nil {
			return fmt.Errorf("scan token: %w", err)
		}
		status := "active"
		if revokedAt.Valid {
			status = "revoked"
		} else if expiresAt.Valid && expiresAt.Int64 <= time.Now().Unix() {
			status = "expired"
		}
		_, _ = fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\t%s\t%s\t%s\n",
			prefix,
			kind,
			ownerID,
			name,
			formatUnixTime(createdAt),
			formatOptionalUnixTime(expiresAt),
			formatOptionalUnixTime(lastUsedAt),
			status)
		count++
	}
	if err := rows.Err(); err != nil {
		return fmt.Errorf("read tokens: %w", err)
	}
	if count == 0 {
		fmt.Println("No API tokens.")
	}
	return nil
}

// revokeAPIToken revokes one linked token or an entire owner via its primary.
func revokeAPIToken(db *sql.DB, prefix string) error {
	readCtx, cancelRead := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancelRead()

	// Resolve the prefix to a single token first so an ambiguous or unknown
	// prefix fails loudly instead of revoking more than the operator asked for.
	rows, err := db.QueryContext(readCtx, `
		SELECT id, COALESCE(owner_id, ''), kind FROM api_tokens
		WHERE token_prefix = ? AND revoked_at IS NULL
		ORDER BY created_at DESC`, prefix)
	if err != nil {
		return fmt.Errorf("lookup token: %w", err)
	}
	defer func() { _ = rows.Close() }()
	var id int64
	var ownerID, kind string
	if rows.Next() {
		if err := rows.Scan(&id, &ownerID, &kind); err != nil {
			return fmt.Errorf("scan token: %w", err)
		}
	} else if err := rows.Err(); err != nil {
		return fmt.Errorf("read token: %w", err)
	} else {
		return fmt.Errorf("no active token with prefix %q", prefix)
	}
	if rows.Next() {
		return fmt.Errorf("multiple active tokens share prefix %q; refusing to revoke an arbitrary token", prefix)
	}
	if err := rows.Err(); err != nil {
		return fmt.Errorf("read token: %w", err)
	}
	if err := rows.Close(); err != nil {
		return fmt.Errorf("close token lookup: %w", err)
	}

	if !promptYesNo(fmt.Sprintf("Revoke token %s? [y/N]: ", prefix)) {
		return fmt.Errorf("revocation cancelled")
	}

	writeCtx, cancelWrite := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancelWrite()
	query := `UPDATE api_tokens SET revoked_at = ?, revoked_reason = ? WHERE id = ? AND revoked_at IS NULL`
	args := []any{time.Now().Unix(), "operator_action", id}
	if kind == "primary" {
		query = `UPDATE api_tokens SET revoked_at = ?, revoked_reason = ? WHERE owner_id = ? AND revoked_at IS NULL`
		args = []any{time.Now().Unix(), "operator_action", ownerID}
	}
	result, err := db.ExecContext(writeCtx, query, args...)
	if err != nil {
		return fmt.Errorf("revoke token: %w", err)
	}
	if affected, err := result.RowsAffected(); err != nil {
		return fmt.Errorf("verify token revocation: %w", err)
	} else if affected != 1 {
		return fmt.Errorf("token %q is no longer active", prefix)
	}
	fmt.Println(terminal.StatusIcon(true) + " Token revoked: " + prefix)
	return nil
}

// formatUnixTime renders a unix timestamp for display, or "-" when unset.
func formatUnixTime(ts int64) string {
	if ts <= 0 {
		return "-"
	}
	return time.Unix(ts, 0).UTC().Format("2006-01-02 15:04:05Z")
}

// formatOptionalUnixTime renders an optional unix timestamp, or "never" when the
// column is NULL (a token that never expires).
func formatOptionalUnixTime(ts sql.NullInt64) string {
	if !ts.Valid {
		return "never"
	}
	return formatUnixTime(ts.Int64)
}
