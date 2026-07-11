// SPDX-License-Identifier: MIT
// AI.md PART 23 & 24: Service Management and User Creation
package system

import (
	"bufio"
	"fmt"
	"os"
	"os/exec"
	"os/user"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
)

// ServiceManager handles system service installation per AI.md PART 23
type ServiceManager struct {
	appName      string
	internalName string
	projectOrg   string
	plistName    string
	binaryPath   string
	configDir    string
	dataDir      string
	user         string
	group        string
	description  string
}

// NewServiceManager creates a new service manager
// Per AI.md PART 24: Uses {project_org} and {internal_name} for paths
func NewServiceManager(appName, binaryPath, configDir, dataDir string) *ServiceManager {
	// Default to apimgr org and appName as internal name
	return NewServiceManagerWithOrg(appName, "apimgr", appName, binaryPath, configDir, dataDir)
}

// NewServiceManagerWithOrg creates a service manager with explicit org/internal names
// Per AI.md PART 24: plist_name = io.github.{project_org}.{internal_name}
func NewServiceManagerWithOrg(appName, projectOrg, internalName, binaryPath, configDir, dataDir string) *ServiceManager {
	return &ServiceManager{
		appName:      appName,
		internalName: internalName,
		projectOrg:   projectOrg,
		plistName:    fmt.Sprintf("io.github.%s.%s", projectOrg, internalName),
		binaryPath:   binaryPath,
		configDir:    configDir,
		dataDir:      dataDir,
		user:         internalName,
		group:        internalName,
		description:  fmt.Sprintf("%s service", appName),
	}
}

// Install installs the service per AI.md PART 23
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

// Disable disables the service from starting at boot per AI.md PART 23
func (sm *ServiceManager) Disable() error {
	switch runtime.GOOS {
	case "linux":
		if sm.hasSystemd() {
			return exec.Command("systemctl", "disable", sm.appName).Run()
		}
		if sm.hasRunit() {
			// Runit: remove symlink from /var/service
			return os.Remove(fmt.Sprintf("/var/service/%s", sm.appName))
		}
		return fmt.Errorf("no supported init system found")
	case "darwin":
		// macOS: unload the plist (stops and prevents autostart)
		// Per AI.md PART 24: plist path uses {plist_name}
		plistPath := fmt.Sprintf("/Library/LaunchDaemons/%s.plist", sm.plistName)
		return exec.Command("launchctl", "unload", "-w", plistPath).Run()
	case "freebsd", "openbsd", "netbsd":
		// BSD: set enable=NO in rc.conf
		return exec.Command("sysrc", fmt.Sprintf("%s_enable=NO", sm.appName)).Run()
	case "windows":
		// Windows: set service start type to disabled
		return exec.Command("sc", "config", sm.appName, "start=", "disabled").Run()
	default:
		return fmt.Errorf("unsupported operating system: %s", runtime.GOOS)
	}
}

