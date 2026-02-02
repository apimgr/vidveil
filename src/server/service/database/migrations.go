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

	// Insert default pages if not exist (driver-specific syntax)
	if err := sm.insertDefaultPages(ctx); err != nil {
		return fmt.Errorf("failed to insert default pages: %w", err)
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
}

// getPostgresDDL returns PostgreSQL-specific DDL per AI.md PART 10
func (sm *SchemaManager) getPostgresDDL() []string {
	return []string{
		`CREATE TABLE IF NOT EXISTS sessions (
			id TEXT PRIMARY KEY,
			user_id TEXT NOT NULL,
			username TEXT NOT NULL,
			created_at TIMESTAMP DEFAULT NOW(),
			expires_at TIMESTAMP NOT NULL,
			ip_address TEXT,
			user_agent TEXT
		)`,

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

		`CREATE TABLE IF NOT EXISTS cluster_nodes (
			id TEXT PRIMARY KEY,
			hostname TEXT NOT NULL,
			address TEXT NOT NULL,
			port INTEGER NOT NULL,
			is_primary BOOLEAN DEFAULT FALSE,
			last_heartbeat TIMESTAMP,
			joined_at TIMESTAMP DEFAULT NOW(),
			status TEXT DEFAULT 'active'
		)`,

		`CREATE TABLE IF NOT EXISTS distributed_locks (
			name TEXT PRIMARY KEY,
			holder_id TEXT NOT NULL,
			acquired_at TIMESTAMP DEFAULT NOW(),
			expires_at TIMESTAMP NOT NULL,
			metadata TEXT
		)`,

		`CREATE TABLE IF NOT EXISTS notifications (
			id SERIAL PRIMARY KEY,
			type TEXT NOT NULL,
			title TEXT NOT NULL,
			message TEXT,
			created_at TIMESTAMP DEFAULT NOW(),
			read_at TIMESTAMP,
			dismissed_at TIMESTAMP,
			priority TEXT DEFAULT 'normal',
			metadata TEXT
		)`,

		`CREATE TABLE IF NOT EXISTS admin_credentials (
			id SERIAL PRIMARY KEY,
			username TEXT UNIQUE NOT NULL,
			password_hash TEXT NOT NULL,
			totp_secret TEXT,
			totp_enabled BOOLEAN DEFAULT FALSE,
			totp_backup_codes TEXT,
			created_at TIMESTAMP DEFAULT NOW(),
			updated_at TIMESTAMP DEFAULT NOW(),
			last_login TIMESTAMP,
			login_count INTEGER DEFAULT 0,
			is_primary BOOLEAN DEFAULT FALSE,
			invited_by INTEGER REFERENCES admin_credentials(id),
			invite_token TEXT,
			invite_expires TIMESTAMP
		)`,

		`CREATE TABLE IF NOT EXISTS setup_tokens (
			id SERIAL PRIMARY KEY,
			token TEXT UNIQUE NOT NULL,
			purpose TEXT NOT NULL,
			created_at TIMESTAMP DEFAULT NOW(),
			expires_at TIMESTAMP NOT NULL,
			used_at TIMESTAMP,
			used_by TEXT
		)`,

		`CREATE TABLE IF NOT EXISTS api_tokens (
			id SERIAL PRIMARY KEY,
			admin_id INTEGER NOT NULL REFERENCES admin_credentials(id),
			name TEXT NOT NULL,
			token_hash TEXT UNIQUE NOT NULL,
			token_prefix TEXT NOT NULL,
			permissions TEXT DEFAULT '*',
			created_at TIMESTAMP DEFAULT NOW(),
			expires_at TIMESTAMP,
			last_used TIMESTAMP,
			use_count INTEGER DEFAULT 0
		)`,

		`CREATE TABLE IF NOT EXISTS smtp_config (
			id INTEGER PRIMARY KEY CHECK (id = 1),
			host TEXT,
			port INTEGER DEFAULT 587,
			username TEXT,
			password_encrypted TEXT,
			from_address TEXT,
			from_name TEXT,
			encryption TEXT DEFAULT 'tls',
			verified BOOLEAN DEFAULT FALSE,
			verified_at TIMESTAMP,
			auto_detected BOOLEAN DEFAULT FALSE,
			updated_at TIMESTAMP DEFAULT NOW()
		)`,

		`CREATE TABLE IF NOT EXISTS recovery_keys (
			id SERIAL PRIMARY KEY,
			admin_id INTEGER NOT NULL REFERENCES admin_credentials(id) ON DELETE CASCADE,
			key_hash TEXT NOT NULL,
			created_at TIMESTAMP DEFAULT NOW(),
			used_at TIMESTAMP
		)`,

		`CREATE TABLE IF NOT EXISTS pages (
			id SERIAL PRIMARY KEY,
			slug TEXT NOT NULL UNIQUE,
			title TEXT NOT NULL,
			content TEXT NOT NULL,
			meta_description TEXT,
			enabled BOOLEAN DEFAULT TRUE,
			updated_by INTEGER REFERENCES admin_credentials(id),
			updated_at TIMESTAMP DEFAULT NOW()
		)`,
	}
}

