// SPDX-License-Identifier: MIT
// AI.md PART 28: Coverage tests for openLibSQL and sync.go apply/handle
// functions. sql.Open for the libsql driver succeeds without a real server
// (driver registration only); actual queries are not made.
package database

import (
	"testing"
)

// ── openLibSQL ────────────────────────────────────────────────────────────────

func TestNewAppDatabase_LibSQL_EmptyURL_ReturnsError(t *testing.T) {
	cfg := DatabaseConfig{Driver: DriverLibSQL}
	_, err := NewAppDatabase(cfg)
	if err == nil {
		t.Fatal("NewAppDatabase libsql without URL: expected error, got nil")
	}
}

func TestNewAppDatabase_LibSQL_Opens(t *testing.T) {
	cfg := DatabaseConfig{
		Driver: DriverLibSQL,
		URL:    "libsql://example.turso.io",
	}
	db, err := NewAppDatabase(cfg)
	if err != nil {
		t.Fatalf("NewAppDatabase libsql: unexpected error: %v", err)
	}
	defer db.Close()
	if db.Driver() != DriverLibSQL {
		t.Errorf("Driver() = %v, want %v", db.Driver(), DriverLibSQL)
	}
}

func TestNewAppDatabase_LibSQL_TursoAlias(t *testing.T) {
	cfg := DatabaseConfig{
		Driver: "turso",
		URL:    "libsql://example.turso.io",
	}
	db, err := NewAppDatabase(cfg)
	if err != nil {
		t.Fatalf("NewAppDatabase turso alias: %v", err)
	}
	defer db.Close()
	if db.Driver() != DriverLibSQL {
		t.Errorf("Driver() = %v, want %v", db.Driver(), DriverLibSQL)
	}
}

func TestNewAppDatabase_LibSQL_WithToken_Opens(t *testing.T) {
	// Token is appended as ?authToken= when the URL has no query string
	cfg := DatabaseConfig{
		Driver: DriverLibSQL,
		URL:    "libsql://example.turso.io",
		Token:  "test-token",
	}
	db, err := NewAppDatabase(cfg)
	if err != nil {
		t.Fatalf("NewAppDatabase libsql with token: %v", err)
	}
	defer db.Close()
}

func TestNewAppDatabase_LibSQL_TokenWithExistingQuery_Opens(t *testing.T) {
	// Token is appended as &authToken= when the URL already has a query string
	cfg := DatabaseConfig{
		Driver: DriverLibSQL,
		URL:    "libsql://example.turso.io?tls=1",
		Token:  "test-token",
	}
	db, err := NewAppDatabase(cfg)
	if err != nil {
		t.Fatalf("NewAppDatabase libsql token with query: %v", err)
	}
	defer db.Close()
}

func TestNewAppDatabase_LibSQL_TokenAlreadyInURL_Opens(t *testing.T) {
	// Token is NOT re-appended when authToken= is already present in the URL
	cfg := DatabaseConfig{
		Driver: DriverLibSQL,
		URL:    "libsql://example.turso.io?authToken=existing",
		Token:  "ignored-token",
	}
	db, err := NewAppDatabase(cfg)
	if err != nil {
		t.Fatalf("NewAppDatabase libsql token in URL: %v", err)
	}
	defer db.Close()
}

func TestNewAppDatabase_UnknownDriver_ReturnsError(t *testing.T) {
	cfg := DatabaseConfig{Driver: "oracle"}
	_, err := NewAppDatabase(cfg)
	if err == nil {
		t.Error("NewAppDatabase(oracle): expected error, got nil")
	}
}

// ── sync.go: apply functions via in-memory SQLite ────────────────────────────

const createSyncTable = "CREATE TABLE IF NOT EXISTS sync_test (id INTEGER PRIMARY KEY, name TEXT)"

func TestSyncManager_ApplyInsert_EmptyData_ReturnsNil(t *testing.T) {
	db := newSQLiteDB(t)
	ch := NewMemorySyncChannel()
	sm := NewSyncManager(db, ch, "node1")
	defer sm.Stop()

	event := &SyncEvent{Type: SyncEventInsert, Table: "sync_test", Data: nil}
	if err := sm.applyInsert(event); err != nil {
		t.Errorf("applyInsert(empty data): expected nil, got %v", err)
	}
}

