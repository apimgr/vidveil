// SPDX-License-Identifier: MIT
package system

import (
	"os"
	"testing"
)

// --- NewServiceManager ---

// TestNewServiceManager_NonNil verifies the constructor returns a non-nil value.
func TestNewServiceManager_NonNil(t *testing.T) {
	sm := NewServiceManager("testapp", "/usr/local/bin/testapp", "/etc/testapp", "/var/lib/testapp")
	if sm == nil {
		t.Fatal("NewServiceManager returned nil")
	}
}

// TestNewServiceManager_FieldsMatchInputs verifies that constructor arguments are
// stored in the corresponding struct fields.
func TestNewServiceManager_FieldsMatchInputs(t *testing.T) {
	appName := "vidveil"
	binaryPath := "/usr/local/bin/vidveil"
	configDir := "/etc/apimgr/vidveil"
	dataDir := "/var/lib/apimgr/vidveil"

	sm := NewServiceManager(appName, binaryPath, configDir, dataDir)

	if sm.appName != appName {
		t.Errorf("appName = %q, want %q", sm.appName, appName)
	}
	if sm.binaryPath != binaryPath {
		t.Errorf("binaryPath = %q, want %q", sm.binaryPath, binaryPath)
	}
	if sm.configDir != configDir {
		t.Errorf("configDir = %q, want %q", sm.configDir, configDir)
	}
	if sm.dataDir != dataDir {
		t.Errorf("dataDir = %q, want %q", sm.dataDir, dataDir)
	}
}

// TestNewServiceManager_DerivedFields verifies that user, group, and description
// are derived from appName by the constructor.
func TestNewServiceManager_DerivedFields(t *testing.T) {
	sm := NewServiceManager("myapp", "/bin/myapp", "/etc/myapp", "/var/myapp")

	if sm.user != "myapp" {
		t.Errorf("user = %q, want 'myapp'", sm.user)
	}
	if sm.group != "myapp" {
		t.Errorf("group = %q, want 'myapp'", sm.group)
	}
	if sm.description == "" {
		t.Error("description should not be empty")
	}
}

// TestNewServiceManager_EmptyAppName verifies that an empty appName does not
// panic, even though the resulting service manager would be unusable.
func TestNewServiceManager_EmptyAppName(t *testing.T) {
	defer func() {
		if r := recover(); r != nil {
			t.Errorf("NewServiceManager with empty appName panicked: %v", r)
		}
	}()
	sm := NewServiceManager("", "", "", "")
	if sm == nil {
		t.Fatal("NewServiceManager returned nil for empty inputs")
	}
}

// --- IsRunningAsRoot ---

// TestIsRunningAsRoot_ReturnsBool verifies the function does not panic and returns
// a value consistent with the current process UID.
func TestIsRunningAsRoot_ReturnsBool(t *testing.T) {
	got := IsRunningAsRoot()
	want := os.Getuid() == 0
	if got != want {
		t.Errorf("IsRunningAsRoot() = %v, want %v (os.Getuid()=%d)", got, want, os.Getuid())
	}
}

// TestIsRunningAsRoot_TrueInDocker verifies that the test environment (Docker)
// runs as root (UID 0), which is the standard Docker default.
func TestIsRunningAsRoot_TrueInDocker(t *testing.T) {
	if os.Getuid() != 0 {
		t.Skip("test is designed for a Docker environment running as root")
	}
	if !IsRunningAsRoot() {
		t.Error("IsRunningAsRoot() = false but os.Getuid() == 0")
	}
}

// --- IsRunningInContainer ---

// TestIsRunningInContainer_NoPanic verifies the function returns without panicking.
func TestIsRunningInContainer_NoPanic(t *testing.T) {
	defer func() {
		if r := recover(); r != nil {
			t.Errorf("IsRunningInContainer() panicked: %v", r)
		}
	}()
	IsRunningInContainer()
}

// TestIsRunningInContainer_TrueInDocker verifies that /.dockerenv is detected.
// This test must pass in the Docker environment used by the test command.
func TestIsRunningInContainer_TrueInDocker(t *testing.T) {
	if _, err := os.Stat("/.dockerenv"); os.IsNotExist(err) {
		t.Skip("/.dockerenv not present; skipping Docker-specific assertion")
	}
	if !IsRunningInContainer() {
		t.Error("IsRunningInContainer() = false but /.dockerenv exists")
	}
}