// GetServiceStatus returns the service status
func (sm *ServiceManager) GetServiceStatus() (string, error) {
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
		// Per AI.md PART 24: list using {plist_name}
		out, err := exec.Command("launchctl", "list", sm.plistName).CombinedOutput()
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
		// Per AI.md PART 24: plist path uses {plist_name}
		plistPath := fmt.Sprintf("/Library/LaunchDaemons/%s.plist", sm.plistName)
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

// hasOpenRC reports whether OpenRC is the active init system per AI.md PART 24.
// OpenRC ships with /sbin/openrc-run and the rc-service / rc-update tools on
// Alpine, Gentoo, and Devuan. The presence of openrc-run is the canonical
// detection signal — /etc/init.d alone matches both OpenRC and SysVinit.
func (sm *ServiceManager) hasOpenRC() bool {
	if _, err := exec.LookPath("openrc-run"); err == nil {
		return true
	}
	if _, err := os.Stat("/sbin/openrc-run"); err == nil {
		return true
	}
	return false
}

// hasSysVInit reports whether the host is using SysVinit-style init.d scripts
// per AI.md PART 24. Per spec: SysVinit is selected only when systemd is
// absent, OpenRC is absent, and /etc/init.d exists with a working
// update-rc.d or chkconfig.
func (sm *ServiceManager) hasSysVInit() bool {
	if sm.hasSystemd() || sm.hasOpenRC() {
		return false
	}
	if _, err := os.Stat("/etc/init.d"); err != nil {
		return false
	}
	if _, err := exec.LookPath("update-rc.d"); err == nil {
		return true
	}
	if _, err := exec.LookPath("chkconfig"); err == nil {
		return true
	}
	return false
}

// installLinux installs service on Linux per AI.md PART 24.
// Detection order matches the spec: systemd → OpenRC → SysVinit → runit.
func (sm *ServiceManager) installLinux() error {

	// Create system user per AI.md PART 23
	if err := sm.createLinuxUser(); err != nil {
		return fmt.Errorf("failed to create user: %w", err)
	}

	// Install based on init system
	if sm.hasSystemd() {
		return sm.installSystemd()
	}
	if sm.hasOpenRC() {
		return sm.installOpenRC()
	}
	if sm.hasSysVInit() {
		return sm.installSysVInit()
	}
	if sm.hasRunit() {
		return sm.installRunit()
	}
	return fmt.Errorf("no supported init system found (systemd, OpenRC, SysVinit, or runit)")
}

// createLinuxUser creates system user per AI.md PART 23
func (sm *ServiceManager) createLinuxUser() error {

	// Check if user exists
	_, err := exec.Command("id", sm.user).CombinedOutput()
	// User already exists
	if err == nil {
		return nil
	}

	// Find available UID in 200-899 range per AI.md PART 23
	uid := sm.findAvailableUID(200, 899)

	// groupadd --system --gid {id} {name} per AI.md PART 23
	exec.Command("groupadd", "--system", "--gid", strconv.Itoa(uid), sm.group).Run()

	// useradd: --system, numeric --gid, --uid, --home-dir, --shell, --comment
	// per AI.md PART 23 (--gid must be numeric, not a group name)
	cmd := exec.Command("useradd",
		"--system",
		"--uid", strconv.Itoa(uid),
		"--gid", strconv.Itoa(uid),
		"--home-dir", sm.dataDir,
		"--shell", "/sbin/nologin",
		"--comment", sm.description,
		sm.user,
	)
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to create user: %w", err)
	}

	// Create and set ownership of directories per PART 23
	// All directories must exist for systemd's ReadWritePaths to work
	// Per PART 24: paths use {project_org}/{internal_name}
	dirs := []string{
		fmt.Sprintf("/etc/%s/%s", sm.projectOrg, sm.internalName),
		fmt.Sprintf("/var/lib/%s/%s", sm.projectOrg, sm.internalName),
		fmt.Sprintf("/var/cache/%s/%s", sm.projectOrg, sm.internalName),
		fmt.Sprintf("/var/log/%s/%s", sm.projectOrg, sm.internalName),
	}
	for _, dir := range dirs {
		os.MkdirAll(dir, 0755)
		exec.Command("chown", "-R", fmt.Sprintf("%s:%s", sm.user, sm.group), dir).Run()
	}

	return nil
}

// findAvailableUID returns an available system UID/GID per AI.md PART 23.
// Delegates to the package-level findAvailableSystemID.
func (sm *ServiceManager) findAvailableUID(min, max int) int {
	id, err := findAvailableSystemID()
	if err != nil {
		return min
	}
	return id
}

