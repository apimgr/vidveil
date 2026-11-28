// SPDX-License-Identifier: MIT
// AI.md PART 10: Database
package database

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"math/rand"
	"regexp"
	"strings"
	"time"

	// Database drivers - imported for side effects per AI.md PART 10.
	// Spec supports ONLY SQLite (local) and libsql/Turso (remote).

	// libsql / Turso (remote SQLite)
	_ "github.com/tursodatabase/libsql-client-go/libsql"

	// SQLite (pure Go)
	_ "modernc.org/sqlite"

	"github.com/apimgr/vidveil/src/common/dbstore"
	"github.com/apimgr/vidveil/src/server/service/metric"
)

// Driver represents a database driver type
type Driver string

const (
	DriverSQLite Driver = "sqlite"
	DriverLibSQL Driver = "libsql"
)

// ErrRestoreSeedInProgress is returned when a remote restore pauses writes.
var ErrRestoreSeedInProgress = errors.New("remote database restore seed is in progress")

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
	// Pool holds connection pool settings per AI.md PART 10 (libsql/remote;
	// SQLite uses a single writer). Zero values fall back to defaults.
	Pool PoolConfig `yaml:"pool"`
}

// PoolConfig holds connection pool settings per AI.md PART 10 Pool Configuration.
// MaxLifetime/MaxIdleTime accept Go duration strings (e.g. "5m", "1m").
type PoolConfig struct {
	MaxOpen     int    `yaml:"max_open"`
	MaxIdle     int    `yaml:"max_idle"`
	MaxLifetime string `yaml:"max_lifetime"`
	MaxIdleTime string `yaml:"max_idle_time"`
}

// queryTableRe extracts the target table from a SQL statement's FROM/INTO/
// UPDATE/TABLE clause for AI.md PART 20 db metric labels. Best-effort only —
// bounded cardinality (finite table names), never used for query execution.
var queryTableRe = regexp.MustCompile(`(?i)\b(?:from|into|update|table)\s+["'` + "`" + `]?([a-zA-Z_][a-zA-Z0-9_]*)`)

// queryLabels derives the low-cardinality "operation"/"table" labels used by
// AI.md PART 20 database metrics from a raw SQL query string.
func queryLabels(query string) (operation, table string) {
	trimmed := strings.TrimSpace(query)
	fields := strings.Fields(trimmed)
	operation = "OTHER"
	if len(fields) > 0 {
		operation = strings.ToLower(fields[0])
		if operation == "with" {
			for _, field := range fields[1:] {
				candidate := strings.ToLower(strings.Trim(field, "(,;"))
				if candidate == "select" || candidate == "insert" || candidate == "update" || candidate == "delete" {
					operation = candidate
					break
				}
			}
		}
	}
	table = "unknown"
	if m := queryTableRe.FindStringSubmatch(trimmed); len(m) == 2 {
		table = strings.ToLower(m[1])
	}
	return operation, table
}

// instrumentQuery records AI.md PART 20 database metrics (queries total,
// duration, errors) around a single query execution.
func instrumentQuery(query string, fn func() error) error {
	operation, table := queryLabels(query)
	start := time.Now()
	err := fn()
	metric.DBQueriesTotal.WithLabelValues(operation, table).Inc()
	metric.DBQueryDuration.WithLabelValues(operation, table).Observe(time.Since(start).Seconds())
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		metric.DBErrorsTotal.WithLabelValues(operation, classifyDBError(err)).Inc()
	}
	return err
}

type MetricsRow struct {
	row       *sql.Row
	operation string
	table     string
	start     time.Time
}

type readPool struct{ db *sql.DB }

func (r readPool) QueryContext(ctx context.Context, query string, args ...any) (*sql.Rows, error) {
	var rows *sql.Rows
	err := instrumentQuery(query, func() error {
		var queryErr error
		rows, queryErr = r.db.QueryContext(ctx, query, args...)
		return queryErr
	})
	return rows, err
}

func (r readPool) QueryRowContext(ctx context.Context, query string, args ...any) dbstore.RowScanner {
	operation, table := queryLabels(query)
	return &MetricsRow{row: r.db.QueryRowContext(ctx, query, args...), operation: operation, table: table, start: time.Now()}
}

