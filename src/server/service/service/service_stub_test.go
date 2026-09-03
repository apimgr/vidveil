// SPDX-License-Identifier: MIT
// AI.md PART 28: Stub-based coverage for all OS service-manager code paths.
package service

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// errNotFound is returned by the stub lookPath for unavailable binaries.
var errNotFound = errors.New("executable file not found in $PATH")

// cmdRecorder records every command invocation and returns configurable results.
type cmdRecorder struct {
	calls  [][]string
	err    error
	out    []byte
	outErr error
}

func (r *cmdRecorder) run(name string, args ...string) error {
	r.calls = append(r.calls, append([]string{name}, args...))
	return r.err
}

func (r *cmdRecorder) runOut(name string, args ...string) ([]byte, error) {
	r.calls = append(r.calls, append([]string{name}, args...))
	return r.out, r.outErr
}

// has reports whether a recorded call starts with the given words.
func (r *cmdRecorder) has(words ...string) bool {
	for _, c := range r.calls {
		if len(c) < len(words) {
			continue
		}
		match := true
		for i, w := range words {
			if c[i] != w {
				match = false
				break
			}
		}
		if match {
			return true
		}
	}
	return false
}

// newStubManager builds a manager rooted in a temp dir with recorded commands and a fixed binary set.
func newStubManager(t *testing.T, goos string, available ...string) (*SystemServiceManager, *cmdRecorder) {
	t.Helper()
	avail := map[string]bool{}
	for _, a := range available {
		avail[a] = true
	}
	rec := &cmdRecorder{}
	m := &SystemServiceManager{
		name:        "vidveil",
		displayName: "VidVeil",
		description: "VidVeil test service",
		execPath:    "/usr/local/bin/vidveil",
		rootPrefix:  t.TempDir(),
		goos:        goos,
		run:         rec.run,
		runOut:      rec.runOut,
		lookPath: func(file string) (string, error) {
			if avail[file] {
				return "/usr/bin/" + file, nil
			}
			return "", errNotFound
		},
	}
	return m, rec
}

// mkroot creates a directory under the manager's root prefix.
func mkroot(t *testing.T, m *SystemServiceManager, dir string) {
	t.Helper()
	if err := os.MkdirAll(m.path(dir), 0755); err != nil {
		t.Fatalf("mkdir %s: %v", dir, err)
	}
}

// enableOpenRC creates the openrc-run marker so hasOpenRC returns true.
func enableOpenRC(t *testing.T, m *SystemServiceManager) {
	t.Helper()
	mkroot(t, m, "/sbin")
	if err := os.WriteFile(m.path("/sbin/openrc-run"), []byte("#!/bin/sh\n"), 0755); err != nil {
		t.Fatalf("write openrc-run: %v", err)
	}
}

// --- path helper ---

func TestPath_JoinsRootPrefix(t *testing.T) {
	m, _ := newStubManager(t, "linux")
	got := m.path("/etc/init.d", "vidveil")
	want := filepath.Join(m.rootPrefix, "etc/init.d/vidveil")
	if got != want {
		t.Errorf("path() = %q, want %q", got, want)
	}
}

func TestPath_EmptyPrefixKeepsAbsolute(t *testing.T) {
	m := &SystemServiceManager{}
	if got := m.path("/etc/systemd/system", "x.service"); got != "/etc/systemd/system/x.service" {
		t.Errorf("path() = %q, want /etc/systemd/system/x.service", got)
	}
}

// --- init system detection ---

func TestHasSystemd_Stubbed(t *testing.T) {
	m, _ := newStubManager(t, "linux", "systemctl")
	if !m.hasSystemd() {
		t.Error("hasSystemd() = false, want true")
	}
	m2, _ := newStubManager(t, "linux")
	if m2.hasSystemd() {
		t.Error("hasSystemd() = true, want false")
	}
}

func TestHasOpenRC_Stubbed(t *testing.T) {
	m, _ := newStubManager(t, "linux")
	if m.hasOpenRC() {
		t.Error("hasOpenRC() = true before marker exists")
	}
	enableOpenRC(t, m)
	if !m.hasOpenRC() {
		t.Error("hasOpenRC() = false, want true")
	}
}

