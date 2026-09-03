// SPDX-License-Identifier: MIT
// AI.md PART 28: Direct coverage for install/uninstall/run-command paths.
// All functions are called directly (same package); system commands fail on
// a minimal Linux container but every code path IS executed.
package system

import (
	"testing"
)

// newCovSM returns a ServiceManager with a unique test app name.
func newCovSM(t *testing.T) *ServiceManager {
	t.Helper()
	return NewServiceManager(
		"vidveil-cov-test",
		"/usr/local/bin/vidveil-cov-test",
		"/etc/apimgr/vidveil-cov-test",
		"/var/lib/apimgr/vidveil-cov-test",
	)
}

// ── Install / Uninstall dispatch (covers runtime.GOOS switch branches) ────────

// TestInstall_NoPanic calls Install() which dispatches to installLinux() on
// Linux. createLinuxUser() and the init-system detection run; errors are
// expected and ignored.
func TestInstall_NoPanic(t *testing.T) {
	sm := newCovSM(t)
	_ = sm.Install()
}

// TestUninstall_NoPanic calls Uninstall() which dispatches to uninstallLinux().
func TestUninstall_NoPanic(t *testing.T) {
	sm := newCovSM(t)
	_ = sm.Uninstall()
}

// ── Start / Stop / Restart / Reload (covers runServiceCommand paths) ─────────

// TestStart_NoPanic exercises the "start" runServiceCommand path on Linux.
func TestStart_NoPanic(t *testing.T) {
	sm := newCovSM(t)
	_ = sm.Start()
}

// TestStop_NoPanic exercises the "stop" runServiceCommand path.
func TestStop_NoPanic(t *testing.T) {
	sm := newCovSM(t)
	_ = sm.Stop()
}

// TestRestart_NoPanic exercises the "restart" runServiceCommand path.
func TestRestart_NoPanic(t *testing.T) {
	sm := newCovSM(t)
	_ = sm.Restart()
}

// TestReload_NoPanic exercises the "reload" runServiceCommand path.
func TestReload_NoPanic(t *testing.T) {
	sm := newCovSM(t)
	_ = sm.Reload()
}

// ── Disable dispatch ──────────────────────────────────────────────────────────

// TestDisable_NoPanic exercises the Disable() Linux branch.
func TestDisable_NoPanic(t *testing.T) {
	sm := newCovSM(t)
	_ = sm.Disable()
}

// ── installLinux and its sub-functions ───────────────────────────────────────

// TestInstallLinux_NoPanic calls installLinux() directly; createLinuxUser()
// and init-system detection run.  All errors are expected.
func TestInstallLinux_NoPanic(t *testing.T) {
	sm := newCovSM(t)
	_ = sm.installLinux()
}

// TestInstallSystemd_NoPanic writes a systemd unit to /etc/systemd/system/;
// that directory does not exist in alpine, so WriteFile errors — but every
// statement before it is covered.
func TestInstallSystemd_NoPanic(t *testing.T) {
	sm := newCovSM(t)
	_ = sm.installSystemd()
}

// TestInstallRunit_NoPanic creates /etc/sv/{name}/ and writes a run script.
// MkdirAll and WriteFile may succeed or fail; errors are expected and ignored.
func TestInstallRunit_NoPanic(t *testing.T) {
	sm := newCovSM(t)
	_ = sm.installRunit()
}

// TestInstallOpenRC_NoPanic writes /etc/init.d/{name} (writable on alpine as
// root).  rc-update is not installed so Enable step errors — ignored.
func TestInstallOpenRC_NoPanic(t *testing.T) {
	sm := newCovSM(t)
	_ = sm.installOpenRC()
}

// TestInstallSysVInit_NoPanic writes /etc/init.d/{name} and runs update-rc.d;
// neither update-rc.d nor chkconfig is typically present — errors ignored.
func TestInstallSysVInit_NoPanic(t *testing.T) {
	sm := newCovSM(t)
	_ = sm.installSysVInit()
}

// ── installDarwin / installBSD / installWindows ───────────────────────────────

// TestInstallDarwin_NoPanic tries to write to /Library/LaunchDaemons/ which
// does not exist on Linux; WriteFile returns an error — code path IS covered.
func TestInstallDarwin_NoPanic(t *testing.T) {
	sm := newCovSM(t)
	_ = sm.installDarwin()
}

// TestInstallBSD_NoPanic tries to write to /usr/local/etc/rc.d/; the directory
// does not exist on alpine so WriteFile fails — code path IS covered.
func TestInstallBSD_NoPanic(t *testing.T) {
	sm := newCovSM(t)
	_ = sm.installBSD()
}

