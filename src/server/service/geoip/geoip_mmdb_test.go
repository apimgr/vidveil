// SPDX-License-Identifier: MIT
// GeoIP MMDB integration tests — use real test MMDB fixtures in testdata/.
// Test MMDB files sourced from github.com/maxmind/MaxMind-DB/test-data
package geoip

import (
	"testing"

	"github.com/apimgr/vidveil/src/config"
)

// newGeoIPWithMMDB opens all three test MMDB files from testdata/.
// The test binary runs with CWD = package dir, so "testdata" resolves correctly.
func newGeoIPWithMMDB(t *testing.T) *GeoIPService {
	t.Helper()
	cfg := config.DefaultAppConfig()
	cfg.Server.GeoIP.Enabled = true
	cfg.Server.GeoIP.Databases.ASN = true
	cfg.Server.GeoIP.Databases.Country = true
	cfg.Server.GeoIP.Databases.City = true
	s := &GeoIPService{appConfig: cfg, dataDir: "testdata"}
	if err := s.openDatabases(); err != nil {
		t.Fatalf("openDatabases(testdata): %v", err)
	}
	t.Cleanup(func() { s.Close() })
	return s
}

// ── openDatabases — success path ──────────────────────────────────────────────

func TestOpenDatabases_RealMMDB_SetsDBsAndLastUpdate(t *testing.T) {
	s := newGeoIPWithMMDB(t)
	if s.asnDB == nil {
		t.Error("asnDB should be non-nil after openDatabases with real MMDB")
	}
	if s.countryDB == nil {
		t.Error("countryDB should be non-nil after openDatabases with real MMDB")
	}
	if s.cityDB == nil {
		t.Error("cityDB should be non-nil after openDatabases with real MMDB")
	}
	if s.lastUpdate.IsZero() {
		t.Error("lastUpdate should be set after openDatabases")
	}
}

// ── Close — with non-nil DBs ──────────────────────────────────────────────────

func TestClose_WithNonNilDBs_NilsPointers(t *testing.T) {
	cfg := config.DefaultAppConfig()
	cfg.Server.GeoIP.Enabled = true
	cfg.Server.GeoIP.Databases.ASN = true
	cfg.Server.GeoIP.Databases.Country = true
	cfg.Server.GeoIP.Databases.City = true
	s := &GeoIPService{appConfig: cfg, dataDir: "testdata"}
	if err := s.openDatabases(); err != nil {
		t.Fatalf("openDatabases: %v", err)
	}
	// Close must not panic and must nil out all DB pointers
	s.Close()
	if s.asnDB != nil {
		t.Error("asnDB should be nil after Close")
	}
	if s.countryDB != nil {
		t.Error("countryDB should be nil after Close")
	}
	if s.cityDB != nil {
		t.Error("cityDB should be nil after Close")
	}
}

// ── Lookup — per-DB coverage ──────────────────────────────────────────────────

func TestLookup_CountryDBOnly_ReturnsCountryCode(t *testing.T) {
	cfg := config.DefaultAppConfig()
	cfg.Server.GeoIP.Enabled = true
	cfg.Server.GeoIP.Databases.ASN = false
	cfg.Server.GeoIP.Databases.Country = true
	cfg.Server.GeoIP.Databases.City = false
	s := &GeoIPService{appConfig: cfg, dataDir: "testdata"}
	if err := s.openDatabases(); err != nil {
		t.Fatalf("openDatabases: %v", err)
	}
	defer s.Close()

	// 81.2.69.142 → GB in GeoIP2-Country-Test.mmdb
	result := s.Lookup("81.2.69.142")
	if result == nil {
		t.Fatal("Lookup returned nil")
	}
	if result.CountryCode != "GB" {
		t.Errorf("Lookup(countryDB, 81.2.69.142): CountryCode = %q, want %q", result.CountryCode, "GB")
	}
}

