// SPDX-License-Identifier: MIT
// Tests for the cache package: SearchCache, CacheKey, NewSearchResultCache, MemoryLockStore, WithLock, and HTTP cache headers.
package cache

import (
	"context"
	"errors"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/apimgr/vidveil/src/server/model"
)

// makeResponse builds a minimal SearchResponse for use in tests.
func makeResponse(query string) *model.SearchResponse {
	return &model.SearchResponse{
		Ok: true,
		Data: model.SearchData{
			Query:       query,
			Results:     []model.VideoResult{{ID: "1", Title: "Test Video", URL: "https://example.com/v/1"}},
			EnginesUsed: []string{"testengine"},
		},
		Pagination: model.PaginationData{Page: 1, Limit: 10, Total: 1, Pages: 1},
	}
}

// ---- NewSearchCache defaults ----

func TestNewSearchCacheNonNil(t *testing.T) {
	c := NewSearchCache(5*time.Minute, 100)
	if c == nil {
		t.Fatal("expected non-nil cache")
	}
	c.Close()
}

func TestNewSearchCacheZeroTTLDefaultsFiveMinutes(t *testing.T) {
	c := NewSearchCache(0, 100)
	defer c.Close()
	if c.ttl != 5*time.Minute {
		t.Errorf("expected 5m default TTL, got %v", c.ttl)
	}
}

func TestNewSearchCacheZeroMaxSizeDefaults1000(t *testing.T) {
	c := NewSearchCache(5*time.Minute, 0)
	defer c.Close()
	if c.maxSize != 1000 {
		t.Errorf("expected 1000 default max size, got %d", c.maxSize)
	}
}

// ---- Get / Set ----

func TestGetOnEmptyCacheReturnsFalse(t *testing.T) {
	c := NewSearchCache(5*time.Minute, 100)
	defer c.Close()

	resp, ok := c.Get("missing-key")
	if ok {
		t.Error("expected ok=false on empty cache")
	}
	if resp != nil {
		t.Error("expected nil response on empty cache")
	}
}

func TestSetThenGetReturnsSameResponse(t *testing.T) {
	c := NewSearchCache(5*time.Minute, 100)
	defer c.Close()

	want := makeResponse("golang")
	c.Set("k1", want)

	got, ok := c.Get("k1")
	if !ok {
		t.Fatal("expected ok=true after Set")
	}
	if got.Data.Query != want.Data.Query {
		t.Errorf("expected query %q, got %q", want.Data.Query, got.Data.Query)
	}
}

func TestGetAfterTTLExpiredReturnsFalse(t *testing.T) {
	// Use a 1 ms TTL so the entry expires almost immediately.
	c := NewSearchCache(time.Millisecond, 100)
	defer c.Close()

	c.Set("expiring", makeResponse("expiring"))
	time.Sleep(5 * time.Millisecond)

	resp, ok := c.Get("expiring")
	if ok {
		t.Error("expected ok=false after TTL expired")
	}
	if resp != nil {
		t.Error("expected nil response after TTL expired")
	}
}

// ---- Delete ----

func TestDeleteRemovesKey(t *testing.T) {
	c := NewSearchCache(5*time.Minute, 100)
	defer c.Close()

	c.Set("del-me", makeResponse("del"))
	c.Delete("del-me")

	_, ok := c.Get("del-me")
	if ok {
		t.Error("expected key to be absent after Delete")
	}
}

func TestDeleteNonExistentKeyNoPanic(t *testing.T) {
	c := NewSearchCache(5*time.Minute, 100)
	defer c.Close()
	// Must not panic.
	c.Delete("does-not-exist")
}

// ---- Clear ----

func TestClearEmptiesCache(t *testing.T) {
	c := NewSearchCache(5*time.Minute, 100)
	defer c.Close()

	c.Set("a", makeResponse("a"))
	c.Set("b", makeResponse("b"))
	c.Clear()

	if c.Size() != 0 {
		t.Errorf("expected size=0 after Clear, got %d", c.Size())
	}
}

