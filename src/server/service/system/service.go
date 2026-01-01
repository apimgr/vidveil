// SPDX-License-Identifier: MIT
// AI.md PART 4 & 5: Service Management and User Creation
package system

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
)

// ServiceManager handles system service installation per AI.md PART 5
type ServiceManager struct {
	appName     string
	binaryPath  string
	configDir   string
	dataDir     string
	user        string
	group       string
	description string
}

// NewServiceManager creates a new service manager
func NewServiceManager(appName, binaryPath, configDir, dataDir string) *ServiceManager {
	return &ServiceManager{
		appName:     appName,
		binaryPath:  binaryPath,
		configDir:   configDir,
		dataDir:     dataDir,
		user:        appName,
		group:       appName,
		description: fmt.Sprintf("%s service", appName),
	}
}

// Install installs the service per AI.md PART 5
func (sm *ServiceManager) Install() error {
	switch runtime.GOOS {
	case "linux":
		return sm.installLinux()
	case "darwin":
		return sm.installDarwin()
	case "freebsd", "openbsd", "netbsd":
		return sm.installBSD()
	case "windows":
		return sm.installWindows()
	default:
		return fmt.Errorf("unsupported operating system: %s", runtime.GOOS)
	}
}

// Uninstall removes the service
func (sm *ServiceManager) Uninstall() error {
	switch runtime.GOOS {
	case "linux":
		return sm.uninstallLinux()
	case "darwin":
		return sm.uninstallDarwin()
	case "freebsd", "openbsd", "netbsd":
		return sm.uninstallBSD()
	case "windows":
		return sm.uninstallWindows()
	default:
		return fmt.Errorf("unsupported operating system: %s", runtime.GOOS)
	}
}

// Start starts the service
func (sm *ServiceManager) Start() error {
	return sm.runServiceCommand("start")
}

// Stop stops the service
func (sm *ServiceManager) Stop() error {
	return sm.runServiceCommand("stop")
}

// Restart restarts the service
func (sm *ServiceManager) Restart() error {
	return sm.runServiceCommand("restart")
}

// Reload reloads the service configuration
func (sm *ServiceManager) Reload() error {
	return sm.runServiceCommand("reload")
}

// Status returns the service status
func (sm *ServiceManager) Status() (string, error) {
	switch runtime.GOOS {
	case "linux":
		if sm.hasSystemd() {
			out, err := exec.Command("systemctl", "status", sm.appName).CombinedOutput()
			if err != nil {
				if strings.Contains(string(out), "inactive") {
					return "stopped", nil
				}
				return "unknown", err
			}
			if strings.Contains(string(out), "active (running)") {
				return "running", nil
			}
			return "stopped", nil
		}
	case "darwin":
		out, err := exec.Command("launchctl", "list", sm.appName).CombinedOutput()
		if err != nil {
			return "stopped", nil
		}
		if len(out) > 0 {
			return "running", nil
		}
	}
	return "unknown", nil
}

// runServiceCommand runs a service management command
func (sm *ServiceManager) runServiceCommand(action string) error {
	switch runtime.GOOS {
	case "linux":
		if sm.hasSystemd() {
			return exec.Command("systemctl", action, sm.appName).Run()
		}
		if sm.hasRunit() {
			return exec.Command("sv", action, sm.appName).Run()
		}
		return exec.Command("service", sm.appName, action).Run()
	case "darwin":
		plistPath := fmt.Sprintf("/Library/LaunchDaemons/%s.plist", sm.appName)
		switch action {
		case "start":
			return exec.Command("launchctl", "load", plistPath).Run()
		case "stop":
			return exec.Command("launchctl", "unload", plistPath).Run()
		case "restart":
			exec.Command("launchctl", "unload", plistPath).Run()
			return exec.Command("launchctl", "load", plistPath).Run()
		}
	case "freebsd", "openbsd", "netbsd":
		return exec.Command("service", sm.appName, action).Run()
	case "windows":
		switch action {
		case "start":
			return exec.Command("sc", "start", sm.appName).Run()
		case "stop":
			return exec.Command("sc", "stop", sm.appName).Run()
		case "restart":
			exec.Command("sc", "stop", sm.appName).Run()
			return exec.Command("sc", "start", sm.appName).Run()
		}
	}
	return fmt.Errorf("unsupported action: %s", action)
}

// hasSystemd checks if systemd is available
func (sm *ServiceManager) hasSystemd() bool {
	_, err := os.Stat("/run/systemd/system")
	return err == nil
}

// hasRunit checks if runit is available
func (sm *ServiceManager) hasRunit() bool {
	_, err := exec.LookPath("sv")
	return err == nil
}

