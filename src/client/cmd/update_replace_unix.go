//go:build !windows

package cmd

import (
	"fmt"
	"os"
	"syscall"
)

func replaceCLIBinary(execPath, newBinaryPath string) error {
	info, err := os.Stat(execPath)
	if err != nil {
		return fmt.Errorf("stating current binary: %w", err)
	}
	if err := os.Rename(newBinaryPath, execPath); err != nil {
		return fmt.Errorf("replacing binary: %w", err)
	}
	if err := os.Chmod(execPath, info.Mode()); err != nil {
		return fmt.Errorf("setting replacement permissions: %w", err)
	}
	return nil
}

func restartCLI(execPath string) error {
	environment := append(os.Environ(), cliUpdateRestartedEnv+"=1")
	return syscall.Exec(execPath, os.Args, environment)
}