// getMySQLDDL returns MySQL-specific DDL per AI.md PART 10
func (sm *SchemaManager) getMySQLDDL() []string {
	return []string{
		`CREATE TABLE IF NOT EXISTS sessions (
			id VARCHAR(255) PRIMARY KEY,
			user_id VARCHAR(255) NOT NULL,
			username VARCHAR(255) NOT NULL,
			created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
			expires_at TIMESTAMP NOT NULL,
			ip_address VARCHAR(45),
			user_agent TEXT
		)`,

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

		`CREATE TABLE IF NOT EXISTS cluster_nodes (
			id VARCHAR(255) PRIMARY KEY,
			hostname VARCHAR(255) NOT NULL,
			address VARCHAR(255) NOT NULL,
			port INT NOT NULL,
			is_primary TINYINT(1) DEFAULT 0,
			last_heartbeat TIMESTAMP NULL,
			joined_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
			status VARCHAR(50) DEFAULT 'active'
		)`,

		`CREATE TABLE IF NOT EXISTS distributed_locks (
			name VARCHAR(255) PRIMARY KEY,
			holder_id VARCHAR(255) NOT NULL,
			acquired_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
			expires_at TIMESTAMP NOT NULL,
			metadata TEXT
		)`,

		`CREATE TABLE IF NOT EXISTS notifications (
			id INT AUTO_INCREMENT PRIMARY KEY,
			type VARCHAR(50) NOT NULL,
			title VARCHAR(255) NOT NULL,
			message TEXT,
			created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
			read_at TIMESTAMP NULL,
			dismissed_at TIMESTAMP NULL,
			priority VARCHAR(50) DEFAULT 'normal',
			metadata TEXT
		)`,

		`CREATE TABLE IF NOT EXISTS admin_credentials (
			id INT AUTO_INCREMENT PRIMARY KEY,
			username VARCHAR(255) UNIQUE NOT NULL,
			password_hash VARCHAR(255) NOT NULL,
			totp_secret VARCHAR(255),
			totp_enabled TINYINT(1) DEFAULT 0,
			totp_backup_codes TEXT,
			created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
			updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
			last_login TIMESTAMP NULL,
			login_count INT DEFAULT 0,
			is_primary TINYINT(1) DEFAULT 0,
			invited_by INT,
			invite_token VARCHAR(255),
			invite_expires TIMESTAMP NULL,
			FOREIGN KEY (invited_by) REFERENCES admin_credentials(id)
		)`,

		`CREATE TABLE IF NOT EXISTS setup_tokens (
			id INT AUTO_INCREMENT PRIMARY KEY,
			token VARCHAR(255) UNIQUE NOT NULL,
			purpose VARCHAR(50) NOT NULL,
			created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
			expires_at TIMESTAMP NOT NULL,
			used_at TIMESTAMP NULL,
			used_by VARCHAR(255)
		)`,

		`CREATE TABLE IF NOT EXISTS api_tokens (
			id INT AUTO_INCREMENT PRIMARY KEY,
			admin_id INT NOT NULL,
			name VARCHAR(255) NOT NULL,
			token_hash VARCHAR(255) UNIQUE NOT NULL,
			token_prefix VARCHAR(50) NOT NULL,
			permissions TEXT DEFAULT '*',
			created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
			expires_at TIMESTAMP NULL,
			last_used TIMESTAMP NULL,
			use_count INT DEFAULT 0,
			FOREIGN KEY (admin_id) REFERENCES admin_credentials(id)
		)`,

		`CREATE TABLE IF NOT EXISTS smtp_config (
			id INT PRIMARY KEY CHECK (id = 1),
			host VARCHAR(255),
			port INT DEFAULT 587,
			username VARCHAR(255),
			password_encrypted TEXT,
			from_address VARCHAR(255),
			from_name VARCHAR(255),
			encryption VARCHAR(50) DEFAULT 'tls',
			verified TINYINT(1) DEFAULT 0,
			verified_at TIMESTAMP NULL,
			auto_detected TINYINT(1) DEFAULT 0,
			updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP
		)`,

		`CREATE TABLE IF NOT EXISTS recovery_keys (
			id INT AUTO_INCREMENT PRIMARY KEY,
			admin_id INT NOT NULL,
			key_hash VARCHAR(255) NOT NULL,
			created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
			used_at TIMESTAMP NULL,
			FOREIGN KEY (admin_id) REFERENCES admin_credentials(id) ON DELETE CASCADE
		)`,

		`CREATE TABLE IF NOT EXISTS pages (
			id INT AUTO_INCREMENT PRIMARY KEY,
			slug VARCHAR(255) NOT NULL UNIQUE,
			title VARCHAR(255) NOT NULL,
			content TEXT NOT NULL,
			meta_description TEXT,
			enabled TINYINT(1) DEFAULT 1,
			updated_by INT,
			updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
			FOREIGN KEY (updated_by) REFERENCES admin_credentials(id)
		)`,
	}
}

