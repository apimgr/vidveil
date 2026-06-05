// SPDX-License-Identifier: MIT
// AI.md PART 28: Coverage tests for database DDL functions.
// Tests getPostgresDDL, getMySQLDDL, getMSSQLDDL, getTablesDDL (all branches),
// RegisterDefaultMigrations, and TranslateQuery (non-SQLite paths).
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

func TestGetPostgresDDL_NonEmpty(t *testing.T) {
	sm := newSchemaManagerForDriver(DriverPostgres)
	ddl := sm.getPostgresDDL()
	if len(ddl) == 0 {
		t.Error("getPostgresDDL: returned empty slice")
	}
	for i, stmt := range ddl {
		if strings.TrimSpace(stmt) == "" {
			t.Errorf("getPostgresDDL[%d]: empty statement", i)
		}
	}
}

func TestGetPostgresDDL_ContainsCreateTable(t *testing.T) {
	sm := newSchemaManagerForDriver(DriverPostgres)
	for _, stmt := range sm.getPostgresDDL() {
		if strings.Contains(strings.ToUpper(stmt), "CREATE TABLE") {
			return
		}
	}
	t.Error("getPostgresDDL: no CREATE TABLE statement found")
}

func TestGetMySQLDDL_NonEmpty(t *testing.T) {
	sm := newSchemaManagerForDriver(DriverMySQL)
	ddl := sm.getMySQLDDL()
	if len(ddl) == 0 {
		t.Error("getMySQLDDL: returned empty slice")
	}
	for i, stmt := range ddl {
		if strings.TrimSpace(stmt) == "" {
			t.Errorf("getMySQLDDL[%d]: empty statement", i)
		}
	}
}

func TestGetMySQLDDL_ContainsCreateTable(t *testing.T) {
	sm := newSchemaManagerForDriver(DriverMySQL)
	for _, stmt := range sm.getMySQLDDL() {
		if strings.Contains(strings.ToUpper(stmt), "CREATE TABLE") {
			return
		}
	}
	t.Error("getMySQLDDL: no CREATE TABLE statement found")
}

func TestGetMSSQLDDL_NonEmpty(t *testing.T) {
	sm := newSchemaManagerForDriver(DriverMSSQL)
	ddl := sm.getMSSQLDDL()
	if len(ddl) == 0 {
		t.Error("getMSSQLDDL: returned empty slice")
	}
	for i, stmt := range ddl {
		if strings.TrimSpace(stmt) == "" {
			t.Errorf("getMSSQLDDL[%d]: empty statement", i)
		}
	}
}

func TestGetMSSQLDDL_ContainsCreateTable(t *testing.T) {
	sm := newSchemaManagerForDriver(DriverMSSQL)
	for _, stmt := range sm.getMSSQLDDL() {
		if strings.Contains(strings.ToUpper(stmt), "CREATE TABLE") {
			return
		}
	}
	t.Error("getMSSQLDDL: no CREATE TABLE statement found")
}

// ── getTablesDDL branch coverage ──────────────────────────────────────────────

func TestGetTablesDDL_PostgresBranch(t *testing.T) {
	sm := newSchemaManagerForDriver(DriverPostgres)
	ddl := sm.getTablesDDL()
	if len(ddl) == 0 {
		t.Error("getTablesDDL postgres: returned empty slice")
	}
}

func TestGetTablesDDL_MySQLBranch(t *testing.T) {
	sm := newSchemaManagerForDriver(DriverMySQL)
	ddl := sm.getTablesDDL()
	if len(ddl) == 0 {
		t.Error("getTablesDDL mysql: returned empty slice")
	}
}

func TestGetTablesDDL_MSSQLBranch(t *testing.T) {
	sm := newSchemaManagerForDriver(DriverMSSQL)
	ddl := sm.getTablesDDL()
	if len(ddl) == 0 {
		t.Error("getTablesDDL mssql: returned empty slice")
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

// ── TranslateQuery (non-SQLite drivers) ───────────────────────────────────────

func TestTranslateQuery_PostgresPassthrough(t *testing.T) {
	db := &AppDatabase{driver: DriverPostgres}
	input := "SELECT * FROM t WHERE id = $1 AND name = $2"
	got := db.TranslateQuery(input)
	if got != input {
		t.Errorf("TranslateQuery postgres: got %q, want unchanged %q", got, input)
	}
}

func TestTranslateQuery_MySQLPassthrough(t *testing.T) {
	db := &AppDatabase{driver: DriverMySQL}
	input := "SELECT * FROM t WHERE id = ? AND name = ?"
	got := db.TranslateQuery(input)
	if got != input {
		t.Errorf("TranslateQuery mysql: got %q, want unchanged %q", got, input)
	}
}

func TestTranslateQuery_MSSQLPassthrough(t *testing.T) {
	db := &AppDatabase{driver: DriverMSSQL}
	input := "INSERT INTO t (a,b) VALUES (?, ?)"
	got := db.TranslateQuery(input)
	if got != input {
		t.Errorf("TranslateQuery mssql: got %q, want unchanged %q", got, input)
	}
}
