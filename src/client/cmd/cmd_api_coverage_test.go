// SPDX-License-Identifier: MIT
// AI.md PART 28: Coverage tests for API-dependent cmd functions.
// Uses an httptest.Server to simulate the vidveil server API so no real
// network connection is needed. Covers RunSearchCommand, RunEnginesCommand,
// FetchEnginesList, RunBangsCommand, RunProbeCommand, fetchEngineList,
// probeEngineByName, and all their output-format branches.
package cmd

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/apimgr/vidveil/src/client/api"
)

// ── server fixtures ───────────────────────────────────────────────────────────

const apiSearchJSON = `{
  "ok": true,
  "query": "test",
  "results": [
    {
      "title": "Test Video One",
      "url": "https://example.com/v/1",
      "thumbnail": "https://example.com/t/1.jpg",
      "duration": "10:00",
      "views": "1000",
      "engine": "ph"
    },
    {
      "title": "Test Video Two",
      "url": "https://example.com/v/2",
      "thumbnail": "",
      "duration": "",
      "views": "",
      "engine": "xv",
      "description": "A test video",
      "tags": ["test", "video"]
    }
  ],
  "count": 2,
  "search_time": 42
}`

const apiEnginesJSON = `{
  "ok": true,
  "engines": [
    {
      "name": "ph",
      "display_name": "PornHub",
      "bang": "ph",
      "tier": 1,
      "enabled": true,
      "method": "html",
      "has_preview": false,
      "has_download": false
    },
    {
      "name": "xv",
      "display_name": "XVideos",
      "bang": "xv",
      "tier": 1,
      "enabled": false,
      "method": "html",
      "has_preview": true,
      "has_download": true
    }
  ],
  "count": 2
}`

const apiEngineDetailJSON = `{
  "ok": true,
  "data": {
    "display_name": "PornHub",
    "tier": 1,
    "enabled": true,
    "capabilities": {
      "has_preview": false,
      "has_download": false
    }
  }
}`

// newAPITestServer creates an httptest.Server that routes API paths.
func newAPITestServer(t *testing.T) *httptest.Server {
	t.Helper()
	mux := http.NewServeMux()

	// search
	mux.HandleFunc("/api/v1/search", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(apiSearchJSON))
	})

	// engines list
	mux.HandleFunc("/api/v1/engines", func(w http.ResponseWriter, r *http.Request) {
		// engine detail: /api/v1/engines/{name}
		if r.URL.Path != "/api/v1/engines" {
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(apiEngineDetailJSON))
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(apiEnginesJSON))
	})

	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	return srv
}

// newBrokenAPIServer returns a server that always replies 500.
func newBrokenAPIServer(t *testing.T) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "internal error", http.StatusInternalServerError)
	}))
	t.Cleanup(srv.Close)
	return srv
}

// setupAPITestEnv wires a fresh APIClient and CLIConfig pointing at srv.
// Returns a cleanup function that restores the originals.
func setupAPITestEnv(t *testing.T, srv *httptest.Server, format string) {
	t.Helper()

	origClient := apiClient
	origConfig := cliConfig

	t.Cleanup(func() {
		apiClient = origClient
		cliConfig = origConfig
	})

	apiClient = api.NewAPIClient(srv.URL, "", 30, "v1")

	cliConfig = &CLIConfig{}
	cliConfig.Output.Format = format
	cliConfig.Output.Verbose = false
}

// ── RunSearchCommand ──────────────────────────────────────────────────────────

func TestRunSearchCommand_NoArgs_ReturnsError(t *testing.T) {
	srv := newAPITestServer(t)
	setupAPITestEnv(t, srv, "table")
	if err := RunSearchCommand([]string{}); err == nil {
		t.Error("RunSearchCommand no args: expected error")
	}
}

