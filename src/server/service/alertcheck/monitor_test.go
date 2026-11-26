// SPDX-License-Identifier: MIT
package alertcheck

import (
	"errors"
	"testing"
	"time"
)

func TestDatabaseMonitorThresholdRepeatAndRecovery(t *testing.T) {
	monitor := &DatabaseMonitor{}
	start := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	probeErr := errors.New("connection failed")
	if event := monitor.Observe(probeErr, start, 2, time.Hour); event.Alert {
		t.Fatal("database alert fired before failure threshold")
	}
	event := monitor.Observe(probeErr, start.Add(time.Minute), 2, time.Hour)
	if !event.Alert || !event.Since.Equal(start) || event.Error != probeErr {
		t.Fatalf("threshold event = %+v", event)
	}
	if event := monitor.Observe(probeErr, start.Add(2*time.Minute), 2, time.Hour); event.Alert {
		t.Fatal("database alert repeated before repeat interval")
	}
	if event := monitor.Observe(probeErr, start.Add(time.Hour+time.Minute), 2, time.Hour); !event.Alert {
		t.Fatal("database alert did not repeat after repeat interval")
	}
	if event := monitor.Observe(nil, start.Add(2*time.Hour), 2, time.Hour); !event.Recovered {
		t.Fatal("database recovery event was not emitted")
	}
	if event := monitor.Observe(probeErr, start.Add(3*time.Hour), 2, time.Hour); event.Alert {
		t.Fatal("database alert fired before the new failure threshold")
	}
}

func TestDiskMonitorDeduplicatesAndUsesRecoveryHysteresis(t *testing.T) {
	monitor := &DiskMonitor{}
	now := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	if event := monitor.Observe("dev1", "/data", 85, 100, 85, time.Hour, now); !event.Alert {
		t.Fatal("disk alert did not fire at threshold")
	}
	if event := monitor.Observe("dev1", "/logs", 90, 50, 85, time.Hour, now.Add(time.Minute)); event.Alert {
		t.Fatal("same volume generated duplicate alert")
	}
	if event := monitor.Observe("dev2", "/backup", 90, 50, 85, time.Hour, now.Add(time.Minute)); !event.Alert {
		t.Fatal("separate volume did not generate its alert")
	}
	if event := monitor.Observe("dev1", "/data", 81, 100, 85, time.Hour, now.Add(2*time.Minute)); event.Recovered {
		t.Fatal("disk recovered before falling five percentage points below threshold")
	}
	if event := monitor.Observe("dev1", "/data", 80, 100, 85, time.Hour, now.Add(3*time.Minute)); !event.Recovered {
		t.Fatal("disk recovery event was not emitted at hysteresis threshold")
	}
	if event := monitor.Observe("dev1", "/data", 90, 50, 85, time.Hour, now.Add(4*time.Minute)); !event.Alert {
		t.Fatal("disk alert did not re-arm after recovery")
	}
}