func TestHasSysVinit_Stubbed(t *testing.T) {
	m, _ := newStubManager(t, "linux", "update-rc.d")
	if m.hasSysVinit() {
		t.Error("hasSysVinit() = true without /etc/init.d")
	}
	mkroot(t, m, "/etc/init.d")
	if !m.hasSysVinit() {
		t.Error("hasSysVinit() = false, want true")
	}
	msd, _ := newStubManager(t, "linux", "systemctl", "update-rc.d")
	mkroot(t, msd, "/etc/init.d")
	if msd.hasSysVinit() {
		t.Error("hasSysVinit() = true when systemd present, want false")
	}
}

func TestHasRunit_Stubbed(t *testing.T) {
	m, _ := newStubManager(t, "linux", "sv")
	if !m.hasRunit() {
		t.Error("hasRunit() = false, want true")
	}
}

// --- linux lifecycle commands per init system ---

func TestLinuxLifecycle_Systemd(t *testing.T) {
	m, rec := newStubManager(t, "linux", "systemctl")
	if err := m.Start(); err != nil {
		t.Fatalf("Start() error: %v", err)
	}
	if err := m.Stop(); err != nil {
		t.Fatalf("Stop() error: %v", err)
	}
	if err := m.Restart(); err != nil {
		t.Fatalf("Restart() error: %v", err)
	}
	if err := m.Reload(); err != nil {
		t.Fatalf("Reload() error: %v", err)
	}
	if err := m.Disable(); err != nil {
		t.Fatalf("Disable() error: %v", err)
	}
	for _, verb := range []string{"start", "stop", "restart", "reload", "disable"} {
		if !rec.has("systemctl", verb, "vidveil") {
			t.Errorf("missing systemctl %s vidveil in %v", verb, rec.calls)
		}
	}
}

func TestLinuxLifecycle_OpenRC(t *testing.T) {
	m, rec := newStubManager(t, "linux", "rc-service", "rc-update")
	enableOpenRC(t, m)
	if err := m.Start(); err != nil {
		t.Fatalf("Start() error: %v", err)
	}
	if err := m.Stop(); err != nil {
		t.Fatalf("Stop() error: %v", err)
	}
	if err := m.Restart(); err != nil {
		t.Fatalf("Restart() error: %v", err)
	}
	if err := m.Reload(); err != nil {
		t.Fatalf("Reload() error: %v", err)
	}
	if err := m.Disable(); err != nil {
		t.Fatalf("Disable() error: %v", err)
	}
	for _, verb := range []string{"start", "stop", "restart", "reload"} {
		if !rec.has("rc-service", "vidveil", verb) {
			t.Errorf("missing rc-service vidveil %s in %v", verb, rec.calls)
		}
	}
	if !rec.has("rc-update", "del", "vidveil", "default") {
		t.Errorf("missing rc-update del in %v", rec.calls)
	}
}

func TestLinuxLifecycle_SysVinit(t *testing.T) {
	m, rec := newStubManager(t, "linux", "update-rc.d")
	mkroot(t, m, "/etc/init.d")
	initScript := m.path("/etc/init.d", "vidveil")
	if err := m.Start(); err != nil {
		t.Fatalf("Start() error: %v", err)
	}
	if err := m.Stop(); err != nil {
		t.Fatalf("Stop() error: %v", err)
	}
	if err := m.Restart(); err != nil {
		t.Fatalf("Restart() error: %v", err)
	}
	if err := m.Reload(); err != nil {
		t.Fatalf("Reload() error: %v", err)
	}
	for _, verb := range []string{"start", "stop", "restart", "reload"} {
		if !rec.has(initScript, verb) {
			t.Errorf("missing %s %s in %v", initScript, verb, rec.calls)
		}
	}
	if err := m.Disable(); err != nil {
		t.Fatalf("Disable() error: %v", err)
	}
	if !rec.has("update-rc.d", "vidveil", "remove") {
		t.Errorf("missing update-rc.d remove in %v", rec.calls)
	}
}

func TestLinuxDisable_SysVinitChkconfig(t *testing.T) {
	m, rec := newStubManager(t, "linux", "chkconfig")
	mkroot(t, m, "/etc/init.d")
	if err := m.Disable(); err != nil {
		t.Fatalf("Disable() error: %v", err)
	}
	if !rec.has("chkconfig", "--del", "vidveil") {
		t.Errorf("missing chkconfig --del in %v", rec.calls)
	}
}

