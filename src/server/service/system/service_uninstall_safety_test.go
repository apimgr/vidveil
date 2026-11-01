// SPDX-License-Identifier: MIT
package system

import (
	"os"
	"path/filepath"
	"testing"
)

func TestValidateProjectRemovalPath(t *testing.T) {
	for _, path := range []string{"/", "/home/alice", "/home"} {
		if err := validateProjectRemovalPath(path, "/home/alice"); err == nil {
			t.Errorf("validateProjectRemovalPath(%q) should reject the target", path)
		}
	}
	if err := validateProjectRemovalPath("/var/lib/apimgr/vidveil", "/home/alice"); err != nil {
		t.Errorf("validateProjectRemovalPath(project data) error = %v", err)
	}
}

func TestValidateProjectRemovalPathRejectsSymlinkParent(t *testing.T) {
	base := t.TempDir()
	actual := filepath.Join(base, "actual")
	if err := os.Mkdir(actual, 0755); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(base, "project-link")
	if err := os.Symlink(actual, link); err != nil {
		t.Skipf("symlinks unavailable: %v", err)
	}
	if err := validateProjectRemovalPath(filepath.Join(link, "vidveil"), filepath.Join(base, "home")); err == nil {
		t.Fatal("validateProjectRemovalPath accepted a target through a symlinked parent")
	}
}
