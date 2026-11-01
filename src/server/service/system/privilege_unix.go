// SPDX-License-Identifier: MIT
// AI.md PART 23: Privilege Dropping (Unix)
//go:build !windows

package system

import (
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"os/user"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"syscall"

	"github.com/apimgr/vidveil/src/common/i18n"
)

// DropPrivileges drops from root to the specified user after port binding
// Per AI.md PART 23: "Unix: Service starts as root, binary drops to user after port binding"
// This should be called AFTER binding to privileged ports but BEFORE serving requests
func DropPrivileges(username string, configDirs ...string) error {
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
		configDir := ""
		if len(configDirs) > 0 {
			configDir = configDirs[0]
		}
		if err := createSystemUser(username, configDir); err != nil {
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
func createSystemUser(username, configDir string) error {
	// Find available UID in 200-899 range per AI.md PART 23
	uid, err := findAvailableSystemID()
	if runtime.GOOS == "darwin" {
		uid, err = findAvailableMacOSSystemID()
	}
	if err != nil {
		return err
	}
	if runtime.GOOS != "linux" && runtime.GOOS != "darwin" && runtime.GOOS != "freebsd" {
		return fmt.Errorf("automatic service user creation is unsupported on %s", runtime.GOOS)
	}

	// Determine home directory
	homeDir := fmt.Sprintf("/var/lib/apimgr/%s", username)
	if runtime.GOOS == "darwin" {
		homeDir = filepath.Join("/usr/local/var", "apimgr", username)
	}

	// groupadd --system --gid {id} {name} per AI.md PART 23
	// --gid is numeric; Alpine uses addgroup -g -S
	if configDir == "" {
		switch runtime.GOOS {
		case "darwin":
			configDir = filepath.Join("/Library/Application Support", "apimgr", username)
		case "freebsd":
			configDir = filepath.Join("/usr/local/etc", "apimgr", username)
		default:
			configDir = filepath.Join("/etc", "apimgr", username)
		}
	}
	if err := os.MkdirAll(configDir, 0755); err != nil {
		return fmt.Errorf("create config directory for service user: %w", err)
	}
	if err := os.MkdirAll(homeDir, 0755); err != nil {
		return fmt.Errorf("create home directory for service user: %w", err)
	}
	if runtime.GOOS == "darwin" {
		if err := createMacOSServiceUser(username, uid, homeDir); err != nil {
			return err
		}
		if err := chownServiceTree(homeDir, uid, uid); err != nil {
			return err
		}
		if err := chownServiceTree(configDir, uid, uid); err != nil {
			return err
		}
		return markServiceUserCreated(configDir, username)
	}
	if runtime.GOOS == "freebsd" {
		if err := exec.Command("pw", "groupadd", "-n", username, "-g", strconv.Itoa(uid)).Run(); err != nil {
			return fmt.Errorf("create service group: %w", err)
		}
		if err := exec.Command("pw", "useradd", "-n", username, "-u", strconv.Itoa(uid), "-g", strconv.Itoa(uid), "-d", homeDir, "-s", "/usr/sbin/nologin", "-c", username+" service account").Run(); err != nil {
			return fmt.Errorf("create service user: %w", err)
		}
		if err := chownServiceTree(homeDir, uid, uid); err != nil {
			return err
		}
		if err := chownServiceTree(configDir, uid, uid); err != nil {
			return err
		}
		return markServiceUserCreated(configDir, username)
	}
	if _, err := exec.LookPath("groupadd"); err == nil {
		// Group may already exist; useradd below fails loudly if it does not
		if err := exec.Command("groupadd", "--system", "--gid", strconv.Itoa(uid), username).Run(); err != nil {
			return fmt.Errorf("create service group: %w", err)
		}
		// useradd: --system, numeric --gid per AI.md PART 23
		cmd := exec.Command("useradd",
			"--system",
			"--uid", strconv.Itoa(uid),
			"--gid", strconv.Itoa(uid),
			"--home-dir", homeDir,
			"--shell", "/sbin/nologin",
			"--comment", username+" service account",
			username,
		)
		if err := cmd.Run(); err != nil {
			return err
		}
		if err := chownServiceTree(homeDir, uid, uid); err != nil {
			return err
		}
		if err := chownServiceTree(configDir, uid, uid); err != nil {
			return err
		}
		return markServiceUserCreated(configDir, username)
	}

	// Alpine Linux uses addgroup/adduser (busybox)
	// Group may already exist; adduser below fails loudly if it does not
	if err := exec.Command("addgroup", "-g", strconv.Itoa(uid), "-S", username).Run(); err != nil {
		return fmt.Errorf("create service group: %w", err)
	}
	if err := exec.Command("adduser",
		"-D",
		"-S",
		"-H",
		"-u", strconv.Itoa(uid),
		"-G", username,
		"-s", "/sbin/nologin",
		username,
	).Run(); err != nil {
		return err
	}
	if err := chownServiceTree(homeDir, uid, uid); err != nil {
		return err
	}
	if err := chownServiceTree(configDir, uid, uid); err != nil {
		return err
	}
	return markServiceUserCreated(configDir, username)
}

// ShouldDropPrivileges returns true if running as root and should drop
func ShouldDropPrivileges() bool {
	return os.Getuid() == 0
}

// GetPrivilegeDropUser returns the user to drop privileges to
func GetPrivilegeDropUser() string {
	return "vidveil"
}

// IsElevated returns true if running as root per AI.md PART 23
func IsElevated() bool {
	return os.Geteuid() == 0
}

// canEscalate checks if user can escalate privileges per AI.md PART 23
// Returns true if user has sudo access (passwordless or with password)
func canEscalate() bool {
	// Already elevated
	if IsElevated() {
		return true
	}

	// Check sudo -n (non-interactive) to see if user has passwordless sudo
	cmd := exec.Command("sudo", "-n", "true")
	if cmd.Run() == nil {
		// Has passwordless sudo
		return true
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
			// Can sudo with password
			return true
		}
	}

	// Check for doas as alternative per AI.md PART 23 (BSD/Alpine escalation
	// tool). execElevated() below is willing to use doas whenever it is on
	// PATH, so canEscalate() must report the same capability here — a doas
	// binary present is treated as sufficient, matching the sudo -n check
	// above (doas has no standard non-interactive probe equivalent to
	// `sudo -n true`, so presence on PATH is the best available signal).
	if _, err := exec.LookPath("doas"); err == nil {
		return true
	}

	return false
}

// HandleEscalation prompts user and re-executes with elevated privileges per AI.md PART 23
// Returns nil if already elevated or if escalation succeeded
// Returns error if user cannot escalate or declined
func HandleEscalation(action string, locales ...string) error {
	locale := "en"
	if len(locales) > 0 && strings.TrimSpace(locales[0]) != "" {
		locale = locales[0]
	}
	if IsElevated() {
		// Already elevated
		return nil
	}

	if !canEscalate() {
		// User CANNOT escalate - don't ask, just inform
		return errors.New(strings.ReplaceAll(i18n.Translate(locale, "cli.privilege.unavailable"), "{action}", action))
	}

	// User CAN escalate - ask and re-exec with elevated privileges
	fmt.Print(strings.ReplaceAll(i18n.Translate(locale, "cli.privilege.prompt_unix"), "{action}", action))

	var response string
	if _, err := fmt.Scanln(&response); err != nil {
		if errors.Is(err, io.EOF) {
			return errors.New(i18n.Translate(locale, "cli.privilege.declined"))
		}
		return fmt.Errorf("read privilege confirmation: %w", err)
	}
	response = strings.TrimSpace(strings.ToLower(response))
	if response != "" && response != "y" && response != "yes" {
		return errors.New(i18n.Translate(locale, "cli.privilege.declined"))
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
