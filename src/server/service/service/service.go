// SPDX-License-Identifier: MIT
package service

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/apimgr/vidveil/src/common/terminal"
)

// SystemServiceManager handles system service management
type SystemServiceManager struct {
	name        string
	displayName string
	description string
	execPath    string
	// rootPrefix is prepended to all absolute filesystem paths; empty in production, a temp dir in tests
	rootPrefix string
	// goos selects the OS-specific implementation; runtime.GOOS in production, overridden in tests
	goos string
	// run executes a command discarding output; runCmd in production, a recording stub in tests
	run func(name string, args ...string) error
	// runOut executes a command returning combined output; runCmdOutput in production, a stub in tests
	runOut func(name string, args ...string) ([]byte, error)
	// lookPath resolves a binary on PATH; exec.LookPath in production, a stub in tests
	lookPath func(file string) (string, error)
}

// NewSystemServiceManager creates a new service manager
func NewSystemServiceManager(name, displayName, description string) (*SystemServiceManager, error) {
	execPath, err := os.Executable()
	if err != nil {
		return nil, fmt.Errorf("failed to get executable path: %w", err)
	}

	return &SystemServiceManager{
		name:        name,
		displayName: displayName,
		description: description,
		execPath:    execPath,
		goos:        runtime.GOOS,
		run:         runCmd,
		runOut:      runCmdOutput,
		lookPath:    exec.LookPath,
	}, nil
}

// path joins the manager's root prefix with the given absolute path elements
func (m *SystemServiceManager) path(elem ...string) string {
	return filepath.Join(append([]string{m.rootPrefix}, elem...)...)
}

// Start starts the service
func (m *SystemServiceManager) Start() error {
	switch m.goos {
	case "linux":
		return m.linuxStart()
	case "darwin":
		return m.darwinStart()
	case "windows":
		return m.windowsStart()
	case "freebsd", "openbsd", "netbsd":
		return m.bsdStart()
	default:
		return fmt.Errorf("unsupported OS: %s", m.goos)
	}
}

// Stop stops the service
func (m *SystemServiceManager) Stop() error {
	switch m.goos {
	case "linux":
		return m.linuxStop()
	case "darwin":
		return m.darwinStop()
	case "windows":
		return m.windowsStop()
	case "freebsd", "openbsd", "netbsd":
		return m.bsdStop()
	default:
		return fmt.Errorf("unsupported OS: %s", m.goos)
	}
}

// Restart restarts the service
func (m *SystemServiceManager) Restart() error {
	switch m.goos {
	case "linux":
		return m.linuxRestart()
	case "darwin":
		return m.darwinRestart()
	case "windows":
		return m.windowsRestart()
	case "freebsd", "openbsd", "netbsd":
		return m.bsdRestart()
	default:
		return fmt.Errorf("unsupported OS: %s", m.goos)
	}
}

// Reload sends SIGHUP to reload configuration
func (m *SystemServiceManager) Reload() error {
	switch m.goos {
	case "linux":
		return m.linuxReload()
	case "darwin":
		return m.darwinReload()
	case "windows":
		return fmt.Errorf("reload not supported on Windows, use restart")
	case "freebsd", "openbsd", "netbsd":
		return m.bsdReload()
	default:
		return fmt.Errorf("unsupported OS: %s", m.goos)
	}
}

// Install installs the service
func (m *SystemServiceManager) Install() error {
	switch m.goos {
	case "linux":
		return m.linuxInstall()
	case "darwin":
		return m.darwinInstall()
	case "windows":
		return m.windowsInstall()
	case "freebsd", "openbsd", "netbsd":
		return m.bsdInstall()
	default:
		return fmt.Errorf("unsupported OS: %s", m.goos)
	}
}

// Uninstall removes the service
func (m *SystemServiceManager) Uninstall() error {
	switch m.goos {
	case "linux":
		return m.linuxUninstall()
	case "darwin":
		return m.darwinUninstall()
	case "windows":
		return m.windowsUninstall()
	case "freebsd", "openbsd", "netbsd":
		return m.bsdUninstall()
	default:
		return fmt.Errorf("unsupported OS: %s", m.goos)
	}
}

