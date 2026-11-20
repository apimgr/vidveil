//go:build windows

// SPDX-License-Identifier: MIT
package i2p

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

func configureI2PDCommand(*exec.Cmd) {}

func terminateI2PDProcess(cmd *exec.Cmd) error {
	if cmd == nil || cmd.Process == nil {
		return nil
	}
	err := cmd.Process.Kill()
	if errors.Is(err, os.ErrProcessDone) {
		return nil
	}
	return err
}

func reapStaleI2PD(binary, confPath, pidPath string) error {
	pid, exists, err := readI2PPID(pidPath)
	if err != nil || !exists {
		return err
	}
	filter := fmt.Sprintf("PID eq %d", pid)
	output, err := exec.Command("tasklist.exe", "/FI", filter, "/FO", "CSV", "/NH").Output()
	if err != nil {
		return fmt.Errorf("verify existing i2pd process %d: %w", pid, err)
	}
	if !strings.Contains(string(output), strconv.Itoa(pid)) {
		return removeStaleI2PPID(pidPath, nil)
	}
	query := fmt.Sprintf("(Get-CimInstance Win32_Process -Filter 'ProcessId=%d').CommandLine", pid)
	commandLine, err := exec.Command("powershell.exe", "-NoProfile", "-NonInteractive", "-Command", query).Output()
	if err != nil {
		return fmt.Errorf("verify i2pd process command line %d: %w", pid, err)
	}
	processInfo := strings.ToLower(string(commandLine))
	if !strings.Contains(processInfo, strings.ToLower(filepath.Base(binary))) || !strings.Contains(processInfo, strings.ToLower(confPath)) {
		return fmt.Errorf("I2P pid file %s points to a process that does not match this instance; refusing to stop it", pidPath)
	}
	process, err := os.FindProcess(pid)
	if err != nil {
		return err
	}
	if err := process.Kill(); err != nil && !errors.Is(err, os.ErrProcessDone) {
		return fmt.Errorf("terminate stale i2pd process %d: %w", pid, err)
	}
	deadline := time.Now().Add(10 * time.Second)
	for time.Now().Before(deadline) {
		output, _ := exec.Command("tasklist.exe", "/FI", filter, "/FO", "CSV", "/NH").Output()
		if !strings.Contains(string(output), strconv.Itoa(pid)) {
			return removeStaleI2PPID(pidPath, nil)
		}
		time.Sleep(100 * time.Millisecond)
	}
	return fmt.Errorf("i2pd process %d did not exit after termination", pid)
}

func removeStaleI2PPID(pidPath string, _ error) error {
	if err := os.Remove(pidPath); err != nil && !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("remove stale I2P pid file: %w", err)
	}
	return nil
}