// installLinux installs service on Linux per AI.md PART 5
func (sm *ServiceManager) installLinux() error {
	// Create system user per AI.md PART 4
	if err := sm.createLinuxUser(); err != nil {
		return fmt.Errorf("failed to create user: %w", err)
	}

	// Install based on init system
	if sm.hasSystemd() {
		return sm.installSystemd()
	}
	if sm.hasRunit() {
		return sm.installRunit()
	}
	return fmt.Errorf("no supported init system found (systemd or runit)")
}

// createLinuxUser creates system user per AI.md PART 4
func (sm *ServiceManager) createLinuxUser() error {
	// Check if user exists
	_, err := exec.Command("id", sm.user).CombinedOutput()
	// User already exists
	if err == nil {
		return nil
	}

	// Find available UID in 100-999 range per AI.md PART 4
	uid := sm.findAvailableUID(100, 999)

	// Create group
	exec.Command("groupadd", "-g", strconv.Itoa(uid), sm.group).Run()

	// Create system user with:
	// -r: System account
	// -u: UID
	// -g: Primary group
	// -d: Home directory
	// -s: No login shell
	// -c: Comment/description
	cmd := exec.Command("useradd",
		"-r",
		"-u", strconv.Itoa(uid),
		"-g", sm.group,
		"-d", sm.dataDir,
		"-s", "/sbin/nologin",
		"-c", sm.description,
		sm.user,
	)
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to create user: %w", err)
	}

	// Create and set ownership of directories
	for _, dir := range []string{sm.configDir, sm.dataDir} {
		os.MkdirAll(dir, 0755)
		exec.Command("chown", "-R", fmt.Sprintf("%s:%s", sm.user, sm.group), dir).Run()
	}

	return nil
}

// findAvailableUID finds an available UID in the given range
func (sm *ServiceManager) findAvailableUID(min, max int) int {
	for uid := min; uid <= max; uid++ {
		_, err := exec.Command("getent", "passwd", strconv.Itoa(uid)).CombinedOutput()
		if err != nil {
			return uid
		}
	}
	return min
}

// installSystemd installs systemd service unit per AI.md PART 5
func (sm *ServiceManager) installSystemd() error {
	unitPath := fmt.Sprintf("/etc/systemd/system/%s.service", sm.appName)

	unit := fmt.Sprintf(`[Unit]
Description=%s
Documentation=https://github.com/apimgr/%s
After=network-online.target
Wants=network-online.target

[Service]
Type=simple
User=%s
Group=%s
ExecStart=%s --config %s --data %s
ExecReload=/bin/kill -HUP $MAINPID
Restart=on-failure
RestartSec=5
LimitNOFILE=65535

# Security hardening
NoNewPrivileges=yes
ProtectSystem=strict
ProtectHome=yes
PrivateTmp=yes
ProtectKernelTunables=yes
ProtectKernelModules=yes
ProtectControlGroups=yes
ReadWritePaths=%s %s

[Install]
WantedBy=multi-user.target
`, sm.description, sm.appName, sm.user, sm.group, sm.binaryPath, sm.configDir, sm.dataDir, sm.configDir, sm.dataDir)

	if err := os.WriteFile(unitPath, []byte(unit), 0644); err != nil {
		return fmt.Errorf("failed to write systemd unit: %w", err)
	}

	// Reload systemd and enable service
	exec.Command("systemctl", "daemon-reload").Run()
	exec.Command("systemctl", "enable", sm.appName).Run()

	fmt.Printf("Systemd service installed: %s\n", unitPath)
	fmt.Printf("Start with: systemctl start %s\n", sm.appName)
	return nil
}

// installRunit installs runit service per AI.md PART 5
func (sm *ServiceManager) installRunit() error {
	serviceDir := fmt.Sprintf("/etc/sv/%s", sm.appName)
	os.MkdirAll(serviceDir, 0755)

	// Create run script
	runScript := fmt.Sprintf(`#!/bin/sh
exec chpst -u %s:%s %s --config %s --data %s 2>&1
`, sm.user, sm.group, sm.binaryPath, sm.configDir, sm.dataDir)

	runPath := filepath.Join(serviceDir, "run")
	if err := os.WriteFile(runPath, []byte(runScript), 0755); err != nil {
		return fmt.Errorf("failed to write run script: %w", err)
	}

	// Create log directory and script
	logDir := filepath.Join(serviceDir, "log")
	os.MkdirAll(logDir, 0755)

	logScript := fmt.Sprintf(`#!/bin/sh
exec svlogd -tt /var/log/%s
`, sm.appName)
	if err := os.WriteFile(filepath.Join(logDir, "run"), []byte(logScript), 0755); err != nil {
		return err
	}

	// Enable service
	exec.Command("ln", "-sf", serviceDir, fmt.Sprintf("/var/service/%s", sm.appName)).Run()

	fmt.Printf("Runit service installed: %s\n", serviceDir)
	fmt.Printf("Start with: sv start %s\n", sm.appName)
	return nil
}

