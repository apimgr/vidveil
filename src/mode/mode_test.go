// SPDX-License-Identifier: MIT
package mode

import (
	"strings"
	"testing"
)

// resetState restores global vars to their zero values between test cases.
func resetState(t *testing.T) {
	t.Helper()
	t.Cleanup(func() {
		currentMode = Production
		debugEnabled = false
	})
}

// --- AppMode type and constants ---

func TestAppModeConstants(t *testing.T) {
	if Production != AppMode(0) {
		t.Errorf("Production = %d, want 0", Production)
	}
	if Development != AppMode(1) {
		t.Errorf("Development = %d, want 1", Development)
	}
}

// --- AppMode.String ---

func TestAppModeString(t *testing.T) {
	tests := []struct {
		mode AppMode
		want string
	}{
		{Production, "production"},
		{Development, "development"},
		// Any unrecognised value falls through to the default branch.
		{AppMode(99), "production"},
	}

	for _, tt := range tests {
		t.Run(tt.want, func(t *testing.T) {
			if got := tt.mode.String(); got != tt.want {
				t.Errorf("AppMode(%d).String() = %q, want %q", tt.mode, got, tt.want)
			}
		})
	}
}

// --- SetAppMode ---

func TestSetAppModeAcceptsDevVariants(t *testing.T) {
	inputs := []string{"dev", "DEV", "Dev", "development", "DEVELOPMENT", "Development"}
	for _, input := range inputs {
		t.Run(input, func(t *testing.T) {
			resetState(t)
			SetAppMode(input)
			if currentMode != Development {
				t.Errorf("SetAppMode(%q): currentMode = %v, want Development", input, currentMode)
			}
		})
	}
}

func TestSetAppModeFallsBackToProductionForUnknownValues(t *testing.T) {
	inputs := []string{"prod", "production", "PRODUCTION", "staging", "release", "", "   "}
	for _, input := range inputs {
		t.Run(input, func(t *testing.T) {
			resetState(t)
			// First set to dev so we can confirm it was overwritten.
			currentMode = Development
			SetAppMode(input)
			if currentMode != Production {
				t.Errorf("SetAppMode(%q): currentMode = %v, want Production", input, currentMode)
			}
		})
	}
}

func TestSetAppModeInvokesProfilingUpdate(t *testing.T) {
	// Verify SetAppMode does not panic (it calls updateProfilingSettings internally).
	resetState(t)
	SetDebug(true)
	SetAppMode("development")
	SetAppMode("production")
}

// --- SetDebug ---

func TestSetDebugEnablesDebug(t *testing.T) {
	resetState(t)
	SetDebug(true)
	if !debugEnabled {
		t.Error("SetDebug(true): debugEnabled = false, want true")
	}
}

func TestSetDebugDisablesDebug(t *testing.T) {
	resetState(t)
	debugEnabled = true
	SetDebug(false)
	if debugEnabled {
		t.Error("SetDebug(false): debugEnabled = true, want false")
	}
}

func TestSetDebugIdempotent(t *testing.T) {
	resetState(t)
	SetDebug(true)
	SetDebug(true)
	if !debugEnabled {
		t.Error("SetDebug(true) twice: debugEnabled = false, want true")
	}
	SetDebug(false)
	SetDebug(false)
	if debugEnabled {
		t.Error("SetDebug(false) twice: debugEnabled = true, want false")
	}
}

// --- CurrentAppMode ---

func TestCurrentAppModeReturnsDefault(t *testing.T) {
	resetState(t)
	if got := CurrentAppMode(); got != Production {
		t.Errorf("CurrentAppMode() = %v, want Production", got)
	}
}

func TestCurrentAppModeReflectsSetAppMode(t *testing.T) {
	resetState(t)
	SetAppMode("dev")
	if got := CurrentAppMode(); got != Development {
		t.Errorf("CurrentAppMode() after SetAppMode(dev) = %v, want Development", got)
	}
}

// --- IsAppModeDevelopment / IsAppModeProduction ---

func TestIsAppModeDevelopmentAndProduction(t *testing.T) {
	tests := []struct {
		name     string
		mode     AppMode
		wantDev  bool
		wantProd bool
	}{
		{"production", Production, false, true},
		{"development", Development, true, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resetState(t)
			currentMode = tt.mode
			if got := IsAppModeDevelopment(); got != tt.wantDev {
				t.Errorf("IsAppModeDevelopment() = %v, want %v", got, tt.wantDev)
			}
			if got := IsAppModeProduction(); got != tt.wantProd {
				t.Errorf("IsAppModeProduction() = %v, want %v", got, tt.wantProd)
			}
		})
	}
}

