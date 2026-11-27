// SPDX-License-Identifier: MIT
//go:build !windows

package maintenance

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"syscall"
)

func replaceRunningBinary(currentPath, stagedPath string, data []byte) (retErr error) {
	info, err := os.Stat(currentPath)
	if err != nil {
		return fmt.Errorf("inspect installed executable: %w", err)
	}
	tmp, err := os.CreateTemp(filepath.Dir(currentPath), ".vidveil-update-*")
	if err != nil {
		return fmt.Errorf("create executable replacement: %w", err)
	}
	tmpPath := tmp.Name()
	defer func() {
		if err := os.Remove(tmpPath); err != nil && !errors.Is(err, os.ErrNotExist) {
			retErr = errors.Join(retErr, fmt.Errorf("remove executable replacement temporary file: %w", err))
		}
	}()
	if err := tmp.Chmod(info.Mode().Perm()); err != nil {
		return fmt.Errorf("set replacement executable permissions: %w", errors.Join(err, tmp.Close()))
	}
	if _, err := tmp.Write(data); err != nil {
		return fmt.Errorf("write replacement executable: %w", errors.Join(err, tmp.Close()))
	}
	if err := tmp.Sync(); err != nil {
		return fmt.Errorf("sync replacement executable: %w", errors.Join(err, tmp.Close()))
	}
	if err := tmp.Close(); err != nil {
		return fmt.Errorf("close replacement executable: %w", err)
	}
	if err := os.Rename(tmpPath, currentPath); err != nil {
		return fmt.Errorf("atomically replace executable: %w", err)
	}
	return nil
}

func restartUpdatedExecutable(path string) error {
	return syscall.Exec(path, os.Args, os.Environ())
}