// installDarwin installs launchd service on macOS per AI.md PART 5
func (sm *ServiceManager) installDarwin() error {
	// Create user per AI.md PART 4 (macOS uses dscl)
	if err := sm.createDarwinUser(); err != nil {
		return fmt.Errorf("failed to create user: %w", err)
	}

	plistPath := fmt.Sprintf("/Library/LaunchDaemons/%s.plist", sm.appName)

	plist := fmt.Sprintf(`<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
    <key>Label</key>
    <string>%s</string>
    <key>ProgramArguments</key>
    <array>
        <string>%s</string>
        <string>--config</string>
        <string>%s</string>
        <string>--data</string>
        <string>%s</string>
    </array>
    <key>RunAtLoad</key>
    <true/>
    <key>KeepAlive</key>
    <true/>
    <key>UserName</key>
    <string>%s</string>
    <key>GroupName</key>
    <string>%s</string>
    <key>StandardOutPath</key>
    <string>/var/log/%s.log</string>
    <key>StandardErrorPath</key>
    <string>/var/log/%s.error.log</string>
</dict>
</plist>
`, sm.appName, sm.binaryPath, sm.configDir, sm.dataDir, sm.user, sm.group, sm.appName, sm.appName)

	if err := os.WriteFile(plistPath, []byte(plist), 0644); err != nil {
		return fmt.Errorf("failed to write plist: %w", err)
	}

	fmt.Printf("LaunchDaemon installed: %s\n", plistPath)
	fmt.Printf("Start with: launchctl load %s\n", plistPath)
	return nil
}

// createDarwinUser creates system user on macOS using dscl per AI.md PART 4
func (sm *ServiceManager) createDarwinUser() error {
	// Check if user exists
	_, err := exec.Command("dscl", ".", "-read", fmt.Sprintf("/Users/%s", sm.user)).CombinedOutput()
	// User already exists
	if err == nil {
		return nil
	}

	// Find available UID
	uid := sm.findAvailableUID(100, 999)

	// Create user using dscl per AI.md PART 4
	commands := [][]string{
		{"dscl", ".", "-create", fmt.Sprintf("/Users/%s", sm.user)},
		{"dscl", ".", "-create", fmt.Sprintf("/Users/%s", sm.user), "UniqueID", strconv.Itoa(uid)},
		{"dscl", ".", "-create", fmt.Sprintf("/Users/%s", sm.user), "PrimaryGroupID", strconv.Itoa(uid)},
		{"dscl", ".", "-create", fmt.Sprintf("/Users/%s", sm.user), "NFSHomeDirectory", sm.dataDir},
		{"dscl", ".", "-create", fmt.Sprintf("/Users/%s", sm.user), "UserShell", "/usr/bin/false"},
		{"dscl", ".", "-create", fmt.Sprintf("/Users/%s", sm.user), "RealName", sm.description},
	}

	for _, cmd := range commands {
		if err := exec.Command(cmd[0], cmd[1:]...).Run(); err != nil {
			return fmt.Errorf("failed to execute %v: %w", cmd, err)
		}
	}

	return nil
}

// installBSD installs rc.d service on BSD per AI.md PART 5
func (sm *ServiceManager) installBSD() error {
	// Create user using pw per AI.md PART 4
	if err := sm.createBSDUser(); err != nil {
		return fmt.Errorf("failed to create user: %w", err)
	}

	rcPath := fmt.Sprintf("/usr/local/etc/rc.d/%s", sm.appName)

	rcScript := fmt.Sprintf(`#!/bin/sh

# PROVIDE: %s
# REQUIRE: NETWORKING
# KEYWORD: shutdown

. /etc/rc.subr

name="%s"
rcvar="${name}_enable"

load_rc_config $name

: ${%s_enable:="NO"}
: ${%s_user:="%s"}

command="%s"
command_args="--config %s --data %s"

run_rc_command "$1"
`, sm.appName, sm.appName, sm.appName, sm.appName, sm.user, sm.binaryPath, sm.configDir, sm.dataDir)

	if err := os.WriteFile(rcPath, []byte(rcScript), 0755); err != nil {
		return fmt.Errorf("failed to write rc script: %w", err)
	}

	fmt.Printf("RC script installed: %s\n", rcPath)
	fmt.Printf("Enable with: sysrc %s_enable=YES\n", sm.appName)
	fmt.Printf("Start with: service %s start\n", sm.appName)
	return nil
}

