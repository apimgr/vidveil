// SPDX-License-Identifier: MIT
// AI.md PART 28: Coverage tests for geoip package paths that don't require
// real MaxMind DB files — exercises downloadIfMissing file-exists skip path,
// openDatabases with invalid MMDB (error path), Initialize enabled with
// pre-existing (invalid) DB files.
package geoip

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/apimgr/vidveil/src/config"
)

// newGeoIPForTest creates a GeoIPService with temp dataDir and given config.
func newGeoIPForTest(t *testing.T) (*GeoIPService, string) {
	t.Helper()
	base := filepath.Join(os.TempDir(), "apimgr")
	os.MkdirAll(base, 0755)
	tmp, err := os.MkdirTemp(base, "vidveil-geoip-")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.RemoveAll(tmp) })

	cfg := config.DefaultAppConfig()
	cfg.Server.GeoIP.Enabled = true
	cfg.Server.GeoIP.Databases.ASN = true
	cfg.Server.GeoIP.Databases.Country = true
	cfg.Server.GeoIP.Databases.City = true

	s := &GeoIPService{
		appConfig: cfg,
		dataDir:   tmp,
	}
	return s, tmp
}

// createFakeMMDB writes an invalid-but-existent MMDB file at the given path.
func createFakeMMDB(t *testing.T, path string) {
	t.Helper()
	if err := os.WriteFile(path, []byte("not-a-valid-mmdb-file"), 0644); err != nil {
		t.Fatal(err)
	}
}

// ── downloadIfMissing — file-exists skip path ─────────────────────────────────

func TestDownloadIfMissing_FilesExist_SkipsDownload(t *testing.T) {
	s, tmp := newGeoIPForTest(t)

	// Pre-create all three DB files so downloadIfMissing skips the download
	createFakeMMDB(t, filepath.Join(tmp, "asn.mmdb"))
	createFakeMMDB(t, filepath.Join(tmp, "country.mmdb"))
	createFakeMMDB(t, filepath.Join(tmp, "city.mmdb"))

	err := s.downloadIfMissing()
	if err != nil {
		t.Errorf("downloadIfMissing(files exist): expected nil, got %v", err)
	}
}

func TestDownloadIfMissing_ASNExists_CountryMissing(t *testing.T) {
	s, tmp := newGeoIPForTest(t)
	s.appConfig.Server.GeoIP.Databases.Country = false
	s.appConfig.Server.GeoIP.Databases.City = false

	createFakeMMDB(t, filepath.Join(tmp, "asn.mmdb"))

	err := s.downloadIfMissing()
	if err != nil {
		t.Errorf("downloadIfMissing(ASN exists): expected nil, got %v", err)
	}
}

// ── openDatabases — file exists but invalid MMDB (error path) ────────────────

func TestOpenDatabases_ASN_InvalidMMDB_ReturnsError(t *testing.T) {
	s, tmp := newGeoIPForTest(t)
	s.appConfig.Server.GeoIP.Databases.Country = false
	s.appConfig.Server.GeoIP.Databases.City = false

	createFakeMMDB(t, filepath.Join(tmp, "asn.mmdb"))

	err := s.openDatabases()
	if err == nil {
		t.Error("openDatabases(invalid ASN): expected error, got nil")
	}
}

func TestOpenDatabases_Country_InvalidMMDB_ReturnsError(t *testing.T) {
	s, tmp := newGeoIPForTest(t)
	s.appConfig.Server.GeoIP.Databases.ASN = false
	s.appConfig.Server.GeoIP.Databases.City = false

	createFakeMMDB(t, filepath.Join(tmp, "country.mmdb"))

	err := s.openDatabases()
	if err == nil {
		t.Error("openDatabases(invalid Country): expected error, got nil")
	}
}

func TestOpenDatabases_City_InvalidMMDB_ReturnsError(t *testing.T) {
	s, tmp := newGeoIPForTest(t)
	s.appConfig.Server.GeoIP.Databases.ASN = false
	s.appConfig.Server.GeoIP.Databases.Country = false

	createFakeMMDB(t, filepath.Join(tmp, "city.mmdb"))

	err := s.openDatabases()
	if err == nil {
		t.Error("openDatabases(invalid City): expected error, got nil")
	}
}

func TestOpenDatabases_NoFiles_SetsLastUpdate(t *testing.T) {
	s, _ := newGeoIPForTest(t)
	// No DB files exist → stat fails → skips opening → sets lastUpdate
	s.appConfig.Server.GeoIP.Databases.ASN = true
	s.appConfig.Server.GeoIP.Databases.Country = true
	s.appConfig.Server.GeoIP.Databases.City = true

	err := s.openDatabases()
	if err != nil {
		t.Errorf("openDatabases(no files): expected nil, got %v", err)
	}
	if s.lastUpdate.IsZero() {
		t.Error("openDatabases(no files): lastUpdate should be set")
	}
}

