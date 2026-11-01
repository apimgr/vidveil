//go:build windows

package cmd

import (
	"errors"
	"fmt"
	"os"
	"os/exec"

	"golang.org/x/sys/windows"
)

func replaceCLIBinary(execPath, newBinaryPath string) error {
	oldPath := execPath + ".old"
	if err := os.Remove(oldPath); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("removing previous update backup: %w", err)
	}
	if err := os.Rename(execPath, oldPath); err != nil {
		return fmt.Errorf("renaming current binary: %w", err)
	}
	if err := os.Rename(newBinaryPath, execPath); err != nil {
		restoreErr := os.Rename(oldPath, execPath)
		return fmt.Errorf("moving new binary: %w", errors.Join(err, restoreErr))
	}
	oldPathPtr, err := windows.UTF16PtrFromString(oldPath)
	if err != nil {
		return fmt.Errorf("encoding update backup path: %w", err)
	}
	if err := windows.MoveFileEx(oldPathPtr, nil, windows.MOVEFILE_DELAY_UNTIL_REBOOT); err != nil {
		return fmt.Errorf("scheduling update backup removal: %w", err)
	}
	return nil
}

func restartCLI(execPath string) error {
	process := exec.Command(execPath, os.Args[1:]...)
	process.Stdin = os.Stdin
	process.Stdout = os.Stdout
	process.Stderr = os.Stderr
	process.Env = append(os.Environ(), cliUpdateRestartedEnv+"=1")
	if err := process.Start(); err != nil {
		return fmt.Errorf("starting updated CLI: %w", err)
	}
	if err := process.Process.Release(); err != nil {
		return fmt.Errorf("releasing updated CLI process: %w", err)
	}
	return nil
}