func TestLinuxLifecycle_Runit(t *testing.T) {
	m, rec := newStubManager(t, "linux", "sv")
	mkroot(t, m, "/etc/service/vidveil")
	if err := m.Start(); err != nil {
		t.Fatalf("Start() error: %v", err)
	}
	if err := m.Stop(); err != nil {
		t.Fatalf("Stop() error: %v", err)
	}
	if err := m.Restart(); err != nil {
		t.Fatalf("Restart() error: %v", err)
	}
	if err := m.Reload(); err != nil {
		t.Fatalf("Reload() error: %v", err)
	}
	for _, verb := range []string{"start", "stop", "restart", "hup"} {
		if !rec.has("sv", verb, "vidveil") {
			t.Errorf("missing sv %s vidveil in %v", verb, rec.calls)
		}
	}
	if err := m.Disable(); err != nil {
		t.Fatalf("Disable() error: %v", err)
	}
	if _, err := os.Stat(m.path("/etc/service", "vidveil")); !os.IsNotExist(err) {
		t.Error("Disable() did not remove /etc/service symlink")
	}
}

func TestLinuxLifecycle_NoInitSystem(t *testing.T) {
	m, _ := newStubManager(t, "linux")
	for name, fn := range map[string]func() error{
		"Start": m.Start, "Stop": m.Stop, "Restart": m.Restart,
		"Reload": m.Reload, "Install": m.Install, "Uninstall": m.Uninstall, "Disable": m.Disable,
	} {
		if err := fn(); err == nil {
			t.Errorf("%s() error = nil, want no-service-manager error", name)
		}
	}
}

// --- unsupported OS dispatch ---

func TestDispatch_UnsupportedOS(t *testing.T) {
	m, _ := newStubManager(t, "plan9")
	for name, fn := range map[string]func() error{
		"Start": m.Start, "Stop": m.Stop, "Restart": m.Restart,
		"Reload": m.Reload, "Install": m.Install, "Uninstall": m.Uninstall, "Disable": m.Disable,
	} {
		err := fn()
		if err == nil || !strings.Contains(err.Error(), "unsupported OS") {
			t.Errorf("%s() error = %v, want unsupported OS", name, err)
		}
	}
	if _, err := m.GetServiceStatus(); err == nil {
		t.Error("GetServiceStatus() error = nil, want unsupported OS")
	}
}

func TestReload_WindowsUnsupported(t *testing.T) {
	m, _ := newStubManager(t, "windows")
	if err := m.Reload(); err == nil {
		t.Error("Reload() on windows error = nil, want error")
	}
}

// --- systemd install/uninstall ---

func TestInstallSystemd_WritesUnitAndEnables(t *testing.T) {
	m, rec := newStubManager(t, "linux", "systemctl")
	mkroot(t, m, "/etc/systemd/system")
	if err := m.Install(); err != nil {
		t.Fatalf("Install() error: %v", err)
	}
	data, err := os.ReadFile(m.path("/etc/systemd/system", "vidveil.service"))
	if err != nil {
		t.Fatalf("read unit file: %v", err)
	}
	unit := string(data)
	for _, want := range []string{
		"Description=VidVeil test service",
		"ExecStart=/usr/local/bin/vidveil",
		"User=vidveil",
		"NoNewPrivileges=yes",
		"ProtectSystem=strict",
		"WantedBy=multi-user.target",
	} {
		if !strings.Contains(unit, want) {
			t.Errorf("unit file missing %q", want)
		}
	}
	if !rec.has("systemctl", "daemon-reload") || !rec.has("systemctl", "enable", "vidveil") {
		t.Errorf("missing systemctl daemon-reload/enable in %v", rec.calls)
	}
}

func TestInstallSystemd_WriteFails(t *testing.T) {
	m, _ := newStubManager(t, "linux", "systemctl")
	if err := m.Install(); err == nil {
		t.Error("Install() error = nil, want write failure (no /etc/systemd/system)")
	}
}

func TestInstallSystemd_CommandFails(t *testing.T) {
	m, rec := newStubManager(t, "linux", "systemctl")
	mkroot(t, m, "/etc/systemd/system")
	rec.err = errors.New("boom")
	if err := m.Install(); err == nil {
		t.Error("Install() error = nil, want daemon-reload failure")
	}
}

func TestUninstallSystemd_RemovesUnit(t *testing.T) {
	m, rec := newStubManager(t, "linux", "systemctl")
	mkroot(t, m, "/etc/systemd/system")
	unitPath := m.path("/etc/systemd/system", "vidveil.service")
	if err := os.WriteFile(unitPath, []byte("[Unit]\n"), 0644); err != nil {
		t.Fatalf("seed unit file: %v", err)
	}
	if err := m.Uninstall(); err != nil {
		t.Fatalf("Uninstall() error: %v", err)
	}
	if _, err := os.Stat(unitPath); !os.IsNotExist(err) {
		t.Error("unit file still exists after Uninstall()")
	}
	if !rec.has("systemctl", "stop", "vidveil") || !rec.has("systemctl", "disable", "vidveil") {
		t.Errorf("missing systemctl stop/disable in %v", rec.calls)
	}
}