// installSystemd installs systemd service unit per AI.md PART 24
// Service starts as root, binary drops privileges after port binding
func (sm *ServiceManager) installSystemd() error {
	// Per PART 24: unit file uses {internal_name}.service
	unitPath := fmt.Sprintf("/etc/systemd/system/%s.service", sm.internalName)

	// Per PART 23: NO User/Group - binary drops privileges after port binding
	// Per PART 24: paths use {project_org}/{internal_name}
	unit := fmt.Sprintf(`[Unit]
Description=%s service
Documentation=https://%s.github.io/%s
After=network-online.target
Wants=network-online.target

[Service]
Type=simple
ExecStart=/usr/local/bin/%s
Restart=on-failure
RestartSec=5
StandardOutput=journal
StandardError=journal

# Security hardening (binary drops privileges after port binding)
ProtectSystem=strict
ProtectHome=yes
PrivateTmp=yes
ReadWritePaths=/etc/%s/%s
ReadWritePaths=/var/lib/%s/%s
ReadWritePaths=/var/cache/%s/%s
ReadWritePaths=/var/log/%s/%s

[Install]
WantedBy=multi-user.target
`, sm.appName, sm.projectOrg, sm.appName, sm.appName,
		sm.projectOrg, sm.internalName,
		sm.projectOrg, sm.internalName,
		sm.projectOrg, sm.internalName,
		sm.projectOrg, sm.internalName)

	if err := os.WriteFile(unitPath, []byte(unit), 0644); err != nil {
		return fmt.Errorf("failed to write systemd unit: %w", err)
	}

	// Reload systemd and enable service
	exec.Command("systemctl", "daemon-reload").Run()
	exec.Command("systemctl", "enable", sm.internalName).Run()

	fmt.Printf("Systemd service installed: %s\n", unitPath)
	fmt.Printf("Start with: systemctl start %s\n", sm.internalName)
	return nil
}

// installRunit installs runit service per AI.md PART 24
// Binary drops privileges after port binding - no chpst needed
func (sm *ServiceManager) installRunit() error {
	// Per PART 24: service name uses {internal_name}
	serviceDir := fmt.Sprintf("/etc/sv/%s", sm.internalName)
	os.MkdirAll(serviceDir, 0755)

	// Per PART 23: binary handles privilege dropping; PART 24: simple run script
	runScript := fmt.Sprintf(`#!/bin/sh
exec /usr/local/bin/%s 2>&1
`, sm.appName)

	runPath := filepath.Join(serviceDir, "run")
	if err := os.WriteFile(runPath, []byte(runScript), 0755); err != nil {
		return fmt.Errorf("failed to write run script: %w", err)
	}

	// Create log directory and script per PART 24
	// Per PART 24: log path uses {project_org}/{internal_name}
	logDir := filepath.Join(serviceDir, "log")
	os.MkdirAll(logDir, 0755)

	logScript := fmt.Sprintf(`#!/bin/sh
exec svlogd -tt /var/log/%s/%s
`, sm.projectOrg, sm.internalName)
	if err := os.WriteFile(filepath.Join(logDir, "run"), []byte(logScript), 0755); err != nil {
		return err
	}

	// Enable service
	exec.Command("ln", "-sf", serviceDir, fmt.Sprintf("/var/service/%s", sm.internalName)).Run()

	fmt.Printf("Runit service installed: %s\n", serviceDir)
	fmt.Printf("Start with: sv start %s\n", sm.internalName)
	return nil
}

// installOpenRC installs an OpenRC service script per AI.md PART 24.
// The script lives at /etc/init.d/{internal_name} (executable). The binary
// drops privileges itself after binding privileged ports, so command_user
// is set to the dedicated service user.
func (sm *ServiceManager) installOpenRC() error {
	// Per PART 24: service name uses {internal_name}
	scriptPath := fmt.Sprintf("/etc/init.d/%s", sm.internalName)

	// Per PART 24: paths use {project_org}/{internal_name}
	script := fmt.Sprintf(`#!/sbin/openrc-run
# OpenRC service for %s per AI.md PART 24.
# Service identity uses the internal name so config/data/log paths stay
# stable across binary renames.

name="%s"
description="%s"
command="/usr/local/bin/%s"
command_args=""
command_user="%s:%s"
pidfile="/var/run/%s/%s.pid"
command_background=true
output_log="/var/log/%s/%s/server.log"
error_log="/var/log/%s/%s/error.log"

depend() {
    need net
    after firewall
    use dns logger
}

start_pre() {
    checkpath -d -m 0755 -o %s:%s /var/run/%s
    checkpath -d -m 0755 -o %s:%s /var/log/%s/%s
}
`,
		sm.appName,
		sm.internalName,
		sm.description,
		sm.appName,
		sm.user, sm.group,
		sm.projectOrg, sm.internalName,
		sm.projectOrg, sm.internalName,
		sm.projectOrg, sm.internalName,
		sm.user, sm.group, sm.projectOrg,
		sm.user, sm.group, sm.projectOrg, sm.internalName,
	)

	if err := os.WriteFile(scriptPath, []byte(script), 0755); err != nil {
		return fmt.Errorf("failed to write OpenRC script: %w", err)
	}

	// Enable at default runlevel (idempotent — rc-update is fine on re-run).
	exec.Command("rc-update", "add", sm.internalName, "default").Run()

	fmt.Printf("OpenRC service installed: %s\n", scriptPath)
	fmt.Printf("Start with: rc-service %s start\n", sm.internalName)
	return nil
}

