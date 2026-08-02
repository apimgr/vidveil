// SPDX-License-Identifier: MIT
// AI.md PART 28: resilience test — a single slow/hung engine must not stall
// the whole fan-out. SearchWithOperators releases its read lock before any
// engine I/O and bounds collection with a batch deadline, so a fast engine's
// results are returned while a hung engine is recorded as a timeout failure.
package engine

import (
	"context"
	"testing"
	"time"

	"github.com/apimgr/vidveil/src/config"
	"github.com/apimgr/vidveil/src/server/model"
)

// blockingEngine deliberately ignores the context and sleeps, simulating an
// engine whose network call hangs well past its configured timeout.
type blockingEngine struct {
	name  string
	delay time.Duration
}

func (b *blockingEngine) Name() string        { return b.name }
func (b *blockingEngine) DisplayName() string { return b.name }
func (b *blockingEngine) Search(_ context.Context, _ string, _ int) ([]model.VideoResult, error) {
	time.Sleep(b.delay)
	return nil, nil
}
func (b *blockingEngine) IsAvailable() bool              { return true }
func (b *blockingEngine) SupportsFeature(_ Feature) bool { return false }
func (b *blockingEngine) Tier() int                      { return 1 }
func (b *blockingEngine) Capabilities() Capabilities     { return Capabilities{} }

func TestSearchWithOperators_SlowEngineDoesNotBlockBatch(t *testing.T) {
	cfg := config.DefaultAppConfig()
	// batchDeadline = longest engineTimeout (1s) + 2s grace = 3s.
	cfg.Search.EngineTimeout = 1
	m := NewEngineManager(cfg)
	m.engines["fast"] = &mockSearchEngine{
		name:    "fast",
		results: []model.VideoResult{validResult("hello world video", "https://example.com/1")},
		avail:   true,
		tier:    1,
	}
	m.engines["slow"] = &blockingEngine{name: "slow", delay: 30 * time.Second}

	start := time.Now()
	resp := m.SearchWithOperators(context.Background(), "hello world", 1,
		[]string{"fast", "slow"}, nil, nil, nil, false, "")
	elapsed := time.Since(start)

	if resp == nil {
		t.Fatal("SearchWithOperators: nil response")
	}
	if elapsed > 10*time.Second {
		t.Fatalf("batch blocked on slow engine: took %v (expected ~3s deadline)", elapsed)
	}
	if len(resp.Data.Results) == 0 {
		t.Fatalf("fast engine results were dropped: %+v", resp.Data)
	}
	if !containsSlice(resp.Data.EnginesUsed, "fast") {
		t.Errorf("fast engine missing from EnginesUsed: %v", resp.Data.EnginesUsed)
	}
	if !containsSlice(resp.Data.EnginesFailed, "slow") {
		t.Errorf("slow engine should be reported in EnginesFailed: %v", resp.Data.EnginesFailed)
	}
}

func containsSlice(s []string, want string) bool {
	for _, v := range s {
		if v == want {
			return true
		}
	}
	return false
}