// createBSDUser creates system user on BSD using pw per AI.md PART 4
func (sm *ServiceManager) createBSDUser() error {
	// Check if user exists
	_, err := exec.Command("pw", "usershow", sm.user).CombinedOutput()
	if err == nil {
		return nil
	}

	// Find available UID
	uid := sm.findAvailableUID(100, 999)

	// Create group
	exec.Command("pw", "groupadd", sm.group, "-g", strconv.Itoa(uid)).Run()

	// Create user using pw per AI.md PART 4
	return exec.Command("pw", "useradd", sm.user,
		"-u", strconv.Itoa(uid),
		"-g", sm.group,
		"-d", sm.dataDir,
		"-s", "/usr/sbin/nologin",
		"-c", sm.description,
	).Run()
}

// installWindows installs Windows service per AI.md PART 5
func (sm *ServiceManager) installWindows() error {
	// Create Windows Virtual Service Account per AI.md PART 4
	account := fmt.Sprintf("NT SERVICE\\%s", sm.appName)

	// Install service using sc.exe
	cmd := exec.Command("sc", "create", sm.appName,
		"binPath=", fmt.Sprintf("\"%s\" --config \"%s\" --data \"%s\"", sm.binaryPath, sm.configDir, sm.dataDir),
		"start=", "auto",
		"DisplayName=", sm.description,
		"obj=", account,
	)

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to create Windows service: %w", err)
	}

	// Set description
	exec.Command("sc", "description", sm.appName, sm.description).Run()

	// Set failure recovery
	exec.Command("sc", "failure", sm.appName, "reset=", "86400", "actions=", "restart/5000/restart/10000/restart/30000").Run()

	fmt.Printf("Windows service installed: %s\n", sm.appName)
	fmt.Printf("Start with: sc start %s\n", sm.appName)
	return nil
}

// uninstallLinux removes Linux service
func (sm *ServiceManager) uninstallLinux() error {
	sm.Stop()

	if sm.hasSystemd() {
		exec.Command("systemctl", "disable", sm.appName).Run()
		os.Remove(fmt.Sprintf("/etc/systemd/system/%s.service", sm.appName))
		exec.Command("systemctl", "daemon-reload").Run()
	}
	if sm.hasRunit() {
		os.Remove(fmt.Sprintf("/var/service/%s", sm.appName))
		os.RemoveAll(fmt.Sprintf("/etc/sv/%s", sm.appName))
	}

	fmt.Printf("Service %s uninstalled\n", sm.appName)
	return nil
}

// uninstallDarwin removes macOS service
func (sm *ServiceManager) uninstallDarwin() error {
	plistPath := fmt.Sprintf("/Library/LaunchDaemons/%s.plist", sm.appName)
	exec.Command("launchctl", "unload", plistPath).Run()
	os.Remove(plistPath)
	fmt.Printf("Service %s uninstalled\n", sm.appName)
	return nil
}

// uninstallBSD removes BSD service
func (sm *ServiceManager) uninstallBSD() error {
	sm.Stop()
	os.Remove(fmt.Sprintf("/usr/local/etc/rc.d/%s", sm.appName))
	fmt.Printf("Service %s uninstalled\n", sm.appName)
	return nil
}

// uninstallWindows removes Windows service
func (sm *ServiceManager) uninstallWindows() error {
	sm.Stop()
	exec.Command("sc", "delete", sm.appName).Run()
	fmt.Printf("Service %s uninstalled\n", sm.appName)
	return nil
}

// DetectEscalation detects available privilege escalation methods per AI.md PART 4
func DetectEscalation() string {
	switch runtime.GOOS {
	case "linux", "darwin", "freebsd", "openbsd", "netbsd":
		// Check in priority order per AI.md
		for _, cmd := range []string{"sudo", "doas", "pkexec"} {
			if _, err := exec.LookPath(cmd); err == nil {
				return cmd
			}
		}
	case "windows":
		// Windows uses UAC elevation
		return "runas"
	}
	return ""
}

// IsRoot checks if running as root/administrator
func IsRoot() bool {
	switch runtime.GOOS {
	case "windows":
		// Check if running with admin privileges
		_, err := os.Open("\\\\.\\PHYSICALDRIVE0")
		return err == nil
	default:
		return os.Getuid() == 0
	}
}