// Disable disables the service from starting at boot
func (m *SystemServiceManager) Disable() error {
	switch m.goos {
	case "linux":
		return m.linuxDisable()
	case "darwin":
		return m.darwinDisable()
	case "windows":
		return m.windowsDisable()
	case "freebsd", "openbsd", "netbsd":
		return m.bsdDisable()
	default:
		return fmt.Errorf("unsupported OS: %s", m.goos)
	}
}

// GetServiceStatus returns the current service status per AI.md PART 24
func (m *SystemServiceManager) GetServiceStatus() (string, error) {
	switch m.goos {
	case "linux":
		return m.linuxStatus()
	case "darwin":
		return m.darwinStatus()
	case "windows":
		return m.windowsStatus()
	case "freebsd", "openbsd", "netbsd":
		return m.bsdStatus()
	default:
		return "unknown", fmt.Errorf("unsupported OS: %s", m.goos)
	}
}

// linuxStatus returns service status on Linux
func (m *SystemServiceManager) linuxStatus() (string, error) {
	if m.hasSystemd() {
		out, err := m.runOut("systemctl", "is-active", m.name)
		status := strings.TrimSpace(string(out))
		if err != nil {
			if status == "inactive" || status == "dead" {
				return "stopped", nil
			}
			return "stopped", nil
		}
		if status == "active" {
			return "running", nil
		}
		return status, nil
	}
	if m.hasOpenRC() {
		out, err := m.runOut("rc-service", m.name, "status")
		if err != nil {
			return "stopped", nil
		}
		if strings.Contains(string(out), "started") {
			return "running", nil
		}
		return "stopped", nil
	}
	if m.hasSysVinit() {
		initScript := m.path("/etc/init.d", m.name)
		out, err := m.runOut(initScript, "status")
		if err != nil {
			return "stopped", nil
		}
		outStr := strings.ToLower(string(out))
		if strings.Contains(outStr, "running") {
			return "running", nil
		}
		return "stopped", nil
	}
	if m.hasRunit() {
		out, err := m.runOut("sv", "status", m.name)
		if err != nil {
			return "stopped", nil
		}
		if strings.HasPrefix(string(out), "run:") {
			return "running", nil
		}
		return "stopped", nil
	}
	return "unknown", fmt.Errorf("no supported service manager found")
}

// darwinStatus returns service status on macOS
func (m *SystemServiceManager) darwinStatus() (string, error) {
	out, err := m.runOut("launchctl", "list", m.launchdLabel())
	if err != nil {
		return "stopped", nil
	}
	if len(out) > 0 {
		return "running", nil
	}
	return "stopped", nil
}

// windowsStatus returns service status on Windows
func (m *SystemServiceManager) windowsStatus() (string, error) {
	out, err := m.runOut("sc", "query", m.name)
	if err != nil {
		return "stopped", nil
	}
	if strings.Contains(string(out), "RUNNING") {
		return "running", nil
	}
	if strings.Contains(string(out), "STOPPED") {
		return "stopped", nil
	}
	return "unknown", nil
}

// bsdStatus returns service status on BSD
func (m *SystemServiceManager) bsdStatus() (string, error) {
	out, err := m.runOut("service", m.name, "status")
	if err != nil {
		return "stopped", nil
	}
	if strings.Contains(strings.ToLower(string(out)), "running") {
		return "running", nil
	}
	return "stopped", nil
}

// Linux - systemd, OpenRC, SysVinit, and runit support
func (m *SystemServiceManager) linuxStart() error {
	if m.hasSystemd() {
		return m.run("systemctl", "start", m.name)
	}
	if m.hasOpenRC() {
		return m.run("rc-service", m.name, "start")
	}
	if m.hasSysVinit() {
		return m.run(m.path("/etc/init.d", m.name), "start")
	}
	if m.hasRunit() {
		return m.run("sv", "start", m.name)
	}
	return fmt.Errorf("no supported service manager found")
}