// --- OpenRC install/uninstall ---

func TestInstallOpenRC_WritesScriptAndEnables(t *testing.T) {
	m, rec := newStubManager(t, "linux", "rc-update")
	enableOpenRC(t, m)
	mkroot(t, m, "/etc/init.d")
	if err := m.Install(); err != nil {
		t.Fatalf("Install() error: %v", err)
	}
	initPath := m.path("/etc/init.d", "vidveil")
	data, err := os.ReadFile(initPath)
	if err != nil {
		t.Fatalf("read init script: %v", err)
	}
	script := string(data)
	for _, want := range []string{"#!/sbin/openrc-run", `command="/usr/local/bin/vidveil"`, `name="vidveil"`} {
		if !strings.Contains(script, want) {
			t.Errorf("OpenRC script missing %q", want)
		}
	}
	info, err := os.Stat(initPath)
	if err != nil {
		t.Fatalf("stat init script: %v", err)
	}
	if info.Mode().Perm() != 0755 {
		t.Errorf("init script mode = %v, want 0755", info.Mode().Perm())
	}
	if !rec.has("rc-update", "add", "vidveil", "default") {
		t.Errorf("missing rc-update add in %v", rec.calls)
	}
}

func TestUninstallOpenRC_RemovesScript(t *testing.T) {
	m, rec := newStubManager(t, "linux", "rc-update")
	enableOpenRC(t, m)
	mkroot(t, m, "/etc/init.d")
	initPath := m.path("/etc/init.d", "vidveil")
	if err := os.WriteFile(initPath, []byte("#!/sbin/openrc-run\n"), 0755); err != nil {
		t.Fatalf("seed init script: %v", err)
	}
	if err := m.Uninstall(); err != nil {
		t.Fatalf("Uninstall() error: %v", err)
	}
	if _, err := os.Stat(initPath); !os.IsNotExist(err) {
		t.Error("init script still exists after Uninstall()")
	}
	if !rec.has("rc-service", "vidveil", "stop") || !rec.has("rc-update", "del", "vidveil", "default") {
		t.Errorf("missing rc-service stop / rc-update del in %v", rec.calls)
	}
}

// --- SysVinit install/uninstall ---

func TestInstallSysVinit_UpdateRcD(t *testing.T) {
	m, rec := newStubManager(t, "linux", "update-rc.d")
	mkroot(t, m, "/etc/init.d")
	if err := m.Install(); err != nil {
		t.Fatalf("Install() error: %v", err)
	}
	data, err := os.ReadFile(m.path("/etc/init.d", "vidveil"))
	if err != nil {
		t.Fatalf("read init script: %v", err)
	}
	script := string(data)
	for _, want := range []string{"### BEGIN INIT INFO", "NAME=vidveil", "DAEMON=/usr/local/bin/vidveil", "start-stop-daemon"} {
		if !strings.Contains(script, want) {
			t.Errorf("SysVinit script missing %q", want)
		}
	}
	if !rec.has("update-rc.d", "vidveil", "defaults") {
		t.Errorf("missing update-rc.d defaults in %v", rec.calls)
	}
}

func TestInstallSysVinit_Chkconfig(t *testing.T) {
	m, rec := newStubManager(t, "linux", "chkconfig")
	mkroot(t, m, "/etc/init.d")
	if err := m.Install(); err != nil {
		t.Fatalf("Install() error: %v", err)
	}
	if !rec.has("chkconfig", "--add", "vidveil") || !rec.has("chkconfig", "vidveil", "on") {
		t.Errorf("missing chkconfig add/on in %v", rec.calls)
	}
}

func TestUninstallSysVinit_RemovesScript(t *testing.T) {
	m, rec := newStubManager(t, "linux", "update-rc.d")
	mkroot(t, m, "/etc/init.d")
	initPath := m.path("/etc/init.d", "vidveil")
	if err := os.WriteFile(initPath, []byte("#!/bin/sh\n"), 0755); err != nil {
		t.Fatalf("seed init script: %v", err)
	}
	if err := m.Uninstall(); err != nil {
		t.Fatalf("Uninstall() error: %v", err)
	}
	if _, err := os.Stat(initPath); !os.IsNotExist(err) {
		t.Error("init script still exists after Uninstall()")
	}
	if !rec.has("update-rc.d", "vidveil", "remove") {
		t.Errorf("missing update-rc.d remove in %v", rec.calls)
	}
}

