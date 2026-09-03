// SPDX-License-Identifier: MIT
// AI.md PART 28: Coverage tests for downloadCVEFeed and Update (enabled path).
// Uses a local httptest.Server as the CVE feed source — no real network calls.
package cve

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"sync"
	"testing"

	"github.com/apimgr/vidveil/src/config"
)

// minimalNVDJSON is valid NVD feed JSON with zero CVE items.
const minimalNVDJSON = `{"CVE_Items":[]}`

// minimalNVDJSONWithItem has one CVE for coverage of the parse loop.
const minimalNVDJSONWithItem = `{
  "CVE_Items": [{
    "cve": {
      "CVE_data_meta": {"ID": "CVE-2024-0001"},
      "description": {"description_data": [{"value": "test vuln"}]}
    },
    "publishedDate": "2024-01-01T00:00Z",
    "lastModifiedDate": "2024-01-01T00:00Z",
    "impact": {"baseMetricV3": {"cvssV3": {"baseScore": 7.5}}},
    "configurations": {"nodes": [{"cpe_match": [{"cpe23Uri": "cpe:2.3:a:example:app:*:*:*:*:*:*:*:*"}]}]}
  }]
}`

// newCVEServiceForTest constructs a CVEService with a temp dataDir and the
// given AppConfig (no filesystem side effects outside tempDir).
func newCVEServiceForTest(t *testing.T, appCfg *config.AppConfig) *CVEService {
	t.Helper()
	tmp := t.TempDir()
	return &CVEService{
		appConfig: appCfg,
		dataDir:   tmp,
		cveData:   make(map[string]CVEItem),
		mu:        sync.RWMutex{},
	}
}

// ── downloadCVEFeed ───────────────────────────────────────────────────────────

func TestDownloadCVEFeed_EmptyItems(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(minimalNVDJSON))
	}))
	defer srv.Close()

	appCfg := config.DefaultAppConfig()
	s := newCVEServiceForTest(t, appCfg)
	if err := s.downloadCVEFeed(context.Background(), srv.URL); err != nil {
		t.Errorf("downloadCVEFeed(empty items): unexpected error: %v", err)
	}
}

func TestDownloadCVEFeed_WithCVEItem(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(minimalNVDJSONWithItem))
	}))
	defer srv.Close()

	appCfg := config.DefaultAppConfig()
	s := newCVEServiceForTest(t, appCfg)
	if err := s.downloadCVEFeed(context.Background(), srv.URL); err != nil {
		t.Errorf("downloadCVEFeed(with item): unexpected error: %v", err)
	}
	if len(s.cveData) == 0 {
		t.Error("downloadCVEFeed: expected cveData to be populated")
	}
}

func TestDownloadCVEFeed_ServerError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()

	appCfg := config.DefaultAppConfig()
	s := newCVEServiceForTest(t, appCfg)
	if err := s.downloadCVEFeed(context.Background(), srv.URL); err == nil {
		t.Error("downloadCVEFeed on 500: expected error, got nil")
	}
}

func TestDownloadCVEFeed_BadJSON(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("not-json"))
	}))
	defer srv.Close()

	appCfg := config.DefaultAppConfig()
	s := newCVEServiceForTest(t, appCfg)
	if err := s.downloadCVEFeed(context.Background(), srv.URL); err == nil {
		t.Error("downloadCVEFeed bad JSON: expected error, got nil")
	}
}

func TestDownloadCVEFeed_InvalidURL(t *testing.T) {
	appCfg := config.DefaultAppConfig()
	s := newCVEServiceForTest(t, appCfg)
	err := s.downloadCVEFeed(context.Background(), "://invalid-url")
	if err == nil {
		t.Error("downloadCVEFeed invalid URL: expected error, got nil")
	}
}

// ── Update (enabled path, custom source) ─────────────────────────────────────

func TestUpdate_Enabled_Success(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(minimalNVDJSON))
	}))
	defer srv.Close()

	appCfg := config.DefaultAppConfig()
	appCfg.Server.Security.CVE.Enabled = true
	appCfg.Server.Security.CVE.Source = srv.URL

	s := newCVEServiceForTest(t, appCfg)
	if err := s.Update(context.Background()); err != nil {
		t.Errorf("Update(enabled, mock source): unexpected error: %v", err)
	}
}

func TestUpdate_Enabled_DefaultSourceOverride(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusForbidden)
	}))
	defer srv.Close()

	appCfg := config.DefaultAppConfig()
	appCfg.Server.Security.CVE.Enabled = true
	// Empty source → falls through to the hardcoded NVD URL which will fail
	// because the NVD URL is redirected to our 403 server via… wait, it won't be.
	// Instead, test with a source pointing to the mock that returns 403.
	appCfg.Server.Security.CVE.Source = srv.URL

	s := newCVEServiceForTest(t, appCfg)
	err := s.Update(context.Background())
	if err == nil {
		t.Error("Update on 403: expected error, got nil")
	}
}

// Verify URL with empty host produces error from downloadCVEFeed, not a panic.
func TestDownloadCVEFeed_ConnRefused(t *testing.T) {
	// Use a port unlikely to be listening.
	parsed, _ := url.Parse("http://127.0.0.1:1")
	appCfg := config.DefaultAppConfig()
	s := newCVEServiceForTest(t, appCfg)
	err := s.downloadCVEFeed(context.Background(), parsed.String())
	if err == nil {
		t.Error("downloadCVEFeed conn refused: expected error, got nil")
	}
}
