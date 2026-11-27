// SPDX-License-Identifier: MIT
//go:build !windows

package system

import (
	"os/user"
	"testing"
)

func TestValidateExistingServiceAccountRequiresSystemIDs(t *testing.T) {
	account := &user.User{Username: "vidveil"}
	for _, ids := range []struct {
		uid int
		gid int
	}{
		{uid: 0, gid: 1},
		{uid: 1, gid: 0},
		{uid: 1000, gid: 1},
		{uid: 1, gid: 1000},
	} {
		if err := validateExistingServiceAccount(account, ids.uid, ids.gid); err == nil {
			t.Errorf("validateExistingServiceAccount(%d, %d) error = nil, want range error", ids.uid, ids.gid)
		}
	}
}

func TestIsNonLoginShell(t *testing.T) {
	for _, shell := range []string{"/sbin/nologin", "/usr/sbin/nologin", "/bin/false"} {
		if !isNonLoginShell(shell) {
			t.Errorf("isNonLoginShell(%q) = false, want true", shell)
		}
	}
	if isNonLoginShell("/bin/bash") {
		t.Error("isNonLoginShell(/bin/bash) = true, want false")
	}
}