func (m *SystemServiceManager) linuxStop() error {
	if m.hasSystemd() {
		return m.run("systemctl", "stop", m.name)
	}
	if m.hasOpenRC() {
		return m.run("rc-service", m.name, "stop")
	}
	if m.hasSysVinit() {
		return m.run(m.path("/etc/init.d", m.name), "stop")
	}
	if m.hasRunit() {
		return m.run("sv", "stop", m.name)
	}
	return fmt.Errorf("no supported service manager found")
}

func (m *SystemServiceManager) linuxRestart() error {
	if m.hasSystemd() {
		return m.run("systemctl", "restart", m.name)
	}
	if m.hasOpenRC() {
		return m.run("rc-service", m.name, "restart")
	}
	if m.hasSysVinit() {
		return m.run(m.path("/etc/init.d", m.name), "restart")
	}
	if m.hasRunit() {
		return m.run("sv", "restart", m.name)
	}
	return fmt.Errorf("no supported service manager found")
}

func (m *SystemServiceManager) linuxReload() error {
	if m.hasSystemd() {
		return m.run("systemctl", "reload", m.name)
	}
	if m.hasOpenRC() {
		return m.run("rc-service", m.name, "reload")
	}
	if m.hasSysVinit() {
		return m.run(m.path("/etc/init.d", m.name), "reload")
	}
	if m.hasRunit() {
		return m.run("sv", "hup", m.name)
	}
	return fmt.Errorf("no supported service manager found")
}

func (m *SystemServiceManager) linuxInstall() error {
	if m.hasSystemd() {
		return m.installSystemd()
	}
	if m.hasOpenRC() {
		return m.installOpenRC()
	}
	if m.hasSysVinit() {
		return m.installSysVinit()
	}
	if m.hasRunit() {
		return m.installRunit()
	}
	return fmt.Errorf("no supported service manager found (need systemd, OpenRC, SysVinit, or runit)")
}

func (m *SystemServiceManager) linuxUninstall() error {
	if m.hasSystemd() {
		return m.uninstallSystemd()
	}
	if m.hasOpenRC() {
		return m.uninstallOpenRC()
	}
	if m.hasSysVinit() {
		return m.uninstallSysVinit()
	}
	if m.hasRunit() {
		return m.uninstallRunit()
	}
	return fmt.Errorf("no supported service manager found")
}

func (m *SystemServiceManager) linuxDisable() error {
	if m.hasSystemd() {
		return m.run("systemctl", "disable", m.name)
	}
	if m.hasOpenRC() {
		return m.run("rc-update", "del", m.name, "default")
	}
	if m.hasSysVinit() {
		if _, err := m.lookPath("update-rc.d"); err == nil {
			return m.run("update-rc.d", m.name, "remove")
		}
		return m.run("chkconfig", "--del", m.name)
	}
	if m.hasRunit() {
		runPath := m.path("/etc/service", m.name)
		return os.Remove(runPath)
	}
	return fmt.Errorf("no supported service manager found")
}

func (m *SystemServiceManager) hasSystemd() bool {
	_, err := m.lookPath("systemctl")
	return err == nil
}

func (m *SystemServiceManager) hasOpenRC() bool {
	_, err := os.Stat(m.path("/sbin/openrc-run"))
	return err == nil
}

func (m *SystemServiceManager) hasSysVinit() bool {
	if m.hasSystemd() || m.hasOpenRC() {
		return false
	}
	if _, err := os.Stat(m.path("/etc/init.d")); err != nil {
		return false
	}
	_, errUpd := m.lookPath("update-rc.d")
	_, errChk := m.lookPath("chkconfig")
	return errUpd == nil || errChk == nil
}

func (m *SystemServiceManager) hasRunit() bool {
	_, err := m.lookPath("sv")
	return err == nil
}

