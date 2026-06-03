// SPDX-License-Identifier: MIT
package blocklist

import (
	"context"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/apimgr/vidveil/src/config"
)

// newTestService builds a BlocklistService without touching the filesystem or
// config.GetAppPaths, using the caller-supplied temp directory as dataDir.
func newTestService(t *testing.T) *BlocklistService {
	t.Helper()
	return &BlocklistService{
		ipBlocks: make(map[string]bool),
		subnets:  make([]*net.IPNet, 0),
		domains:  make(map[string]bool),
		dataDir:  t.TempDir(),
	}
}

// TestNewBlocklistService_Smoke verifies that NewBlocklistService returns a
// fully initialised struct when given a valid AppConfig.
func TestNewBlocklistService_Smoke(t *testing.T) {
	cfg := config.DefaultAppConfig()
	svc := NewBlocklistService(cfg)

	if svc == nil {
		t.Fatal("NewBlocklistService returned nil")
	}
	if svc.ipBlocks == nil {
		t.Error("ipBlocks map is nil")
	}
	if svc.domains == nil {
		t.Error("domains map is nil")
	}
	if svc.subnets == nil {
		t.Error("subnets slice is nil")
	}
}

// TestInitialize_CreatesDirectory verifies that Initialize creates the dataDir
// when it does not yet exist.
func TestInitialize_CreatesDirectory(t *testing.T) {
	svc := newTestService(t)
	svc.dataDir = filepath.Join(t.TempDir(), "nested", "blocklists")

	if err := svc.Initialize(); err != nil {
		t.Fatalf("Initialize() returned unexpected error: %v", err)
	}
	if _, err := os.Stat(svc.dataDir); os.IsNotExist(err) {
		t.Error("Initialize() did not create dataDir")
	}
}

// TestInitialize_Idempotent verifies that calling Initialize twice succeeds.
func TestInitialize_Idempotent(t *testing.T) {
	svc := newTestService(t)

	if err := svc.Initialize(); err != nil {
		t.Fatalf("first Initialize() error: %v", err)
	}
	if err := svc.Initialize(); err != nil {
		t.Fatalf("second Initialize() error: %v", err)
	}
}

// TestParseIPLine covers the ip-address and CIDR parsing paths, including
// comment stripping and invalid input handling.
func TestParseIPLine(t *testing.T) {
	tests := []struct {
		name          string
		line          string
		wantIP        string
		wantSubnet    string
		wantIPCount   int
		wantNetCount  int
	}{
		{
			name:        "plain IPv4",
			line:        "192.168.1.1",
			wantIP:      "192.168.1.1",
			wantIPCount: 1,
		},
		{
			name:        "CIDR block",
			line:        "10.0.0.0/8",
			wantNetCount: 1,
		},
		{
			name:        "IPv4 with comment",
			line:        "192.168.1.1 # comment",
			wantIP:      "192.168.1.1",
			wantIPCount: 1,
		},
		{
			name:        "CIDR with comment",
			line:        "10.0.0.0/8 # comment",
			wantNetCount: 1,
		},
		{
			name: "invalid plain token",
			line: "invalid",
		},
		{
			name: "invalid CIDR",
			line: "not/a/cidr",
		},
		{
			name: "empty string",
			line: "",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			svc := newTestService(t)
			svc.parseIPLine(tc.line)

			if got := len(svc.ipBlocks); got != tc.wantIPCount {
				t.Errorf("ipBlocks count: got %d, want %d", got, tc.wantIPCount)
			}
			if tc.wantIP != "" && !svc.ipBlocks[tc.wantIP] {
				t.Errorf("ipBlocks missing %q", tc.wantIP)
			}
			if got := len(svc.subnets); got != tc.wantNetCount {
				t.Errorf("subnets count: got %d, want %d", got, tc.wantNetCount)
			}
		})
	}
}

