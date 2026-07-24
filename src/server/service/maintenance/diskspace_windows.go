// SPDX-License-Identifier: MIT
// AI.md PART 21: Backup & Restore — disk-space precheck (Windows).
//go:build windows

package maintenance

import "golang.org/x/sys/windows"

// diskSpace returns the total and free bytes for the filesystem containing path.
func diskSpace(path string) (total, free uint64, err error) {
	var freeBytesAvailable, totalBytes, totalFreeBytes uint64
	pathPtr, err := windows.UTF16PtrFromString(path)
	if err != nil {
		return 0, 0, err
	}
	if err := windows.GetDiskFreeSpaceEx(pathPtr, &freeBytesAvailable, &totalBytes, &totalFreeBytes); err != nil {
		return 0, 0, err
	}
	return totalBytes, totalFreeBytes, nil
}