func (m *SystemServiceManager) installSystemd() error {
	unitPath := m.path("/etc/systemd/system", m.name+".service")

	// Per AI.md PART 24: systemd unit file with security hardening
	content := fmt.Sprintf(`[Unit]
Description=%s
Documentation=https://apimgr.github.io/%s
After=network-online.target
Wants=network-online.target

[Service]
Type=simple
User=%s
Group=%s
ExecStart=%s
Restart=on-failure
RestartSec=5
StandardOutput=journal
StandardError=journal

# Security hardening per AI.md PART 24
NoNewPrivileges=yes
ProtectSystem=strict
ProtectHome=yes
PrivateTmp=yes
ReadWritePaths=/etc/apimgr/%s
ReadWritePaths=/var/lib/apimgr/%s
ReadWritePaths=/var/cache/apimgr/%s
ReadWritePaths=/var/log/apimgr/%s

[Install]
WantedBy=multi-user.target
`, m.description, m.name, m.name, m.name, m.execPath, m.name, m.name, m.name, m.name)

	if err := os.WriteFile(unitPath, []byte(content), 0644); err != nil {
		return fmt.Errorf("failed to write unit file: %w", err)
	}

	if err := m.run("systemctl", "daemon-reload"); err != nil {
		return fmt.Errorf("failed to reload systemd: %w", err)
	}

	if err := m.run("systemctl", "enable", m.name); err != nil {
		return fmt.Errorf("failed to enable service: %w", err)
	}

	fmt.Printf("%s Installed systemd service: %s\n", terminal.StatusIcon(true), unitPath)
	return nil
}

func (m *SystemServiceManager) uninstallSystemd() error {
	_ = m.run("systemctl", "stop", m.name)
	_ = m.run("systemctl", "disable", m.name)

	unitPath := m.path("/etc/systemd/system", m.name+".service")
	if err := os.Remove(unitPath); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("failed to remove unit file: %w", err)
	}

	_ = m.run("systemctl", "daemon-reload")

	fmt.Printf("%s Uninstalled systemd service: %s\n", terminal.StatusIcon(true), m.name)
	return nil
}

func (m *SystemServiceManager) installOpenRC() error {
	initPath := m.path("/etc/init.d", m.name)

	// Per AI.md PART 24: OpenRC init script
	content := fmt.Sprintf(`#!/sbin/openrc-run
# Service identity comes from the internal name so config/data paths stay
# stable across binary renames.

name="%s"
description="%s"
command="%s"
command_args=""
command_user="%s:%s"
pidfile="/var/run/apimgr/%s.pid"
command_background=true
output_log="/var/log/apimgr/%s/server.log"
error_log="/var/log/apimgr/%s/error.log"

depend() {
    need net
    after firewall
    use dns logger
}

start_pre() {
    checkpath -d -m 0755 -o %s:%s /var/run/apimgr
    checkpath -d -m 0755 -o %s:%s /var/log/apimgr/%s
}
`, m.name, m.description, m.execPath,
		m.name, m.name,
		m.name,
		m.name,
		m.name,
		m.name, m.name,
		m.name, m.name, m.name)

	if err := os.WriteFile(initPath, []byte(content), 0755); err != nil {
		return fmt.Errorf("failed to write OpenRC init script: %w", err)
	}

	if err := m.run("rc-update", "add", m.name, "default"); err != nil {
		return fmt.Errorf("failed to enable OpenRC service: %w", err)
	}

	fmt.Printf("%s Installed OpenRC service: %s\n", terminal.StatusIcon(true), initPath)
	return nil
}

func (m *SystemServiceManager) uninstallOpenRC() error {
	_ = m.run("rc-service", m.name, "stop")
	_ = m.run("rc-update", "del", m.name, "default")

	initPath := m.path("/etc/init.d", m.name)
	if err := os.Remove(initPath); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("failed to remove OpenRC init script: %w", err)
	}

	fmt.Printf("%s Uninstalled OpenRC service: %s\n", terminal.StatusIcon(true), m.name)
	return nil
}