// installSysVInit installs a SysVinit-style init script per AI.md PART 24.
// Same path as OpenRC (/etc/init.d/{internal_name}); detection picks one or the
// other. Uses start-stop-daemon (Debian-style) so it works on legacy
// Debian/Ubuntu and any distro that ships start-stop-daemon as part of
// dpkg or sysvinit-utils.
func (sm *ServiceManager) installSysVInit() error {
	// Per PART 24: service name uses {internal_name}
	scriptPath := fmt.Sprintf("/etc/init.d/%s", sm.internalName)

	// Per PART 24: paths use {project_org}/{internal_name}
	script := fmt.Sprintf(`#!/bin/sh
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
DAEMON=/usr/local/bin/%s
DAEMON_USER=%s
PIDFILE=/var/run/%s/%s.pid
LOGFILE=/var/log/%s/%s/server.log

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
        echo "Usage: $0 {start|stop|restart|status}"
        exit 1
        ;;
esac
exit 0
`,
		sm.internalName,
		sm.description,
		sm.description,
		sm.internalName,
		sm.appName,
		sm.user,
		sm.projectOrg, sm.internalName,
		sm.projectOrg, sm.internalName,
	)

	if err := os.WriteFile(scriptPath, []byte(script), 0755); err != nil {
		return fmt.Errorf("failed to write SysVinit script: %w", err)
	}

	// Register with the host's runlevel manager. Try Debian's update-rc.d
	// first, fall back to RHEL's chkconfig if present.
	if _, err := exec.LookPath("update-rc.d"); err == nil {
		exec.Command("update-rc.d", sm.internalName, "defaults").Run()
	} else if _, err := exec.LookPath("chkconfig"); err == nil {
		exec.Command("chkconfig", "--add", sm.internalName).Run()
		exec.Command("chkconfig", sm.internalName, "on").Run()
	}

	fmt.Printf("SysVinit service installed: %s\n", scriptPath)
	fmt.Printf("Start with: service %s start (or /etc/init.d/%s start)\n", sm.internalName, sm.internalName)
	return nil
}

// installDarwin installs launchd service on macOS per AI.md PART 24
// Binary drops privileges after port binding - no UserName/GroupName needed
func (sm *ServiceManager) installDarwin() error {
	// Per PART 24: Path is /Library/LaunchDaemons/{plist_name}.plist
	// plist_name = io.github.{project_org}.{internal_name}
	plistPath := fmt.Sprintf("/Library/LaunchDaemons/%s.plist", sm.plistName)

	// Per PART 23: No UserName/GroupName - binary handles privilege dropping
	// Per PART 24: Label uses {plist_name}, paths use {project_org}/{internal_name}
	plist := fmt.Sprintf(`<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
    <key>Label</key>
    <string>%s</string>
    <key>ProgramArguments</key>
    <array>
        <string>/usr/local/bin/%s</string>
    </array>
    <key>RunAtLoad</key>
    <true/>
    <key>KeepAlive</key>
    <true/>
    <key>StandardOutPath</key>
    <string>/var/log/%s/%s/stdout.log</string>
    <key>StandardErrorPath</key>
    <string>/var/log/%s/%s/stderr.log</string>
</dict>
</plist>
`, sm.plistName, sm.appName, sm.projectOrg, sm.internalName, sm.projectOrg, sm.internalName)

	if err := os.WriteFile(plistPath, []byte(plist), 0644); err != nil {
		return fmt.Errorf("failed to write plist: %w", err)
	}

	fmt.Printf("LaunchDaemon installed: %s\n", plistPath)
	fmt.Printf("Start with: sudo launchctl load %s\n", plistPath)
	return nil
}