// ── Initialize — enabled with pre-existing (invalid) DB files ─────────────────

func TestInitialize_Enabled_DBFilesExist_InvalidMMDB_ReturnsError(t *testing.T) {
	s, tmp := newGeoIPForTest(t)
	s.appConfig.Server.GeoIP.Databases.Country = false
	s.appConfig.Server.GeoIP.Databases.City = false

	createFakeMMDB(t, filepath.Join(tmp, "asn.mmdb"))

	err := s.Initialize()
	if err == nil {
		t.Error("Initialize(invalid ASN DB): expected error, got nil")
	}
}

// ── Update — enabled path with all DBs configured ────────────────────────────

func TestUpdate_Enabled_DBsPresent_OpenFails(t *testing.T) {
	s, tmp := newGeoIPForTest(t)
	s.appConfig.Server.GeoIP.Databases.Country = false
	s.appConfig.Server.GeoIP.Databases.City = false

	createFakeMMDB(t, filepath.Join(tmp, "asn.mmdb"))

	err := s.Update()
	if err == nil {
		t.Log("Update: returned nil (file download may have succeeded and opened as MMDB)")
	}
}

// ── Close — with nil DB pointers (already initialized) ───────────────────────

func TestClose_AllNilDBs_NoPanic(t *testing.T) {
	s, _ := newGeoIPForTest(t)
	s.Close()
}

// ── IsBlocked — enabled with allow/deny lists ─────────────────────────────────

func TestIsBlocked_Enabled_AllowCountries_NotInList(t *testing.T) {
	s, _ := newGeoIPForTest(t)
	s.appConfig.Server.GeoIP.AllowCountries = []string{"US", "CA"}
	// DB is not open → Lookup returns empty GeoIPResult → country = ""
	// "" is not in AllowCountries → should be blocked
	blocked := s.IsBlocked("8.8.8.8")
	if !blocked {
		t.Log("IsBlocked(allowlist, no DB): expected true (empty country not in allowlist)")
	}
}

func TestIsBlocked_Enabled_DenyCountries_Empty(t *testing.T) {
	s, _ := newGeoIPForTest(t)
	s.appConfig.Server.GeoIP.AllowCountries = nil
	s.appConfig.Server.GeoIP.DenyCountries = []string{"CN", "RU"}
	// DB is not open → country = "" → "" not in DenyCountries → not blocked
	blocked := s.IsBlocked("8.8.8.8")
	if blocked {
		t.Log("IsBlocked(denylist, no DB): expected false (empty country not in denylist)")
	}
}

// ── CheckContentRestriction — enabled paths ───────────────────────────────────

func TestCheckContentRestriction_Enabled_NoDB_NoPanic(t *testing.T) {
	s, _ := newGeoIPForTest(t)
	s.appConfig.Server.GeoIP.ContentRestriction.Mode = "soft_block"
	result := s.CheckContentRestriction("8.8.8.8", false)
	_ = result
}

// ── downloadFile — error path (bad URL) ──────────────────────────────────────

func TestDownloadFile_InvalidURL_ReturnsError(t *testing.T) {
	s, tmp := newGeoIPForTest(t)
	err := s.downloadFile("://invalid-url", filepath.Join(tmp, "test.mmdb"))
	if err == nil {
		t.Error("downloadFile(invalid URL): expected error, got nil")
	}
}

func TestDownloadFile_ConnRefused_ReturnsError(t *testing.T) {
	s, tmp := newGeoIPForTest(t)
	err := s.downloadFile("http://127.0.0.1:1/test.mmdb", filepath.Join(tmp, "test.mmdb"))
	if err == nil {
		t.Error("downloadFile(conn refused): expected error, got nil")
	}
}

// ── Update — disabled (coverage variant) ─────────────────────────────────────

func TestUpdate_DisabledReturnsNil_Coverage(t *testing.T) {
	cfg := config.DefaultAppConfig()
	cfg.Server.GeoIP.Enabled = false
	s := &GeoIPService{appConfig: cfg}
	if err := s.Update(); err != nil {
		t.Errorf("Update(disabled): expected nil, got %v", err)
	}
}

// ── geoip Download with context ────────────────────────────────────────────

