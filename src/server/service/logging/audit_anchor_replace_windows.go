// SPDX-License-Identifier: MIT
//go:build windows

package logging

import "golang.org/x/sys/windows"

func replaceAuditAnchor(source, destination string) error {
	from, err := windows.UTF16PtrFromString(source)
	if err != nil {
		return err
	}
	to, err := windows.UTF16PtrFromString(destination)
	if err != nil {
		return err
	}
	return windows.MoveFileEx(from, to, windows.MOVEFILE_REPLACE_EXISTING|windows.MOVEFILE_WRITE_THROUGH)
}

func syncAuditDirectory(string) error {
	return nil
}