func (m *SystemServiceManager) installSysVinit() error {
	initPath := m.path("/etc/init.d", m.name)

	// Per AI.md PART 24: SysVinit init script
	content := fmt.Sprintf(`#!/bin/sh
### BEGIN INIT INFO
# Provides:          %s
# Required-Start:    $network $remote_fs $syslog
# Required-Stop:     $network $remote_fs $syslog
# Default-Start:     2 3 4 5
# Default-Stop:      0 1 6
# Short-Description: %s
# Description:       %s daemon
### END INIT INFO

NAME=%s
DAEMON=%s
DAEMON_USER=%s
PIDFILE=/var/run/apimgr/%s.pid
LOGFILE=/var/log/apimgr/%s/server.log

case "$1" in
    start)
        echo "Starting $NAME..."
        mkdir -p $(dirname $PIDFILE) $(dirname $LOGFILE)
        chown -R $DAEMON_USER:$DAEMON_USER $(dirname $PIDFILE) $(dirname $LOGFILE)
        start-stop-daemon --start --quiet --background --make-pidfile \
            --pidfile $PIDFILE --chuid $DAEMON_USER --exec $DAEMON \
            --no-close >> $LOGFILE 2>&1
        ;;
    stop)
        echo "Stopping $NAME..."
        start-stop-daemon --stop --quiet --pidfile $PIDFILE --retry 30
        rm -f $PIDFILE
        ;;
    restart)
        $0 stop
        sleep 1
        $0 start
        ;;
    reload)
        echo "Reloading $NAME..."
        start-stop-daemon --stop --signal HUP --quiet --pidfile $PIDFILE
        ;;
    status)
        if [ -f $PIDFILE ] && kill -0 $(cat $PIDFILE) 2>/dev/null; then
            echo "$NAME is running (pid $(cat $PIDFILE))"
            exit 0
        else
            echo "$NAME is stopped"
            exit 3
        fi
        ;;
    *)
        echo "Usage: $0 {start|stop|restart|reload|status}"
        exit 1
        ;;
esac
exit 0
`, m.name, m.description, m.description,
		m.name, m.execPath, m.name,
		m.name, m.name)

	if err := os.WriteFile(initPath, []byte(content), 0755); err != nil {
		return fmt.Errorf("failed to write SysVinit script: %w", err)
	}

	if _, err := m.lookPath("update-rc.d"); err == nil {
		if err := m.run("update-rc.d", m.name, "defaults"); err != nil {
			return fmt.Errorf("failed to enable SysVinit service: %w", err)
		}
	} else {
		if err := m.run("chkconfig", "--add", m.name); err != nil {
			return fmt.Errorf("failed to add SysVinit service: %w", err)
		}
		if err := m.run("chkconfig", m.name, "on"); err != nil {
			return fmt.Errorf("failed to enable SysVinit service: %w", err)
		}
	}

	fmt.Printf("%s Installed SysVinit service: %s\n", terminal.StatusIcon(true), initPath)
	return nil
}

func (m *SystemServiceManager) uninstallSysVinit() error {
	initPath := m.path("/etc/init.d", m.name)

	_ = m.run(initPath, "stop")

	if _, err := m.lookPath("update-rc.d"); err == nil {
		_ = m.run("update-rc.d", m.name, "remove")
	} else {
		_ = m.run("chkconfig", "--del", m.name)
	}

	if err := os.Remove(initPath); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("failed to remove SysVinit script: %w", err)
	}

	fmt.Printf("%s Uninstalled SysVinit service: %s\n", terminal.StatusIcon(true), m.name)
	return nil
}

func (m *SystemServiceManager) installRunit() error {
	svcDir := m.path("/etc/sv", m.name)
	runPath := m.path("/etc/service", m.name)

	if err := os.MkdirAll(svcDir, 0755); err != nil {
		return fmt.Errorf("failed to create service directory: %w", err)
	}

	runScript := fmt.Sprintf(`#!/bin/sh
exec 2>&1
exec %s
`, m.execPath)

	runFile := filepath.Join(svcDir, "run")
	if err := os.WriteFile(runFile, []byte(runScript), 0755); err != nil {
		return fmt.Errorf("failed to write run script: %w", err)
	}

	// Create log directory and run script
	logDir := filepath.Join(svcDir, "log")
	if err := os.MkdirAll(logDir, 0755); err != nil {
		return fmt.Errorf("failed to create log directory: %w", err)
	}

	logScript := `#!/bin/sh
exec svlogd -tt ./main
`
	logFile := filepath.Join(logDir, "run")
	if err := os.WriteFile(logFile, []byte(logScript), 0755); err != nil {
		return fmt.Errorf("failed to write log script: %w", err)
	}

	// Symlink to enable
	if err := os.Symlink(svcDir, runPath); err != nil && !os.IsExist(err) {
		return fmt.Errorf("failed to enable service: %w", err)
	}

	fmt.Printf("%s Installed runit service: %s\n", terminal.StatusIcon(true), svcDir)
	return nil
}