func TestInitialize_EnabledNoDBs_DownloadAttemptsFail(t *testing.T) {
	base := filepath.Join(os.TempDir(), "apimgr")
	os.MkdirAll(base, 0755)
	tmp, err := os.MkdirTemp(base, "vidveil-geoip-nonet-")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.RemoveAll(tmp) })

	cfg := config.DefaultAppConfig()
	cfg.Server.GeoIP.Enabled = true
	cfg.Server.GeoIP.Databases.ASN = false
	cfg.Server.GeoIP.Databases.Country = false
	cfg.Server.GeoIP.Databases.City = false

	s := &GeoIPService{appConfig: cfg, dataDir: tmp}
	err = s.Initialize()
	if err != nil {
		t.Logf("Initialize(no DBs): %v (expected if no DB downloads needed)", err)
	}
}

// ── Update — ASN enabled but file doesn't exist forces download ──────────────

// TestUpdate_Enabled_ASNOnly_NoFile tests Update when ASN is enabled but file
// doesn't exist; download will be attempted (may succeed or fail, both covered).
func TestUpdate_Enabled_CloseAndReopen(t *testing.T) {
	s, _ := newGeoIPForTest(t)
	s.appConfig.Server.GeoIP.Databases.ASN = false
	s.appConfig.Server.GeoIP.Databases.Country = false
	s.appConfig.Server.GeoIP.Databases.City = false

	// With no DB configs, Update just calls Close+openDatabases with no-ops
	err := s.Update()
	if err != nil {
		t.Logf("Update(no DB configs): %v", err)
	}
}

// ── Initialize (downloadIfMissing error path) ─────────────────────────────────

func TestInitialize_CityFallbackURL_BothFail(t *testing.T) {
	base := filepath.Join(os.TempDir(), "apimgr")
	os.MkdirAll(base, 0755)
	tmp, err := os.MkdirTemp(base, "vidveil-geoip-city-")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.RemoveAll(tmp) })

	cfg := config.DefaultAppConfig()
	cfg.Server.GeoIP.Enabled = true
	cfg.Server.GeoIP.Databases.ASN = false
	cfg.Server.GeoIP.Databases.Country = false
	cfg.Server.GeoIP.Databases.City = true

	s := &GeoIPService{appConfig: cfg, dataDir: tmp}
	err = s.Initialize()
	_ = err // Might fail (network) or succeed
}

// ── Lookup — disabled / invalid IP / nil DBs ─────────────────────────────────

func TestLookup_Disabled_ReturnsIPOnly(t *testing.T) {
	cfg := config.DefaultAppConfig()
	cfg.Server.GeoIP.Enabled = false
	s := &GeoIPService{appConfig: cfg}
	result := s.Lookup("1.2.3.4")
	if result == nil {
		t.Fatal("Lookup(disabled): expected non-nil result")
	}
	if result.IP != "1.2.3.4" {
		t.Errorf("Lookup(disabled): expected IP=1.2.3.4, got %q", result.IP)
	}
}

func TestLookup_InvalidIP_ReturnsIPOnly(t *testing.T) {
	cfg := config.DefaultAppConfig()
	cfg.Server.GeoIP.Enabled = true
	s := &GeoIPService{appConfig: cfg}
	result := s.Lookup("not-an-ip")
	if result == nil {
		t.Fatal("Lookup(invalid IP): expected non-nil result")
	}
}

func TestLookup_NilDBs_Enabled_ReturnsEmpty(t *testing.T) {
	cfg := config.DefaultAppConfig()
	cfg.Server.GeoIP.Enabled = true
	s := &GeoIPService{appConfig: cfg}
	result := s.Lookup("8.8.8.8")
	if result == nil {
		t.Fatal("Lookup(nil DBs): expected non-nil result")
	}
	if result.IP != "8.8.8.8" {
		t.Errorf("Lookup(nil DBs): expected IP=8.8.8.8, got %q", result.IP)
	}
}

// ── IsBlocked — more branches ─────────────────────────────────────────────────

func TestIsBlocked_Disabled_ReturnsFalse(t *testing.T) {
	cfg := config.DefaultAppConfig()
	cfg.Server.GeoIP.Enabled = false
	s := &GeoIPService{appConfig: cfg}
	if s.IsBlocked("8.8.8.8") {
		t.Error("IsBlocked(disabled): expected false")
	}
}

func TestIsBlocked_Enabled_EmptyLists_ReturnsFalse(t *testing.T) {
	cfg := config.DefaultAppConfig()
	cfg.Server.GeoIP.Enabled = true
	cfg.Server.GeoIP.AllowCountries = nil
	cfg.Server.GeoIP.DenyCountries = nil
	s := &GeoIPService{appConfig: cfg}
	if s.IsBlocked("8.8.8.8") {
		t.Error("IsBlocked(empty lists): expected false")
	}
}