// --- IsDebugEnabled ---

func TestIsDebugEnabledReflectsSetDebug(t *testing.T) {
	resetState(t)
	if IsDebugEnabled() {
		t.Error("IsDebugEnabled() before SetDebug = true, want false")
	}
	SetDebug(true)
	if !IsDebugEnabled() {
		t.Error("IsDebugEnabled() after SetDebug(true) = false, want true")
	}
}

// --- AppModeString ---

func TestAppModeStringVariants(t *testing.T) {
	tests := []struct {
		name  string
		mode  AppMode
		debug bool
		want  string
	}{
		{"prod no debug", Production, false, "production"},
		{"prod with debug", Production, true, "production [debugging]"},
		{"dev no debug", Development, false, "development"},
		{"dev with debug", Development, true, "development [debugging]"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resetState(t)
			currentMode = tt.mode
			debugEnabled = tt.debug
			if got := AppModeString(); got != tt.want {
				t.Errorf("AppModeString() = %q, want %q", got, tt.want)
			}
		})
	}
}

// --- SetAppModeFromEnv ---

func TestSetAppModeFromEnvSetsModFromMODEVar(t *testing.T) {
	resetState(t)
	t.Setenv("MODE", "development")
	t.Setenv("DEBUG", "")
	SetAppModeFromEnv()
	if currentMode != Development {
		t.Errorf("SetAppModeFromEnv with MODE=development: currentMode = %v, want Development", currentMode)
	}
}

func TestSetAppModeFromEnvIgnoresEmptyMODE(t *testing.T) {
	resetState(t)
	currentMode = Development
	t.Setenv("MODE", "")
	t.Setenv("DEBUG", "")
	SetAppModeFromEnv()
	// Empty MODE must not change the current mode.
	if currentMode != Development {
		t.Errorf("SetAppModeFromEnv with MODE='': currentMode = %v, want Development (unchanged)", currentMode)
	}
}

func TestSetAppModeFromEnvSetsDebugFromDEBUGVar(t *testing.T) {
	truthyValues := []string{"1", "true", "yes", "enabled", "on"}
	for _, v := range truthyValues {
		t.Run("DEBUG="+v, func(t *testing.T) {
			resetState(t)
			t.Setenv("MODE", "")
			t.Setenv("DEBUG", v)
			SetAppModeFromEnv()
			if !debugEnabled {
				t.Errorf("SetAppModeFromEnv with DEBUG=%q: debugEnabled = false, want true", v)
			}
		})
	}
}

func TestSetAppModeFromEnvExplicitFalsyDEBUGDisablesDebug(t *testing.T) {
	// PART 6: an explicitly set DEBUG (truthy OR falsy) always wins. A falsy
	// DEBUG must turn debug off even if it was previously on (e.g. via MODE=debug).
	falsyValues := []string{"0", "false", "no", "disabled", "off", ""}
	for _, v := range falsyValues {
		t.Run("DEBUG="+v, func(t *testing.T) {
			resetState(t)
			debugEnabled = true
			t.Setenv("MODE", "")
			t.Setenv("DEBUG", v)
			SetAppModeFromEnv()
			if debugEnabled {
				t.Errorf("SetAppModeFromEnv with explicit DEBUG=%q: debugEnabled = true, want false", v)
			}
		})
	}
}

func TestSetAppModeDevelVariant(t *testing.T) {
	resetState(t)
	SetAppMode("devel")
	if currentMode != Development {
		t.Errorf("SetAppMode(devel): currentMode = %v, want Development", currentMode)
	}
}

func TestSetAppModeDebugAliasEnablesDevelopmentAndDebug(t *testing.T) {
	resetState(t)
	SetAppMode("debug")
	if currentMode != Development {
		t.Errorf("SetAppMode(debug): currentMode = %v, want Development", currentMode)
	}
	if !debugEnabled {
		t.Error("SetAppMode(debug): debugEnabled = false, want true")
	}
}

func TestSetAppModeFromEnvDebugAliasOverriddenByExplicitDEBUG(t *testing.T) {
	// PART 6: MODE=debug DEBUG=false runs development mode with debug off.
	resetState(t)
	t.Setenv("MODE", "debug")
	t.Setenv("DEBUG", "false")
	SetAppModeFromEnv()
	if currentMode != Development {
		t.Errorf("MODE=debug DEBUG=false: currentMode = %v, want Development", currentMode)
	}
	if debugEnabled {
		t.Error("MODE=debug DEBUG=false: debugEnabled = true, want false")
	}
}

