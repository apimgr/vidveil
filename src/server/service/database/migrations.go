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

	return nil
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
