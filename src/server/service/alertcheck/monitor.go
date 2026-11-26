// SPDX-License-Identifier: MIT
package alertcheck

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"sync"
	"time"

	"github.com/apimgr/vidveil/src/server/service/maintenance"
)

// DatabaseEvent describes a threshold crossing or recovery.
type DatabaseEvent struct {
	Alert     bool
	Recovered bool
	Since     time.Time
	Error     error
}

// DatabaseMonitor tracks consecutive failures and notification repeat timing.
type DatabaseMonitor struct {
	mu           sync.Mutex
	failures     int
	firstFailure time.Time
	lastNotified time.Time
	active       bool
}

// Observe updates the database trigger state and reports whether to notify.
func (m *DatabaseMonitor) Observe(err error, now time.Time, threshold int, repeat time.Duration) DatabaseEvent {
	m.mu.Lock()
	defer m.mu.Unlock()
	if threshold < 1 {
		threshold = 3
	}
	if err == nil {
		event := DatabaseEvent{Recovered: m.active}
		m.failures, m.firstFailure, m.lastNotified, m.active = 0, time.Time{}, time.Time{}, false
		return event
	}
	if m.failures == 0 {
		m.firstFailure = now
	}
	m.failures++
	if m.failures < threshold {
		return DatabaseEvent{}
	}
	if repeat <= 0 {
		repeat = 24 * time.Hour
	}
	if m.lastNotified.IsZero() || !now.Before(m.lastNotified.Add(repeat)) {
		m.active = true
		m.lastNotified = now
		return DatabaseEvent{Alert: true, Since: m.firstFailure, Error: err}
	}
	return DatabaseEvent{}
}

// DiskEvent describes a low-space threshold crossing or recovery.
type DiskEvent struct {
	Path      string
	Used      float64
	FreeBytes uint64
	Alert     bool
	Recovered bool
}

type diskState struct {
	active       bool
	lastNotified time.Time
}

// DiskMonitor tracks notification state separately for each filesystem.
type DiskMonitor struct {
	mu      sync.Mutex
	volumes map[string]diskState
}

// Observe applies the threshold and five-point recovery hysteresis to a volume.
func (m *DiskMonitor) Observe(volumeID, path string, used float64, free uint64, threshold int, repeat time.Duration, now time.Time) DiskEvent {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.volumes == nil {
		m.volumes = make(map[string]diskState)
	}
	if threshold <= 0 || threshold > 100 {
		threshold = 85
	}
	if repeat <= 0 {
		repeat = 24 * time.Hour
	}
	state := m.volumes[volumeID]
	event := DiskEvent{Path: path, Used: used, FreeBytes: free}
	if used >= float64(threshold) {
		if !state.active || state.lastNotified.IsZero() || !now.Before(state.lastNotified.Add(repeat)) {
			event.Alert = true
			state.lastNotified = now
		}
		state.active = true
	} else if state.active && used <= float64(threshold-5) {
		event.Recovered = true
		state = diskState{}
	}
	m.volumes[volumeID] = state
	return event
}

// ProbeDatabase runs a bounded availability probe against the configured DB handle.
func ProbeDatabase(ctx context.Context, db *sql.DB) error {
	if db == nil {
		return fmt.Errorf("database handle is unavailable")
	}
	probeCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	if err := db.PingContext(probeCtx); err != nil {
		return err
	}
	var one int
	if err := db.QueryRowContext(probeCtx, "SELECT 1").Scan(&one); err != nil {
		return err
	}
	return nil
}

// ProbeDisk reads filesystem usage for a path.
func ProbeDisk(path string) (float64, uint64, string, error) {
	total, free, err := maintenance.DiskSpace(path)
	if err != nil {
		return 0, 0, "", err
	}
	if total == 0 {
		return 0, free, "", fmt.Errorf("filesystem size is zero for %s", path)
	}
	used := float64(total-free) / float64(total) * 100
	info, err := os.Stat(path)
	if err != nil {
		return 0, 0, "", err
	}
	volume := filepath.VolumeName(path)
	if info.Sys() != nil {
		sys := reflect.ValueOf(info.Sys())
		if sys.Kind() == reflect.Pointer {
			sys = sys.Elem()
		}
		dev := sys.FieldByName("Dev")
		if dev.IsValid() && dev.CanInterface() {
			volume = fmt.Sprintf("device:%v", dev.Interface())
		}
	}
	if volume == "" {
		volume = filepath.Clean(path)
	}
	return used, free, volume, nil
}