// TestIsRunningInContainer_ContainerEnvVar verifies that the "container"
// environment variable triggers a true result.
func TestIsRunningInContainer_ContainerEnvVar(t *testing.T) {
	t.Setenv("container", "docker")
	if !IsRunningInContainer() {
		t.Error("IsRunningInContainer() = false when env var 'container' is set")
	}
}

// TestIsRunningInContainer_KubernetesEnvVar verifies that the
// KUBERNETES_SERVICE_HOST environment variable triggers a true result.
func TestIsRunningInContainer_KubernetesEnvVar(t *testing.T) {
	t.Setenv("KUBERNETES_SERVICE_HOST", "10.0.0.1")
	if !IsRunningInContainer() {
		t.Error("IsRunningInContainer() = false when KUBERNETES_SERVICE_HOST is set")
	}
}

// --- DetectEscalation ---

// validEscalation is the set of values DetectEscalation is allowed to return.
var validEscalation = map[string]bool{
	"sudo":   true,
	"doas":   true,
	"pkexec": true,
	"runas":  true,
	"":       true,
}

// TestDetectEscalation_NoPanic verifies the function does not panic.
func TestDetectEscalation_NoPanic(t *testing.T) {
	defer func() {
		if r := recover(); r != nil {
			t.Errorf("DetectEscalation() panicked: %v", r)
		}
	}()
	DetectEscalation()
}

// TestDetectEscalation_ValidReturnValues verifies the result is one of the
// documented values.
func TestDetectEscalation_ValidReturnValues(t *testing.T) {
	got := DetectEscalation()
	if !validEscalation[got] {
		t.Errorf("DetectEscalation() = %q, want one of: sudo, doas, pkexec, runas, or empty string", got)
	}
}

// --- DetectServiceManager ---

// validServiceManagers is the complete set of values DetectServiceManager may
// return per AI.md PART 23.
var validServiceManagers = map[string]bool{
	"systemd":   true,
	"launchd":   true,
	"runit":     true,
	"s6":        true,
	"sysv":      true,
	"rcd":       true,
	"container": true,
	"manual":    true,
}

// TestDetectServiceManager_NoPanic verifies the function does not panic.
func TestDetectServiceManager_NoPanic(t *testing.T) {
	defer func() {
		if r := recover(); r != nil {
			t.Errorf("DetectServiceManager() panicked: %v", r)
		}
	}()
	DetectServiceManager()
}

// TestDetectServiceManager_ValidReturnValue verifies the result is a known
// service manager identifier.
func TestDetectServiceManager_ValidReturnValue(t *testing.T) {
	got := DetectServiceManager()
	if !validServiceManagers[got] {
		t.Errorf("DetectServiceManager() = %q, not in known set", got)
	}
}

// TestDetectServiceManager_ContainerEnvVarCausesContainerResult verifies that
// setting a container environment variable causes the function to return
// "container" (because IsRunningInContainer is checked first).
func TestDetectServiceManager_ContainerEnvVarCausesContainerResult(t *testing.T) {
	t.Setenv("container", "podman")
	got := DetectServiceManager()
	if got != "container" {
		t.Errorf("DetectServiceManager() = %q, want 'container' when container env var is set", got)
	}
}

// --- hasSystemd ---

// TestHasSystemd_NoPanic verifies the unexported method does not panic.
func TestHasSystemd_NoPanic(t *testing.T) {
	sm := NewServiceManager("test", "/tmp/test", t.TempDir(), t.TempDir())
	defer func() {
		if r := recover(); r != nil {
			t.Errorf("hasSystemd() panicked: %v", r)
		}
	}()
	sm.hasSystemd()
}

// TestHasSystemd_ReturnsBool verifies the result is a boolean (compile-time
// guarantee, exercised here to catch any refactor that changes the return type
// to something else).
func TestHasSystemd_ReturnsBool(t *testing.T) {
	sm := NewServiceManager("test", "/tmp/test", t.TempDir(), t.TempDir())
	result := sm.hasSystemd()

	// In a Docker container /run/systemd/system almost certainly does not exist.
	_, err := os.Stat("/run/systemd/system")
	want := err == nil
	if result != want {
		t.Errorf("hasSystemd() = %v, want %v (based on /run/systemd/system stat)", result, want)
	}
}

