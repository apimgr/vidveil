// SPDX-License-Identifier: MIT
// AI.md compliance "Data export" / "Right to delete" CLI commands, and the
// "Right to erasure vs Retention Requirements" rule that governs what delete
// actually does.
package app

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"os/user"
	"path/filepath"
	"strings"
	"time"

	"github.com/apimgr/vidveil/src/common/dbstore"
	"github.com/apimgr/vidveil/src/common/terminal"
	"github.com/apimgr/vidveil/src/config"
	"github.com/apimgr/vidveil/src/paths"
	"github.com/apimgr/vidveil/src/server/service/database"
	"github.com/apimgr/vidveil/src/server/service/logging"
	"github.com/apimgr/vidveil/src/server/service/system"
)

// handleMaintenanceData implements --maintenance data export|delete.
func handleMaintenanceData(arg, configDir, dataDir string) error {
	fields := strings.Fields(arg)
	binaryName := filepath.Base(os.Args[0])
	usage := fmt.Sprintf("Usage: %s --maintenance data export --owner <prefix> [--output FILE|-]", binaryName)
	usage += fmt.Sprintf("\n       %s --maintenance data delete --owner <prefix> [--yes]", binaryName)
	if len(fields) == 0 {
		return fmt.Errorf("missing data action\n   %s", usage)
	}

	switch fields[0] {
	case "export":
		return exportData(fields[1:], configDir, dataDir)
	case "delete":
		return deleteData(fields[1:], configDir, dataDir)
	default:
		return fmt.Errorf("unknown data action %q\n   %s", fields[0], usage)
	}
}

// exportData writes only one owner's metadata, safe token metadata and favorites.
func exportData(args []string, configDir, dataDir string) error {
	ownerPrefix, outPath := "", ""
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--owner":
			if i+1 >= len(args) || ownerPrefix != "" {
				return fmt.Errorf("--owner requires one token prefix")
			}
			ownerPrefix = args[i+1]
			i++
		case "--output":
			if i+1 >= len(args) {
				return fmt.Errorf("--output requires a file path or -")
			}
			outPath = args[i+1]
			i++
		default:
			return fmt.Errorf("unknown data export option %q", args[i])
		}
	}
	if ownerPrefix == "" {
		return fmt.Errorf("--owner <prefix> is required")
	}

	if err := authorizeDataExport(configDir, dataDir, outPath == "-"); err != nil {
		return err
	}
	mgr, err := openSecretDB(dataDir)
	if err != nil {
		return fmt.Errorf("open database: %w", err)
	}
	defer func() { _ = mgr.Close() }()
	db := mgr.GetAppDatabase()
	ownerID, err := resolveOwnerID(db, ownerPrefix)
	if err != nil {
		return err
	}
	requestID, err := newDataRequestID()
	if err != nil {
		return fmt.Errorf("create export request ID: %w", err)
	}
	if err := emitDataAudit(configDir, dataDir, "compliance.data_export_requested", requestID, map[string]interface{}{"owner_id": ownerID}); err != nil {
		return err
	}

	var createdAt, lastSeenAt int64
	if err := db.Read(context.Background(), func(ctx context.Context, readDB dbstore.Reader) error {
		return readDB.QueryRowContext(ctx, `SELECT created_at, last_seen_at FROM owners WHERE id = ?`, ownerID).Scan(&createdAt, &lastSeenAt)
	}); err != nil {
		return fmt.Errorf("read owner metadata: %w", err)
	}
	tokens, err := readOwnerTokens(db, ownerID)
	if err != nil {
		return err
	}
	favorites, err := readOwnerFavorites(db, ownerID)
	if err != nil {
		return err
	}

	export := map[string]any{
		"format": 1,
		"owner": map[string]any{
			"id": ownerID, "created_at": createdAt, "last_seen_at": lastSeenAt,
		},
		"tokens":    tokens,
		"resources": map[string]any{"favorites": favorites},
	}

	payload, err := json.MarshalIndent(export, "", "  ")
	if err != nil {
		return fmt.Errorf("encode export: %w", err)
	}
	payload = append(payload, '\n')

	if outPath == "-" {
		if _, err := os.Stdout.Write(payload); err != nil {
			return fmt.Errorf("write export to stdout: %w", err)
		}
		return emitDataAudit(configDir, dataDir, "compliance.data_export_completed", requestID, map[string]interface{}{"owner_id": ownerID, "file_size": len(payload)})
	}
	if outPath == "" {
		outPath = fmt.Sprintf("%s-export-%s-%s.json", paths.ProjectName, ownerID, time.Now().Format("20060102"))
	}
	file, err := os.OpenFile(outPath, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	if err != nil {
		return fmt.Errorf("write export: %w", err)
	}
	if _, err := file.Write(payload); err != nil {
		_ = file.Close()
		_ = os.Remove(outPath)
		return fmt.Errorf("write export: %w", err)
	}
	if err := file.Sync(); err != nil {
		_ = file.Close()
		_ = os.Remove(outPath)
		return fmt.Errorf("sync export: %w", err)
	}
	if err := file.Close(); err != nil {
		_ = os.Remove(outPath)
		return fmt.Errorf("close export: %w", err)
	}
	if err := emitDataAudit(configDir, dataDir, "compliance.data_export_completed", requestID, map[string]interface{}{"owner_id": ownerID, "file_size": len(payload)}); err != nil {
		return err
	}
	fmt.Println(terminal.StatusIcon(true) + " Data exported: " + outPath)
	return nil
}

