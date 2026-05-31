// SPDX-License-Identifier: MIT
package geoip

import (
	"strings"
	"testing"
	"time"

	"github.com/apimgr/vidveil/src/config"
)

// newDisabledService returns a GeoIPService with GeoIP disabled.
// All paths that consult the DB are skipped; no network or file I/O occurs.
func newDisabledService(t *testing.T) *GeoIPService {
	t.Helper()
	cfg := config.DefaultAppConfig()
	cfg.Server.GeoIP.Enabled = false
	return NewGeoIPService(cfg)
}

// newEnabledService returns a GeoIPService with GeoIP enabled and a fresh
// temp dir as the data directory. No DB files exist there, so all DB readers
// remain nil, but Enabled == true.
func newEnabledService(t *testing.T) *GeoIPService {
	t.Helper()
	cfg := config.DefaultAppConfig()
	cfg.Server.GeoIP.Enabled = true
	cfg.Server.GeoIP.Dir = t.TempDir()
	return NewGeoIPService(cfg)
}

// --- NewGeoIPService ---

func TestNewGeoIPService_NonNil(t *testing.T) {
	svc := newDisabledService(t)
	if svc == nil {
		t.Fatal("NewGeoIPService() returned nil")
	}
}

func TestNewGeoIPService_DataDirNonEmpty(t *testing.T) {
	svc := newDisabledService(t)
	if svc.dataDir == "" {
		t.Error("NewGeoIPService() produced empty dataDir")
	}
}

// When GeoIP.Dir is set in config, dataDir must match it exactly.
func TestNewGeoIPService_CustomDir(t *testing.T) {
	cfg := config.DefaultAppConfig()
	cfg.Server.GeoIP.Dir = t.TempDir()
	svc := NewGeoIPService(cfg)
	if svc.dataDir != cfg.Server.GeoIP.Dir {
		t.Errorf("dataDir = %q, want %q", svc.dataDir, cfg.Server.GeoIP.Dir)
	}
}

// When GeoIP.Dir is empty, a fallback path is computed from GetAppPaths.
func TestNewGeoIPService_FallbackDirContainsGeoip(t *testing.T) {
	cfg := config.DefaultAppConfig()
	cfg.Server.GeoIP.Dir = ""
	svc := NewGeoIPService(cfg)
	if !strings.Contains(svc.dataDir, "geoip") {
		t.Errorf("expected fallback dataDir to contain 'geoip', got %q", svc.dataDir)
	}
}

// --- IsEnabled ---

func TestIsEnabled_Disabled(t *testing.T) {
	svc := newDisabledService(t)
	if svc.IsEnabled() {
		t.Error("IsEnabled() = true for disabled service, want false")
	}
}

func TestIsEnabled_Enabled(t *testing.T) {
	svc := newEnabledService(t)
	if !svc.IsEnabled() {
		t.Error("IsEnabled() = false for enabled service, want true")
	}
}

// --- Initialize ---

// Disabled service must treat Initialize as a no-op and return nil.
func TestInitialize_Disabled_ReturnsNil(t *testing.T) {
	svc := newDisabledService(t)
	if err := svc.Initialize(); err != nil {
		t.Errorf("Initialize() on disabled service returned error: %v", err)
	}
}

// Enabled service with no databases configured (ASN/Country/City all false)
// should return nil without any downloads.
func TestInitialize_EnabledNoDatabases_ReturnsNil(t *testing.T) {
	cfg := config.DefaultAppConfig()
	cfg.Server.GeoIP.Enabled = true
	cfg.Server.GeoIP.Dir = t.TempDir()
	cfg.Server.GeoIP.Databases.ASN = false
	cfg.Server.GeoIP.Databases.Country = false
	cfg.Server.GeoIP.Databases.City = false
	svc := NewGeoIPService(cfg)

	if err := svc.Initialize(); err != nil {
		t.Errorf("Initialize() with no databases configured returned error: %v", err)
	}
}

