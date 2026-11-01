// SPDX-License-Identifier: MIT
//go:build !windows

package maintenance

import "os"

func replaceBackupFile(tempPath, backupPath string) error {
	return os.Rename(tempPath, backupPath)
}
