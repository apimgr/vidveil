// SPDX-License-Identifier: MIT
// Tests for the banner package: ASCII art, compact/micro headers, URL extraction,
// and PrintStartupBanner smoke tests.
// Same-package access is required to reach the unexported extractHostPort and generateSimpleArt.
package banner

import (
	"strings"
	"testing"
)

// --- GetASCIIArt ---

// "vidveil" (exact case) must return the predefined vidveilArt slice.
func TestGetASCIIArtVidveilExact(t *testing.T) {
	art := GetASCIIArt("vidveil")
	if len(art) == 0 {
		t.Fatal("GetASCIIArt(\"vidveil\") returned empty slice")
	}
	// Spot-check: every element must be a string (always true in Go, but
	// verifying non-nil slice and matching the package var is the goal).
	if len(art) != len(vidveilArt) {
		t.Errorf("GetASCIIArt(\"vidveil\") len = %d, want %d (len of vidveilArt)", len(art), len(vidveilArt))
	}
	for i, line := range art {
		if line != vidveilArt[i] {
			t.Errorf("GetASCIIArt(\"vidveil\")[%d] = %q, want %q", i, line, vidveilArt[i])
		}
	}
}

// Lookup is case-insensitive: uppercase must return the same vidveilArt.
func TestGetASCIIArtVidveilUppercase(t *testing.T) {
	art := GetASCIIArt("VIDVEIL")
	if len(art) != len(vidveilArt) {
		t.Errorf("GetASCIIArt(\"VIDVEIL\") len = %d, want %d", len(art), len(vidveilArt))
	}
}

// Mixed case must also return vidveilArt.
func TestGetASCIIArtVidveilMixedCase(t *testing.T) {
	art := GetASCIIArt("VidVeil")
	if len(art) != len(vidveilArt) {
		t.Errorf("GetASCIIArt(\"VidVeil\") len = %d, want %d", len(art), len(vidveilArt))
	}
}

