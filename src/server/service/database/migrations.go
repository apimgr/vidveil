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
// Supports SQLite, PostgreSQL, MySQL, and MSSQL backends
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
// Supports SQLite, PostgreSQL, MySQL, and MSSQL per AI.md PART 10
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
// Generates driver-specific DDL for SQLite, PostgreSQL, MySQL, and MSSQL
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

// getTablesDDL returns CREATE TABLE statements for the current driver
// Per AI.md PART 10: Driver-specific syntax differences
func (sm *SchemaManager) getTablesDDL() []string {
	switch sm.driver {
	case DriverPostgres:
		return sm.getPostgresDDL()
	case DriverMySQL:
		return sm.getMySQLDDL()
	case DriverMSSQL:
		return sm.getMSSQLDDL()
	default:
		return sm.getSQLiteDDL()
	}
}

// getSQLiteDDL returns SQLite-specific DDL
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

// getPostgresDDL returns PostgreSQL-specific DDL per AI.md PART 10
func (sm *SchemaManager) getPostgresDDL() []string {
	return []string{
		`CREATE TABLE IF NOT EXISTS audit_log (
			id SERIAL PRIMARY KEY,
			timestamp TIMESTAMP DEFAULT NOW(),
			user_id TEXT,
			username TEXT,
			action TEXT NOT NULL,
			resource TEXT,
			details TEXT,
			ip_address TEXT,
			user_agent TEXT
		)`,

		`CREATE TABLE IF NOT EXISTS settings (
			key TEXT PRIMARY KEY,
			value TEXT,
			type TEXT DEFAULT 'string',
			updated_at TIMESTAMP DEFAULT NOW(),
			updated_by TEXT
		)`,

		`CREATE TABLE IF NOT EXISTS scheduled_tasks (
			id TEXT PRIMARY KEY,
			name TEXT NOT NULL,
			schedule TEXT NOT NULL,
			enabled BOOLEAN DEFAULT TRUE,
			last_run TIMESTAMP,
			next_run TIMESTAMP,
			last_result TEXT,
			last_error TEXT,
			run_count INTEGER DEFAULT 0,
			fail_count INTEGER DEFAULT 0
		)`,

		`CREATE TABLE IF NOT EXISTS task_history (
			id SERIAL PRIMARY KEY,
			task_id TEXT NOT NULL REFERENCES scheduled_tasks(id),
			start_time TIMESTAMP NOT NULL,
			end_time TIMESTAMP,
			duration_ms INTEGER,
			result TEXT,
			error TEXT
		)`,

		// App secrets table per AI.md PART 11
		`CREATE TABLE IF NOT EXISTS app_secrets (
			key TEXT PRIMARY KEY,
			value TEXT NOT NULL,
			created_at TIMESTAMP DEFAULT NOW(),
			rotated_at TIMESTAMP,
			expires_at TIMESTAMP,
			previous_value TEXT
		)`,

		// Notifications table per AI.md PART 17
		`CREATE TABLE IF NOT EXISTS notifications (
			id TEXT PRIMARY KEY,
			type TEXT NOT NULL,
			title TEXT NOT NULL,
			message TEXT,
			targets INTEGER NOT NULL,
			created_at TIMESTAMP DEFAULT NOW(),
			read_at TIMESTAMP,
			details TEXT
		)`,
	}
}