func TestUninstallSysVinit_ChkconfigBranch(t *testing.T) {
	m, rec := newStubManager(t, "linux", "chkconfig")
	mkroot(t, m, "/etc/init.d")
	if err := m.Uninstall(); err != nil {
		t.Fatalf("Uninstall() error: %v", err)
	}
	if !rec.has("chkconfig", "--del", "vidveil") {
		t.Errorf("missing chkconfig --del in %v", rec.calls)
	}
}

// --- runit install/uninstall ---

func TestInstallRunit_CreatesServiceDir(t *testing.T) {
	m, _ := newStubManager(t, "linux", "sv")
	mkroot(t, m, "/etc/sv")
	mkroot(t, m, "/etc/service")
	if err := m.Install(); err != nil {
		t.Fatalf("Install() error: %v", err)
	}
	data, err := os.ReadFile(m.path("/etc/sv", "vidveil", "run"))
	if err != nil {
		t.Fatalf("read run script: %v", err)
	}
	if !strings.Contains(string(data), "exec /usr/local/bin/vidveil") {
		t.Errorf("run script missing exec line: %q", string(data))
	}
	logData, err := os.ReadFile(m.path("/etc/sv", "vidveil", "log", "run"))
	if err != nil {
		t.Fatalf("read log run script: %v", err)
	}
	if !strings.Contains(string(logData), "svlogd") {
		t.Errorf("log script missing svlogd: %q", string(logData))
	}
	link, err := os.Readlink(m.path("/etc/service", "vidveil"))
	if err != nil {
		t.Fatalf("readlink service symlink: %v", err)
	}
	if link != m.path("/etc/sv", "vidveil") {
		t.Errorf("symlink target = %q, want %q", link, m.path("/etc/sv", "vidveil"))
	}
}

func TestUninstallRunit_RemovesAll(t *testing.T) {
	m, rec := newStubManager(t, "linux", "sv")
	mkroot(t, m, "/etc/sv")
	mkroot(t, m, "/etc/service")
	if err := m.Install(); err != nil {
		t.Fatalf("Install() error: %v", err)
	}
	if err := m.Uninstall(); err != nil {
		t.Fatalf("Uninstall() error: %v", err)
	}
	if _, err := os.Stat(m.path("/etc/sv", "vidveil")); !os.IsNotExist(err) {
		t.Error("service dir still exists after Uninstall()")
	}
	if !rec.has("sv", "stop", "vidveil") {
		t.Errorf("missing sv stop in %v", rec.calls)
	}
}

// --- darwin (launchd) ---

func TestDarwinLifecycle(t *testing.T) {
	m, rec := newStubManager(t, "darwin")
	if err := m.Start(); err != nil {
		t.Fatalf("Start() error: %v", err)
	}
	if err := m.Stop(); err != nil {
		t.Fatalf("Stop() error: %v", err)
	}
	if err := m.Restart(); err != nil {
		t.Fatalf("Restart() error: %v", err)
	}
	if err := m.Reload(); err != nil {
		t.Fatalf("Reload() error: %v", err)
	}
	label := "io.github.apimgr.vidveil"
	if !rec.has("launchctl", "start", label) || !rec.has("launchctl", "stop", label) {
		t.Errorf("missing launchctl start/stop in %v", rec.calls)
	}
	if !rec.has("launchctl", "kickstart", "-k", "gui/"+label) {
		t.Errorf("missing launchctl kickstart in %v", rec.calls)
	}
}

func TestDarwinInstall_WritesPlist(t *testing.T) {
	m, rec := newStubManager(t, "darwin")
	mkroot(t, m, "/Library/LaunchDaemons")
	if err := m.Install(); err != nil {
		t.Fatalf("Install() error: %v", err)
	}
	plistPath := m.path("/Library/LaunchDaemons", "io.github.apimgr.vidveil.plist")
	data, err := os.ReadFile(plistPath)
	if err != nil {
		t.Fatalf("read plist: %v", err)
	}
	plist := string(data)
	for _, want := range []string{"<string>io.github.apimgr.vidveil</string>", "<string>/usr/local/bin/vidveil</string>", "<key>RunAtLoad</key>"} {
		if !strings.Contains(plist, want) {
			t.Errorf("plist missing %q", want)
		}
	}
	if !rec.has("launchctl", "load", plistPath) {
		t.Errorf("missing launchctl load in %v", rec.calls)
	}
}