// ---- Size ----

func TestSizeReturnsCorrectCount(t *testing.T) {
	c := NewSearchCache(5*time.Minute, 100)
	defer c.Close()

	if c.Size() != 0 {
		t.Errorf("expected initial size=0, got %d", c.Size())
	}
	c.Set("x", makeResponse("x"))
	c.Set("y", makeResponse("y"))
	if c.Size() != 2 {
		t.Errorf("expected size=2, got %d", c.Size())
	}
}

// ---- Stats ----

func TestStatsContainsRequiredKeys(t *testing.T) {
	c := NewSearchCache(30*time.Second, 500)
	defer c.Close()

	stats := c.Stats()
	for _, key := range []string{"size", "max_size", "ttl_sec"} {
		if _, ok := stats[key]; !ok {
			t.Errorf("stats missing key %q", key)
		}
	}
	if stats["max_size"] != 500 {
		t.Errorf("expected max_size=500, got %v", stats["max_size"])
	}
	if stats["ttl_sec"] != (30 * time.Second).Seconds() {
		t.Errorf("expected ttl_sec=30, got %v", stats["ttl_sec"])
	}
}

// ---- Close ----

func TestCloseNoPanic(t *testing.T) {
	c := NewSearchCache(5*time.Minute, 100)
	if err := c.Close(); err != nil {
		t.Errorf("Close returned error: %v", err)
	}
	// Calling Close twice must not panic.
	if err := c.Close(); err != nil {
		t.Errorf("second Close returned error: %v", err)
	}
}

// ---- Eviction ----

func TestEvictionKeepsSizeWithinBounds(t *testing.T) {
	// maxSize=2, add 3 entries; the implementation evicts 10% (min 1) on overflow.
	// After the third Set, size must be ≤ 3 (eviction shrinks by at least 1).
	c := NewSearchCache(5*time.Minute, 2)
	defer c.Close()

	c.Set("e1", makeResponse("e1"))
	// Small sleep so timestamps differ and oldest-first ordering is reliable.
	time.Sleep(time.Millisecond)
	c.Set("e2", makeResponse("e2"))
	time.Sleep(time.Millisecond)
	c.Set("e3", makeResponse("e3"))

	if sz := c.Size(); sz > 3 {
		t.Errorf("expected size ≤ 3 after eviction, got %d", sz)
	}
}

// ---- CacheKey ----

func TestCacheKeyDeterministic(t *testing.T) {
	k1 := CacheKey("golang videos", 1, []string{"youtube", "vimeo"})
	k2 := CacheKey("golang videos", 1, []string{"youtube", "vimeo"})
	if k1 != k2 {
		t.Errorf("CacheKey not deterministic: %q != %q", k1, k2)
	}
}

func TestCacheKeyDiffersOnQuery(t *testing.T) {
	k1 := CacheKey("cats", 1, []string{"youtube"})
	k2 := CacheKey("dogs", 1, []string{"youtube"})
	if k1 == k2 {
		t.Error("expected different keys for different queries")
	}
}

func TestCacheKeyDiffersOnPage(t *testing.T) {
	k1 := CacheKey("test", 1, []string{"youtube"})
	k2 := CacheKey("test", 2, []string{"youtube"})
	if k1 == k2 {
		t.Error("expected different keys for different pages")
	}
}

func TestCacheKeyEmptyEnginesNoTrailingPipe(t *testing.T) {
	k := CacheKey("query", 1, []string{})
	if len(k) == 0 {
		t.Error("expected non-empty key")
	}
	// The key must not end with a bare pipe that would indicate a spurious engine entry.
	if k[len(k)-1] == '|' {
		t.Errorf("key ends with pipe for empty engines: %q", k)
	}
}

func TestCacheKeyNilEnginesNoTrailingPipe(t *testing.T) {
	k := CacheKey("query", 1, nil)
	if len(k) == 0 {
		t.Error("expected non-empty key")
	}
	if k[len(k)-1] == '|' {
		t.Errorf("key ends with pipe for nil engines: %q", k)
	}
}

