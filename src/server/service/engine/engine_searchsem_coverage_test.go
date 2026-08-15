// SPDX-License-Identifier: MIT
// AI.md PART 12 "Rate Limiting": coverage tests for the searchSem concurrency
// guard added to EngineManager.SearchWithOperators — verifies capacity is
// sized from Search.ConcurrentRequests (falling back to
// defaultSearchConcurrency when unset/invalid), and that a saturated
// semaphore produces the canonical RATE_LIMITED overload envelope instead of
// blocking indefinitely.
package engine

import (
	"context"
	"testing"
	"time"

	"github.com/apimgr/vidveil/src/config"
)

// ── searchSem capacity sizing ─────────────────────────────────────────────

func TestNewEngineManager_SearchSemCapacity_FromConfig(t *testing.T) {
	cfg := config.DefaultAppConfig()
	cfg.Search.ConcurrentRequests = 3
	m := NewEngineManager(cfg)
	if got := cap(m.searchSem); got != 3 {
		t.Fatalf("searchSem capacity = %d, want 3 (from Search.ConcurrentRequests)", got)
	}
}

func TestNewEngineManager_SearchSemCapacity_DefaultsWhenZero(t *testing.T) {
	cfg := config.DefaultAppConfig()
	cfg.Search.ConcurrentRequests = 0
	m := NewEngineManager(cfg)
	if got := cap(m.searchSem); got != defaultSearchConcurrency {
		t.Fatalf("searchSem capacity = %d, want defaultSearchConcurrency (%d) when ConcurrentRequests is 0", got, defaultSearchConcurrency)
	}
}

func TestNewEngineManager_SearchSemCapacity_DefaultsWhenNegative(t *testing.T) {
	cfg := config.DefaultAppConfig()
	cfg.Search.ConcurrentRequests = -1
	m := NewEngineManager(cfg)
	if got := cap(m.searchSem); got != defaultSearchConcurrency {
		t.Fatalf("searchSem capacity = %d, want defaultSearchConcurrency (%d) when ConcurrentRequests is negative", got, defaultSearchConcurrency)
	}
}

func TestNewEngineManager_SearchSemCapacity_DefaultsWhenNilConfig(t *testing.T) {
	m := NewEngineManager(nil)
	if got := cap(m.searchSem); got != defaultSearchConcurrency {
		t.Fatalf("searchSem capacity = %d, want defaultSearchConcurrency (%d) when appConfig is nil", got, defaultSearchConcurrency)
	}
}

// ── saturated semaphore -> RATE_LIMITED overload envelope ────────────────

// saturateSearchSem fills m.searchSem to capacity so the next
// SearchWithOperators call cannot immediately acquire a slot, exercising the
// searchQueueTimeout/ctx.Done wait path. Returns a release func that must be
// called (even via defer) to drain the slots back out.
func saturateSearchSem(m *EngineManager) (release func()) {
	n := cap(m.searchSem)
	for i := 0; i < n; i++ {
		m.searchSem <- struct{}{}
	}
	return func() {
		for i := 0; i < n; i++ {
			<-m.searchSem
		}
	}
}

func TestSearchWithOperators_SaturatedSem_ContextCancelled_ReturnsOverloadFast(t *testing.T) {
	cfg := config.DefaultAppConfig()
	cfg.Search.ConcurrentRequests = 1
	m := NewEngineManager(cfg)
	release := saturateSearchSem(m)
	defer release()

	// A context that's already near-expired makes SearchWithOperators hit the
	// ctx.Done() branch of the searchSem select instead of waiting out the
	// full searchQueueTimeout — keeps this test fast (AI.md PART 28 "One run,
	// then fix", no gratuitous multi-second sleeps).
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
	defer cancel()

	start := time.Now()
	resp := m.SearchWithOperators(ctx, "test", 1, nil, nil, nil, nil, false, "", 0, ResultFilterOptions{})
	elapsed := time.Since(start)

	if resp == nil {
		t.Fatal("SearchWithOperators: nil response")
	}
	if resp.Ok {
		t.Fatalf("SearchWithOperators with saturated searchSem: Ok = true, want false (RATE_LIMITED overload envelope)")
	}
	if resp.Error != "RATE_LIMITED" {
		t.Fatalf("SearchWithOperators with saturated searchSem: Error = %q, want RATE_LIMITED", resp.Error)
	}
	if resp.Message == "" {
		t.Error("SearchWithOperators overload response: Message is empty")
	}
	// Must fail fast (well under searchQueueTimeout), not hang until the
	// server's WriteTimeout — that's the entire point of the guard.
	if elapsed > searchQueueTimeout {
		t.Errorf("SearchWithOperators with cancelled ctx took %v, want well under searchQueueTimeout (%v)", elapsed, searchQueueTimeout)
	}
}

func TestSearchWithOperators_SaturatedSem_QueueTimeout_ReturnsOverload(t *testing.T) {
	cfg := config.DefaultAppConfig()
	cfg.Search.ConcurrentRequests = 1
	m := NewEngineManager(cfg)
	release := saturateSearchSem(m)
	defer release()

	start := time.Now()
	resp := m.SearchWithOperators(context.Background(), "test", 1, nil, nil, nil, nil, false, "", 0, ResultFilterOptions{})
	elapsed := time.Since(start)

	if resp == nil {
		t.Fatal("SearchWithOperators: nil response")
	}
	if resp.Ok || resp.Error != "RATE_LIMITED" {
		t.Fatalf("SearchWithOperators after searchQueueTimeout: Ok=%v Error=%q, want Ok=false Error=RATE_LIMITED", resp.Ok, resp.Error)
	}
	// Should return at (not much past) searchQueueTimeout, never hang longer.
	if elapsed < searchQueueTimeout {
		t.Errorf("SearchWithOperators returned after %v, want at least searchQueueTimeout (%v)", elapsed, searchQueueTimeout)
	}
	if elapsed > searchQueueTimeout+2*time.Second {
		t.Errorf("SearchWithOperators returned after %v, want close to searchQueueTimeout (%v)", elapsed, searchQueueTimeout)
	}
}

func TestSearchWithOperators_UnsaturatedSem_AcquiresSlotAndReleases(t *testing.T) {
	m := newEmptyMgr()
	resp := m.SearchWithOperators(context.Background(), "test", 1, nil, nil, nil, nil, false, "", 0, ResultFilterOptions{})
	if resp == nil {
		t.Fatal("SearchWithOperators: nil response")
	}
	// A normal (non-saturated) call must acquire and release its slot,
	// leaving the semaphore empty for the next caller.
	if len(m.searchSem) != 0 {
		t.Errorf("searchSem not released after SearchWithOperators returned: len = %d, want 0", len(m.searchSem))
	}
}
