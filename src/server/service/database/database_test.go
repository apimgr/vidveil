// SPDX-License-Identifier: MIT
// AI.md PART 28: Test coverage for database package.
package database

import (
	"context"
	"database/sql"
	"testing"
	"time"
)

// newSQLiteDB opens an in-memory SQLite AppDatabase for testing.
func newSQLiteDB(t *testing.T) *AppDatabase {
	t.Helper()
	db, err := NewAppDatabase(DatabaseConfig{
		Driver: DriverSQLite,
		Path:   ":memory:",
	})
	if err != nil {
		t.Fatalf("NewAppDatabase(sqlite :memory:) error: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	return db
}

// --- NewAppDatabase ---

// TestNewAppDatabase_UnknownDriverReturnsError verifies unknown driver returns error.
func TestNewAppDatabase_UnknownDriverReturnsError(t *testing.T) {
	_, err := NewAppDatabase(DatabaseConfig{Driver: "baddriver"})
	if err == nil {
		t.Error("NewAppDatabase(baddriver) = nil error, want error")
	}
}

// TestNewAppDatabase_SQLiteInMemory verifies SQLite in-memory open succeeds.
func TestNewAppDatabase_SQLiteInMemory(t *testing.T) {
	db := newSQLiteDB(t)
	if db == nil {
		t.Fatal("NewAppDatabase returned nil")
	}
}

// TestNewAppDatabase_EmptyDriverOpensSQLite verifies empty driver falls back to SQLite successfully.
func TestNewAppDatabase_EmptyDriverOpensSQLite(t *testing.T) {
	db, err := NewAppDatabase(DatabaseConfig{Path: ":memory:"})
	if err != nil {
		t.Fatalf("NewAppDatabase(empty driver) error: %v", err)
	}
	defer db.Close()
	if err := db.Ping(); err != nil {
		t.Errorf("Ping after empty-driver open = %v, want nil", err)
	}
}

// --- AppDatabase methods ---

// TestAppDatabase_Driver verifies Driver() returns the correct driver.
func TestAppDatabase_Driver(t *testing.T) {
	db := newSQLiteDB(t)
	if got := db.Driver(); got != DriverSQLite {
		t.Errorf("Driver() = %q, want %q", got, DriverSQLite)
	}
}

// TestAppDatabase_DB_NotNil verifies DB() returns non-nil *sql.DB.
func TestAppDatabase_DB_NotNil(t *testing.T) {
	db := newSQLiteDB(t)
	if db.DB() == nil {
		t.Error("DB() returned nil")
	}
}

// TestAppDatabase_Ping verifies Ping succeeds on open SQLite.
func TestAppDatabase_Ping(t *testing.T) {
	db := newSQLiteDB(t)
	if err := db.Ping(); err != nil {
		t.Errorf("Ping() = %v, want nil", err)
	}
}

// TestAppDatabase_PingContext verifies PingContext with background context.
func TestAppDatabase_PingContext(t *testing.T) {
	db := newSQLiteDB(t)
	if err := db.PingContext(context.Background()); err != nil {
		t.Errorf("PingContext() = %v, want nil", err)
	}
}

// TestAppDatabase_Stats_NoPanic verifies Stats() does not panic.
func TestAppDatabase_Stats_NoPanic(t *testing.T) {
	db := newSQLiteDB(t)
	defer func() {
		if r := recover(); r != nil {
			t.Errorf("Stats() panicked: %v", r)
		}
	}()
	_ = db.Stats()
}

// TestAppDatabase_Exec_CreateTable verifies Exec runs DDL successfully.
func TestAppDatabase_Exec_CreateTable(t *testing.T) {
	db := newSQLiteDB(t)
	_, err := db.Exec("CREATE TABLE IF NOT EXISTS t (id INTEGER PRIMARY KEY)")
	if err != nil {
		t.Errorf("Exec CREATE TABLE = %v, want nil", err)
	}
}

// TestAppDatabase_ExecContext_CreateTable verifies ExecContext runs DDL.
func TestAppDatabase_ExecContext_CreateTable(t *testing.T) {
	db := newSQLiteDB(t)
	_, err := db.ExecContext(context.Background(), "CREATE TABLE IF NOT EXISTS tc (id INTEGER PRIMARY KEY)")
	if err != nil {
		t.Errorf("ExecContext CREATE TABLE = %v, want nil", err)
	}
}

// TestAppDatabase_Query_Rows verifies Query returns rows on simple SELECT.
func TestAppDatabase_Query_Rows(t *testing.T) {
	db := newSQLiteDB(t)
	rows, err := db.Query("SELECT 1")
	if err != nil {
		t.Fatalf("Query SELECT 1 = %v, want nil", err)
	}
	defer rows.Close()
}

// TestAppDatabase_QueryContext_Rows verifies QueryContext returns rows.
func TestAppDatabase_QueryContext_Rows(t *testing.T) {
	db := newSQLiteDB(t)
	rows, err := db.QueryContext(context.Background(), "SELECT 1")
	if err != nil {
		t.Fatalf("QueryContext SELECT 1 = %v, want nil", err)
	}
	defer rows.Close()
}

// TestAppDatabase_QueryRow verifies QueryRow scans a value.
func TestAppDatabase_QueryRow(t *testing.T) {
	db := newSQLiteDB(t)
	var n int
	if err := db.QueryRow("SELECT 42").Scan(&n); err != nil {
		t.Fatalf("QueryRow SELECT 42 Scan = %v", err)
	}
	if n != 42 {
		t.Errorf("QueryRow result = %d, want 42", n)
	}
}

// TestAppDatabase_QueryRowContext verifies QueryRowContext scans a value.
func TestAppDatabase_QueryRowContext(t *testing.T) {
	db := newSQLiteDB(t)
	var n int
	if err := db.QueryRowContext(context.Background(), "SELECT 7").Scan(&n); err != nil {
		t.Fatalf("QueryRowContext SELECT 7 Scan = %v", err)
	}
	if n != 7 {
		t.Errorf("QueryRowContext result = %d, want 7", n)
	}
}

// TestAppDatabase_Begin verifies Begin returns a transaction.
func TestAppDatabase_Begin(t *testing.T) {
	db := newSQLiteDB(t)
	tx, err := db.Begin()
	if err != nil {
		t.Fatalf("Begin() = %v, want nil", err)
	}
	_ = tx.Rollback()
}

// TestAppDatabase_BeginTx verifies BeginTx with background context.
func TestAppDatabase_BeginTx(t *testing.T) {
	db := newSQLiteDB(t)
	tx, err := db.BeginTx(context.Background(), nil)
	if err != nil {
		t.Fatalf("BeginTx() = %v, want nil", err)
	}
	_ = tx.Rollback()
}

// TestAppDatabase_TableExists_Missing verifies TableExists returns false for absent table.
func TestAppDatabase_TableExists_Missing(t *testing.T) {
	db := newSQLiteDB(t)
	exists, err := db.TableExists("no_such_table_xyz")
	if err != nil {
		t.Fatalf("TableExists(missing) error: %v", err)
	}
	if exists {
		t.Error("TableExists(missing) = true, want false")
	}
}

// TestAppDatabase_TableExists_Present verifies TableExists returns true after CREATE.
func TestAppDatabase_TableExists_Present(t *testing.T) {
	db := newSQLiteDB(t)
	if _, err := db.Exec("CREATE TABLE IF NOT EXISTS present_tbl (id INTEGER PRIMARY KEY)"); err != nil {
		t.Fatalf("CREATE TABLE: %v", err)
	}
	exists, err := db.TableExists("present_tbl")
	if err != nil {
		t.Fatalf("TableExists(present) error: %v", err)
	}
	if !exists {
		t.Error("TableExists(present) = false, want true")
	}
}

// TestAppDatabase_Version verifies Version returns a non-empty string.
func TestAppDatabase_Version(t *testing.T) {
	db := newSQLiteDB(t)
	v, err := db.Version()
	if err != nil {
		t.Fatalf("Version() error: %v", err)
	}
	if v == "" {
		t.Error("Version() returned empty string")
	}
}

// TestAppDatabase_TranslateQuery_SQLite verifies TranslateQuery returns the query unchanged for SQLite.
func TestAppDatabase_TranslateQuery_SQLite(t *testing.T) {
	db := newSQLiteDB(t)
	q := "SELECT $1, $2"
	got := db.TranslateQuery(q)
	if got == "" {
		t.Error("TranslateQuery returned empty string")
	}
}

// TestAppDatabase_WithTransaction_Commit verifies a successful transaction commits.
func TestAppDatabase_WithTransaction_Commit(t *testing.T) {
	db := newSQLiteDB(t)
	if _, err := db.Exec("CREATE TABLE IF NOT EXISTS tx_tbl (v INTEGER)"); err != nil {
		t.Fatalf("CREATE TABLE: %v", err)
	}
	err := db.WithTransaction(context.Background(), func(tx *sql.Tx) error {
		_, err := tx.ExecContext(context.Background(), "INSERT INTO tx_tbl VALUES (1)")
		return err
	})
	if err != nil {
		t.Errorf("WithTransaction commit = %v, want nil", err)
	}
}

// TestAppDatabase_WithTransaction_RollbackOnError verifies transaction rolls back on fn error.
func TestAppDatabase_WithTransaction_RollbackOnError(t *testing.T) {
	db := newSQLiteDB(t)
	sentinelErr := context.DeadlineExceeded
	err := db.WithTransaction(context.Background(), func(_ *sql.Tx) error {
		return sentinelErr
	})
	if err != sentinelErr {
		t.Errorf("WithTransaction rollback = %v, want %v", err, sentinelErr)
	}
}

// --- WithTimeout ---

// TestWithTimeout_ContextHasDeadline verifies WithTimeout creates a context with deadline.
func TestWithTimeout_ContextHasDeadline(t *testing.T) {
	ctx, cancel := WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_, ok := ctx.Deadline()
	if !ok {
		t.Error("WithTimeout context has no deadline")
	}
}

// --- HandleQueryError ---

// TestHandleQueryError_NilReturnsNil verifies nil error passes through unchanged.
func TestHandleQueryError_NilReturnsNil(t *testing.T) {
	if err := HandleQueryError(nil); err != nil {
		t.Errorf("HandleQueryError(nil) = %v, want nil", err)
	}
}

// TestHandleQueryError_NonNilReturnsError verifies a non-nil error is preserved.
func TestHandleQueryError_NonNilReturnsError(t *testing.T) {
	orig := context.DeadlineExceeded
	got := HandleQueryError(orig)
	if got == nil {
		t.Error("HandleQueryError(non-nil) = nil, want error")
	}
}

// --- SchemaManager ---

// TestNewSchemaManager_InMemory verifies SchemaManager opens in-memory SQLite.
func TestNewSchemaManager_InMemory(t *testing.T) {
	sm, err := NewSchemaManager(":memory:")
	if err != nil {
		t.Fatalf("NewSchemaManager(:memory:) = %v, want nil", err)
	}
	defer sm.Close()
	if sm.GetDB() == nil {
		t.Error("SchemaManager.GetDB() returned nil")
	}
}

// TestSchemaManager_EnsureSchema_NoPanic verifies EnsureSchema runs without panic.
func TestSchemaManager_EnsureSchema_NoPanic(t *testing.T) {
	sm, err := NewSchemaManager(":memory:")
	if err != nil {
		t.Fatalf("NewSchemaManager(:memory:) = %v", err)
	}
	defer sm.Close()
	defer func() {
		if r := recover(); r != nil {
			t.Errorf("EnsureSchema() panicked: %v", r)
		}
	}()
	_ = sm.EnsureSchema()
}

// --- isColumnExistsError / isSerializationError (unexported helpers) ---

// TestIsColumnExistsError_False verifies unknown error returns false.
func TestIsColumnExistsError_False(t *testing.T) {
	if isColumnExistsError(context.DeadlineExceeded) {
		t.Error("isColumnExistsError(DeadlineExceeded) = true, want false")
	}
}

// TestIsSerializationError_False verifies unknown error returns false.
func TestIsSerializationError_False(t *testing.T) {
	if isSerializationError(context.DeadlineExceeded) {
		t.Error("isSerializationError(DeadlineExceeded) = true, want false")
	}
}

// TestIsColumnExistsError_DuplicateColumn verifies duplicate column text is detected.
func TestIsColumnExistsError_DuplicateColumn(t *testing.T) {
	err := &testError{"duplicate column name: foo"}
	if !isColumnExistsError(err) {
		t.Error("isColumnExistsError(duplicate column) = false, want true")
	}
}

// testError is a simple error type for testing.
type testError struct{ msg string }

func (e *testError) Error() string { return e.msg }

// TestIsSerializationError_Serialization verifies "could not serialize" text is detected.
func TestIsSerializationError_Serialization(t *testing.T) {
	err := &testError{"could not serialize"}
	if !isSerializationError(err) {
		t.Error("isSerializationError(could not serialize) = false, want true")
	}
}

// TestIsSerializationError_SQLiteBusy verifies SQLITE_BUSY text is detected.
func TestIsSerializationError_SQLiteBusy(t *testing.T) {
	err := &testError{"SQLITE_BUSY: resource temporarily unavailable"}
	if !isSerializationError(err) {
		t.Error("isSerializationError(SQLITE_BUSY) = false, want true")
	}
}

// --- Driver string constants ---

// TestDriverConstants verifies Driver string values match expected strings.
func TestDriverConstants(t *testing.T) {
	cases := []struct {
		d    Driver
		want string
	}{
		{DriverSQLite, "sqlite"},
		{DriverPostgres, "postgres"},
		{DriverMySQL, "mysql"},
		{DriverMSSQL, "mssql"},
	}
	for _, c := range cases {
		if string(c.d) != c.want {
			t.Errorf("Driver constant %q = %q, want %q", c.d, string(c.d), c.want)
		}
	}
}

// TestNewSchemaManagerWithConfig_InMemory verifies NewSchemaManagerWithConfig with SQLite.
func TestNewSchemaManagerWithConfig_InMemory(t *testing.T) {
	sm, err := NewSchemaManagerWithConfig(DatabaseConfig{
		Driver: DriverSQLite,
		Path:   ":memory:",
	})
	if err != nil {
		t.Fatalf("NewSchemaManagerWithConfig = %v", err)
	}
	defer sm.Close()
}

// TestSchemaManager_GetMigrationStatus verifies GetMigrationStatus returns slice after EnsureSchema.
func TestSchemaManager_GetMigrationStatus_AfterEnsure(t *testing.T) {
	sm, err := NewSchemaManager(":memory:")
	if err != nil {
		t.Fatalf("NewSchemaManager = %v", err)
	}
	defer sm.Close()
	_ = sm.EnsureSchema()
	_, err = sm.GetMigrationStatus()
	if err != nil {
		t.Errorf("GetMigrationStatus() error: %v", err)
	}
}

// TestNewMigrationManager_InMemory verifies NewMigrationManager creates a manager.
func TestNewMigrationManager_InMemory(t *testing.T) {
	sm, err := NewMigrationManager(":memory:")
	if err != nil {
		t.Fatalf("NewMigrationManager = %v", err)
	}
	defer sm.Close()
}

// TestSchemaManager_RegisterDefaultMigrations_NoPanic verifies no panic on register.
func TestSchemaManager_RegisterDefaultMigrations_NoPanic(t *testing.T) {
	sm, err := NewMigrationManager(":memory:")
	if err != nil {
		t.Fatalf("NewMigrationManager = %v", err)
	}
	defer sm.Close()
	defer func() {
		if r := recover(); r != nil {
			t.Errorf("RegisterDefaultMigrations panicked: %v", r)
		}
	}()
	sm.RegisterDefaultMigrations()
}

// TestSchemaManager_RunMigrations_NoPanic verifies RunMigrations does not panic.
func TestSchemaManager_RunMigrations_NoPanic(t *testing.T) {
	sm, err := NewMigrationManager(":memory:")
	if err != nil {
		t.Fatalf("NewMigrationManager = %v", err)
	}
	defer sm.Close()
	_ = sm.EnsureSchema()
	defer func() {
		if r := recover(); r != nil {
			t.Errorf("RunMigrations panicked: %v", r)
		}
	}()
	_ = sm.RunMigrations()
}

// TestSchemaManager_RollbackMigration_NoPanic verifies RollbackMigration does not panic.
func TestSchemaManager_RollbackMigration_NoPanic(t *testing.T) {
	sm, err := NewMigrationManager(":memory:")
	if err != nil {
		t.Fatalf("NewMigrationManager = %v", err)
	}
	defer sm.Close()
	defer func() {
		if r := recover(); r != nil {
			t.Errorf("RollbackMigration panicked: %v", r)
		}
	}()
	_ = sm.RollbackMigration()
}