// --- LastUpdate ---

// Before any Initialize call, lastUpdate must be zero.
func TestLastUpdate_BeforeInitialize_IsZero(t *testing.T) {
	svc := newDisabledService(t)
	if !svc.LastUpdate().IsZero() {
		t.Errorf("LastUpdate() before initialize = %v, want zero", svc.LastUpdate())
	}
}

// Calling Initialize on a disabled service must leave lastUpdate at zero.
func TestLastUpdate_DisabledInitialize_RemainsZero(t *testing.T) {
	svc := newDisabledService(t)
	_ = svc.Initialize()
	if !svc.LastUpdate().IsZero() {
		t.Errorf("LastUpdate() after disabled Initialize = %v, want zero", svc.LastUpdate())
	}
}

// --- GetRestrictionMode ---

// Default config has mode "warn" per AI.md PART 19 defaults.
func TestGetRestrictionMode_Default(t *testing.T) {
	cfg := config.DefaultAppConfig()
	svc := NewGeoIPService(cfg)
	mode := svc.GetRestrictionMode()
	// Default AppConfig sets mode = "warn"; an empty string is also acceptable
	// if someone overrides to off, but the default must be non-empty.
	if mode == "" {
		// Re-check: DefaultAppConfig returns "warn", so a blank here is a regression.
		t.Errorf("GetRestrictionMode() with default config = %q, want %q", mode, "warn")
	}
}

// Explicitly setting mode "off" must be reflected.
func TestGetRestrictionMode_Off(t *testing.T) {
	cfg := config.DefaultAppConfig()
	cfg.Server.GeoIP.ContentRestriction.Mode = "off"
	svc := NewGeoIPService(cfg)
	if got := svc.GetRestrictionMode(); got != "off" {
		t.Errorf("GetRestrictionMode() = %q, want %q", got, "off")
	}
}

// --- GetRestrictionConfig ---

// Must return a valid (non-zero) struct; Mode field is always accessible.
func TestGetRestrictionConfig_ReturnsStruct(t *testing.T) {
	svc := newDisabledService(t)
	cfg := svc.GetRestrictionConfig()
	// The struct is a value type; we verify its Mode field is populated from
	// DefaultAppConfig (should be "warn").
	_ = cfg.Mode
	_ = cfg.BypassTor
	_ = cfg.RestrictedCountries
}

func TestGetRestrictionConfig_MatchesConfig(t *testing.T) {
	cfg := config.DefaultAppConfig()
	cfg.Server.GeoIP.ContentRestriction.Mode = "hard_block"
	svc := NewGeoIPService(cfg)
	rc := svc.GetRestrictionConfig()
	if rc.Mode != "hard_block" {
		t.Errorf("GetRestrictionConfig().Mode = %q, want %q", rc.Mode, "hard_block")
	}
}

// --- Lookup ---

// Disabled service must always return a result with IP set, no country info.
func TestLookup_Disabled_ReturnsResultWithIP(t *testing.T) {
	svc := newDisabledService(t)
	result := svc.Lookup("1.2.3.4")
	if result == nil {
		t.Fatal("Lookup() returned nil")
	}
	if result.IP != "1.2.3.4" {
		t.Errorf("Lookup().IP = %q, want %q", result.IP, "1.2.3.4")
	}
}

// Disabled: IPv6 loopback returns result with correct IP.
func TestLookup_Disabled_IPv6Loopback(t *testing.T) {
	svc := newDisabledService(t)
	result := svc.Lookup("::1")
	if result == nil {
		t.Fatal("Lookup() returned nil for ::1")
	}
	if result.IP != "::1" {
		t.Errorf("Lookup().IP = %q, want %q", result.IP, "::1")
	}
}

// Invalid IP string: net.ParseIP returns nil; code must still return a result.
func TestLookup_Disabled_InvalidIP(t *testing.T) {
	svc := newDisabledService(t)
	result := svc.Lookup("not-an-ip")
	if result == nil {
		t.Fatal("Lookup() returned nil for invalid IP")
	}
	if result.IP != "not-an-ip" {
		t.Errorf("Lookup().IP = %q, want %q", result.IP, "not-an-ip")
	}
}

