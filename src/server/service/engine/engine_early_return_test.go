// SPDX-License-Identifier: MIT
// AI.md PART 28: first-page-fast early return — once the accepted pool
// reaches resultsPerPage*earlyReturnHeadroomFactor, SearchWithOperators must
// respond immediately instead of waiting out the slowest engines, and the
// engines it stopped waiting for must be marked skipped, never failed.
package engine

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/apimgr/vidveil/src/config"
	"github.com/apimgr/vidveil/src/server/model"
)

// earlyReturnResults builds n clearly distinct valid results that all match
// the single-term query "hello" and are far enough apart to survive the
// fuzzy title dedup.
func earlyReturnResults(n int) []model.VideoResult {
	distinct := []string{
		"alpha wonder", "bravo galaxy", "charlie mountain", "delta ocean",
		"echo forest", "foxtrot desert", "golf river", "hotel canyon",
		"india valley", "juliet glacier", "kilo island", "lima volcano",
	}
	results := make([]model.VideoResult, 0, n)
	for i := 0; i < n; i++ {
		title := fmt.Sprintf("hello %s %d", distinct[i%len(distinct)], i)
		results = append(results, validResult(title, fmt.Sprintf("https://example.com/er/%d", i)))
	}
	return results
}

func TestSearchWithOperators_EarlyReturn_PageFilledSkipsSlowEngine(t *testing.T) {
	cfg := config.DefaultAppConfig()
	// Long per-engine budget: batchDeadline = 10s + 2s grace = 12s, so a
	// fast response can only come from the early-return path, never from
	// the deadline firing.
	cfg.Search.EngineTimeout = 10
	m := NewEngineManager(cfg)
	// resultsPerPage=2 makes the early-return target 2*earlyReturnHeadroomFactor;
	// the fast engine supplies well over that so one report fills the page.
	resultsPerPage := 2
	m.engines["fast"] = &mockSearchEngine{
		name:    "fast",
		results: earlyReturnResults(resultsPerPage*earlyReturnHeadroomFactor + 6),
		avail:   true,
		tier:    1,
	}
	m.engines["slow"] = &blockingEngine{name: "slow", delay: 30 * time.Second}

	start := time.Now()
	resp := m.SearchWithOperators(context.Background(), "hello", 1,
		[]string{"fast", "slow"}, nil, nil, nil, false, "", resultsPerPage, ResultFilterOptions{})
	elapsed := time.Since(start)

	if resp == nil {
		t.Fatal("SearchWithOperators: nil response")
	}
	if elapsed > 5*time.Second {
		t.Fatalf("early return did not fire: took %v (deadline path is ~12s)", elapsed)
	}
	if len(resp.Data.Results) != resultsPerPage {
		t.Fatalf("expected %d results after page slice, got %d", resultsPerPage, len(resp.Data.Results))
	}
	if !containsSlice(resp.Data.EnginesUsed, "fast") {
		t.Errorf("fast engine missing from EnginesUsed: %v", resp.Data.EnginesUsed)
	}
	if containsSlice(resp.Data.EnginesFailed, "slow") {
		t.Errorf("skipped engine must not be reported as failed: %v", resp.Data.EnginesFailed)
	}
	if stat, ok := resp.Data.EngineStats["slow"]; !ok || stat.Error != "skipped_page_filled" {
		t.Errorf("slow engine stat should be skipped_page_filled, got %+v", resp.Data.EngineStats["slow"])
	}
}
