// SPDX-License-Identifier: MIT
//go:build !windows

package paths

import (
	"fmt"
	"os"
	"syscall"
)

// EnsurePathOwnership verifies that a path is owned by the current user.
func EnsurePathOwnership(path string) error {
	fileInfo, err := os.Stat(path)
	if err != nil {
		return fmt.Errorf("stat %s: %w", path, err)
	}

	statData, ok := fileInfo.Sys().(*syscall.Stat_t)
	if !ok {
		return fmt.Errorf("read ownership for %s", path)
	}

	currentUID := uint32(os.Getuid())
	currentGID := uint32(os.Getgid())
	if statData.Uid != currentUID || statData.Gid != currentGID {
		return fmt.Errorf(
			"path %s owned by uid=%d gid=%d, want uid=%d gid=%d",
			path,
			statData.Uid,
			statData.Gid,
			currentUID,
			currentGID,
		)
	}

	return nil
}