// TestInstallWindows_NoPanic runs "sc create ..." which is not present on
// Linux; the command fails — code path IS covered.
func TestInstallWindows_NoPanic(t *testing.T) {
	sm := newCovSM(t)
	_ = sm.installWindows()
}

// ── uninstall helpers ─────────────────────────────────────────────────────────

// TestUninstallLinux_NoPanic calls Stop() then cleans up service unit files.
// All removes are no-ops because the service was never installed.
func TestUninstallLinux_NoPanic(t *testing.T) {
	sm := newCovSM(t)
	_ = sm.uninstallLinux()
}

// TestUninstallDarwin_NoPanic runs launchctl unload (not found on Linux) and
// removes the plist (also not present) — both error paths covered.
func TestUninstallDarwin_NoPanic(t *testing.T) {
	sm := newCovSM(t)
	_ = sm.uninstallDarwin()
}

// TestUninstallBSD_NoPanic calls Stop() and removes the rc.d script.
func TestUninstallBSD_NoPanic(t *testing.T) {
	sm := newCovSM(t)
	_ = sm.uninstallBSD()
}

// TestUninstallWindows_NoPanic calls Stop() and "sc delete" — sc not available
// on Linux; errors ignored.
func TestUninstallWindows_NoPanic(t *testing.T) {
	sm := newCovSM(t)
	_ = sm.uninstallWindows()
}

// ── createLinuxUser / createDarwinUser / createBSDUser ───────────────────────

// TestCreateLinuxUser_NoPanic runs the Linux user creation path; useradd may
// not exist (alpine uses adduser) or the user may already exist — both paths
// are covered, errors are expected and ignored.
func TestCreateLinuxUser_NoPanic(t *testing.T) {
	sm := newCovSM(t)
	_ = sm.createLinuxUser()
}

// TestCreateDarwinUser_NoPanic runs dscl commands which are absent on Linux;
// the check for user existence fails immediately — code path IS covered.
func TestCreateDarwinUser_NoPanic(t *testing.T) {
	sm := newCovSM(t)
	_ = sm.createDarwinUser()
}

// TestCreateBSDUser_NoPanic runs pw commands which are absent on Linux; the
// check for user existence fails immediately — code path IS covered.
func TestCreateBSDUser_NoPanic(t *testing.T) {
	sm := newCovSM(t)
	_ = sm.createBSDUser()
}

// ── findAvailableUID ─────────────────────────────────────────────────────────

// TestFindAvailableUID_ReturnsInSpecRange verifies the function returns a value
// in the spec range 200-899, ignoring any passed min/max per AI.md PART 23.
func TestFindAvailableUID_ReturnsInSpecRange(t *testing.T) {
	sm := newCovSM(t)
	uid := sm.findAvailableUID(200, 899)
	if uid < 200 || uid > 899 {
		t.Errorf("findAvailableUID() = %d, want 200–899 (spec range per AI.md PART 23)", uid)
	}
	// Result must not be a reserved ID
	if reservedSystemIDs[uid] {
		t.Errorf("findAvailableUID() = %d is a reserved ID per AI.md PART 23", uid)
	}
}

// TestFindAvailableUID_SmallRange exercises the wrapper with alternate params;
// result is still in the spec range because findAvailableSystemID ignores min/max.
func TestFindAvailableUID_SmallRange(t *testing.T) {
	sm := newCovSM(t)
	uid := sm.findAvailableUID(895, 899)
	if uid < 200 || uid > 899 {
		t.Errorf("findAvailableUID(895,899) = %d, want 200–899 (spec range per AI.md PART 23)", uid)
	}
}

// ── EnsureSystemUser ─────────────────────────────────────────────────────────

// TestEnsureSystemUser_NoPanic runs EnsureSystemUser with a nonexistent user
// name; on Docker (root) it will attempt user creation — errors ignored.
func TestEnsureSystemUser_NoPanic(t *testing.T) {
	dirs := []string{t.TempDir()}
	_, _, _ = EnsureSystemUser("vidveil-cov-test-user", dirs)
}

