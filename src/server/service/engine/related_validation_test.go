// SPDX-License-Identifier: MIT
// AI.md PART 14/16: Tests for the related-searches result-validation cache
// and EngineManager.GetValidatedRelatedSearches, added alongside
// related_validation.go per the project's same-pass-test rule.
package engine

import (
	"testing"
	"time"
)

// resetRelatedTermCache clears package-level validation cache state between
// tests so cases don't leak into each other.
func resetRelatedTermCache() {
	relatedTermMu.Lock()
	relatedTermCache = make(map[string]relatedTermStatus)
	relatedTermMu.Unlock()
}

func TestLookupRelatedTermStatus_MissingEntry_NotFresh(t *testing.T) {
	resetRelatedTermCache()
	_, fresh := lookupRelatedTermStatus("nonexistent term")
	if fresh {
		t.Fatal("expected fresh=false for a term never stored")
	}
}

func TestStoreAndLookupRelatedTermStatus_RoundTrip(t *testing.T) {
	resetRelatedTermCache()
	storeRelatedTermStatus("good term", true)
	storeRelatedTermStatus("bad term", false)

	if has, fresh := lookupRelatedTermStatus("good term"); !fresh || !has {
		t.Fatalf("good term: got has=%v fresh=%v, want has=true fresh=true", has, fresh)
	}
	if has, fresh := lookupRelatedTermStatus("bad term"); !fresh || has {
		t.Fatalf("bad term: got has=%v fresh=%v, want has=false fresh=true", has, fresh)
	}
}

func TestLookupRelatedTermStatus_ExpiredEntry_NotFresh(t *testing.T) {
	resetRelatedTermCache()
	relatedTermMu.Lock()
	relatedTermCache["stale term"] = relatedTermStatus{
		hasResults: true,
		checkedAt:  time.Now().Add(-(relatedTermTTL + time.Hour)),
	}
	relatedTermMu.Unlock()

	if _, fresh := lookupRelatedTermStatus("stale term"); fresh {
		t.Fatal("expected fresh=false for an entry older than relatedTermTTL")
	}
}

func TestStoreRelatedTermStatus_EvictsOldestWhenFull(t *testing.T) {
	resetRelatedTermCache()
	relatedTermMu.Lock()
	base := time.Now().Add(-24 * time.Hour)
	for i := 0; i < relatedTermMaxCache; i++ {
		term := "term-" + time.Duration(i).String()
		relatedTermCache[term] = relatedTermStatus{
			hasResults: true,
			checkedAt:  base.Add(time.Duration(i) * time.Second),
		}
	}
	relatedTermMu.Unlock()

	if len(relatedTermCache) != relatedTermMaxCache {
		t.Fatalf("setup: got %d entries, want %d", len(relatedTermCache), relatedTermMaxCache)
	}

	storeRelatedTermStatus("new term", true)

	relatedTermMu.Lock()
	size := len(relatedTermCache)
	_, newPresent := relatedTermCache["new term"]
	relatedTermMu.Unlock()

	if size != relatedTermMaxCache {
		t.Fatalf("after eviction: got %d entries, want %d (cache must stay bounded)", size, relatedTermMaxCache)
	}
	if !newPresent {
		t.Fatal("newly stored term should be present after eviction of the oldest entry")
	}
}

func TestGetValidatedRelatedSearches_EmptyQuery_ReturnsNil(t *testing.T) {
	resetRelatedTermCache()
	m := newEmptyMgr()
	if got := m.GetValidatedRelatedSearches("", 8); got != nil {
		t.Fatalf("empty query: got %v, want nil", got)
	}
}

func TestGetValidatedRelatedSearches_ZeroMaxResults_ReturnsNil(t *testing.T) {
	resetRelatedTermCache()
	m := newEmptyMgr()
	if got := m.GetValidatedRelatedSearches("some query", 0); got != nil {
		t.Fatalf("zero maxResults: got %v, want nil", got)
	}
}

func TestGetValidatedRelatedSearches_ColdCache_BackfillsUnvalidated(t *testing.T) {
	resetRelatedTermCache()
	m := newEmptyMgr()

	// On a never-before-seen query, no candidates have been validated yet,
	// so the result must still be non-empty (cold-cache backfill) as long as
	// GetRelatedSearches itself produces candidates for this query.
	candidates := GetRelatedSearches("amateur", 24)
	if len(candidates) == 0 {
		t.Skip("GetRelatedSearches produced no candidates for this query in this build; nothing to validate")
	}

	got := m.GetValidatedRelatedSearches("amateur", 8)
	if len(got) == 0 {
		t.Fatal("expected cold-cache backfill to return unvalidated candidates rather than an empty slice")
	}
	if len(got) > 8 {
		t.Fatalf("got %d results, want at most 8", len(got))
	}
}

func TestGetValidatedRelatedSearches_SkipsKnownDeadEnds(t *testing.T) {
	resetRelatedTermCache()
	m := newEmptyMgr()

	candidates := GetRelatedSearches("amateur", 24)
	if len(candidates) == 0 {
		t.Skip("GetRelatedSearches produced no candidates for this query in this build; nothing to validate")
	}

	// Mark every candidate as a known dead end.
	for _, c := range candidates {
		storeRelatedTermStatus(c, false)
	}

	got := m.GetValidatedRelatedSearches("amateur", 8)
	for _, term := range got {
		if has, fresh := lookupRelatedTermStatus(term); fresh && !has {
			t.Fatalf("result %q was a known dead end and should have been filtered out", term)
		}
	}
}

func TestGetValidatedRelatedSearches_PrefersValidatedGoodTerms(t *testing.T) {
	resetRelatedTermCache()
	m := newEmptyMgr()

	candidates := GetRelatedSearches("amateur", 24)
	if len(candidates) < 2 {
		t.Skip("GetRelatedSearches produced too few candidates for this query in this build")
	}

	storeRelatedTermStatus(candidates[0], true)
	for _, c := range candidates[1:] {
		storeRelatedTermStatus(c, false)
	}

	got := m.GetValidatedRelatedSearches("amateur", 8)
	found := false
	for _, term := range got {
		if term == candidates[0] {
			found = true
		}
	}
	if !found {
		t.Fatalf("expected validated-good term %q to appear in result %v", candidates[0], got)
	}
}
