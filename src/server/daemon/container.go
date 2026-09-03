// SPDX-License-Identifier: MIT
// AI.md PART 8: Container and service manager detection

package daemon

import (
	"fmt"
	"os"
	"os/exec"
	"runtime"
	"strconv"
	"strings"
)

// isContainer returns true if running inside a container per AI.md PART 8.
func isContainer() bool {
	// File-based detection
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

	// Environment variable detection
	// Generic container env var (systemd-nspawn, lxc, etc.)
	if os.Getenv("container") != "" {
		return true
	}
	// Kubernetes pod indicator
	if os.Getenv("KUBERNETES_SERVICE_HOST") != "" {
		return true
	}

	// Check parent process name for container init systems
	parentName := getParentProcessName()
	switch parentName {
	case "tini", "dumb-init", "s6-svscan", "runsv", "runsvdir", "catatonit":
		return true
	case "vidveil":
		// Parent is our own binary — likely container entrypoint
		return true
	}

	// Check cgroup for container indicators
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

// detectServiceManager returns the active service manager per AI.md PART 8.
func detectServiceManager() string {
	// Check for container environment first
	if isContainer() {
		return "container"
	}

	ppid := os.Getppid()

	// systemd: PPID=1 and /run/systemd/system exists
	if ppid == 1 {
		if _, err := os.Stat("/run/systemd/system"); err == nil {
			return "systemd"
		}
	}
	// Also check INVOCATION_ID (set by systemd for all units)
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

	// SysV init: /etc/init.d exists but no systemd
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

// getParentProcessName returns the name of the parent process per AI.md PART 8.
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

// shouldDaemonize determines if the process should daemonize based on context.
// Per AI.md PART 8: service managers (systemd, launchd, etc.) handle daemonization;
// the binary should run in foreground for them. Manual start respects flags.
func shouldDaemonize(isServiceStart bool, daemonFlag bool, configDaemonize bool) bool {
	if isServiceStart {
		switch detectServiceManager() {
		case "systemd", "launchd", "runit", "s6", "container":
			// These managers require foreground
			return false
		case "sysv", "rcd":
			// Traditional init systems expect background
			return true
		default:
			return false
		}
	}

	// Manual start — respect flag then config
	if daemonFlag {
		return true
	}
	return configDaemonize
}
