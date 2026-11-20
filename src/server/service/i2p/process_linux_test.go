//go:build linux

// SPDX-License-Identifier: MIT
package i2p

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"syscall"
	"testing"
	"time"
)

func TestConfigureI2PDCommandUsesParentDeathSignal(t *testing.T) {
	cmd := exec.Command("i2pd")
	configureI2PDCommand(cmd)
	if cmd.SysProcAttr == nil || cmd.SysProcAttr.Pdeathsig != syscall.SIGTERM {
		t.Fatalf("i2pd parent-death signal = %v, want SIGTERM", cmd.SysProcAttr)
	}
}

func TestReapStaleI2PDStopsMatchingProcess(t *testing.T) {
	root := t.TempDir()
	confPath := filepath.Join(root, "i2pd.conf")
	pidPath := filepath.Join(root, "i2pd.pid")
	command := exec.Command("/bin/sh", "-c", "read -r line; # --conf "+confPath)
	stdin, err := command.StdinPipe()
	if err != nil {
		t.Fatal(err)
	}
	if err := command.Start(); err != nil {
		t.Fatal(err)
	}
	defer stdin.Close()
	if err := os.WriteFile(pidPath, []byte(strconv.Itoa(command.Process.Pid)), 0600); err != nil {
		_ = command.Process.Kill()
		_ = command.Wait()
		t.Fatal(err)
	}
	if err := reapStaleI2PD("/bin/sh", confPath, pidPath); err != nil {
		_ = command.Process.Kill()
		_ = command.Wait()
		t.Fatalf("reapStaleI2PD: %v", err)
	}
	if err := command.Wait(); err == nil {
		t.Fatal("matching stale i2pd process exited without a termination signal")
	}
	if _, err := os.Stat(pidPath); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("stale PID file remains: %v", err)
	}
}

func TestParentDeathSignalTerminatesChild(t *testing.T) {
	const roleVar = "VIDVEIL_I2P_PARENT_DEATH_TEST"
	if os.Getenv(roleVar) == "parent" {
		pidFile := os.Getenv("VIDVEIL_I2P_PARENT_DEATH_PIDFILE")
		child := exec.Command("/bin/sh", "-c", "read -r line")
		stdin, err := child.StdinPipe()
		if err != nil {
			os.Exit(2)
		}
		defer stdin.Close()
		configureI2PDCommand(child)
		if err := child.Start(); err != nil {
			os.Exit(3)
		}
		if err := os.WriteFile(pidFile, []byte(strconv.Itoa(child.Process.Pid)), 0600); err != nil {
			_ = child.Process.Kill()
			os.Exit(4)
		}
		select {}
	}
	root := t.TempDir()
	pidFile := filepath.Join(root, "child.pid")
	parent := exec.Command(os.Args[0], "-test.run=^TestParentDeathSignalTerminatesChild$")
	parent.Env = append(os.Environ(), roleVar+"=parent", "VIDVEIL_I2P_PARENT_DEATH_PIDFILE="+pidFile)
	if err := parent.Start(); err != nil {
		t.Fatalf("start parent helper: %v", err)
	}
	deadline := time.Now().Add(5 * time.Second)
	var childPID int
	for time.Now().Before(deadline) {
		data, err := os.ReadFile(pidFile)
		if err == nil {
			childPID, _ = strconv.Atoi(string(data))
			if childPID > 1 {
				break
			}
		}
		time.Sleep(20 * time.Millisecond)
	}
	if childPID <= 1 {
		_ = parent.Process.Kill()
		_ = parent.Wait()
		t.Fatal("parent helper did not publish child PID")
	}
	if err := parent.Process.Kill(); err != nil {
		t.Fatalf("kill parent helper: %v", err)
	}
	_ = parent.Wait()
	childPath := filepath.Join("/proc", strconv.Itoa(childPID))
	deadline = time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		stat, err := os.ReadFile(filepath.Join(childPath, "stat"))
		if errors.Is(err, os.ErrNotExist) || (err == nil && procState(string(stat)) == 'Z') {
			return
		}
		time.Sleep(50 * time.Millisecond)
	}
	if process, err := os.FindProcess(childPID); err == nil {
		_ = process.Kill()
	}
	t.Fatalf("i2pd child process %d survived its parent's SIGKILL", childPID)
}
