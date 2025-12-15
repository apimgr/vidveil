// SPDX-License-Identifier: MIT
// TEMPLATE.md PART 20: Database Migrations
package database

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"sync"
	"time"

	_ "modernc.org/sqlite"
)

// Migration represents a database migration
type Migration struct {
	Version     int64     `json:"version"`
	Name        string    `json:"name"`
	AppliedAt   time.Time `json:"applied_at"`
	Description string    `json:"description"`
	Up          func(*sql.Tx) error `json:"-"`
	Down        func(*sql.Tx) error `json:"-"`
}

// MigrationManager handles database migrations per TEMPLATE.md PART 20
type MigrationManager struct {
	db         *sql.DB
	dbPath     string
	migrations []*Migration
	mu         sync.Mutex
}

// NewMigrationManager creates a new migration manager
func NewMigrationManager(dbPath string) (*MigrationManager, error) {
	// Ensure directory exists
	dir := filepath.Dir(dbPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create database directory: %w", err)
	}

	db, err := sql.Open("sqlite", dbPath+"?_journal_mode=WAL&_busy_timeout=5000")
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}

	mm := &MigrationManager{
		db:         db,
		dbPath:     dbPath,
		migrations: make([]*Migration, 0),
	}

	// Create schema_migrations table if it doesn't exist
	if err := mm.createMigrationsTable(); err != nil {
		db.Close()
		return nil, err
	}

	return mm, nil
}

// createMigrationsTable creates the schema_migrations table per TEMPLATE.md
func (mm *MigrationManager) createMigrationsTable() error {
	_, err := mm.db.Exec(`
		CREATE TABLE IF NOT EXISTS schema_migrations (
			version INTEGER PRIMARY KEY,
			name TEXT NOT NULL,
			description TEXT,
			applied_at DATETIME DEFAULT CURRENT_TIMESTAMP
		)
	`)
	return err
}

// RegisterMigration registers a new migration
func (mm *MigrationManager) RegisterMigration(version int64, name, description string, up, down func(*sql.Tx) error) {
	mm.mu.Lock()
	defer mm.mu.Unlock()

	mm.migrations = append(mm.migrations, &Migration{
		Version:     version,
		Name:        name,
		Description: description,
		Up:          up,
		Down:        down,
	})

	// Keep migrations sorted by version
	sort.Slice(mm.migrations, func(i, j int) bool {
		return mm.migrations[i].Version < mm.migrations[j].Version
	})
}

// RunMigrations runs all pending migrations on startup per TEMPLATE.md
func (mm *MigrationManager) RunMigrations() error {
	mm.mu.Lock()
	defer mm.mu.Unlock()

	applied, err := mm.getAppliedMigrations()
	if err != nil {
		return err
	}

	appliedMap := make(map[int64]bool)
	for _, v := range applied {
		appliedMap[v] = true
	}

	for _, m := range mm.migrations {
		if appliedMap[m.Version] {
			continue
		}

		if err := mm.applyMigration(m); err != nil {
			return fmt.Errorf("migration %d (%s) failed: %w", m.Version, m.Name, err)
		}
	}

	return nil
}