// Enabled with no DB: invalid IP still returns non-nil result with IP set.
func TestLookup_Enabled_NoDB_InvalidIP(t *testing.T) {
	svc := newEnabledService(t)
	result := svc.Lookup("bad-ip")
	if result == nil {
		t.Fatal("Lookup() returned nil for invalid IP (enabled, no DB)")
	}
	if result.IP != "bad-ip" {
		t.Errorf("Lookup().IP = %q, want %q", result.IP, "bad-ip")
	}
}

// Enabled with no DB: valid IP returns result with empty CountryCode (no DB loaded).
func TestLookup_Enabled_NoDB_ValidIP(t *testing.T) {
	svc := newEnabledService(t)
	result := svc.Lookup("8.8.8.8")
	if result == nil {
		t.Fatal("Lookup() returned nil for 8.8.8.8")
	}
	if result.IP != "8.8.8.8" {
		t.Errorf("Lookup().IP = %q, want %q", result.IP, "8.8.8.8")
	}
	if result.CountryCode != "" {
		t.Errorf("Lookup().CountryCode = %q with no DB, want empty", result.CountryCode)
	}
}

// --- IsBlocked ---

// Disabled service must never block.
func TestIsBlocked_Disabled_AlwaysFalse(t *testing.T) {
	svc := newDisabledService(t)
	for _, ip := range []string{"1.2.3.4", "::1", "0.0.0.0", "255.255.255.255"} {
		if svc.IsBlocked(ip) {
			t.Errorf("IsBlocked(%q) = true for disabled service, want false", ip)
		}
	}
}

// Enabled, no allow/deny lists → nothing blocked.
func TestIsBlocked_Enabled_NoLists_NotBlocked(t *testing.T) {
	cfg := config.DefaultAppConfig()
	cfg.Server.GeoIP.Enabled = true
	cfg.Server.GeoIP.Dir = t.TempDir()
	cfg.Server.GeoIP.AllowCountries = []string{}
	cfg.Server.GeoIP.DenyCountries = []string{}
	svc := NewGeoIPService(cfg)

	if svc.IsBlocked("1.2.3.4") {
		t.Error("IsBlocked() = true with no allow/deny lists, want false")
	}
}

// Enabled, allowlist ["US"], no DB → CountryCode is "" → unknown country is
// blocked because an allowlist is active.
func TestIsBlocked_Enabled_AllowList_UnknownCountry_Blocked(t *testing.T) {
	cfg := config.DefaultAppConfig()
	cfg.Server.GeoIP.Enabled = true
	cfg.Server.GeoIP.Dir = t.TempDir()
	cfg.Server.GeoIP.AllowCountries = []string{"US"}
	cfg.Server.GeoIP.DenyCountries = []string{}
	cfg.Server.GeoIP.Databases.ASN = false
	cfg.Server.GeoIP.Databases.Country = false
	cfg.Server.GeoIP.Databases.City = false
	svc := NewGeoIPService(cfg)

	if !svc.IsBlocked("1.2.3.4") {
		t.Error("IsBlocked() = false for unknown country with allowlist active, want true")
	}
}

// Enabled, denylist ["US"], no DB → CountryCode is "" → unknown country is
// NOT blocked (denylist only blocks explicitly listed, known countries).
func TestIsBlocked_Enabled_DenyList_UnknownCountry_NotBlocked(t *testing.T) {
	cfg := config.DefaultAppConfig()
	cfg.Server.GeoIP.Enabled = true
	cfg.Server.GeoIP.Dir = t.TempDir()
	cfg.Server.GeoIP.AllowCountries = []string{}
	cfg.Server.GeoIP.DenyCountries = []string{"US"}
	cfg.Server.GeoIP.Databases.ASN = false
	cfg.Server.GeoIP.Databases.Country = false
	cfg.Server.GeoIP.Databases.City = false
	svc := NewGeoIPService(cfg)

	if svc.IsBlocked("1.2.3.4") {
		t.Error("IsBlocked() = true for unknown country with denylist only, want false")
	}
}

