// SPDX-License-Identifier: MIT
// AI.md PART 10: Database
package database

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"sync"
	"time"

	// Database drivers - imported for side effects per AI.md PART 10.
	// Spec supports ONLY SQLite (local) and libsql/Turso (remote).

	// libsql / Turso (remote SQLite)
	_ "github.com/tursodatabase/libsql-client-go/libsql"

	// SQLite (pure Go)
	_ "modernc.org/sqlite"
)

// Driver represents a database driver type
type Driver string

const (
	DriverSQLite Driver = "sqlite"
	DriverLibSQL Driver = "libsql"
)

// DatabaseConfig holds database connection configuration per AI.md PART 10.
// Supported drivers: sqlite (aliases sqlite2/sqlite3/file) and libsql (alias turso).
type DatabaseConfig struct {
	Driver Driver `yaml:"driver"`
	// URL is the connection URL for libsql/Turso (remote-only)
	URL string `yaml:"url"`
	// Token is the auth token for libsql/Turso; appended as authToken if not in URL
	Token string `yaml:"token"`
	// SQLite-specific
	Path        string `yaml:"path"`
	JournalMode string `yaml:"journal_mode"`
	BusyTimeout int    `yaml:"busy_timeout"`
}

// normalizeDriver maps config driver aliases to canonical drivers per AI.md PART 3
func normalizeDriver(driver Driver) Driver {
	switch driver {
	case DriverSQLite, "sqlite2", "sqlite3", "file", "":
		return DriverSQLite
	case DriverLibSQL, "turso":
		return DriverLibSQL
	default:
		return driver
	}
}

// AppDatabase provides a unified interface for multiple database backends
type AppDatabase struct {
	db     *sql.DB
	driver Driver
	config DatabaseConfig
	mu     sync.RWMutex
	ctx    context.Context
	cancel context.CancelFunc
}

// NewAppDatabase creates a new database connection based on the driver
func NewAppDatabase(cfg DatabaseConfig) (*AppDatabase, error) {
	var db *sql.DB
	var err error

	driver := normalizeDriver(cfg.Driver)
	switch driver {
	case DriverSQLite:
		db, err = openSQLite(cfg)
	case DriverLibSQL:
		db, err = openLibSQL(cfg)
	default:
		return nil, fmt.Errorf("unsupported database driver: %s", cfg.Driver)
	}

	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}

	// Configure connection pool per AI.md PART 10
	db.SetMaxOpenConns(25)
	db.SetMaxIdleConns(5)
	db.SetConnMaxLifetime(5 * time.Minute)
	// Per PART 10 requirement
	db.SetConnMaxIdleTime(1 * time.Minute)

	ctx, cancel := context.WithCancel(context.Background())

	return &AppDatabase{
		db:     db,
		driver: driver,
		config: cfg,
		ctx:    ctx,
		cancel: cancel,
	}, nil
}

// openSQLite opens a SQLite database connection
func openSQLite(cfg DatabaseConfig) (*sql.DB, error) {
	dsn := cfg.Path
	if dsn == "" {
		dsn = "vidveil.db"
	}

	journalMode := cfg.JournalMode
	if journalMode == "" {
		journalMode = "WAL"
	}

	busyTimeout := cfg.BusyTimeout
	if busyTimeout == 0 {
		busyTimeout = 5000
	}

	dsn = fmt.Sprintf("%s?_journal_mode=%s&_busy_timeout=%d", dsn, journalMode, busyTimeout)
	return sql.Open("sqlite", dsn)
}

// openLibSQL opens a libsql/Turso remote database connection per AI.md PART 3.
// libsql is remote-only: a URL is required; the auth token is appended if not already present.
func openLibSQL(cfg DatabaseConfig) (*sql.DB, error) {
	url := cfg.URL
	if url == "" {
		return nil, fmt.Errorf("libsql driver requires a database URL (libsql is remote-only)")
	}

	if cfg.Token != "" && !strings.Contains(url, "authToken=") {
		if strings.Contains(url, "?") {
			url += "&authToken=" + cfg.Token
		} else {
			url += "?authToken=" + cfg.Token
		}
	}

	return sql.Open("libsql", url)
}

// DB returns the underlying *sql.DB connection
func (d *AppDatabase) DB() *sql.DB {
	d.mu.RLock()
	defer d.mu.RUnlock()
	return d.db
}

// Driver returns the database driver type
func (d *AppDatabase) Driver() Driver {
	return d.driver
}

// Exec executes a query without returning rows
// Per AI.md PART 10: All queries MUST have timeouts (10s for writes)
func (d *AppDatabase) Exec(query string, args ...interface{}) (sql.Result, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	return d.db.ExecContext(ctx, query, args...)
}

// ExecContext executes a query without returning rows with context
func (d *AppDatabase) ExecContext(ctx context.Context, query string, args ...interface{}) (sql.Result, error) {
	return d.db.ExecContext(ctx, query, args...)
}

