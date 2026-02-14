// SPDX-License-Identifier: MIT
// AI.md PART 10: Database & Cluster
package database

import (
	"context"
	"database/sql"
	"fmt"
	"sync"
	"time"

	// Database drivers - imported for side effects per AI.md PART 10
	_ "github.com/go-sql-driver/mysql"    // MySQL/MariaDB
	_ "github.com/jackc/pgx/v5/stdlib"    // PostgreSQL
	_ "github.com/microsoft/go-mssqldb"   // Microsoft SQL Server
	_ "modernc.org/sqlite"                // SQLite (pure Go)
)

// Driver represents a database driver type
type Driver string

const (
	DriverSQLite   Driver = "sqlite"
	DriverPostgres Driver = "postgres"
	DriverMySQL    Driver = "mysql"
	DriverMSSQL    Driver = "mssql"
)

// DatabaseConfig holds database connection configuration
type DatabaseConfig struct {
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

// AppDatabase provides a unified interface for multiple database backends
type AppDatabase struct {
	db       *sql.DB
	driver   Driver
	config   DatabaseConfig
	mu       sync.RWMutex
	ctx      context.Context
	cancel   context.CancelFunc
	isLeader bool
}

// NewAppDatabase creates a new database connection based on the driver
func NewAppDatabase(cfg DatabaseConfig) (*AppDatabase, error) {
	var db *sql.DB
	var err error

	switch cfg.Driver {
	case DriverSQLite, "sqlite3", "":
		db, err = openSQLite(cfg)
	case DriverPostgres, "postgresql":
		db, err = openPostgres(cfg)
	case DriverMySQL, "mariadb":
		db, err = openMySQL(cfg)
	case DriverMSSQL, "sqlserver":
		db, err = openMSSQL(cfg)
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
		driver: cfg.Driver,
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

// openPostgres opens a PostgreSQL database connection
func openPostgres(cfg DatabaseConfig) (*sql.DB, error) {
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
func openMySQL(cfg DatabaseConfig) (*sql.DB, error) {
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

// openMSSQL opens a Microsoft SQL Server database connection
func openMSSQL(cfg DatabaseConfig) (*sql.DB, error) {
	port := cfg.Port
	if port == 0 {
		port = 1433
	}

	host := cfg.Host
	if host == "" {
		host = "localhost"
	}

	// DSN format for MSSQL: sqlserver://user:password@host:port?database=dbname
	dsn := fmt.Sprintf("sqlserver://%s:%s@%s:%d?database=%s",
		cfg.User, cfg.Password, host, port, cfg.Name)

	return sql.Open("sqlserver", dsn)
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

// SetLeader sets whether this node is the cluster leader
func (d *AppDatabase) SetLeader(isLeader bool) {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.isLeader = isLeader
}

// IsLeader returns whether this node is the cluster leader
func (d *AppDatabase) IsLeader() bool {
	d.mu.RLock()
	defer d.mu.RUnlock()
	return d.isLeader
}

// TranslateQuery translates a query for the specific database driver
// This handles differences in SQL syntax between databases
func (d *AppDatabase) TranslateQuery(query string) string {
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
func (d *AppDatabase) TableExists(tableName string) (bool, error) {
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
	err := d.QueryRow(query, tableName).Scan(&exists)
	return exists > 0, err
}

// Version returns the database server version
func (d *AppDatabase) Version() (string, error) {
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
	err := d.QueryRow(query).Scan(&version)
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
