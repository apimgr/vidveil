// SPDX-License-Identifier: MIT
// AI.md PART 10: Database - Schema Management
// Per PART 10: "ALL apps use CREATE TABLE IF NOT EXISTS for self-creating schema"
// Per PART 10: "No migrations table | Keep it simple"
package database

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/google/uuid"
)

// SchemaManager handles database schema creation per AI.md PART 10
// Uses CREATE TABLE IF NOT EXISTS - no migrations tracking table
// Supports SQLite (local) and libsql/Turso (remote) backends per AI.md PART 3
type SchemaManager struct {
	db     *sql.DB
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

	db, err := sql.Open("sqlite", dbPath+"?_journal_mode=WAL&_busy_timeout=5000")
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}

	return &SchemaManager{
		db:     db,
		dbPath: dbPath,
		driver: DriverSQLite,
	}, nil
}

// NewSchemaManagerWithConfig creates a schema manager using DatabaseConfig
// Supports SQLite and libsql/Turso per AI.md PART 10
func NewSchemaManagerWithConfig(cfg DatabaseConfig) (*SchemaManager, error) {
	appDB, err := NewAppDatabase(cfg)
	if err != nil {
		return nil, err
	}

	return &SchemaManager{
		db:     appDB.DB(),
		dbPath: cfg.Path,
		driver: appDB.Driver(),
	}, nil
}

