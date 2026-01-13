// SPDX-License-Identifier: MIT
// AI.md PART 10: Database & Cluster - Schema Management
// Per PART 10: "ALL apps use CREATE TABLE IF NOT EXISTS for self-creating schema"
// Per PART 10: "No migrations table | Keep it simple"
package database

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"time"

	_ "modernc.org/sqlite"
)

// SchemaManager handles database schema creation per AI.md PART 10
// Uses CREATE TABLE IF NOT EXISTS - no migrations tracking table
type SchemaManager struct {
	db     *sql.DB
	dbPath string
}

// NewSchemaManager creates a new schema manager
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
	}, nil
}

// EnsureSchema creates all required tables if they don't exist
// Per AI.md PART 10: Idempotent, safe to run multiple times
func (sm *SchemaManager) EnsureSchema() error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	// Create all tables using CREATE TABLE IF NOT EXISTS
	tables := []string{
		// Sessions table for admin authentication
		`CREATE TABLE IF NOT EXISTS sessions (
			id TEXT PRIMARY KEY,
			user_id TEXT NOT NULL,
			username TEXT NOT NULL,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			expires_at DATETIME NOT NULL,
			ip_address TEXT,
			user_agent TEXT
		)`,

		// Audit log table for tracking admin actions
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

		// Cluster nodes table for distributed mode
		`CREATE TABLE IF NOT EXISTS cluster_nodes (
			id TEXT PRIMARY KEY,
			hostname TEXT NOT NULL,
			address TEXT NOT NULL,
			port INTEGER NOT NULL,
			is_primary INTEGER DEFAULT 0,
			last_heartbeat DATETIME,
			joined_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			status TEXT DEFAULT 'active'
		)`,

		// Distributed locks table for cluster coordination
		`CREATE TABLE IF NOT EXISTS distributed_locks (
			name TEXT PRIMARY KEY,
			holder_id TEXT NOT NULL,
			acquired_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			expires_at DATETIME NOT NULL,
			metadata TEXT
		)`,

		// Notifications table
		`CREATE TABLE IF NOT EXISTS notifications (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			type TEXT NOT NULL,
			title TEXT NOT NULL,
			message TEXT,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			read_at DATETIME,
			dismissed_at DATETIME,
			priority TEXT DEFAULT 'normal',
			metadata TEXT
		)`,

		// Admin credentials table per AI.md PART 31
		`CREATE TABLE IF NOT EXISTS admin_credentials (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			username TEXT UNIQUE NOT NULL,
			password_hash TEXT NOT NULL,
			totp_secret TEXT,
			totp_enabled INTEGER DEFAULT 0,
			totp_backup_codes TEXT,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			updated_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			last_login DATETIME,
			login_count INTEGER DEFAULT 0,
			is_primary INTEGER DEFAULT 0,
			invited_by INTEGER,
			invite_token TEXT,
			invite_expires DATETIME,
			FOREIGN KEY (invited_by) REFERENCES admin_credentials(id)
		)`,

		// Setup tokens table per AI.md PART 31
		`CREATE TABLE IF NOT EXISTS setup_tokens (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			token TEXT UNIQUE NOT NULL,
			purpose TEXT NOT NULL,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			expires_at DATETIME NOT NULL,
			used_at DATETIME,
			used_by TEXT
		)`,

		// API tokens table per AI.md PART 31
		`CREATE TABLE IF NOT EXISTS api_tokens (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			admin_id INTEGER NOT NULL,
			name TEXT NOT NULL,
			token_hash TEXT UNIQUE NOT NULL,
			token_prefix TEXT NOT NULL,
			permissions TEXT DEFAULT '*',
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			expires_at DATETIME,
			last_used DATETIME,
			use_count INTEGER DEFAULT 0,
			FOREIGN KEY (admin_id) REFERENCES admin_credentials(id)
		)`,

		// SMTP config table per AI.md PART 31
		`CREATE TABLE IF NOT EXISTS smtp_config (
			id INTEGER PRIMARY KEY CHECK (id = 1),
			host TEXT,
			port INTEGER DEFAULT 587,
			username TEXT,
			password_encrypted TEXT,
			from_address TEXT,
			from_name TEXT,
			encryption TEXT DEFAULT 'tls',
			verified INTEGER DEFAULT 0,
			verified_at DATETIME,
			auto_detected INTEGER DEFAULT 0,
			updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
		)`,

		// Recovery keys table for 2FA backup
		`CREATE TABLE IF NOT EXISTS recovery_keys (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			admin_id INTEGER NOT NULL,
			key_hash TEXT NOT NULL,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			used_at DATETIME,
			FOREIGN KEY (admin_id) REFERENCES admin_credentials(id) ON DELETE CASCADE
		)`,

		// Pages table for standard page content
		`CREATE TABLE IF NOT EXISTS pages (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			slug TEXT NOT NULL UNIQUE,
			title TEXT NOT NULL,
			content TEXT NOT NULL,
			meta_description TEXT,
			enabled INTEGER DEFAULT 1,
			updated_by INTEGER,
			updated_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			FOREIGN KEY (updated_by) REFERENCES admin_credentials(id)
		)`,
	}

	// Execute all table creation statements
	for _, ddl := range tables {
		if _, err := sm.db.ExecContext(ctx, ddl); err != nil {
			return fmt.Errorf("failed to create table: %w", err)
		}
	}

	// Insert default pages if not exist
	_, err := sm.db.ExecContext(ctx, `
		INSERT OR IGNORE INTO pages (slug, title, content, meta_description) VALUES
		('about', 'About', 'Welcome to our service. This page describes what we do and our mission.', 'About our service'),
		('privacy', 'Privacy Policy', 'Your privacy is important to us. This policy describes how we handle your data.', 'Privacy policy'),
		('contact', 'Contact Us', 'Get in touch with us using the form below or via email.', 'Contact information'),
		('help', 'Help & FAQ', 'Find answers to common questions and get help with our service.', 'Help and frequently asked questions')
	`)
	if err != nil {
		return fmt.Errorf("failed to insert default pages: %w", err)
	}

	return nil
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
		"sessions", "audit_log", "settings", "scheduled_tasks", "task_history",
		"cluster_nodes", "distributed_locks", "notifications", "admin_credentials",
		"setup_tokens", "api_tokens", "smtp_config", "recovery_keys", "pages",
	}

	var status []map[string]interface{}
	for _, table := range tables {
		exists := false
		// Check if table exists
		row := sm.db.QueryRowContext(ctx,
			"SELECT name FROM sqlite_master WHERE type='table' AND name=?", table)
		var name string
		if err := row.Scan(&name); err == nil {
			exists = true
		}

		status = append(status, map[string]interface{}{
			"name":    table,
			"applied": exists,
		})
	}

	return status, nil
}

// RollbackMigration is not supported with simple schema management
// Per AI.md PART 10: No migrations table, rollback not tracked
func (sm *SchemaManager) RollbackMigration() error {
	return fmt.Errorf("rollback not supported: per AI.md PART 10, use CREATE TABLE IF NOT EXISTS pattern")
}