func TestRunSearchCommand_Help_NoPanic(t *testing.T) {
	srv := newAPITestServer(t)
	setupAPITestEnv(t, srv, "table")
	if err := RunSearchCommand([]string{"--help"}); err != nil {
		t.Errorf("RunSearchCommand --help: %v", err)
	}
}

func TestRunSearchCommand_TableFormat_NoPanic(t *testing.T) {
	srv := newAPITestServer(t)
	setupAPITestEnv(t, srv, "table")
	if err := RunSearchCommand([]string{"test"}); err != nil {
		t.Errorf("RunSearchCommand table: %v", err)
	}
}

func TestRunSearchCommand_JSONFormat_NoPanic(t *testing.T) {
	srv := newAPITestServer(t)
	setupAPITestEnv(t, srv, "json")
	if err := RunSearchCommand([]string{"test"}); err != nil {
		t.Errorf("RunSearchCommand json: %v", err)
	}
}

func TestRunSearchCommand_YAMLFormat_NoPanic(t *testing.T) {
	srv := newAPITestServer(t)
	setupAPITestEnv(t, srv, "yaml")
	if err := RunSearchCommand([]string{"test"}); err != nil {
		t.Errorf("RunSearchCommand yaml: %v", err)
	}
}

func TestRunSearchCommand_CSVFormat_NoPanic(t *testing.T) {
	srv := newAPITestServer(t)
	setupAPITestEnv(t, srv, "csv")
	if err := RunSearchCommand([]string{"test"}); err != nil {
		t.Errorf("RunSearchCommand csv: %v", err)
	}
}

func TestRunSearchCommand_PlainFormat_NoPanic(t *testing.T) {
	srv := newAPITestServer(t)
	setupAPITestEnv(t, srv, "plain")
	if err := RunSearchCommand([]string{"test"}); err != nil {
		t.Errorf("RunSearchCommand plain: %v", err)
	}
}

func TestRunSearchCommand_WithLimitAndPage_NoPanic(t *testing.T) {
	srv := newAPITestServer(t)
	setupAPITestEnv(t, srv, "table")
	if err := RunSearchCommand([]string{"--limit", "10", "--page", "2", "test"}); err != nil {
		t.Errorf("RunSearchCommand with limit/page: %v", err)
	}
}

func TestRunSearchCommand_WithEngines_NoPanic(t *testing.T) {
	srv := newAPITestServer(t)
	setupAPITestEnv(t, srv, "table")
	if err := RunSearchCommand([]string{"--engines", "ph,xv", "test"}); err != nil {
		t.Errorf("RunSearchCommand with engines: %v", err)
	}
}

func TestRunSearchCommand_MultiWordQuery_NoPanic(t *testing.T) {
	srv := newAPITestServer(t)
	setupAPITestEnv(t, srv, "plain")
	if err := RunSearchCommand([]string{"test", "video", "query"}); err != nil {
		t.Errorf("RunSearchCommand multi-word: %v", err)
	}
}

func TestRunSearchCommand_ServerError_ReturnsError(t *testing.T) {
	srv := newBrokenAPIServer(t)
	setupAPITestEnv(t, srv, "table")
	if err := RunSearchCommand([]string{"test"}); err == nil {
		t.Error("RunSearchCommand broken server: expected error")
	}
}

