// SPDX-License-Identifier: MIT
//go:build windows

package paths

// EnsurePathOwnership is a no-op on Windows because user-directory ACLs are inherited.
func EnsurePathOwnership(path string) error {
	return nil
}