func TestDarwinUninstall_RemovesPlist(t *testing.T) {
	m, rec := newStubManager(t, "darwin")
	mkroot(t, m, "/Library/LaunchDaemons")
	plistPath := m.path("/Library/LaunchDaemons", "io.github.apimgr.vidveil.plist")
	if err := os.WriteFile(plistPath, []byte("<plist/>\n"), 0644); err != nil {
		t.Fatalf("seed plist: %v", err)
	}
	if err := m.Uninstall(); err != nil {
		t.Fatalf("Uninstall() error: %v", err)
	}
	if _, err := os.Stat(plistPath); !os.IsNotExist(err) {
		t.Error("plist still exists after Uninstall()")
	}
	if !rec.has("launchctl", "unload", plistPath) {
		t.Errorf("missing launchctl unload in %v", rec.calls)
	}
}

func TestDarwinDisable(t *testing.T) {
	m, rec := newStubManager(t, "darwin")
	if err := m.Disable(); err != nil {
		t.Fatalf("Disable() error: %v", err)
	}
	if !rec.has("launchctl", "unload", "-w") {
		t.Errorf("missing launchctl unload -w in %v", rec.calls)
	}
}

// --- windows (sc) ---

func TestWindowsLifecycle(t *testing.T) {
	m, rec := newStubManager(t, "windows")
	if err := m.Start(); err != nil {
		t.Fatalf("Start() error: %v", err)
	}
	if err := m.Stop(); err != nil {
		t.Fatalf("Stop() error: %v", err)
	}
	if err := m.Restart(); err != nil {
		t.Fatalf("Restart() error: %v", err)
	}
	if err := m.Disable(); err != nil {
		t.Fatalf("Disable() error: %v", err)
	}
	if !rec.has("sc", "start", "vidveil") || !rec.has("sc", "stop", "vidveil") {
		t.Errorf("missing sc start/stop in %v", rec.calls)
	}
	if !rec.has("sc", "config", "vidveil", "start=", "disabled") {
		t.Errorf("missing sc config disabled in %v", rec.calls)
	}
}

func TestWindowsInstall_UsesVirtualServiceAccount(t *testing.T) {
	m, rec := newStubManager(t, "windows")
	if err := m.Install(); err != nil {
		t.Fatalf("Install() error: %v", err)
	}
	found := false
	for _, c := range rec.calls {
		if c[0] == "sc" && len(c) > 1 && c[1] == "create" {
			joined := strings.Join(c, " ")
			if strings.Contains(joined, `NT SERVICE\vidveil`) && strings.Contains(joined, "/usr/local/bin/vidveil") {
				found = true
			}
		}
	}
	if !found {
		t.Errorf("sc create with VSA not found in %v", rec.calls)
	}
}

func TestWindowsInstall_CreateFails(t *testing.T) {
	m, rec := newStubManager(t, "windows")
	rec.err = errors.New("access denied")
	if err := m.Install(); err == nil {
		t.Error("Install() error = nil, want create failure")
	}
}

func TestWindowsUninstall_DeletesService(t *testing.T) {
	m, rec := newStubManager(t, "windows")
	if err := m.Uninstall(); err != nil {
		t.Fatalf("Uninstall() error: %v", err)
	}
	if !rec.has("sc", "delete", "vidveil") {
		t.Errorf("missing sc delete in %v", rec.calls)
	}
}

// --- BSD (rc.d) ---

func TestBsdLifecycle(t *testing.T) {
	m, rec := newStubManager(t, "freebsd")
	if err := m.Start(); err != nil {
		t.Fatalf("Start() error: %v", err)
	}
	if err := m.Stop(); err != nil {
		t.Fatalf("Stop() error: %v", err)
	}
	if err := m.Restart(); err != nil {
		t.Fatalf("Restart() error: %v", err)
	}
	if err := m.Reload(); err != nil {
		t.Fatalf("Reload() error: %v", err)
	}
	for _, verb := range []string{"start", "stop", "restart", "reload"} {
		if !rec.has("service", "vidveil", verb) {
			t.Errorf("missing service vidveil %s in %v", verb, rec.calls)
		}
	}
}

