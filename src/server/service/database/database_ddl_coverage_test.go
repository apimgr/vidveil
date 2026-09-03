// SPDX-License-Identifier: MIT
// AI.md PART 28: Coverage tests for database DDL functions.
// Per AI.md PART 3/10 only SQLite and libsql/Turso are supported; both share one DDL dialect.
package database

import (
	"strings"
	"testing"
)

// ── DDL function coverage ─────────────────────────────────────────────────────

// newSchemaManagerForDriver constructs a SchemaManager with the given driver
// without opening a real database connection. The db field is nil — only DDL
// helper methods (which do not use the db field) are safe to call.
func newSchemaManagerForDriver(d Driver) *SchemaManager {
	return &SchemaManager{driver: d}
}

func TestGetSQLiteDDL_NonEmpty(t *testing.T) {
	sm := newSchemaManagerForDriver(DriverSQLite)
	ddl := sm.getSQLiteDDL()
	if len(ddl) == 0 {
		t.Error("getSQLiteDDL: returned empty slice")
	}
	for i, stmt := range ddl {
		if strings.TrimSpace(stmt) == "" {
			t.Errorf("getSQLiteDDL[%d]: empty statement", i)
		}
	}
}

func TestGetSQLiteDDL_ContainsCreateTable(t *testing.T) {
	sm := newSchemaManagerForDriver(DriverSQLite)
	for _, stmt := range sm.getSQLiteDDL() {
		if strings.Contains(strings.ToUpper(stmt), "CREATE TABLE") {
			return
		}
	}
	t.Error("getSQLiteDDL: no CREATE TABLE statement found")
}

// ── getTablesDDL coverage ─────────────────────────────────────────────────────

func TestGetTablesDDL_SQLite(t *testing.T) {
	sm := newSchemaManagerForDriver(DriverSQLite)
	ddl := sm.getTablesDDL()
	if len(ddl) == 0 {
		t.Error("getTablesDDL sqlite: returned empty slice")
	}
}

func TestGetTablesDDL_LibSQL_SharesSQLiteDialect(t *testing.T) {
	sm := newSchemaManagerForDriver(DriverLibSQL)
	ddl := sm.getTablesDDL()
	if len(ddl) == 0 {
		t.Error("getTablesDDL libsql: returned empty slice")
	}
}

// ── RegisterDefaultMigrations ─────────────────────────────────────────────────

func TestRegisterDefaultMigrations_NoPanic(t *testing.T) {
	sm := newSchemaManagerForDriver(DriverSQLite)
	defer func() {
		if r := recover(); r != nil {
			t.Errorf("RegisterDefaultMigrations panicked: %v", r)
		}
	}()
	sm.RegisterDefaultMigrations()
}

// ── TranslateQuery ────────────────────────────────────────────────────────────

func TestTranslateQuery_Passthrough(t *testing.T) {
	db := &AppDatabase{driver: DriverSQLite}
	input := "SELECT * FROM t WHERE id = ? AND name = ?"
	got := db.TranslateQuery(input)
	if got != input {
		t.Errorf("TranslateQuery: got %q, want unchanged %q", got, input)
	}
}

func TestTranslateQuery_LibSQLPassthrough(t *testing.T) {
	db := &AppDatabase{driver: DriverLibSQL}
	input := "INSERT INTO t (a,b) VALUES (?, ?)"
	got := db.TranslateQuery(input)
	if got != input {
		t.Errorf("TranslateQuery libsql: got %q, want unchanged %q", got, input)
	}
}