func (r readPool) PingContext(ctx context.Context) error { return r.db.PingContext(ctx) }

func (r *MetricsRow) Scan(dest ...any) error {
	err := r.row.Scan(dest...)
	operation, table := r.operation, r.table
	metric.DBQueriesTotal.WithLabelValues(operation, table).Inc()
	metric.DBQueryDuration.WithLabelValues(operation, table).Observe(time.Since(r.start).Seconds())
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		metric.DBErrorsTotal.WithLabelValues(operation, classifyDBError(err)).Inc()
	}
	return err
}

// classifyDBError maps a database error to a low-cardinality error_type label.
func classifyDBError(err error) string {
	message := strings.ToLower(err.Error())
	switch {
	case errors.Is(err, context.DeadlineExceeded):
		return "timeout"
	case strings.Contains(message, "connection") || strings.Contains(message, "closed database"):
		return "connection"
	case strings.Contains(message, "unique constraint") || strings.Contains(message, "duplicate key") || strings.Contains(message, "already exists"):
		return "duplicate"
	case strings.Contains(message, "constraint"):
		return "constraint"
	default:
		return "other"
	}
}

// maxOpen returns the configured max open connections or the default (25).
func (p PoolConfig) maxOpen() int {
	if p.MaxOpen > 0 {
		return p.MaxOpen
	}
	return 25
}

// maxIdle returns the configured max idle connections or the default (5).
func (p PoolConfig) maxIdle() int {
	if p.MaxIdle > 0 {
		return p.MaxIdle
	}
	return 5
}

// maxLifetime returns the configured connection max lifetime or the default (5m).
func (p PoolConfig) maxLifetime() time.Duration {
	if d, err := time.ParseDuration(p.MaxLifetime); err == nil && d > 0 {
		return d
	}
	return 5 * time.Minute
}

// maxIdleTime returns the configured connection max idle time or the default (1m).
func (p PoolConfig) maxIdleTime() time.Duration {
	if d, err := time.ParseDuration(p.MaxIdleTime); err == nil && d > 0 {
		return d
	}
	return 1 * time.Minute
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
	readDB *sql.DB
	driver Driver
	config DatabaseConfig
	ctx    context.Context
	cancel context.CancelFunc
}