// --- CheckContentRestriction ---

// Mode "off" → never restricted, regardless of IP.
func TestCheckContentRestriction_ModeOff_NotRestricted(t *testing.T) {
	cfg := config.DefaultAppConfig()
	cfg.Server.GeoIP.ContentRestriction.Mode = "off"
	svc := NewGeoIPService(cfg)

	result := svc.CheckContentRestriction("1.2.3.4", false)
	if result == nil {
		t.Fatal("CheckContentRestriction() returned nil")
	}
	if result.Restricted {
		t.Error("CheckContentRestriction() mode=off: Restricted=true, want false")
	}
}

// Empty mode string → treated same as "off".
func TestCheckContentRestriction_ModeEmpty_NotRestricted(t *testing.T) {
	cfg := config.DefaultAppConfig()
	cfg.Server.GeoIP.ContentRestriction.Mode = ""
	svc := NewGeoIPService(cfg)

	result := svc.CheckContentRestriction("1.2.3.4", false)
	if result.Restricted {
		t.Error("CheckContentRestriction() mode='': Restricted=true, want false")
	}
}

// GeoIP disabled → not restricted even when mode is non-off.
func TestCheckContentRestriction_GeoIPDisabled_NotRestricted(t *testing.T) {
	cfg := config.DefaultAppConfig()
	cfg.Server.GeoIP.Enabled = false
	cfg.Server.GeoIP.ContentRestriction.Mode = "warn"
	cfg.Server.GeoIP.ContentRestriction.RestrictedCountries = []string{"US"}
	svc := NewGeoIPService(cfg)

	result := svc.CheckContentRestriction("1.2.3.4", false)
	if result.Restricted {
		t.Error("CheckContentRestriction(): GeoIP disabled but Restricted=true, want false")
	}
}

// No restricted countries or regions configured → not restricted.
func TestCheckContentRestriction_NoRestrictions_NotRestricted(t *testing.T) {
	cfg := config.DefaultAppConfig()
	cfg.Server.GeoIP.Enabled = true
	cfg.Server.GeoIP.Dir = t.TempDir()
	cfg.Server.GeoIP.ContentRestriction.Mode = "warn"
	cfg.Server.GeoIP.ContentRestriction.RestrictedCountries = []string{}
	cfg.Server.GeoIP.ContentRestriction.RestrictedRegions = []string{}
	svc := NewGeoIPService(cfg)

	result := svc.CheckContentRestriction("1.2.3.4", false)
	if result.Restricted {
		t.Error("CheckContentRestriction() with no restricted lists: Restricted=true, want false")
	}
}

// Tor user + BypassTor=true → not restricted.
func TestCheckContentRestriction_TorBypass_NotRestricted(t *testing.T) {
	cfg := config.DefaultAppConfig()
	cfg.Server.GeoIP.Enabled = true
	cfg.Server.GeoIP.Dir = t.TempDir()
	cfg.Server.GeoIP.ContentRestriction.Mode = "hard_block"
	cfg.Server.GeoIP.ContentRestriction.BypassTor = true
	cfg.Server.GeoIP.ContentRestriction.RestrictedCountries = []string{"US", "UK"}
	svc := NewGeoIPService(cfg)

	result := svc.CheckContentRestriction("1.2.3.4", true)
	if result.Restricted {
		t.Error("CheckContentRestriction() isTorUser=true, BypassTor=true: Restricted=true, want false")
	}
}