func TestSetAppModeFromEnvProductionMODE(t *testing.T) {
	resetState(t)
	currentMode = Development
	t.Setenv("MODE", "production")
	t.Setenv("DEBUG", "")
	SetAppModeFromEnv()
	if currentMode != Production {
		t.Errorf("SetAppModeFromEnv with MODE=production: currentMode = %v, want Production", currentMode)
	}
}

func TestSetAppModeFromEnvDevShorthand(t *testing.T) {
	resetState(t)
	t.Setenv("MODE", "dev")
	t.Setenv("DEBUG", "")
	SetAppModeFromEnv()
	if currentMode != Development {
		t.Errorf("SetAppModeFromEnv with MODE=dev: currentMode = %v, want Development", currentMode)
	}
}

// --- InitializeAppMode ---

func TestInitializeAppModeCLIModeFlagOverridesEnv(t *testing.T) {
	resetState(t)
	t.Setenv("MODE", "production")
	t.Setenv("DEBUG", "")
	InitializeAppMode("development", false)
	if currentMode != Development {
		t.Errorf("InitializeAppMode(development, false): currentMode = %v, want Development", currentMode)
	}
}

func TestInitializeAppModeCLIDebugFlagOverridesEnv(t *testing.T) {
	resetState(t)
	t.Setenv("MODE", "")
	t.Setenv("DEBUG", "false")
	InitializeAppMode("", true)
	if !debugEnabled {
		t.Error("InitializeAppMode('', true): debugEnabled = false, want true")
	}
}

func TestInitializeAppModeEmptyFlagsUsesEnv(t *testing.T) {
	resetState(t)
	t.Setenv("MODE", "dev")
	t.Setenv("DEBUG", "1")
	InitializeAppMode("", false)
	if currentMode != Development {
		t.Errorf("InitializeAppMode empty flags: currentMode = %v, want Development from env", currentMode)
	}
	if !debugEnabled {
		t.Error("InitializeAppMode empty flags: debugEnabled = false, want true from env")
	}
}

func TestInitializeAppModeEmptyFlagsEmptyEnvDefaultsToProduction(t *testing.T) {
	resetState(t)
	t.Setenv("MODE", "")
	t.Setenv("DEBUG", "")
	InitializeAppMode("", false)
	if currentMode != Production {
		t.Errorf("InitializeAppMode no flags no env: currentMode = %v, want Production", currentMode)
	}
	if debugEnabled {
		t.Error("InitializeAppMode no flags no env: debugEnabled = true, want false")
	}
}

func TestInitializeAppModeProdFlagOverridesDevEnv(t *testing.T) {
	resetState(t)
	t.Setenv("MODE", "development")
	t.Setenv("DEBUG", "")
	InitializeAppMode("production", false)
	if currentMode != Production {
		t.Errorf("InitializeAppMode(production, false) with MODE=development: currentMode = %v, want Production", currentMode)
	}
}

// --- ConsoleIcon ---

func TestConsoleIconProduction(t *testing.T) {
	resetState(t)
	currentMode = Production
	if got := ConsoleIcon(); got != "🔒" {
		t.Errorf("ConsoleIcon() in production = %q, want 🔒", got)
	}
}

func TestConsoleIconDevelopment(t *testing.T) {
	resetState(t)
	currentMode = Development
	if got := ConsoleIcon(); got != "🔧" {
		t.Errorf("ConsoleIcon() in development = %q, want 🔧", got)
	}
}

// --- ConsoleModeMessage ---

func TestConsoleModeMessageFormat(t *testing.T) {
	tests := []struct {
		name       string
		mode       AppMode
		debug      bool
		wantPrefix string
		wantSuffix string
	}{
		{"prod no debug", Production, false, "🔒", "production"},
		{"prod with debug", Production, true, "🔒", "production [debugging]"},
		{"dev no debug", Development, false, "🔧", "development"},
		{"dev with debug", Development, true, "🔧", "development [debugging]"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resetState(t)
			currentMode = tt.mode
			debugEnabled = tt.debug
			msg := ConsoleModeMessage()
			if !strings.HasPrefix(msg, tt.wantPrefix) {
				t.Errorf("ConsoleModeMessage() = %q, want prefix %q", msg, tt.wantPrefix)
			}
			if !strings.HasSuffix(msg, tt.wantSuffix) {
				t.Errorf("ConsoleModeMessage() = %q, want suffix %q", msg, tt.wantSuffix)
			}
			if !strings.Contains(msg, "Running in mode:") {
				t.Errorf("ConsoleModeMessage() = %q, missing 'Running in mode:'", msg)
			}
		})
	}
}