// NewAppDatabase creates a new database connection based on the driver
func NewAppDatabase(cfg DatabaseConfig) (*AppDatabase, error) {
	var db, readDB *sql.DB
	var err error

	driver := normalizeDriver(cfg.Driver)
	switch driver {
	case DriverSQLite:
		db, err = openSQLite(cfg)
		if err == nil {
			if cfg.Path == ":memory:" {
				readDB = db
			} else {
				readDB, err = openSQLiteRead(cfg)
			}
		}
	case DriverLibSQL:
		db, err = openLibSQL(cfg)
		if err == nil {
			readDB, err = openLibSQL(cfg)
		}
	default:
		return nil, fmt.Errorf("unsupported database driver: %s", cfg.Driver)
	}

	if err != nil {
		if db != nil {
			_ = db.Close()
		}
		if readDB != nil && readDB != db {
			_ = readDB.Close()
		}
		return nil, fmt.Errorf("failed to open database: %w", err)
	}

	// Configure connection pool per AI.md PART 10 (configurable via
	// server.database.pool; zero values fall back to defaults).
	if driver == DriverSQLite {
		db.SetMaxOpenConns(1)
		db.SetMaxIdleConns(1)
		db.SetConnMaxLifetime(0)
	} else {
		db.SetMaxOpenConns(cfg.Pool.maxOpen())
		db.SetMaxIdleConns(cfg.Pool.maxIdle())
		db.SetConnMaxLifetime(cfg.Pool.maxLifetime())
		db.SetConnMaxIdleTime(cfg.Pool.maxIdleTime())
	}
	if readDB != db && driver == DriverSQLite {
		readDB.SetMaxOpenConns(cfg.Pool.maxOpen())
		readDB.SetMaxIdleConns(cfg.Pool.maxIdle())
		readDB.SetConnMaxLifetime(cfg.Pool.maxLifetime())
		readDB.SetConnMaxIdleTime(cfg.Pool.maxIdleTime())
	} else if readDB != db {
		readDB.SetMaxOpenConns(cfg.Pool.maxOpen())
		readDB.SetMaxIdleConns(cfg.Pool.maxIdle())
		readDB.SetConnMaxLifetime(cfg.Pool.maxLifetime())
		readDB.SetConnMaxIdleTime(cfg.Pool.maxIdleTime())
	}

	// Verify the connection per AI.md PART 10 (NewDB pattern: PingContext
	// with a bounded timeout before the pool is handed to callers).
	pingCtx, pingCancel := context.WithTimeout(context.Background(), TimeoutSimpleSelect)
	defer pingCancel()
	if err := db.PingContext(pingCtx); err != nil {
		_ = db.Close()
		if readDB != db {
			_ = readDB.Close()
		}
		return nil, fmt.Errorf("failed to verify database connection: %w", err)
	}
	if readDB != db {
		if err := readDB.PingContext(pingCtx); err != nil {
			_ = db.Close()
			_ = readDB.Close()
			return nil, fmt.Errorf("failed to verify read database connection: %w", err)
		}
	}

	ctx, cancel := context.WithCancel(context.Background())

	appDB := &AppDatabase{
		db:     db,
		readDB: readDB,
		driver: driver,
		config: cfg,
		ctx:    ctx,
		cancel: cancel,
	}

	// Poll pool stats into AI.md PART 20 db_connections_open/in_use gauges
	// until Close() cancels ctx.
	go appDB.pollConnectionMetrics()

	return appDB, nil
}

// pollConnectionMetrics periodically publishes sql.DBStats to the AI.md
// PART 20 db_connections_open/db_connections_in_use gauges until Close().
func (d *AppDatabase) pollConnectionMetrics() {
	ticker := time.NewTicker(15 * time.Second)
	defer ticker.Stop()
	for {
		stats := d.db.Stats()
		if d.readDB != d.db {
			readStats := d.readDB.Stats()
			stats.OpenConnections += readStats.OpenConnections
			stats.InUse += readStats.InUse
		}
		metric.DBConnectionsOpen.Set(float64(stats.OpenConnections))
		metric.DBConnectionsInUse.Set(float64(stats.InUse))
		select {
		case <-d.ctx.Done():
			return
		case <-ticker.C:
		}
	}
}

// openSQLite opens a SQLite database connection
func sqliteDSN(cfg DatabaseConfig, readOnly bool) string {
	dsn := cfg.Path
	if dsn == "" {
		dsn = "vidveil.db"
	}

	journalMode := cfg.JournalMode
	if journalMode == "" {
		journalMode = "WAL"
	}

	if dsn == ":memory:" {
		return dsn
	}
	if strings.HasPrefix(dsn, "file:") {
		dsn += "&"
	} else {
		dsn = "file:" + dsn + "?"
	}
	dsn += "_pragma=busy_timeout(10000)&_pragma=journal_mode(" + strings.ToLower(journalMode) + ")&_pragma=synchronous(NORMAL)&_pragma=foreign_keys(1)"
	if readOnly {
		dsn += "&_pragma=query_only(1)"
	} else {
		dsn += "&_txlock=immediate"
	}
	return dsn
}

func openSQLite(cfg DatabaseConfig) (*sql.DB, error) {
	db, err := sql.Open("sqlite", sqliteDSN(cfg, false))
	if err != nil {
		return nil, err
	}
	db.SetMaxOpenConns(1)
	db.SetMaxIdleConns(1)
	db.SetConnMaxLifetime(0)
	var mode string
	if err := db.QueryRow(`PRAGMA journal_mode`).Scan(&mode); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("read SQLite journal mode for %q: %w", cfg.Path, err)
	}
	if !strings.EqualFold(mode, "wal") && (cfg.Path != ":memory:" || !strings.EqualFold(mode, "memory")) {
		_ = db.Close()
		return nil, fmt.Errorf("SQLite database %q requires WAL mode; got %q", cfg.Path, mode)
	}
	return db, nil
}

