// SPDX-License-Identifier: MIT
//go:build windows

package maintenance

import "golang.org/x/sys/windows"

func replaceBackupFile(tempPath, backupPath string) error {
	temp, err := windows.UTF16PtrFromString(tempPath)
	if err != nil {
		return err
	}
	backup, err := windows.UTF16PtrFromString(backupPath)
	if err != nil {
		return err
	}
	return windows.MoveFileEx(temp, backup, windows.MOVEFILE_REPLACE_EXISTING|windows.MOVEFILE_WRITE_THROUGH)
}
