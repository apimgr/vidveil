// SPDX-License-Identifier: MIT
// Tests for the tor package: construction, status constants, config accessors,
// routing logic, HTTP client creation, onion address generation, and uptime formatting.
// No Tor binary or network access is required — all tests are pure-logic.
package tor

import (
	"crypto/ed25519"
	"crypto/rand"
	"encoding/base32"
	"strings"
	"testing"
	"time"

	"github.com/apimgr/vidveil/src/config"
)

// newTestService builds a TorService backed by a temp dir with a nil logger.
// nil is safe because the logger is only used in goroutines Start() spawns,
// which we never call here.
func newTestService(t *testing.T) *TorService {
	t.Helper()
	return NewTorService(t.TempDir(), nil)
}

// boolPtr returns a pointer to the given bool — helper for ShouldUseTor tests.
func boolPtr(b bool) *bool { return &b }

// ---- NewTorService ----

func TestNewTorServiceNonNil(t *testing.T) {
	s := newTestService(t)
	if s == nil {
		t.Fatal("NewTorService returned nil")
	}
}

func TestNewTorServiceInitialStatusDisabled(t *testing.T) {
	s := newTestService(t)
	if s.status != TorServiceStatusDisabled {
		t.Errorf("status = %q, want %q", s.status, TorServiceStatusDisabled)
	}
}

func TestNewTorServiceDataDirHasTorSuffix(t *testing.T) {
	s := newTestService(t)
	if !strings.HasSuffix(s.dataDir, "tor") {
		t.Errorf("dataDir = %q, expected it to end with \"tor\"", s.dataDir)
	}
}

func TestNewTorServiceCfgNonNil(t *testing.T) {
	s := newTestService(t)
	if s.cfg == nil {
		t.Fatal("cfg is nil after NewTorService")
	}
}

// ---- Status constants ----

func TestStatusConstantValues(t *testing.T) {
	cases := []struct {
		got  TorServiceStatus
		want string
	}{
		{TorServiceStatusDisabled, "disabled"},
		{TorServiceStatusStarting, "starting"},
		{TorServiceStatusConnected, "connected"},
		{TorServiceStatusDisconnected, "disconnected"},
		{TorServiceStatusError, "error"},
		{TorServiceStatusNoTorBinary, "no_tor_binary"},
	}
	for _, c := range cases {
		if string(c.got) != c.want {
			t.Errorf("status constant = %q, want %q", c.got, c.want)
		}
	}
}

// ---- SetConfig / SetConfigDir ----

func TestSetConfigStoresTorConfig(t *testing.T) {
	s := newTestService(t)
	cfg := &config.TorConfig{UseNetwork: true}
	s.SetConfig(cfg)
	if !s.UseNetworkEnabled() {
		t.Error("UseNetworkEnabled should be true after SetConfig with UseNetwork=true")
	}
}

func TestSetConfigDirStoresDir(t *testing.T) {
	s := newTestService(t)
	dir := t.TempDir()
	s.SetConfigDir(dir)
	s.mu.RLock()
	got := s.configDir
	s.mu.RUnlock()
	if got != dir {
		t.Errorf("configDir = %q, want %q", got, dir)
	}
}

// ---- AllowUserPreference ----

func TestAllowUserPreferenceNilConfigReturnsFalse(t *testing.T) {
	s := newTestService(t)
	if s.AllowUserPreference() {
		t.Error("AllowUserPreference should be false when torConfig is nil")
	}
}

func TestAllowUserPreferenceFalseReturnsFalse(t *testing.T) {
	s := newTestService(t)
	s.SetConfig(&config.TorConfig{AllowUserPreference: false})
	if s.AllowUserPreference() {
		t.Error("AllowUserPreference should be false when config has AllowUserPreference=false")
	}
}

func TestAllowUserPreferenceTrueReturnsTrue(t *testing.T) {
	s := newTestService(t)
	s.SetConfig(&config.TorConfig{AllowUserPreference: true})
	if !s.AllowUserPreference() {
		t.Error("AllowUserPreference should be true when config has AllowUserPreference=true")
	}
}

// ---- ShouldUseTor ----

func TestShouldUseTorNilConfigReturnsFalse(t *testing.T) {
	s := newTestService(t)
	if s.ShouldUseTor(nil) {
		t.Error("ShouldUseTor should be false when torConfig is nil")
	}
}

func TestShouldUseTorUseNetworkFalseNoUserPrefReturnsFalse(t *testing.T) {
	s := newTestService(t)
	s.SetConfig(&config.TorConfig{UseNetwork: false, AllowUserPreference: false})
	if s.ShouldUseTor(nil) {
		t.Error("ShouldUseTor should be false: UseNetwork=false, AllowUserPreference=false, no user pref")
	}
}

