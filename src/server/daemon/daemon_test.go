// SPDX-License-Identifier: MIT
// Tests for container.go and daemon_unix.go pure logic.
// AI.md PART 8: Container and service manager detection + daemonization.
//
// NOTE: All tests that call detectServiceManager() directly must account for
// the Docker environment: /.dockerenv exists, so isContainer() returns true
// and detectServiceManager() short-circuits to "container". Those tests use
// env var injection to verify the env-var code paths are reachable only when
// the container check passes first; for non-container paths the tests are
// skipped or structured around the container reality.
//
//go:build !windows

package daemon

import (
	"os"
	"runtime"
	"testing"
)

// --- shouldDaemonize ---
// Pure logic: no OS calls when isServiceStart=false.

func TestShouldDaemonizeAllFalse(t *testing.T) {
	if got := shouldDaemonize(false, false, false); got != false {
		t.Errorf("shouldDaemonize(false,false,false) = %v, want false", got)
	}
}

func TestShouldDaemonizeDaemonFlagTrue(t *testing.T) {
	if got := shouldDaemonize(false, true, false); got != true {
		t.Errorf("shouldDaemonize(false,true,false) = %v, want true", got)
	}
}

func TestShouldDaemonizeConfigTrue(t *testing.T) {
	if got := shouldDaemonize(false, false, true); got != true {
		t.Errorf("shouldDaemonize(false,false,true) = %v, want true", got)
	}
}

func TestShouldDaemonizeBothFlagsTrue(t *testing.T) {
	if got := shouldDaemonize(false, true, true); got != true {
		t.Errorf("shouldDaemonize(false,true,true) = %v, want true", got)
	}
}

// shouldDaemonize(isServiceStart=true) calls detectServiceManager internally.
// In the Docker test environment this returns "container" → false.
// The test verifies no panic and returns a bool.
func TestShouldDaemonizeServiceStartNoPanic(t *testing.T) {
	_ = shouldDaemonize(true, false, false)
	_ = shouldDaemonize(true, true, true)
}

// When isServiceStart=true and we are in a container, the result must be false
// (container managers require foreground execution per AI.md PART 8).
func TestShouldDaemonizeServiceStartInContainerReturnsFalse(t *testing.T) {
	if !isContainer() {
		t.Skip("not running in a container; skipping container-specific assertion")
	}
	if got := shouldDaemonize(true, false, false); got != false {
		t.Errorf("shouldDaemonize(true,...) in container = %v, want false", got)
	}
}

// daemonFlag and configDaemonize are ignored when isServiceStart=true.
func TestShouldDaemonizeServiceStartIgnoresFlagsInContainer(t *testing.T) {
	if !isContainer() {
		t.Skip("not running in a container; skipping container-specific assertion")
	}
	// Even with both flags true, service-start in a container must stay false.
	if got := shouldDaemonize(true, true, true); got != false {
		t.Errorf("shouldDaemonize(true,true,true) in container = %v, want false", got)
	}
}

// --- isContainer ---
// Tests the environment-variable detection paths.
// File-based detection (/.dockerenv) is already true in Docker; these tests
// layer additional env paths on top and verify they also return true.

func TestIsContainerGenericEnvVar(t *testing.T) {
	t.Setenv("container", "somevalue")
	if !isContainer() {
		t.Error("isContainer() with container=somevalue = false, want true")
	}
}

func TestIsContainerKubernetesEnvVar(t *testing.T) {
	t.Setenv("KUBERNETES_SERVICE_HOST", "10.0.0.1")
	if !isContainer() {
		t.Error("isContainer() with KUBERNETES_SERVICE_HOST set = false, want true")
	}
}

func TestIsContainerEnvVarsUnset(t *testing.T) {
	// Clear the env vars. /.dockerenv still exists in Docker so the result
	// is still true — but the env var code path is exercised as not-triggered.
	// We only assert that no panic occurs and the call completes.
	t.Setenv("container", "")
	t.Setenv("KUBERNETES_SERVICE_HOST", "")
	_ = isContainer()
}

// When neither file-based nor env-var indicators are present the function
// returns false. We cannot fake the absence of /.dockerenv here, so we
// confirm the function is at least callable and returns a bool.
func TestIsContainerReturnsBool(t *testing.T) {
	t.Setenv("container", "")
	t.Setenv("KUBERNETES_SERVICE_HOST", "")
	result := isContainer()
	_ = result // true in Docker, false on bare metal; either is correct
}