func openSQLiteRead(cfg DatabaseConfig) (*sql.DB, error) {
	return sql.Open("sqlite", sqliteDSN(cfg, true))
}

// OpenReadOnlySQLite opens a SQLite artifact for verification without a writer pool.
func OpenReadOnlySQLite(path string) (*AppDatabase, error) {
	dsn := path
	if strings.HasPrefix(dsn, "file:") {
		dsn += "&"
	} else {
		dsn = "file:" + dsn + "?"
	}
	dsn += "_pragma=busy_timeout(10000)&_pragma=query_only(1)&mode=ro"
	readDB, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, err
	}
	readDB.SetMaxOpenConns(1)
	readDB.SetMaxIdleConns(1)
	ctx, cancel := context.WithTimeout(context.Background(), TimeoutSimpleSelect)
	defer cancel()
	if err := readDB.PingContext(ctx); err != nil {
		_ = readDB.Close()
		return nil, err
	}
	return &AppDatabase{readDB: readDB, driver: DriverSQLite}, nil
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

// Driver returns the database driver type
func (d *AppDatabase) Driver() Driver {
	return d.driver
}

// Exec executes a query without returning rows
// Per AI.md PART 10: All queries MUST have timeouts (10s for writes)
func (d *AppDatabase) Exec(query string, args ...interface{}) (sql.Result, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	var result sql.Result
	err := instrumentQuery(query, func() error {
		return d.Write(ctx, func(tx *sql.Tx) error {
			var execErr error
			result, execErr = tx.ExecContext(ctx, query, args...)
			return execErr
		})
	})
	return result, err
}

// ExecContext executes a query without returning rows with context
func (d *AppDatabase) ExecContext(ctx context.Context, query string, args ...interface{}) (sql.Result, error) {
	ctx, cancel := withDefaultTimeout(ctx, TimeoutWrite)
	defer cancel()
	var result sql.Result
	err := instrumentQuery(query, func() error {
		return d.Write(ctx, func(tx *sql.Tx) error {
			var execErr error
			result, execErr = tx.ExecContext(ctx, query, args...)
			return execErr
		})
	})
	return result, err
}

// Query executes a query that returns rows
// Per AI.md PART 10: All queries MUST have timeouts (5s for reads).
// The returned *sql.Rows is consumed by the caller after this function
// returns, so cancel is fired by the timeout timer rather than deferred
// (a deferred cancel would cancel the context before the caller iterates).
func (d *AppDatabase) Query(query string, args ...interface{}) (*sql.Rows, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	var rows *sql.Rows
	err := instrumentQuery(query, func() error {
		var queryErr error
		rows, queryErr = d.readDB.QueryContext(ctx, query, args...)
		return queryErr
	})
	if err != nil {
		cancel()
		return nil, err
	}
	time.AfterFunc(5*time.Second, cancel)
	return rows, nil
}

// QueryContext executes a query that returns rows with context
func (d *AppDatabase) QueryContext(ctx context.Context, query string, args ...interface{}) (*sql.Rows, error) {
	ctx, cancel := withDefaultTimeout(ctx, TimeoutSimpleSelect)
	var rows *sql.Rows
	err := instrumentQuery(query, func() error {
		var queryErr error
		rows, queryErr = d.readDB.QueryContext(ctx, query, args...)
		return queryErr
	})
	if err != nil {
		cancel()
	} else {
		time.AfterFunc(TimeoutSimpleSelect, cancel)
	}
	return rows, err
}

// QueryRow executes a query that returns at most one row with a five-second timeout.
func (d *AppDatabase) QueryRow(query string, args ...interface{}) *MetricsRow {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	time.AfterFunc(5*time.Second, cancel)
	return d.QueryRowContext(ctx, query, args...)
}

