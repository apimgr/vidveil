// SPDX-License-Identifier: MIT
// AI.md PART 10: Database - Schema Management
// Per PART 10: "ALL apps use CREATE TABLE IF NOT EXISTS for self-creating schema"
// Per PART 10: "No migrations table | Keep it simple"
package database

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/apimgr/vidveil/src/server/service/logging"
	"github.com/google/uuid"
)

// SchemaManager handles database schema creation per AI.md PART 10
// Uses CREATE TABLE IF NOT EXISTS - no migrations tracking table
// Supports SQLite (local) and libsql/Turso (remote) backends per AI.md PART 3
type SchemaManager struct {
	db     *sql.DB
	readDB *sql.DB
	appDB  *AppDatabase
	dbPath string
	driver Driver
}

// NewSchemaManager creates a new schema manager for SQLite (backward compatibility)
func NewSchemaManager(dbPath string) (*SchemaManager, error) {
	// Ensure directory exists
	dir := filepath.Dir(dbPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create database directory: %w", err)
	}

	appDB, err := NewAppDatabase(DatabaseConfig{Driver: DriverSQLite, Path: dbPath})
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}
	return &SchemaManager{db: appDB.db, readDB: appDB.readDB, appDB: appDB, dbPath: dbPath, driver: DriverSQLite}, nil
}

// NewSchemaManagerWithConfig creates a schema manager using DatabaseConfig
// Supports SQLite and libsql/Turso per AI.md PART 10
func NewSchemaManagerWithConfig(cfg DatabaseConfig) (*SchemaManager, error) {
	appDB, err := NewAppDatabase(cfg)
	if err != nil {
		return nil, err
	}

	readDB := appDB.readDB
	if readDB == nil {
		_ = appDB.Close()
		return nil, fmt.Errorf("database read handle is unavailable")
	}
	return &SchemaManager{db: appDB.db, readDB: readDB, appDB: appDB, dbPath: cfg.Path, driver: appDB.Driver()}, nil
}

// EnsureSchema creates all required tables if they don't exist
// Per AI.md PART 10: Idempotent, safe to run multiple times
func (sm *SchemaManager) EnsureSchema() error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	// Get tables DDL for the current driver
	tables := sm.getTablesDDL()
	var deferredSchedulerIndexes []string

	// Execute all table creation statements
	for _, ddl := range tables {
		if strings.Contains(ddl, "idx_scheduler_history_") {
			deferredSchedulerIndexes = append(deferredSchedulerIndexes, ddl)
			continue
		}
		if _, err := sm.db.ExecContext(ctx, ddl); err != nil {
			return fmt.Errorf("failed to create table: %w", err)
		}
	}
	if err := sm.migrateSchedulerSchema(ctx); err != nil {
		return err
	}
	for _, ddl := range deferredSchedulerIndexes {
		if _, err := sm.db.ExecContext(ctx, ddl); err != nil {
			return fmt.Errorf("create scheduler history index: %w", err)
		}
	}
	if _, err := sm.db.ExecContext(ctx, `INSERT OR IGNORE INTO owners(id) VALUES ('system')`); err != nil {
		return fmt.Errorf("create reserved system owner: %w", err)
	}
	if err := sm.ensureOwnerTokenColumns(ctx); err != nil {
		return err
	}
	if err := sm.ensureFavoriteOwnerColumn(ctx); err != nil {
		return err
	}
	if err := sm.ensureAuditMirrorColumns(ctx); err != nil {
		return err
	}

	return nil
}

