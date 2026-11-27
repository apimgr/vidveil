// SPDX-License-Identifier: MIT
//go:build windows

package maintenance

import "fmt"

func replaceRunningBinary(currentPath, stagedPath string, data []byte) error {
	return fmt.Errorf("automatic staged update is unsupported for a Windows service account")
}

func restartUpdatedExecutable(path string) error {
	return fmt.Errorf("automatic staged update is unsupported on Windows")
}