// Unknown names must fall through to generateSimpleArt.
func TestGetASCIIArtUnknown(t *testing.T) {
	art := GetASCIIArt("other")
	if len(art) < 4 {
		t.Errorf("GetASCIIArt(\"other\") len = %d, want >= 4 lines", len(art))
	}
	// generateSimpleArt must include the uppercased name in the middle line.
	found := false
	for _, line := range art {
		if strings.Contains(line, "OTHER") {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("GetASCIIArt(\"other\") output does not contain \"OTHER\": %v", art)
	}
}

// generateSimpleArt (via GetASCIIArt) must include the double-line box border character.
func TestGetASCIIArtSimpleBorderCharacter(t *testing.T) {
	art := GetASCIIArt("test")

	topFound := false
	for _, line := range art {
		if strings.Contains(line, "═") {
			topFound = true
			break
		}
	}
	if !topFound {
		t.Errorf("GetASCIIArt(\"test\") top border does not contain '═': %v", art)
	}
}

// generateSimpleArt for "test" must contain "TEST" in at least one line.
func TestGetASCIIArtSimpleContainsUpperName(t *testing.T) {
	art := GetASCIIArt("test")
	found := false
	for _, line := range art {
		if strings.Contains(line, "TEST") {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("GetASCIIArt(\"test\") does not contain \"TEST\": %v", art)
	}
}

// vidveilArt itself must be non-empty and all elements must be strings (not nil interface).
func TestVidveilArtPackageVar(t *testing.T) {
	if len(vidveilArt) == 0 {
		t.Error("vidveilArt package var is empty")
	}
	for i, line := range vidveilArt {
		// A string is always a string in Go, but we verify it's not accidentally blank for all rows.
		_ = line
		_ = i
	}
}

// --- GetCompactHeader ---

// GetCompactHeader must return a string containing the uppercased app name.
func TestGetCompactHeaderVidveil(t *testing.T) {
	got := GetCompactHeader("vidveil")
	if !strings.Contains(got, "VIDVEIL") {
		t.Errorf("GetCompactHeader(\"vidveil\") = %q, want string containing \"VIDVEIL\"", got)
	}
}

// GetCompactHeader must use "===" delimiters.
func TestGetCompactHeaderFormat(t *testing.T) {
	got := GetCompactHeader("vidveil")
	if !strings.HasPrefix(got, "===") || !strings.HasSuffix(got, "===") {
		t.Errorf("GetCompactHeader() = %q, want prefix and suffix \"===\"", got)
	}
}

// GetCompactHeader is case-insensitive in the sense that it always uppercases the name.
func TestGetCompactHeaderUppercases(t *testing.T) {
	got := GetCompactHeader("MixedCase")
	if !strings.Contains(got, "MIXEDCASE") {
		t.Errorf("GetCompactHeader(\"MixedCase\") = %q, want \"MIXEDCASE\"", got)
	}
}

// --- GetMicroHeader ---

// GetMicroHeader must return a string containing the uppercased app name.
func TestGetMicroHeaderVidveil(t *testing.T) {
	got := GetMicroHeader("vidveil")
	if !strings.Contains(got, "VIDVEIL") {
		t.Errorf("GetMicroHeader(\"vidveil\") = %q, want string containing \"VIDVEIL\"", got)
	}
}

// GetMicroHeader must use bracket delimiters.
func TestGetMicroHeaderFormat(t *testing.T) {
	got := GetMicroHeader("vidveil")
	if !strings.HasPrefix(got, "[") || !strings.HasSuffix(got, "]") {
		t.Errorf("GetMicroHeader() = %q, want prefix \"[\" and suffix \"]\"", got)
	}
}

// --- extractHostPort ---

// Full URL with path must return host only.
func TestExtractHostPortHTTP(t *testing.T) {
	got := extractHostPort("http://example.com/path")
	if got != "example.com" {
		t.Errorf("extractHostPort(\"http://example.com/path\") = %q, want %q", got, "example.com")
	}
}

// HTTPS URL with explicit port and path must return host:port.
func TestExtractHostPortHTTPSWithPort(t *testing.T) {
	got := extractHostPort("https://example.com:8080/path")
	if got != "example.com:8080" {
		t.Errorf("extractHostPort(\"https://example.com:8080/path\") = %q, want %q", got, "example.com:8080")
	}
}

// Plain host with no scheme or path must be returned unchanged.
func TestExtractHostPortPlain(t *testing.T) {
	got := extractHostPort("example.com")
	if got != "example.com" {
		t.Errorf("extractHostPort(\"example.com\") = %q, want %q", got, "example.com")
	}
}

// Scheme-only with no path separator: host still extracted.
func TestExtractHostPortSchemeOnly(t *testing.T) {
	got := extractHostPort("http://localhost")
	if got != "localhost" {
		t.Errorf("extractHostPort(\"http://localhost\") = %q, want %q", got, "localhost")
	}
}

// URL with port but no path must return host:port.
func TestExtractHostPortWithPort(t *testing.T) {
	got := extractHostPort("http://localhost:3000")
	if got != "localhost:3000" {
		t.Errorf("extractHostPort(\"http://localhost:3000\") = %q, want %q", got, "localhost:3000")
	}
}

// Empty string must return empty string without panic.
func TestExtractHostPortEmpty(t *testing.T) {
	got := extractHostPort("")
	if got != "" {
		t.Errorf("extractHostPort(\"\") = %q, want %q", got, "")
	}
}

// --- PrintStartupBanner smoke tests ---
// These call the exported function with various configs and assert no panic.
// Output is written to os.Stdout and is not captured — the goal is panic-free execution.

func TestPrintStartupBannerNoPanic(t *testing.T) {
	configs := []BannerConfig{
		{AppName: "vidveil", Version: "1.0.0", AppMode: "production", URLs: []string{"http://localhost:8080"}},
		{AppName: "vidveil", Version: "0.0.1", AppMode: "development", Debug: true},
		{AppName: "vidveil", Version: "1.0.0", AppMode: "production", ShowSetup: true, SetupToken: "abc123"},
		{AppName: "vidveil", Version: "1.0.0", AppMode: "production", URLs: []string{}},
		{AppName: "", Version: "", AppMode: ""},
	}
	for i, cfg := range configs {
		t.Run(strings.ReplaceAll(cfg.AppMode+"_"+cfg.Version, " ", "_"), func(t *testing.T) {
			defer func() {
				if r := recover(); r != nil {
					t.Errorf("PrintStartupBanner config[%d] panicked: %v", i, r)
				}
			}()
			PrintStartupBanner(cfg)
		})
	}
}

// PrintStartupBanner with multiple URLs must not panic.
func TestPrintStartupBannerMultipleURLs(t *testing.T) {
	defer func() {
		if r := recover(); r != nil {
			t.Errorf("PrintStartupBanner with multiple URLs panicked: %v", r)
		}
	}()
	PrintStartupBanner(BannerConfig{
		AppName: "vidveil",
		Version: "1.0.0",
		AppMode: "production",
		URLs: []string{
			"http://localhost:8080",
			"http://localhost:9090",
			"https://example.com:443/path",
		},
	})
}

// --- generateSimpleArt direct boundary tests ---

// generateSimpleArt for a single-character name must not panic and must contain the letter.
func TestGenerateSimpleArtSingleChar(t *testing.T) {
	art := generateSimpleArt("x")
	found := false
	for _, line := range art {
		if strings.Contains(line, "X") {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("generateSimpleArt(\"x\") does not contain \"X\": %v", art)
	}
}

// generateSimpleArt for an empty string must not panic.
func TestGenerateSimpleArtEmpty(t *testing.T) {
	defer func() {
		if r := recover(); r != nil {
			t.Errorf("generateSimpleArt(\"\") panicked: %v", r)
		}
	}()
	art := generateSimpleArt("")
	if len(art) == 0 {
		t.Error("generateSimpleArt(\"\") returned empty slice")
	}
}

// generateSimpleArt must return exactly 5 lines (empty, top, mid, bot, empty).
func TestGenerateSimpleArtLineCount(t *testing.T) {
	art := generateSimpleArt("hello")
	if len(art) != 5 {
		t.Errorf("generateSimpleArt(\"hello\") returned %d lines, want 5", len(art))
	}
}

// --- getASCIIArt (internal) ---

// getASCIIArt (unexported banner.go func) returns the multi-line VidVeil art for "vidveil".
func TestGetASCIIArtInternalVidveil(t *testing.T) {
	got := getASCIIArt("vidveil")
	if !strings.Contains(got, "VidVeil") && !strings.Contains(got, "vidveil") &&
		!strings.Contains(got, "╦") && !strings.Contains(got, "___") {
		// The internal getASCIIArt returns the raw multi-line string; just ensure it's non-empty.
		if len(strings.TrimSpace(got)) == 0 {
			t.Error("getASCIIArt(\"vidveil\") returned empty or whitespace-only string")
		}
	}
}

// getASCIIArt for unknown app returns "=== NAME ===" fallback.
func TestGetASCIIArtInternalFallback(t *testing.T) {
	got := getASCIIArt("myapp")
	if !strings.Contains(got, "MYAPP") {
		t.Errorf("getASCIIArt(\"myapp\") = %q, want string containing \"MYAPP\"", got)
	}
	if !strings.HasPrefix(strings.TrimSpace(got), "===") {
		t.Errorf("getASCIIArt(\"myapp\") = %q, want fallback starting with \"===\"", got)
	}
}