func (sm *SchemaManager) ensureFavoriteOwnerColumn(ctx context.Context) error {
	var count int
	if err := sm.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM pragma_table_info('favorites') WHERE name = 'owner_id'`).Scan(&count); err != nil {
		return fmt.Errorf("inspect favorites ownership schema: %w", err)
	}
	if count == 0 {
		if _, err := sm.db.ExecContext(ctx, `ALTER TABLE favorites ADD COLUMN owner_id TEXT REFERENCES owners(id)`); err != nil {
			return fmt.Errorf("add favorites owner_id column: %w", err)
		}
	}
	if _, err := sm.db.ExecContext(ctx, `UPDATE favorites SET owner_id = 'system' WHERE owner_id IS NULL`); err != nil {
		return fmt.Errorf("assign legacy favorites to system owner: %w", err)
	}
	if _, err := sm.db.ExecContext(ctx, `CREATE INDEX IF NOT EXISTS idx_favorites_owner_id ON favorites(owner_id)`); err != nil {
		return fmt.Errorf("index favorite owners: %w", err)
	}
	return nil
}

func (sm *SchemaManager) ensureAuditMirrorColumns(ctx context.Context) error {
	table := sm.auditTableName()
	columns := map[string]string{
		"level":       `TEXT NOT NULL DEFAULT 'info'`,
		"category":    `TEXT NOT NULL DEFAULT 'system'`,
		"actor_ip":    `TEXT`,
		"target_type": `TEXT`,
		"target_id":   `TEXT`,
		"success":     `INTEGER NOT NULL DEFAULT 1`,
	}
	known := make(map[string]bool)
	rows, err := sm.db.QueryContext(ctx, "PRAGMA table_info('"+table+"')")
	if err != nil {
		return fmt.Errorf("inspect audit mirror schema: %w", err)
	}
	for rows.Next() {
		var cid, notNull, primaryKey int
		var name, columnType string
		var defaultValue any
		if err := rows.Scan(&cid, &name, &columnType, &notNull, &defaultValue, &primaryKey); err != nil {
			_ = rows.Close()
			return fmt.Errorf("read audit mirror schema: %w", err)
		}
		known[name] = true
	}
	if err := rows.Err(); err != nil {
		_ = rows.Close()
		return fmt.Errorf("read audit mirror schema: %w", err)
	}
	if err := rows.Close(); err != nil {
		return fmt.Errorf("close audit mirror schema rows: %w", err)
	}
	for name, definition := range columns {
		if known[name] {
			continue
		}
		if _, err := sm.db.ExecContext(ctx, "ALTER TABLE "+table+" ADD COLUMN "+name+" "+definition); err != nil {
			return fmt.Errorf("add audit mirror column %s: %w", name, err)
		}
	}
	return nil
}

func (sm *SchemaManager) auditTableName() string {
	if normalizeDriver(sm.driver) == DriverLibSQL {
		return "srv_audit_log"
	}
	return "audit_log"
}

func (sm *SchemaManager) InsertAuditMirror(ctx context.Context, entry logging.AuditEntry) error {
	if ctx == nil {
		ctx = context.Background()
	}
	writeCtx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()
	details := make(map[string]interface{}, len(entry.Details)+1)
	for key, value := range entry.Details {
		details[key] = value
	}
	details["audit_id"] = entry.ID
	encodedDetails, err := json.Marshal(details)
	if err != nil {
		return fmt.Errorf("encode audit mirror details: %w", err)
	}
	timestamp, err := time.Parse("2006-01-02T15:04:05.000Z", entry.Time)
	if err != nil {
		return fmt.Errorf("parse audit mirror timestamp: %w", err)
	}
	success := 0
	if entry.Result == "success" {
		success = 1
	}
	var targetType, targetID, actorIP any
	if entry.Target != nil {
		targetType, targetID = entry.Target.Type, entry.Target.ID
	}
	if entry.Actor.IP != "" {
		actorIP = entry.Actor.IP
	}
	query := "INSERT INTO " + sm.auditTableName() + " (timestamp, level, category, action, actor_ip, target_type, target_id, details, success) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)"
	if _, err := sm.db.ExecContext(writeCtx, query, timestamp.Unix(), entry.Severity, entry.Category, entry.Event, actorIP, targetType, targetID, string(encodedDetails), success); err != nil {
		return fmt.Errorf("insert audit mirror: %w", err)
	}
	return nil
}

// ensureOwnerTokenColumns upgrades databases created before anonymous owners
// were added. Legacy tokens remain linked to isolated owners and cannot be
// revealed because their original raw values were never stored.
func (sm *SchemaManager) ensureOwnerTokenColumns(ctx context.Context) error {
	columns, idType, err := apiTokenColumns(ctx, sm.db)
	if err != nil {
		return err
	}
	if !columns["id"] {
		return fmt.Errorf("api_tokens is missing its existing id column")
	}
	legacyID := !strings.EqualFold(idType, "TEXT")
	definitions := map[string]string{
		"owner_id":       `TEXT REFERENCES owners(id)`,
		"kind":           `TEXT NOT NULL DEFAULT 'linked'`,
		"name":           `TEXT`,
		"scope":          `TEXT NOT NULL DEFAULT 'read-write' CHECK (scope IN ('read', 'read-write', 'global'))`,
		"token_hash":     `TEXT NOT NULL DEFAULT ''`,
		"token_prefix":   `TEXT NOT NULL DEFAULT ''`,
		"token_last4":    `TEXT NOT NULL DEFAULT ''`,
		"token_enc":      `TEXT NOT NULL DEFAULT ''`,
		"created_at":     `INTEGER NOT NULL DEFAULT 0`,
		"rotated_at":     `INTEGER`,
		"expires_at":     `INTEGER`,
		"last_used_at":   `INTEGER`,
		"revoked_at":     `INTEGER`,
		"revoked_reason": `TEXT`,
	}
	if legacyID {
		definitions["public_id"] = `TEXT`
	}
	for name, definition := range definitions {
		if columns[name] {
			continue
		}
		if _, err := sm.db.ExecContext(ctx, `ALTER TABLE api_tokens ADD COLUMN "`+name+`" `+definition); err != nil {
			return fmt.Errorf("add API-token column %s: %w", name, err)
		}
		columns[name] = true
	}
	if legacyID {
		if err := sm.populateLegacyTokenColumns(ctx, columns); err != nil {
			return err
		}
	}
	if _, err := sm.db.ExecContext(ctx, `UPDATE api_tokens SET scope = 'global' WHERE kind = 'primary' AND scope <> 'global'`); err != nil {
		return fmt.Errorf("set primary API-token scope: %w", err)
	}
	indexes := []string{
		`CREATE UNIQUE INDEX IF NOT EXISTS idx_api_tokens_primary ON api_tokens(owner_id) WHERE kind = 'primary' AND revoked_at IS NULL`,
		`CREATE INDEX IF NOT EXISTS idx_api_tokens_hash ON api_tokens(token_hash)`,
		`CREATE INDEX IF NOT EXISTS idx_api_tokens_prefix ON api_tokens(token_prefix)`,
		`CREATE INDEX IF NOT EXISTS idx_api_tokens_owner ON api_tokens(owner_id)`,
		`CREATE INDEX IF NOT EXISTS idx_api_tokens_active ON api_tokens(revoked_at) WHERE revoked_at IS NULL`,
	}
	if legacyID {
		indexes = append(indexes, `CREATE UNIQUE INDEX IF NOT EXISTS idx_api_tokens_public_id ON api_tokens(public_id) WHERE public_id IS NOT NULL`)
	}
	for _, ddl := range indexes {
		if _, err := sm.db.ExecContext(ctx, ddl); err != nil {
			return fmt.Errorf("create owner-token index: %w", err)
		}
	}
	return nil
}

func (sm *SchemaManager) populateLegacyTokenColumns(ctx context.Context, columns map[string]bool) error {
	last4Expr := `''`
	if columns["token_suffix"] {
		last4Expr = `COALESCE(token_suffix, '')`
	}
	query := `SELECT CAST(id AS TEXT), COALESCE(owner_id, ''), COALESCE(public_id, ''), ` + last4Expr + ` FROM api_tokens ORDER BY id`
	rows, err := sm.db.QueryContext(ctx, query)
	if err != nil {
		return fmt.Errorf("read legacy API-token identifiers: %w", err)
	}
	type legacyIdentity struct{ id, ownerID, publicID, last4 string }
	var identities []legacyIdentity
	for rows.Next() {
		var item legacyIdentity
		if err := rows.Scan(&item.id, &item.ownerID, &item.publicID, &item.last4); err != nil {
			_ = rows.Close()
			return fmt.Errorf("scan legacy API-token identifiers: %w", err)
		}
		identities = append(identities, item)
	}
	if err := rows.Err(); err != nil {
		_ = rows.Close()
		return fmt.Errorf("read legacy API-token identifiers: %w", err)
	}
	if err := rows.Close(); err != nil {
		return fmt.Errorf("close legacy API-token identifiers: %w", err)
	}
	for _, item := range identities {
		publicID := item.publicID
		if publicID == "" {
			id, err := uuid.NewV7()
			if err != nil {
				return fmt.Errorf("generate public id for legacy API token %q: %w", item.id, err)
			}
			publicID = id.String()
		}
		ownerID := item.ownerID
		if ownerID == "" {
			id, err := uuid.NewV7()
			if err != nil {
				return fmt.Errorf("generate owner for legacy API token %q: %w", item.id, err)
			}
			ownerID = id.String()
		}
		if _, err := sm.db.ExecContext(ctx, `INSERT OR IGNORE INTO owners(id) VALUES (?)`, ownerID); err != nil {
			return fmt.Errorf("preserve owner for legacy API token %q: %w", item.id, err)
		}
		if _, err := sm.db.ExecContext(ctx, `UPDATE api_tokens SET public_id = ?, owner_id = ?, token_last4 = COALESCE(NULLIF(token_last4, ''), ?) WHERE id = ?`, publicID, ownerID, item.last4, item.id); err != nil {
			return fmt.Errorf("preserve legacy API token %q: %w", item.id, err)
		}
	}
	return nil
}

func apiTokenColumns(ctx context.Context, db *sql.DB) (map[string]bool, string, error) {
	rows, err := db.QueryContext(ctx, `PRAGMA table_info(api_tokens)`)
	if err != nil {
		return nil, "", fmt.Errorf("inspect API-token schema: %w", err)
	}
	defer func() { _ = rows.Close() }()
	columns := make(map[string]bool)
	idType := ""
	for rows.Next() {
		var cid, notNull, primaryKey int
		var name, columnType string
		var defaultValue sql.NullString
		if err := rows.Scan(&cid, &name, &columnType, &notNull, &defaultValue, &primaryKey); err != nil {
			return nil, "", fmt.Errorf("read API-token schema: %w", err)
		}
		columns[name] = true
		if name == "id" {
			idType = columnType
		}
	}
	if err := rows.Err(); err != nil {
		return nil, "", fmt.Errorf("read API-token schema: %w", err)
	}
	return columns, idType, nil
}

// getTablesDDL returns CREATE TABLE statements.
// SQLite and libsql share the same DDL dialect per AI.md PART 10.
func (sm *SchemaManager) getTablesDDL() []string {
	statements := sm.getSQLiteDDL()
	if normalizeDriver(sm.driver) == DriverLibSQL {
		for index := range statements {
			statements[index] = strings.ReplaceAll(statements[index], "audit_log", "srv_audit_log")
			statements[index] = strings.ReplaceAll(statements[index], "idx_audit_", "srv_idx_audit_")
		}
	}
	return statements
}

// getSQLiteDDL returns SQLite/libsql DDL
func (sm *SchemaManager) getSQLiteDDL() []string {
	return []string{
		// Audit log table for tracking actions
		`CREATE TABLE IF NOT EXISTS audit_log (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			timestamp INTEGER NOT NULL DEFAULT (strftime('%s', 'now')),
			level TEXT NOT NULL DEFAULT 'info',
			category TEXT NOT NULL DEFAULT 'system',
			action TEXT NOT NULL,
			actor_ip TEXT,
			target_type TEXT,
			target_id TEXT,
			details TEXT,
			success INTEGER NOT NULL DEFAULT 1
		)`,
		`CREATE INDEX IF NOT EXISTS idx_audit_timestamp ON audit_log(timestamp)`,
		`CREATE INDEX IF NOT EXISTS idx_audit_category ON audit_log(category)`,
		`CREATE INDEX IF NOT EXISTS idx_audit_actor_ip ON audit_log(actor_ip)`,
		`CREATE INDEX IF NOT EXISTS idx_audit_target ON audit_log(target_type, target_id)`,

		// Settings table for runtime config
		`CREATE TABLE IF NOT EXISTS settings (
			key TEXT PRIMARY KEY,
			value TEXT,
			type TEXT DEFAULT 'string',
			updated_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			updated_by TEXT
		)`,

		// Scheduler task state per AI.md PART 10 / PART 18.
		`CREATE TABLE IF NOT EXISTS scheduler_tasks (
			id TEXT PRIMARY KEY,
			name TEXT NOT NULL,
			enabled INTEGER NOT NULL DEFAULT 1,
			schedule TEXT NOT NULL,
			last_run INTEGER,
			last_status TEXT,
			last_error TEXT,
			next_run INTEGER,
			run_count INTEGER NOT NULL DEFAULT 0,
			fail_count INTEGER NOT NULL DEFAULT 0
		)`,

		// Scheduler execution history per AI.md PART 10 / PART 18.
		`CREATE TABLE IF NOT EXISTS scheduler_history (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			task_id TEXT NOT NULL REFERENCES scheduler_tasks(id),
			started_at INTEGER NOT NULL,
			finished_at INTEGER,
			status TEXT NOT NULL,
			error TEXT,
			duration_ms INTEGER
		)`,
		`CREATE INDEX IF NOT EXISTS idx_scheduler_history_task ON scheduler_history(task_id)`,
		`CREATE INDEX IF NOT EXISTS idx_scheduler_history_started ON scheduler_history(started_at)`,

		// App secrets table per AI.md PART 11
		// Stores installation_secret, cookie_signing_key, csrf_token_secret
		`CREATE TABLE IF NOT EXISTS app_secrets (
			key TEXT PRIMARY KEY,
			value TEXT NOT NULL,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			rotated_at DATETIME,
			expires_at DATETIME,
			previous_value TEXT
		)`,
		`CREATE TABLE IF NOT EXISTS captcha_challenges (
			id VARCHAR(32) PRIMARY KEY,
			answer_hash VARCHAR(64) NOT NULL,
			expires_at TIMESTAMP NOT NULL
		)`,
		`CREATE INDEX IF NOT EXISTS idx_captcha_challenges_expires ON captcha_challenges(expires_at)`,

		// Notifications table per AI.md PART 17
		// Stores notification center history (toast/banner are real-time only)
		`CREATE TABLE IF NOT EXISTS notifications (
			id TEXT PRIMARY KEY,
			type TEXT NOT NULL,
			title TEXT NOT NULL,
			message TEXT,
			targets INTEGER NOT NULL,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			read_at DATETIME,
			details TEXT
		)`,

		// PGP keypair metadata per AI.md PART 12 "GPG Keypair Management".
		// The keys themselves live on disk under {config_dir}/security/; this
		// table stores only metadata. Single logical row (id=1).
		`CREATE TABLE IF NOT EXISTS pgp_keypair (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			fingerprint TEXT NOT NULL,
			created_at DATETIME NOT NULL,
			expires_at DATETIME NOT NULL,
			last_rotated_at DATETIME,
			keyservers_published TEXT DEFAULT '[]',
			revoked INTEGER DEFAULT 0
		)`,

		// Security reports table per AI.md PART 11 "Security Reports —
		// Coordinated Disclosure Pipeline". Tracking metadata only; the
		// report body is encrypted at rest (PGP if a keypair exists, else
		// AES-256-GCM under server.security.encryption_key) and plaintext is
		// NEVER persisted. No admin web UI/API exists for this table per
		// PART 11 "Security Administration" — status/comments are set by
		// out-of-band maintainer tooling.
		`CREATE TABLE IF NOT EXISTS security_reports (
			tracking_id VARCHAR(32) PRIMARY KEY,
			id_window BIGINT NOT NULL,
			severity VARCHAR(16) NOT NULL,
			component VARCHAR(255) NOT NULL,
			status VARCHAR(16) NOT NULL DEFAULT 'received',
			body_enc BLOB NOT NULL,
			body_alg VARCHAR(16) NOT NULL,
			contact_enc BLOB,
			credit_pref VARCHAR(8) NOT NULL DEFAULT 'no',
			credit_name VARCHAR(100),
			researcher_comment TEXT NOT NULL DEFAULT '',
			disclosure_date DATE,
			cve_requested BOOLEAN NOT NULL DEFAULT FALSE,
			token_hash VARCHAR(64) NOT NULL,
			token_last_used DATE,
			created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
			updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
			closed_at TIMESTAMP
		)`,
		`CREATE INDEX IF NOT EXISTS idx_security_reports_status ON security_reports(status, created_at)`,
		`CREATE TABLE IF NOT EXISTS security_thanks (
			tracking_id VARCHAR(32) PRIMARY KEY,
			credit VARCHAR(120) NOT NULL,
			severity VARCHAR(16) NOT NULL,
			disclosed_on DATE NOT NULL
		)`,

		// Favorites table per AI.md PART 16 progressive-enhancement rule
		// ("core functionality MUST work without JS") and PART 32's
		// cookie-vs-localStorage split (cookies are server-read/authoritative;
		// localStorage is JS-only, never load-bearing). owner_id binds each
		// favorite to its anonymous primary-token owner.
		`CREATE TABLE IF NOT EXISTS favorites (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			owner_id TEXT NOT NULL REFERENCES owners(id),
			visitor_id TEXT NOT NULL,
			url TEXT NOT NULL,
			title TEXT NOT NULL,
			thumbnail TEXT,
			source TEXT,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			UNIQUE(visitor_id, url)
		)`,

		`CREATE INDEX IF NOT EXISTS idx_favorites_visitor_id ON favorites(visitor_id)`,

		// API tokens per AI.md PART 11: token_hash stores the SHA-256 digest;
		// token_enc stores the AES-256-GCM encrypted raw owner token for reveal.
		// The operator server.token is never stored in this table.
		ownerTokenTableDDL("api_tokens"),

		`CREATE TABLE IF NOT EXISTS owners (
			id TEXT PRIMARY KEY,
			created_at INTEGER NOT NULL DEFAULT (strftime('%s', 'now')),
			last_seen_at INTEGER NOT NULL DEFAULT (strftime('%s', 'now'))
		)`,
		`CREATE TABLE IF NOT EXISTS health_stats (
			name TEXT PRIMARY KEY,
			value INTEGER NOT NULL DEFAULT 0,
			updated_at INTEGER NOT NULL DEFAULT 0
		)`,
		`CREATE TABLE IF NOT EXISTS health_request_buckets (
			bucket_hour INTEGER PRIMARY KEY,
			request_count INTEGER NOT NULL DEFAULT 0
		)`,
	}
}

// GetAppDatabase returns the transaction-aware database store for service
// layers that need the standard read/write pools.
func (sm *SchemaManager) GetAppDatabase() *AppDatabase { return sm.appDB }

// Close closes the database connection
func (sm *SchemaManager) Close() error {
	var readErr error
	if sm.appDB == nil && sm.readDB != nil && sm.readDB != sm.db {
		readErr = sm.readDB.Close()
	}
	writeErr := error(nil)
	if sm.appDB != nil {
		writeErr = sm.appDB.Close()
	} else if sm.db != nil {
		writeErr = sm.db.Close()
	}
	return errors.Join(readErr, writeErr)
}

// MigrationManager is an alias for SchemaManager for backward compatibility
// Deprecated: Use SchemaManager instead
type MigrationManager = SchemaManager

// NewMigrationManager creates a new schema manager (backward compatibility)
// Deprecated: Use NewSchemaManager instead
func NewMigrationManager(dbPath string) (*SchemaManager, error) {
	return NewSchemaManager(dbPath)
}

// RegisterDefaultMigrations is a no-op for backward compatibility
// Per AI.md PART 10: No migrations table, use EnsureSchema instead
func (sm *SchemaManager) RegisterDefaultMigrations() {
	// No-op: Tables are created via EnsureSchema()
}

// RunMigrations calls EnsureSchema for backward compatibility
// Per AI.md PART 10: No migrations table, use EnsureSchema instead
func (sm *SchemaManager) RunMigrations() error {
	return sm.EnsureSchema()
}

// GetMigrationStatus returns the status of all tables (for interface compatibility)
// Per AI.md PART 10: No migrations table - reports table existence instead
func (sm *SchemaManager) GetMigrationStatus() ([]map[string]interface{}, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// List all tables we manage
	tables := []string{
		"audit_log", "settings", "scheduler_tasks", "scheduler_history",
	}

	var status []map[string]interface{}
	for _, table := range tables {
		exists, _ := sm.tableExists(ctx, table)
		status = append(status, map[string]interface{}{
			"name":    table,
			"applied": exists,
		})
	}

	return status, nil
}

func (sm *SchemaManager) migrateSchedulerSchema(ctx context.Context) error {
	if err := sm.renameSchedulerColumns(ctx, "scheduler_tasks", map[string]string{
		"task_id":     "id",
		"task_name":   "name",
		"last_result": "last_status",
	}); err != nil {
		return err
	}
	if err := sm.renameSchedulerColumns(ctx, "scheduler_history", map[string]string{
		"ran_at": "started_at",
		"result": "status",
	}); err != nil {
		return err
	}
	if err := sm.ensureSchedulerColumns(ctx); err != nil {
		return err
	}
	for _, column := range []struct{ table, name string }{
		{"scheduler_tasks", "last_run"}, {"scheduler_tasks", "next_run"},
		{"scheduler_history", "started_at"}, {"scheduler_history", "finished_at"},
	} {
		if _, err := sm.db.ExecContext(ctx, `UPDATE "`+column.table+`" SET "`+column.name+`" = CAST(strftime('%s', "`+column.name+`") AS INTEGER) WHERE "`+column.name+`" IS NOT NULL AND typeof("`+column.name+`") != 'integer'`); err != nil {
			return fmt.Errorf("normalize scheduler timestamp %s.%s: %w", column.table, column.name, err)
		}
	}
	for _, table := range []string{"scheduler_tasks", "scheduler_history"} {
		column := "last_status"
		if table == "scheduler_history" {
			column = "status"
		}
		if _, err := sm.db.ExecContext(ctx, `UPDATE "`+table+`" SET "`+column+`" = 'failed' WHERE lower("`+column+`") = 'failure'`); err != nil {
			return fmt.Errorf("normalize scheduler status %s.%s: %w", table, column, err)
		}
	}
	if err := sm.copyLegacySchedulerTables(ctx); err != nil {
		return err
	}
	return nil
}

func (sm *SchemaManager) ensureSchedulerColumns(ctx context.Context) error {
	for table, required := range map[string]map[string]string{
		"scheduler_tasks": {
			"name": "TEXT NOT NULL DEFAULT ''", "enabled": "INTEGER NOT NULL DEFAULT 1",
			"schedule": "TEXT NOT NULL DEFAULT ''", "last_run": "INTEGER", "last_status": "TEXT",
			"last_error": "TEXT", "next_run": "INTEGER", "run_count": "INTEGER NOT NULL DEFAULT 0",
			"fail_count": "INTEGER NOT NULL DEFAULT 0",
		},
		"scheduler_history": {
			"started_at": "INTEGER NOT NULL DEFAULT 0", "finished_at": "INTEGER", "status": "TEXT NOT NULL DEFAULT 'failed'",
			"error": "TEXT", "duration_ms": "INTEGER",
		},
	} {
		columns, err := sm.tableColumns(ctx, table)
		if err != nil {
			return err
		}
		if !columns["id"] {
			return fmt.Errorf("%s is missing required id column", table)
		}
		if table == "scheduler_history" && !columns["task_id"] {
			return fmt.Errorf("%s is missing required task_id column", table)
		}
		for name, definition := range required {
			if columns[name] {
				continue
			}
			if _, err := sm.db.ExecContext(ctx, `ALTER TABLE "`+table+`" ADD COLUMN "`+name+`" `+definition); err != nil {
				return fmt.Errorf("add scheduler column %s.%s: %w", table, name, err)
			}
		}
	}
	return nil
}

func (sm *SchemaManager) renameSchedulerColumns(ctx context.Context, table string, renames map[string]string) error {
	exists, err := sm.tableExists(ctx, table)
	if err != nil || !exists {
		return err
	}
	columns, err := sm.tableColumns(ctx, table)
	if err != nil {
		return err
	}
	for oldName, newName := range renames {
		if columns[oldName] && !columns[newName] {
			if _, err := sm.db.ExecContext(ctx, `ALTER TABLE "`+table+`" RENAME COLUMN "`+oldName+`" TO "`+newName+`"`); err != nil {
				return fmt.Errorf("rename scheduler column %s.%s to %s: %w", table, oldName, newName, err)
			}
			columns[newName] = true
			delete(columns, oldName)
		}
	}
	return nil
}

func (sm *SchemaManager) tableColumns(ctx context.Context, table string) (map[string]bool, error) {
	rows, err := sm.db.QueryContext(ctx, `PRAGMA table_info("`+table+`")`)
	if err != nil {
		return nil, fmt.Errorf("inspect %s columns: %w", table, err)
	}
	defer func() { _ = rows.Close() }()
	columns := make(map[string]bool)
	for rows.Next() {
		var cid, notNull, primaryKey int
		var name, dataType string
		var defaultValue sql.NullString
		if err := rows.Scan(&cid, &name, &dataType, &notNull, &defaultValue, &primaryKey); err != nil {
			return nil, fmt.Errorf("scan %s columns: %w", table, err)
		}
		columns[name] = true
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("read %s columns: %w", table, err)
	}
	return columns, nil
}

func (sm *SchemaManager) copyLegacySchedulerTables(ctx context.Context) error {
	legacyTasks, err := sm.tableExists(ctx, "scheduled_tasks")
	if err != nil {
		return fmt.Errorf("check legacy scheduler task table: %w", err)
	}
	legacyHistory, err := sm.tableExists(ctx, "task_history")
	if err != nil {
		return fmt.Errorf("check legacy scheduler history table: %w", err)
	}
	if !legacyTasks && !legacyHistory {
		return nil
	}
	var taskColumns, historyColumns map[string]bool
	if legacyTasks {
		taskColumns, err = sm.tableColumns(ctx, "scheduled_tasks")
		if err != nil {
			return err
		}
	}
	if legacyHistory {
		historyColumns, err = sm.tableColumns(ctx, "task_history")
		if err != nil {
			return err
		}
	}
	tx, err := sm.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin scheduler schema migration: %w", err)
	}
	defer func() { _ = tx.Rollback() }()
	if legacyTasks {
		lastRun := legacyUnixColumn(taskColumns, "last_run")
		nextRun := legacyUnixColumn(taskColumns, "next_run")
		lastStatus := legacyColumn(taskColumns, "last_result", "NULL")
		lastError := legacyColumn(taskColumns, "last_error", "NULL")
		runCount := legacyColumn(taskColumns, "run_count", "0")
		failCount := legacyColumn(taskColumns, "fail_count", "0")
		if _, err := tx.ExecContext(ctx, `INSERT OR IGNORE INTO scheduler_tasks
			(id, name, enabled, schedule, last_run, last_status, last_error, next_run, run_count, fail_count)
			SELECT id, name, COALESCE(enabled, 1), schedule, `+lastRun+`,
			CASE lower(COALESCE(`+lastStatus+`, '')) WHEN 'failure' THEN 'failed' ELSE `+lastStatus+` END,
			`+lastError+`, `+nextRun+`, COALESCE(`+runCount+`, 0), COALESCE(`+failCount+`, 0)
			FROM scheduled_tasks`); err != nil {
			return fmt.Errorf("copy legacy scheduler tasks: %w", err)
		}
	}
	if legacyHistory {
		if _, err := tx.ExecContext(ctx, `INSERT OR IGNORE INTO scheduler_tasks(id, name, enabled, schedule)
			SELECT DISTINCT h.task_id, h.task_id, 0, '' FROM task_history h
			WHERE NOT EXISTS (SELECT 1 FROM scheduler_tasks t WHERE t.id = h.task_id)`); err != nil {
			return fmt.Errorf("preserve scheduler history task references: %w", err)
		}
		startedAt := legacyUnixColumn(historyColumns, "start_time")
		finishedAt := legacyUnixColumn(historyColumns, "end_time")
		status := legacyColumn(historyColumns, "result", "'failed'")
		errorText := legacyColumn(historyColumns, "error", "NULL")
		duration := legacyColumn(historyColumns, "duration_ms", "NULL")
		if _, err := tx.ExecContext(ctx, `INSERT OR IGNORE INTO scheduler_history
			(id, task_id, started_at, finished_at, status, error, duration_ms)
			SELECT id, task_id, `+startedAt+`, `+finishedAt+`,
			CASE lower(COALESCE(`+status+`, 'failed')) WHEN 'failure' THEN 'failed' ELSE `+status+` END,
			`+errorText+`, `+duration+` FROM task_history`); err != nil {
			return fmt.Errorf("copy legacy scheduler history: %w", err)
		}
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit scheduler schema migration: %w", err)
	}
	return nil
}

func legacyColumn(columns map[string]bool, name, fallback string) string {
	if columns[name] {
		return `"` + name + `"`
	}
	return fallback
}

func ownerTokenTableDDL(table string) string {
	return `CREATE TABLE IF NOT EXISTS ` + table + ` (
		id TEXT PRIMARY KEY,
		owner_id TEXT NOT NULL REFERENCES owners(id) ON DELETE CASCADE,
		kind TEXT NOT NULL CHECK (kind IN ('primary', 'linked')),
		name TEXT,
		scope TEXT NOT NULL DEFAULT 'read-write' CHECK (scope IN ('read', 'read-write', 'global')),
		token_hash TEXT NOT NULL UNIQUE,
		token_prefix TEXT NOT NULL,
		token_last4 TEXT NOT NULL,
		token_enc TEXT NOT NULL,
		created_at INTEGER NOT NULL DEFAULT (strftime('%s', 'now')),
		rotated_at INTEGER,
		expires_at INTEGER,
		last_used_at INTEGER,
		revoked_at INTEGER,
		revoked_reason TEXT
	)`
}

func legacyUnixColumn(columns map[string]bool, name string) string {
	column := legacyColumn(columns, name, "NULL")
	return `CASE WHEN ` + column + ` IS NULL THEN NULL WHEN typeof(` + column + `) IN ('integer','real') THEN CAST(` + column + ` AS INTEGER) ELSE CAST(strftime('%s', ` + column + `) AS INTEGER) END`
}

// tableExists checks if a table exists.
// SQLite and libsql share the sqlite_master catalog.
func (sm *SchemaManager) tableExists(ctx context.Context, tableName string) (bool, error) {
	query := "SELECT COUNT(*) FROM sqlite_master WHERE type='table' AND name=?"

	var count int
	err := sm.db.QueryRowContext(ctx, query, tableName).Scan(&count)
	return count > 0, err
}

// RollbackMigration is not supported with simple schema management
// Per AI.md PART 10: No migrations table, rollback not tracked
func (sm *SchemaManager) RollbackMigration() error {
	return fmt.Errorf("rollback not supported: per AI.md PART 10, use CREATE TABLE IF NOT EXISTS pattern")
}

// isColumnExistsError checks whether an error indicates a column already exists.
// Per AI.md PART 10: Used with ALTER TABLE ADD COLUMN to make schema updates idempotent.
// Covers SQLite/libsql ("duplicate column", "already exists").
func isColumnExistsError(err error) bool {
	if err == nil {
		return false
	}
	msg := err.Error()
	return strings.Contains(msg, "duplicate column") ||
		strings.Contains(msg, "already exists")
}

// isSerializationError checks whether an error is a database serialization / lock conflict.
// Per AI.md PART 10: Used with WithSerializableRetry to retry on transient lock failures.
// Covers SQLite/libsql ("SQLITE_BUSY", "database is locked").
func isSerializationError(err error) bool {
	if err == nil {
		return false
	}
	msg := err.Error()
	return strings.Contains(msg, "SQLITE_BUSY") ||
		strings.Contains(msg, "database is locked")
}