// TestParseDomainLine covers domain normalisation: comment stripping, protocol
// removal, path removal, and case-folding.
func TestParseDomainLine(t *testing.T) {
	tests := []struct {
		name       string
		line       string
		wantDomain string
		wantCount  int
	}{
		{
			name:       "plain domain",
			line:       "example.com",
			wantDomain: "example.com",
			wantCount:  1,
		},
		{
			name:       "uppercase domain",
			line:       "EXAMPLE.COM",
			wantDomain: "example.com",
			wantCount:  1,
		},
		{
			name:       "http URL",
			line:       "http://example.com",
			wantDomain: "example.com",
			wantCount:  1,
		},
		{
			name:       "https URL with path",
			line:       "https://example.com/path/to/something",
			wantDomain: "example.com",
			wantCount:  1,
		},
		{
			name:       "domain with inline comment",
			line:       "example.com # comment",
			wantDomain: "example.com",
			wantCount:  1,
		},
		{
			name:      "empty string",
			line:      "",
			wantCount: 0,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			svc := newTestService(t)
			svc.parseDomainLine(tc.line)

			if got := len(svc.domains); got != tc.wantCount {
				t.Errorf("domains count: got %d, want %d", got, tc.wantCount)
			}
			if tc.wantDomain != "" && !svc.domains[tc.wantDomain] {
				t.Errorf("domains missing %q", tc.wantDomain)
			}
		})
	}
}

// TestIsBlocked_IP covers exact-IP match, CIDR containment, and non-blocked
// addresses.
func TestIsBlocked_IP(t *testing.T) {
	t.Run("exact IP blocked", func(t *testing.T) {
		svc := newTestService(t)
		svc.ipBlocks["1.2.3.4"] = true

		if !svc.IsBlocked("1.2.3.4") {
			t.Error("expected 1.2.3.4 to be blocked")
		}
	})

	t.Run("unlisted IP not blocked", func(t *testing.T) {
		svc := newTestService(t)
		svc.ipBlocks["1.2.3.4"] = true

		if svc.IsBlocked("1.2.3.5") {
			t.Error("expected 1.2.3.5 to not be blocked")
		}
	})

	t.Run("IP inside CIDR blocked", func(t *testing.T) {
		svc := newTestService(t)
		_, ipNet, _ := net.ParseCIDR("10.0.0.0/8")
		svc.subnets = append(svc.subnets, ipNet)

		if !svc.IsBlocked("10.1.2.3") {
			t.Error("expected 10.1.2.3 to be blocked by 10.0.0.0/8")
		}
	})

	t.Run("IP outside CIDR not blocked", func(t *testing.T) {
		svc := newTestService(t)
		_, ipNet, _ := net.ParseCIDR("10.0.0.0/8")
		svc.subnets = append(svc.subnets, ipNet)

		if svc.IsBlocked("192.168.1.1") {
			t.Error("expected 192.168.1.1 to not be blocked")
		}
	})

	t.Run("non-IP non-domain token returns false", func(t *testing.T) {
		svc := newTestService(t)

		if svc.IsBlocked("not-an-ip-or-domain") {
			t.Error("expected non-IP junk token to not be blocked")
		}
	})
}

// TestIsBlocked_Domain covers exact domain match, case folding, parent-domain
// (subdomain) matching, and unblocked domains.
func TestIsBlocked_Domain(t *testing.T) {
	t.Run("exact domain blocked", func(t *testing.T) {
		svc := newTestService(t)
		svc.domains["example.com"] = true

		if !svc.IsBlocked("example.com") {
			t.Error("expected example.com to be blocked")
		}
	})

	t.Run("case-insensitive domain check", func(t *testing.T) {
		svc := newTestService(t)
		svc.domains["example.com"] = true

		if !svc.IsBlocked("EXAMPLE.COM") {
			t.Error("expected EXAMPLE.COM to be blocked (case-insensitive)")
		}
	})

	t.Run("subdomain blocked via parent", func(t *testing.T) {
		svc := newTestService(t)
		svc.domains["example.com"] = true

		if !svc.IsBlocked("www.example.com") {
			t.Error("expected www.example.com to be blocked when example.com is blocked")
		}
	})

	t.Run("deeper subdomain blocked via parent", func(t *testing.T) {
		svc := newTestService(t)
		svc.domains["example.com"] = true

		if !svc.IsBlocked("sub.example.com") {
			t.Error("expected sub.example.com to be blocked when example.com is blocked")
		}
	})

	t.Run("unrelated domain not blocked", func(t *testing.T) {
		svc := newTestService(t)
		svc.domains["example.com"] = true

		if svc.IsBlocked("other.com") {
			t.Error("expected other.com to not be blocked")
		}
	})
}

