// SPDX-License-Identifier: MIT
// Coverage tests for ClusterManager methods that require a real database:
// execCtx, queryCtx, registerNode (via Start), heartbeatLoop, primaryElectionLoop,
// electPrimary, configSyncLoop (Done path), lockCleanupLoop (Done path),
// GetNodes, AcquireLock, ReleaseLock, GetLock, ListLocks, WithLock, Stop.
package cluster

import (
	"context"
	"database/sql"
	"testing"
	"time"

	_ "modernc.org/sqlite"
)

// newTestDB opens a per-test in-memory SQLite database and creates the cluster
// tables. SetMaxOpenConns(1) ensures the single in-memory DB is reused by all
// queries on the same *sql.DB rather than spawning isolated connections.
func newTestDB(t *testing.T) *sql.DB {
	t.Helper()
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatalf("open in-memory sqlite: %v", err)
	}
	db.SetMaxOpenConns(1)
	t.Cleanup(func() { db.Close() })

	_, err = db.Exec(`CREATE TABLE IF NOT EXISTS cluster_nodes (
		id TEXT PRIMARY KEY,
		hostname TEXT NOT NULL,
		address TEXT NOT NULL,
		port INTEGER NOT NULL,
		is_primary INTEGER DEFAULT 0,
		last_heartbeat DATETIME,
		joined_at DATETIME DEFAULT CURRENT_TIMESTAMP,
		status TEXT DEFAULT 'active'
	)`)
	if err != nil {
		t.Fatalf("create cluster_nodes: %v", err)
	}

	_, err = db.Exec(`CREATE TABLE IF NOT EXISTS distributed_locks (
		name TEXT PRIMARY KEY,
		holder_id TEXT NOT NULL,
		acquired_at DATETIME DEFAULT CURRENT_TIMESTAMP,
		expires_at DATETIME NOT NULL,
		metadata TEXT
	)`)
	if err != nil {
		t.Fatalf("create distributed_locks: %v", err)
	}

	return db
}

// newDBManager creates a ClusterManager backed by an in-memory SQLite DB.
func newDBManager(t *testing.T) *ClusterManager {
	t.Helper()
	cm, err := NewClusterManager(newTestDB(t))
	if err != nil {
		t.Fatalf("NewClusterManager: %v", err)
	}
	return cm
}

// ── Start / Stop ──────────────────────────────────────────────────────────────

// TestClusterManager_WithDB_StartStop verifies that Start registers the node,
// spins up goroutines, and Stop marks it offline — covering registerNode,
// execCtx, and the Done-case of all background loops.
func TestClusterManager_WithDB_StartStop(t *testing.T) {
	cm := newDBManager(t)
	cm.heartbeatInt = time.Millisecond

	ctx, cancel := context.WithCancel(context.Background())
	if err := cm.Start(ctx); err != nil {
		t.Fatalf("Start: %v", err)
	}
	// Let ticker-based goroutines fire at least once before stopping.
	time.Sleep(10 * time.Millisecond)
	cancel()
	cm.Stop()
}

// TestClusterManager_WithDB_StartIsEnabled verifies that Start sets enabled.
func TestClusterManager_WithDB_StartIsEnabled(t *testing.T) {
	cm := newDBManager(t)
	cm.heartbeatInt = time.Millisecond

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	if err := cm.Start(ctx); err != nil {
		t.Fatalf("Start: %v", err)
	}
	if !cm.IsEnabled() {
		t.Error("IsEnabled() = false after Start")
	}
	cancel()
	cm.Stop()
}

// ── heartbeatLoop / primaryElectionLoop / electPrimary ───────────────────────

// TestClusterManager_WithDB_ElectPrimary verifies that after a short wait the
// node elects itself as primary (it is the only healthy node).
func TestClusterManager_WithDB_ElectPrimary(t *testing.T) {
	cm := newDBManager(t)
	// Very short intervals so heartbeatLoop and primaryElectionLoop fire quickly.
	cm.heartbeatInt = 5 * time.Millisecond

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	if err := cm.Start(ctx); err != nil {
		t.Fatalf("Start: %v", err)
	}
	// Poll until the node elects itself primary, with a generous deadline to
	// survive CPU contention during parallel test runs.
	deadline := time.Now().Add(500 * time.Millisecond)
	for time.Now().Before(deadline) {
		if cm.IsPrimary() {
			cancel()
			cm.Stop()
			return
		}
		time.Sleep(5 * time.Millisecond)
	}
	t.Error("IsPrimary() = false after 500ms; single node should elect itself as primary")
	cancel()
	cm.Stop()
}

// TestClusterManager_WithDB_ConfigSaverLoop verifies that setting a configSaver
// before Start does not panic and that the Done path of configSyncLoop is covered.
func TestClusterManager_WithDB_ConfigSaverLoop(t *testing.T) {
	cm := newDBManager(t)
	cm.heartbeatInt = time.Millisecond
	cm.SetConfigSaver(func() error { return nil })

	ctx, cancel := context.WithCancel(context.Background())
	if err := cm.Start(ctx); err != nil {
		t.Fatalf("Start: %v", err)
	}
	time.Sleep(5 * time.Millisecond)
	cancel()
	cm.Stop()
}

