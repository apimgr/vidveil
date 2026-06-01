// SPDX-License-Identifier: MIT
// Additional coverage tests for ValkeyCache closed-path branches and nodeID init.
package cache

import (
	"testing"
	"time"

	"github.com/apimgr/vidveil/src/server/model"
	"github.com/redis/go-redis/v9"
)

// newClosedValkeyCache builds a ValkeyCache with closed=true pointing at a
// non-listening address so none of the live-path branches can be reached.
func newClosedValkeyCache() *ValkeyCache {
	client := redis.NewClient(&redis.Options{Addr: "127.0.0.1:19999"})
	return &ValkeyCache{
		client: client,
		prefix: "vidveil:",
		ttl:    time.Second,
		closed: true,
	}
}

// ---- ValkeyCache closed=true paths ----

func TestValkeyCacheClosedGetReturnsNilFalse(t *testing.T) {
	vc := newClosedValkeyCache()
	defer vc.client.Close()

	resp, ok := vc.Get("key")
	if ok {
		t.Error("expected ok=false when cache is closed")
	}
	if resp != nil {
		t.Error("expected nil response when cache is closed")
	}
}

func TestValkeyCacheClosedSetNoPanic(t *testing.T) {
	vc := newClosedValkeyCache()
	defer vc.client.Close()

	// Must return early without panicking.
	vc.Set("key", &model.SearchResponse{})
}

func TestValkeyCacheClosedDeleteNoPanic(t *testing.T) {
	vc := newClosedValkeyCache()
	defer vc.client.Close()

	// Must return early without panicking.
	vc.Delete("key")
}

func TestValkeyCacheClosedClearNoPanic(t *testing.T) {
	vc := newClosedValkeyCache()
	defer vc.client.Close()

	// Must return early without panicking.
	vc.Clear()
}

func TestValkeyCacheClosedSizeReturnsZero(t *testing.T) {
	vc := newClosedValkeyCache()
	defer vc.client.Close()

	if sz := vc.Size(); sz != 0 {
		t.Errorf("expected Size()=0 when cache is closed, got %d", sz)
	}
}

func TestValkeyCacheClosedStatsContainsTypeAndClosedTrue(t *testing.T) {
	vc := newClosedValkeyCache()
	defer vc.client.Close()

	stats := vc.Stats()

	typeVal, ok := stats["type"]
	if !ok {
		t.Fatal("stats map missing key \"type\"")
	}
	if typeVal != "valkey" {
		t.Errorf("expected stats[\"type\"]=\"valkey\", got %v", typeVal)
	}

	closedVal, ok := stats["closed"]
	if !ok {
		t.Fatal("stats map missing key \"closed\"")
	}
	if closedVal != true {
		t.Errorf("expected stats[\"closed\"]=true, got %v", closedVal)
	}
}

// ---- ValkeyCache.Close on a live (closed=false) struct ----

func TestValkeyCacheCloseOnOpenStructNoPanic(t *testing.T) {
	client := redis.NewClient(&redis.Options{Addr: "127.0.0.1:19999"})
	vc := &ValkeyCache{
		client: client,
		prefix: "vidveil:",
		ttl:    time.Second,
		closed: false,
	}

	// Close sets closed=true and calls client.Close(); must not panic.
	_ = vc.Close()

	if !vc.closed {
		t.Error("expected closed=true after Close()")
	}
}

// ---- nodeID init ----

func TestNodeIDIsNonEmpty(t *testing.T) {
	// The init() function in lock.go guarantees nodeID is never empty;
	// verify that guarantee holds at test time.
	if nodeID == "" {
		t.Error("expected nodeID to be non-empty after package init")
	}
}