func authorizeDataExport(configDir, dataDir string, stdoutOutput bool) error {
	if stdoutOutput && system.IsRunningAsRoot() {
		return nil
	}
	return authorizeSensitiveOperation(configDir, dataDir)
}

func resolveOwnerID(db *database.AppDatabase, prefix string) (string, error) {
	if strings.TrimSpace(prefix) != prefix || len(prefix) < 4 || len(prefix) > 64 {
		return "", fmt.Errorf("owner token prefix must be 4 to 64 characters with no surrounding whitespace")
	}
	var ownerID string
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	err := db.Read(ctx, func(readCtx context.Context, readDB dbstore.Reader) error {
		rows, err := readDB.QueryContext(readCtx, `SELECT DISTINCT owner_id FROM api_tokens WHERE owner_id IS NOT NULL AND substr(token_prefix, 1, length(?)) = ?`, prefix, prefix)
		if err != nil {
			return fmt.Errorf("find owner by token prefix: %w", err)
		}
		defer func() { _ = rows.Close() }()
		for rows.Next() {
			var candidate string
			if err := rows.Scan(&candidate); err != nil {
				return fmt.Errorf("read owner prefix match: %w", err)
			}
			if ownerID != "" && ownerID != candidate {
				return fmt.Errorf("owner token prefix %q is ambiguous", prefix)
			}
			ownerID = candidate
		}
		return rows.Err()
	})
	if err != nil {
		return "", err
	}
	if ownerID == "" {
		return "", fmt.Errorf("no owner matches token prefix %q", prefix)
	}
	return ownerID, nil
}

func readOwnerTokens(db *database.AppDatabase, ownerID string) ([]map[string]any, error) {
	tokens := make([]map[string]any, 0)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	err := db.Read(ctx, func(readCtx context.Context, readDB dbstore.Reader) error {
		rows, err := readDB.QueryContext(readCtx, `SELECT token_prefix, kind, COALESCE(name, ''), created_at, last_used_at, expires_at, revoked_at FROM api_tokens WHERE owner_id = ? ORDER BY created_at, id`, ownerID)
		if err != nil {
			return fmt.Errorf("read owner token metadata: %w", err)
		}
		defer func() { _ = rows.Close() }()
		for rows.Next() {
			var prefix, kind, name string
			var created int64
			var lastUsed, expires, revoked sql.NullInt64
			if err := rows.Scan(&prefix, &kind, &name, &created, &lastUsed, &expires, &revoked); err != nil {
				return fmt.Errorf("scan owner token metadata: %w", err)
			}
			status := "active"
			if revoked.Valid {
				status = "revoked"
			} else if expires.Valid && expires.Int64 <= time.Now().Unix() {
				status = "expired"
			}
			tokens = append(tokens, map[string]any{
				"prefix": prefix, "kind": kind, "name": name, "created_at": created,
				"last_used_at": nullableInt64(lastUsed), "expires_at": nullableInt64(expires), "status": status,
			})
		}
		return rows.Err()
	})
	return tokens, err
}