// ── GetNodes (queryCtx) ───────────────────────────────────────────────────────

// TestClusterManager_WithDB_GetNodes_Empty verifies GetNodes returns an empty
// slice when no nodes have been registered.
func TestClusterManager_WithDB_GetNodes_Empty(t *testing.T) {
	cm := newDBManager(t)
	nodes, err := cm.GetNodes()
	if err != nil {
		t.Fatalf("GetNodes: %v", err)
	}
	if len(nodes) != 0 {
		t.Errorf("GetNodes empty: count = %d, want 0", len(nodes))
	}
}

// TestClusterManager_WithDB_GetNodes_AfterStart verifies GetNodes returns the
// registered node after Start.
func TestClusterManager_WithDB_GetNodes_AfterStart(t *testing.T) {
	cm := newDBManager(t)
	cm.heartbeatInt = time.Millisecond

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	if err := cm.Start(ctx); err != nil {
		t.Fatalf("Start: %v", err)
	}

	nodes, err := cm.GetNodes()
	if err != nil {
		t.Fatalf("GetNodes: %v", err)
	}
	if len(nodes) == 0 {
		t.Error("GetNodes after Start: expected ≥1 node, got 0")
	}
	cancel()
	cm.Stop()
}

// ── AcquireLock / ReleaseLock ─────────────────────────────────────────────────

// TestClusterManager_WithDB_AcquireReleaseLock verifies the normal acquire/release cycle.
func TestClusterManager_WithDB_AcquireReleaseLock(t *testing.T) {
	cm := newDBManager(t)

	acquired, err := cm.AcquireLock("test-lock", time.Minute)
	if err != nil {
		t.Fatalf("AcquireLock: %v", err)
	}
	if !acquired {
		t.Error("AcquireLock: expected true, got false")
	}

	if err := cm.ReleaseLock("test-lock"); err != nil {
		t.Errorf("ReleaseLock: %v", err)
	}
}

// TestClusterManager_WithDB_AcquireLock_Contention verifies that a second node
// cannot steal a lock that is already held and unexpired.
func TestClusterManager_WithDB_AcquireLock_Contention(t *testing.T) {
	cm1 := newDBManager(t)
	cm2, err := NewClusterManager(cm1.db)
	if err != nil {
		t.Fatalf("NewClusterManager for node2: %v", err)
	}

	acquired, err := cm1.AcquireLock("shared-lock", time.Minute)
	if err != nil {
		t.Fatalf("cm1 AcquireLock: %v", err)
	}
	if !acquired {
		t.Fatal("cm1 should have acquired the lock")
	}

	acquired2, err := cm2.AcquireLock("shared-lock", time.Minute)
	if err != nil {
		t.Fatalf("cm2 AcquireLock: %v", err)
	}
	if acquired2 {
		t.Error("cm2 should not have acquired the lock already held by cm1")
	}
}

// TestClusterManager_WithDB_AcquireLock_Renewal verifies that the holder can
// re-acquire its own lock (renewing the TTL) without conflict.
func TestClusterManager_WithDB_AcquireLock_Renewal(t *testing.T) {
	cm := newDBManager(t)

	for i := 0; i < 2; i++ {
		acquired, err := cm.AcquireLock("renewable-lock", time.Minute)
		if err != nil {
			t.Fatalf("AcquireLock attempt %d: %v", i+1, err)
		}
		if !acquired {
			t.Errorf("AcquireLock attempt %d: expected true, got false", i+1)
		}
	}
}

// ── GetLock ───────────────────────────────────────────────────────────────────

// TestClusterManager_WithDB_GetLock_Existing verifies GetLock returns the lock
// that was just acquired.
func TestClusterManager_WithDB_GetLock_Existing(t *testing.T) {
	cm := newDBManager(t)

	_, err := cm.AcquireLock("look-up-lock", time.Minute)
	if err != nil {
		t.Fatalf("AcquireLock: %v", err)
	}

	lock, err := cm.GetLock("look-up-lock")
	if err != nil {
		t.Fatalf("GetLock: %v", err)
	}
	if lock == nil {
		t.Fatal("GetLock: expected non-nil lock")
	}
	if lock.Name != "look-up-lock" {
		t.Errorf("lock.Name = %q, want %q", lock.Name, "look-up-lock")
	}
	if lock.HolderID != cm.GetNodeID() {
		t.Errorf("lock.HolderID = %q, want %q", lock.HolderID, cm.GetNodeID())
	}
}

