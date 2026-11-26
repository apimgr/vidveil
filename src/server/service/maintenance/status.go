// SPDX-License-Identifier: MIT
package maintenance

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"time"
)

const MaintenanceRetryInterval = 30 * time.Second

type SelfHealingStatus struct {
	Enabled     bool       `json:"enabled"`
	Exhausted   bool       `json:"exhausted"`
	Attempts    uint64     `json:"attempts"`
	LastAttempt *time.Time `json:"last_attempt,omitempty"`
	NextAttempt *time.Time `json:"next_attempt,omitempty"`
}

type MaintenanceStatus struct {
	Reason      string            `json:"reason"`
	Message     string            `json:"message"`
	Diagnostic  string            `json:"diagnostic,omitempty"`
	Since       time.Time         `json:"since"`
	SelfHealing SelfHealingStatus `json:"self_healing"`
}

// SelfHealingAttemptStatus returns the externally reported state after a
// failed recovery check. A zero limit retries forever; a reached positive
// limit disables further retries and removes the next-attempt timestamp.
func SelfHealingAttemptStatus(attempts, maxAttempts uint64, lastAttempt, nextAttempt time.Time) SelfHealingStatus {
	exhausted := maxAttempts > 0 && attempts >= maxAttempts
	status := SelfHealingStatus{
		Enabled: !exhausted, Exhausted: exhausted, Attempts: attempts,
		LastAttempt: &lastAttempt,
	}
	if !exhausted {
		status.NextAttempt = &nextAttempt
	}
	return status
}

func maintenanceStatusPath(dataDir string) string {
	return filepath.Join(dataDir, "maintenance.status.json")
}

func WriteMaintenanceStatus(dataDir string, status MaintenanceStatus) (retErr error) {
	data, err := json.Marshal(status)
	if err != nil {
		return err
	}
	temp, err := os.CreateTemp(dataDir, ".maintenance-status-*")
	if err != nil {
		return err
	}
	tempPath := temp.Name()
	defer func() {
		if err := os.Remove(tempPath); err != nil && !errors.Is(err, os.ErrNotExist) && retErr == nil {
			retErr = err
		}
	}()
	if err := temp.Chmod(0600); err != nil {
		_ = temp.Close()
		return err
	}
	if _, err := temp.Write(append(data, '\n')); err != nil {
		_ = temp.Close()
		return err
	}
	if err := temp.Sync(); err != nil {
		_ = temp.Close()
		return err
	}
	if err := temp.Close(); err != nil {
		return err
	}
	return os.Rename(tempPath, maintenanceStatusPath(dataDir))
}

func ClearMaintenanceStatus(dataDir string) error {
	err := os.Remove(maintenanceStatusPath(dataDir))
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	return err
}

func ReadMaintenanceStatus(dataDir string) (*MaintenanceStatus, bool) {
	flagPath := filepath.Join(dataDir, "maintenance.flag")
	flagData, err := os.ReadFile(flagPath)
	if err != nil {
		return nil, false
	}
	var status MaintenanceStatus
	if data, err := os.ReadFile(maintenanceStatusPath(dataDir)); err == nil && json.Unmarshal(data, &status) == nil {
		if status.Reason == "database_connection" || status.Reason == "file_write" {
			if status.SelfHealing.Exhausted {
				status.Message = "Self-healing retry limit reached; correct the cause and restart the service."
			} else if status.Reason == "database_connection" {
				status.Message = "The server cannot connect to its database. It is read-only while recovery is attempted."
			} else {
				status.Message = "The server cannot write to its data directory. It is read-only while recovery is attempted."
			}
			if !status.SelfHealing.Exhausted {
				status.SelfHealing.Enabled = true
			}
			if status.Since.IsZero() {
				status.Since = maintenanceFlagTime(flagData)
			}
			return &status, true
		}
	}
	status = MaintenanceStatus{Reason: "manual", Message: "Server is in maintenance mode", Since: maintenanceFlagTime(flagData), SelfHealing: SelfHealingStatus{Enabled: false}}
	return &status, true
}

func maintenanceFlagTime(flagData []byte) time.Time {
	if parsed, err := time.Parse(time.RFC3339, string(flagData)); err == nil {
		return parsed.UTC()
	}
	return time.Now().UTC()
}
