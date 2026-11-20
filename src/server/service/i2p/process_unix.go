//go:build !windows && !linux

// SPDX-License-Identifier: MIT
package i2p

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"syscall"
	"time"
)

func terminateI2PDProcess(cmd *exec.Cmd) error {
	if cmd == nil || cmd.Process == nil {
		return nil
	}
	err := cmd.Process.Signal(syscall.SIGTERM)
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
	process, err := os.FindProcess(pid)
	if err != nil {
		return removeStaleI2PPID(pidPath, err)
	}
	if err := process.Signal(syscall.Signal(0)); err != nil {
		return removeStaleI2PPID(pidPath, err)
	}
	output, err := exec.Command("ps", "-p", strconv.Itoa(pid), "-o", "stat=,command=").Output()
	if err != nil {
		return fmt.Errorf("verify existing i2pd process %d: %w", pid, err)
	}
	processInfo := strings.TrimSpace(string(output))
	if processInfo == "" || strings.HasPrefix(processInfo, "Z") {
		return removeStaleI2PPID(pidPath, nil)
	}
	if !strings.Contains(processInfo, binary) || !strings.Contains(processInfo, confPath) {
		return fmt.Errorf("I2P pid file %s points to a process that does not match this instance; refusing to stop it", pidPath)
	}
	if err := process.Signal(syscall.SIGTERM); err != nil && !errors.Is(err, os.ErrProcessDone) {
		return fmt.Errorf("terminate stale i2pd process %d: %w", pid, err)
	}
	deadline := time.Now().Add(10 * time.Second)
	for time.Now().Before(deadline) {
		if err := process.Signal(syscall.Signal(0)); err != nil {
			return removeStaleI2PPID(pidPath, nil)
		}
		time.Sleep(100 * time.Millisecond)
	}
	if err := process.Kill(); err != nil && !errors.Is(err, os.ErrProcessDone) {
		return fmt.Errorf("kill stale i2pd process %d: %w", pid, err)
	}
	deadline = time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if err := process.Signal(syscall.Signal(0)); err != nil {
			return removeStaleI2PPID(pidPath, nil)
		}
		time.Sleep(50 * time.Millisecond)
	}
	return fmt.Errorf("i2pd process %d remained after forced termination", pid)
}

func removeStaleI2PPID(pidPath string, cause error) error {
	if cause != nil && !errors.Is(cause, os.ErrProcessDone) && !errors.Is(cause, syscall.ESRCH) {
		return fmt.Errorf("check stale i2pd process: %w", cause)
	}
	if err := os.Remove(pidPath); err != nil && !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("remove stale I2P pid file: %w", err)
	}
	return nil
}