// getMSSQLDDL returns Microsoft SQL Server-specific DDL per AI.md PART 10
func (sm *SchemaManager) getMSSQLDDL() []string {
	return []string{
		`IF NOT EXISTS (SELECT * FROM sys.tables WHERE name = 'sessions')
		CREATE TABLE sessions (
			id NVARCHAR(255) PRIMARY KEY,
			user_id NVARCHAR(255) NOT NULL,
			username NVARCHAR(255) NOT NULL,
			created_at DATETIME2 DEFAULT GETDATE(),
			expires_at DATETIME2 NOT NULL,
			ip_address NVARCHAR(45),
			user_agent NVARCHAR(MAX)
		)`,

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

		`IF NOT EXISTS (SELECT * FROM sys.tables WHERE name = 'cluster_nodes')
		CREATE TABLE cluster_nodes (
			id NVARCHAR(255) PRIMARY KEY,
			hostname NVARCHAR(255) NOT NULL,
			address NVARCHAR(255) NOT NULL,
			port INT NOT NULL,
			is_primary BIT DEFAULT 0,
			last_heartbeat DATETIME2,
			joined_at DATETIME2 DEFAULT GETDATE(),
			status NVARCHAR(50) DEFAULT 'active'
		)`,

		`IF NOT EXISTS (SELECT * FROM sys.tables WHERE name = 'distributed_locks')
		CREATE TABLE distributed_locks (
			name NVARCHAR(255) PRIMARY KEY,
			holder_id NVARCHAR(255) NOT NULL,
			acquired_at DATETIME2 DEFAULT GETDATE(),
			expires_at DATETIME2 NOT NULL,
			metadata NVARCHAR(MAX)
		)`,

		`IF NOT EXISTS (SELECT * FROM sys.tables WHERE name = 'notifications')
		CREATE TABLE notifications (
			id INT IDENTITY(1,1) PRIMARY KEY,
			type NVARCHAR(50) NOT NULL,
			title NVARCHAR(255) NOT NULL,
			message NVARCHAR(MAX),
			created_at DATETIME2 DEFAULT GETDATE(),
			read_at DATETIME2,
			dismissed_at DATETIME2,
			priority NVARCHAR(50) DEFAULT 'normal',
			metadata NVARCHAR(MAX)
		)`,

		`IF NOT EXISTS (SELECT * FROM sys.tables WHERE name = 'admin_credentials')
		CREATE TABLE admin_credentials (
			id INT IDENTITY(1,1) PRIMARY KEY,
			username NVARCHAR(255) UNIQUE NOT NULL,
			password_hash NVARCHAR(255) NOT NULL,
			totp_secret NVARCHAR(255),
			totp_enabled BIT DEFAULT 0,
			totp_backup_codes NVARCHAR(MAX),
			created_at DATETIME2 DEFAULT GETDATE(),
			updated_at DATETIME2 DEFAULT GETDATE(),
			last_login DATETIME2,
			login_count INT DEFAULT 0,
			is_primary BIT DEFAULT 0,
			invited_by INT,
			invite_token NVARCHAR(255),
			invite_expires DATETIME2
		)`,

		`IF NOT EXISTS (SELECT * FROM sys.tables WHERE name = 'setup_tokens')
		CREATE TABLE setup_tokens (
			id INT IDENTITY(1,1) PRIMARY KEY,
			token NVARCHAR(255) UNIQUE NOT NULL,
			purpose NVARCHAR(50) NOT NULL,
			created_at DATETIME2 DEFAULT GETDATE(),
			expires_at DATETIME2 NOT NULL,
			used_at DATETIME2,
			used_by NVARCHAR(255)
		)`,

		`IF NOT EXISTS (SELECT * FROM sys.tables WHERE name = 'api_tokens')
		CREATE TABLE api_tokens (
			id INT IDENTITY(1,1) PRIMARY KEY,
			admin_id INT NOT NULL,
			name NVARCHAR(255) NOT NULL,
			token_hash NVARCHAR(255) UNIQUE NOT NULL,
			token_prefix NVARCHAR(50) NOT NULL,
			permissions NVARCHAR(MAX) DEFAULT '*',
			created_at DATETIME2 DEFAULT GETDATE(),
			expires_at DATETIME2,
			last_used DATETIME2,
			use_count INT DEFAULT 0,
			FOREIGN KEY (admin_id) REFERENCES admin_credentials(id)
		)`,

		`IF NOT EXISTS (SELECT * FROM sys.tables WHERE name = 'smtp_config')
		CREATE TABLE smtp_config (
			id INT PRIMARY KEY CHECK (id = 1),
			host NVARCHAR(255),
			port INT DEFAULT 587,
			username NVARCHAR(255),
			password_encrypted NVARCHAR(MAX),
			from_address NVARCHAR(255),
			from_name NVARCHAR(255),
			encryption NVARCHAR(50) DEFAULT 'tls',
			verified BIT DEFAULT 0,
			verified_at DATETIME2,
			auto_detected BIT DEFAULT 0,
			updated_at DATETIME2 DEFAULT GETDATE()
		)`,

		`IF NOT EXISTS (SELECT * FROM sys.tables WHERE name = 'recovery_keys')
		CREATE TABLE recovery_keys (
			id INT IDENTITY(1,1) PRIMARY KEY,
			admin_id INT NOT NULL,
			key_hash NVARCHAR(255) NOT NULL,
			created_at DATETIME2 DEFAULT GETDATE(),
			used_at DATETIME2,
			FOREIGN KEY (admin_id) REFERENCES admin_credentials(id) ON DELETE CASCADE
		)`,

		`IF NOT EXISTS (SELECT * FROM sys.tables WHERE name = 'pages')
		CREATE TABLE pages (
			id INT IDENTITY(1,1) PRIMARY KEY,
			slug NVARCHAR(255) NOT NULL UNIQUE,
			title NVARCHAR(255) NOT NULL,
			content NVARCHAR(MAX) NOT NULL,
			meta_description NVARCHAR(MAX),
			enabled BIT DEFAULT 1,
			updated_by INT,
			updated_at DATETIME2 DEFAULT GETDATE(),
			FOREIGN KEY (updated_by) REFERENCES admin_credentials(id)
		)`,
	}
}

