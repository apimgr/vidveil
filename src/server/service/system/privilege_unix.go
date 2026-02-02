// SPDX-License-Identifier: MIT
// AI.md PART 25: Privilege Dropping (Unix)
//go:build !windows

package system

import (
	"fmt"
	"os"
	"os/exec"
	"os/user"
	"strconv"
	"strings"
	"syscall"
)

// DropPrivileges drops from root to the specified user after port binding
// Per AI.md PART 25: "Unix: Service starts as root, binary drops to user after port binding"
// This should be called AFTER binding to privileged ports but BEFORE serving requests
func DropPrivileges(username string) error {
	// Only drop if we're root
	if os.Getuid() != 0 {
		// Already non-root, nothing to do
		return nil
	}

	// If no username specified, use the app name
	if username == "" {
		username = "vidveil"
	}

	// Look up user
	u, err := user.Lookup(username)
	if err != nil {
		// User doesn't exist - create it
		if err := createSystemUser(username); err != nil {
			return fmt.Errorf("failed to create user %s: %w", username, err)
		}
		u, err = user.Lookup(username)
		if err != nil {
			return fmt.Errorf("failed to lookup user %s after creation: %w", username, err)
		}
	}

	uid, err := strconv.Atoi(u.Uid)
	if err != nil {
		return fmt.Errorf("invalid uid for user %s: %w", username, err)
	}

	gid, err := strconv.Atoi(u.Gid)
	if err != nil {
		return fmt.Errorf("invalid gid for user %s: %w", username, err)
	}

	// Drop supplementary groups first
	if err := syscall.Setgroups([]int{gid}); err != nil {
		return fmt.Errorf("setgroups failed: %w", err)
	}

	// Drop group privileges (must be before setuid)
	if err := syscall.Setgid(gid); err != nil {
		return fmt.Errorf("setgid(%d) failed: %w", gid, err)
	}

	// Drop user privileges (final step - cannot regain root after this)
	if err := syscall.Setuid(uid); err != nil {
		return fmt.Errorf("setuid(%d) failed: %w", uid, err)
	}

	// Verify we're no longer root
	if os.Getuid() == 0 || os.Geteuid() == 0 {
		return fmt.Errorf("failed to drop privileges: still running as root")
	}

	return nil
}

// createSystemUser creates a system user for the service
func createSystemUser(username string) error {
	// Find available UID in 200-899 range per AI.md PART 24
	uid := findAvailableID(200, 899)

	// Determine home directory
	homeDir := fmt.Sprintf("/var/lib/apimgr/%s", username)

	// Try standard Linux commands first (Debian, RHEL, etc.)
	if _, err := exec.LookPath("groupadd"); err == nil {
		exec.Command("groupadd", "-g", strconv.Itoa(uid), username).Run()
		cmd := exec.Command("useradd",
			"-r",                            // System account
			"-u", strconv.Itoa(uid),         // UID
			"-g", username,                  // Primary group
			"-d", homeDir,                   // Home directory
			"-s", "/sbin/nologin",           // No login shell
			"-c", username+" service account",
			username,
		)
		return cmd.Run()
	}

	// Alpine Linux uses addgroup/adduser (busybox)
	exec.Command("addgroup", "-g", strconv.Itoa(uid), "-S", username).Run()
	return exec.Command("adduser",
		"-D",                      // Don't assign password
		"-S",                      // System user
		"-H",                      // No home directory
		"-u", strconv.Itoa(uid),   // UID
		"-G", username,            // Primary group
		"-s", "/sbin/nologin",     // No login shell
		username,
	).Run()
}

// ShouldDropPrivileges returns true if running as root and should drop
func ShouldDropPrivileges() bool {
	return os.Getuid() == 0
}

// GetPrivilegeDropUser returns the user to drop privileges to
func GetPrivilegeDropUser() string {
	return "vidveil"
}

// IsElevated returns true if running as root per AI.md PART 24
func IsElevated() bool {
	return os.Geteuid() == 0
}

// CanEscalate checks if user can escalate privileges per AI.md PART 24
// Returns true if user has sudo access (passwordless or with password)
func CanEscalate() bool {
	// Already elevated
	if IsElevated() {
		return true
	}

	// Check sudo -n (non-interactive) to see if user has passwordless sudo
	cmd := exec.Command("sudo", "-n", "true")
	if cmd.Run() == nil {
		return true // Has passwordless sudo
	}

	// Check if user is in sudo/wheel/admin group (can sudo with password)
	u, err := user.Current()
	if err != nil {
		return false
	}

	groups, err := u.GroupIds()
	if err != nil {
		return false
	}

	for _, gid := range groups {
		group, err := user.LookupGroupId(gid)
		if err != nil {
			continue
		}
		if group.Name == "sudo" || group.Name == "wheel" || group.Name == "admin" {
			return true // Can sudo with password
		}
	}

	// Check for doas as alternative
	if _, err := exec.LookPath("doas"); err == nil {
		// Check if user is in doas.conf
		// For simplicity, just check if doas exists and user is in wheel
		return false
	}

	return false
}

// HandleEscalation prompts user and re-executes with elevated privileges per AI.md PART 24
// Returns nil if already elevated or if escalation succeeded
// Returns error if user cannot escalate or declined
func HandleEscalation(action string) error {
	if IsElevated() {
		return nil // Already elevated
	}

	if !CanEscalate() {
		// User CANNOT escalate - don't ask, just inform
		return fmt.Errorf("%s requires administrator privileges\n\n"+
			"You do not have sudo/admin access. Contact your system administrator.", action)
	}

	// User CAN escalate - ask and re-exec with elevated privileges
	fmt.Printf("%s requires elevated privileges.\n", action)
	fmt.Print("Escalate with sudo? [Y/n]: ")

	var response string
	fmt.Scanln(&response)
	response = strings.TrimSpace(strings.ToLower(response))
	if response != "" && response != "y" && response != "yes" {
		return fmt.Errorf("escalation declined")
	}

	// Re-execute with sudo
	return execElevated(os.Args)
}

// execElevated re-executes the current process with elevated privileges
func execElevated(args []string) error {
	// Find sudo or doas
	var elevateCmd string
	if _, err := exec.LookPath("sudo"); err == nil {
		elevateCmd = "sudo"
	} else if _, err := exec.LookPath("doas"); err == nil {
		elevateCmd = "doas"
	} else {
		return fmt.Errorf("no privilege escalation tool found (sudo or doas)")
	}

	// Build command: sudo/doas <current binary> <args...>
	cmdArgs := append([]string{args[0]}, args[1:]...)
	cmd := exec.Command(elevateCmd, cmdArgs...)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("escalation failed: %w", err)
	}

	// Exit after elevated command completes
	os.Exit(0)
	return nil
}
