// SPDX-License-Identifier: MIT
// AI.md PART 28: Coverage tests for sync.go pure/in-memory functions.
package database

import (
	"context"
	"errors"
	"testing"
	"time"
)

// ── joinStrings ───────────────────────────────────────────────────────────────

func TestJoinStrings_Empty(t *testing.T) {
	got := joinStrings([]string{}, ",")
	if got != "" {
		t.Errorf("joinStrings([]): expected '', got %q", got)
	}
}

func TestJoinStrings_Single(t *testing.T) {
	got := joinStrings([]string{"a"}, ",")
	if got != "a" {
		t.Errorf("joinStrings(single): expected 'a', got %q", got)
	}
}

func TestJoinStrings_Multiple(t *testing.T) {
	got := joinStrings([]string{"a", "b", "c"}, "-")
	if got != "a-b-c" {
		t.Errorf("joinStrings: expected 'a-b-c', got %q", got)
	}
}

// ── MemorySyncChannel ─────────────────────────────────────────────────────────

func TestMemorySyncChannel_NewNotNil(t *testing.T) {
	ch := NewMemorySyncChannel()
	if ch == nil {
		t.Fatal("NewMemorySyncChannel returned nil")
	}
}

func TestMemorySyncChannel_Close(t *testing.T) {
	ch := NewMemorySyncChannel()
	if err := ch.Close(); err != nil {
		t.Errorf("MemorySyncChannel.Close: unexpected error: %v", err)
	}
}

func TestMemorySyncChannel_PublishNoSubscribers(t *testing.T) {
	ch := NewMemorySyncChannel()
	event := &SyncEvent{Table: "test", NodeID: "n1"}
	if err := ch.Publish(context.Background(), event); err != nil {
		t.Errorf("Publish with no subscribers: unexpected error: %v", err)
	}
}

func TestMemorySyncChannel_PublishCallsSubscriber(t *testing.T) {
	ch := NewMemorySyncChannel()
	received := make(chan *SyncEvent, 1)

	ctx, cancel := context.WithCancel(context.Background())
	go func() {
		_ = ch.Subscribe(ctx, func(e *SyncEvent) {
			received <- e
		})
	}()

	// Give subscriber goroutine time to register
	time.Sleep(10 * time.Millisecond)

	event := &SyncEvent{Table: "videos", NodeID: "node1"}
	_ = ch.Publish(context.Background(), event)

	select {
	case got := <-received:
		if got.Table != "videos" {
			t.Errorf("subscriber got event with Table=%q, want 'videos'", got.Table)
		}
	case <-time.After(500 * time.Millisecond):
		t.Error("subscriber never received event within 500ms")
	}

	cancel()
}

func TestMemorySyncChannel_SubscribeCancelledByContext(t *testing.T) {
	ch := NewMemorySyncChannel()
	ctx, cancel := context.WithCancel(context.Background())

	done := make(chan error, 1)
	go func() {
		done <- ch.Subscribe(ctx, func(_ *SyncEvent) {})
	}()

	cancel()

	select {
	case err := <-done:
		if !errors.Is(err, context.Canceled) {
			t.Errorf("Subscribe returned %v, want context.Canceled", err)
		}
	case <-time.After(500 * time.Millisecond):
		t.Error("Subscribe did not return after context cancel")
	}
}

// ── SyncManager basic operations ─────────────────────────────────────────────

func TestNewSyncManager_NotNil(t *testing.T) {
	db := newSQLiteDB(t)
	ch := NewMemorySyncChannel()
	sm := NewSyncManager(db, ch, "node1")
	if sm == nil {
		t.Fatal("NewSyncManager returned nil")
	}
	// Stop cleans up context
	_ = sm.Stop()
}

func TestSyncManager_RegisterUnregister(t *testing.T) {
	db := newSQLiteDB(t)
	ch := NewMemorySyncChannel()
	sm := NewSyncManager(db, ch, "node1")
	defer sm.Stop()

	sm.RegisterTable("videos")
	sm.RegisterTable("searches")
	sm.UnregisterTable("videos")
}

func TestSyncManager_RecordChange_WhenDisabled(t *testing.T) {
	db := newSQLiteDB(t)
	ch := NewMemorySyncChannel()
	sm := NewSyncManager(db, ch, "node1")
	defer sm.Stop()

	// manager is not started — enabled=false; RecordChange should return nil
	err := sm.RecordChange(SyncEventInsert, "test_table", "1", map[string]interface{}{"key": "val"})
	if err != nil {
		t.Errorf("RecordChange while disabled: expected nil, got %v", err)
	}
}

func TestSyncManager_StartStop(t *testing.T) {
	db := newSQLiteDB(t)
	ch := NewMemorySyncChannel()
	sm := NewSyncManager(db, ch, "node1")

	if err := sm.Start(); err != nil {
		t.Errorf("SyncManager.Start: unexpected error: %v", err)
	}
	if err := sm.Stop(); err != nil {
		t.Errorf("SyncManager.Stop: unexpected error: %v", err)
	}
}

// ── SyncEventType constants ───────────────────────────────────────────────────

func TestSyncEventTypeConstants(t *testing.T) {
	types := []SyncEventType{SyncEventInsert, SyncEventUpdate, SyncEventDelete}
	for _, et := range types {
		if et == "" {
			t.Errorf("SyncEventType constant is empty string")
		}
	}
}
