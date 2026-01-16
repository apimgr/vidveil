// SPDX-License-Identifier: MIT
//go:build windows

// AI.md PART 4: Windows UAC Elevation
package system

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"unsafe"
)

var (
	modShell32          = syscall.NewLazyDLL("shell32.dll")
	modAdvapi32         = syscall.NewLazyDLL("advapi32.dll")
	procShellExecuteW   = modShell32.NewProc("ShellExecuteW")
	procGetTokenInfo    = modAdvapi32.NewProc("GetTokenInformation")
	procOpenProcessToken = modAdvapi32.NewProc("OpenProcessToken")
)

const (
	TOKEN_QUERY              = 0x0008
	TokenElevation           = 20
	TokenElevationType       = 18
	TokenElevationTypeDefault = 1
	TokenElevationTypeFull   = 2
	TokenElevationTypeLimited = 3
)

// IsRunningElevated checks if the current process has administrator privileges
// per AI.md PART 4 Windows requirements
func IsRunningElevated() bool {
	var token syscall.Token
	currentProcess, _ := syscall.GetCurrentProcess()

	err := syscall.OpenProcessToken(currentProcess, TOKEN_QUERY, &token)
	if err != nil {
		return false
	}
	defer token.Close()

	// Check elevation status
	var elevation uint32
	var returnedLen uint32

	err = getTokenInformation(token, TokenElevation, unsafe.Pointer(&elevation), 4, &returnedLen)
	if err != nil {
		return false
	}

	return elevation != 0
}

// getTokenInformation is a wrapper for GetTokenInformation
func getTokenInformation(token syscall.Token, infoClass uint32, info unsafe.Pointer, infoLen uint32, returnedLen *uint32) error {
	r1, _, err := procGetTokenInfo.Call(
		uintptr(token),
		uintptr(infoClass),
		uintptr(info),
		uintptr(infoLen),
		uintptr(unsafe.Pointer(returnedLen)),
	)
	if r1 == 0 {
		return err
	}
	return nil
}

// ElevationResult represents the result of an elevation attempt
type ElevationResult int

const (
	ElevationAlreadyAdmin ElevationResult = iota
	ElevationSuccess
	ElevationCanceled
	ElevationFailed
)

// RequestElevation attempts to restart the current process with admin privileges
// per AI.md PART 4: "UAC prompt (requires GUI)"
func RequestElevation(args ...string) ElevationResult {
	if IsRunningElevated() {
		return ElevationAlreadyAdmin
	}

	// Get the current executable path
	exe, err := os.Executable()
	if err != nil {
		return ElevationFailed
	}

	// Build arguments string
	var argsStr string
	if len(args) > 0 {
		argsStr = strings.Join(args, " ")
	} else {
		argsStr = strings.Join(os.Args[1:], " ")
	}

	// Use ShellExecute with "runas" verb to trigger UAC
	err = shellExecute(0, "runas", exe, argsStr, filepath.Dir(exe), syscall.SW_NORMAL)
	if err != nil {
		// User likely canceled UAC
		if strings.Contains(err.Error(), "cancelled") ||
		   strings.Contains(err.Error(), "canceled") {
			return ElevationCanceled
		}
		return ElevationFailed
	}

	return ElevationSuccess
}

// shellExecute wraps the Windows ShellExecuteW API
func shellExecute(hwnd uintptr, operation, file, parameters, directory string, showCmd int) error {
	op, _ := syscall.UTF16PtrFromString(operation)
	f, _ := syscall.UTF16PtrFromString(file)
	p, _ := syscall.UTF16PtrFromString(parameters)
	d, _ := syscall.UTF16PtrFromString(directory)

	r1, _, err := procShellExecuteW.Call(
		hwnd,
		uintptr(unsafe.Pointer(op)),
		uintptr(unsafe.Pointer(f)),
		uintptr(unsafe.Pointer(p)),
		uintptr(unsafe.Pointer(d)),
		uintptr(showCmd),
	)

	// ShellExecuteW returns > 32 on success
	if r1 <= 32 {
		return fmt.Errorf("ShellExecute failed: %v", err)
	}

	return nil
}

// RunAsAdmin runs a command with administrator privileges via UAC
// Falls back to runas command if ShellExecute fails
func RunAsAdmin(command string, args ...string) error {
	if IsRunningElevated() {
		// Already admin, just run directly
		cmd := exec.Command(command, args...)
		return cmd.Run()
	}

	// Try ShellExecute with runas verb first
	argsStr := strings.Join(args, " ")
	err := shellExecute(0, "runas", command, argsStr, "", syscall.SW_NORMAL)
	if err == nil {
		return nil
	}

	// Fallback to runas command (command line, requires admin password)
	runasArgs := append([]string{"/user:Administrator", command}, args...)
	cmd := exec.Command("runas", runasArgs...)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

// RequireAdmin checks if admin privileges are required and requests elevation if needed
// Returns true if the program should exit (because a new elevated process was started)
// per AI.md PART 4 flow
func RequireAdmin(operation string) (bool, error) {
	if IsRunningElevated() {
		return false, nil
	}

	fmt.Printf("Operation '%s' requires administrator privileges.\n", operation)
	fmt.Println("Requesting elevation via UAC...")

	result := RequestElevation()
	switch result {
	case ElevationAlreadyAdmin:
		return false, nil
	// Exit this process, elevated one is starting
	case ElevationSuccess:
		return true, nil
	case ElevationCanceled:
		return false, fmt.Errorf("UAC elevation was canceled by user")
	case ElevationFailed:
		return false, fmt.Errorf("failed to elevate privileges")
	}

	return false, nil
}

// GetWindowsServiceAccount returns the Virtual Service Account name
// per AI.md PART 4: "NT SERVICE\vidveil"
func GetWindowsServiceAccount(serviceName string) string {
	return fmt.Sprintf("NT SERVICE\\%s", serviceName)
}

// IsRunningAsService checks if the current process is running as a Windows service
func IsRunningAsService() bool {
	// Check if stdin is attached - services don't have stdin
	fi, err := os.Stdin.Stat()
	// Probably a service
	if err != nil {
		return true
	}

	// If no character device (no console), likely a service
	return fi.Mode()&os.ModeCharDevice == 0
}