func TestShouldUseTorUseNetworkTrueNoUserPrefReturnsTrue(t *testing.T) {
	s := newTestService(t)
	s.SetConfig(&config.TorConfig{UseNetwork: true, AllowUserPreference: false})
	if !s.ShouldUseTor(nil) {
		t.Error("ShouldUseTor should be true: UseNetwork=true, AllowUserPreference=false, no user pref")
	}
}

func TestShouldUseTorUserPrefIgnoredWhenNotAllowed(t *testing.T) {
	s := newTestService(t)
	s.SetConfig(&config.TorConfig{UseNetwork: false, AllowUserPreference: false})
	// Even with userPref=true, server setting must win
	if s.ShouldUseTor(boolPtr(true)) {
		t.Error("ShouldUseTor should be false: AllowUserPreference=false blocks user override")
	}
}

func TestShouldUseTorInheritServerWhenUserPrefNil(t *testing.T) {
	s := newTestService(t)
	s.SetConfig(&config.TorConfig{UseNetwork: false, AllowUserPreference: true})
	// user pref nil → inherit server (false)
	if s.ShouldUseTor(nil) {
		t.Error("ShouldUseTor should be false: AllowUserPreference=true, nil user pref, UseNetwork=false")
	}
}

func TestShouldUseTorUserPrefTrueOverridesServerFalse(t *testing.T) {
	s := newTestService(t)
	s.SetConfig(&config.TorConfig{UseNetwork: false, AllowUserPreference: true})
	if !s.ShouldUseTor(boolPtr(true)) {
		t.Error("ShouldUseTor should be true: user override=true beats UseNetwork=false")
	}
}

func TestShouldUseTorUserPrefFalseOverridesServerTrue(t *testing.T) {
	s := newTestService(t)
	s.SetConfig(&config.TorConfig{UseNetwork: true, AllowUserPreference: true})
	if s.ShouldUseTor(boolPtr(false)) {
		t.Error("ShouldUseTor should be false: user override=false beats UseNetwork=true")
	}
}

// ---- OutboundEnabled ----

func TestOutboundEnabledFalseWhenDialerNil(t *testing.T) {
	s := newTestService(t)
	if s.OutboundEnabled() {
		t.Error("OutboundEnabled should be false when dialer is nil (no Tor started)")
	}
}

// ---- UseNetworkEnabled ----

func TestUseNetworkEnabledNilConfigReturnsFalse(t *testing.T) {
	s := newTestService(t)
	if s.UseNetworkEnabled() {
		t.Error("UseNetworkEnabled should be false when torConfig is nil")
	}
}

func TestUseNetworkEnabledFalseReturnsFalse(t *testing.T) {
	s := newTestService(t)
	s.SetConfig(&config.TorConfig{UseNetwork: false})
	if s.UseNetworkEnabled() {
		t.Error("UseNetworkEnabled should be false when UseNetwork=false")
	}
}

func TestUseNetworkEnabledTrueReturnsTrue(t *testing.T) {
	s := newTestService(t)
	s.SetConfig(&config.TorConfig{UseNetwork: true})
	if !s.UseNetworkEnabled() {
		t.Error("UseNetworkEnabled should be true when UseNetwork=true")
	}
}

// ---- AllowUserIPForward ----

func TestAllowUserIPForwardNilConfigReturnsFalse(t *testing.T) {
	s := newTestService(t)
	if s.AllowUserIPForward() {
		t.Error("AllowUserIPForward should be false when torConfig is nil")
	}
}

func TestAllowUserIPForwardTrueReturnsTrue(t *testing.T) {
	s := newTestService(t)
	s.SetConfig(&config.TorConfig{AllowUserIPForward: true})
	if !s.AllowUserIPForward() {
		t.Error("AllowUserIPForward should be true when config has AllowUserIPForward=true")
	}
}

// ---- GetHTTPClient ----

func TestGetHTTPClientDirectIsNonNil(t *testing.T) {
	s := newTestService(t)
	c := s.GetHTTPClient(false)
	if c == nil {
		t.Fatal("GetHTTPClient(false) returned nil")
	}
}

func TestGetHTTPClientDirectTimeout30s(t *testing.T) {
	s := newTestService(t)
	c := s.GetHTTPClient(false)
	if c.Timeout != 30*time.Second {
		t.Errorf("direct client timeout = %v, want 30s", c.Timeout)
	}
}

func TestGetHTTPClientTorRequestedButNoDialerFallsBackToDirect(t *testing.T) {
	// When useTor=true but dialer is nil (Tor not started), should return a
	// direct client rather than panicking or returning nil.
	s := newTestService(t)
	c := s.GetHTTPClient(true)
	if c == nil {
		t.Fatal("GetHTTPClient(true) with nil dialer returned nil")
	}
	if c.Timeout != 30*time.Second {
		t.Errorf("fallback client timeout = %v, want 30s", c.Timeout)
	}
}

