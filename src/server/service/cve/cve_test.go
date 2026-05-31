// SPDX-License-Identifier: MIT
// Tests for the CVE service: NewCVEService smoke, Initialize, calculateSeverity,
// GetCVE, SearchByCPE, GetStats, LastUpdate, and loadCVEData.
package cve

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/apimgr/vidveil/src/config"
)

// newTestService builds a CVEService without touching the real filesystem or
// config.GetAppPaths. The dataDir is a fresh temp directory per test.
func newTestService(t *testing.T) *CVEService {
	t.Helper()
	return &CVEService{
		cveData: make(map[string]CVEItem),
		dataDir: t.TempDir(),
	}
}

// writeTempJSON writes v as JSON to a temp file and returns its path.
func writeTempJSON(t *testing.T, v interface{}) string {
	t.Helper()
	f, err := os.CreateTemp(t.TempDir(), "nvd-*.json")
	if err != nil {
		t.Fatalf("failed to create temp file: %v", err)
	}
	defer f.Close()
	if err := json.NewEncoder(f).Encode(v); err != nil {
		t.Fatalf("failed to encode JSON: %v", err)
	}
	return f.Name()
}

// ---- NewCVEService smoke ----

// TestNewCVEService_Smoke verifies that NewCVEService returns a non-nil service
// with an initialised cveData map when given a valid AppConfig.
func TestNewCVEService_Smoke(t *testing.T) {
	cfg := config.DefaultAppConfig()
	svc := NewCVEService(cfg)

	if svc == nil {
		t.Fatal("NewCVEService returned nil")
	}
	if svc.cveData == nil {
		t.Error("cveData map is nil")
	}
}

// ---- Initialize ----