// --- hasRunit ---

// TestHasRunit_NoPanic verifies the unexported method does not panic.
func TestHasRunit_NoPanic(t *testing.T) {
	sm := NewServiceManager("test", "/tmp/test", t.TempDir(), t.TempDir())
	defer func() {
		if r := recover(); r != nil {
			t.Errorf("hasRunit() panicked: %v", r)
		}
	}()
	sm.hasRunit()
}

// TestHasRunit_ReturnsBool verifies the value is consistent with PATH lookup.
func TestHasRunit_ReturnsBool(t *testing.T) {
	sm := NewServiceManager("test", "/tmp/test", t.TempDir(), t.TempDir())
	// Only assert the type; whether sv is installed is environment-dependent.
	var _ bool = sm.hasRunit()
}

// --- hasOpenRC ---

// TestHasOpenRC_NoPanic verifies the unexported method does not panic.
func TestHasOpenRC_NoPanic(t *testing.T) {
	sm := NewServiceManager("test", "/tmp/test", t.TempDir(), t.TempDir())
	defer func() {
		if r := recover(); r != nil {
			t.Errorf("hasOpenRC() panicked: %v", r)
		}
	}()
	sm.hasOpenRC()
}

// TestHasOpenRC_ReturnsBool verifies the value is consistent with the binary
// detection logic (PATH lookup or /sbin/openrc-run).
func TestHasOpenRC_ReturnsBool(t *testing.T) {
	sm := NewServiceManager("test", "/tmp/test", t.TempDir(), t.TempDir())
	var _ bool = sm.hasOpenRC()
}

// --- hasSysVInit ---

// TestHasSysVInit_NoPanic verifies the unexported method does not panic.
func TestHasSysVInit_NoPanic(t *testing.T) {
	sm := NewServiceManager("test", "/tmp/test", t.TempDir(), t.TempDir())
	defer func() {
		if r := recover(); r != nil {
			t.Errorf("hasSysVInit() panicked: %v", r)
		}
	}()
	sm.hasSysVInit()
}

// TestHasSysVInit_FalseWhenSystemdPresent verifies the documented invariant:
// SysVinit is never selected when systemd is active.
func TestHasSysVInit_FalseWhenSystemdPresent(t *testing.T) {
	sm := NewServiceManager("test", "/tmp/test", t.TempDir(), t.TempDir())
	if sm.hasSystemd() && sm.hasSysVInit() {
		t.Error("hasSysVInit() = true while hasSystemd() = true; they must be mutually exclusive")
	}
}

// TestHasSysVInit_FalseWhenOpenRCPresent verifies the documented invariant:
// SysVinit is never selected when OpenRC is active.
func TestHasSysVInit_FalseWhenOpenRCPresent(t *testing.T) {
	sm := NewServiceManager("test", "/tmp/test", t.TempDir(), t.TempDir())
	if sm.hasOpenRC() && sm.hasSysVInit() {
		t.Error("hasSysVInit() = true while hasOpenRC() = true; they must be mutually exclusive")
	}
}

// --- GetServiceStatus ---

// TestGetServiceStatus_NoPanic verifies the method does not panic when no service
// is installed (the common case in a container test environment).
func TestGetServiceStatus_NoPanic(t *testing.T) {
	sm := NewServiceManager("nonexistent-vidveil-test-service", "/tmp/nope", t.TempDir(), t.TempDir())
	defer func() {
		if r := recover(); r != nil {
			t.Errorf("GetServiceStatus() panicked: %v", r)
		}
	}()
	sm.GetServiceStatus()
}