func TestBsdInstall_WritesRcScriptAndEnables(t *testing.T) {
	m, _ := newStubManager(t, "freebsd")
	mkroot(t, m, "/usr/local/etc/rc.d")
	mkroot(t, m, "/etc")
	if err := m.Install(); err != nil {
		t.Fatalf("Install() error: %v", err)
	}
	data, err := os.ReadFile(m.path("/usr/local/etc/rc.d", "vidveil"))
	if err != nil {
		t.Fatalf("read rc script: %v", err)
	}
	script := string(data)
	for _, want := range []string{"PROVIDE: vidveil", `command="/usr/local/bin/vidveil"`, "run_rc_command"} {
		if !strings.Contains(script, want) {
			t.Errorf("rc script missing %q", want)
		}
	}
	conf, err := os.ReadFile(m.path("/etc/rc.conf.local"))
	if err != nil {
		t.Fatalf("read rc.conf.local: %v", err)
	}
	if !strings.Contains(string(conf), `vidveil_enable="YES"`) {
		t.Errorf("rc.conf.local missing enable line: %q", string(conf))
	}
	// Second install must not duplicate the enable line
	if err := m.Install(); err != nil {
		t.Fatalf("second Install() error: %v", err)
	}
	conf2, err := os.ReadFile(m.path("/etc/rc.conf.local"))
	if err != nil {
		t.Fatalf("re-read rc.conf.local: %v", err)
	}
	if strings.Count(string(conf2), `vidveil_enable="YES"`) != 1 {
		t.Errorf("enable line duplicated: %q", string(conf2))
	}
}

func TestBsdUninstall_RemovesRcScript(t *testing.T) {
	m, rec := newStubManager(t, "freebsd")
	mkroot(t, m, "/usr/local/etc/rc.d")
	rcPath := m.path("/usr/local/etc/rc.d", "vidveil")
	if err := os.WriteFile(rcPath, []byte("#!/bin/sh\n"), 0755); err != nil {
		t.Fatalf("seed rc script: %v", err)
	}
	if err := m.Uninstall(); err != nil {
		t.Fatalf("Uninstall() error: %v", err)
	}
	if _, err := os.Stat(rcPath); !os.IsNotExist(err) {
		t.Error("rc script still exists after Uninstall()")
	}
	if !rec.has("service", "vidveil", "stop") {
		t.Errorf("missing service stop in %v", rec.calls)
	}
}

func TestBsdDisable_RewritesEnableLine(t *testing.T) {
	m, _ := newStubManager(t, "freebsd")
	mkroot(t, m, "/etc")
	rcConf := m.path("/etc/rc.conf.local")
	if err := os.WriteFile(rcConf, []byte("vidveil_enable=\"YES\"\n"), 0644); err != nil {
		t.Fatalf("seed rc.conf.local: %v", err)
	}
	if err := m.Disable(); err != nil {
		t.Fatalf("Disable() error: %v", err)
	}
	data, err := os.ReadFile(rcConf)
	if err != nil {
		t.Fatalf("read rc.conf.local: %v", err)
	}
	if !strings.Contains(string(data), `vidveil_enable="NO"`) {
		t.Errorf("rc.conf.local not disabled: %q", string(data))
	}
}

func TestBsdDisable_MissingConfErrors(t *testing.T) {
	m, _ := newStubManager(t, "freebsd")
	if err := m.Disable(); err == nil {
		t.Error("Disable() error = nil, want read failure")
	}
}

// --- status ---

func TestGetServiceStatus_SystemdActive(t *testing.T) {
	m, rec := newStubManager(t, "linux", "systemctl")
	rec.out = []byte("active\n")
	status, err := m.GetServiceStatus()
	if err != nil {
		t.Fatalf("GetServiceStatus() error: %v", err)
	}
	if status != "running" {
		t.Errorf("status = %q, want running", status)
	}
}

func TestGetServiceStatus_SystemdInactive(t *testing.T) {
	m, rec := newStubManager(t, "linux", "systemctl")
	rec.out = []byte("inactive\n")
	rec.outErr = errors.New("exit status 3")
	status, err := m.GetServiceStatus()
	if err != nil {
		t.Fatalf("GetServiceStatus() error: %v", err)
	}
	if status != "stopped" {
		t.Errorf("status = %q, want stopped", status)
	}
}