// ---- NewSearchResultCache ----

func TestNewSearchResultCacheDefaultTypeReturnsMemoryCache(t *testing.T) {
	cfg := CacheConfig{
		Type:    CacheTypeMemory,
		TTL:     10,
		MaxSize: 100,
	}
	cache, err := NewSearchResultCache(cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cache == nil {
		t.Fatal("expected non-nil cache")
	}
	if err := cache.Close(); err != nil {
		t.Errorf("Close returned error: %v", err)
	}
}

func TestNewSearchResultCacheEmptyTypeDefaultsToMemory(t *testing.T) {
	cfg := CacheConfig{}
	cache, err := NewSearchResultCache(cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cache == nil {
		t.Fatal("expected non-nil cache")
	}
	cache.Close()
}

// ---- MemoryLockStore — AcquireLock ----

func TestAcquireLockFirstAcquireSucceeds(t *testing.T) {
	ls := NewMemoryLockStore()
	ctx := context.Background()

	ok, err := ls.AcquireLock(ctx, "resource1", time.Second)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !ok {
		t.Error("expected first acquire to return true")
	}
}

func TestAcquireLockHeldLockReturnsFalse(t *testing.T) {
	ls := NewMemoryLockStore()
	ctx := context.Background()

	_, _ = ls.AcquireLock(ctx, "resource2", time.Second)
	ok, err := ls.AcquireLock(ctx, "resource2", time.Second)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if ok {
		t.Error("expected second acquire on held lock to return false")
	}
}

func TestAcquireLockExpiredLockCanBeReacquired(t *testing.T) {
	ls := NewMemoryLockStore()
	ctx := context.Background()

	// Acquire with a 1 ms TTL, let it expire.
	_, _ = ls.AcquireLock(ctx, "explock", time.Millisecond)
	time.Sleep(5 * time.Millisecond)

	ok, err := ls.AcquireLock(ctx, "explock", time.Second)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !ok {
		t.Error("expected expired lock to be reacquirable")
	}
}

// ---- MemoryLockStore — ReleaseLock ----

func TestReleaseLockAllowsSubsequentAcquire(t *testing.T) {
	ls := NewMemoryLockStore()
	ctx := context.Background()

	_, _ = ls.AcquireLock(ctx, "relkey", time.Second)
	if err := ls.ReleaseLock(ctx, "relkey"); err != nil {
		t.Fatalf("ReleaseLock error: %v", err)
	}

	ok, err := ls.AcquireLock(ctx, "relkey", time.Second)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !ok {
		t.Error("expected acquire to succeed after release")
	}
}

func TestReleaseLockNonExistentKeyNoPanic(t *testing.T) {
	ls := NewMemoryLockStore()
	ctx := context.Background()
	// Must not panic or error.
	if err := ls.ReleaseLock(ctx, "never-acquired"); err != nil {
		t.Errorf("ReleaseLock on non-existent key returned error: %v", err)
	}
}

// ---- MemoryLockStore — IsLocked ----

func TestIsLockedUnlockedKeyReturnsFalse(t *testing.T) {
	ls := NewMemoryLockStore()
	ctx := context.Background()

	locked, err := ls.IsLocked(ctx, "free-key")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if locked {
		t.Error("expected false for unlocked key")
	}
}

func TestIsLockedLockedKeyReturnsTrue(t *testing.T) {
	ls := NewMemoryLockStore()
	ctx := context.Background()

	_, _ = ls.AcquireLock(ctx, "held", time.Second)
	locked, err := ls.IsLocked(ctx, "held")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !locked {
		t.Error("expected true for locked key")
	}
}

func TestIsLockedExpiredKeyReturnsFalse(t *testing.T) {
	ls := NewMemoryLockStore()
	ctx := context.Background()

	_, _ = ls.AcquireLock(ctx, "shortlived", time.Millisecond)
	time.Sleep(5 * time.Millisecond)

	locked, err := ls.IsLocked(ctx, "shortlived")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if locked {
		t.Error("expected false for expired lock")
	}
}

// ---- WithLock ----

func TestWithLockExecutesFn(t *testing.T) {
	ls := NewMemoryLockStore()
	ctx := context.Background()

	called := false
	err := WithLock(ctx, ls, "wl-key", time.Second, func() error {
		called = true
		return nil
	})
	if err != nil {
		t.Fatalf("WithLock returned error: %v", err)
	}
	if !called {
		t.Error("expected fn to be called when lock was free")
	}
}

func TestWithLockPropagatesFnError(t *testing.T) {
	ls := NewMemoryLockStore()
	ctx := context.Background()

	want := errors.New("fn error")
	got := WithLock(ctx, ls, "wl-err", time.Second, func() error {
		return want
	})
	if !errors.Is(got, want) {
		t.Errorf("expected fn error to propagate, got %v", got)
	}
}

func TestWithLockSkipsFnWhenAlreadyHeld(t *testing.T) {
	ls := NewMemoryLockStore()
	ctx := context.Background()

	// Hold the lock externally.
	_, _ = ls.AcquireLock(ctx, "wl-held", time.Second)

	called := false
	err := WithLock(ctx, ls, "wl-held", time.Second, func() error {
		called = true
		return nil
	})
	if err != nil {
		t.Fatalf("WithLock returned error when lock was held: %v", err)
	}
	if called {
		t.Error("expected fn NOT to be called when lock was already held")
	}
}

func TestWithLockReleasesLockAfterFn(t *testing.T) {
	ls := NewMemoryLockStore()
	ctx := context.Background()

	_ = WithLock(ctx, ls, "wl-release", time.Second, func() error {
		return nil
	})

	// Lock should be released; a second WithLock must be able to acquire it.
	called := false
	_ = WithLock(ctx, ls, "wl-release", time.Second, func() error {
		called = true
		return nil
	})
	if !called {
		t.Error("expected lock to be released after WithLock completes")
	}
}

// errLockStore is a fake LockStore that always returns an error from AcquireLock.
type errLockStore struct{}

func (e *errLockStore) AcquireLock(_ context.Context, _ string, _ time.Duration) (bool, error) {
	return false, errors.New("store unavailable")
}
func (e *errLockStore) ReleaseLock(_ context.Context, _ string) error { return nil }
func (e *errLockStore) IsLocked(_ context.Context, _ string) (bool, error) {
	return false, nil
}

func TestWithLockPropagatesAcquireError(t *testing.T) {
	ctx := context.Background()
	called := false
	err := WithLock(ctx, &errLockStore{}, "any", time.Second, func() error {
		called = true
		return nil
	})
	if err == nil {
		t.Error("expected error from acquire to propagate")
	}
	if called {
		t.Error("fn must not be called when AcquireLock errors")
	}
}

// TestNewSearchResultCacheValkeyFailsWithoutServer confirms that requesting a
// Valkey/Redis backend against a non-listening address returns an error — the
// connection-refused path is exercised without a real server.
func TestNewSearchResultCacheValkeyFailsWithoutServer(t *testing.T) {
	cfg := CacheConfig{
		Type: CacheTypeValkey,
		Addr: "127.0.0.1:19736",
		TTL:  5,
	}
	_, err := NewSearchResultCache(cfg)
	if err == nil {
		t.Error("expected error when Valkey server is not reachable")
	}
}

// TestNewSearchResultCacheRedisFailsWithoutServer mirrors the Valkey test for
// the redis type alias so that branch of NewSearchResultCache is covered.
func TestNewSearchResultCacheRedisFailsWithoutServer(t *testing.T) {
	cfg := CacheConfig{
		Type: CacheTypeRedis,
		Addr: "127.0.0.1:19737",
		TTL:  5,
	}
	_, err := NewSearchResultCache(cfg)
	if err == nil {
		t.Error("expected error when Redis server is not reachable")
	}
}

// ---- SetCacheHeaders ----

func TestSetCacheHeadersStatic(t *testing.T) {
	w := httptest.NewRecorder()
	SetCacheHeaders(w, ContentStatic, false)
	got := w.Header().Get("Cache-Control")
	want := "public, max-age=31536000, immutable"
	if got != want {
		t.Errorf("ContentStatic: got %q, want %q", got, want)
	}
}

func TestSetCacheHeadersAPI(t *testing.T) {
	w := httptest.NewRecorder()
	SetCacheHeaders(w, ContentAPI, false)
	got := w.Header().Get("Cache-Control")
	want := "public, max-age=60"
	if got != want {
		t.Errorf("ContentAPI: got %q, want %q", got, want)
	}
}

func TestSetCacheHeadersHTML(t *testing.T) {
	w := httptest.NewRecorder()
	SetCacheHeaders(w, ContentHTML, false)
	got := w.Header().Get("Cache-Control")
	want := "no-store"
	if got != want {
		t.Errorf("ContentHTML: got %q, want %q", got, want)
	}
}

func TestSetCacheHeadersPrivate(t *testing.T) {
	w := httptest.NewRecorder()
	SetCacheHeaders(w, ContentPrivate, false)
	got := w.Header().Get("Cache-Control")
	want := "private, no-store"
	if got != want {
		t.Errorf("ContentPrivate: got %q, want %q", got, want)
	}
}

func TestSetCacheHeadersError(t *testing.T) {
	w := httptest.NewRecorder()
	SetCacheHeaders(w, ContentError, false)
	got := w.Header().Get("Cache-Control")
	want := "no-store"
	if got != want {
		t.Errorf("ContentError: got %q, want %q", got, want)
	}
}

func TestSetCacheHeadersUnknownType(t *testing.T) {
	w := httptest.NewRecorder()
	SetCacheHeaders(w, ContentType("unknown"), false)
	got := w.Header().Get("Cache-Control")
	want := "no-store"
	if got != want {
		t.Errorf("unknown content type: got %q, want %q", got, want)
	}
}

func TestSetCacheHeadersAuthenticatedOverridesStatic(t *testing.T) {
	w := httptest.NewRecorder()
	SetCacheHeaders(w, ContentStatic, true)
	got := w.Header().Get("Cache-Control")
	want := "private, no-store"
	if got != want {
		t.Errorf("authenticated static: got %q, want %q", got, want)
	}
}

func TestSetCacheHeadersAuthenticatedOverridesAPI(t *testing.T) {
	w := httptest.NewRecorder()
	SetCacheHeaders(w, ContentAPI, true)
	got := w.Header().Get("Cache-Control")
	want := "private, no-store"
	if got != want {
		t.Errorf("authenticated API: got %q, want %q", got, want)
	}
}

// ---- Convenience helpers ----

func TestSetStaticCacheHeaders(t *testing.T) {
	w := httptest.NewRecorder()
	SetStaticCacheHeaders(w)
	got := w.Header().Get("Cache-Control")
	want := "public, max-age=31536000, immutable"
	if got != want {
		t.Errorf("SetStaticCacheHeaders: got %q, want %q", got, want)
	}
}

func TestSetAPICacheHeaders(t *testing.T) {
	w := httptest.NewRecorder()
	SetAPICacheHeaders(w)
	got := w.Header().Get("Cache-Control")
	want := "public, max-age=60"
	if got != want {
		t.Errorf("SetAPICacheHeaders: got %q, want %q", got, want)
	}
}

func TestSetNoCache(t *testing.T) {
	w := httptest.NewRecorder()
	SetNoCache(w)
	got := w.Header().Get("Cache-Control")
	want := "no-store"
	if got != want {
		t.Errorf("SetNoCache: got %q, want %q", got, want)
	}
}