func TestRunSearchCommand_OkFalse_ReturnsError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"ok":false,"error":"rate limited","query":"test","results":[],"count":0}`))
	}))
	defer srv.Close()

	origClient := apiClient
	origConfig := cliConfig
	t.Cleanup(func() { apiClient = origClient; cliConfig = origConfig })
	apiClient = api.NewAPIClient(srv.URL, "", 30, "v1")
	cliConfig = &CLIConfig{}
	cliConfig.Output.Format = "table"

	err := RunSearchCommand([]string{"test"})
	if err == nil {
		t.Error("RunSearchCommand ok=false: expected error")
	}
}

// ── FetchEnginesList ──────────────────────────────────────────────────────────

func TestFetchEnginesList_Success_ReturnsEngines(t *testing.T) {
	srv := newAPITestServer(t)
	setupAPITestEnv(t, srv, "table")
	resp, err := FetchEnginesList()
	if err != nil {
		t.Fatalf("FetchEnginesList: %v", err)
	}
	if len(resp.Engines) == 0 {
		t.Error("FetchEnginesList: expected engines in response")
	}
}

func TestFetchEnginesList_BrokenServer_ReturnsError(t *testing.T) {
	srv := newBrokenAPIServer(t)
	setupAPITestEnv(t, srv, "table")
	if _, err := FetchEnginesList(); err == nil {
		t.Error("FetchEnginesList broken server: expected error")
	}
}

func TestFetchEnginesList_BadJSON_ReturnsError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`not json`))
	}))
	defer srv.Close()

	origClient := apiClient
	origConfig := cliConfig
	t.Cleanup(func() { apiClient = origClient; cliConfig = origConfig })
	apiClient = api.NewAPIClient(srv.URL, "", 30, "v1")
	cliConfig = &CLIConfig{}

	if _, err := FetchEnginesList(); err == nil {
		t.Error("FetchEnginesList bad JSON: expected error")
	}
}

func TestFetchEnginesList_OkFalse_ReturnsError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"ok":false,"error":"not allowed","engines":[],"count":0}`))
	}))
	defer srv.Close()

	origClient := apiClient
	origConfig := cliConfig
	t.Cleanup(func() { apiClient = origClient; cliConfig = origConfig })
	apiClient = api.NewAPIClient(srv.URL, "", 30, "v1")
	cliConfig = &CLIConfig{}

	if _, err := FetchEnginesList(); err == nil {
		t.Error("FetchEnginesList ok=false: expected error")
	}
}

// ── RunEnginesCommand ─────────────────────────────────────────────────────────

func TestRunEnginesCommand_Help_NoPanic(t *testing.T) {
	srv := newAPITestServer(t)
	setupAPITestEnv(t, srv, "table")
	if err := RunEnginesCommand([]string{"--help"}); err != nil {
		t.Errorf("RunEnginesCommand --help: %v", err)
	}
}

func TestRunEnginesCommand_TableFormat_NoPanic(t *testing.T) {
	srv := newAPITestServer(t)
	setupAPITestEnv(t, srv, "table")
	if err := RunEnginesCommand([]string{}); err != nil {
		t.Errorf("RunEnginesCommand table: %v", err)
	}
}

func TestRunEnginesCommand_JSONFormat_NoPanic(t *testing.T) {
	srv := newAPITestServer(t)
	setupAPITestEnv(t, srv, "json")
	if err := RunEnginesCommand([]string{}); err != nil {
		t.Errorf("RunEnginesCommand json: %v", err)
	}
}

func TestRunEnginesCommand_YAMLFormat_NoPanic(t *testing.T) {
	srv := newAPITestServer(t)
	setupAPITestEnv(t, srv, "yaml")
	if err := RunEnginesCommand([]string{}); err != nil {
		t.Errorf("RunEnginesCommand yaml: %v", err)
	}
}

func TestRunEnginesCommand_CSVFormat_NoPanic(t *testing.T) {
	srv := newAPITestServer(t)
	setupAPITestEnv(t, srv, "csv")
	if err := RunEnginesCommand([]string{}); err != nil {
		t.Errorf("RunEnginesCommand csv: %v", err)
	}
}

func TestRunEnginesCommand_PlainFormat_NoPanic(t *testing.T) {
	srv := newAPITestServer(t)
	setupAPITestEnv(t, srv, "plain")
	if err := RunEnginesCommand([]string{}); err != nil {
		t.Errorf("RunEnginesCommand plain: %v", err)
	}
}

func TestRunEnginesCommand_AllDetails_NoPanic(t *testing.T) {
	srv := newAPITestServer(t)
	setupAPITestEnv(t, srv, "table")
	if err := RunEnginesCommand([]string{"--all"}); err != nil {
		t.Errorf("RunEnginesCommand --all: %v", err)
	}
}