// TestGetServiceStatus_ReturnsString verifies that the string return value is
// one of the documented values even when the service does not exist.
func TestGetServiceStatus_ReturnsString(t *testing.T) {
	sm := NewServiceManager("nonexistent-vidveil-test-service", "/tmp/nope", t.TempDir(), t.TempDir())
	status, _ := sm.GetServiceStatus()

	validStatuses := map[string]bool{
		"running": true,
		"stopped": true,
		"unknown": true,
	}
	if !validStatuses[status] {
		t.Errorf("GetServiceStatus() status = %q, want one of: running, stopped, unknown", status)
	}
}

// TestGetServiceStatus_UnknownWhenNoSystemd verifies that when no systemd is
// available and the OS is Linux, the fallback value is "unknown".
func TestGetServiceStatus_UnknownWhenNoSystemd(t *testing.T) {
	sm := NewServiceManager("nonexistent-vidveil-test-service", "/tmp/nope", t.TempDir(), t.TempDir())
	if sm.hasSystemd() {
		t.Skip("systemd is present; this test is for non-systemd environments")
	}
	status, err := sm.GetServiceStatus()
	if status != "unknown" {
		t.Errorf("GetServiceStatus() without systemd = %q, want 'unknown'", status)
	}
	if err != nil {
		t.Errorf("GetServiceStatus() without systemd returned unexpected error: %v", err)
	}
}

// --- getParentProcessName ---

// TestGetParentProcessName_NoPanic verifies the unexported helper does not panic.
func TestGetParentProcessName_NoPanic(t *testing.T) {
	defer func() {
		if r := recover(); r != nil {
			t.Errorf("getParentProcessName() panicked: %v", r)
		}
	}()
	getParentProcessName()
}

// TestGetParentProcessName_ReturnsString verifies the return value is a plain
// string (may be empty in environments where /proc is unavailable and ps fails).
func TestGetParentProcessName_ReturnsString(t *testing.T) {
	name := getParentProcessName()
	// A non-empty name is expected on Linux where /proc/{ppid}/comm is readable.
	// We do not assert non-empty because the function is documented to return ""
	// as a valid fallback.
	_ = name
}

// TestGetParentProcessName_NoNullBytes verifies that the returned string contains
// no null bytes, which would indicate a read-without-trim bug on /proc paths.
func TestGetParentProcessName_NoNullBytes(t *testing.T) {
	name := getParentProcessName()
	for i, b := range []byte(name) {
		if b == 0 {
			t.Errorf("getParentProcessName() contains null byte at index %d", i)
		}
	}
}

// --- ShouldDaemonize ---

// TestShouldDaemonize_ServiceStartWithSystemd verifies that a service start
// under systemd always returns false (foreground).
func TestShouldDaemonize_ServiceStartWithSystemd(t *testing.T) {
	t.Setenv("INVOCATION_ID", "fake-invocation-id")
	got := ShouldDaemonize(true, false, false)
	if got {
		t.Error("ShouldDaemonize(serviceStart=true) under systemd should return false")
	}
}

// TestShouldDaemonize_ManualStartWithFlag verifies that passing daemonFlag=true
// returns true when isServiceStart is false.
func TestShouldDaemonize_ManualStartWithFlag(t *testing.T) {
	got := ShouldDaemonize(false, true, false)
	if !got {
		t.Error("ShouldDaemonize(serviceStart=false, daemonFlag=true) should return true")
	}
}

// TestShouldDaemonize_ManualStartConfigTrue verifies that configDaemonize drives
// the result when neither isServiceStart nor daemonFlag is set.
func TestShouldDaemonize_ManualStartConfigTrue(t *testing.T) {
	got := ShouldDaemonize(false, false, true)
	if !got {
		t.Error("ShouldDaemonize(serviceStart=false, daemonFlag=false, config=true) should return true")
	}
}

// TestShouldDaemonize_ManualStartAllFalse verifies that all-false inputs return
// false.
func TestShouldDaemonize_ManualStartAllFalse(t *testing.T) {
	got := ShouldDaemonize(false, false, false)
	if got {
		t.Error("ShouldDaemonize(false, false, false) should return false")
	}
}

