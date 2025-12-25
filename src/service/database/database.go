// SPDX-License-Identifier: MIT
// TEMPLATE.md PART 23: Database Abstraction Layer
package database

import (
	"context"
	"database/sql"
	"fmt"
	"sync"
	"time"

	// Database drivers - imported for side effects
	_ "github.com/go-sql-driver/mysql"
	_ "github.com/jackc/pgx/v5/stdlib"
)

// Driver represents a database driver type
type Driver string

const (
	DriverSQLite   Driver = "sqlite"
	DriverPostgres Driver = "postgres"
	DriverMySQL    Driver = "mysql"
)

// Config holds database connection configuration
type Config struct {
	Driver   Driver `yaml:"driver"`
	Host     string `yaml:"host"`
	Port     int    `yaml:"port"`
	Name     string `yaml:"name"`
	User     string `yaml:"user"`
	Password string `yaml:"password"`
	SSLMode  string `yaml:"ssl_mode"`
	// SQLite-specific
	Path        string `yaml:"path"`
	JournalMode string `yaml:"journal_mode"`
	BusyTimeout int    `yaml:"busy_timeout"`
}

// Database provides a unified interface for multiple database backends
type Database struct {
	db       *sql.DB
	driver   Driver
	config   Config
	mu       sync.RWMutex
	ctx      context.Context
	cancel   context.CancelFunc
	isLeader bool
}

// NewDatabase creates a new database connection based on the driver
func NewDatabase(cfg Config) (*Database, error) {
	var db *sql.DB
	var err error

	switch cfg.Driver {
	case DriverSQLite, "sqlite3", "":
		db, err = openSQLite(cfg)
	case DriverPostgres, "postgresql":
		db, err = openPostgres(cfg)
	case DriverMySQL, "mariadb":
		db, err = openMySQL(cfg)
	default:
		return nil, fmt.Errorf("unsupported database driver: %s", cfg.Driver)
	}

	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}

	// Configure connection pool
	db.SetMaxOpenConns(25)
	db.SetMaxIdleConns(5)
	db.SetConnMaxLifetime(5 * time.Minute)

	ctx, cancel := context.WithCancel(context.Background())

	return &Database{
		db:     db,
		driver: cfg.Driver,
		config: cfg,
		ctx:    ctx,
		cancel: cancel,
	}, nil
}

