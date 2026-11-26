// SPDX-License-Identifier: MIT
//go:build !windows

package logging

import "os"

func replaceAuditAnchor(source, destination string) error {
	return os.Rename(source, destination)
}

func syncAuditDirectory(path string) error {
	directory, err := os.Open(path)
	if err != nil {
		return err
	}
	defer func() { _ = directory.Close() }()
	return directory.Sync()
}