// EnsureSchema creates all required tables if they don't exist
// Per AI.md PART 10: Idempotent, safe to run multiple times
func (sm *SchemaManager) EnsureSchema() error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	// Get tables DDL for the current driver
	tables := sm.getTablesDDL()

	// Execute all table creation statements
	for _, ddl := range tables {
		if _, err := sm.db.ExecContext(ctx, ddl); err != nil {
			return fmt.Errorf("failed to create table: %w", err)
		}
	}
	if err := sm.ensureOwnerTokenColumns(ctx); err != nil {
		return err
	}
	if err := sm.ensureFavoriteOwnerColumn(ctx); err != nil {
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
	if _, err := sm.db.ExecContext(ctx, `INSERT OR IGNORE INTO owners(id) SELECT DISTINCT 'legacy-favorite-' || visitor_id FROM favorites WHERE owner_id IS NULL`); err != nil {
		return fmt.Errorf("preserve legacy favorite owners: %w", err)
	}
	if _, err := sm.db.ExecContext(ctx, `UPDATE favorites SET owner_id = 'legacy-favorite-' || visitor_id WHERE owner_id IS NULL`); err != nil {
		return fmt.Errorf("assign legacy favorite owners: %w", err)
	}
	if _, err := sm.db.ExecContext(ctx, `DELETE FROM favorites WHERE id NOT IN (SELECT MIN(id) FROM favorites GROUP BY owner_id, url)`); err != nil {
		return fmt.Errorf("deduplicate favorites per owner: %w", err)
	}
	if _, err := sm.db.ExecContext(ctx, `UPDATE favorites SET visitor_id = owner_id WHERE visitor_id != owner_id`); err != nil {
		return fmt.Errorf("normalize favorite owner keys: %w", err)
	}
	if _, err := sm.db.ExecContext(ctx, `CREATE INDEX IF NOT EXISTS idx_favorites_owner_id ON favorites(owner_id)`); err != nil {
		return fmt.Errorf("index favorite owners: %w", err)
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
	required := []string{"id", "owner_id", "kind", "name", "token_hash", "token_prefix", "token_last4", "token_enc", "created_at", "rotated_at", "expires_at", "last_used_at", "revoked_at", "revoked_reason"}
	needsRebuild := !strings.EqualFold(idType, "TEXT") || len(columns) != len(required)
	for _, name := range required {
		if !columns[name] {
			needsRebuild = true
		}
	}
	if needsRebuild {
		if err := sm.rebuildOwnerTokenTable(ctx, columns); err != nil {
			return err
		}
	}
	for _, ddl := range []string{
		`CREATE UNIQUE INDEX IF NOT EXISTS idx_api_tokens_primary ON api_tokens(owner_id) WHERE kind = 'primary' AND revoked_at IS NULL`,
		`CREATE INDEX IF NOT EXISTS idx_api_tokens_hash ON api_tokens(token_hash)`,
		`CREATE INDEX IF NOT EXISTS idx_api_tokens_prefix ON api_tokens(token_prefix)`,
		`CREATE INDEX IF NOT EXISTS idx_api_tokens_owner ON api_tokens(owner_id)`,
		`CREATE INDEX IF NOT EXISTS idx_api_tokens_active ON api_tokens(revoked_at) WHERE revoked_at IS NULL`,
	} {
		if _, err := sm.db.ExecContext(ctx, ddl); err != nil {
			return fmt.Errorf("create owner-token index: %w", err)
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

func (sm *SchemaManager) rebuildOwnerTokenTable(ctx context.Context, columns map[string]bool) error {
	tx, err := sm.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin API-token schema migration: %w", err)
	}
	defer func() { _ = tx.Rollback() }()
	selectColumn := func(name, fallback string) string {
		if columns[name] {
			return `"` + name + `"`
		}
		return fallback
	}
	last4Column := "''"
	if columns["token_last4"] {
		last4Column = `COALESCE(token_last4, '')`
	} else if columns["token_suffix"] {
		last4Column = `COALESCE(token_suffix, '')`
	}
	createdColumn := `CAST(strftime('%s', 'now') AS INTEGER)`
	if columns["created_at"] {
		createdColumn = `COALESCE(created_at, CAST(strftime('%s', 'now') AS INTEGER))`
	}
	query := `SELECT ` + strings.Join([]string{
		selectColumn("id", `CAST(rowid AS TEXT)`),
		selectColumn("owner_id", `NULL`),
		selectColumn("kind", `'linked'`),
		selectColumn("name", `NULL`),
		selectColumn("token_hash", `''`),
		selectColumn("token_prefix", `''`),
		last4Column,
		selectColumn("token_enc", `''`),
		createdColumn,
		selectColumn("rotated_at", `NULL`),
		selectColumn("expires_at", `NULL`),
		selectColumn("last_used_at", `NULL`),
		selectColumn("revoked_at", `NULL`),
		selectColumn("revoked_reason", `NULL`),
	}, ",") + ` FROM api_tokens ORDER BY rowid`
	rows, err := tx.QueryContext(ctx, query)
	if err != nil {
		return fmt.Errorf("read existing API tokens: %w", err)
	}
	type legacyToken struct {
		id, owner, kind, name, hash, prefix, last4, encrypted, revokedReason sql.NullString
		created, rotated, expires, lastUsed, revoked                         sql.NullInt64
	}
	var legacyTokens []legacyToken
	for rows.Next() {
		var item legacyToken
		if err := rows.Scan(&item.id, &item.owner, &item.kind, &item.name, &item.hash, &item.prefix, &item.last4, &item.encrypted, &item.created, &item.rotated, &item.expires, &item.lastUsed, &item.revoked, &item.revokedReason); err != nil {
			_ = rows.Close()
			return fmt.Errorf("scan existing API token: %w", err)
		}
		legacyTokens = append(legacyTokens, item)
	}
	if err := rows.Err(); err != nil {
		_ = rows.Close()
		return fmt.Errorf("read existing API tokens: %w", err)
	}
	if err := rows.Close(); err != nil {
		return fmt.Errorf("close existing API-token rows: %w", err)
	}
	if _, err := tx.ExecContext(ctx, `DROP TABLE IF EXISTS api_tokens_migrating`); err != nil {
		return fmt.Errorf("remove stale API-token migration table: %w", err)
	}
	if _, err := tx.ExecContext(ctx, ownerTokenTableDDL("api_tokens_migrating")); err != nil {
		return fmt.Errorf("create API-token migration table: %w", err)
	}
	for _, old := range legacyTokens {
		if !old.hash.Valid || old.hash.String == "" {
			return fmt.Errorf("migrate API token %q: missing token hash", old.id.String)
		}
		id, parseErr := uuid.Parse(old.id.String)
		if parseErr != nil || id.Version() != 7 {
			id, err = uuid.NewV7()
			if err != nil {
				return fmt.Errorf("generate migrated API-token id: %w", err)
			}
		}
		owner := old.owner.String
		if !old.owner.Valid || owner == "" {
			ownerUUID, ownerErr := uuid.NewV7()
			if ownerErr != nil {
				return fmt.Errorf("generate legacy API-token owner id: %w", ownerErr)
			}
			owner = ownerUUID.String()
		}
		if _, err := tx.ExecContext(ctx, `INSERT OR IGNORE INTO owners(id) VALUES (?)`, owner); err != nil {
			return fmt.Errorf("preserve API-token owner %q: %w", owner, err)
		}
		tokenKind := old.kind.String
		if !old.kind.Valid || (tokenKind != "primary" && tokenKind != "linked") {
			tokenKind = "linked"
		}
		if old.prefix.String == "" {
			old.prefix.String = "legacy"
		}
		if _, err := tx.ExecContext(ctx, `INSERT INTO api_tokens_migrating
			(id, owner_id, kind, name, token_hash, token_prefix, token_last4, token_enc, created_at, rotated_at, expires_at, last_used_at, revoked_at, revoked_reason)
			VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`, id.String(), owner, tokenKind,
			old.name, old.hash.String, old.prefix.String, old.last4, old.encrypted,
			old.created.Int64, nullInt64Value(old.rotated), nullInt64Value(old.expires), nullInt64Value(old.lastUsed), nullInt64Value(old.revoked), old.revokedReason); err != nil {
			return fmt.Errorf("copy API token %q: %w", old.id.String, err)
		}
	}
	if _, err := tx.ExecContext(ctx, `DROP TABLE api_tokens`); err != nil {
		return fmt.Errorf("replace existing API-token table: %w", err)
	}
	if _, err := tx.ExecContext(ctx, `ALTER TABLE api_tokens_migrating RENAME TO api_tokens`); err != nil {
		return fmt.Errorf("install migrated API-token table: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit API-token schema migration: %w", err)
	}
	return nil
}

func ownerTokenTableDDL(table string) string {
	return `CREATE TABLE IF NOT EXISTS ` + table + ` (
		id TEXT PRIMARY KEY,
		owner_id TEXT NOT NULL REFERENCES owners(id) ON DELETE CASCADE,
		kind TEXT NOT NULL CHECK (kind IN ('primary', 'linked')),
		name TEXT,
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

func nullInt64Value(value sql.NullInt64) any {
	if !value.Valid {
		return nil
	}
	return value.Int64
}

// getTablesDDL returns CREATE TABLE statements.
// SQLite and libsql share the same DDL dialect per AI.md PART 10.
func (sm *SchemaManager) getTablesDDL() []string {
	return sm.getSQLiteDDL()
}

// getSQLiteDDL returns SQLite/libsql DDL
func (sm *SchemaManager) getSQLiteDDL() []string {
	return []string{
		// Audit log table for tracking actions
		`CREATE TABLE IF NOT EXISTS audit_log (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			timestamp DATETIME DEFAULT CURRENT_TIMESTAMP,
			user_id TEXT,
			username TEXT,
			action TEXT NOT NULL,
			resource TEXT,
			details TEXT,
			ip_address TEXT,
			user_agent TEXT
		)`,

		// Settings table for runtime config
		`CREATE TABLE IF NOT EXISTS settings (
			key TEXT PRIMARY KEY,
			value TEXT,
			type TEXT DEFAULT 'string',
			updated_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			updated_by TEXT
		)`,

		// Scheduled tasks table
		`CREATE TABLE IF NOT EXISTS scheduled_tasks (
			id TEXT PRIMARY KEY,
			name TEXT NOT NULL,
			schedule TEXT NOT NULL,
			enabled INTEGER DEFAULT 1,
			last_run DATETIME,
			next_run DATETIME,
			last_result TEXT,
			last_error TEXT,
			run_count INTEGER DEFAULT 0,
			fail_count INTEGER DEFAULT 0
		)`,

		// Task history table
		`CREATE TABLE IF NOT EXISTS task_history (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			task_id TEXT NOT NULL,
			start_time DATETIME NOT NULL,
			end_time DATETIME,
			duration_ms INTEGER,
			result TEXT,
			error TEXT,
			FOREIGN KEY (task_id) REFERENCES scheduled_tasks(id)
		)`,

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
	}
}

// GetDB returns the database connection
func (sm *SchemaManager) GetDB() *sql.DB {
	return sm.db
}

// Close closes the database connection
func (sm *SchemaManager) Close() error {
	return sm.db.Close()
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
		"audit_log", "settings", "scheduled_tasks", "task_history",
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