// QueryRowContext defers query metrics and errors until database/sql reports them from Scan.
func (d *AppDatabase) QueryRowContext(ctx context.Context, query string, args ...interface{}) *MetricsRow {
	operation, table := queryLabels(query)
	queryCtx, cancel := withDefaultTimeout(ctx, TimeoutSimpleSelect)
	time.AfterFunc(TimeoutSimpleSelect, cancel)
	return &MetricsRow{row: d.readDB.QueryRowContext(queryCtx, query, args...), operation: operation, table: table, start: time.Now()}
}

// Ping verifies the database connection
func (d *AppDatabase) Ping() error {
	return d.PingContext(context.Background())
}

// PingContext verifies the database connection with context
func (d *AppDatabase) PingContext(ctx context.Context) error {
	if d == nil {
		return errors.New("database handle is unavailable")
	}
	if d.db != nil {
		if err := d.db.PingContext(ctx); err != nil {
			return err
		}
	}
	if d.readDB != nil && d.readDB != d.db {
		return d.readDB.PingContext(ctx)
	}
	if d.db == nil {
		return errors.New("database handle is unavailable")
	}
	return nil
}

// Close closes the database connection
func (d *AppDatabase) Close() error {
	if d.cancel != nil {
		d.cancel()
	}
	var err error
	if d.db != nil {
		err = d.db.Close()
	}
	if d.readDB != nil && d.readDB != d.db {
		if readErr := d.readDB.Close(); err == nil {
			err = readErr
		}
	}
	return err
}

func withDefaultTimeout(ctx context.Context, timeout time.Duration) (context.Context, context.CancelFunc) {
	if ctx == nil {
		ctx = context.Background()
	}
	if deadline, ok := ctx.Deadline(); ok && time.Until(deadline) <= timeout {
		return context.WithCancel(ctx)
	}
	return context.WithTimeout(ctx, timeout)
}

// Stats returns database connection statistics
func (d *AppDatabase) Stats() sql.DBStats {
	stats := d.db.Stats()
	if d.readDB != d.db {
		readStats := d.readDB.Stats()
		stats.OpenConnections += readStats.OpenConnections
		stats.InUse += readStats.InUse
		stats.Idle += readStats.Idle
		stats.WaitCount += readStats.WaitCount
		stats.WaitDuration += readStats.WaitDuration
		stats.MaxIdleClosed += readStats.MaxIdleClosed
		stats.MaxIdleTimeClosed += readStats.MaxIdleTimeClosed
		stats.MaxLifetimeClosed += readStats.MaxLifetimeClosed
	}
	return stats
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
	row := d.QueryRow(query, tableName)
	err := row.Scan(&exists)
	return exists > 0, err
}

// Version returns the database server version
func (d *AppDatabase) Version() (string, error) {
	var version string
	row := d.QueryRow("SELECT sqlite_version()")
	err := row.Scan(&version)
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
	return d.Write(ctx, fn)
}

// Read runs read-only database work with the standard simple-query deadline.
func (d *AppDatabase) Read(ctx context.Context, fn func(context.Context, dbstore.Reader) error) error {
	if d == nil || d.readDB == nil {
		return errors.New("database read handle is unavailable")
	}
	readCtx, cancel := withDefaultTimeout(ctx, TimeoutSimpleSelect)
	defer cancel()
	if err := readCtx.Err(); err != nil {
		return err
	}
	return fn(readCtx, readPool{db: d.readDB})
}

// Write runs one bounded transaction on the writer handle and retries only
// transient SQLite lock errors while the context deadline remains active.
func (d *AppDatabase) Write(ctx context.Context, fn func(*sql.Tx) error) error {
	return d.write(ctx, fn, false)
}

// WriteRestoreSeedBatch renews the remote restore lease and writes one bounded
// archive batch while ordinary remote writes are paused.
func (d *AppDatabase) WriteRestoreSeedBatch(ctx context.Context, fn func(*sql.Tx) error) error {
	return d.write(ctx, fn, true)
}

