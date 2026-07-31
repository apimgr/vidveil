// SPDX-License-Identifier: MIT
// AI.md PART 14/16: Tests for the related-searches result-validation cache
// and EngineManager.GetValidatedRelatedSearches, added alongside
// related_validation.go per the project's same-pass-test rule.
package engine

import (
	"testing"
	"time"
)

func TestLookupRelatedTermStatus_MissingEntry_NotFresh(t *testing.T) {
	m := newEmptyMgr()
	_, fresh := m.lookupRelatedTermStatus("nonexistent term")
	if fresh {
		t.Fatal("expected fresh=false for a term never stored")
	}
}

func TestStoreAndLookupRelatedTermStatus_RoundTrip(t *testing.T) {
	m := newEmptyMgr()
	m.storeRelatedTermStatus("good term", true)
	m.storeRelatedTermStatus("bad term", false)

	if has, fresh := m.lookupRelatedTermStatus("good term"); !fresh || !has {
		t.Fatalf("good term: got has=%v fresh=%v, want has=true fresh=true", has, fresh)
	}
	if has, fresh := m.lookupRelatedTermStatus("bad term"); !fresh || has {
		t.Fatalf("bad term: got has=%v fresh=%v, want has=false fresh=true", has, fresh)
	}
}

func TestLookupRelatedTermStatus_ExpiredEntry_NotFresh(t *testing.T) {
	m := newEmptyMgr()
	m.relatedTermMu.Lock()
	m.relatedTermCache["stale term"] = relatedTermStatus{
		hasResults: true,
		checkedAt:  time.Now().Add(-(relatedTermTTL + time.Hour)),
	}
	m.relatedTermMu.Unlock()

	if _, fresh := m.lookupRelatedTermStatus("stale term"); fresh {
		t.Fatal("expected fresh=false for an entry older than relatedTermTTL")
	}
}

func TestStoreRelatedTermStatus_EvictsOldestWhenFull(t *testing.T) {
	m := newEmptyMgr()
	m.relatedTermMu.Lock()
	base := time.Now().Add(-24 * time.Hour)
	for i := 0; i < relatedTermMaxCache; i++ {
		term := "term-" + time.Duration(i).String()
		m.relatedTermCache[term] = relatedTermStatus{
			hasResults: true,
			checkedAt:  base.Add(time.Duration(i) * time.Second),
		}
	}
	m.relatedTermMu.Unlock()

	if len(m.relatedTermCache) != relatedTermMaxCache {
		t.Fatalf("setup: got %d entries, want %d", len(m.relatedTermCache), relatedTermMaxCache)
	}

	m.storeRelatedTermStatus("new term", true)

	m.relatedTermMu.Lock()
	size := len(m.relatedTermCache)
	_, newPresent := m.relatedTermCache["new term"]
	m.relatedTermMu.Unlock()

	if size != relatedTermMaxCache {
		t.Fatalf("after eviction: got %d entries, want %d (cache must stay bounded)", size, relatedTermMaxCache)
	}
	if !newPresent {
		t.Fatal("newly stored term should be present after eviction of the oldest entry")
	}
}

func TestGetValidatedRelatedSearches_EmptyQuery_ReturnsNil(t *testing.T) {
	m := newEmptyMgr()
	if got := m.GetValidatedRelatedSearches("", 8); got != nil {
		t.Fatalf("empty query: got %v, want nil", got)
	}
}

func TestGetValidatedRelatedSearches_ZeroMaxResults_ReturnsNil(t *testing.T) {
	m := newEmptyMgr()
	if got := m.GetValidatedRelatedSearches("some query", 0); got != nil {
		t.Fatalf("zero maxResults: got %v, want nil", got)
	}
}

func TestGetValidatedRelatedSearches_ColdCache_BackfillsUnvalidated(t *testing.T) {
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
	m := newEmptyMgr()

	candidates := GetRelatedSearches("amateur", 24)
	if len(candidates) == 0 {
		t.Skip("GetRelatedSearches produced no candidates for this query in this build; nothing to validate")
	}

	// Mark every candidate as a known dead end.
	for _, c := range candidates {
		m.storeRelatedTermStatus(c, false)
	}

	got := m.GetValidatedRelatedSearches("amateur", 8)
	for _, term := range got {
		if has, fresh := m.lookupRelatedTermStatus(term); fresh && !has {
			t.Fatalf("result %q was a known dead end and should have been filtered out", term)
		}
	}
}

func TestGetValidatedRelatedSearches_PrefersValidatedGoodTerms(t *testing.T) {
	m := newEmptyMgr()

	candidates := GetRelatedSearches("amateur", 24)
	if len(candidates) < 2 {
		t.Skip("GetRelatedSearches produced too few candidates for this query in this build")
	}

	m.storeRelatedTermStatus(candidates[0], true)
	for _, c := range candidates[1:] {
		m.storeRelatedTermStatus(c, false)
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