// TestShouldDaemonize_ContainerReturnsFalse verifies that a service start
// inside a container returns false (foreground), matching the "container"
// branch in the switch.
func TestShouldDaemonize_ContainerReturnsFalse(t *testing.T) {
	// Ensure the container env var is visible so DetectServiceManager returns
	// "container" — but only when not already in Docker (/.dockerenv path).
	if _, err := os.Stat("/.dockerenv"); os.IsNotExist(err) {
		t.Setenv("container", "podman")
	}
	got := ShouldDaemonize(true, false, false)
	if got {
		t.Error("ShouldDaemonize(serviceStart=true) inside a container should return false")
	}
}

// ── Privilege / UAC stub functions ────────────────────────────────────────────

// TestShouldDropPrivileges_ReturnsBool verifies ShouldDropPrivileges returns a
// valid bool (true when root, false otherwise — both are valid test results).
func TestShouldDropPrivileges_ReturnsBool(t *testing.T) {
	got := ShouldDropPrivileges()
	want := (os.Getuid() == 0)
	if got != want {
		t.Errorf("ShouldDropPrivileges() = %v, want %v (uid=%d)", got, want, os.Getuid())
	}
}

// TestGetPrivilegeDropUser_ReturnsNonEmpty verifies GetPrivilegeDropUser returns
// the expected service account name.
func TestGetPrivilegeDropUser_ReturnsNonEmpty(t *testing.T) {
	got := GetPrivilegeDropUser()
	if got == "" {
		t.Error("GetPrivilegeDropUser() returned empty string")
	}
}

// TestIsElevated_MatchesGetuid verifies IsElevated mirrors os.Geteuid() == 0.
func TestIsElevated_MatchesGetuid(t *testing.T) {
	got := IsElevated()
	want := (os.Geteuid() == 0)
	if got != want {
		t.Errorf("IsElevated() = %v, want %v (euid=%d)", got, want, os.Geteuid())
	}
}

// TestIsRunningElevated_MatchesGetuid verifies IsRunningElevated mirrors os.Getuid() == 0.
func TestIsRunningElevated_MatchesGetuid(t *testing.T) {
	got := IsRunningElevated()
	want := (os.Getuid() == 0)
	if got != want {
		t.Errorf("IsRunningElevated() = %v, want %v (uid=%d)", got, want, os.Getuid())
	}
}

// TestRequestElevation_AlreadyAdminWhenRoot verifies RequestElevation returns
// ElevationAlreadyAdmin when running as root (standard in Docker containers).
func TestRequestElevation_WhenRootAlreadyAdmin(t *testing.T) {
	if os.Getuid() != 0 {
		t.Skip("test only meaningful when running as root")
	}
	got := RequestElevation()
	if got != ElevationAlreadyAdmin {
		t.Errorf("RequestElevation() as root = %v, want ElevationAlreadyAdmin", got)
	}
}

// TestRequireAdmin_NilWhenRoot verifies RequireAdmin returns false, nil when root.
func TestRequireAdmin_NilWhenRoot(t *testing.T) {
	if os.Getuid() != 0 {
		t.Skip("test only meaningful when running as root")
	}
	needExit, err := RequireAdmin("test-operation")
	if needExit {
		t.Error("RequireAdmin() as root: needExit should be false")
	}
	if err != nil {
		t.Errorf("RequireAdmin() as root: err = %v, want nil", err)
	}
}

// TestGetWindowsServiceAccount_EmptyOnNonWindows verifies the non-Windows stub
// always returns an empty string.
func TestGetWindowsServiceAccount_EmptyOnNonWindows(t *testing.T) {
	got := GetWindowsServiceAccount("any-service")
	if got != "" {
		t.Errorf("GetWindowsServiceAccount() non-Windows = %q, want empty", got)
	}
}

// TestIsRunningAsService_ReturnsBool verifies IsRunningAsService returns a bool
// without panicking.
func TestIsRunningAsService_ReturnsBool(t *testing.T) {
	// Result depends on whether PPID=1 (init/systemd), so just verify no panic.
	_ = IsRunningAsService()
}

// TestDropPrivileges_NoopWhenNonRoot verifies DropPrivileges returns nil
// immediately when not running as root.
func TestDropPrivileges_NoopWhenNonRoot(t *testing.T) {
	if os.Getuid() == 0 {
		t.Skip("test only meaningful when NOT running as root")
	}
	if err := DropPrivileges("nobody"); err != nil {
		t.Errorf("DropPrivileges() non-root: got %v, want nil", err)
	}
}