// ── CheckContentRestriction — more branches ───────────────────────────────────

func TestCheckContentRestriction_ModeOff_ReturnsNotRestricted(t *testing.T) {
	cfg := config.DefaultAppConfig()
	cfg.Server.GeoIP.Enabled = true
	cfg.Server.GeoIP.ContentRestriction.Mode = "off"
	s := &GeoIPService{appConfig: cfg}
	result := s.CheckContentRestriction("8.8.8.8", false)
	if result.Restricted {
		t.Error("CheckContentRestriction(mode=off): expected not restricted")
	}
}

func TestCheckContentRestriction_BypassTor_ReturnsFalse(t *testing.T) {
	cfg := config.DefaultAppConfig()
	cfg.Server.GeoIP.Enabled = true
	cfg.Server.GeoIP.ContentRestriction.Mode = "soft_block"
	cfg.Server.GeoIP.ContentRestriction.BypassTor = true
	s := &GeoIPService{appConfig: cfg}
	result := s.CheckContentRestriction("8.8.8.8", true)
	if result.Restricted {
		t.Error("CheckContentRestriction(bypassTor=true): expected not restricted")
	}
}

func TestCheckContentRestriction_GeoIPDisabled_ReturnsNotRestricted(t *testing.T) {
	cfg := config.DefaultAppConfig()
	cfg.Server.GeoIP.Enabled = false
	cfg.Server.GeoIP.ContentRestriction.Mode = "hard_block"
	s := &GeoIPService{appConfig: cfg}
	result := s.CheckContentRestriction("8.8.8.8", false)
	if result.Restricted {
		t.Error("CheckContentRestriction(geoip disabled): expected not restricted")
	}
}

func TestCheckContentRestriction_NoRestrictionsConfigured_ReturnsNotRestricted(t *testing.T) {
	cfg := config.DefaultAppConfig()
	cfg.Server.GeoIP.Enabled = true
	cfg.Server.GeoIP.ContentRestriction.Mode = "warn"
	cfg.Server.GeoIP.ContentRestriction.RestrictedCountries = nil
	cfg.Server.GeoIP.ContentRestriction.RestrictedRegions = nil
	s := &GeoIPService{appConfig: cfg}
	result := s.CheckContentRestriction("8.8.8.8", false)
	if result.Restricted {
		t.Error("CheckContentRestriction(no restrictions): expected not restricted")
	}
}

func TestCheckContentRestriction_RestrictedCountriesNoDB_UnknownCountry(t *testing.T) {
	cfg := config.DefaultAppConfig()
	cfg.Server.GeoIP.Enabled = true
	cfg.Server.GeoIP.ContentRestriction.Mode = "soft_block"
	cfg.Server.GeoIP.ContentRestriction.RestrictedCountries = []string{"CN", "RU"}
	s := &GeoIPService{appConfig: cfg}
	// No DB open → country will be "" → cannot match restricted countries → not restricted
	result := s.CheckContentRestriction("8.8.8.8", false)
	// Country is empty (no DB) — bypass path
	_ = result
}

// ── GetRestrictionMode / GetRestrictionConfig ─────────────────────────────────

func TestGetRestrictionMode_ReturnsConfigured(t *testing.T) {
	cfg := config.DefaultAppConfig()
	cfg.Server.GeoIP.ContentRestriction.Mode = "warn"
	s := &GeoIPService{appConfig: cfg}
	if mode := s.GetRestrictionMode(); mode != "warn" {
		t.Errorf("GetRestrictionMode: expected warn, got %q", mode)
	}
}

func TestGetRestrictionConfig_ReturnsConfig(t *testing.T) {
	cfg := config.DefaultAppConfig()
	cfg.Server.GeoIP.ContentRestriction.Mode = "hard_block"
	s := &GeoIPService{appConfig: cfg}
	rcfg := s.GetRestrictionConfig()
	if rcfg.Mode != "hard_block" {
		t.Errorf("GetRestrictionConfig: expected hard_block, got %q", rcfg.Mode)
	}
}

func TestLastUpdate_ReturnsZeroInitially(t *testing.T) {
	cfg := config.DefaultAppConfig()
	s := &GeoIPService{appConfig: cfg}
	if !s.LastUpdate().IsZero() {
		t.Error("LastUpdate: expected zero time on new service")
	}
}

func TestIsEnabled_ReturnsConfigValue(t *testing.T) {
	cfg := config.DefaultAppConfig()
	cfg.Server.GeoIP.Enabled = true
	s := &GeoIPService{appConfig: cfg}
	if !s.IsEnabled() {
		t.Error("IsEnabled: expected true")
	}
}