func (m *SystemServiceManager) uninstallRunit() error {
	runPath := m.path("/etc/service", m.name)
	svcDir := m.path("/etc/sv", m.name)

	// Stop first
	_ = m.run("sv", "stop", m.name)

	// Remove symlink
	if err := os.Remove(runPath); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("failed to remove service symlink: %w", err)
	}

	// Remove service directory
	if err := os.RemoveAll(svcDir); err != nil {
		return fmt.Errorf("failed to remove service directory: %w", err)
	}

	fmt.Printf("%s Uninstalled runit service: %s\n", terminal.StatusIcon(true), m.name)
	return nil
}

// macOS - launchd support
func (m *SystemServiceManager) darwinStart() error {
	return m.run("launchctl", "start", m.launchdLabel())
}

func (m *SystemServiceManager) darwinStop() error {
	return m.run("launchctl", "stop", m.launchdLabel())
}

func (m *SystemServiceManager) darwinRestart() error {
	_ = m.darwinStop()
	return m.darwinStart()
}

func (m *SystemServiceManager) darwinReload() error {
	return m.run("launchctl", "kickstart", "-k", "gui/"+m.launchdLabel())
}

func (m *SystemServiceManager) darwinInstall() error {
	plistPath := m.launchdPlistPath()

	content := fmt.Sprintf(`<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
    <key>Label</key>
    <string>%s</string>
    <key>ProgramArguments</key>
    <array>
        <string>%s</string>
    </array>
    <key>RunAtLoad</key>
    <true/>
    <key>KeepAlive</key>
    <true/>
    <key>StandardOutPath</key>
    <string>/var/log/apimgr/%s/stdout.log</string>
    <key>StandardErrorPath</key>
    <string>/var/log/apimgr/%s/stderr.log</string>
</dict>
</plist>
`, m.launchdLabel(), m.execPath, m.name, m.name)

	if err := os.WriteFile(plistPath, []byte(content), 0644); err != nil {
		return fmt.Errorf("failed to write plist: %w", err)
	}

	if err := m.run("launchctl", "load", plistPath); err != nil {
		return fmt.Errorf("failed to load service: %w", err)
	}

	fmt.Printf("%s Installed launchd service: %s\n", terminal.StatusIcon(true), plistPath)
	return nil
}

func (m *SystemServiceManager) darwinUninstall() error {
	plistPath := m.launchdPlistPath()

	_ = m.run("launchctl", "unload", plistPath)

	if err := os.Remove(plistPath); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("failed to remove plist: %w", err)
	}

	fmt.Printf("%s Uninstalled launchd service: %s\n", terminal.StatusIcon(true), m.name)
	return nil
}

func (m *SystemServiceManager) darwinDisable() error {
	return m.run("launchctl", "unload", "-w", m.launchdPlistPath())
}

func (m *SystemServiceManager) launchdLabel() string {
	// Per AI.md PART 24: plist_name = io.github.apimgr.vidveil
	return "io.github.apimgr." + m.name
}

func (m *SystemServiceManager) launchdPlistPath() string {
	return m.path("/Library/LaunchDaemons", m.launchdLabel()+".plist")
}

// Windows - Windows Service Manager
func (m *SystemServiceManager) windowsStart() error {
	return m.run("sc", "start", m.name)
}

func (m *SystemServiceManager) windowsStop() error {
	return m.run("sc", "stop", m.name)
}

func (m *SystemServiceManager) windowsRestart() error {
	_ = m.windowsStop()
	return m.windowsStart()
}