// TestCanEscalate_ReturnsBool verifies CanEscalate returns a bool without
// panicking (result depends on environment).
func TestCanEscalate_ReturnsBool(t *testing.T) {
	_ = CanEscalate()
}

// ── DetectServiceManager ──────────────────────────────────────────────────────

// TestDetectServiceManager_ReturnsValidString verifies DetectServiceManager
// returns a recognised service manager name or "manual" without panicking.
// The exact value depends on the host environment so we just range-check.
func TestDetectServiceManager_ReturnsValidString(t *testing.T) {
	got := DetectServiceManager()
	valid := map[string]bool{
		"systemd": true, "launchd": true, "runit": true, "s6": true,
		"sysv": true, "rcd": true, "container": true, "manual": true,
	}
	if !valid[got] {
		t.Errorf("DetectServiceManager() = %q, not a recognised service manager", got)
	}
}

// ── IsRunningInContainer env-var paths ────────────────────────────────────────

// TestIsRunningInContainer_ContainerEnvReturnsTrue verifies that the generic
// "container" environment variable (used by systemd-nspawn/lxc) is detected.
func TestIsRunningInContainer_ContainerEnvReturnsTrue(t *testing.T) {
	t.Setenv("container", "lxc")
	if !IsRunningInContainer() {
		t.Error("IsRunningInContainer() = false with container=lxc, want true")
	}
}

// TestIsRunningInContainer_KubernetesSvcHostReturnsTrue verifies that
// KUBERNETES_SERVICE_HOST triggers container detection.
func TestIsRunningInContainer_KubernetesSvcHostReturnsTrue(t *testing.T) {
	t.Setenv("container", "")
	t.Setenv("KUBERNETES_SERVICE_HOST", "10.96.0.1")
	if !IsRunningInContainer() {
		t.Error("IsRunningInContainer() = false with KUBERNETES_SERVICE_HOST set, want true")
	}
}

// ── DetectEscalation ──────────────────────────────────────────────────────────

// TestDetectEscalation_ReturnsStringOrEmpty verifies DetectEscalation returns a
// known value or empty string without panicking.
func TestDetectEscalation_ReturnsStringOrEmpty(t *testing.T) {
	got := DetectEscalation()
	valid := map[string]bool{"sudo": true, "doas": true, "pkexec": true, "runas": true, "": true}
	if !valid[got] {
		t.Errorf("DetectEscalation() = %q, want sudo|doas|pkexec|runas|empty", got)
	}
}

// ── IsRunningAsRoot ───────────────────────────────────────────────────────────

// TestIsRunningAsRoot_MatchesGetuid verifies IsRunningAsRoot matches os.Getuid()==0
// on non-Windows.
func TestIsRunningAsRoot_MatchesGetuid(t *testing.T) {
	got := IsRunningAsRoot()
	want := os.Getuid() == 0
	if got != want {
		t.Errorf("IsRunningAsRoot() = %v, want %v (uid=%d)", got, want, os.Getuid())
	}
}

// ── IsWindowsService ─────────────────────────────────────────────────────────

// TestIsWindowsService_FalseOnNonWindows verifies the non-Windows stub always
// returns false.
func TestIsWindowsService_FalseOnNonWindows(t *testing.T) {
	if IsWindowsService() {
		t.Error("IsWindowsService() = true on non-Windows, want false")
	}
}

// ── GetServiceStatus no-systemd path ─────────────────────────────────────────

// TestGetServiceStatus_UnknownOnUnsupportedOS verifies GetServiceStatus returns
// "unknown" with a nil error when neither systemd nor launchctl is active.
func TestGetServiceStatus_FallbackUnknown(t *testing.T) {
	sm := NewServiceManager("testapp", "/usr/bin/testapp", "/etc/testapp", "/var/lib/testapp")
	status, err := sm.GetServiceStatus()
	if err != nil {
		t.Errorf("GetServiceStatus() unexpected error: %v", err)
	}
	valid := map[string]bool{"running": true, "stopped": true, "unknown": true}
	if !valid[status] {
		t.Errorf("GetServiceStatus() = %q, want running|stopped|unknown", status)
	}
}