// getMySQLDDL returns MySQL-specific DDL per AI.md PART 10
func (sm *SchemaManager) getMySQLDDL() []string {
	return []string{
		`CREATE TABLE IF NOT EXISTS audit_log (
			id INT AUTO_INCREMENT PRIMARY KEY,
			timestamp TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
			user_id VARCHAR(255),
			username VARCHAR(255),
			action VARCHAR(255) NOT NULL,
			resource VARCHAR(255),
			details TEXT,
			ip_address VARCHAR(45),
			user_agent TEXT
		)`,

		`CREATE TABLE IF NOT EXISTS settings (
			` + "`key`" + ` VARCHAR(255) PRIMARY KEY,
			value TEXT,
			type VARCHAR(50) DEFAULT 'string',
			updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
			updated_by VARCHAR(255)
		)`,

		`CREATE TABLE IF NOT EXISTS scheduled_tasks (
			id VARCHAR(255) PRIMARY KEY,
			name VARCHAR(255) NOT NULL,
			schedule VARCHAR(255) NOT NULL,
			enabled TINYINT(1) DEFAULT 1,
			last_run TIMESTAMP NULL,
			next_run TIMESTAMP NULL,
			last_result TEXT,
			last_error TEXT,
			run_count INT DEFAULT 0,
			fail_count INT DEFAULT 0
		)`,

		`CREATE TABLE IF NOT EXISTS task_history (
			id INT AUTO_INCREMENT PRIMARY KEY,
			task_id VARCHAR(255) NOT NULL,
			start_time TIMESTAMP NOT NULL,
			end_time TIMESTAMP NULL,
			duration_ms INT,
			result TEXT,
			error TEXT,
			FOREIGN KEY (task_id) REFERENCES scheduled_tasks(id)
		)`,

		// App secrets table per AI.md PART 11
		// MySQL requires backtick-quoted "key" since it's a reserved word
		"CREATE TABLE IF NOT EXISTS app_secrets (" +
			"`key` VARCHAR(255) PRIMARY KEY," +
			"value TEXT NOT NULL," +
			"created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP," +
			"rotated_at TIMESTAMP NULL," +
			"expires_at TIMESTAMP NULL," +
			"previous_value TEXT" +
			")",

		// Notifications table per AI.md PART 17
		`CREATE TABLE IF NOT EXISTS notifications (
			id VARCHAR(255) PRIMARY KEY,
			type VARCHAR(50) NOT NULL,
			title VARCHAR(255) NOT NULL,
			message TEXT,
			targets INT NOT NULL,
			created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
			read_at TIMESTAMP NULL,
			details TEXT
		)`,
	}
}