func TestRunEnginesCommand_EnabledOnly_NoPanic(t *testing.T) {
	srv := newAPITestServer(t)
	setupAPITestEnv(t, srv, "table")
	if err := RunEnginesCommand([]string{"--enabled"}); err != nil {
		t.Errorf("RunEnginesCommand --enabled: %v", err)
	}
}

func TestRunEnginesCommand_DisabledOnly_NoPanic(t *testing.T) {
	srv := newAPITestServer(t)
	setupAPITestEnv(t, srv, "table")
	if err := RunEnginesCommand([]string{"--disabled"}); err != nil {
		t.Errorf("RunEnginesCommand --disabled: %v", err)
	}
}

func TestRunEnginesCommand_BrokenServer_ReturnsError(t *testing.T) {
	srv := newBrokenAPIServer(t)
	setupAPITestEnv(t, srv, "table")
	if err := RunEnginesCommand([]string{}); err == nil {
		t.Error("RunEnginesCommand broken server: expected error")
	}
}

// ── RunBangsCommand ───────────────────────────────────────────────────────────

func TestRunBangsCommand_Help_NoPanic(t *testing.T) {
	srv := newAPITestServer(t)
	setupAPITestEnv(t, srv, "table")
	if err := RunBangsCommand([]string{"--help"}); err != nil {
		t.Errorf("RunBangsCommand --help: %v", err)
	}
}

func TestRunBangsCommand_TableFormat_NoPanic(t *testing.T) {
	srv := newAPITestServer(t)
	setupAPITestEnv(t, srv, "table")
	if err := RunBangsCommand([]string{}); err != nil {
		t.Errorf("RunBangsCommand table: %v", err)
	}
}

func TestRunBangsCommand_JSONFormat_NoPanic(t *testing.T) {
	srv := newAPITestServer(t)
	setupAPITestEnv(t, srv, "json")
	if err := RunBangsCommand([]string{}); err != nil {
		t.Errorf("RunBangsCommand json: %v", err)
	}
}

func TestRunBangsCommand_YAMLFormat_NoPanic(t *testing.T) {
	srv := newAPITestServer(t)
	setupAPITestEnv(t, srv, "yaml")
	if err := RunBangsCommand([]string{}); err != nil {
		t.Errorf("RunBangsCommand yaml: %v", err)
	}
}

func TestRunBangsCommand_CSVFormat_NoPanic(t *testing.T) {
	srv := newAPITestServer(t)
	setupAPITestEnv(t, srv, "csv")
	if err := RunBangsCommand([]string{}); err != nil {
		t.Errorf("RunBangsCommand csv: %v", err)
	}
}

func TestRunBangsCommand_PlainFormat_NoPanic(t *testing.T) {
	srv := newAPITestServer(t)
	setupAPITestEnv(t, srv, "plain")
	if err := RunBangsCommand([]string{}); err != nil {
		t.Errorf("RunBangsCommand plain: %v", err)
	}
}

func TestRunBangsCommand_WithSearchFilter_NoPanic(t *testing.T) {
	srv := newAPITestServer(t)
	setupAPITestEnv(t, srv, "table")
	if err := RunBangsCommand([]string{"--search", "ph"}); err != nil {
		t.Errorf("RunBangsCommand --search: %v", err)
	}
}

func TestRunBangsCommand_SearchFilterNoMatch_Empty(t *testing.T) {
	srv := newAPITestServer(t)
	setupAPITestEnv(t, srv, "table")
	// "zzz" won't match any bang
	if err := RunBangsCommand([]string{"--search", "zzz"}); err != nil {
		t.Errorf("RunBangsCommand no-match search: %v", err)
	}
}

func TestRunBangsCommand_BrokenServer_ReturnsError(t *testing.T) {
	srv := newBrokenAPIServer(t)
	setupAPITestEnv(t, srv, "table")
	if err := RunBangsCommand([]string{}); err == nil {
		t.Error("RunBangsCommand broken server: expected error")
	}
}