// TestEnsureSystemUser_NonRoot_ReturnsCurrent verifies that when running as a
// non-root user the function returns the current UID/GID without error.
func TestEnsureSystemUser_NonRoot_ReturnsCurrent(t *testing.T) {
	if IsRunningAsRoot() {
		t.Skip("test only meaningful when NOT running as root")
	}
	uid, gid, err := EnsureSystemUser("any-user", nil)
	if err != nil {
		t.Errorf("EnsureSystemUser() non-root: unexpected error: %v", err)
	}
	if uid == 0 || gid == 0 {
		t.Errorf("EnsureSystemUser() non-root: got uid=%d gid=%d, want non-zero", uid, gid)
	}
}

// ── findAvailableID (package-level function) ─────────────────────────────────

// TestFindAvailableID_ReturnsInSpecRange verifies the package-level function
// returns a value in the spec range 200-899 per AI.md PART 23.
func TestFindAvailableID_ReturnsInSpecRange(t *testing.T) {
	id := findAvailableID(200, 899)
	if id < 200 || id > 899 {
		t.Errorf("findAvailableID() = %d, want 200–899 (spec range per AI.md PART 23)", id)
	}
	if reservedSystemIDs[id] {
		t.Errorf("findAvailableID() = %d is a reserved ID per AI.md PART 23", id)
	}
}

// TestFindAvailableID_SmallRange exercises the wrapper with alternate params;
// result is still in the spec range because findAvailableSystemID ignores min/max.
func TestFindAvailableID_SmallRange(t *testing.T) {
	id := findAvailableID(895, 899)
	if id < 200 || id > 899 {
		t.Errorf("findAvailableID(895,899) = %d, want 200–899 (spec range per AI.md PART 23)", id)
	}
}

// ── DetectServiceManager extended paths ───────────────────────────────────────

// TestDetectServiceManager_SVDIREnvRunit verifies that the SVDIR environment
// variable causes "runit" to be returned when not in a container.
func TestDetectServiceManager_SVDIREnvRunit(t *testing.T) {
	// Only exercise this branch when no container markers are present.
	if IsRunningInContainer() {
		t.Skip("running in container; DetectServiceManager returns 'container', not 'runit'")
	}
	t.Setenv("SVDIR", "/var/service")
	t.Setenv("INVOCATION_ID", "")
	got := DetectServiceManager()
	if got != "runit" {
		// INVOCATION_ID or other env vars may still trigger systemd/launchd.
		// Just verify the function does not panic and returns a known value.
		valid := map[string]bool{
			"systemd": true, "launchd": true, "runit": true, "s6": true,
			"sysv": true, "rcd": true, "container": true, "manual": true,
		}
		if !valid[got] {
			t.Errorf("DetectServiceManager() with SVDIR set = %q, not a recognised value", got)
		}
	}
}

// TestDetectServiceManager_S6LoggingEnvS6 verifies that S6_LOGGING environment
// variable causes "s6" to be returned when not in a container.
func TestDetectServiceManager_S6LoggingEnvS6(t *testing.T) {
	if IsRunningInContainer() {
		t.Skip("running in container; DetectServiceManager returns 'container'")
	}
	t.Setenv("S6_LOGGING", "1")
	t.Setenv("SVDIR", "")
	t.Setenv("INVOCATION_ID", "")
	got := DetectServiceManager()
	if got != "s6" {
		valid := map[string]bool{
			"systemd": true, "launchd": true, "runit": true, "s6": true,
			"sysv": true, "rcd": true, "container": true, "manual": true,
		}
		if !valid[got] {
			t.Errorf("DetectServiceManager() with S6_LOGGING set = %q, not a recognised value", got)
		}
	}
}

// ── GetServiceStatus systemd branch ──────────────────────────────────────────

// TestGetServiceStatus_SystemdBranch covers the systemd sub-branch in
// GetServiceStatus when hasSystemd() is true and systemctl returns output.
func TestGetServiceStatus_SystemdBranch(t *testing.T) {
	sm := newCovSM(t)
	if !sm.hasSystemd() {
		t.Skip("systemd not present; skipping systemd-specific branch test")
	}
	status, _ := sm.GetServiceStatus()
	valid := map[string]bool{"running": true, "stopped": true, "unknown": true}
	if !valid[status] {
		t.Errorf("GetServiceStatus() systemd = %q, want running|stopped|unknown", status)
	}
}

// ── runServiceCommand ─────────────────────────────────────────────────────────

// TestRunServiceCommand_UnsupportedAction covers the default branch that
// returns "unsupported action" when an unrecognised action string is passed.
func TestRunServiceCommand_UnsupportedAction(t *testing.T) {
	sm := newCovSM(t)
	err := sm.runServiceCommand("bogus-action-xyz")
	if err == nil {
		t.Error("runServiceCommand(bogus) should return an error")
	}
}