func TestGetServiceStatus_OpenRCStarted(t *testing.T) {
	m, rec := newStubManager(t, "linux")
	enableOpenRC(t, m)
	rec.out = []byte(" * status: started\n")
	status, err := m.GetServiceStatus()
	if err != nil {
		t.Fatalf("GetServiceStatus() error: %v", err)
	}
	if status != "running" {
		t.Errorf("status = %q, want running", status)
	}
}

func TestGetServiceStatus_SysVinitRunning(t *testing.T) {
	m, rec := newStubManager(t, "linux", "update-rc.d")
	mkroot(t, m, "/etc/init.d")
	rec.out = []byte("vidveil is running (pid 42)\n")
	status, err := m.GetServiceStatus()
	if err != nil {
		t.Fatalf("GetServiceStatus() error: %v", err)
	}
	if status != "running" {
		t.Errorf("status = %q, want running", status)
	}
}

func TestGetServiceStatus_RunitRunning(t *testing.T) {
	m, rec := newStubManager(t, "linux", "sv")
	rec.out = []byte("run: vidveil: (pid 42) 100s\n")
	status, err := m.GetServiceStatus()
	if err != nil {
		t.Fatalf("GetServiceStatus() error: %v", err)
	}
	if status != "running" {
		t.Errorf("status = %q, want running", status)
	}
}

func TestGetServiceStatus_NoInitSystem(t *testing.T) {
	m, _ := newStubManager(t, "linux")
	if _, err := m.GetServiceStatus(); err == nil {
		t.Error("GetServiceStatus() error = nil, want no-service-manager error")
	}
}

func TestGetServiceStatus_Darwin(t *testing.T) {
	m, rec := newStubManager(t, "darwin")
	rec.out = []byte(`{"PID" = 42;}`)
	status, err := m.GetServiceStatus()
	if err != nil {
		t.Fatalf("GetServiceStatus() error: %v", err)
	}
	if status != "running" {
		t.Errorf("status = %q, want running", status)
	}
	rec.outErr = errors.New("not loaded")
	status, err = m.GetServiceStatus()
	if err != nil {
		t.Fatalf("GetServiceStatus() error: %v", err)
	}
	if status != "stopped" {
		t.Errorf("status = %q, want stopped", status)
	}
}

func TestGetServiceStatus_Windows(t *testing.T) {
	m, rec := newStubManager(t, "windows")
	rec.out = []byte("STATE : 4 RUNNING\n")
	status, err := m.GetServiceStatus()
	if err != nil {
		t.Fatalf("GetServiceStatus() error: %v", err)
	}
	if status != "running" {
		t.Errorf("status = %q, want running", status)
	}
	rec.out = []byte("STATE : 1 STOPPED\n")
	status, _ = m.GetServiceStatus()
	if status != "stopped" {
		t.Errorf("status = %q, want stopped", status)
	}
	rec.out = []byte("STATE : 2 START_PENDING\n")
	status, _ = m.GetServiceStatus()
	if status != "unknown" {
		t.Errorf("status = %q, want unknown", status)
	}
}

func TestGetServiceStatus_Bsd(t *testing.T) {
	m, rec := newStubManager(t, "freebsd")
	rec.out = []byte("vidveil is running as pid 42.\n")
	status, err := m.GetServiceStatus()
	if err != nil {
		t.Fatalf("GetServiceStatus() error: %v", err)
	}
	if status != "running" {
		t.Errorf("status = %q, want running", status)
	}
	rec.outErr = errors.New("vidveil is not running")
	status, _ = m.GetServiceStatus()
	if status != "stopped" {
		t.Errorf("status = %q, want stopped", status)
	}
}

// --- real command helpers ---

func TestRunCmd_Success(t *testing.T) {
	if err := runCmd("sh", "-c", "exit 0"); err != nil {
		t.Errorf("runCmd() error: %v", err)
	}
}

func TestRunCmd_FailureIncludesOutput(t *testing.T) {
	err := runCmd("sh", "-c", "echo boom >&2; exit 1")
	if err == nil {
		t.Fatal("runCmd() error = nil, want failure")
	}
	if !strings.Contains(err.Error(), "boom") {
		t.Errorf("error missing command output: %v", err)
	}
}

func TestRunCmdOutput_ReturnsCombined(t *testing.T) {
	out, err := runCmdOutput("sh", "-c", "echo hello")
	if err != nil {
		t.Fatalf("runCmdOutput() error: %v", err)
	}
	if !strings.Contains(string(out), "hello") {
		t.Errorf("output = %q, want hello", string(out))
	}
}