// ── RunProbeCommand ───────────────────────────────────────────────────────────

func TestRunProbeCommand_Help_NoPanic(t *testing.T) {
	srv := newAPITestServer(t)
	setupAPITestEnv(t, srv, "table")
	if err := RunProbeCommand([]string{"--help"}); err != nil {
		t.Errorf("RunProbeCommand --help: %v", err)
	}
}

func TestRunProbeCommand_NoArgs_ReturnsError(t *testing.T) {
	srv := newAPITestServer(t)
	setupAPITestEnv(t, srv, "table")
	if err := RunProbeCommand([]string{}); err == nil {
		t.Error("RunProbeCommand no args: expected error")
	}
}

func TestRunProbeCommand_SpecificEngine_Table_NoPanic(t *testing.T) {
	srv := newAPITestServer(t)
	setupAPITestEnv(t, srv, "table")
	if err := RunProbeCommand([]string{"--engines", "ph"}); err != nil {
		t.Errorf("RunProbeCommand --engines ph table: %v", err)
	}
}

func TestRunProbeCommand_SpecificEngine_JSON_NoPanic(t *testing.T) {
	srv := newAPITestServer(t)
	setupAPITestEnv(t, srv, "json")
	if err := RunProbeCommand([]string{"--engines", "ph"}); err != nil {
		t.Errorf("RunProbeCommand --engines ph json: %v", err)
	}
}

func TestRunProbeCommand_SpecificEngine_YAML_NoPanic(t *testing.T) {
	srv := newAPITestServer(t)
	setupAPITestEnv(t, srv, "yaml")
	if err := RunProbeCommand([]string{"--engines", "ph"}); err != nil {
		t.Errorf("RunProbeCommand --engines ph yaml: %v", err)
	}
}

func TestRunProbeCommand_SpecificEngine_CSV_NoPanic(t *testing.T) {
	srv := newAPITestServer(t)
	setupAPITestEnv(t, srv, "csv")
	if err := RunProbeCommand([]string{"--engines", "ph"}); err != nil {
		t.Errorf("RunProbeCommand --engines ph csv: %v", err)
	}
}

func TestRunProbeCommand_SpecificEngine_Verbose_NoPanic(t *testing.T) {
	srv := newAPITestServer(t)
	setupAPITestEnv(t, srv, "table")
	if err := RunProbeCommand([]string{"--engines", "ph", "--verbose"}); err != nil {
		t.Errorf("RunProbeCommand --verbose: %v", err)
	}
}

func TestRunProbeCommand_SpecificEngine_CustomQuery_NoPanic(t *testing.T) {
	srv := newAPITestServer(t)
	setupAPITestEnv(t, srv, "table")
	if err := RunProbeCommand([]string{"--engines", "ph", "--query", "amateur"}); err != nil {
		t.Errorf("RunProbeCommand --query: %v", err)
	}
}

func TestRunProbeCommand_MultipleEngines_NoPanic(t *testing.T) {
	srv := newAPITestServer(t)
	setupAPITestEnv(t, srv, "table")
	if err := RunProbeCommand([]string{"--engines", "ph,xv"}); err != nil {
		t.Errorf("RunProbeCommand multiple engines: %v", err)
	}
}

func TestRunProbeCommand_AllEngines_NoPanic(t *testing.T) {
	srv := newAPITestServer(t)
	setupAPITestEnv(t, srv, "table")
	if err := RunProbeCommand([]string{"--all"}); err != nil {
		t.Errorf("RunProbeCommand --all: %v", err)
	}
}

func TestRunProbeCommand_AllEngines_Verbose_NoPanic(t *testing.T) {
	srv := newAPITestServer(t)
	setupAPITestEnv(t, srv, "table")
	if err := RunProbeCommand([]string{"--all", "--verbose"}); err != nil {
		t.Errorf("RunProbeCommand --all --verbose: %v", err)
	}
}

