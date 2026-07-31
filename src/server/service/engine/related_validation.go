// SPDX-License-Identifier: MIT
package engine

import (
	"context"
	"log"
	"strings"
	"time"
)

// relatedTermTTL controls how long a validated candidate term's has-results
// status is trusted before being re-checked, since indexed content on the
// upstream engines drifts over time.
const relatedTermTTL = 12 * time.Hour

// relatedTermMaxCache bounds the validation cache so it never grows
// unbounded (AI.md PART 10: "Cache with TTL — never unbounded caches").
const relatedTermMaxCache = 2000

// quickValidationEngines limits result-validation probes to a small, fast,
// Tier-1 subset of engines rather than the full engine list, so validating a
// batch of generated candidate terms doesn't multiply full-search cost.
var quickValidationEngines = []string{"pornhub", "xvideos"}

// relatedTermStatus records the outcome of the last validation probe for a
// candidate related/suggested search term.
type relatedTermStatus struct {
	hasResults bool
	checkedAt  time.Time
}

// lookupRelatedTermStatus returns the cached has-results status for term and
// whether a still-fresh (within relatedTermTTL) entry exists.
func (m *EngineManager) lookupRelatedTermStatus(term string) (hasResults bool, fresh bool) {
	m.relatedTermMu.Lock()
	defer m.relatedTermMu.Unlock()
	st, ok := m.relatedTermCache[term]
	if !ok || time.Since(st.checkedAt) > relatedTermTTL {
		return false, false
	}
	return st.hasResults, true
}

// storeRelatedTermStatus records the has-results status for term, evicting
// the single oldest entry first if the cache is already at capacity.
func (m *EngineManager) storeRelatedTermStatus(term string, hasResults bool) {
	m.relatedTermMu.Lock()
	defer m.relatedTermMu.Unlock()
	if _, exists := m.relatedTermCache[term]; !exists && len(m.relatedTermCache) >= relatedTermMaxCache {
		var oldestTerm string
		var oldestTime time.Time
		for t, st := range m.relatedTermCache {
			if oldestTerm == "" || st.checkedAt.Before(oldestTime) {
				oldestTerm = t
				oldestTime = st.checkedAt
			}
		}
		if oldestTerm != "" {
			delete(m.relatedTermCache, oldestTerm)
		}
	}
	m.relatedTermCache[term] = relatedTermStatus{hasResults: hasResults, checkedAt: time.Now()}
}

// validateRelatedTermsAsync probes each not-yet-cached candidate term against
// a small, fast engine subset in the background. It never blocks the caller
// — a term's validated status simply is not available for the current
// request unless it was already cached from an earlier probe.
func (m *EngineManager) validateRelatedTermsAsync(candidates []string) {
	for _, raw := range candidates {
		term := strings.ToLower(strings.TrimSpace(raw))
		if term == "" {
			continue
		}
		if _, fresh := m.lookupRelatedTermStatus(term); fresh {
			continue
		}
		go func(term string) {
			defer func() {
				if rec := recover(); rec != nil {
					log.Printf("[related] panic validating term %q: %v", term, rec)
				}
			}()
			ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
			defer cancel()
			resp := m.Search(ctx, term, 1, quickValidationEngines, "")
			m.storeRelatedTermStatus(term, resp != nil && len(resp.Data.Results) > 0)
		}(term)
	}
}

// GetValidatedRelatedSearches returns related/suggested search terms known
// (from a prior probe search) to actually return results, filtering out any
// candidate already confirmed to dead-end. Not-yet-validated candidates are
// queued for background validation (see validateRelatedTermsAsync) and, only
// if too few validated terms are available yet, included unvalidated so the
// section is not left empty while the cache warms up.
func (m *EngineManager) GetValidatedRelatedSearches(query string, maxResults int) []string {
	if query == "" || maxResults <= 0 {
		return nil
	}

	// Overgenerate so filtering out unvalidated/known-bad terms still leaves
	// enough candidates to fill maxResults.
	candidates := GetRelatedSearches(query, maxResults*3)
	if len(candidates) == 0 {
		return nil
	}

	var validated []string
	var unknown []string
	for _, term := range candidates {
		normalized := strings.ToLower(strings.TrimSpace(term))
		hasResults, fresh := m.lookupRelatedTermStatus(normalized)
		switch {
		case fresh && hasResults:
			validated = append(validated, term)
		case fresh && !hasResults:
			// Known dead-end from a prior probe — skip it.
		default:
			unknown = append(unknown, term)
		}
	}

	m.validateRelatedTermsAsync(candidates)

	if len(validated) >= maxResults {
		return validated[:maxResults]
	}

	// Cache still warming (e.g. first time these candidates were ever
	// generated) — show plausible unvalidated terms rather than an empty
	// section; they will be filtered out on subsequent requests once
	// validated as dead ends.
	for _, term := range unknown {
		if len(validated) >= maxResults {
			break
		}
		validated = append(validated, term)
	}
	return validated
}
