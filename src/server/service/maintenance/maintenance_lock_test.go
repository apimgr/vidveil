// SPDX-License-Identifier: MIT
package maintenance

import (
	"testing"

	projectpath "github.com/apimgr/vidveil/src/path"
	"github.com/gofrs/flock"
)

func TestMaintenanceLockExcludesOtherProcesses(t *testing.T) {
	backupDir := t.TempDir()
	first := flock.New(maintenanceLockPath(backupDir), flock.SetPermissions(0644))
	if err := first.Lock(); err != nil {
		t.Fatalf("first lock: %v", err)
	}
	firstLocked := true
	defer func() {
		if firstLocked {
			_ = first.Unlock()
		}
	}()

	second := flock.New(maintenanceLockPath(backupDir), flock.SetPermissions(0644))
	locked, err := second.TryLock()
	if err != nil {
		t.Fatalf("try second lock: %v", err)
	}
	if locked {
		_ = second.Unlock()
		t.Fatal("second maintenance lock acquired while first lock was held")
	}

	if err := first.Unlock(); err != nil {
		t.Fatalf("release first lock: %v", err)
	}
	firstLocked = false
	locked, err = second.TryLock()
	if err != nil {
		t.Fatalf("retry second lock: %v", err)
	}
	if !locked {
		t.Fatal("second maintenance lock was not acquired after first lock released")
	}
	if err := second.Unlock(); err != nil {
		t.Fatalf("release second lock: %v", err)
	}
}

func TestWithMaintenanceOperationAcquiresAndReleasesLock(t *testing.T) {
	manager := NewMaintenanceManager(t.TempDir(), t.TempDir(), "test")
	manager.paths.Backup = t.TempDir()
	probe := flock.New(maintenanceLockPath(projectpath.GetMaintenanceLockDir(manager.paths.Data)), flock.SetPermissions(0644))
	called := false
	if err := manager.withMaintenanceOperation(func() error {
		called = true
		locked, err := probe.TryLock()
		if err != nil {
			return err
		}
		if locked {
			_ = probe.Unlock()
			t.Fatal("independent lock acquired while operation lock was held")
		}
		return nil
	}); err != nil {
		t.Fatalf("withMaintenanceOperation: %v", err)
	}
	if !called {
		t.Fatal("maintenance operation was not called")
	}
	locked, err := probe.TryLock()
	if err != nil {
		t.Fatalf("try lock after operation: %v", err)
	}
	if !locked {
		t.Fatal("maintenance lock remained held after operation returned")
	}
	if err := probe.Unlock(); err != nil {
		t.Fatalf("release probe lock: %v", err)
	}
}