func TestSyncManager_ApplyInsert_WithData(t *testing.T) {
	db := newSQLiteDB(t)
	if _, err := db.Exec(createSyncTable); err != nil {
		t.Fatal(err)
	}
	ch := NewMemorySyncChannel()
	sm := NewSyncManager(db, ch, "node1")
	defer sm.Stop()

	event := &SyncEvent{
		Type:  SyncEventInsert,
		Table: "sync_test",
		Data:  map[string]interface{}{"id": int64(1), "name": "hello"},
	}
	if err := sm.applyInsert(event); err != nil {
		t.Errorf("applyInsert: unexpected error: %v", err)
	}
}

func TestSyncManager_ApplyUpdate_EmptyData_ReturnsNil(t *testing.T) {
	db := newSQLiteDB(t)
	ch := NewMemorySyncChannel()
	sm := NewSyncManager(db, ch, "node1")
	defer sm.Stop()

	event := &SyncEvent{Type: SyncEventUpdate, Table: "sync_test", Data: nil}
	if err := sm.applyUpdate(event); err != nil {
		t.Errorf("applyUpdate(empty data): expected nil, got %v", err)
	}
}

func TestSyncManager_ApplyUpdate_WithData(t *testing.T) {
	db := newSQLiteDB(t)
	if _, err := db.Exec(createSyncTable); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec("INSERT INTO sync_test VALUES (1, 'original')"); err != nil {
		t.Fatal(err)
	}
	ch := NewMemorySyncChannel()
	sm := NewSyncManager(db, ch, "node1")
	defer sm.Stop()

	event := &SyncEvent{
		Type:       SyncEventUpdate,
		Table:      "sync_test",
		PrimaryKey: int64(1),
		Data:       map[string]interface{}{"name": "updated"},
	}
	if err := sm.applyUpdate(event); err != nil {
		t.Errorf("applyUpdate: unexpected error: %v", err)
	}
}

func TestSyncManager_ApplyDelete_WithRow(t *testing.T) {
	db := newSQLiteDB(t)
	if _, err := db.Exec(createSyncTable); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec("INSERT INTO sync_test VALUES (1, 'to-delete')"); err != nil {
		t.Fatal(err)
	}
	ch := NewMemorySyncChannel()
	sm := NewSyncManager(db, ch, "node1")
	defer sm.Stop()

	event := &SyncEvent{
		Type:       SyncEventDelete,
		Table:      "sync_test",
		PrimaryKey: int64(1),
	}
	if err := sm.applyDelete(event); err != nil {
		t.Errorf("applyDelete: unexpected error: %v", err)
	}
}

func TestSyncManager_ApplyEvent_Insert(t *testing.T) {
	db := newSQLiteDB(t)
	if _, err := db.Exec(createSyncTable); err != nil {
		t.Fatal(err)
	}
	ch := NewMemorySyncChannel()
	sm := NewSyncManager(db, ch, "node1")
	defer sm.Stop()

	event := &SyncEvent{
		Type:  SyncEventInsert,
		Table: "sync_test",
		Data:  map[string]interface{}{"id": int64(10), "name": "ev"},
	}
	if err := sm.applyEvent(event); err != nil {
		t.Errorf("applyEvent(INSERT): unexpected error: %v", err)
	}
}

func TestSyncManager_ApplyEvent_Update(t *testing.T) {
	db := newSQLiteDB(t)
	if _, err := db.Exec(createSyncTable); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec("INSERT INTO sync_test VALUES (2, 'row')"); err != nil {
		t.Fatal(err)
	}
	ch := NewMemorySyncChannel()
	sm := NewSyncManager(db, ch, "node1")
	defer sm.Stop()

	event := &SyncEvent{
		Type:       SyncEventUpdate,
		Table:      "sync_test",
		PrimaryKey: int64(2),
		Data:       map[string]interface{}{"name": "changed"},
	}
	if err := sm.applyEvent(event); err != nil {
		t.Errorf("applyEvent(UPDATE): unexpected error: %v", err)
	}
}

