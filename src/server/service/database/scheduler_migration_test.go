// SPDX-License-Identifier: MIT
package database

import (
	"context"
	"path/filepath"
	"testing"
)

func TestEnsureSchemaRenamesLegacySchedulerColumnsBeforeCreatingIndexes(t *testing.T) {
	manager, err := NewSchemaManager(filepath.Join(t.TempDir(), "server.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = manager.Close() }()

	legacy := manager.db
	for _, statement := range []string{
		`CREATE TABLE scheduler_tasks (
			task_id TEXT PRIMARY KEY, task_name TEXT NOT NULL, enabled INTEGER NOT NULL,
			schedule TEXT NOT NULL, last_run DATETIME, last_result TEXT,
			last_error TEXT, next_run DATETIME, run_count INTEGER, fail_count INTEGER
		)`,
		`CREATE TABLE scheduler_history (
			id INTEGER PRIMARY KEY AUTOINCREMENT, task_id TEXT NOT NULL,
			ran_at DATETIME NOT NULL, finished_at DATETIME, status TEXT NOT NULL,
			error TEXT, duration_ms INTEGER,
			FOREIGN KEY (task_id) REFERENCES scheduler_tasks(task_id)
		)`,
		`INSERT INTO scheduler_tasks (task_id, task_name, enabled, schedule, last_run, last_result, next_run, run_count, fail_count)
			VALUES ('backup_daily', 'Daily Backup', 1, '0 2 * * *', '2026-11-25 02:00:00', 'failure', '2026-11-26 02:00:00', 3, 1)`,
		`INSERT INTO scheduler_history (task_id, ran_at, finished_at, status, error, duration_ms)
			VALUES ('backup_daily', '2026-11-25 02:00:00', '2026-11-25 02:00:05', 'failure', 'disk full', 5000)`,
	} {
		if _, err := legacy.Exec(statement); err != nil {
			t.Fatalf("prepare old scheduler schema: %v", err)
		}
	}

	if err := manager.EnsureSchema(); err != nil {
		t.Fatalf("EnsureSchema: %v", err)
	}

	var id, name, status string
	var lastRun, nextRun int64
	if err := legacy.QueryRow(`SELECT id, name, last_status, last_run, next_run FROM scheduler_tasks WHERE id = 'backup_daily'`).Scan(&id, &name, &status, &lastRun, &nextRun); err != nil {
		t.Fatal(err)
	}
	if id != "backup_daily" || name != "Daily Backup" || status != "failed" || lastRun == 0 || nextRun == 0 {
		t.Fatalf("migrated task = id %q, name %q, status %q, last_run %d, next_run %d", id, name, status, lastRun, nextRun)
	}
	var startedAt, finishedAt int64
	var historyStatus string
	if err := legacy.QueryRow(`SELECT started_at, finished_at, status FROM scheduler_history WHERE task_id = 'backup_daily'`).Scan(&startedAt, &finishedAt, &historyStatus); err != nil {
		t.Fatal(err)
	}
	if startedAt == 0 || finishedAt == 0 || historyStatus != "failed" {
		t.Fatalf("migrated history = started_at %d, finished_at %d, status %q", startedAt, finishedAt, historyStatus)
	}
	for table, staleColumns := range map[string][]string{
		"scheduler_tasks":   {"task_id", "task_name", "last_result"},
		"scheduler_history": {"ran_at"},
	} {
		columns, err := manager.tableColumns(context.Background(), table)
		if err != nil {
			t.Fatal(err)
		}
		for _, column := range staleColumns {
			if columns[column] {
				t.Errorf("legacy column %s.%s remains after migration", table, column)
			}
		}
	}
}
