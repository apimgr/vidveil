// SPDX-License-Identifier: MIT
package maintenance

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"

	projectpath "github.com/apimgr/vidveil/src/paths"
	"github.com/gofrs/flock"
)

func (m *MaintenanceManager) withMaintenanceOperation(operation func() error) (retErr error) {
	maintenanceOperationMu.Lock()
	defer maintenanceOperationMu.Unlock()

	lockDir := projectpath.GetMaintenanceLockDir(m.paths.Data)
	if err := os.MkdirAll(lockDir, 0750); err != nil {
		return fmt.Errorf("prepare maintenance lock directory: %w", err)
	}
	lock := flock.New(maintenanceLockPath(lockDir), flock.SetPermissions(0644))
	if err := lock.Lock(); err != nil {
		return fmt.Errorf("lock maintenance operations: %w", err)
	}
	defer func() {
		if err := lock.Unlock(); err != nil {
			retErr = errors.Join(retErr, fmt.Errorf("unlock maintenance operations: %w", err))
		}
	}()

	return operation()
}

func maintenanceLockPath(backupDir string) string {
	return filepath.Join(backupDir, ".vidveil-maintenance.lock")
}