// ---- GetOnionAddress ----

func TestGetOnionAddressInitiallyEmpty(t *testing.T) {
	s := newTestService(t)
	if addr := s.GetOnionAddress(); addr != "" {
		t.Errorf("GetOnionAddress = %q, want empty initially", addr)
	}
}

// ---- GetStatus ----

func TestGetStatusInitiallyDisabled(t *testing.T) {
	s := newTestService(t)
	if s.GetStatus() != TorServiceStatusDisabled {
		t.Errorf("GetStatus = %q, want %q", s.GetStatus(), TorServiceStatusDisabled)
	}
}

// ---- IsEnabled ----

func TestIsEnabledFalseInitially(t *testing.T) {
	s := newTestService(t)
	if s.IsEnabled() {
		t.Error("IsEnabled should be false initially")
	}
}

func TestIsEnabledTrueWhenConnected(t *testing.T) {
	s := newTestService(t)
	s.status = TorServiceStatusConnected
	if !s.IsEnabled() {
		t.Error("IsEnabled should be true when status is connected")
	}
}

// ---- IsRunning ----

func TestIsRunningFalseInitially(t *testing.T) {
	s := newTestService(t)
	if s.IsRunning() {
		t.Error("IsRunning should be false initially (torInstance is nil)")
	}
}

// ---- IsStarting ----

func TestIsStartingFalseInitially(t *testing.T) {
	s := newTestService(t)
	if s.IsStarting() {
		t.Error("IsStarting should be false initially (status is disabled, not starting)")
	}
}

func TestIsStartingTrueWhenStarting(t *testing.T) {
	s := newTestService(t)
	s.status = TorServiceStatusStarting
	if !s.IsStarting() {
		t.Error("IsStarting should be true when status is starting")
	}
}

// ---- GetStatusString ----

// GetStatusString maps connected/starting/error/no_tor_binary to specific strings;
// everything else (including disabled/disconnected) maps to "disconnected".
func TestGetStatusStringDisabled(t *testing.T) {
	s := newTestService(t)
	s.status = TorServiceStatusDisabled
	if got := s.GetStatusString(); got != "disconnected" {
		t.Errorf("GetStatusString(disabled) = %q, want %q", got, "disconnected")
	}
}

func TestGetStatusStringStarting(t *testing.T) {
	s := newTestService(t)
	s.status = TorServiceStatusStarting
	if got := s.GetStatusString(); got != "starting" {
		t.Errorf("GetStatusString(starting) = %q, want %q", got, "starting")
	}
}

func TestGetStatusStringConnected(t *testing.T) {
	s := newTestService(t)
	s.status = TorServiceStatusConnected
	if got := s.GetStatusString(); got != "connected" {
		t.Errorf("GetStatusString(connected) = %q, want %q", got, "connected")
	}
}

func TestGetStatusStringDisconnected(t *testing.T) {
	s := newTestService(t)
	s.status = TorServiceStatusDisconnected
	if got := s.GetStatusString(); got != "disconnected" {
		t.Errorf("GetStatusString(disconnected) = %q, want %q", got, "disconnected")
	}
}

func TestGetStatusStringError(t *testing.T) {
	s := newTestService(t)
	s.status = TorServiceStatusError
	if got := s.GetStatusString(); got != "error" {
		t.Errorf("GetStatusString(error) = %q, want %q", got, "error")
	}
}

func TestGetStatusStringNoTorBinary(t *testing.T) {
	s := newTestService(t)
	s.status = TorServiceStatusNoTorBinary
	if got := s.GetStatusString(); got != "no-binary" {
		t.Errorf("GetStatusString(no_tor_binary) = %q, want %q", got, "no-binary")
	}
}

// ---- GetUptime ----

func TestGetUptimeNotConnectedReturnsZeroS(t *testing.T) {
	s := newTestService(t)
	// Default status is disabled — not connected
	if got := s.GetUptime(); got != "0s" {
		t.Errorf("GetUptime when not connected = %q, want %q", got, "0s")
	}
}

func TestGetUptimeConnectedReturnsHMS(t *testing.T) {
	s := newTestService(t)
	s.status = TorServiceStatusConnected
	s.startTime = time.Now()
	got := s.GetUptime()
	// Should be formatted as "Xh Ym Zs" (less than 24 h)
	if !strings.Contains(got, "h") || !strings.Contains(got, "m") {
		t.Errorf("GetUptime (connected, recent) = %q, expected \"Xh Ym Zs\" format", got)
	}
}