func TestLookup_CityDBOnly_ReturnsCountryAndCity(t *testing.T) {
	cfg := config.DefaultAppConfig()
	cfg.Server.GeoIP.Enabled = true
	cfg.Server.GeoIP.Databases.ASN = false
	cfg.Server.GeoIP.Databases.Country = false
	cfg.Server.GeoIP.Databases.City = true
	s := &GeoIPService{appConfig: cfg, dataDir: "testdata"}
	if err := s.openDatabases(); err != nil {
		t.Fatalf("openDatabases: %v", err)
	}
	defer s.Close()

	// 81.2.69.142 → GB/London in GeoIP2-City-Test.mmdb
	result := s.Lookup("81.2.69.142")
	if result == nil {
		t.Fatal("Lookup returned nil")
	}
	if result.CountryCode != "GB" {
		t.Errorf("Lookup(cityDB, 81.2.69.142): CountryCode = %q, want %q", result.CountryCode, "GB")
	}
	if result.City != "London" {
		t.Errorf("Lookup(cityDB, 81.2.69.142): City = %q, want %q", result.City, "London")
	}
}

func TestLookup_ASNDBOnly_ReturnsASN(t *testing.T) {
	cfg := config.DefaultAppConfig()
	cfg.Server.GeoIP.Enabled = true
	cfg.Server.GeoIP.Databases.ASN = true
	cfg.Server.GeoIP.Databases.Country = false
	cfg.Server.GeoIP.Databases.City = false
	s := &GeoIPService{appConfig: cfg, dataDir: "testdata"}
	if err := s.openDatabases(); err != nil {
		t.Fatalf("openDatabases: %v", err)
	}
	defer s.Close()

	// 89.160.20.112 → ASN 29518 in GeoLite2-ASN-Test.mmdb
	result := s.Lookup("89.160.20.112")
	if result == nil {
		t.Fatal("Lookup returned nil")
	}
	if result.ASN != 29518 {
		t.Errorf("Lookup(asnDB, 89.160.20.112): ASN = %d, want %d", result.ASN, 29518)
	}
}

func TestLookup_AllDBs_PopulatesMultipleFields(t *testing.T) {
	s := newGeoIPWithMMDB(t)

	// 89.160.20.112 → SE in city DB, ASN=29518 in ASN DB
	result := s.Lookup("89.160.20.112")
	if result == nil {
		t.Fatal("Lookup returned nil")
	}
	if result.CountryCode != "SE" {
		t.Errorf("Lookup(allDBs, 89.160.20.112): CountryCode = %q, want %q", result.CountryCode, "SE")
	}
	if result.ASN != 29518 {
		t.Errorf("Lookup(allDBs, 89.160.20.112): ASN = %d, want %d", result.ASN, 29518)
	}
}

func TestLookup_InvalidIP_ReturnsEmptyResult(t *testing.T) {
	s := newGeoIPWithMMDB(t)
	result := s.Lookup("not-an-ip")
	if result == nil {
		t.Fatal("Lookup(invalid IP) returned nil")
	}
	if result.CountryCode != "" {
		t.Errorf("Lookup(invalid IP): CountryCode = %q, want empty", result.CountryCode)
	}
}

// ── IsBlocked — with real country lookup ──────────────────────────────────────

func TestIsBlocked_AllowList_CountryPresent_NotBlocked(t *testing.T) {
	s := newGeoIPWithMMDB(t)
	s.appConfig.Server.GeoIP.AllowCountries = []string{"GB", "US"}

	// 81.2.69.142 → GB, which is in the allowlist → not blocked
	if s.IsBlocked("81.2.69.142") {
		t.Error("IsBlocked(allowlist=[GB,US], 81.2.69.142→GB): expected false, got true")
	}
}

func TestIsBlocked_AllowList_CountryAbsent_Blocked(t *testing.T) {
	s := newGeoIPWithMMDB(t)
	s.appConfig.Server.GeoIP.AllowCountries = []string{"US", "CA"}

	// 81.2.69.142 → GB, which is NOT in the allowlist → blocked
	if !s.IsBlocked("81.2.69.142") {
		t.Error("IsBlocked(allowlist=[US,CA], 81.2.69.142→GB): expected true, got false")
	}
}

func TestIsBlocked_DenyList_CountryPresent_Blocked(t *testing.T) {
	s := newGeoIPWithMMDB(t)
	s.appConfig.Server.GeoIP.AllowCountries = nil
	s.appConfig.Server.GeoIP.DenyCountries = []string{"GB", "CN"}

	// 81.2.69.142 → GB, which is in the denylist → blocked
	if !s.IsBlocked("81.2.69.142") {
		t.Error("IsBlocked(denylist=[GB,CN], 81.2.69.142→GB): expected true, got false")
	}
}

