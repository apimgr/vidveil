// SPDX-License-Identifier: MIT
// AI.md PART 21: Backup & Restore — disk-space precheck (Unix).
//go:build linux || darwin || freebsd

package maintenance

import "golang.org/x/sys/unix"

// diskSpace returns the total and free bytes for the filesystem containing path.
func diskSpace(path string) (total, free uint64, err error) {
	var stat unix.Statfs_t
	if err := unix.Statfs(path, &stat); err != nil {
		return 0, 0, err
	}
	total = uint64(stat.Blocks) * uint64(stat.Bsize)
	free = uint64(stat.Bfree) * uint64(stat.Bsize)
	return total, free, nil
}
