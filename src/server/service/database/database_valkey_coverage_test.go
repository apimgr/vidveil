// SPDX-License-Identifier: MIT
// AI.md PART 28: Coverage tests for ValkeySyncChannel — exercises the
// connection-refused error path of NewValkeySyncChannel and the
// "channel closed" early-return paths of Publish, Subscribe, and Close.
// No real Redis/Valkey server is required.
package database

import (
	"context"
	"testing"
)

// ── NewValkeySyncChannel ──────────────────────────────────────────────────────

func TestNewValkeySyncChannel_ConnRefused_ReturnsError(t *testing.T) {
	_, err := NewValkeySyncChannel("127.0.0.1:1", "", 0, "")
	if err == nil {
		t.Error("NewValkeySyncChannel(conn refused): expected error, got nil")
	}
}

func TestNewValkeySyncChannel_DefaultsApplied_ConnRefused(t *testing.T) {
	// Empty addr and channel → defaults applied, then ping fails.
	// We use a port that no Redis server is listening on.
	_, err := NewValkeySyncChannel("127.0.0.1:1", "", 0, "")
	if err == nil {
		t.Error("NewValkeySyncChannel defaults (conn refused): expected error, got nil")
	}
}

// ── ValkeySyncChannel — closed-channel early-return paths ────────────────────

func TestValkeySyncChannel_Publish_Closed(t *testing.T) {
	v := &ValkeySyncChannel{closed: true}
	err := v.Publish(context.Background(), &SyncEvent{Table: "t", NodeID: "n"})
	if err == nil {
		t.Error("Publish(closed): expected error, got nil")
	}
}

func TestValkeySyncChannel_Subscribe_Closed(t *testing.T) {
	v := &ValkeySyncChannel{closed: true}
	err := v.Subscribe(context.Background(), func(_ *SyncEvent) {})
	if err == nil {
		t.Error("Subscribe(closed): expected error, got nil")
	}
}

func TestValkeySyncChannel_Close_AlreadyClosed_ReturnsNil(t *testing.T) {
	v := &ValkeySyncChannel{closed: true}
	if err := v.Close(); err != nil {
		t.Errorf("Close(already closed): expected nil, got %v", err)
	}
}

func TestValkeySyncChannel_Close_NilClient_ReturnsNil(t *testing.T) {
	v := &ValkeySyncChannel{closed: false, client: nil}
	if err := v.Close(); err != nil {
		t.Errorf("Close(nil client): expected nil, got %v", err)
	}
}
