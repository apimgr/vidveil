// SPDX-License-Identifier: MIT
package i2p

import (
	"errors"
	"fmt"
	"os"
	"strconv"
	"strings"
)

func readI2PPID(pidPath string) (int, bool, error) {
	info, err := os.Lstat(pidPath)
	if errors.Is(err, os.ErrNotExist) {
		return 0, false, nil
	}
	if err != nil {
		return 0, false, fmt.Errorf("inspect I2P pid file: %w", err)
	}
	if !info.Mode().IsRegular() {
		return 0, false, fmt.Errorf("I2P pid file %s is not a regular file", pidPath)
	}
	data, err := os.ReadFile(pidPath)
	if err != nil {
		return 0, false, fmt.Errorf("read I2P pid file: %w", err)
	}
	pid, err := strconv.Atoi(strings.TrimSpace(string(data)))
	if err != nil || pid <= 1 {
		return 0, false, fmt.Errorf("I2P pid file %s contains an invalid process ID", pidPath)
	}
	return pid, true, nil
}