func (d *AppDatabase) write(ctx context.Context, fn func(*sql.Tx) error, restoreSeedBatch bool) error {
	if d == nil || d.db == nil {
		return errors.New("database write handle is unavailable")
	}
	writeCtx, cancel := withDefaultTimeout(ctx, TimeoutWrite)
	defer cancel()
	var lastErr error
	for attempt := range 5 {
		attemptStart := time.Now()
		tx, err := d.db.BeginTx(writeCtx, nil)
		if d.driver == DriverSQLite {
			metric.DBWriteWaitSeconds.Observe(time.Since(attemptStart).Seconds())
		}
		if err == nil {
			if d.driver == DriverLibSQL {
				if restoreSeedBatch {
					var result sql.Result
					result, err = tx.ExecContext(writeCtx, `UPDATE restore_seed_locks SET lease_until = datetime('now', '+30 minutes') WHERE lock_id = 'remote_seed' AND lease_until > CURRENT_TIMESTAMP`)
					if err == nil {
						var affected int64
						affected, err = result.RowsAffected()
						if err == nil && affected != 1 {
							err = ErrRestoreSeedInProgress
						}
					}
				} else {
					err = checkRemoteRestoreSeedLock(tx)
				}
			}
			if err == nil {
				err = fn(tx)
			}
			if err != nil {
				_ = tx.Rollback()
			} else {
				err = tx.Commit()
			}
		}
		if err == nil {
			return nil
		}
		if !isSerializationError(err) || attempt == 4 {
			return err
		}
		if d.driver == DriverSQLite && attempt < 4 {
			metric.DBBusyTotal.Inc()
		}
		lastErr = err
		base := 25 * time.Millisecond * time.Duration(1<<attempt)
		jitter := time.Duration(rand.Intn(26)) * time.Millisecond
		select {
		case <-writeCtx.Done():
			return errors.Join(lastErr, writeCtx.Err())
		case <-time.After(base + jitter):
		}
	}
	return lastErr
}

func checkRemoteRestoreSeedLock(tx *sql.Tx) error {
	var lockTableExists int
	if err := tx.QueryRow(`SELECT COUNT(*) FROM sqlite_master WHERE type = 'table' AND name = 'restore_seed_locks'`).Scan(&lockTableExists); err != nil {
		return err
	}
	if lockTableExists == 0 {
		return nil
	}
	var active int
	if err := tx.QueryRow(`SELECT COUNT(*) FROM restore_seed_locks WHERE lock_id = 'remote_seed' AND lease_until > CURRENT_TIMESTAMP`).Scan(&active); err != nil {
		return err
	}
	if active != 0 {
		return ErrRestoreSeedInProgress
	}
	return nil
}

// VacuumInto performs a SQLite online snapshot outside a transaction through
// the store's writer handle, as required for backups.
func (d *AppDatabase) VacuumInto(ctx context.Context, destination string) error {
	if d == nil || d.db == nil || d.driver != DriverSQLite {
		return errors.New("SQLite snapshot requires an available local writer handle")
	}
	ctx, cancel := withDefaultTimeout(ctx, TimeoutBulk)
	defer cancel()
	statement := "VACUUM INTO '" + strings.ReplaceAll(destination, "'", "''") + "'"
	return instrumentQuery(statement, func() error {
		_, err := d.db.ExecContext(ctx, statement)
		return err
	})
}

// WithSerializableRetry executes fn in a serializable-isolation transaction,
// retrying on transient serialization/lock conflicts per AI.md PART 10
// (SQLITE_BUSY / "database is locked" via isSerializationError). Used for
// correctness-critical multi-statement writes that need strict consistency.
func (d *AppDatabase) WithSerializableRetry(ctx context.Context, fn func(*sql.Tx) error) error {
	return d.Write(ctx, fn)
}

// HandleQueryError translates database errors to appropriate error codes
// Per AI.md PART 10
func HandleQueryError(err error) error {
	if err == nil {
		return nil
	}
	switch {
	case errors.Is(err, context.DeadlineExceeded):
		return errors.New("TIMEOUT: request timed out")
	case errors.Is(err, sql.ErrNoRows):
		return errors.New("NOT_FOUND: resource not found")
	case errors.Is(err, context.Canceled):
		return errors.New("CANCELED: request was canceled")
	default:
		return fmt.Errorf("SERVER_ERROR: database error: %w", err)
	}
}
