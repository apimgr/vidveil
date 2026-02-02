// SPDX-License-Identifier: MIT
// AI.md PART 25: Privilege Dropping (Windows)
//go:build windows

package system

import (
	"fmt"
	"os"
	"strings"
	"syscall"

	"golang.org/x/sys/windows"
)

// DropPrivileges is a no-op on Windows per AI.md PART 25
// Windows uses Virtual Service Account (NT SERVICE\vidveil) which is already minimal-privilege
func DropPrivileges(username string) error {
	// Windows: No privilege dropping needed
	// Virtual Service Account (VSA) is already a minimal-privilege isolated account
	return nil
}

// ShouldDropPrivileges returns false on Windows - VSA handles this
func ShouldDropPrivileges() bool {
	return false
}

// GetPrivilegeDropUser returns empty on Windows - uses VSA
func GetPrivilegeDropUser() string {
	return ""
}

// IsElevated returns true if running as Administrator per AI.md PART 24
func IsElevated() bool {
	var sid *windows.SID
	err := windows.AllocateAndInitializeSid(
		&windows.SECURITY_NT_AUTHORITY,
		2,
		windows.SECURITY_BUILTIN_DOMAIN_RID,
		windows.DOMAIN_ALIAS_RID_ADMINS,
		0, 0, 0, 0, 0, 0,
		&sid)
	if err != nil {
		return false
	}
	defer windows.FreeSid(sid)

	token := windows.Token(0)
	member, err := token.IsMember(sid)
	return err == nil && member
}

// CanEscalate checks if user can escalate via UAC per AI.md PART 24
func CanEscalate() bool {
	// If already elevated, no need to escalate
	if IsElevated() {
		return true
	}
	// On Windows, any interactive user can potentially elevate via UAC
	// (unless UAC is disabled or policy prevents it)
	// Check if user is in Administrators group (can elevate)
	return isInAdminGroup()
}

// isInAdminGroup checks if user is member of Administrators group
func isInAdminGroup() bool {
	var sid *windows.SID
	err := windows.AllocateAndInitializeSid(
		&windows.SECURITY_NT_AUTHORITY,
		2,
		windows.SECURITY_BUILTIN_DOMAIN_RID,
		windows.DOMAIN_ALIAS_RID_ADMINS,
		0, 0, 0, 0, 0, 0,
		&sid)
	if err != nil {
		return false
	}
	defer windows.FreeSid(sid)

	// Get current process token
	var token windows.Token
	proc := windows.CurrentProcess()
	err = windows.OpenProcessToken(proc, windows.TOKEN_QUERY, &token)
	if err != nil {
		return false
	}
	defer token.Close()

	// Check group membership
	member, err := token.IsMember(sid)
	return err == nil && member
}

// HandleEscalation prompts user and re-executes with elevated privileges per AI.md PART 24
func HandleEscalation(action string) error {
	if IsElevated() {
		return nil // Already elevated
	}

	if !CanEscalate() {
		return fmt.Errorf("%s requires administrator privileges\n\n"+
			"You do not have admin access. Contact your system administrator.", action)
	}

	// User CAN escalate - ask and re-exec with elevated privileges
	fmt.Printf("%s requires elevated privileges.\n", action)
	fmt.Print("Escalate with UAC? [Y/n]: ")

	var response string
	fmt.Scanln(&response)
	response = strings.TrimSpace(strings.ToLower(response))
	if response != "" && response != "y" && response != "yes" {
		return fmt.Errorf("escalation declined")
	}

	// Re-execute with elevated privileges via UAC
	return execElevated(os.Args)
}

// execElevated re-executes the current process with elevated privileges via UAC
func execElevated(args []string) error {
	verb := "runas"
	exe, _ := os.Executable()
	cwd, _ := os.Getwd()
	argStr := strings.Join(args[1:], " ")

	verbPtr, _ := syscall.UTF16PtrFromString(verb)
	exePtr, _ := syscall.UTF16PtrFromString(exe)
	argPtr, _ := syscall.UTF16PtrFromString(argStr)
	cwdPtr, _ := syscall.UTF16PtrFromString(cwd)

	err := windows.ShellExecute(0, verbPtr, exePtr, argPtr, cwdPtr, windows.SW_NORMAL)
	if err != nil {
		return fmt.Errorf("UAC escalation failed: %w", err)
	}

	// Exit after launching elevated process
	os.Exit(0)
	return nil
}
