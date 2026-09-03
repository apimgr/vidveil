// SPDX-License-Identifier: MIT
// AI.md PART 28: Coverage tests for downloadAndParse and Update (enabled path).
// Uses a local httptest.Server — no real network calls.
package blocklist

import (
	"context"
	"net"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"

	"github.com/apimgr/vidveil/src/config"
)

// ipListText is a simple IP blocklist in plain-text format.
const ipListText = "# comment\n192.0.2.1\n10.0.0.0/8\n\n203.0.113.5\n"

// domainListText is a simple domain blocklist.
const domainListText = "# comment\nexample-blocked.com\nbad-domain.net\n\n"

// newBlocklistServiceForTest creates a BlocklistService with a temp dataDir.
func newBlocklistServiceForTest(t *testing.T) *BlocklistService {
	t.Helper()
	tmp := t.TempDir()
	return &BlocklistService{
		appConfig: config.DefaultAppConfig(),
		dataDir:   tmp,
		ipBlocks:  make(map[string]bool),
		subnets:   make([]*net.IPNet, 0),
		domains:   make(map[string]bool),
		mu:        sync.RWMutex{},
	}
}

// ── downloadAndParse ──────────────────────────────────────────────────────────

func TestDownloadAndParse_IPType_Success(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(ipListText))
	}))
	defer srv.Close()

	s := newBlocklistServiceForTest(t)
	source := config.BlocklistSource{
		Name:    "test-ip",
		URL:     srv.URL,
		Type:    "ip",
		Enabled: true,
	}
	if err := s.downloadAndParse(context.Background(), source); err != nil {
		t.Errorf("downloadAndParse IP: unexpected error: %v", err)
	}
	if !s.IsBlocked("192.0.2.1") {
		t.Error("downloadAndParse IP: expected 192.0.2.1 to be blocked")
	}
}

func TestDownloadAndParse_DomainType_Success(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(domainListText))
	}))
	defer srv.Close()

	s := newBlocklistServiceForTest(t)
	source := config.BlocklistSource{
		Name:    "test-domain",
		URL:     srv.URL,
		Type:    "domain",
		Enabled: true,
	}
	if err := s.downloadAndParse(context.Background(), source); err != nil {
		t.Errorf("downloadAndParse domain: unexpected error: %v", err)
	}
	if !s.IsBlocked("example-blocked.com") {
		t.Error("downloadAndParse domain: expected example-blocked.com to be blocked")
	}
}

func TestDownloadAndParse_ServerError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusForbidden)
	}))
	defer srv.Close()

	s := newBlocklistServiceForTest(t)
	source := config.BlocklistSource{Name: "test", URL: srv.URL, Type: "ip"}
	if err := s.downloadAndParse(context.Background(), source); err == nil {
		t.Error("downloadAndParse on 403: expected error, got nil")
	}
}

func TestDownloadAndParse_InvalidURL(t *testing.T) {
	s := newBlocklistServiceForTest(t)
	source := config.BlocklistSource{Name: "test", URL: "://bad-url", Type: "ip"}
	if err := s.downloadAndParse(context.Background(), source); err == nil {
		t.Error("downloadAndParse invalid URL: expected error, got nil")
	}
}

func TestDownloadAndParse_ConnRefused(t *testing.T) {
	s := newBlocklistServiceForTest(t)
	source := config.BlocklistSource{Name: "test", URL: "http://127.0.0.1:1", Type: "ip"}
	if err := s.downloadAndParse(context.Background(), source); err == nil {
		t.Error("downloadAndParse conn refused: expected error, got nil")
	}
}

// ── Update (enabled, source configured) ──────────────────────────────────────

func TestUpdate_EnabledSingleSource(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(ipListText))
	}))
	defer srv.Close()

	appCfg := config.DefaultAppConfig()
	appCfg.Server.Security.Blocklists.Enabled = true
	appCfg.Server.Security.Blocklists.Sources = []config.BlocklistSource{
		{Name: "test", URL: srv.URL, Type: "ip", Enabled: true},
	}

	s := newBlocklistServiceForTest(t)
	s.appConfig = appCfg

	if err := s.Update(context.Background()); err != nil {
		t.Errorf("Update enabled with source: unexpected error: %v", err)
	}
}

func TestUpdate_EnabledSourceDownloadFails(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusServiceUnavailable)
	}))
	defer srv.Close()

	appCfg := config.DefaultAppConfig()
	appCfg.Server.Security.Blocklists.Enabled = true
	appCfg.Server.Security.Blocklists.Sources = []config.BlocklistSource{
		{Name: "fail-source", URL: srv.URL, Type: "ip", Enabled: true},
	}

	s := newBlocklistServiceForTest(t)
	s.appConfig = appCfg

	err := s.Update(context.Background())
	if err == nil {
		t.Error("Update with failed source: expected error, got nil")
	}
}