func TestRunProbeCommand_AllEngines_BrokenServer_ReturnsError(t *testing.T) {
	srv := newBrokenAPIServer(t)
	setupAPITestEnv(t, srv, "table")
	if err := RunProbeCommand([]string{"--all"}); err == nil {
		t.Error("RunProbeCommand broken server --all: expected error")
	}
}

func TestRunProbeCommand_EngineSearchError_StillOutputs(t *testing.T) {
	// Server returns success on /engines list but error on /search
	callCount := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.Contains(r.URL.Path, "/search") {
			callCount++
			http.Error(w, "search unavailable", http.StatusServiceUnavailable)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(apiEnginesJSON))
	}))
	defer srv.Close()

	origClient := apiClient
	origConfig := cliConfig
	t.Cleanup(func() { apiClient = origClient; cliConfig = origConfig })
	apiClient = api.NewAPIClient(srv.URL, "", 30, "v1")
	cliConfig = &CLIConfig{}
	cliConfig.Output.Format = "json"

	// probeEngineByName records the error but doesn't fail RunProbeCommand
	if err := RunProbeCommand([]string{"--engines", "ph"}); err != nil {
		t.Errorf("RunProbeCommand engine search error: unexpected error %v", err)
	}
}

// ── helper output functions (not yet covered elsewhere) ───────────────────────

func TestOutputEnginesAsCSV_NoPanic(t *testing.T) {
	engines := []EngineInfo{{Name: "ph", DisplayName: "PornHub", Bang: "ph", Tier: 1, Enabled: false, HasPreview: true, HasDownload: true}}
	if err := OutputEnginesAsCSV(engines); err != nil {
		t.Errorf("OutputEnginesAsCSV: %v", err)
	}
}

func TestOutputEnginesAsTable_NoDetails_NoPanic(t *testing.T) {
	engines := []EngineInfo{
		{Name: "ph", DisplayName: "PornHub", Bang: "ph", Tier: 1, Enabled: true},
	}
	if err := OutputEnginesAsTable(engines, false); err != nil {
		t.Errorf("OutputEnginesAsTable no-details: %v", err)
	}
}

func TestOutputEnginesAsTable_WithDetails_NoPanic(t *testing.T) {
	engines := []EngineInfo{
		{Name: "ph", DisplayName: "PornHub", Bang: "ph", Tier: 1, Enabled: true, Method: "html", HasPreview: true, HasDownload: false},
		{Name: "xv", DisplayName: "XVideos", Bang: "xv", Tier: 1, Enabled: false, Method: "html"},
	}
	if err := OutputEnginesAsTable(engines, true); err != nil {
		t.Errorf("OutputEnginesAsTable with-details: %v", err)
	}
}

func TestOutputSearchResultsAsYAML_NoPanic(t *testing.T) {
	resp := searchResponseFixture()
	if err := OutputSearchResultsAsYAML(resp); err != nil {
		t.Errorf("OutputSearchResultsAsYAML: %v", err)
	}
}

func TestOutputSearchResultsAsCSV_NoPanic(t *testing.T) {
	resp := searchResponseFixture()
	if err := OutputSearchResultsAsCSV(resp); err != nil {
		t.Errorf("OutputSearchResultsAsCSV: %v", err)
	}
}

// searchResponseFixture builds a minimal SearchResponse for output tests.
func searchResponseFixture() *api.SearchResponse {
	return &api.SearchResponse{
		Ok:    true,
		Query: "test",
		Count: 2,
		Results: []api.SearchResult{
			{Title: "Alpha", URL: "https://a.com/1", Duration: "5:00", Views: "500", Engine: "ph"},
			{Title: "Beta", URL: "https://b.com/2", Duration: "", Views: "", Engine: "xv", Description: "desc", Tags: []string{"a", "b"}},
		},
	}
}