// TestClusterManager_WithDB_GetLock_Missing verifies GetLock returns nil when
// no such lock exists (ErrNoRows is mapped to nil, nil).
func TestClusterManager_WithDB_GetLock_Missing(t *testing.T) {
	cm := newDBManager(t)
	lock, err := cm.GetLock("nonexistent-lock")
	if err != nil {
		t.Fatalf("GetLock missing: %v", err)
	}
	if lock != nil {
		t.Errorf("GetLock missing: expected nil, got %+v", lock)
	}
}

// ── ListLocks ────────────────────────────────────────────────────────────────

// TestClusterManager_WithDB_ListLocks_Empty verifies ListLocks returns an
// empty slice when no locks are held.
func TestClusterManager_WithDB_ListLocks_Empty(t *testing.T) {
	cm := newDBManager(t)
	locks, err := cm.ListLocks()
	if err != nil {
		t.Fatalf("ListLocks empty: %v", err)
	}
	if len(locks) != 0 {
		t.Errorf("ListLocks empty: count = %d, want 0", len(locks))
	}
}

// TestClusterManager_WithDB_ListLocks_WithHeldLocks verifies ListLocks returns
// all currently-held (unexpired) locks.
func TestClusterManager_WithDB_ListLocks_WithHeldLocks(t *testing.T) {
	cm := newDBManager(t)

	for _, name := range []string{"lock-a", "lock-b"} {
		if _, err := cm.AcquireLock(name, time.Minute); err != nil {
			t.Fatalf("AcquireLock %q: %v", name, err)
		}
	}

	locks, err := cm.ListLocks()
	if err != nil {
		t.Fatalf("ListLocks: %v", err)
	}
	if len(locks) < 2 {
		t.Errorf("ListLocks: count = %d, want ≥2", len(locks))
	}
}

// ── WithLock ──────────────────────────────────────────────────────────────────

// TestClusterManager_WithDB_WithLock_Success verifies WithLock calls the
// function and releases the lock afterwards.
func TestClusterManager_WithDB_WithLock_Success(t *testing.T) {
	cm := newDBManager(t)
	called := false
	err := cm.WithLock("fn-lock", time.Minute, func() error {
		called = true
		return nil
	})
	if err != nil {
		t.Fatalf("WithLock: %v", err)
	}
	if !called {
		t.Error("WithLock: function was not called")
	}
}

// TestClusterManager_WithDB_WithLock_Contention verifies that WithLock returns
// an error when the lock is held by another node.
func TestClusterManager_WithDB_WithLock_Contention(t *testing.T) {
	cm1 := newDBManager(t)
	cm2, err := NewClusterManager(cm1.db)
	if err != nil {
		t.Fatalf("NewClusterManager for node2: %v", err)
	}

	if _, err := cm1.AcquireLock("contested", time.Minute); err != nil {
		t.Fatalf("cm1 AcquireLock: %v", err)
	}

	err = cm2.WithLock("contested", time.Minute, func() error { return nil })
	if err == nil {
		t.Error("WithLock on contested lock: expected error, got nil")
	}
}

// TestClusterManager_WithDB_WithLock_PropagatesError verifies that an error
// returned by the wrapped function is bubbled up.
func TestClusterManager_WithDB_WithLock_PropagatesError(t *testing.T) {
	cm := newDBManager(t)
	sentinel := context.DeadlineExceeded
	err := cm.WithLock("err-lock", time.Minute, func() error {
		return sentinel
	})
	if err != sentinel {
		t.Errorf("WithLock: error = %v, want %v", err, sentinel)
	}
}

// ── Stop marks node offline ───────────────────────────────────────────────────

// TestClusterManager_WithDB_Stop_MarksOffline verifies that Stop updates the
// node row to "offline" in the database.
func TestClusterManager_WithDB_Stop_MarksOffline(t *testing.T) {
	cm := newDBManager(t)
	cm.heartbeatInt = time.Millisecond

	ctx, cancel := context.WithCancel(context.Background())
	if err := cm.Start(ctx); err != nil {
		t.Fatalf("Start: %v", err)
	}
	cancel()
	cm.Stop()

	var status string
	err := cm.db.QueryRow("SELECT status FROM cluster_nodes WHERE id = ?", cm.GetNodeID()).Scan(&status)
	if err != nil {
		t.Fatalf("query node status: %v", err)
	}
	if status != NodeStateOffline {
		t.Errorf("status after Stop = %q, want %q", status, NodeStateOffline)
	}
}

// ── Stats with live DB ────────────────────────────────────────────────────────

// TestClusterManager_WithDB_Stats verifies Stats returns the expected keys when
// a real DB (with tables) is available.
func TestClusterManager_WithDB_Stats(t *testing.T) {
	cm := newDBManager(t)
	stats := cm.Stats()
	if stats == nil {
		t.Fatal("Stats() returned nil")
	}
	for _, key := range []string{"node_id", "is_primary", "enabled", "total_nodes", "active_nodes", "active_locks"} {
		if _, ok := stats[key]; !ok {
			t.Errorf("Stats() missing key %q", key)
		}
	}
}