// Query executes a query that returns rows
// Per AI.md PART 10: All queries MUST have timeouts (5s for reads)
func (d *AppDatabase) Query(query string, args ...interface{}) (*sql.Rows, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	return d.db.QueryContext(ctx, query, args...)
}

// QueryContext executes a query that returns rows with context
func (d *AppDatabase) QueryContext(ctx context.Context, query string, args ...interface{}) (*sql.Rows, error) {
	return d.db.QueryContext(ctx, query, args...)
}

// QueryRow executes a query that returns at most one row
// Per AI.md PART 10: All queries MUST have timeouts (5s for reads)
func (d *AppDatabase) QueryRow(query string, args ...interface{}) *sql.Row {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	return d.db.QueryRowContext(ctx, query, args...)
}

// QueryRowContext executes a query that returns at most one row with context
func (d *AppDatabase) QueryRowContext(ctx context.Context, query string, args ...interface{}) *sql.Row {
	return d.db.QueryRowContext(ctx, query, args...)
}

// Begin starts a new transaction
// Per AI.md PART 10: Transactions have 30s timeout
func (d *AppDatabase) Begin() (*sql.Tx, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	return d.db.BeginTx(ctx, nil)
}

// BeginTx starts a new transaction with context and options
func (d *AppDatabase) BeginTx(ctx context.Context, opts *sql.TxOptions) (*sql.Tx, error) {
	return d.db.BeginTx(ctx, opts)
}

// Ping verifies the database connection
func (d *AppDatabase) Ping() error {
	return d.db.Ping()
}

// PingContext verifies the database connection with context
func (d *AppDatabase) PingContext(ctx context.Context) error {
	return d.db.PingContext(ctx)
}

// Close closes the database connection
func (d *AppDatabase) Close() error {
	d.cancel()
	return d.db.Close()
}

// Stats returns database connection statistics
func (d *AppDatabase) Stats() sql.DBStats {
	return d.db.Stats()
}

// TranslateQuery translates a query for the specific database driver.
// SQLite and libsql share the same dialect; queries pass through unchanged.
func (d *AppDatabase) TranslateQuery(query string) string {
	return query
}

// TableExists checks if a table exists in the database.
// SQLite and libsql share the sqlite_master catalog.
func (d *AppDatabase) TableExists(tableName string) (bool, error) {
	query := "SELECT COUNT(*) FROM sqlite_master WHERE type='table' AND name=?"

	var exists int
	err := d.QueryRow(query, tableName).Scan(&exists)
	return exists > 0, err
}

// Version returns the database server version
func (d *AppDatabase) Version() (string, error) {
	var version string
	err := d.QueryRow("SELECT sqlite_version()").Scan(&version)
	return version, err
}

// Query timeout constants per AI.md PART 10
const (
	// TimeoutSimpleSelect for simple SELECT queries
	TimeoutSimpleSelect = 5 * time.Second
	// TimeoutComplexSelect for complex SELECT with JOINs
	TimeoutComplexSelect = 15 * time.Second
	// TimeoutWrite for INSERT/UPDATE/DELETE
	TimeoutWrite = 10 * time.Second
	// TimeoutBulk for bulk operations
	TimeoutBulk = 60 * time.Second
	// TimeoutMigration for schema changes
	TimeoutMigration = 5 * time.Minute
	// TimeoutReport for aggregation reports
	TimeoutReport = 2 * time.Minute
	// TimeoutTransaction is default transaction timeout
	TimeoutTransaction = 30 * time.Second
)

// WithTimeout creates a context with the specified timeout
// Per AI.md PART 10: All database queries MUST have timeouts
func WithTimeout(ctx context.Context, timeout time.Duration) (context.Context, context.CancelFunc) {
	return context.WithTimeout(ctx, timeout)
}

// WithTransaction executes a function within a transaction
// Per AI.md PART 10 transaction patterns
func (d *AppDatabase) WithTransaction(ctx context.Context, fn func(*sql.Tx) error) error {
	// Transaction timeout per PART 10
	ctx, cancel := context.WithTimeout(ctx, TimeoutTransaction)
	defer cancel()

	tx, err := d.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}

	if err := fn(tx); err != nil {
		tx.Rollback()
		return err
	}

	return tx.Commit()
}

// HandleQueryError translates database errors to appropriate error codes
// Per AI.md PART 10
func HandleQueryError(err error) error {
	if err == nil {
		return nil
	}
	switch {
	case err == context.DeadlineExceeded:
		return fmt.Errorf("TIMEOUT: query timed out")
	case err == sql.ErrNoRows:
		return fmt.Errorf("NOT_FOUND: resource not found")
	case err == context.Canceled:
		return fmt.Errorf("CANCELED: request was canceled")
	default:
		return fmt.Errorf("SERVER_ERROR: database error: %w", err)
	}
}
