// SPDX-License-Identifier: MIT
// AI.md compliance "Data export" / "Right to delete" CLI commands, and the
// "Right to erasure vs Retention Requirements" rule that governs what delete
// actually does.
package main

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

	"github.com/apimgr/vidveil/src/common/terminal"
	"github.com/apimgr/vidveil/src/config"
	"github.com/apimgr/vidveil/src/server/service/logging"
)

// redactedMarker replaces PII under AI.md's erasure rule: "All PII is removed,
// replaced with [REDACTED] or anonymized IDs".
const redactedMarker = "[REDACTED]"

// handleMaintenanceData implements --maintenance data export|delete.
func handleMaintenanceData(arg, configDir, dataDir string) error {
	fields := strings.Fields(arg)
	binaryName := filepath.Base(os.Args[0])
	usage := fmt.Sprintf("Usage: %s --maintenance data export [--out <file>]", binaryName)
	usage += fmt.Sprintf("\n       %s --maintenance data delete", binaryName)
	if len(fields) == 0 {
		return fmt.Errorf("missing data action\n   %s", usage)
	}

	switch fields[0] {
	case "export":
		return exportData(fields[1:], configDir, dataDir)
	case "delete":
		if len(fields) != 1 {
			return fmt.Errorf("data delete takes no arguments\n   %s", usage)
		}
		return deleteData(configDir, dataDir)
	default:
		return fmt.Errorf("unknown data action %q\n   %s", fields[0], usage)
	}
}

// exportableTables lists the tables a compliance export covers, in the order
// they appear in the output. app_secrets is absent on purpose: it holds the
// installation secret and the cookie/CSRF signing keys, and an export must not
// become a secret-exfiltration path.
var exportableTables = []string{
	"security_reports",
	"favorites",
	"notifications",
	"scheduled_tasks",
	"task_history",
	"audit_log",
	"owners",
	"api_tokens",
}

// exportData writes a JSON dump of exportableTables to stdout, or to a file
// when --out is given. security_reports.encrypted_body is omitted so the
// export carries report metadata without handing out the reporter's plaintext.
func exportData(args []string, configDir, dataDir string) error {
	outPath := ""
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--out":
			if i+1 >= len(args) {
				return fmt.Errorf("--out requires a file path")
			}
			outPath = args[i+1]
			i++
		default:
			return fmt.Errorf("unknown data export option %q", args[i])
		}
	}

	// Export writes every row of every table to a file the operator chooses, so
	// it takes the same authorization path as the other maintenance commands.
	if err := authorizeSensitiveOperation(configDir, dataDir); err != nil {
		return err
	}
	requestID, err := newDataRequestID()
	if err != nil {
		return fmt.Errorf("create export request ID: %w", err)
	}
	if err := emitDataAudit(configDir, dataDir, "compliance.data_export_requested", requestID, nil); err != nil {
		return err
	}

	mgr, err := openSecretDB(dataDir)
	if err != nil {
		return fmt.Errorf("open database: %w", err)
	}
	defer func() { _ = mgr.Close() }()

	export := map[string]any{
		"exported_at": time.Now().UTC().Format(time.RFC3339),
		"tables":      map[string][]map[string]any{},
	}
	tables := export["tables"].(map[string][]map[string]any)

	for _, table := range exportableTables {
		rows, err := readTableForExport(mgr.GetDB(), table)
		if err != nil {
			// A table missing from an older schema must not abort the whole
			// export; record it as empty so the dump stays complete.
			if strings.Contains(err.Error(), "no such table") {
				tables[table] = []map[string]any{}
				continue
			}
			return err
		}
		tables[table] = rows
	}

	payload, err := json.MarshalIndent(export, "", "  ")
	if err != nil {
		return fmt.Errorf("encode export: %w", err)
	}
	payload = append(payload, '\n')

	if outPath == "" {
		if _, err := os.Stdout.Write(payload); err != nil {
			return fmt.Errorf("write export to stdout: %w", err)
		}
		return emitDataAudit(configDir, dataDir, "compliance.data_export_completed", requestID, map[string]interface{}{"file_size": len(payload)})
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
	if err := emitDataAudit(configDir, dataDir, "compliance.data_export_completed", requestID, map[string]interface{}{"file_size": len(payload)}); err != nil {
		return err
	}
	fmt.Println(terminal.StatusIcon(true) + " Data exported: " + outPath)
	return nil
}

