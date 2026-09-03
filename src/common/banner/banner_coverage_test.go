// SPDX-License-Identifier: MIT
// Coverage tests for unexported banner printing functions.
// Same-package access (package banner) is required to call the unexported functions directly.
package banner

import "testing"

// TestPrintStartupBannerFull verifies that printStartupBannerFull does not panic,
// including the ShowSetup/SetupToken branch.
func TestPrintStartupBannerFull(t *testing.T) {
	defer func() {
		if r := recover(); r != nil {
			t.Errorf("printStartupBannerFull panicked: %v", r)
		}
	}()
	printStartupBannerFull(BannerConfig{
		AppName:    "vidveil",
		Version:    "1.0.0",
		AppMode:    "production",
		URLs:       []string{"http://localhost:8080"},
		ShowSetup:  true,
		SetupToken: "test-token-abc123",
	})
}

// TestPrintStartupBannerFullNoURLs verifies that printStartupBannerFull does not panic
// when no URLs are provided.
func TestPrintStartupBannerFullNoURLs(t *testing.T) {
	defer func() {
		if r := recover(); r != nil {
			t.Errorf("printStartupBannerFull (no URLs) panicked: %v", r)
		}
	}()
	printStartupBannerFull(BannerConfig{
		AppName: "vidveil",
		Version: "2.0.0",
		AppMode: "development",
		Debug:   true,
	})
}

// TestPrintStartupBannerCompactWithSetup verifies that printStartupBannerCompact does not
// panic when ShowSetup and SetupToken are both set.
func TestPrintStartupBannerCompactWithSetup(t *testing.T) {
	defer func() {
		if r := recover(); r != nil {
			t.Errorf("printStartupBannerCompact (with setup) panicked: %v", r)
		}
	}()
	printStartupBannerCompact(BannerConfig{
		AppName:    "vidveil",
		Version:    "1.0.0",
		AppMode:    "production",
		URLs:       []string{"http://localhost:8080", "https://example.com"},
		ShowSetup:  true,
		SetupToken: "setup-xyz",
	})
}

// TestPrintStartupBannerCompactNoSetup verifies that printStartupBannerCompact does not
// panic when ShowSetup is false.
func TestPrintStartupBannerCompactNoSetup(t *testing.T) {
	defer func() {
		if r := recover(); r != nil {
			t.Errorf("printStartupBannerCompact (no setup) panicked: %v", r)
		}
	}()
	printStartupBannerCompact(BannerConfig{
		AppName: "vidveil",
		Version: "1.0.0",
		AppMode: "development",
		URLs:    []string{"http://localhost:9090"},
	})
}

// TestPrintStartupBannerMinimalWithURLs verifies that printStartupBannerMinimal does not
// panic when URLs are provided.
func TestPrintStartupBannerMinimalWithURLs(t *testing.T) {
	defer func() {
		if r := recover(); r != nil {
			t.Errorf("printStartupBannerMinimal (with URLs) panicked: %v", r)
		}
	}()
	printStartupBannerMinimal(BannerConfig{
		AppName: "vidveil",
		Version: "1.0.0",
		URLs:    []string{"http://localhost:8080", "https://example.com:443/path"},
	})
}

// TestPrintStartupBannerMinimalNoURLs verifies that printStartupBannerMinimal does not
// panic when no URLs are provided.
func TestPrintStartupBannerMinimalNoURLs(t *testing.T) {
	defer func() {
		if r := recover(); r != nil {
			t.Errorf("printStartupBannerMinimal (no URLs) panicked: %v", r)
		}
	}()
	printStartupBannerMinimal(BannerConfig{
		AppName: "vidveil",
		Version: "1.0.0",
	})
}

// TestPrintStartupBannerMicroNoURLs verifies that printStartupBannerMicro does not panic
// when no URLs are present, exercising the AppName-only branch.
func TestPrintStartupBannerMicroNoURLs(t *testing.T) {
	defer func() {
		if r := recover(); r != nil {
			t.Errorf("printStartupBannerMicro (no URLs) panicked: %v", r)
		}
	}()
	printStartupBannerMicro(BannerConfig{
		AppName: "vidveil",
		Version: "1.0.0",
	})
}

// TestPrintStartupBannerMicroWithURLs verifies that printStartupBannerMicro does not panic
// when URLs are present, exercising the host:port branch.
func TestPrintStartupBannerMicroWithURLs(t *testing.T) {
	defer func() {
		if r := recover(); r != nil {
			t.Errorf("printStartupBannerMicro (with URLs) panicked: %v", r)
		}
	}()
	printStartupBannerMicro(BannerConfig{
		AppName: "vidveil",
		Version: "1.0.0",
		URLs:    []string{"http://localhost:8080", "https://example.com"},
	})
}

// TestPrintStartupBannerAppModeLineNoIcons exercises the else branch where useIcons=false,
// which prints "Mode: <appMode>" regardless of debug or appMode value.
func TestPrintStartupBannerAppModeLineNoIcons(t *testing.T) {
	defer func() {
		if r := recover(); r != nil {
			t.Errorf("printStartupBannerAppModeLine (useIcons=false) panicked: %v", r)
		}
	}()
	printStartupBannerAppModeLine("production", false, false)
	printStartupBannerAppModeLine("development", true, false)
}

// TestPrintStartupBannerAppModeLineIconsDebug exercises the BugIcon branch
// (useIcons=true, debug=true).
func TestPrintStartupBannerAppModeLineIconsDebug(t *testing.T) {
	defer func() {
		if r := recover(); r != nil {
			t.Errorf("printStartupBannerAppModeLine (debug=true) panicked: %v", r)
		}
	}()
	printStartupBannerAppModeLine("development", true, true)
}

// TestPrintStartupBannerAppModeLineIconsDevelopment exercises the WrenchIcon branch
// (useIcons=true, debug=false, appMode="development").
func TestPrintStartupBannerAppModeLineIconsDevelopment(t *testing.T) {
	defer func() {
		if r := recover(); r != nil {
			t.Errorf("printStartupBannerAppModeLine (development) panicked: %v", r)
		}
	}()
	printStartupBannerAppModeLine("development", false, true)
}

// TestPrintStartupBannerAppModeLineIconsProduction exercises the LockIcon branch
// (useIcons=true, debug=false, appMode="production").
func TestPrintStartupBannerAppModeLineIconsProduction(t *testing.T) {
	defer func() {
		if r := recover(); r != nil {
			t.Errorf("printStartupBannerAppModeLine (production) panicked: %v", r)
		}
	}()
	printStartupBannerAppModeLine("production", false, true)
}