// TestGetStats verifies that GetStats returns accurate counts and the correct
// dataDir value.
func TestGetStats(t *testing.T) {
	t.Run("empty service", func(t *testing.T) {
		svc := newTestService(t)
		stats := svc.GetStats()

		if stats["ip_count"] != 0 {
			t.Errorf("ip_count: got %v, want 0", stats["ip_count"])
		}
		if stats["subnet_count"] != 0 {
			t.Errorf("subnet_count: got %v, want 0", stats["subnet_count"])
		}
		if stats["domain_count"] != 0 {
			t.Errorf("domain_count: got %v, want 0", stats["domain_count"])
		}
		if stats["data_dir"] != svc.dataDir {
			t.Errorf("data_dir: got %v, want %s", stats["data_dir"], svc.dataDir)
		}
	})

	t.Run("populated service", func(t *testing.T) {
		svc := newTestService(t)
		svc.ipBlocks["1.1.1.1"] = true
		svc.ipBlocks["2.2.2.2"] = true
		_, ipNet, _ := net.ParseCIDR("10.0.0.0/8")
		svc.subnets = append(svc.subnets, ipNet)
		svc.domains["a.com"] = true
		svc.domains["b.com"] = true
		svc.domains["c.com"] = true

		stats := svc.GetStats()

		if stats["ip_count"] != 2 {
			t.Errorf("ip_count: got %v, want 2", stats["ip_count"])
		}
		if stats["subnet_count"] != 1 {
			t.Errorf("subnet_count: got %v, want 1", stats["subnet_count"])
		}
		if stats["domain_count"] != 3 {
			t.Errorf("domain_count: got %v, want 3", stats["domain_count"])
		}
	})
}

// TestLastUpdate covers three scenarios: missing timestamp file returns zero
// time, valid RFC3339 file returns the parsed time, and malformed content
// returns zero time.
func TestLastUpdate(t *testing.T) {
	t.Run("no timestamp file returns zero time", func(t *testing.T) {
		svc := newTestService(t)

		got := svc.LastUpdate()
		if !got.IsZero() {
			t.Errorf("expected zero time, got %v", got)
		}
	})

	t.Run("valid RFC3339 timestamp file", func(t *testing.T) {
		svc := newTestService(t)
		want := time.Date(2025, 6, 15, 12, 0, 0, 0, time.UTC)
		tsFile := filepath.Join(svc.dataDir, ".last_updated")
		if err := os.WriteFile(tsFile, []byte(want.Format(time.RFC3339)), 0644); err != nil {
			t.Fatalf("failed to write timestamp: %v", err)
		}

		got := svc.LastUpdate()
		if !got.Equal(want) {
			t.Errorf("LastUpdate: got %v, want %v", got, want)
		}
	})

	t.Run("invalid content in timestamp file returns zero time", func(t *testing.T) {
		svc := newTestService(t)
		tsFile := filepath.Join(svc.dataDir, ".last_updated")
		if err := os.WriteFile(tsFile, []byte("not-a-timestamp"), 0644); err != nil {
			t.Fatalf("failed to write timestamp: %v", err)
		}

		got := svc.LastUpdate()
		if !got.IsZero() {
			t.Errorf("expected zero time for invalid content, got %v", got)
		}
	})
}