func TestIsBlocked_DenyList_CountryAbsent_NotBlocked(t *testing.T) {
	s := newGeoIPWithMMDB(t)
	s.appConfig.Server.GeoIP.AllowCountries = nil
	s.appConfig.Server.GeoIP.DenyCountries = []string{"CN", "RU"}

	// 81.2.69.142 → GB, which is NOT in the denylist → not blocked
	if s.IsBlocked("81.2.69.142") {
		t.Error("IsBlocked(denylist=[CN,RU], 81.2.69.142→GB): expected false, got true")
	}
}

// ── CheckContentRestriction — with real country lookup ────────────────────────

func TestCheckContentRestriction_CountryMatch_Restricted(t *testing.T) {
	s := newGeoIPWithMMDB(t)
	s.appConfig.Server.GeoIP.ContentRestriction.Mode = "soft_block"
	s.appConfig.Server.GeoIP.ContentRestriction.RestrictedCountries = []string{"GB"}

	// 81.2.69.142 → GB, which is restricted
	res := s.CheckContentRestriction("81.2.69.142", false)
	if !res.Restricted {
		t.Error("CheckContentRestriction(GB restricted, 81.2.69.142→GB): expected Restricted=true")
	}
	if res.Reason == "" {
		t.Error("CheckContentRestriction: expected non-empty Reason")
	}
}

func TestCheckContentRestriction_CountryNotMatch_NotRestricted(t *testing.T) {
	s := newGeoIPWithMMDB(t)
	s.appConfig.Server.GeoIP.ContentRestriction.Mode = "soft_block"
	s.appConfig.Server.GeoIP.ContentRestriction.RestrictedCountries = []string{"CN"}

	// 81.2.69.142 → GB, which is not in restricted list
	res := s.CheckContentRestriction("81.2.69.142", false)
	if res.Restricted {
		t.Error("CheckContentRestriction(CN restricted, 81.2.69.142→GB): expected Restricted=false")
	}
}

func TestCheckContentRestriction_BypassTor_NotRestricted(t *testing.T) {
	s := newGeoIPWithMMDB(t)
	s.appConfig.Server.GeoIP.ContentRestriction.Mode = "hard_block"
	s.appConfig.Server.GeoIP.ContentRestriction.RestrictedCountries = []string{"GB"}
	s.appConfig.Server.GeoIP.ContentRestriction.BypassTor = true

	// Tor user should bypass restriction even if country is restricted
	res := s.CheckContentRestriction("81.2.69.142", true)
	if res.Restricted {
		t.Error("CheckContentRestriction(bypassTor=true, isTorUser=true): expected Restricted=false")
	}
}

func TestCheckContentRestriction_RegionMatch_Restricted(t *testing.T) {
	// Use only city DB so region is populated
	cfg := config.DefaultAppConfig()
	cfg.Server.GeoIP.Enabled = true
	cfg.Server.GeoIP.Databases.ASN = false
	cfg.Server.GeoIP.Databases.Country = false
	cfg.Server.GeoIP.Databases.City = true
	cfg.Server.GeoIP.ContentRestriction.Mode = "soft_block"
	s := &GeoIPService{appConfig: cfg, dataDir: "testdata"}
	if err := s.openDatabases(); err != nil {
		t.Fatalf("openDatabases: %v", err)
	}
	defer s.Close()

	// Lookup 81.2.69.142 to find its actual region before testing restriction
	probe := s.Lookup("81.2.69.142")
	if probe.CountryCode == "" || probe.Region == "" {
		t.Skipf("city MMDB has no region for 81.2.69.142 (CountryCode=%q, Region=%q); skipping region test", probe.CountryCode, probe.Region)
	}
	regionKey := probe.CountryCode + ":" + probe.Region
	s.appConfig.Server.GeoIP.ContentRestriction.RestrictedRegions = []string{regionKey}

	res := s.CheckContentRestriction("81.2.69.142", false)
	if !res.Restricted {
		t.Errorf("CheckContentRestriction(regionKey=%q): expected Restricted=true, got false", regionKey)
	}
}