// openSQLite opens a SQLite database connection
func openSQLite(cfg Config) (*sql.DB, error) {
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

// openPostgres opens a PostgreSQL database connection
func openPostgres(cfg Config) (*sql.DB, error) {
	sslMode := cfg.SSLMode
	if sslMode == "" {
		sslMode = "disable"
	}

	port := cfg.Port
	if port == 0 {
		port = 5432
	}

	host := cfg.Host
	if host == "" {
		host = "localhost"
	}

	// DSN format for PostgreSQL: host=%s port=%d user=%s password=%s dbname=%s sslmode=%s
	dsn := fmt.Sprintf("host=%s port=%d user=%s password=%s dbname=%s sslmode=%s",
		host, port, cfg.User, cfg.Password, cfg.Name, sslMode)

	return sql.Open("pgx", dsn)
}

// openMySQL opens a MySQL/MariaDB database connection
func openMySQL(cfg Config) (*sql.DB, error) {
	port := cfg.Port
	if port == 0 {
		port = 3306
	}

	host := cfg.Host
	if host == "" {
		host = "localhost"
	}

	// DSN format for MySQL: user:password@tcp(host:port)/dbname?parseTime=true
	dsn := fmt.Sprintf("%s:%s@tcp(%s:%d)/%s?parseTime=true",
		cfg.User, cfg.Password, host, port, cfg.Name)

	return sql.Open("mysql", dsn)
}

// DB returns the underlying *sql.DB connection
func (d *Database) DB() *sql.DB {
	d.mu.RLock()
	defer d.mu.RUnlock()
	return d.db
}

// Driver returns the database driver type
func (d *Database) Driver() Driver {
	return d.driver
}

// Exec executes a query without returning rows
func (d *Database) Exec(query string, args ...interface{}) (sql.Result, error) {
	return d.db.Exec(query, args...)
}

// ExecContext executes a query without returning rows with context
func (d *Database) ExecContext(ctx context.Context, query string, args ...interface{}) (sql.Result, error) {
	return d.db.ExecContext(ctx, query, args...)
}

// Query executes a query that returns rows
func (d *Database) Query(query string, args ...interface{}) (*sql.Rows, error) {
	return d.db.Query(query, args...)
}

// QueryContext executes a query that returns rows with context
func (d *Database) QueryContext(ctx context.Context, query string, args ...interface{}) (*sql.Rows, error) {
	return d.db.QueryContext(ctx, query, args...)
}

// QueryRow executes a query that returns at most one row
func (d *Database) QueryRow(query string, args ...interface{}) *sql.Row {
	return d.db.QueryRow(query, args...)
}

// QueryRowContext executes a query that returns at most one row with context
func (d *Database) QueryRowContext(ctx context.Context, query string, args ...interface{}) *sql.Row {
	return d.db.QueryRowContext(ctx, query, args...)
}

// Begin starts a new transaction
func (d *Database) Begin() (*sql.Tx, error) {
	return d.db.Begin()
}

// BeginTx starts a new transaction with context and options
func (d *Database) BeginTx(ctx context.Context, opts *sql.TxOptions) (*sql.Tx, error) {
	return d.db.BeginTx(ctx, opts)
}

// Ping verifies the database connection
func (d *Database) Ping() error {
	return d.db.Ping()
}

// PingContext verifies the database connection with context
func (d *Database) PingContext(ctx context.Context) error {
	return d.db.PingContext(ctx)
}

// Close closes the database connection
func (d *Database) Close() error {
	d.cancel()
	return d.db.Close()
}

// Stats returns database connection statistics
func (d *Database) Stats() sql.DBStats {
	return d.db.Stats()
}

// SetLeader sets whether this node is the cluster leader
func (d *Database) SetLeader(isLeader bool) {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.isLeader = isLeader
}

// IsLeader returns whether this node is the cluster leader
func (d *Database) IsLeader() bool {
	d.mu.RLock()
	defer d.mu.RUnlock()
	return d.isLeader
}

// TranslateQuery translates a query for the specific database driver
// This handles differences in SQL syntax between databases
func (d *Database) TranslateQuery(query string) string {
	switch d.driver {
	case DriverPostgres:
		// PostgreSQL uses $1, $2, etc. for placeholders
		// Convert ? to $1, $2, etc. if needed
		return query
	case DriverMySQL:
		// MySQL uses ? for placeholders (same as SQLite)
		return query
	default:
		// SQLite uses ? for placeholders
		return query
	}
}

// TableExists checks if a table exists in the database
func (d *Database) TableExists(tableName string) (bool, error) {
	var query string
	switch d.driver {
	case DriverPostgres:
		query = "SELECT EXISTS (SELECT FROM information_schema.tables WHERE table_name = $1)"
	case DriverMySQL:
		query = "SELECT COUNT(*) FROM information_schema.tables WHERE table_name = ?"
	default:
		query = "SELECT COUNT(*) FROM sqlite_master WHERE type='table' AND name=?"
	}

	var exists int
	err := d.db.QueryRow(query, tableName).Scan(&exists)
	return exists > 0, err
}

// Version returns the database server version
func (d *Database) Version() (string, error) {
	var query string
	switch d.driver {
	case DriverPostgres:
		query = "SELECT version()"
	case DriverMySQL:
		query = "SELECT VERSION()"
	default:
		query = "SELECT sqlite_version()"
	}

	var version string
	err := d.db.QueryRow(query).Scan(&version)
	return version, err
}