// --- ShouldLogVerbose ---

func TestShouldLogVerbose(t *testing.T) {
	tests := []struct {
		name  string
		mode  AppMode
		debug bool
		want  bool
	}{
		{"prod no debug", Production, false, false},
		{"prod with debug", Production, true, true},
		{"dev no debug", Development, false, true},
		{"dev with debug", Development, true, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resetState(t)
			currentMode = tt.mode
			debugEnabled = tt.debug
			if got := ShouldLogVerbose(); got != tt.want {
				t.Errorf("ShouldLogVerbose() = %v, want %v", got, tt.want)
			}
		})
	}
}

// --- ShouldCacheTemplates ---

func TestShouldCacheTemplates(t *testing.T) {
	tests := []struct {
		name  string
		mode  AppMode
		debug bool
		want  bool
	}{
		{"prod no debug", Production, false, true},
		{"prod with debug", Production, true, false},
		{"dev no debug", Development, false, false},
		{"dev with debug", Development, true, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resetState(t)
			currentMode = tt.mode
			debugEnabled = tt.debug
			if got := ShouldCacheTemplates(); got != tt.want {
				t.Errorf("ShouldCacheTemplates() = %v, want %v", got, tt.want)
			}
		})
	}
}

// --- ShouldCacheStatic ---

func TestShouldCacheStatic(t *testing.T) {
	tests := []struct {
		name  string
		mode  AppMode
		debug bool
		want  bool
	}{
		{"prod no debug", Production, false, true},
		{"prod with debug", Production, true, false},
		{"dev no debug", Development, false, false},
		{"dev with debug", Development, true, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resetState(t)
			currentMode = tt.mode
			debugEnabled = tt.debug
			if got := ShouldCacheStatic(); got != tt.want {
				t.Errorf("ShouldCacheStatic() = %v, want %v", got, tt.want)
			}
		})
	}
}

// --- ShouldShowStackTraces ---

func TestShouldShowStackTraces(t *testing.T) {
	tests := []struct {
		name  string
		mode  AppMode
		debug bool
		want  bool
	}{
		{"prod no debug", Production, false, false},
		{"prod with debug", Production, true, true},
		{"dev no debug", Development, false, true},
		{"dev with debug", Development, true, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resetState(t)
			currentMode = tt.mode
			debugEnabled = tt.debug
			if got := ShouldShowStackTraces(); got != tt.want {
				t.Errorf("ShouldShowStackTraces() = %v, want %v", got, tt.want)
			}
		})
	}
}

// --- ShouldEnableDebugEndpoints ---

func TestShouldEnableDebugEndpoints(t *testing.T) {
	tests := []struct {
		name  string
		mode  AppMode
		debug bool
		want  bool
	}{
		{"prod no debug", Production, false, false},
		{"prod with debug", Production, true, true},
		{"dev no debug", Development, false, false},
		{"dev with debug", Development, true, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resetState(t)
			currentMode = tt.mode
			debugEnabled = tt.debug
			if got := ShouldEnableDebugEndpoints(); got != tt.want {
				t.Errorf("ShouldEnableDebugEndpoints() = %v, want %v", got, tt.want)
			}
		})
	}
}

// --- state isolation regression: ensure tests are truly independent ---

func TestGlobalStateIsIsolatedBetweenTests(t *testing.T) {
	resetState(t)
	SetAppMode("development")
	SetDebug(true)

	if currentMode != Development {
		t.Errorf("currentMode = %v, want Development", currentMode)
	}
	if !debugEnabled {
		t.Error("debugEnabled = false, want true")
	}
	// Cleanup registered by resetState will restore both values.
}

func TestGlobalStateDefaultsAfterPreviousTest(t *testing.T) {
	// This test runs after the one above; cleanup must have run already.
	if currentMode != Production {
		t.Errorf("currentMode not reset: got %v, want Production", currentMode)
	}
	if debugEnabled {
		t.Error("debugEnabled not reset: got true, want false")
	}
}