func TestSyncManager_ApplyEvent_Delete(t *testing.T) {
	db := newSQLiteDB(t)
	if _, err := db.Exec(createSyncTable); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec("INSERT INTO sync_test VALUES (3, 'del')"); err != nil {
		t.Fatal(err)
	}
	ch := NewMemorySyncChannel()
	sm := NewSyncManager(db, ch, "node1")
	defer sm.Stop()

	event := &SyncEvent{
		Type:       SyncEventDelete,
		Table:      "sync_test",
		PrimaryKey: int64(3),
	}
	if err := sm.applyEvent(event); err != nil {
		t.Errorf("applyEvent(DELETE): unexpected error: %v", err)
	}
}

func TestSyncManager_ApplyEvent_UnknownType_ReturnsError(t *testing.T) {
	db := newSQLiteDB(t)
	ch := NewMemorySyncChannel()
	sm := NewSyncManager(db, ch, "node1")
	defer sm.Stop()

	event := &SyncEvent{Type: SyncEventType("BOGUS"), Table: "t"}
	if err := sm.applyEvent(event); err == nil {
		t.Error("applyEvent(BOGUS): expected error, got nil")
	}
}

// ── handleEvent paths ─────────────────────────────────────────────────────────

func TestSyncManager_HandleEvent_OwnNodeSkipped(t *testing.T) {
	db := newSQLiteDB(t)
	ch := NewMemorySyncChannel()
	sm := NewSyncManager(db, ch, "node1")
	if err := sm.Start(); err != nil {
		t.Fatal(err)
	}
	defer sm.Stop()
	sm.RegisterTable("sync_test")

	event := &SyncEvent{NodeID: "node1", Type: SyncEventInsert, Table: "sync_test"}
	sm.handleEvent(event)
}

func TestSyncManager_HandleEvent_Disabled_Noop(t *testing.T) {
	db := newSQLiteDB(t)
	ch := NewMemorySyncChannel()
	sm := NewSyncManager(db, ch, "node1")
	defer sm.Stop()
	sm.RegisterTable("sync_test")

	event := &SyncEvent{NodeID: "node2", Type: SyncEventInsert, Table: "sync_test"}
	sm.handleEvent(event)
}

func TestSyncManager_HandleEvent_UnregisteredTable_Noop(t *testing.T) {
	db := newSQLiteDB(t)
	ch := NewMemorySyncChannel()
	sm := NewSyncManager(db, ch, "node1")
	if err := sm.Start(); err != nil {
		t.Fatal(err)
	}
	defer sm.Stop()

	event := &SyncEvent{NodeID: "node2", Type: SyncEventInsert, Table: "not_registered"}
	sm.handleEvent(event)
}

func TestSyncManager_HandleEvent_OtherNode_AppliesInsert(t *testing.T) {
	db := newSQLiteDB(t)
	if _, err := db.Exec(createSyncTable); err != nil {
		t.Fatal(err)
	}
	ch := NewMemorySyncChannel()
	sm := NewSyncManager(db, ch, "node1")
	if err := sm.Start(); err != nil {
		t.Fatal(err)
	}
	defer sm.Stop()
	sm.RegisterTable("sync_test")

	event := &SyncEvent{
		NodeID: "node2",
		Type:   SyncEventInsert,
		Table:  "sync_test",
		Data:   map[string]interface{}{"id": int64(99), "name": "applied"},
	}
	sm.handleEvent(event)
}

// ── RecordChange when enabled and table is registered ─────────────────────────

func TestSyncManager_RecordChange_EnabledRegistered(t *testing.T) {
	db := newSQLiteDB(t)
	ch := NewMemorySyncChannel()
	sm := NewSyncManager(db, ch, "node1")
	if err := sm.Start(); err != nil {
		t.Fatal(err)
	}
	defer sm.Stop()
	sm.RegisterTable("test_table")

	err := sm.RecordChange(SyncEventInsert, "test_table", "1", map[string]interface{}{"k": "v"})
	if err != nil {
		t.Errorf("RecordChange when enabled+registered: unexpected error: %v", err)
	}
}

func TestSyncManager_RecordChange_EnabledUnregisteredTable(t *testing.T) {
	db := newSQLiteDB(t)
	ch := NewMemorySyncChannel()
	sm := NewSyncManager(db, ch, "node1")
	if err := sm.Start(); err != nil {
		t.Fatal(err)
	}
	defer sm.Stop()

	err := sm.RecordChange(SyncEventInsert, "not_registered", "1", nil)
	if err != nil {
		t.Errorf("RecordChange unregistered table: expected nil, got %v", err)
	}
}