func readOwnerFavorites(db *database.AppDatabase, ownerID string) ([]map[string]any, error) {
	return readTableForExport(db, "favorites", ownerID)
}

func nullableInt64(value sql.NullInt64) any {
	if !value.Valid {
		return nil
	}
	return value.Int64
}

// readTableForExport uses a fixed query per table and decodes each row into a
// map keyed by column name. Secret material is excluded by the query itself.
func readTableForExport(db *database.AppDatabase, table, ownerID string) (out []map[string]any, err error) {
	query, ok := exportTableQuery(table)
	if !ok {
		return nil, fmt.Errorf("table %q is not exportable", table)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	err = db.Read(ctx, func(readCtx context.Context, readDB dbstore.Reader) error {
		rows, err := readDB.QueryContext(readCtx, query, ownerID)
		if err != nil {
			return err
		}
		defer func() { _ = rows.Close() }()
		cols, err := rows.Columns()
		if err != nil {
			return err
		}
		out = []map[string]any{}
		for rows.Next() {
			values := make([]any, len(cols))
			scanArgs := make([]any, len(cols))
			for i := range values {
				scanArgs[i] = &values[i]
			}
			if err := rows.Scan(scanArgs...); err != nil {
				return err
			}
			row := map[string]any{}
			for i, name := range cols {
				row[name] = values[i]
			}
			out = append(out, row)
		}
		rowsErr := rows.Err()
		if err := rows.Close(); err != nil {
			return err
		}
		return rowsErr
	})
	return out, err
}

// exportTableQuery returns explicit, reviewed resource queries scoped by owner.
func exportTableQuery(table string) (string, bool) {
	queries := map[string]string{
		"favorites": `SELECT id, url, title, thumbnail, source, created_at FROM favorites WHERE owner_id = ? ORDER BY created_at DESC, id DESC`,
	}
	query, ok := queries[table]
	return query, ok
}

// deleteData removes the selected owner's resources and tokens in one
// transaction, then removes its owner identity.
func deleteData(args []string, configDir, dataDir string) error {
	ownerPrefix := ""
	confirmed := false
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--owner":
			if ownerPrefix != "" || i+1 >= len(args) {
				return fmt.Errorf("--owner requires one token prefix")
			}
			ownerPrefix = args[i+1]
			i++
		case "--yes":
			confirmed = true
		default:
			return fmt.Errorf("unknown data delete option %q", args[i])
		}
	}
	if ownerPrefix == "" {
		return fmt.Errorf("--owner <prefix> is required")
	}
	if err := authorizeSensitiveOperation(configDir, dataDir); err != nil {
		return err
	}
	mgr, err := openSecretDB(dataDir)
	if err != nil {
		return fmt.Errorf("open database: %w", err)
	}
	defer func() { _ = mgr.Close() }()
	db := mgr.GetAppDatabase()
	ownerID, err := resolveOwnerID(db, ownerPrefix)
	if err != nil {
		return err
	}
	if !confirmed && promptLine("Type the owner id "+ownerID+" to confirm deletion: ") != ownerID {
		return fmt.Errorf("deletion cancelled: owner id confirmation did not match")
	}
	anonymousID, err := newDataRequestID()
	if err != nil {
		return fmt.Errorf("create anonymized owner id: %w", err)
	}
	requestID, err := newDataRequestID()
	if err != nil {
		return fmt.Errorf("create deletion request ID: %w", err)
	}
	if err := emitDataAudit(configDir, dataDir, "compliance.data_deletion_requested", requestID, map[string]interface{}{"owner_id": anonymousID}); err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	deletedCount := 0
	err = db.Write(ctx, func(tx *sql.Tx) error {
		rows, err := tx.QueryContext(ctx, `SELECT DISTINCT owner_id FROM api_tokens WHERE owner_id IS NOT NULL AND substr(token_prefix, 1, length(?)) = ?`, ownerPrefix, ownerPrefix)
		if err != nil {
			return fmt.Errorf("recheck owner token prefix: %w", err)
		}
		matched := ""
		for rows.Next() {
			var candidate string
			if err := rows.Scan(&candidate); err != nil {
				_ = rows.Close()
				return fmt.Errorf("read owner prefix match: %w", err)
			}
			if matched != "" && matched != candidate {
				_ = rows.Close()
				return fmt.Errorf("owner token prefix %q became ambiguous; refusing to delete", ownerPrefix)
			}
			matched = candidate
		}
		if err := rows.Err(); err != nil {
			_ = rows.Close()
			return err
		}
		if err := rows.Close(); err != nil {
			return err
		}
		if matched != ownerID {
			return fmt.Errorf("owner token prefix %q no longer resolves to the confirmed owner", ownerPrefix)
		}
		resourceCount, err := deleteOwnerResources(ctx, tx, ownerID)
		if err != nil {
			return err
		}
		deletedCount = resourceCount
		tokenRes, err := tx.ExecContext(ctx, "DELETE FROM api_tokens WHERE owner_id = ?", ownerID)
		if err != nil {
			return fmt.Errorf("delete owner tokens: %w", err)
		}
		tokenCount, err := tokenRes.RowsAffected()
		if err != nil {
			return fmt.Errorf("count deleted owner tokens: %w", err)
		}
		ownerRes, err := tx.ExecContext(ctx, "DELETE FROM owners WHERE id = ?", ownerID)
		if err != nil {
			return fmt.Errorf("delete owner record: %w", err)
		}
		ownerCount, err := ownerRes.RowsAffected()
		if err != nil {
			return fmt.Errorf("count deleted owner record: %w", err)
		}
		deletedCount += int(tokenCount) + int(ownerCount)
		return nil
	})
	if err != nil {
		return fmt.Errorf("delete owner data: %w", err)
	}
	if err := emitDataAudit(configDir, dataDir, "compliance.data_deletion_completed", requestID, map[string]interface{}{"owner_id": anonymousID, "deleted_count": deletedCount, "anonymized_count": 0}); err != nil {
		return err
	}
	fmt.Printf("%s Data deleted: %d item(s) deleted, 0 anonymized\n", terminal.StatusIcon(true), deletedCount)
	return nil
}