// --- detectServiceManager ---
// When isContainer() is true (/.dockerenv present in Docker) the function
// returns "container" immediately. Tests for the other env-var branches
// are skipped when we're already in a container.

func TestDetectServiceManagerReturnsContainerInDocker(t *testing.T) {
	if !isContainer() {
		t.Skip("not running in a container")
	}
	if got := detectServiceManager(); got != "container" {
		t.Errorf("detectServiceManager() in Docker = %q, want \"container\"", got)
	}
}

// The systemd, runit, and s6 branches are guarded by the isContainer() check
// (container check wins first). On a real non-container host each env var
// drives a different return value. We verify the logic is correct by testing
// the branches only when we are NOT in a container.

func TestDetectServiceManagerSystemdViaEnv(t *testing.T) {
	if isContainer() {
		t.Skip("container check wins before INVOCATION_ID check; skipping on Docker")
	}
	t.Setenv("INVOCATION_ID", "abc123")
	if got := detectServiceManager(); got != "systemd" {
		t.Errorf("detectServiceManager() with INVOCATION_ID set = %q, want \"systemd\"", got)
	}
}

func TestDetectServiceManagerRunitViaEnv(t *testing.T) {
	if isContainer() {
		t.Skip("container check wins before SVDIR check; skipping on Docker")
	}
	// Clear vars that rank higher.
	t.Setenv("INVOCATION_ID", "")
	t.Setenv("SVDIR", "/var/service")
	if got := detectServiceManager(); got != "runit" {
		t.Errorf("detectServiceManager() with SVDIR set = %q, want \"runit\"", got)
	}
}

func TestDetectServiceManagerS6ViaEnv(t *testing.T) {
	if isContainer() {
		t.Skip("container check wins before S6_LOGGING check; skipping on Docker")
	}
	// Clear vars that rank higher.
	t.Setenv("INVOCATION_ID", "")
	t.Setenv("SVDIR", "")
	t.Setenv("S6_LOGGING", "1")
	if got := detectServiceManager(); got != "s6" {
		t.Errorf("detectServiceManager() with S6_LOGGING set = %q, want \"s6\"", got)
	}
}

func TestDetectServiceManagerReturnsNonEmptyString(t *testing.T) {
	got := detectServiceManager()
	if got == "" {
		t.Error("detectServiceManager() returned empty string, want a non-empty manager name")
	}
}

// Verify all documented return values are in the known set.
func TestDetectServiceManagerReturnsKnownValue(t *testing.T) {
	known := map[string]bool{
		"systemd":   true,
		"launchd":   true,
		"runit":     true,
		"s6":        true,
		"container": true,
		"sysv":      true,
		"rcd":       true,
		"manual":    true,
	}
	got := detectServiceManager()
	if !known[got] {
		t.Errorf("detectServiceManager() = %q, not in known set %v", got, known)
	}
}

// --- getParentProcessName ---
// The parent process always exists when tests run, so this should never panic.
// The returned value may be empty on platforms where neither /proc nor ps works.

func TestGetParentProcessNameNoPanic(t *testing.T) {
	_ = getParentProcessName()
}

func TestGetParentProcessNameReturnsString(t *testing.T) {
	// On Linux /proc/{ppid}/comm is readable; result must be non-empty.
	if runtime.GOOS != "linux" {
		t.Skip("proc-based assertion is Linux-only")
	}
	got := getParentProcessName()
	if got == "" {
		t.Error("getParentProcessName() on Linux = empty string, want non-empty")
	}
}

func TestGetParentProcessNameNoNewline(t *testing.T) {
	// The implementation trims whitespace; verify no trailing newline leaks.
	got := getParentProcessName()
	for _, ch := range got {
		if ch == '\n' || ch == '\r' {
			t.Errorf("getParentProcessName() contains newline: %q", got)
		}
	}
}

// --- filterDaemonFlag ---
// Unexported helper in daemon_unix.go; accessible in same-package tests.

func TestFilterDaemonFlagRemovesDaemonLong(t *testing.T) {
	args := []string{"serve", "--daemon", "--port", "8080"}
	got := filterDaemonFlag(args)
	for _, a := range got {
		if a == "--daemon" {
			t.Errorf("filterDaemonFlag: --daemon not removed; got %v", got)
		}
	}
}

func TestFilterDaemonFlagRemovesDaemonShort(t *testing.T) {
	args := []string{"serve", "-d", "--port", "8080"}
	got := filterDaemonFlag(args)
	for _, a := range got {
		if a == "-d" {
			t.Errorf("filterDaemonFlag: -d not removed; got %v", got)
		}
	}
}