// createDarwinUser creates system user on macOS using dscl per AI.md PART 23
func (sm *ServiceManager) createDarwinUser() error {
	// Check if user exists
	_, err := exec.Command("dscl", ".", "-read", fmt.Sprintf("/Users/%s", sm.user)).CombinedOutput()
	// User already exists
	if err == nil {
		return nil
	}

	// Find available UID in 200-899 range per AI.md PART 23
	uid := sm.findAvailableUID(200, 899)

	// Create user using dscl per AI.md PART 23
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

// installBSD installs rc.d service on BSD per AI.md PART 24
// Binary drops privileges after port binding
func (sm *ServiceManager) installBSD() error {
	// Per PART 24: service name uses {internal_name}
	rcPath := fmt.Sprintf("/usr/local/etc/rc.d/%s", sm.internalName)

	// Per PART 23: binary handles privilege dropping; PART 24: simple rc.d script
	rcScript := fmt.Sprintf(`#!/bin/sh

# PROVIDE: %s
# REQUIRE: NETWORKING
# KEYWORD: shutdown

. /etc/rc.subr

name="%s"
rcvar="${name}_enable"
command="/usr/local/bin/%s"

load_rc_config $name
run_rc_command "$1"
`, sm.internalName, sm.internalName, sm.appName)

	if err := os.WriteFile(rcPath, []byte(rcScript), 0755); err != nil {
		return fmt.Errorf("failed to write rc script: %w", err)
	}

	fmt.Printf("RC script installed: %s\n", rcPath)
	fmt.Printf("Enable with: sysrc %s_enable=YES\n", sm.internalName)
	fmt.Printf("Start with: service %s start\n", sm.internalName)
	return nil
}

// createBSDUser creates system user on BSD using pw per AI.md PART 23
func (sm *ServiceManager) createBSDUser() error {
	// Check if user exists
	_, err := exec.Command("pw", "usershow", sm.user).CombinedOutput()
	if err == nil {
		return nil
	}

	// Find available UID in 200-899 range per AI.md PART 23
	uid := sm.findAvailableUID(200, 899)

	// Create group per AI.md PART 23
	exec.Command("pw", "groupadd", sm.group, "-g", strconv.Itoa(uid)).Run()

	// Create user using pw per AI.md PART 23
	return exec.Command("pw", "useradd", sm.user,
		"-u", strconv.Itoa(uid),
		"-g", sm.group,
		"-d", sm.dataDir,
		"-s", "/usr/sbin/nologin",
		"-c", sm.description,
	).Run()
}

// installWindows installs Windows service per AI.md PART 23
func (sm *ServiceManager) installWindows() error {
	// Create Windows Virtual Service Account per AI.md PART 23
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

// confirmUninstall prints the required prompt and returns nil only if the
// user types "y" or "Y". Per AI.md PART 23: required before any destructive
// uninstall that removes data, config, and the system user.
func confirmUninstall() error {
	fmt.Print("This will delete ALL data, configs, and the system user. Continue? [y/N]: ")
	scanner := bufio.NewScanner(os.Stdin)
	scanner.Scan()
	resp := strings.TrimSpace(scanner.Text())
	if strings.ToLower(resp) != "y" {
		return fmt.Errorf("uninstall aborted")
	}
	return nil
}

// uninstallLinux removes Linux service
func (sm *ServiceManager) uninstallLinux() error {
	if err := confirmUninstall(); err != nil {
		return err
	}
	sm.Stop()

	// Per PART 24: service name uses {internal_name}
	if sm.hasSystemd() {
		exec.Command("systemctl", "disable", sm.internalName).Run()
		os.Remove(fmt.Sprintf("/etc/systemd/system/%s.service", sm.internalName))
		exec.Command("systemctl", "daemon-reload").Run()
	}
	if sm.hasOpenRC() {
		exec.Command("rc-update", "del", sm.internalName, "default").Run()
		os.Remove(fmt.Sprintf("/etc/init.d/%s", sm.internalName))
	}
	if sm.hasSysVInit() {
		if _, err := exec.LookPath("update-rc.d"); err == nil {
			exec.Command("update-rc.d", "-f", sm.internalName, "remove").Run()
		} else if _, err := exec.LookPath("chkconfig"); err == nil {
			exec.Command("chkconfig", "--del", sm.internalName).Run()
		}
		os.Remove(fmt.Sprintf("/etc/init.d/%s", sm.internalName))
	}
	if sm.hasRunit() {
		os.Remove(fmt.Sprintf("/var/service/%s", sm.internalName))
		os.RemoveAll(fmt.Sprintf("/etc/sv/%s", sm.internalName))
	}

	fmt.Printf("Service %s uninstalled\n", sm.internalName)
	return nil
}

// uninstallDarwin removes macOS service
func (sm *ServiceManager) uninstallDarwin() error {
	if err := confirmUninstall(); err != nil {
		return err
	}
	// Per PART 24: Path is /Library/LaunchDaemons/{plist_name}.plist
	plistPath := fmt.Sprintf("/Library/LaunchDaemons/%s.plist", sm.plistName)
	exec.Command("launchctl", "unload", plistPath).Run()
	os.Remove(plistPath)
	fmt.Printf("Service %s uninstalled\n", sm.internalName)
	return nil
}

// uninstallBSD removes BSD service
func (sm *ServiceManager) uninstallBSD() error {
	if err := confirmUninstall(); err != nil {
		return err
	}
	sm.Stop()
	// Per PART 24: service name uses {internal_name}
	os.Remove(fmt.Sprintf("/usr/local/etc/rc.d/%s", sm.internalName))
	fmt.Printf("Service %s uninstalled\n", sm.internalName)
	return nil
}

// uninstallWindows removes Windows service
func (sm *ServiceManager) uninstallWindows() error {
	if err := confirmUninstall(); err != nil {
		return err
	}
	sm.Stop()
	exec.Command("sc", "delete", sm.appName).Run()
	fmt.Printf("Service %s uninstalled\n", sm.appName)
	return nil
}

// DetectEscalation detects available privilege escalation methods per AI.md PART 23
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

// IsRunningAsRoot checks if running as root/administrator
func IsRunningAsRoot() bool {
	switch runtime.GOOS {
	case "windows":
		// Check if running with admin privileges
		_, err := os.Open("\\\\.\\PHYSICALDRIVE0")
		return err == nil
	default:
		return os.Getuid() == 0
	}
}

// IsRunningInContainer checks if running in a container environment per AI.md PART 23
func IsRunningInContainer() bool {
	// File-based detection per AI.md PART 8 7732-7740
	// Docker, Podman, LXC/LXD/Incus marker files
	containerFiles := []string{
		"/.dockerenv",
		"/run/.containerenv",
		"/dev/lxc",
	}
	for _, f := range containerFiles {
		if _, err := os.Stat(f); err == nil {
			return true
		}
	}

	// Environment variable detection per AI.md PART 8 7743-7748
	if os.Getenv("container") != "" {
		// Generic (systemd-nspawn, lxc, etc.)
		return true
	}
	if os.Getenv("KUBERNETES_SERVICE_HOST") != "" {
		// Kubernetes
		return true
	}

	// Check parent process name for container init systems per AI.md PART 8 7752-7758
	parentName := getParentProcessName()
	switch parentName {
	case "tini", "dumb-init", "s6-svscan", "runsv", "runsvdir", "catatonit":
		return true
	case "vidveil":
		// Parent is our own binary - likely container entrypoint
		return true
	}

	// Check cgroup for container indicators per AI.md PART 8 7762-7768
	if data, err := os.ReadFile("/proc/1/cgroup"); err == nil {
		content := string(data)
		if strings.Contains(content, "docker") ||
			strings.Contains(content, "kubepods") ||
			strings.Contains(content, "lxc") {
			return true
		}
	}

	return false
}

// getParentProcessName returns the name of the parent process per AI.md PART 8 7827-7843
func getParentProcessName() string {
	ppid := os.Getppid()

	// Linux: read /proc/{ppid}/comm
	if data, err := os.ReadFile(fmt.Sprintf("/proc/%d/comm", ppid)); err == nil {
		return strings.TrimSpace(string(data))
	}

	// macOS/BSD: use ps command
	cmd := exec.Command("ps", "-p", strconv.Itoa(ppid), "-o", "comm=")
	if output, err := cmd.Output(); err == nil {
		return strings.TrimSpace(string(output))
	}

	return ""
}

// DetectServiceManager returns the active service manager per AI.md PART 23
func DetectServiceManager() string {
	// Check for container environment first
	if IsRunningInContainer() {
		return "container"
	}

	// Check parent process / init system
	ppid := os.Getppid()

	// systemd: parent is systemd or PPID=1 with systemd running
	if ppid == 1 {
		if _, err := os.Stat("/run/systemd/system"); err == nil {
			return "systemd"
		}
	}
	// Also check INVOCATION_ID (set by systemd)
	if os.Getenv("INVOCATION_ID") != "" {
		return "systemd"
	}

	// launchd: macOS with PPID=1
	if runtime.GOOS == "darwin" && ppid == 1 {
		return "launchd"
	}

	// runit: check for SVDIR
	if os.Getenv("SVDIR") != "" {
		return "runit"
	}

	// s6: check for S6_* vars
	if os.Getenv("S6_LOGGING") != "" {
		return "s6"
	}

	// SysV init: /etc/init.d script, no systemd
	if ppid == 1 {
		if _, err := os.Stat("/etc/init.d"); err == nil {
			if _, err := os.Stat("/run/systemd/system"); os.IsNotExist(err) {
				return "sysv"
			}
		}
	}

	// rc.d (BSD): check for rc.subr
	if _, err := os.Stat("/etc/rc.subr"); err == nil {
		return "rcd"
	}

	return "manual"
}

// ShouldDaemonize determines if we should daemonize based on context per AI.md PART 8 7845-7867
func ShouldDaemonize(isServiceStart bool, daemonFlag bool, configDaemonize bool) bool {
	if isServiceStart {
		// Service start - detect manager and ignore config
		switch DetectServiceManager() {
		case "systemd", "launchd", "runit", "s6", "docker", "container":
			// Always foreground
			return false
		case "sysv", "rcd":
			// Always daemonize
			return true
		default:
			// Unknown, default to foreground
			return false
		}
	}

	// Manual start - respect flag and config
	if daemonFlag {
		return true
	}
	return configDaemonize
}

// EnsureSystemUser creates system user/group on startup per AI.md PART 23
// "Binary handles EVERYTHING else: directories, permissions, user/group, Tor, etc."
// Returns uid, gid for chown operations. Returns 0, 0 if not running as root.
func EnsureSystemUser(appName string, dirs []string) (uid, gid int, err error) {
	// Only create user if running as root
	if !IsRunningAsRoot() {
		// Running as non-root user, use current user
		return os.Getuid(), os.Getgid(), nil
	}

	// On Windows, skip user creation (use Virtual Service Account)
	if runtime.GOOS == "windows" {
		return 0, 0, nil
	}

	// Check if user already exists
	output, err := exec.Command("id", "-u", appName).Output()
	if err == nil {
		// User exists, get UID
		uidStr := strings.TrimSpace(string(output))
		uid, _ = strconv.Atoi(uidStr)

		// Get GID
		output, err = exec.Command("id", "-g", appName).Output()
		if err == nil {
			gidStr := strings.TrimSpace(string(output))
			gid, _ = strconv.Atoi(gidStr)
		}

		// Ensure directories are owned by this user
		for _, dir := range dirs {
			if dir != "" {
				os.MkdirAll(dir, 0755)
				exec.Command("chown", "-R", fmt.Sprintf("%s:%s", appName, appName), dir).Run()
			}
		}

		return uid, gid, nil
	}

	// User doesn't exist, create it
	// Find available UID/GID in 200-899 range per AI.md PART 23
	id := findAvailableID(200, 899)

	// Determine home directory (use first data dir if available)
	homeDir := "/nonexistent"
	if len(dirs) > 0 && dirs[0] != "" {
		homeDir = dirs[0]
	}

	// groupadd --system --gid {id} {name} per AI.md PART 23
	// --gid is numeric; Alpine uses addgroup -g -S
	if _, err := exec.LookPath("groupadd"); err == nil {
		exec.Command("groupadd", "--system", "--gid", strconv.Itoa(id), appName).Run()
		// useradd: --system, numeric --gid per AI.md PART 23
		cmd := exec.Command("useradd",
			"--system",
			"--uid", strconv.Itoa(id),
			"--gid", strconv.Itoa(id),
			"--home-dir", homeDir,
			"--shell", "/sbin/nologin",
			"--comment", appName+" service account",
			appName,
		)
		cmd.Run()
	} else {
		// Alpine Linux uses addgroup/adduser (busybox)
		exec.Command("addgroup", "-g", strconv.Itoa(id), "-S", appName).Run()
		cmd := exec.Command("adduser",
			"-D",
			"-S",
			"-H",
			"-u", strconv.Itoa(id),
			"-G", appName,
			"-s", "/sbin/nologin",
			appName,
		)
		cmd.Run()
	}

	// Create directories and set ownership
	for _, dir := range dirs {
		if dir != "" {
			os.MkdirAll(dir, 0755)
			exec.Command("chown", "-R", fmt.Sprintf("%s:%s", appName, appName), dir).Run()
		}
	}

	return id, id, nil
}

// reservedSystemIDs is the set of IDs never to assign, per AI.md PART 23.
// Covers common service UIDs (999-980), well-known daemon IDs (101-110,
// 170-179), and the nobody/nfsnobody sentinel (65534).
var reservedSystemIDs = map[int]bool{
	65534: true,
	999: true, 998: true, 997: true, 996: true, 995: true,
	994: true, 993: true, 992: true, 991: true, 990: true,
	989: true, 988: true, 987: true, 986: true, 985: true,
	984: true, 983: true, 982: true, 981: true, 980: true,
	101: true, 102: true, 103: true, 104: true, 105: true,
	106: true, 107: true, 108: true, 109: true, 110: true,
	170: true, 171: true, 172: true, 173: true, 174: true,
	175: true, 176: true, 177: true, 178: true, 179: true,
}

// findAvailableSystemID finds an available UID/GID per AI.md PART 23.
// Starts at 899 and works DOWN to 200, skipping reserved IDs.
// Uses os/user package (not getent) for portable, subprocess-free lookup.
func findAvailableSystemID() (int, error) {
	for id := 899; id >= 200; id-- {
		if reservedSystemIDs[id] {
			continue
		}
		if _, err := user.LookupId(strconv.Itoa(id)); err == nil {
			continue
		}
		if _, err := user.LookupGroupId(strconv.Itoa(id)); err == nil {
			continue
		}
		return id, nil
	}
	return 0, fmt.Errorf("no available UID/GID in safe range 200-899")
}

// findAvailableID is a backward-compatible wrapper around findAvailableSystemID.
func findAvailableID(min, max int) int {
	id, err := findAvailableSystemID()
	if err != nil {
		return min
	}
	return id
}
