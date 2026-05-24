// SPDX-License-Identifier: MIT
// AI.md PART 8: Directory creation and validation

package paths

import (
	"fmt"
	"os"
	"path/filepath"
)

// EnsureDir creates a directory with proper permissions if it does not exist,
// then verifies the directory is writable. Per AI.md PART 8:
//   - root: 0755 permissions
//   - user: 0700 permissions
func EnsureDir(path string, isRoot bool) error {
	perm := os.FileMode(0700)
	if isRoot {
		perm = 0755
	}

	if err := os.MkdirAll(path, perm); err != nil {
		return fmt.Errorf("failed to create directory %s: %w", path, err)
	}

	// Verify writable
	testFile := filepath.Join(path, ".write-test")
	if err := os.WriteFile(testFile, []byte{}, 0600); err != nil {
		return fmt.Errorf("directory %s is not writable: %w", path, err)
	}
	os.Remove(testFile)

	return nil
}

// EnsurePIDFile creates the directory for a PID file and validates the path.
// Per AI.md PART 8: PID file directory uses same permissions as EnsureDir.
func EnsurePIDFile(path string, isRoot bool) error {
	dir := filepath.Dir(path)
	return EnsureDir(dir, isRoot)
}