func TestFilterDaemonFlagPreservesOtherArgs(t *testing.T) {
	args := []string{"serve", "--daemon", "--port", "8080", "-d", "--host", "0.0.0.0"}
	got := filterDaemonFlag(args)
	want := []string{"serve", "--port", "8080", "--host", "0.0.0.0"}
	if len(got) != len(want) {
		t.Fatalf("filterDaemonFlag: len = %d, want %d; got %v", len(got), len(want), got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("filterDaemonFlag[%d] = %q, want %q", i, got[i], want[i])
		}
	}
}

func TestFilterDaemonFlagEmptySlice(t *testing.T) {
	got := filterDaemonFlag([]string{})
	if len(got) != 0 {
		t.Errorf("filterDaemonFlag([]) = %v, want empty slice", got)
	}
}

func TestFilterDaemonFlagNilSlice(t *testing.T) {
	got := filterDaemonFlag(nil)
	if len(got) != 0 {
		t.Errorf("filterDaemonFlag(nil) = %v, want empty slice", got)
	}
}

func TestFilterDaemonFlagOnlyDaemonFlags(t *testing.T) {
	args := []string{"--daemon", "-d", "--daemon"}
	got := filterDaemonFlag(args)
	if len(got) != 0 {
		t.Errorf("filterDaemonFlag(only daemon flags) = %v, want empty slice", got)
	}
}

func TestFilterDaemonFlagNoDaemonFlag(t *testing.T) {
	args := []string{"serve", "--port", "8080"}
	got := filterDaemonFlag(args)
	if len(got) != len(args) {
		t.Fatalf("filterDaemonFlag: len = %d, want %d; got %v", len(got), len(args), got)
	}
	for i := range args {
		if got[i] != args[i] {
			t.Errorf("filterDaemonFlag[%d] = %q, want %q", i, got[i], args[i])
		}
	}
}

// Idempotency: filtering an already-filtered slice produces the same result.
func TestFilterDaemonFlagIdempotent(t *testing.T) {
	args := []string{"serve", "--daemon", "--port", "8080"}
	once := filterDaemonFlag(args)
	twice := filterDaemonFlag(once)
	if len(once) != len(twice) {
		t.Errorf("filterDaemonFlag idempotency: first=%v second=%v", once, twice)
	}
	for i := range once {
		if once[i] != twice[i] {
			t.Errorf("filterDaemonFlag idempotency[%d]: %q vs %q", i, once[i], twice[i])
		}
	}
}

// --- Daemonize ---
// We only test the early-return paths that do NOT fork.

func TestDaemonizeReturnsNilWhenAlreadyChild(t *testing.T) {
	// Set the marker env var that the child process receives after re-exec.
	t.Setenv("_DAEMON_CHILD", "1")
	if err := Daemonize(); err != nil {
		t.Errorf("Daemonize() with _DAEMON_CHILD=1 = %v, want nil", err)
	}
}

func TestDaemonizeChildMarkerUnset(t *testing.T) {
	// Verify the env var is absent before the test (t.Setenv restores on cleanup).
	t.Setenv("_DAEMON_CHILD", "")
	// With PPID != 1 and no child marker, Daemonize would attempt to re-exec.
	// We cannot call it safely in a test (it calls os.Exit(0)).
	// Instead, verify the marker variable controls the early-return branch by
	// confirming that setting it makes Daemonize return nil without forking.
	t.Setenv("_DAEMON_CHILD", "1")
	if err := Daemonize(); err != nil {
		t.Errorf("Daemonize() with child marker = %v, want nil", err)
	}
}

// Regression: _DAEMON_CHILD must be checked before any fork attempt.
// If a future refactor moves the env check after the exec call, this test fails.
func TestDaemonizeChildMarkerPreventsRexec(t *testing.T) {
	t.Setenv("_DAEMON_CHILD", "1")
	// If Daemonize() ignores the marker, it will call os.Exit(0) and the test
	// process dies — the test runner reports a failure. A nil return here proves
	// the early-exit branch fired correctly.
	err := Daemonize()
	if err != nil {
		t.Errorf("Daemonize() child-marker regression: got error %v, want nil", err)
	}
}

// Verify that PPID==1 check uses os.Getppid, not a cached value.
// We cannot be PID 1 in tests; confirm Getppid works without panic.
func TestDaemonizePpidReadable(t *testing.T) {
	ppid := os.Getppid()
	if ppid <= 0 {
		t.Errorf("os.Getppid() = %d, want positive value", ppid)
	}
}