func TestGetUptimeConnected25HoursReturnsDayFormat(t *testing.T) {
	s := newTestService(t)
	s.status = TorServiceStatusConnected
	s.startTime = time.Now().Add(-25 * time.Hour)
	got := s.GetUptime()
	// 25 hours → "1d 1h 0m"
	if !strings.Contains(got, "d") {
		t.Errorf("GetUptime (25h) = %q, expected day format containing \"d\"", got)
	}
	if !strings.HasPrefix(got, "1d") {
		t.Errorf("GetUptime (25h) = %q, expected to start with \"1d\"", got)
	}
}

// ---- generateOnionAddress ----

func TestGenerateOnionAddressEndsWithOnion(t *testing.T) {
	pub, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatalf("ed25519.GenerateKey: %v", err)
	}
	s := newTestService(t)
	s.privateKey = priv
	s.publicKey = pub
	addr := s.generateOnionAddress()
	if !strings.HasSuffix(addr, ".onion") {
		t.Errorf("address %q does not end with .onion", addr)
	}
}

func TestGenerateOnionAddressCorrectLength(t *testing.T) {
	pub, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatalf("ed25519.GenerateKey: %v", err)
	}
	s := newTestService(t)
	s.privateKey = priv
	s.publicKey = pub
	addr := s.generateOnionAddress()
	// Tor v3 address: 56-char base32 + ".onion" = 62 chars total
	const wantLen = 62
	if len(addr) != wantLen {
		t.Errorf("onion address length = %d, want %d; addr = %q", len(addr), wantLen, addr)
	}
}

func TestGenerateOnionAddressBase32Alphabet(t *testing.T) {
	pub, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatalf("ed25519.GenerateKey: %v", err)
	}
	s := newTestService(t)
	s.privateKey = priv
	s.publicKey = pub
	addr := s.generateOnionAddress()
	// Strip ".onion" suffix — remainder must be valid lower-case base32 (no padding)
	encoded := strings.TrimSuffix(addr, ".onion")
	upper := strings.ToUpper(encoded)
	_, decodeErr := base32.StdEncoding.WithPadding(base32.NoPadding).DecodeString(upper)
	if decodeErr != nil {
		t.Errorf("onion address base32 part %q is not valid base32: %v", encoded, decodeErr)
	}
	// Must be lowercase only (no uppercase, no '=' padding)
	if encoded != strings.ToLower(encoded) {
		t.Errorf("onion address base32 part %q contains uppercase characters", encoded)
	}
	if strings.Contains(encoded, "=") {
		t.Errorf("onion address base32 part %q contains padding characters", encoded)
	}
}

func TestGenerateOnionAddressDeterministic(t *testing.T) {
	pub, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatalf("ed25519.GenerateKey: %v", err)
	}
	s := newTestService(t)
	s.privateKey = priv
	s.publicKey = pub
	addr1 := s.generateOnionAddress()
	addr2 := s.generateOnionAddress()
	if addr1 != addr2 {
		t.Errorf("generateOnionAddress is not deterministic: %q != %q", addr1, addr2)
	}
}

func TestGenerateOnionAddressDifferentKeysProduceDifferentAddresses(t *testing.T) {
	pub1, priv1, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatalf("ed25519.GenerateKey (1): %v", err)
	}
	pub2, priv2, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatalf("ed25519.GenerateKey (2): %v", err)
	}

	s1 := newTestService(t)
	s1.privateKey = priv1
	s1.publicKey = pub1

	s2 := newTestService(t)
	s2.privateKey = priv2
	s2.publicKey = pub2

	if s1.generateOnionAddress() == s2.generateOnionAddress() {
		t.Error("different key pairs produced the same .onion address")
	}
}

// ---- GenerateVanityAddress input validation ----

func TestGenerateVanityAddressTooLongPrefix(t *testing.T) {
	s := newTestService(t)
	err := s.GenerateVanityAddress("abcdefg")
	if err == nil {
		t.Error("expected error for prefix longer than 6 characters")
	}
}

func TestGenerateVanityAddressInvalidCharacter(t *testing.T) {
	s := newTestService(t)
	// '0', '1', '8', '9' are not in the base32 a-z 2-7 alphabet
	err := s.GenerateVanityAddress("abc0")
	if err == nil {
		t.Error("expected error for prefix with invalid base32 character '0'")
	}
}

func TestGenerateVanityAddressValidPrefixStartsGeneration(t *testing.T) {
	s := newTestService(t)
	err := s.GenerateVanityAddress("abc")
	if err != nil {
		t.Errorf("unexpected error for valid prefix: %v", err)
	}
	// Cancel immediately so the background goroutine does not outlive the test
	s.CancelVanityGeneration()
}

// ---- GetVanityStatus ----

func TestGetVanityStatusNilBeforeGeneration(t *testing.T) {
	s := newTestService(t)
	if s.GetVanityStatus() != nil {
		t.Error("GetVanityStatus should be nil before any generation is started")
	}
}