// readTableForExport uses a fixed query per table and decodes each row into a
// map keyed by column name. Secret material is excluded by the query itself.
func readTableForExport(db *sql.DB, table string) (out []map[string]any, err error) {
	query, ok := exportTableQuery(table)
	if !ok {
		return nil, fmt.Errorf("table %q is not exportable", table)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	rows, err := db.QueryContext(ctx, query)
	if err != nil {
		return nil, err
	}
	defer func() {
		if closeErr := rows.Close(); err == nil && closeErr != nil {
			err = closeErr
		}
	}()

	cols, err := rows.Columns()
	if err != nil {
		return nil, err
	}
	out = []map[string]any{}
	for rows.Next() {
		values := make([]any, len(cols))
		scanArgs := make([]any, len(cols))
		for i := range values {
			scanArgs[i] = &values[i]
		}
		if err := rows.Scan(scanArgs...); err != nil {
			return nil, err
		}
		row := map[string]any{}
		for i, name := range cols {
			row[name] = values[i]
		}
		out = append(out, row)
	}
	return out, rows.Err()
}

// exportTableQuery returns explicit, reviewed column lists for compliance data.
func exportTableQuery(table string) (string, bool) {
	queries := map[string]string{
		"security_reports": `SELECT tracking_id, status, severity, component, endpoint,
			summary, encryption_method, researcher_email, researcher_gpg_fingerprint,
			cve_requested, disclosure_window_days, credit_preference, credit_name,
			disclosed, app_version, commit_hash, report_token_last_used_date,
			maintainer_comments, expected_disclosure_date, created_at, updated_at,
			closed_at FROM security_reports`,
		"favorites":       `SELECT id, owner_id, url, title, thumbnail, source, created_at FROM favorites`,
		"notifications":   `SELECT id, type, title, message, targets, created_at, read_at, details FROM notifications`,
		"scheduled_tasks": `SELECT id, name, schedule, enabled, last_run, next_run, last_result, last_error, run_count, fail_count FROM scheduled_tasks`,
		"task_history":    `SELECT id, task_id, start_time, end_time, duration_ms, result, error FROM task_history`,
		"audit_log":       `SELECT id, timestamp, user_id, username, action, resource, details, ip_address, user_agent FROM audit_log`,
		"owners":          `SELECT id, created_at, last_seen_at FROM owners`,
		"api_tokens":      `SELECT id, owner_id, kind, name, token_prefix, created_at, rotated_at, expires_at, last_used_at, revoked_at, revoked_reason FROM api_tokens`,
	}
	query, ok := queries[table]
	return query, ok
}

// anonymizations lists the PII columns cleared by "data delete", per table.
// Each row is (table, column, replacement); an empty replacement NULLs the
// column, which is right for genuinely optional PII.
//
// Per AI.md "Right to Erasure vs Retention Requirements" this anonymizes
// rather than deletes rows: "User data is anonymized (not deleted) to preserve
// audit trail", "All PII is removed, replaced with [REDACTED] or anonymized
// IDs", "Audit log entries reference anonymized user ID". Favorites are the
// exception: they carry no retention obligation, so they are removed outright.
var anonymizations = []struct {
	table       string
	column      string
	replacement string
}{
	{"security_reports", "researcher_email", redactedMarker},
	{"security_reports", "researcher_gpg_fingerprint", redactedMarker},
	{"security_reports", "credit_name", redactedMarker},
	{"security_reports", "maintainer_comments", redactedMarker},
	{"security_reports", "report_token_hash", ""},
	{"audit_log", "username", redactedMarker},
	{"audit_log", "user_id", redactedMarker},
	{"audit_log", "ip_address", redactedMarker},
	{"audit_log", "user_agent", redactedMarker},
}

// deleteData clears PII across every table that holds it. Favorites are deleted
// outright; every other table is anonymized in place so the audit trail and the
// security-report record survive erasure, per AI.md's erasure-vs-retention rule.
func deleteData(configDir, dataDir string) error {
	if err := authorizeSensitiveOperation(configDir, dataDir); err != nil {
		return err
	}
	if promptLine("Type DELETE to confirm data erasure: ") != "DELETE" {
		return fmt.Errorf("erasure cancelled: typed confirmation did not match DELETE")
	}
	requestID, err := newDataRequestID()
	if err != nil {
		return fmt.Errorf("create deletion request ID: %w", err)
	}
	if err := emitDataAudit(configDir, dataDir, "compliance.data_deletion_requested", requestID, nil); err != nil {
		return err
	}

	mgr, err := openSecretDB(dataDir)
	if err != nil {
		return fmt.Errorf("open database: %w", err)
	}
	defer func() { _ = mgr.Close() }()
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	tx, err := mgr.GetDB().BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin transaction: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	total := 0
	for _, a := range anonymizations {
		res, err := tx.ExecContext(ctx,
			"UPDATE "+a.table+" SET "+a.column+" = ? WHERE "+a.column+" IS NOT NULL",
			nullableMarker(a.replacement))
		if err != nil {
			// A column absent from an older schema is not a reason to abort the
			// erasure; the tables that matter most come later in the list.
			if strings.Contains(err.Error(), "no such column") {
				continue
			}
			return fmt.Errorf("anonymize %s.%s: %w", a.table, a.column, err)
		}
		n, err := res.RowsAffected()
		if err != nil {
			return fmt.Errorf("count anonymized %s.%s rows: %w", a.table, a.column, err)
		}
		total += int(n)
	}

	res, err := tx.ExecContext(ctx, "DELETE FROM favorites")
	if err != nil {
		return fmt.Errorf("delete favorites: %w", err)
	}
	deleted, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("count deleted favorites: %w", err)
	}
	total += int(deleted)

	// Token rows contain credential metadata and encrypted reveal copies, so
	// remove them before deleting their owner identities.
	tokenRes, err := tx.ExecContext(ctx, "DELETE FROM api_tokens")
	if err != nil {
		return fmt.Errorf("delete api_tokens: %w", err)
	}
	tokenCount, err := tokenRes.RowsAffected()
	if err != nil {
		return fmt.Errorf("count deleted api_tokens: %w", err)
	}
	total += int(tokenCount)
	ownerRes, err := tx.ExecContext(ctx, "DELETE FROM owners")
	if err != nil {
		return fmt.Errorf("delete owners: %w", err)
	}
	ownerCount, err := ownerRes.RowsAffected()
	if err != nil {
		return fmt.Errorf("count deleted owners: %w", err)
	}
	total += int(ownerCount)

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit erasure: %w", err)
	}
	if err := emitDataAudit(configDir, dataDir, "compliance.data_deletion_completed", requestID, map[string]interface{}{"records_anonymized_or_deleted": total}); err != nil {
		return err
	}
	fmt.Println(terminal.StatusIcon(true) + " Data erased: " +
		fmt.Sprintf("%d record(s) anonymized or deleted", total))
	return nil
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

// nullableMarker converts the empty replacement to a SQL NULL so an optional
// PII column is cleared rather than set to the empty string.
func nullableMarker(replacement string) any {
	if replacement == "" {
		return nil
	}
	return replacement
}
