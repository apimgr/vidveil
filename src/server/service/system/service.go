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

// Disable disables the service from starting at boot per AI.md PART 8
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
		plistPath := fmt.Sprintf("/Library/LaunchDaemons/apimgr.%s.plist", sm.appName)
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
	fmt.Fprintf(os.Stderr, "[DEBUG] installLinux: Starting Linux service installation\n")
	
	// Create system user per AI.md PART 4
	fmt.Fprintf(os.Stderr, "[DEBUG] installLinux: Calling createLinuxUser\n")
	if err := sm.createLinuxUser(); err != nil {
		return fmt.Errorf("failed to create user: %w", err)
	}
	
	fmt.Fprintf(os.Stderr, "[DEBUG] installLinux: User creation completed\n")

	// Install based on init system
	if sm.hasSystemd() {
		fmt.Fprintf(os.Stderr, "[DEBUG] installLinux: Installing systemd service\n")
		return sm.installSystemd()
	}
	if sm.hasRunit() {
		fmt.Fprintf(os.Stderr, "[DEBUG] installLinux: Installing runit service\n")
		return sm.installRunit()
	}
	return fmt.Errorf("no supported init system found (systemd or runit)")
}

// createLinuxUser creates system user per AI.md PART 4
func (sm *ServiceManager) createLinuxUser() error {
	fmt.Fprintf(os.Stderr, "[DEBUG] createLinuxUser: Checking if user '%s' exists...\n", sm.user)
	
	// Check if user exists
	_, err := exec.Command("id", sm.user).CombinedOutput()
	// User already exists
	if err == nil {
		fmt.Fprintf(os.Stderr, "[DEBUG] createLinuxUser: User '%s' already exists, skipping creation\n", sm.user)
		return nil
	}
	
	fmt.Fprintf(os.Stderr, "[DEBUG] createLinuxUser: User '%s' does not exist, creating...\n", sm.user)

	// Find available UID in 200-899 range per AI.md PART 24
	uid := sm.findAvailableUID(200, 899)
	fmt.Fprintf(os.Stderr, "[DEBUG] createLinuxUser: Found available UID: %d\n", uid)

	// Create group
	fmt.Fprintf(os.Stderr, "[DEBUG] createLinuxUser: Creating group '%s' with GID %d\n", sm.group, uid)
	if err := exec.Command("groupadd", "-g", strconv.Itoa(uid), sm.group).Run(); err != nil {
		fmt.Fprintf(os.Stderr, "[DEBUG] createLinuxUser: groupadd failed: %v\n", err)
	}

	// Create system user with:
	// -r: System account
	// -u: UID
	// -g: Primary group
	// -d: Home directory
	// -s: No login shell
	// -c: Comment/description
	fmt.Fprintf(os.Stderr, "[DEBUG] createLinuxUser: Creating user '%s' with UID %d\n", sm.user, uid)
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
		fmt.Fprintf(os.Stderr, "[DEBUG] createLinuxUser: useradd failed: %v\n", err)
		return fmt.Errorf("failed to create user: %w", err)
	}
	
	fmt.Fprintf(os.Stderr, "[DEBUG] createLinuxUser: User '%s' created successfully\n", sm.user)

	// Create and set ownership of directories per PART 25
	// All directories must exist for systemd's ReadWritePaths to work
	dirs := []string{
		"/etc/apimgr/" + sm.appName,
		"/var/lib/apimgr/" + sm.appName,
		"/var/cache/apimgr/" + sm.appName,
		"/var/log/apimgr/" + sm.appName,
	}
	for _, dir := range dirs {
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

// installSystemd installs systemd service unit per AI.md PART 25
// Service starts as root, binary drops privileges after port binding
func (sm *ServiceManager) installSystemd() error {
	unitPath := fmt.Sprintf("/etc/systemd/system/%s.service", sm.appName)

	// Per PART 25: NO User/Group - binary drops privileges after port binding
	unit := fmt.Sprintf(`[Unit]
Description=%s service
Documentation=https://x.scour.li
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
ReadWritePaths=/etc/apimgr/%s
ReadWritePaths=/var/lib/apimgr/%s
ReadWritePaths=/var/cache/apimgr/%s
ReadWritePaths=/var/log/apimgr/%s

[Install]
WantedBy=multi-user.target
`, sm.appName, sm.appName, sm.appName, sm.appName, sm.appName, sm.appName)

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

// installRunit installs runit service per AI.md PART 25
// Binary drops privileges after port binding - no chpst needed
func (sm *ServiceManager) installRunit() error {
	serviceDir := fmt.Sprintf("/etc/sv/%s", sm.appName)
	os.MkdirAll(serviceDir, 0755)

	// Per PART 25: Simple run script, binary handles privilege dropping
	runScript := fmt.Sprintf(`#!/bin/sh
exec /usr/local/bin/%s 2>&1
`, sm.appName)

	runPath := filepath.Join(serviceDir, "run")
	if err := os.WriteFile(runPath, []byte(runScript), 0755); err != nil {
		return fmt.Errorf("failed to write run script: %w", err)
	}

	// Create log directory and script per PART 25
	logDir := filepath.Join(serviceDir, "log")
	os.MkdirAll(logDir, 0755)

	logScript := fmt.Sprintf(`#!/bin/sh
exec svlogd -tt /var/log/apimgr/%s
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

// installDarwin installs launchd service on macOS per AI.md PART 25
// Binary drops privileges after port binding - no UserName/GroupName needed
func (sm *ServiceManager) installDarwin() error {
	// Per PART 25: Path is /Library/LaunchDaemons/apimgr.{appname}.plist
	plistPath := fmt.Sprintf("/Library/LaunchDaemons/apimgr.%s.plist", sm.appName)

	// Per PART 25: No UserName/GroupName - binary handles privilege dropping
	plist := fmt.Sprintf(`<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
    <key>Label</key>
    <string>apimgr.%s</string>
    <key>ProgramArguments</key>
    <array>
        <string>/usr/local/bin/%s</string>
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
`, sm.appName, sm.appName, sm.appName, sm.appName)

	if err := os.WriteFile(plistPath, []byte(plist), 0644); err != nil {
		return fmt.Errorf("failed to write plist: %w", err)
	}

	fmt.Printf("LaunchDaemon installed: %s\n", plistPath)
	fmt.Printf("Start with: sudo launchctl load %s\n", plistPath)
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

	// Find available UID in 200-899 range per AI.md PART 24
	uid := sm.findAvailableUID(200, 899)

	// Create user using dscl per AI.md PART 24
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

// installBSD installs rc.d service on BSD per AI.md PART 25
// Binary drops privileges after port binding
func (sm *ServiceManager) installBSD() error {
	rcPath := fmt.Sprintf("/usr/local/etc/rc.d/%s", sm.appName)

	// Per PART 25: Simple rc.d script, binary handles privilege dropping
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
`, sm.appName, sm.appName, sm.appName)

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

	// Find available UID in 200-899 range per AI.md PART 24
	uid := sm.findAvailableUID(200, 899)

	// Create group per AI.md PART 24
	exec.Command("pw", "groupadd", sm.group, "-g", strconv.Itoa(uid)).Run()

	// Create user using pw per AI.md PART 24
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
	// Per PART 25: Path is /Library/LaunchDaemons/apimgr.{appname}.plist
	plistPath := fmt.Sprintf("/Library/LaunchDaemons/apimgr.%s.plist", sm.appName)
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

// IsRunningInContainer checks if running in a container environment per AI.md PART 8
func IsRunningInContainer() bool {
	// File-based detection per AI.md PART 8 7732-7740
	containerFiles := []string{
		"/.dockerenv",        // Docker
		"/run/.containerenv", // Podman
		"/dev/lxc",           // LXC/LXD/Incus
	}
	for _, f := range containerFiles {
		if _, err := os.Stat(f); err == nil {
			return true
		}
	}

	// Environment variable detection per AI.md PART 8 7743-7748
	if os.Getenv("container") != "" {
		return true // Generic (systemd-nspawn, lxc, etc.)
	}
	if os.Getenv("KUBERNETES_SERVICE_HOST") != "" {
		return true // Kubernetes
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

// DetectServiceManager returns the active service manager per AI.md PART 8
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

// EnsureSystemUser creates system user/group on startup per AI.md PART 27
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
	// Find available UID/GID in 200-899 range per AI.md PART 24
	id := findAvailableID(200, 899)

	// Determine home directory (use first data dir if available)
	homeDir := "/nonexistent"
	if len(dirs) > 0 && dirs[0] != "" {
		homeDir = dirs[0]
	}

	// Try standard Linux commands first (Debian, RHEL, etc.)
	if _, err := exec.LookPath("groupadd"); err == nil {
		exec.Command("groupadd", "-g", strconv.Itoa(id), appName).Run()
		cmd := exec.Command("useradd",
			"-r",                              // System account
			"-u", strconv.Itoa(id),            // UID
			"-g", appName,                     // Primary group
			"-d", homeDir,                     // Home directory
			"-s", "/sbin/nologin",             // No login shell
			"-c", appName+" service account",  // Comment
			appName,
		)
		cmd.Run()
	} else {
		// Alpine Linux uses addgroup/adduser (busybox)
		exec.Command("addgroup", "-g", strconv.Itoa(id), "-S", appName).Run()
		cmd := exec.Command("adduser",
			"-D",                          // Don't assign password
			"-S",                          // System user
			"-H",                          // No home directory
			"-u", strconv.Itoa(id),        // UID
			"-G", appName,                 // Primary group
			"-s", "/sbin/nologin",         // No login shell
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

// findAvailableID finds an available UID/GID in the given range per AI.md PART 24
func findAvailableID(min, max int) int {
	// Start from top of range (999) and work down
	for id := max; id >= min; id-- {
		idStr := strconv.Itoa(id)

		// Check if UID is unused
		uidUsed := false
		// Try getent first (standard Linux)
		if _, err := exec.Command("getent", "passwd", idStr).Output(); err == nil {
			uidUsed = true
		} else {
			// Fallback: grep /etc/passwd for :UID: pattern
			// grep -q returns 0 if found, non-zero if not found
			if err := exec.Command("grep", "-q", ":"+idStr+":", "/etc/passwd").Run(); err == nil {
				uidUsed = true
			}
		}
		if uidUsed {
			continue
		}

		// Check if GID is unused
		gidUsed := false
		if _, err := exec.Command("getent", "group", idStr).Output(); err == nil {
			gidUsed = true
		} else {
			if err := exec.Command("grep", "-q", ":"+idStr+":", "/etc/group").Run(); err == nil {
				gidUsed = true
			}
		}
		if gidUsed {
			continue
		}

		// Both available
		return id
	}
	return min
}
