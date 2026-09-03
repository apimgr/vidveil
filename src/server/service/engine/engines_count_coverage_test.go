package engine

import (
	"testing"

	"github.com/apimgr/vidveil/src/config"
)

// TestEnginesToUseCount verifies the SSE "engines queried" count reflects the
// engine name filter and excludes unavailable engines, independent of whether
// any engine returns results.
func TestEnginesToUseCount(t *testing.T) {
	cfg := config.DefaultAppConfig()
	m := NewEngineManager(cfg)
	m.engines["alpha"] = &mockSearchEngine{name: "alpha", avail: true, tier: 1}
	m.engines["beta"] = &mockSearchEngine{name: "beta", avail: true, tier: 1}
	m.engines["gamma"] = &mockSearchEngine{name: "gamma", avail: false, tier: 1}

	if got := m.EnginesToUseCount(nil); got != 2 {
		t.Errorf("no filter: want 2 available engines, got %d", got)
	}

	if got := m.EnginesToUseCount([]string{"alpha"}); got != 1 {
		t.Errorf("named filter: want 1, got %d", got)
	}

	if got := m.EnginesToUseCount([]string{"gamma"}); got != 0 {
		t.Errorf("unavailable engine must be excluded: want 0, got %d", got)
	}

	if got := m.EnginesToUseCount([]string{"alpha", "beta", "missing"}); got != 2 {
		t.Errorf("mixed named filter: want 2, got %d", got)
	}
}