// TestLoadBlocklist covers the full file-parsing pipeline: IP and domain files,
// nonexistent file error, comment/blank-line skipping, and mixed content.
func TestLoadBlocklist(t *testing.T) {
	t.Run("load IP file", func(t *testing.T) {
		svc := newTestService(t)
		content := "1.2.3.4\n10.0.0.0/8\n5.6.7.8\n"
		f := writeTempFile(t, content)

		if err := svc.loadBlocklist(f, "ip"); err != nil {
			t.Fatalf("loadBlocklist error: %v", err)
		}
		if !svc.ipBlocks["1.2.3.4"] {
			t.Error("missing 1.2.3.4")
		}
		if !svc.ipBlocks["5.6.7.8"] {
			t.Error("missing 5.6.7.8")
		}
		if len(svc.subnets) != 1 {
			t.Errorf("expected 1 subnet, got %d", len(svc.subnets))
		}
	})

	t.Run("load domain file", func(t *testing.T) {
		svc := newTestService(t)
		content := "bad.com\nmalware.net\n"
		f := writeTempFile(t, content)

		if err := svc.loadBlocklist(f, "domain"); err != nil {
			t.Fatalf("loadBlocklist error: %v", err)
		}
		if !svc.domains["bad.com"] {
			t.Error("missing bad.com")
		}
		if !svc.domains["malware.net"] {
			t.Error("missing malware.net")
		}
	})

	t.Run("nonexistent file returns error", func(t *testing.T) {
		svc := newTestService(t)
		err := svc.loadBlocklist("/nonexistent/path/file.txt", "ip")
		if err == nil {
			t.Error("expected error for nonexistent file, got nil")
		}
	})

	t.Run("comment lines are skipped", func(t *testing.T) {
		svc := newTestService(t)
		content := "# this is a comment\n1.2.3.4\n# another comment\n"
		f := writeTempFile(t, content)

		if err := svc.loadBlocklist(f, "ip"); err != nil {
			t.Fatalf("loadBlocklist error: %v", err)
		}
		if len(svc.ipBlocks) != 1 {
			t.Errorf("expected 1 IP, got %d", len(svc.ipBlocks))
		}
	})

	t.Run("blank lines are skipped", func(t *testing.T) {
		svc := newTestService(t)
		content := "\n\n1.2.3.4\n\n"
		f := writeTempFile(t, content)

		if err := svc.loadBlocklist(f, "ip"); err != nil {
			t.Fatalf("loadBlocklist error: %v", err)
		}
		if len(svc.ipBlocks) != 1 {
			t.Errorf("expected 1 IP, got %d", len(svc.ipBlocks))
		}
	})

	t.Run("mixed IP file with CIDRs comments and blanks", func(t *testing.T) {
		svc := newTestService(t)
		content := fmt.Sprintf(
			"# blocklist header\n\n9.9.9.9\n192.168.0.0/16 # LAN\n\n8.8.8.8\n# end\n",
		)
		f := writeTempFile(t, content)

		if err := svc.loadBlocklist(f, "ip"); err != nil {
			t.Fatalf("loadBlocklist error: %v", err)
		}
		if len(svc.ipBlocks) != 2 {
			t.Errorf("expected 2 IPs, got %d", len(svc.ipBlocks))
		}
		if len(svc.subnets) != 1 {
			t.Errorf("expected 1 subnet, got %d", len(svc.subnets))
		}
	})
}

// writeTempFile creates a temporary file with the given content and returns its
// path. The file is removed when the test ends.
func writeTempFile(t *testing.T, content string) string {
	t.Helper()
	f, err := os.CreateTemp(t.TempDir(), "blocklist-*.txt")
	if err != nil {
		t.Fatalf("failed to create temp file: %v", err)
	}
	defer f.Close()
	if _, err := f.WriteString(content); err != nil {
		t.Fatalf("failed to write temp file: %v", err)
	}
	return f.Name()
}

// TestUpdate_DisabledReturnsNil verifies that Update returns nil without touching
// the network when blocklists are disabled in config.
func TestUpdate_DisabledReturnsNil(t *testing.T) {
	svc := newTestService(t)
	cfg := config.DefaultAppConfig()
	cfg.Server.Security.Blocklists.Enabled = false
	svc.appConfig = cfg

	if err := svc.Update(context.Background()); err != nil {
		t.Fatalf("Update() disabled: got %v, want nil", err)
	}
}

// TestUpdate_EmptySourcesReturnsNil verifies that Update returns nil when
// blocklists are enabled but the sources slice is empty.
func TestUpdate_EmptySourcesReturnsNil(t *testing.T) {
	svc := newTestService(t)
	cfg := config.DefaultAppConfig()
	cfg.Server.Security.Blocklists.Enabled = true
	cfg.Server.Security.Blocklists.Sources = nil
	svc.appConfig = cfg

	if err := svc.Update(context.Background()); err != nil {
		t.Fatalf("Update() empty sources: got %v, want nil", err)
	}
}

// TestUpdate_AllSourcesDisabledWritesTimestamp verifies that Update writes the
// .last_updated file and returns nil when all sources are individually disabled.
func TestUpdate_AllSourcesDisabledWritesTimestamp(t *testing.T) {
	svc := newTestService(t)
	cfg := config.DefaultAppConfig()
	cfg.Server.Security.Blocklists.Enabled = true
	cfg.Server.Security.Blocklists.Sources = []config.BlocklistSource{
		{Name: "list1", URL: "http://example.com/1.txt", Type: "ip", Enabled: false},
		{Name: "list2", URL: "http://example.com/2.txt", Type: "domain", Enabled: false},
	}
	svc.appConfig = cfg

	if err := svc.Update(context.Background()); err != nil {
		t.Fatalf("Update() all-disabled sources: got %v, want nil", err)
	}

	tsFile := filepath.Join(svc.dataDir, ".last_updated")
	if _, err := os.Stat(tsFile); os.IsNotExist(err) {
		t.Error("Update() did not write .last_updated timestamp file")
	}
}
