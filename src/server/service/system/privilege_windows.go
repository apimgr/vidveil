// SPDX-License-Identifier: MIT
// AI.md PART 25: Privilege Dropping (Windows)
//go:build windows

package system

// DropPrivileges is a no-op on Windows per AI.md PART 25
// Windows uses Virtual Service Account (NT SERVICE\vidveil) which is already minimal-privilege
func DropPrivileges(username string) error {
	// Windows: No privilege dropping needed
	// Virtual Service Account (VSA) is already a minimal-privilege isolated account
	return nil
}

// ShouldDropPrivileges returns false on Windows - VSA handles this
func ShouldDropPrivileges() bool {
	return false
}

// GetPrivilegeDropUser returns empty on Windows - uses VSA
func GetPrivilegeDropUser() string {
	return ""
}
