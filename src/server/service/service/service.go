// SPDX-License-Identifier: MIT
package service

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
)

// SystemServiceManager handles system service management
type SystemServiceManager struct {
	name        string
	displayName string
	description string
	execPath    string
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
	}, nil
}

// Start starts the service
func (m *SystemServiceManager) Start() error {
	switch runtime.GOOS {
	case "linux":
		return m.linuxStart()
	case "darwin":
		return m.darwinStart()
	case "windows":
		return m.windowsStart()
	case "freebsd", "openbsd", "netbsd":
		return m.bsdStart()
	default:
		return fmt.Errorf("unsupported OS: %s", runtime.GOOS)
	}
}

// Stop stops the service
func (m *SystemServiceManager) Stop() error {
	switch runtime.GOOS {
	case "linux":
		return m.linuxStop()
	case "darwin":
		return m.darwinStop()
	case "windows":
		return m.windowsStop()
	case "freebsd", "openbsd", "netbsd":
		return m.bsdStop()
	default:
		return fmt.Errorf("unsupported OS: %s", runtime.GOOS)
	}
}

// Restart restarts the service
func (m *SystemServiceManager) Restart() error {
	switch runtime.GOOS {
	case "linux":
		return m.linuxRestart()
	case "darwin":
		return m.darwinRestart()
	case "windows":
		return m.windowsRestart()
	case "freebsd", "openbsd", "netbsd":
		return m.bsdRestart()
	default:
		return fmt.Errorf("unsupported OS: %s", runtime.GOOS)
	}
}

// Reload sends SIGHUP to reload configuration
func (m *SystemServiceManager) Reload() error {
	switch runtime.GOOS {
	case "linux":
		return m.linuxReload()
	case "darwin":
		return m.darwinReload()
	case "windows":
		return fmt.Errorf("reload not supported on Windows, use restart")
	case "freebsd", "openbsd", "netbsd":
		return m.bsdReload()
	default:
		return fmt.Errorf("unsupported OS: %s", runtime.GOOS)
	}
}

// Install installs the service
func (m *SystemServiceManager) Install() error {
	switch runtime.GOOS {
	case "linux":
		return m.linuxInstall()
	case "darwin":
		return m.darwinInstall()
	case "windows":
		return m.windowsInstall()
	case "freebsd", "openbsd", "netbsd":
		return m.bsdInstall()
	default:
		return fmt.Errorf("unsupported OS: %s", runtime.GOOS)
	}
}

// Uninstall removes the service
func (m *SystemServiceManager) Uninstall() error {
	switch runtime.GOOS {
	case "linux":
		return m.linuxUninstall()
	case "darwin":
		return m.darwinUninstall()
	case "windows":
		return m.windowsUninstall()
	case "freebsd", "openbsd", "netbsd":
		return m.bsdUninstall()
	default:
		return fmt.Errorf("unsupported OS: %s", runtime.GOOS)
	}
}

// Disable disables the service from starting at boot
func (m *SystemServiceManager) Disable() error {
	switch runtime.GOOS {
	case "linux":
		return m.linuxDisable()
	case "darwin":
		return m.darwinDisable()
	case "windows":
		return m.windowsDisable()
	case "freebsd", "openbsd", "netbsd":
		return m.bsdDisable()
	default:
		return fmt.Errorf("unsupported OS: %s", runtime.GOOS)
	}
}

// GetServiceStatus returns the current service status per AI.md PART 25
func (m *SystemServiceManager) GetServiceStatus() (string, error) {
	switch runtime.GOOS {
	case "linux":
		return m.linuxStatus()
	case "darwin":
		return m.darwinStatus()
	case "windows":
		return m.windowsStatus()
	case "freebsd", "openbsd", "netbsd":
		return m.bsdStatus()
	default:
		return "unknown", fmt.Errorf("unsupported OS: %s", runtime.GOOS)
	}
}

