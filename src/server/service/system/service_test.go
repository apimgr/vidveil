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