// TestInitialize_CreatesDirectory verifies that Initialize creates the dataDir
// when it does not yet exist.
func TestInitialize_CreatesDirectory(t *testing.T) {
	svc := newTestService(t)
	svc.dataDir = filepath.Join(t.TempDir(), "nested", "cve")

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

// ---- calculateSeverity ----

// TestCalculateSeverity verifies CVSS-to-severity mapping at every boundary
// including negative scores which should return "NONE".
func TestCalculateSeverity(t *testing.T) {
	tests := []struct {
		cvss float64
		want string
	}{
		{cvss: 9.0, want: "CRITICAL"},
		{cvss: 9.9, want: "CRITICAL"},
		{cvss: 10.0, want: "CRITICAL"},
		{cvss: 7.0, want: "HIGH"},
		{cvss: 8.9, want: "HIGH"},
		{cvss: 4.0, want: "MEDIUM"},
		{cvss: 6.9, want: "MEDIUM"},
		{cvss: 0.1, want: "LOW"},
		{cvss: 3.9, want: "LOW"},
		{cvss: 0.0, want: "NONE"},
		{cvss: -1.0, want: "NONE"},
	}

	svc := newTestService(t)
	for _, tc := range tests {
		got := svc.calculateSeverity(tc.cvss)
		if got != tc.want {
			t.Errorf("calculateSeverity(%v) = %q, want %q", tc.cvss, got, tc.want)
		}
	}
}

// ---- GetCVE ----

// TestGetCVE_Found verifies that a populated entry is returned with true.
func TestGetCVE_Found(t *testing.T) {
	svc := newTestService(t)
	svc.cveData["CVE-2024-0001"] = CVEItem{ID: "CVE-2024-0001", Severity: "HIGH"}

	item, ok := svc.GetCVE("CVE-2024-0001")
	if !ok {
		t.Fatal("GetCVE returned false, expected true")
	}
	if item.ID != "CVE-2024-0001" {
		t.Errorf("item.ID = %q, want %q", item.ID, "CVE-2024-0001")
	}
}

// TestGetCVE_CaseInsensitive verifies that a lowercase query matches a
// key stored in uppercase, because GetCVE calls strings.ToUpper on the input.
func TestGetCVE_CaseInsensitive(t *testing.T) {
	svc := newTestService(t)
	svc.cveData["CVE-2024-0001"] = CVEItem{ID: "CVE-2024-0001", Severity: "HIGH"}

	item, ok := svc.GetCVE("cve-2024-0001")
	if !ok {
		t.Fatal("GetCVE with lowercase ID returned false, expected true")
	}
	if item.ID != "CVE-2024-0001" {
		t.Errorf("item.ID = %q, want %q", item.ID, "CVE-2024-0001")
	}
}

// TestGetCVE_NotFound verifies that a missing ID returns the zero value and false.
func TestGetCVE_NotFound(t *testing.T) {
	svc := newTestService(t)

	item, ok := svc.GetCVE("CVE-NOTEXIST")
	if ok {
		t.Error("GetCVE returned true for a missing ID")
	}
	if item.ID != "" {
		t.Errorf("expected zero CVEItem, got ID=%q", item.ID)
	}
}

// ---- SearchByCPE ----

// TestSearchByCPE_Match verifies that a substring CPE query returns the matching item.
func TestSearchByCPE_Match(t *testing.T) {
	svc := newTestService(t)
	svc.cveData["CVE-2024-1000"] = CVEItem{
		ID:       "CVE-2024-1000",
		Severity: "CRITICAL",
		CPEs:     []string{"cpe:2.3:a:example:product:1.0:*:*:*:*:*:*:*"},
	}

	results := svc.SearchByCPE("example:product")
	if len(results) != 1 {
		t.Fatalf("SearchByCPE: got %d results, want 1", len(results))
	}
	if results[0].ID != "CVE-2024-1000" {
		t.Errorf("result ID = %q, want %q", results[0].ID, "CVE-2024-1000")
	}
}

// TestSearchByCPE_CaseInsensitive verifies that both the stored CPE and the
// query are lowercased before comparison.
func TestSearchByCPE_CaseInsensitive(t *testing.T) {
	svc := newTestService(t)
	svc.cveData["CVE-2024-2000"] = CVEItem{
		ID:   "CVE-2024-2000",
		CPEs: []string{"cpe:2.3:a:EXAMPLE:PRODUCT:1.0:*:*:*:*:*:*:*"},
	}

	results := svc.SearchByCPE("example:product")
	if len(results) != 1 {
		t.Fatalf("SearchByCPE (case-insensitive): got %d results, want 1", len(results))
	}
}

// TestSearchByCPE_NoMatch verifies that a query with no matching CPE returns
// an empty slice.
func TestSearchByCPE_NoMatch(t *testing.T) {
	svc := newTestService(t)
	svc.cveData["CVE-2024-3000"] = CVEItem{
		ID:   "CVE-2024-3000",
		CPEs: []string{"cpe:2.3:a:example:product:1.0:*:*:*:*:*:*:*"},
	}

	results := svc.SearchByCPE("nomatch")
	if len(results) != 0 {
		t.Errorf("SearchByCPE nomatch: got %d results, want 0", len(results))
	}
}

// TestSearchByCPE_NoDuplicates verifies that a CVE matching on multiple CPEs
// appears only once in the results.
func TestSearchByCPE_NoDuplicates(t *testing.T) {
	svc := newTestService(t)
	svc.cveData["CVE-2024-4000"] = CVEItem{
		ID: "CVE-2024-4000",
		CPEs: []string{
			"cpe:2.3:a:example:product:1.0:*:*:*:*:*:*:*",
			"cpe:2.3:a:example:product:2.0:*:*:*:*:*:*:*",
		},
	}

	results := svc.SearchByCPE("example:product")
	if len(results) != 1 {
		t.Errorf("SearchByCPE duplicate check: got %d results, want 1", len(results))
	}
}

// ---- GetStats ----

// TestGetStats_Empty verifies correct zero-value stats for a service with no data.
func TestGetStats_Empty(t *testing.T) {
	svc := newTestService(t)
	stats := svc.GetStats()

	if stats["total_cves"] != 0 {
		t.Errorf("total_cves: got %v, want 0", stats["total_cves"])
	}
	bySeverity, ok := stats["by_severity"].(map[string]int)
	if !ok {
		t.Fatalf("by_severity is not map[string]int: %T", stats["by_severity"])
	}
	if len(bySeverity) != 0 {
		t.Errorf("by_severity: expected empty map, got %v", bySeverity)
	}
	if stats["data_dir"] != svc.dataDir {
		t.Errorf("data_dir: got %v, want %s", stats["data_dir"], svc.dataDir)
	}
}

// TestGetStats_Populated verifies that total_cves and by_severity counts are
// accurate when the service holds multiple items with different severities.
func TestGetStats_Populated(t *testing.T) {
	svc := newTestService(t)
	svc.cveData["CVE-A"] = CVEItem{ID: "CVE-A", Severity: "CRITICAL"}
	svc.cveData["CVE-B"] = CVEItem{ID: "CVE-B", Severity: "CRITICAL"}
	svc.cveData["CVE-C"] = CVEItem{ID: "CVE-C", Severity: "HIGH"}

	stats := svc.GetStats()

	if stats["total_cves"] != 3 {
		t.Errorf("total_cves: got %v, want 3", stats["total_cves"])
	}
	bySeverity, ok := stats["by_severity"].(map[string]int)
	if !ok {
		t.Fatalf("by_severity is not map[string]int: %T", stats["by_severity"])
	}
	if bySeverity["CRITICAL"] != 2 {
		t.Errorf("CRITICAL count: got %d, want 2", bySeverity["CRITICAL"])
	}
	if bySeverity["HIGH"] != 1 {
		t.Errorf("HIGH count: got %d, want 1", bySeverity["HIGH"])
	}
}

// ---- LastUpdate ----

// TestLastUpdate_NoFile verifies that a missing .last_updated file returns
// the zero time.Time.
func TestLastUpdate_NoFile(t *testing.T) {
	svc := newTestService(t)

	got := svc.LastUpdate()
	if !got.IsZero() {
		t.Errorf("expected zero time, got %v", got)
	}
}

// TestLastUpdate_ValidRFC3339 verifies that a well-formed RFC3339 timestamp
// file returns the corresponding time.
func TestLastUpdate_ValidRFC3339(t *testing.T) {
	svc := newTestService(t)
	want := time.Date(2024, 3, 15, 10, 0, 0, 0, time.UTC)
	tsFile := filepath.Join(svc.dataDir, ".last_updated")
	if err := os.WriteFile(tsFile, []byte(want.Format(time.RFC3339)), 0644); err != nil {
		t.Fatalf("failed to write timestamp: %v", err)
	}

	got := svc.LastUpdate()
	if !got.Equal(want) {
		t.Errorf("LastUpdate: got %v, want %v", got, want)
	}
}

// TestLastUpdate_GarbageContent verifies that an unparseable timestamp file
// returns the zero time.Time.
func TestLastUpdate_GarbageContent(t *testing.T) {
	svc := newTestService(t)
	tsFile := filepath.Join(svc.dataDir, ".last_updated")
	if err := os.WriteFile(tsFile, []byte("not-a-timestamp"), 0644); err != nil {
		t.Fatalf("failed to write timestamp: %v", err)
	}

	got := svc.LastUpdate()
	if !got.IsZero() {
		t.Errorf("expected zero time for garbage content, got %v", got)
	}
}

// ---- loadCVEData ----

// TestLoadCVEData_Valid verifies that a well-formed NVD JSON file is parsed
// correctly: the CVE is stored with the right description, severity, and CPE.
func TestLoadCVEData_Valid(t *testing.T) {
	svc := newTestService(t)

	// appConfig must be non-nil so that FilterByCPE can be read safely.
	svc.appConfig = config.DefaultAppConfig()

	nvd := NVDResponse{}
	nvd.CVEItems = []struct {
		CVE struct {
			DataMeta struct {
				ID string `json:"ID"`
			} `json:"CVE_data_meta"`
			Description struct {
				DescriptionData []struct {
					Value string `json:"value"`
				} `json:"description_data"`
			} `json:"description"`
		} `json:"cve"`
		PublishedDate    string `json:"publishedDate"`
		LastModifiedDate string `json:"lastModifiedDate"`
		Impact           struct {
			BaseMetricV3 struct {
				CVSSV3 struct {
					BaseScore float64 `json:"baseScore"`
				} `json:"cvssV3"`
			} `json:"baseMetricV3"`
		} `json:"impact"`
		Configurations struct {
			Nodes []struct {
				CPEMatch []struct {
					CPE23URI string `json:"cpe23Uri"`
				} `json:"cpe_match"`
			} `json:"nodes"`
		} `json:"configurations"`
	}{
		{},
	}
	nvd.CVEItems[0].CVE.DataMeta.ID = "CVE-2024-TEST"
	nvd.CVEItems[0].CVE.Description.DescriptionData = []struct {
		Value string `json:"value"`
	}{{Value: "Test CVE"}}
	nvd.CVEItems[0].PublishedDate = "2024-01-01T00:00:00Z"
	nvd.CVEItems[0].LastModifiedDate = "2024-01-02T00:00:00Z"
	nvd.CVEItems[0].Impact.BaseMetricV3.CVSSV3.BaseScore = 9.1
	nvd.CVEItems[0].Configurations.Nodes = []struct {
		CPEMatch []struct {
			CPE23URI string `json:"cpe23Uri"`
		} `json:"cpe_match"`
	}{
		{CPEMatch: []struct {
			CPE23URI string `json:"cpe23Uri"`
		}{{CPE23URI: "cpe:2.3:a:test:product:*"}}},
	}

	filename := writeTempJSON(t, nvd)

	if err := svc.loadCVEData(filename); err != nil {
		t.Fatalf("loadCVEData returned unexpected error: %v", err)
	}

	item, ok := svc.GetCVE("CVE-2024-TEST")
	if !ok {
		t.Fatal("CVE-2024-TEST not found after loadCVEData")
	}
	if item.Description != "Test CVE" {
		t.Errorf("Description = %q, want %q", item.Description, "Test CVE")
	}
	if item.Severity != "CRITICAL" {
		t.Errorf("Severity = %q, want CRITICAL (CVSS 9.1)", item.Severity)
	}
	if len(item.CPEs) == 0 || item.CPEs[0] != "cpe:2.3:a:test:product:*" {
		t.Errorf("CPEs = %v, want [cpe:2.3:a:test:product:*]", item.CPEs)
	}
}

// TestLoadCVEData_NonexistentFile verifies that loadCVEData returns an error
// when the specified file does not exist.
func TestLoadCVEData_NonexistentFile(t *testing.T) {
	svc := newTestService(t)
	svc.appConfig = config.DefaultAppConfig()

	err := svc.loadCVEData("/nonexistent/path/nvd.json")
	if err == nil {
		t.Error("expected error for nonexistent file, got nil")
	}
}

// TestLoadCVEData_InvalidJSON verifies that loadCVEData returns an error when
// the file contains malformed JSON.
func TestLoadCVEData_InvalidJSON(t *testing.T) {
	svc := newTestService(t)
	svc.appConfig = config.DefaultAppConfig()

	f, err := os.CreateTemp(t.TempDir(), "bad-*.json")
	if err != nil {
		t.Fatalf("failed to create temp file: %v", err)
	}
	if _, err := f.WriteString("{not valid json{{"); err != nil {
		t.Fatalf("failed to write bad JSON: %v", err)
	}
	f.Close()

	if err := svc.loadCVEData(f.Name()); err == nil {
		t.Error("expected error for invalid JSON, got nil")
	}
}