// linuxStatus returns service status on Linux
func (m *SystemServiceManager) linuxStatus() (string, error) {
	if m.hasSystemd() {
		out, err := exec.Command("systemctl", "is-active", m.name).Output()
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
	if m.hasRunit() {
		out, err := exec.Command("sv", "status", m.name).CombinedOutput()
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
	out, err := exec.Command("launchctl", "list", m.launchdLabel()).CombinedOutput()
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
	out, err := exec.Command("sc", "query", m.name).CombinedOutput()
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
	out, err := exec.Command("service", m.name, "status").CombinedOutput()
	if err != nil {
		return "stopped", nil
	}
	if strings.Contains(strings.ToLower(string(out)), "running") {
		return "running", nil
	}
	return "stopped", nil
}

// Linux - systemd and runit support
func (m *SystemServiceManager) linuxStart() error {
	if m.hasSystemd() {
		return runCmd("systemctl", "start", m.name)
	}
	if m.hasRunit() {
		return runCmd("sv", "start", m.name)
	}
	return fmt.Errorf("no supported service manager found")
}

func (m *SystemServiceManager) linuxStop() error {
	if m.hasSystemd() {
		return runCmd("systemctl", "stop", m.name)
	}
	if m.hasRunit() {
		return runCmd("sv", "stop", m.name)
	}
	return fmt.Errorf("no supported service manager found")
}

func (m *SystemServiceManager) linuxRestart() error {
	if m.hasSystemd() {
		return runCmd("systemctl", "restart", m.name)
	}
	if m.hasRunit() {
		return runCmd("sv", "restart", m.name)
	}
	return fmt.Errorf("no supported service manager found")
}

func (m *SystemServiceManager) linuxReload() error {
	if m.hasSystemd() {
		return runCmd("systemctl", "reload", m.name)
	}
	if m.hasRunit() {
		return runCmd("sv", "hup", m.name)
	}
	return fmt.Errorf("no supported service manager found")
}

func (m *SystemServiceManager) linuxInstall() error {
	if m.hasSystemd() {
		return m.installSystemd()
	}
	if m.hasRunit() {
		return m.installRunit()
	}
	return fmt.Errorf("no supported service manager found (need systemd or runit)")
}

func (m *SystemServiceManager) linuxUninstall() error {
	if m.hasSystemd() {
		return m.uninstallSystemd()
	}
	if m.hasRunit() {
		return m.uninstallRunit()
	}
	return fmt.Errorf("no supported service manager found")
}

func (m *SystemServiceManager) linuxDisable() error {
	if m.hasSystemd() {
		return runCmd("systemctl", "disable", m.name)
	}
	if m.hasRunit() {
		runPath := filepath.Join("/etc/service", m.name)
		return os.Remove(runPath)
	}
	return fmt.Errorf("no supported service manager found")
}

func (m *SystemServiceManager) hasSystemd() bool {
	_, err := exec.LookPath("systemctl")
	return err == nil
}

func (m *SystemServiceManager) hasRunit() bool {
	_, err := exec.LookPath("sv")
	return err == nil
}

func (m *SystemServiceManager) installSystemd() error {
	unitPath := filepath.Join("/etc/systemd/system", m.name+".service")

	// Per AI.md PART 25: systemd unit file with security hardening
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

# Security hardening per AI.md PART 25
NoNewPrivileges=yes
ProtectSystem=strict
ProtectHome=yes
PrivateTmp=yes
ReadWritePaths=/var/lib/apimgr/%s
ReadWritePaths=/var/log/apimgr/%s

[Install]
WantedBy=multi-user.target
`, m.description, m.name, m.name, m.name, m.execPath, m.name, m.name)

	if err := os.WriteFile(unitPath, []byte(content), 0644); err != nil {
		return fmt.Errorf("failed to write unit file: %w", err)
	}

	if err := runCmd("systemctl", "daemon-reload"); err != nil {
		return fmt.Errorf("failed to reload systemd: %w", err)
	}

	if err := runCmd("systemctl", "enable", m.name); err != nil {
		return fmt.Errorf("failed to enable service: %w", err)
	}

	fmt.Printf("✅ Installed systemd service: %s\n", unitPath)
	return nil
}

func (m *SystemServiceManager) uninstallSystemd() error {
	_ = runCmd("systemctl", "stop", m.name)
	_ = runCmd("systemctl", "disable", m.name)

	unitPath := filepath.Join("/etc/systemd/system", m.name+".service")
	if err := os.Remove(unitPath); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("failed to remove unit file: %w", err)
	}

	_ = runCmd("systemctl", "daemon-reload")

	fmt.Printf("✅ Uninstalled systemd service: %s\n", m.name)
	return nil
}

func (m *SystemServiceManager) installRunit() error {
	svcDir := filepath.Join("/etc/sv", m.name)
	runPath := filepath.Join("/etc/service", m.name)

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

	fmt.Printf("✅ Installed runit service: %s\n", svcDir)
	return nil
}

func (m *SystemServiceManager) uninstallRunit() error {
	runPath := filepath.Join("/etc/service", m.name)
	svcDir := filepath.Join("/etc/sv", m.name)

	// Stop first
	_ = runCmd("sv", "stop", m.name)

	// Remove symlink
	if err := os.Remove(runPath); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("failed to remove service symlink: %w", err)
	}

	// Remove service directory
	if err := os.RemoveAll(svcDir); err != nil {
		return fmt.Errorf("failed to remove service directory: %w", err)
	}

	fmt.Printf("✅ Uninstalled runit service: %s\n", m.name)
	return nil
}

// macOS - launchd support
func (m *SystemServiceManager) darwinStart() error {
	return runCmd("launchctl", "start", m.launchdLabel())
}

func (m *SystemServiceManager) darwinStop() error {
	return runCmd("launchctl", "stop", m.launchdLabel())
}

func (m *SystemServiceManager) darwinRestart() error {
	_ = m.darwinStop()
	return m.darwinStart()
}

func (m *SystemServiceManager) darwinReload() error {
	return runCmd("launchctl", "kickstart", "-k", "gui/"+m.launchdLabel())
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
    <string>/var/log/%s.log</string>
    <key>StandardErrorPath</key>
    <string>/var/log/%s.err</string>
</dict>
</plist>
`, m.launchdLabel(), m.execPath, m.name, m.name)

	if err := os.WriteFile(plistPath, []byte(content), 0644); err != nil {
		return fmt.Errorf("failed to write plist: %w", err)
	}

	if err := runCmd("launchctl", "load", plistPath); err != nil {
		return fmt.Errorf("failed to load service: %w", err)
	}

	fmt.Printf("✅ Installed launchd service: %s\n", plistPath)
	return nil
}

func (m *SystemServiceManager) darwinUninstall() error {
	plistPath := m.launchdPlistPath()

	_ = runCmd("launchctl", "unload", plistPath)

	if err := os.Remove(plistPath); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("failed to remove plist: %w", err)
	}

	fmt.Printf("✅ Uninstalled launchd service: %s\n", m.name)
	return nil
}