// getMSSQLDDL returns Microsoft SQL Server-specific DDL per AI.md PART 10
func (sm *SchemaManager) getMSSQLDDL() []string {
	return []string{
		`IF NOT EXISTS (SELECT * FROM sys.tables WHERE name = 'audit_log')
		CREATE TABLE audit_log (
			id INT IDENTITY(1,1) PRIMARY KEY,
			timestamp DATETIME2 DEFAULT GETDATE(),
			user_id NVARCHAR(255),
			username NVARCHAR(255),
			action NVARCHAR(255) NOT NULL,
			resource NVARCHAR(255),
			details NVARCHAR(MAX),
			ip_address NVARCHAR(45),
			user_agent NVARCHAR(MAX)
		)`,

		`IF NOT EXISTS (SELECT * FROM sys.tables WHERE name = 'settings')
		CREATE TABLE settings (
			[key] NVARCHAR(255) PRIMARY KEY,
			value NVARCHAR(MAX),
			type NVARCHAR(50) DEFAULT 'string',
			updated_at DATETIME2 DEFAULT GETDATE(),
			updated_by NVARCHAR(255)
		)`,

		`IF NOT EXISTS (SELECT * FROM sys.tables WHERE name = 'scheduled_tasks')
		CREATE TABLE scheduled_tasks (
			id NVARCHAR(255) PRIMARY KEY,
			name NVARCHAR(255) NOT NULL,
			schedule NVARCHAR(255) NOT NULL,
			enabled BIT DEFAULT 1,
			last_run DATETIME2,
			next_run DATETIME2,
			last_result NVARCHAR(MAX),
			last_error NVARCHAR(MAX),
			run_count INT DEFAULT 0,
			fail_count INT DEFAULT 0
		)`,

		`IF NOT EXISTS (SELECT * FROM sys.tables WHERE name = 'task_history')
		CREATE TABLE task_history (
			id INT IDENTITY(1,1) PRIMARY KEY,
			task_id NVARCHAR(255) NOT NULL,
			start_time DATETIME2 NOT NULL,
			end_time DATETIME2,
			duration_ms INT,
			result NVARCHAR(MAX),
			error NVARCHAR(MAX),
			FOREIGN KEY (task_id) REFERENCES scheduled_tasks(id)
		)`,

		// App secrets table per AI.md PART 11
		`IF NOT EXISTS (SELECT * FROM sys.tables WHERE name = 'app_secrets')
		CREATE TABLE app_secrets (
			[key] NVARCHAR(255) PRIMARY KEY,
			value NVARCHAR(MAX) NOT NULL,
			created_at DATETIME2 DEFAULT GETDATE(),
			rotated_at DATETIME2,
			expires_at DATETIME2,
			previous_value NVARCHAR(MAX)
		)`,

		// Notifications table per AI.md PART 17
		`IF NOT EXISTS (SELECT * FROM sys.tables WHERE name = 'notifications')
		CREATE TABLE notifications (
			id NVARCHAR(255) PRIMARY KEY,
			type NVARCHAR(50) NOT NULL,
			title NVARCHAR(255) NOT NULL,
			message NVARCHAR(MAX),
			targets INT NOT NULL,
			created_at DATETIME2 DEFAULT GETDATE(),
			read_at DATETIME2,
			details NVARCHAR(MAX)
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
// Supports SQLite, PostgreSQL, MySQL, and MSSQL
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

// tableExists checks if a table exists using driver-specific query
func (sm *SchemaManager) tableExists(ctx context.Context, tableName string) (bool, error) {
	var query string
	var args []interface{}

	switch sm.driver {
	case DriverPostgres:
		query = "SELECT EXISTS (SELECT FROM information_schema.tables WHERE table_name = $1)"
		args = []interface{}{tableName}
	case DriverMySQL:
		query = "SELECT COUNT(*) FROM information_schema.tables WHERE table_schema = DATABASE() AND table_name = ?"
		args = []interface{}{tableName}
	case DriverMSSQL:
		query = "SELECT COUNT(*) FROM sys.tables WHERE name = @p1"
		args = []interface{}{tableName}
	default:
		query = "SELECT COUNT(*) FROM sqlite_master WHERE type='table' AND name=?"
		args = []interface{}{tableName}
	}

	var count int
	err := sm.db.QueryRowContext(ctx, query, args...).Scan(&count)
	return count > 0, err
}

// RollbackMigration is not supported with simple schema management
// Per AI.md PART 10: No migrations table, rollback not tracked
func (sm *SchemaManager) RollbackMigration() error {
	return fmt.Errorf("rollback not supported: per AI.md PART 10, use CREATE TABLE IF NOT EXISTS pattern")
}

// isColumnExistsError checks whether an error indicates a column already exists.
// Per AI.md PART 10: Used with ALTER TABLE ADD COLUMN to make schema updates idempotent.
// Covers SQLite ("duplicate column"), PostgreSQL ("already exists"), MySQL ("Duplicate column"),
// and MSSQL ("Column names in each table must be unique").
func isColumnExistsError(err error) bool {
	if err == nil {
		return false
	}
	msg := err.Error()
	return strings.Contains(msg, "duplicate column") ||
		strings.Contains(msg, "already exists") ||
		strings.Contains(msg, "Duplicate column") ||
		strings.Contains(msg, "Column names in each table must be unique")
}

// isSerializationError checks whether an error is a database serialization / lock conflict.
// Per AI.md PART 10: Used with WithSerializableRetry to retry on transient lock failures.
// Covers SQLite ("SQLITE_BUSY", "database is locked") and PostgreSQL ("could not serialize").
func isSerializationError(err error) bool {
	if err == nil {
		return false
	}
	msg := err.Error()
	return strings.Contains(msg, "SQLITE_BUSY") ||
		strings.Contains(msg, "database is locked") ||
		strings.Contains(msg, "could not serialize")
}