// insertDefaultPages inserts default pages using driver-specific syntax
func (sm *SchemaManager) insertDefaultPages(ctx context.Context) error {
	pages := []struct {
		slug, title, content, metaDesc string
	}{
		{"about", "About", "Welcome to our service. This page describes what we do and our mission.", "About our service"},
		{"privacy", "Privacy Policy", "Your privacy is important to us. This policy describes how we handle your data.", "Privacy policy"},
		{"contact", "Contact Us", "Get in touch with us using the form below or via email.", "Contact information"},
		{"help", "Help & FAQ", "Find answers to common questions and get help with our service.", "Help and frequently asked questions"},
	}

	for _, p := range pages {
		var query string
		switch sm.driver {
		case DriverPostgres:
			query = `INSERT INTO pages (slug, title, content, meta_description) VALUES ($1, $2, $3, $4) ON CONFLICT (slug) DO NOTHING`
		case DriverMySQL:
			query = `INSERT IGNORE INTO pages (slug, title, content, meta_description) VALUES (?, ?, ?, ?)`
		case DriverMSSQL:
			// MSSQL doesn't have INSERT IGNORE, use MERGE or check existence
			query = `IF NOT EXISTS (SELECT 1 FROM pages WHERE slug = @p1) INSERT INTO pages (slug, title, content, meta_description) VALUES (@p1, @p2, @p3, @p4)`
		default:
			query = `INSERT OR IGNORE INTO pages (slug, title, content, meta_description) VALUES (?, ?, ?, ?)`
		}

		if _, err := sm.db.ExecContext(ctx, query, p.slug, p.title, p.content, p.metaDesc); err != nil {
			return err
		}
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
// Supports SQLite, PostgreSQL, MySQL, and MSSQL
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