func (m *SystemServiceManager) darwinDisable() error {
	return runCmd("launchctl", "unload", "-w", m.launchdPlistPath())
}

func (m *SystemServiceManager) launchdLabel() string {
	return "com.apimgr." + m.name
}

func (m *SystemServiceManager) launchdPlistPath() string {
	return filepath.Join("/Library/LaunchDaemons", m.launchdLabel()+".plist")
}

// Windows - Windows Service Manager
func (m *SystemServiceManager) windowsStart() error {
	return runCmd("sc", "start", m.name)
}

func (m *SystemServiceManager) windowsStop() error {
	return runCmd("sc", "stop", m.name)
}

func (m *SystemServiceManager) windowsRestart() error {
	_ = m.windowsStop()
	return m.windowsStart()
}

func (m *SystemServiceManager) windowsInstall() error {
	// Create service using sc command
	err := runCmd("sc", "create", m.name,
		"binPath=", m.execPath,
		"DisplayName=", m.displayName,
		"start=", "auto",
	)
	if err != nil {
		return fmt.Errorf("failed to create service: %w", err)
	}

	// Set description
	_ = runCmd("sc", "description", m.name, m.description)

	fmt.Printf("✅ Installed Windows service: %s\n", m.name)
	return nil
}

func (m *SystemServiceManager) windowsUninstall() error {
	_ = m.windowsStop()

	if err := runCmd("sc", "delete", m.name); err != nil {
		return fmt.Errorf("failed to delete service: %w", err)
	}

	fmt.Printf("✅ Uninstalled Windows service: %s\n", m.name)
	return nil
}

func (m *SystemServiceManager) windowsDisable() error {
	return runCmd("sc", "config", m.name, "start=", "disabled")
}

// BSD - rc.d support
func (m *SystemServiceManager) bsdStart() error {
	return runCmd("service", m.name, "start")
}

func (m *SystemServiceManager) bsdStop() error {
	return runCmd("service", m.name, "stop")
}

func (m *SystemServiceManager) bsdRestart() error {
	return runCmd("service", m.name, "restart")
}

func (m *SystemServiceManager) bsdReload() error {
	return runCmd("service", m.name, "reload")
}

func (m *SystemServiceManager) bsdInstall() error {
	rcPath := filepath.Join("/usr/local/etc/rc.d", m.name)

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
	rcConf := "/etc/rc.conf.local"
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

	fmt.Printf("✅ Installed BSD rc.d service: %s\n", rcPath)
	return nil
}

func (m *SystemServiceManager) bsdUninstall() error {
	_ = m.bsdStop()

	rcPath := filepath.Join("/usr/local/etc/rc.d", m.name)
	if err := os.Remove(rcPath); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("failed to remove rc script: %w", err)
	}

	fmt.Printf("✅ Uninstalled BSD rc.d service: %s\n", m.name)
	return nil
}

func (m *SystemServiceManager) bsdDisable() error {
	rcConf := "/etc/rc.conf.local"
	data, err := os.ReadFile(rcConf)
	if err != nil {
		return err
	}

	disableLine := fmt.Sprintf(`%s_enable="NO"`, m.name)
	enableLine := fmt.Sprintf(`%s_enable="YES"`, m.name)

	newData := strings.ReplaceAll(string(data), enableLine, disableLine)
	return os.WriteFile(rcConf, []byte(newData), 0644)
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
