//go:build linux

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
	"syscall"
	"time"
)

func configureI2PDCommand(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{Pdeathsig: syscall.SIGTERM}
}

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
	procPath := filepath.Join("/proc", strconv.Itoa(pid))
	command, err := os.ReadFile(filepath.Join(procPath, "cmdline"))
	if errors.Is(err, os.ErrNotExist) {
		return removeStaleI2PPID(pidPath, nil)
	}
	if err != nil {
		return fmt.Errorf("inspect stale i2pd process %d: %w", pid, err)
	}
	args := strings.Split(strings.TrimRight(string(command), "\x00"), "\x00")
	if len(args) == 0 || args[0] == "" {
		return removeStaleI2PPID(pidPath, nil)
	}
	actualBinary, linkErr := os.Readlink(filepath.Join(procPath, "exe"))
	expectedBinary, _ := filepath.EvalSymlinks(binary)
	if linkErr != nil || expectedBinary == "" || actualBinary != expectedBinary || !hasI2PConfigArg(args, confPath) {
		return fmt.Errorf("I2P pid file %s points to a process that does not match this instance; refusing to stop it", pidPath)
	}
	process, err := os.FindProcess(pid)
	if err != nil {
		return removeStaleI2PPID(pidPath, err)
	}
	if err := process.Signal(syscall.SIGTERM); err != nil && !errors.Is(err, os.ErrProcessDone) {
		return fmt.Errorf("terminate stale i2pd process %d: %w", pid, err)
	}
	deadline := time.Now().Add(10 * time.Second)
	for time.Now().Before(deadline) {
		if _, err := os.Stat(procPath); errors.Is(err, os.ErrNotExist) {
			return removeStaleI2PPID(pidPath, nil)
		}
		state, _ := os.ReadFile(filepath.Join(procPath, "stat"))
		if procState(string(state)) == 'Z' {
			return removeStaleI2PPID(pidPath, nil)
		}
		time.Sleep(100 * time.Millisecond)
	}
	if err := process.Kill(); err != nil && !errors.Is(err, os.ErrProcessDone) {
		return fmt.Errorf("kill stale i2pd process %d: %w", pid, err)
	}
	deadline = time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if _, err := os.Stat(procPath); errors.Is(err, os.ErrNotExist) {
			return removeStaleI2PPID(pidPath, nil)
		}
		state, _ := os.ReadFile(filepath.Join(procPath, "stat"))
		if procState(string(state)) == 'Z' {
			return removeStaleI2PPID(pidPath, nil)
		}
		time.Sleep(50 * time.Millisecond)
	}
	return fmt.Errorf("i2pd process %d remained after forced termination", pid)
}

func hasI2PConfigArg(args []string, confPath string) bool {
	for index, arg := range args {
		if arg == "--conf" && index+1 < len(args) && args[index+1] == confPath {
			return true
		}
		if strings.TrimPrefix(arg, "--conf=") == confPath && strings.HasPrefix(arg, "--conf=") {
			return true
		}
	}
	joined := strings.Join(args, " ")
	return strings.Contains(joined, "--conf "+confPath) || strings.Contains(joined, "--conf="+confPath)
}

func procState(stat string) byte {
	fields := strings.Fields(stat)
	if len(fields) > 2 && len(fields[2]) == 1 {
		return fields[2][0]
	}
	return 0
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