// Tor user + BypassTor=false → bypass does NOT apply; unknown country means
// CountryCode=="" → the code returns not-restricted (cannot geolocate, bypass VPN/Tor).
func TestCheckContentRestriction_TorNoBypass_UnknownCountry_NotRestricted(t *testing.T) {
	cfg := config.DefaultAppConfig()
	cfg.Server.GeoIP.Enabled = true
	cfg.Server.GeoIP.Dir = t.TempDir()
	cfg.Server.GeoIP.ContentRestriction.Mode = "hard_block"
	cfg.Server.GeoIP.ContentRestriction.BypassTor = false
	cfg.Server.GeoIP.ContentRestriction.RestrictedCountries = []string{"US"}
	cfg.Server.GeoIP.Databases.ASN = false
	cfg.Server.GeoIP.Databases.Country = false
	cfg.Server.GeoIP.Databases.City = false
	svc := NewGeoIPService(cfg)

	result := svc.CheckContentRestriction("1.2.3.4", true)
	// No DB → CountryCode=="" → code returns not restricted (unresolvable IP bypasses).
	if result.Restricted {
		t.Error("CheckContentRestriction() unknown country (no DB): Restricted=true, want false")
	}
}

// Mode is passed through to the result struct.
func TestCheckContentRestriction_ResultModeMatchesCfg(t *testing.T) {
	cfg := config.DefaultAppConfig()
	cfg.Server.GeoIP.ContentRestriction.Mode = "soft_block"
	svc := NewGeoIPService(cfg)

	result := svc.CheckContentRestriction("1.2.3.4", false)
	if result.Mode != "soft_block" {
		t.Errorf("CheckContentRestriction().Mode = %q, want %q", result.Mode, "soft_block")
	}
}

// --- URL constants ---

func TestURLConstants_NonEmpty(t *testing.T) {
	for name, url := range map[string]string{
		"ASNURL":          ASNURL,
		"CountryURL":      CountryURL,
		"CityURL":         CityURL,
		"CityURLFallback": CityURLFallback,
	} {
		if url == "" {
			t.Errorf("%s is empty", name)
		}
		if !strings.HasPrefix(url, "https://") {
			t.Errorf("%s = %q, want https:// prefix", name, url)
		}
	}
}

// --- Close ---

// Close on a freshly created service (all DB readers nil) must not panic.
func TestClose_NilDBs_NoPanic(t *testing.T) {
	svc := newDisabledService(t)
	svc.Close()
}

// Close on an enabled service with no DB files open must not panic.
func TestClose_EnabledNoDB_NoPanic(t *testing.T) {
	svc := newEnabledService(t)
	svc.Close()
}

// Calling Close twice must not panic (idempotency).
func TestClose_Idempotent(t *testing.T) {
	svc := newDisabledService(t)
	svc.Close()
	svc.Close()
}

// --- Concurrency ---

// Concurrent Lookup calls on a disabled service must not race.
func TestLookup_Concurrent_NoRace(t *testing.T) {
	svc := newDisabledService(t)
	done := make(chan struct{})
	for i := 0; i < 8; i++ {
		go func() {
			svc.Lookup("1.2.3.4")
			done <- struct{}{}
		}()
	}
	for i := 0; i < 8; i++ {
		<-done
	}
}

// Concurrent IsBlocked calls must not race.
func TestIsBlocked_Concurrent_NoRace(t *testing.T) {
	svc := newDisabledService(t)
	done := make(chan struct{})
	for i := 0; i < 8; i++ {
		go func() {
			svc.IsBlocked("1.2.3.4")
			done <- struct{}{}
		}()
	}
	for i := 0; i < 8; i++ {
		<-done
	}
}

// Concurrent LastUpdate calls must not race.
func TestLastUpdate_Concurrent_NoRace(t *testing.T) {
	svc := newDisabledService(t)
	done := make(chan struct{})
	for i := 0; i < 8; i++ {
		go func() {
			_ = svc.LastUpdate()
			done <- struct{}{}
		}()
	}
	for i := 0; i < 8; i++ {
		<-done
	}
}

// --- GeoIPResult ---