func (m *SystemServiceManager) windowsInstall() error {
	// Create service using Virtual Service Account (NT SERVICE\{name}) per AI.md PART 24.
	// VSA is a minimal-privilege isolated account auto-managed by Windows — no
	// privilege dropping needed.
	vsaName := `NT SERVICE\` + m.name
	err := m.run("sc", "create", m.name,
		"binPath=", m.execPath,
		"DisplayName=", m.displayName,
		"start=", "auto",
		"obj=", vsaName,
	)
	if err != nil {
		return fmt.Errorf("failed to create service: %w", err)
	}

	// Set description
	_ = m.run("sc", "description", m.name, m.description)

	fmt.Printf("%s Installed Windows service: %s (runs as %s)\n",
		terminal.StatusIcon(true), m.name, vsaName)
	return nil
}

func (m *SystemServiceManager) windowsUninstall() error {
	_ = m.windowsStop()

	if err := m.run("sc", "delete", m.name); err != nil {
		return fmt.Errorf("failed to delete service: %w", err)
	}

	fmt.Printf("%s Uninstalled Windows service: %s\n", terminal.StatusIcon(true), m.name)
	return nil
}

func (m *SystemServiceManager) windowsDisable() error {
	return m.run("sc", "config", m.name, "start=", "disabled")
}

// BSD - rc.d support
func (m *SystemServiceManager) bsdStart() error {
	return m.run("service", m.name, "start")
}

func (m *SystemServiceManager) bsdStop() error {
	return m.run("service", m.name, "stop")
}

func (m *SystemServiceManager) bsdRestart() error {
	return m.run("service", m.name, "restart")
}

func (m *SystemServiceManager) bsdReload() error {
	return m.run("service", m.name, "reload")
}

func (m *SystemServiceManager) bsdInstall() error {
	rcPath := m.path("/usr/local/etc/rc.d", m.name)

	content := fmt.Sprintf(`#!/bin/sh
#
# PROVIDE: %s
# REQUIRE: NETWORKING
# KEYWORD: shutdown

. /etc/rc.subr

name="%s"
rcvar="${name}_enable"
command="%s"
command_args=""
pidfile="/var/run/${name}.pid"

load_rc_config $name
run_rc_command "$1"
`, m.name, m.name, m.execPath)

	if err := os.WriteFile(rcPath, []byte(content), 0755); err != nil {
		return fmt.Errorf("failed to write rc script: %w", err)
	}

	// Enable in rc.conf
	rcConf := m.path("/etc/rc.conf.local")
	enableLine := fmt.Sprintf(`%s_enable="YES"`, m.name)

	data, _ := os.ReadFile(rcConf)
	if !strings.Contains(string(data), enableLine) {
		f, err := os.OpenFile(rcConf, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
		if err != nil {
			return fmt.Errorf("failed to open rc.conf: %w", err)
		}
		defer f.Close()
		f.WriteString(enableLine + "\n")
	}

	fmt.Printf("%s Installed BSD rc.d service: %s\n", terminal.StatusIcon(true), rcPath)
	return nil
}

func (m *SystemServiceManager) bsdUninstall() error {
	_ = m.bsdStop()

	rcPath := m.path("/usr/local/etc/rc.d", m.name)
	if err := os.Remove(rcPath); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("failed to remove rc script: %w", err)
	}

	fmt.Printf("%s Uninstalled BSD rc.d service: %s\n", terminal.StatusIcon(true), m.name)
	return nil
}

func (m *SystemServiceManager) bsdDisable() error {
	rcConf := m.path("/etc/rc.conf.local")
	data, err := os.ReadFile(rcConf)
	if err != nil {
		return err
	}

	disableLine := fmt.Sprintf(`%s_enable="NO"`, m.name)
	enableLine := fmt.Sprintf(`%s_enable="YES"`, m.name)

	newData := strings.ReplaceAll(string(data), enableLine, disableLine)
	return os.WriteFile(rcConf, []byte(newData), 0644)
}

// runCmdOutput runs a command and returns its combined output
func runCmdOutput(name string, args ...string) ([]byte, error) {
	return exec.Command(name, args...).CombinedOutput()
}

// Helper to run commands
func runCmd(name string, args ...string) error {
	cmd := exec.Command(name, args...)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("%s %s: %w\n%s", name, strings.Join(args, " "), err, output)
	}
	return nil
}
