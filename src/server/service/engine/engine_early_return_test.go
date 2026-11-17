// SPDX-License-Identifier: MIT
// SearchWithOperators must collect every selected source before returning a
// global result count and sorted page.
package engine

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/apimgr/vidveil/src/config"
	"github.com/apimgr/vidveil/src/server/model"
)

type delayedSearchEngine struct {
	*mockSearchEngine
	delay time.Duration
}

func (e *delayedSearchEngine) Search(ctx context.Context, query string, page int) ([]model.VideoResult, error) {
	time.Sleep(e.delay)
	return e.mockSearchEngine.Search(ctx, query, page)
}

func TestSearchWithOperatorsCollectsEverySelectedEngineForTotal(t *testing.T) {
	cfg := config.DefaultAppConfig()
	cfg.Search.EngineTimeout = 1
	m := NewEngineManager(cfg)
	fastResults := make([]model.VideoResult, 0, 6)
	for i, title := range []string{"alpha wonder", "bravo galaxy", "charlie mountain", "delta ocean", "echo forest", "foxtrot desert"} {
		fastResults = append(fastResults, validResult(fmt.Sprintf("hello %s %d", title, i), fmt.Sprintf("https://example.com/fast/%d", i)))
	}
	slowResults := []model.VideoResult{
		validResult("hello zebra valley", "https://example.com/slow/1"),
		validResult("hello yellow canyon", "https://example.com/slow/2"),
		validResult("hello violet river", "https://example.com/slow/3"),
	}
	m.engines["fast"] = &mockSearchEngine{name: "fast", results: fastResults, avail: true, tier: 1}
	m.engines["slow"] = &delayedSearchEngine{
		mockSearchEngine: &mockSearchEngine{name: "slow", results: slowResults, avail: true, tier: 1},
		delay:            100 * time.Millisecond,
	}

	start := time.Now()
	resp := m.SearchWithOperators(context.Background(), "hello", 1,
		[]string{"fast", "slow"}, nil, nil, nil, false, "", 2, ResultFilterOptions{})
	if elapsed := time.Since(start); elapsed < 100*time.Millisecond {
		t.Fatalf("search returned before the delayed source completed: %v", elapsed)
	}
	if resp == nil {
		t.Fatal("SearchWithOperators returned nil response")
	}
	if got := resp.Pagination.Total; got != len(fastResults)+len(slowResults) {
		t.Fatalf("Pagination.Total = %d, want %d results across both sources", got, len(fastResults)+len(slowResults))
	}
	if len(resp.Data.Results) != 2 {
		t.Fatalf("page result count = %d, want 2", len(resp.Data.Results))
	}
	if got := resp.Data.EngineStats["slow"].ResultCount; got != len(slowResults) {
		t.Fatalf("slow engine result count = %d, want %d", got, len(slowResults))
	}
}