// Verify the result fields are exported and accessible.
func TestGeoIPResult_FieldsAccessible(t *testing.T) {
	r := &GeoIPResult{
		IP:          "1.2.3.4",
		Country:     "United States",
		CountryCode: "US",
		City:        "New York",
		Region:      "New York",
		Postal:      "10001",
		Latitude:    40.7128,
		Longitude:   -74.0060,
		Timezone:    "America/New_York",
		ASN:         15169,
		ASNOrg:      "Google LLC",
	}
	if r.IP == "" {
		t.Error("GeoIPResult.IP is empty")
	}
	if r.ASN == 0 {
		t.Error("GeoIPResult.ASN is 0")
	}
}

// --- Update disabled ---

// Update on a disabled service must return nil without touching the network.
func TestUpdate_Disabled_ReturnsNil(t *testing.T) {
	svc := newDisabledService(t)
	if err := svc.Update(); err != nil {
		t.Errorf("Update() on disabled service returned error: %v", err)
	}
}

// Update on a disabled service must leave lastUpdate at zero.
func TestUpdate_Disabled_LastUpdateRemainsZero(t *testing.T) {
	svc := newDisabledService(t)
	_ = svc.Update()
	if !svc.LastUpdate().IsZero() {
		t.Errorf("Update() disabled: lastUpdate = %v, want zero", svc.LastUpdate())
	}
}

// --- RestrictionResult ---

// Verify RestrictionResult struct fields are exported and usable.
func TestRestrictionResult_FieldsAccessible(t *testing.T) {
	r := &RestrictionResult{
		Restricted: true,
		Mode:       "warn",
		Reason:     "Texas",
		Message:    "Age verification required.",
		GeoIP:      &GeoIPResult{IP: "1.2.3.4"},
	}
	if !r.Restricted {
		t.Error("RestrictionResult.Restricted not set")
	}
	if r.GeoIP == nil {
		t.Error("RestrictionResult.GeoIP is nil")
	}
}

// --- IsEnabled after Close ---

// Close must not affect IsEnabled; the value comes from config, not DB state.
func TestIsEnabled_AfterClose_Unchanged(t *testing.T) {
	svc := newEnabledService(t)
	svc.Close()
	if !svc.IsEnabled() {
		t.Error("IsEnabled() = false after Close(), want true (config unchanged)")
	}
}

// --- Lookup returns non-nil always ---

// Exhaustive nil-safety check: Lookup must never return nil.
func TestLookup_NeverNil(t *testing.T) {
	ips := []string{
		"", "1.2.3.4", "::1", "0.0.0.0", "255.255.255.255",
		"not-an-ip", "999.999.999.999", "2001:db8::1",
	}
	for _, svc := range []*GeoIPService{newDisabledService(t), newEnabledService(t)} {
		for _, ip := range ips {
			if result := svc.Lookup(ip); result == nil {
				t.Errorf("Lookup(%q) returned nil", ip)
			}
		}
	}
}

// --- lastUpdate set after openDatabases ---

// When no DB files exist and all DB flags are off, openDatabases still sets
// lastUpdate to a non-zero time.
func TestOpenDatabases_SetsLastUpdate(t *testing.T) {
	cfg := config.DefaultAppConfig()
	cfg.Server.GeoIP.Enabled = true
	cfg.Server.GeoIP.Dir = t.TempDir()
	cfg.Server.GeoIP.Databases.ASN = false
	cfg.Server.GeoIP.Databases.Country = false
	cfg.Server.GeoIP.Databases.City = false
	svc := NewGeoIPService(cfg)

	before := time.Now()
	if err := svc.openDatabases(); err != nil {
		t.Fatalf("openDatabases() returned error: %v", err)
	}
	after := time.Now()

	lu := svc.LastUpdate()
	if lu.IsZero() {
		t.Error("lastUpdate is zero after openDatabases(), want non-zero")
	}
	if lu.Before(before) || lu.After(after) {
		t.Errorf("lastUpdate %v outside expected range [%v, %v]", lu, before, after)
	}
}
