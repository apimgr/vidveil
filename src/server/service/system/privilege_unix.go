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
	"syscall"
)

// DropPrivileges drops from root to the specified user after port binding
// Per AI.md PART 25: "Unix: Service starts as root, binary drops to user after port binding"
// This should be called AFTER binding to privileged ports but BEFORE serving requests
func DropPrivileges(username string) error {
	// Only drop if we're root
	if os.Getuid() != 0 {
		return nil // Already non-root, nothing to do
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