func deleteOwnerResources(ctx context.Context, tx *sql.Tx, ownerID string) (int, error) {
	result, err := tx.ExecContext(ctx, "DELETE FROM favorites WHERE owner_id = ?", ownerID)
	if err != nil {
		return 0, fmt.Errorf("delete owner favorites: %w", err)
	}
	count, err := result.RowsAffected()
	if err != nil {
		return 0, fmt.Errorf("count deleted owner favorites: %w", err)
	}
	return int(count), nil
}

func newDataRequestID() (string, error) {
	var id [16]byte
	if _, err := rand.Read(id[:]); err != nil {
		return "", err
	}
	return hex.EncodeToString(id[:]), nil
}

func emitDataAudit(configDir, dataDir, event, requestID string, details map[string]interface{}) error {
	cfg, _, err := config.LoadAppConfig(configDir, dataDir)
	if err != nil {
		return fmt.Errorf("compliance audit log unavailable: %w", err)
	}
	logger, err := logging.NewAppLogger(cfg)
	if err != nil {
		return fmt.Errorf("compliance audit log unavailable: %w", err)
	}
	defer logger.Close()
	operator := "root"
	if current, err := user.Current(); err == nil {
		operator = current.Username
	}
	if details == nil {
		details = make(map[string]interface{})
	}
	details["request_id"] = requestID
	logger.Audit(event, operator, "operator", localOperatorIP(), "success", details)
	return nil
}
