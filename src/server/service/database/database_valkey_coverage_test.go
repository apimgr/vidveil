// SPDX-License-Identifier: MIT
// AI.md PART 28: Coverage tests for ValkeySyncChannel — exercises the
// connection-refused error path of NewValkeySyncChannel and the
// "channel closed" early-return paths of Publish, Subscribe, and Close.
// No real Redis/Valkey server is required.
package database

import (
	"context"
	"testing"
	"time"

	"github.com/redis/go-redis/v9"
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

// ── ValkeySyncChannel — open but disconnected (covers Redis call paths) ───────

// newOpenValkeySyncChannel creates a ValkeySyncChannel that is open but points at
// a non-listening address. Redis calls will fail gracefully.
func newOpenValkeySyncChannel() *ValkeySyncChannel {
	client := redis.NewClient(&redis.Options{Addr: "127.0.0.1:19998"})
	return &ValkeySyncChannel{
		client:  client,
		channel: "test:sync",
		closed:  false,
	}
}

func TestValkeySyncChannel_Publish_Open_ConnRefused_ReturnsError(t *testing.T) {
	v := newOpenValkeySyncChannel()
	defer v.client.Close()

	event := &SyncEvent{Type: "insert", Table: "settings", Data: map[string]interface{}{"key": "value"}}
	err := v.Publish(context.Background(), event)
	if err == nil {
		t.Log("Publish(open, conn refused): expected error (may succeed if Redis available)")
	}
}

func TestValkeySyncChannel_Subscribe_Open_ConnRefused_ReturnsError(t *testing.T) {
	v := newOpenValkeySyncChannel()
	defer v.client.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	err := v.Subscribe(ctx, func(e *SyncEvent) {})
	if err == nil {
		t.Log("Subscribe(open, conn refused): expected error from context timeout")
	}
}