// applyMigration applies a single migration with automatic rollback on failure
func (mm *MigrationManager) applyMigration(m *Migration) error {
	tx, err := mm.db.Begin()
	if err != nil {
		return fmt.Errorf("failed to start transaction: %w", err)
	}

	// Run the up migration
	if err := m.Up(tx); err != nil {
		tx.Rollback()
		return fmt.Errorf("up migration failed: %w", err)
	}

	// Record the migration
	_, err = tx.Exec(
		"INSERT INTO schema_migrations (version, name, description) VALUES (?, ?, ?)",
		m.Version, m.Name, m.Description,
	)
	if err != nil {
		tx.Rollback()
		return fmt.Errorf("failed to record migration: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("failed to commit migration: %w", err)
	}

	fmt.Printf("Applied migration %d: %s\n", m.Version, m.Name)
	return nil
}

// RollbackMigration rolls back the last migration
func (mm *MigrationManager) RollbackMigration() error {
	mm.mu.Lock()
	defer mm.mu.Unlock()

	// Get the last applied migration
	var version int64
	var name string
	err := mm.db.QueryRow(
		"SELECT version, name FROM schema_migrations ORDER BY version DESC LIMIT 1",
	).Scan(&version, &name)
	if err != nil {
		if err == sql.ErrNoRows {
			return fmt.Errorf("no migrations to rollback")
		}
		return fmt.Errorf("failed to get last migration: %w", err)
	}

	// Find the migration
	var migration *Migration
	for _, m := range mm.migrations {
		if m.Version == version {
			migration = m
			break
		}
	}

	if migration == nil || migration.Down == nil {
		return fmt.Errorf("migration %d has no rollback function", version)
	}

	tx, err := mm.db.Begin()
	if err != nil {
		return fmt.Errorf("failed to start transaction: %w", err)
	}

	// Run the down migration
	if err := migration.Down(tx); err != nil {
		tx.Rollback()
		return fmt.Errorf("down migration failed: %w", err)
	}

	// Remove the migration record
	_, err = tx.Exec("DELETE FROM schema_migrations WHERE version = ?", version)
	if err != nil {
		tx.Rollback()
		return fmt.Errorf("failed to remove migration record: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("failed to commit rollback: %w", err)
	}

	fmt.Printf("Rolled back migration %d: %s\n", version, name)
	return nil
}

// getAppliedMigrations returns a list of applied migration versions
func (mm *MigrationManager) getAppliedMigrations() ([]int64, error) {
	rows, err := mm.db.Query("SELECT version FROM schema_migrations ORDER BY version")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var versions []int64
	for rows.Next() {
		var v int64
		if err := rows.Scan(&v); err != nil {
			return nil, err
		}
		versions = append(versions, v)
	}

	return versions, rows.Err()
}

// GetMigrationStatus returns the status of all migrations
func (mm *MigrationManager) GetMigrationStatus() ([]map[string]interface{}, error) {
	mm.mu.Lock()
	defer mm.mu.Unlock()

	applied, err := mm.getAppliedMigrations()
	if err != nil {
		return nil, err
	}

	appliedMap := make(map[int64]bool)
	for _, v := range applied {
		appliedMap[v] = true
	}

	// Get applied_at times
	appliedTimes := make(map[int64]time.Time)
	rows, err := mm.db.Query("SELECT version, applied_at FROM schema_migrations")
	if err == nil {
		defer rows.Close()
		for rows.Next() {
			var v int64
			var t time.Time
			if err := rows.Scan(&v, &t); err == nil {
				appliedTimes[v] = t
			}
		}
	}

	var status []map[string]interface{}
	for _, m := range mm.migrations {
		s := map[string]interface{}{
			"version":     m.Version,
			"name":        m.Name,
			"description": m.Description,
			"applied":     appliedMap[m.Version],
		}
		if t, ok := appliedTimes[m.Version]; ok {
			s["applied_at"] = t
		}
		status = append(status, s)
	}

	return status, nil
}

// GetDB returns the database connection
func (mm *MigrationManager) GetDB() *sql.DB {
	return mm.db
}

// Close closes the database connection
func (mm *MigrationManager) Close() error {
	return mm.db.Close()
}

// RegisterDefaultMigrations registers the default migrations for vidveil
func (mm *MigrationManager) RegisterDefaultMigrations() {
	// Migration 1: Create sessions table
	mm.RegisterMigration(1, "create_sessions_table", "Create sessions table for admin authentication", func(tx *sql.Tx) error {
		_, err := tx.Exec(`
			CREATE TABLE IF NOT EXISTS sessions (
				id TEXT PRIMARY KEY,
				user_id TEXT NOT NULL,
				username TEXT NOT NULL,
				created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
				expires_at DATETIME NOT NULL,
				ip_address TEXT,
				user_agent TEXT
			)
		`)
		return err
	}, func(tx *sql.Tx) error {
		_, err := tx.Exec("DROP TABLE IF EXISTS sessions")
		return err
	})

	// Migration 2: Create audit_log table
	mm.RegisterMigration(2, "create_audit_log_table", "Create audit log table for tracking admin actions", func(tx *sql.Tx) error {
		_, err := tx.Exec(`
			CREATE TABLE IF NOT EXISTS audit_log (
				id INTEGER PRIMARY KEY AUTOINCREMENT,
				timestamp DATETIME DEFAULT CURRENT_TIMESTAMP,
				user_id TEXT,
				username TEXT,
				action TEXT NOT NULL,
				resource TEXT,
				details TEXT,
				ip_address TEXT,
				user_agent TEXT
			)
		`)
		return err
	}, func(tx *sql.Tx) error {
		_, err := tx.Exec("DROP TABLE IF EXISTS audit_log")
		return err
	})

	// Migration 3: Create settings table for runtime config
	mm.RegisterMigration(3, "create_settings_table", "Create settings table for runtime configuration", func(tx *sql.Tx) error {
		_, err := tx.Exec(`
			CREATE TABLE IF NOT EXISTS settings (
				key TEXT PRIMARY KEY,
				value TEXT,
				type TEXT DEFAULT 'string',
				updated_at DATETIME DEFAULT CURRENT_TIMESTAMP,
				updated_by TEXT
			)
		`)
		return err
	}, func(tx *sql.Tx) error {
		_, err := tx.Exec("DROP TABLE IF EXISTS settings")
		return err
	})

	// Migration 4: Create scheduled_tasks table
	mm.RegisterMigration(4, "create_scheduled_tasks_table", "Create scheduled tasks tracking table", func(tx *sql.Tx) error {
		_, err := tx.Exec(`
			CREATE TABLE IF NOT EXISTS scheduled_tasks (
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
			)
		`)
		return err
	}, func(tx *sql.Tx) error {
		_, err := tx.Exec("DROP TABLE IF EXISTS scheduled_tasks")
		return err
	})

	// Migration 5: Create task_history table
	mm.RegisterMigration(5, "create_task_history_table", "Create task run history table", func(tx *sql.Tx) error {
		_, err := tx.Exec(`
			CREATE TABLE IF NOT EXISTS task_history (
				id INTEGER PRIMARY KEY AUTOINCREMENT,
				task_id TEXT NOT NULL,
				start_time DATETIME NOT NULL,
				end_time DATETIME,
				duration_ms INTEGER,
				result TEXT,
				error TEXT,
				FOREIGN KEY (task_id) REFERENCES scheduled_tasks(id)
			)
		`)
		return err
	}, func(tx *sql.Tx) error {
		_, err := tx.Exec("DROP TABLE IF EXISTS task_history")
		return err
	})

	// Migration 6: Create cluster_nodes table for cluster mode
	mm.RegisterMigration(6, "create_cluster_nodes_table", "Create cluster nodes table for distributed mode", func(tx *sql.Tx) error {
		_, err := tx.Exec(`
			CREATE TABLE IF NOT EXISTS cluster_nodes (
				id TEXT PRIMARY KEY,
				hostname TEXT NOT NULL,
				address TEXT NOT NULL,
				port INTEGER NOT NULL,
				is_primary INTEGER DEFAULT 0,
				last_heartbeat DATETIME,
				joined_at DATETIME DEFAULT CURRENT_TIMESTAMP,
				status TEXT DEFAULT 'active'
			)
		`)
		return err
	}, func(tx *sql.Tx) error {
		_, err := tx.Exec("DROP TABLE IF EXISTS cluster_nodes")
		return err
	})

	// Migration 7: Create distributed_locks table
	mm.RegisterMigration(7, "create_distributed_locks_table", "Create distributed locks table for cluster coordination", func(tx *sql.Tx) error {
		_, err := tx.Exec(`
			CREATE TABLE IF NOT EXISTS distributed_locks (
				name TEXT PRIMARY KEY,
				holder_id TEXT NOT NULL,
				acquired_at DATETIME DEFAULT CURRENT_TIMESTAMP,
				expires_at DATETIME NOT NULL,
				metadata TEXT
			)
		`)
		return err
	}, func(tx *sql.Tx) error {
		_, err := tx.Exec("DROP TABLE IF EXISTS distributed_locks")
		return err
	})

	// Migration 8: Create notifications table
	mm.RegisterMigration(8, "create_notifications_table", "Create notifications table", func(tx *sql.Tx) error {
		_, err := tx.Exec(`
			CREATE TABLE IF NOT EXISTS notifications (
				id INTEGER PRIMARY KEY AUTOINCREMENT,
				type TEXT NOT NULL,
				title TEXT NOT NULL,
				message TEXT,
				created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
				read_at DATETIME,
				dismissed_at DATETIME,
				priority TEXT DEFAULT 'normal',
				metadata TEXT
			)
		`)
		return err
	}, func(tx *sql.Tx) error {
		_, err := tx.Exec("DROP TABLE IF EXISTS notifications")
		return err
	})
}
