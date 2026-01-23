// SPDX-License-Identifier: MIT
// AI.md PART 19: Scheduler with Database Persistence
package scheduler

import (
	"context"
	"database/sql"
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/robfig/cron/v3"
)

// TaskFunc is a function that executes a scheduled task
type TaskFunc func(ctx context.Context) error

// ScheduledTask represents a scheduled task
type ScheduledTask struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description"`
	// Schedule format: hourly, daily, weekly, monthly, or cron expression (e.g., "0 2 * * *")
	Schedule   string    `json:"schedule"`
	Enabled    bool      `json:"enabled"`
	LastRun    time.Time `json:"last_run"`
	// LastResult: success, failure, running, or pending
	LastResult string    `json:"last_result"`
	LastError  string    `json:"last_error,omitempty"`
	NextRun    time.Time `json:"next_run"`
	RunCount   int64     `json:"run_count"`
	FailCount  int64     `json:"fail_count"`
	// Interval is for simple duration-based schedules
	Interval time.Duration `json:"-"`
	// cronSched is for cron-expression schedules per AI.md PART 22
	cronSched cron.Schedule `json:"-"`
	fn        TaskFunc
}

// TaskHistory represents a historical run of a task
type TaskHistory struct {
	TaskID    string        `json:"task_id"`
	StartTime time.Time     `json:"start_time"`
	EndTime   time.Time     `json:"end_time"`
	Duration  time.Duration `json:"duration"`
	Result    string        `json:"result"`
	Error     string        `json:"error,omitempty"`
}

// Scheduler manages scheduled tasks per AI.md PART 19
// Supports optional database persistence for task state survival across restarts
type Scheduler struct {
	tasks    map[string]*ScheduledTask
	history  []TaskHistory
	mu       sync.RWMutex
	ctx      context.Context
	cancel   context.CancelFunc
	running  bool
	maxHist  int
	db       *sql.DB // Optional database for persistence per AI.md PART 19
}

// NewScheduler creates a new scheduler without database persistence
func NewScheduler() *Scheduler {
	return &Scheduler{
		tasks:   make(map[string]*ScheduledTask),
		history: make([]TaskHistory, 0),
		// Keep last 100 history entries in memory
		maxHist: 100,
	}
}

// NewSchedulerWithDB creates a new scheduler with database persistence per AI.md PART 19
// Task state survives restarts when db is provided
func NewSchedulerWithDB(db *sql.DB) *Scheduler {
	return &Scheduler{
		tasks:   make(map[string]*ScheduledTask),
		history: make([]TaskHistory, 0),
		maxHist: 100,
		db:      db,
	}
}

// SetDB sets the database connection for persistence
// Can be called after NewScheduler() to enable persistence
func (s *Scheduler) SetDB(db *sql.DB) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.db = db
}

// loadTaskStateFromDB loads persisted task state from database
// Called during RegisterTask to restore run_count, fail_count, last_run, etc.
func (s *Scheduler) loadTaskStateFromDB(taskID string) (*ScheduledTask, error) {
	if s.db == nil {
		return nil, nil
	}

	row := s.db.QueryRow(`
		SELECT id, name, schedule, enabled, last_run, next_run,
		       last_result, last_error, run_count, fail_count
		FROM scheduled_tasks WHERE id = ?`, taskID)

	var task ScheduledTask
	var lastRun, nextRun sql.NullTime
	var lastResult, lastError sql.NullString

	err := row.Scan(
		&task.ID, &task.Name, &task.Schedule, &task.Enabled,
		&lastRun, &nextRun, &lastResult, &lastError,
		&task.RunCount, &task.FailCount,
	)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("failed to load task state: %w", err)
	}

	if lastRun.Valid {
		task.LastRun = lastRun.Time
	}
	if nextRun.Valid {
		task.NextRun = nextRun.Time
	}
	if lastResult.Valid {
		task.LastResult = lastResult.String
	}
	if lastError.Valid {
		task.LastError = lastError.String
	}

	return &task, nil
}

// saveTaskStateToDB persists task state to database
func (s *Scheduler) saveTaskStateToDB(task *ScheduledTask) error {
	if s.db == nil {
		return nil
	}

	_, err := s.db.Exec(`
		INSERT INTO scheduled_tasks (id, name, schedule, enabled, last_run, next_run,
		                             last_result, last_error, run_count, fail_count)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(id) DO UPDATE SET
			name = excluded.name,
			schedule = excluded.schedule,
			enabled = excluded.enabled,
			last_run = excluded.last_run,
			next_run = excluded.next_run,
			last_result = excluded.last_result,
			last_error = excluded.last_error,
			run_count = excluded.run_count,
			fail_count = excluded.fail_count`,
		task.ID, task.Name, task.Schedule, task.Enabled,
		task.LastRun, task.NextRun, task.LastResult, task.LastError,
		task.RunCount, task.FailCount,
	)
	return err
}

// saveHistoryToDB persists task history entry to database
func (s *Scheduler) saveHistoryToDB(hist TaskHistory) error {
	if s.db == nil {
		return nil
	}

	_, err := s.db.Exec(`
		INSERT INTO task_history (task_id, start_time, end_time, duration_ms, result, error)
		VALUES (?, ?, ?, ?, ?, ?)`,
		hist.TaskID, hist.StartTime, hist.EndTime,
		hist.Duration.Milliseconds(), hist.Result, hist.Error,
	)
	return err
}

// LoadHistoryFromDB loads recent task history from database
func (s *Scheduler) LoadHistoryFromDB(limit int) error {
	if s.db == nil {
		return nil
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	rows, err := s.db.Query(`
		SELECT task_id, start_time, end_time, duration_ms, result, error
		FROM task_history
		ORDER BY start_time DESC
		LIMIT ?`, limit)
	if err != nil {
		return fmt.Errorf("failed to load history: %w", err)
	}
	defer rows.Close()

	var history []TaskHistory
	for rows.Next() {
		var h TaskHistory
		var durationMs int64
		var errStr sql.NullString

		if err := rows.Scan(&h.TaskID, &h.StartTime, &h.EndTime, &durationMs, &h.Result, &errStr); err != nil {
			return fmt.Errorf("failed to scan history row: %w", err)
		}

		h.Duration = time.Duration(durationMs) * time.Millisecond
		if errStr.Valid {
			h.Error = errStr.String
		}
		history = append(history, h)
	}

	// Reverse to get chronological order (oldest first)
	for i, j := 0, len(history)-1; i < j; i, j = i+1, j-1 {
		history[i], history[j] = history[j], history[i]
	}

	s.history = history
	return nil
}

// RegisterTask registers a new scheduled task
// Per AI.md PART 19/22: Supports cron expressions or simple intervals
// Persisted state (run_count, fail_count, last_run) is restored from database
func (s *Scheduler) RegisterTask(id, name, description, schedule string, fn TaskFunc) error {
	// Load existing state from DB before acquiring lock (db operations are thread-safe)
	existingState, _ := s.loadTaskStateFromDB(id)

	s.mu.Lock()
	defer s.mu.Unlock()

	task := &ScheduledTask{
		ID:          id,
		Name:        name,
		Description: description,
		Schedule:    schedule,
		Enabled:     true,
		LastResult:  "pending",
		fn:          fn,
	}

	// Try parsing as cron expression first (per AI.md PART 22)
	if cronSched, err := parseCronSchedule(schedule); err == nil {
		task.cronSched = cronSched
		task.NextRun = cronSched.Next(time.Now())
	} else {
		// Fall back to simple interval
		interval, err := parseInterval(schedule)
		if err != nil {
			return fmt.Errorf("invalid schedule '%s': %w", schedule, err)
		}
		task.Interval = interval
		task.NextRun = time.Now().Add(interval)
	}

	// Merge persisted state from database per AI.md PART 19
	// This ensures run_count, fail_count, last_run survive restarts
	if existingState != nil {
		task.RunCount = existingState.RunCount
		task.FailCount = existingState.FailCount
		task.LastRun = existingState.LastRun
		task.LastResult = existingState.LastResult
		task.LastError = existingState.LastError
		// Use persisted enabled state only if task has been run before
		if existingState.RunCount > 0 {
			task.Enabled = existingState.Enabled
		}
		// Calculate proper next run based on last run if available
		if !existingState.LastRun.IsZero() {
			if task.cronSched != nil {
				nextFromLast := task.cronSched.Next(existingState.LastRun)
				// If next run from last run is in the past, calculate from now
				if nextFromLast.Before(time.Now()) {
					task.NextRun = task.cronSched.Next(time.Now())
				} else {
					task.NextRun = nextFromLast
				}
			} else {
				nextFromLast := existingState.LastRun.Add(task.Interval)
				// If next run from last run is in the past, calculate from now
				if nextFromLast.Before(time.Now()) {
					task.NextRun = time.Now().Add(task.Interval)
				} else {
					task.NextRun = nextFromLast
				}
			}
		}
	}

	s.tasks[id] = task

	// Persist initial state to database
	s.saveTaskStateToDB(task)

	return nil
}

// parseCronSchedule parses a cron expression per AI.md PART 22
// Supports standard cron format: minute hour day-of-month month day-of-week
func parseCronSchedule(schedule string) (cron.Schedule, error) {
	// Check if it looks like a cron expression (5 space-separated fields)
	fields := strings.Fields(schedule)
	if len(fields) != 5 {
		return nil, fmt.Errorf("not a cron expression")
	}

	// Use standard cron parser (minute hour dom month dow)
	parser := cron.NewParser(cron.Minute | cron.Hour | cron.Dom | cron.Month | cron.Dow)
	return parser.Parse(schedule)
}

// parseInterval converts schedule string to duration per AI.md
func parseInterval(schedule string) (time.Duration, error) {
	switch schedule {
	case "hourly":
		return time.Hour, nil
	case "daily":
		return 24 * time.Hour, nil
	case "weekly":
		return 7 * 24 * time.Hour, nil
	case "monthly":
		return 30 * 24 * time.Hour, nil
	// For testing purposes
	case "minutely":
		return time.Minute, nil
	default:
		// Try parsing as duration
		d, err := time.ParseDuration(schedule)
		if err != nil {
			return 0, fmt.Errorf("unknown schedule: %s", schedule)
		}
		return d, nil
	}
}

// Start starts the scheduler
func (s *Scheduler) Start(ctx context.Context) {
	s.mu.Lock()
	if s.running {
		s.mu.Unlock()
		return
	}
	s.ctx, s.cancel = context.WithCancel(ctx)
	s.running = true
	s.mu.Unlock()

	go s.run()
}

// Stop stops the scheduler
func (s *Scheduler) Stop() {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.cancel != nil {
		s.cancel()
	}
	s.running = false
}

// run is the main scheduler loop
func (s *Scheduler) run() {
	ticker := time.NewTicker(time.Minute)
	defer ticker.Stop()

	for {
		select {
		case <-s.ctx.Done():
			return
		case <-ticker.C:
			s.checkAndRunTasks()
		}
	}
}

// checkAndRunTasks checks all tasks and runs any that are due
func (s *Scheduler) checkAndRunTasks() {
	s.mu.RLock()
	tasks := make([]*ScheduledTask, 0, len(s.tasks))
	for _, task := range s.tasks {
		tasks = append(tasks, task)
	}
	s.mu.RUnlock()

	now := time.Now()
	for _, task := range tasks {
		if task.Enabled && now.After(task.NextRun) {
			go s.runTask(task)
		}
	}
}

// runTask executes a single task
// Per AI.md PART 19: Task state is persisted to database after each run
func (s *Scheduler) runTask(task *ScheduledTask) {
	s.mu.Lock()
	task.LastResult = "running"
	startTime := time.Now()
	s.mu.Unlock()

	// Create task context with timeout
	ctx, cancel := context.WithTimeout(s.ctx, 5*time.Minute)
	defer cancel()

	err := task.fn(ctx)

	s.mu.Lock()
	endTime := time.Now()
	duration := endTime.Sub(startTime)

	task.LastRun = startTime
	// Calculate next run: use cron schedule if available, else use interval
	if task.cronSched != nil {
		task.NextRun = task.cronSched.Next(startTime)
	} else {
		task.NextRun = startTime.Add(task.Interval)
	}
	task.RunCount++

	hist := TaskHistory{
		TaskID:    task.ID,
		StartTime: startTime,
		EndTime:   endTime,
		Duration:  duration,
	}

	if err != nil {
		task.LastResult = "failure"
		task.LastError = err.Error()
		task.FailCount++
		hist.Result = "failure"
		hist.Error = err.Error()
	} else {
		task.LastResult = "success"
		task.LastError = ""
		hist.Result = "success"
	}

	// Add to in-memory history
	s.history = append(s.history, hist)
	// Trim in-memory history if needed
	if len(s.history) > s.maxHist {
		s.history = s.history[len(s.history)-s.maxHist:]
	}

	// Make a copy of task for DB operations outside lock
	taskCopy := *task
	s.mu.Unlock()

	// Persist state to database per AI.md PART 19
	// Done outside lock to avoid blocking other operations
	s.saveTaskStateToDB(&taskCopy)
	s.saveHistoryToDB(hist)
}

// RunTaskNow manually triggers a task
func (s *Scheduler) RunTaskNow(taskID string) error {
	s.mu.RLock()
	task, ok := s.tasks[taskID]
	s.mu.RUnlock()

	if !ok {
		return fmt.Errorf("task not found: %s", taskID)
	}

	go s.runTask(task)
	return nil
}

// EnableTask enables a task
// Per AI.md PART 19: Enabled state is persisted to database
func (s *Scheduler) EnableTask(taskID string) error {
	s.mu.Lock()
	task, ok := s.tasks[taskID]
	if !ok {
		s.mu.Unlock()
		return fmt.Errorf("task not found: %s", taskID)
	}

	task.Enabled = true
	if task.NextRun.Before(time.Now()) {
		// Calculate next run: use cron schedule if available, else use interval
		if task.cronSched != nil {
			task.NextRun = task.cronSched.Next(time.Now())
		} else {
			task.NextRun = time.Now().Add(task.Interval)
		}
	}
	taskCopy := *task
	s.mu.Unlock()

	// Persist to database
	s.saveTaskStateToDB(&taskCopy)
	return nil
}

// DisableTask disables a task
// Per AI.md PART 19: Enabled state is persisted to database
func (s *Scheduler) DisableTask(taskID string) error {
	s.mu.Lock()
	task, ok := s.tasks[taskID]
	if !ok {
		s.mu.Unlock()
		return fmt.Errorf("task not found: %s", taskID)
	}

	task.Enabled = false
	taskCopy := *task
	s.mu.Unlock()

	// Persist to database
	s.saveTaskStateToDB(&taskCopy)
	return nil
}

// SetSchedule updates a task's schedule
// Per AI.md PART 19/22: Schedule changes are persisted to database
func (s *Scheduler) SetSchedule(taskID, schedule string) error {
	s.mu.Lock()
	task, ok := s.tasks[taskID]
	if !ok {
		s.mu.Unlock()
		return fmt.Errorf("task not found: %s", taskID)
	}

	// Try parsing as cron expression first
	if cronSched, err := parseCronSchedule(schedule); err == nil {
		task.cronSched = cronSched
		task.Interval = 0
		task.NextRun = cronSched.Next(time.Now())
	} else {
		// Fall back to simple interval
		interval, err := parseInterval(schedule)
		if err != nil {
			s.mu.Unlock()
			return fmt.Errorf("invalid schedule '%s': %w", schedule, err)
		}
		task.cronSched = nil
		task.Interval = interval
		task.NextRun = time.Now().Add(interval)
	}

	task.Schedule = schedule
	taskCopy := *task
	s.mu.Unlock()

	// Persist to database
	s.saveTaskStateToDB(&taskCopy)
	return nil
}

// GetTask returns a task by ID
func (s *Scheduler) GetTask(taskID string) (*ScheduledTask, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	task, ok := s.tasks[taskID]
	if !ok {
		return nil, fmt.Errorf("task not found: %s", taskID)
	}

	// Return a copy
	taskCopy := *task
	return &taskCopy, nil
}

// ListTasks returns all registered tasks
func (s *Scheduler) ListTasks() []*ScheduledTask {
	s.mu.RLock()
	defer s.mu.RUnlock()

	tasks := make([]*ScheduledTask, 0, len(s.tasks))
	for _, task := range s.tasks {
		taskCopy := *task
		tasks = append(tasks, &taskCopy)
	}

	// Sort by next run time
	sort.Slice(tasks, func(i, j int) bool {
		return tasks[i].NextRun.Before(tasks[j].NextRun)
	})

	return tasks
}

// GetHistory returns task run history
func (s *Scheduler) GetHistory(taskID string, limit int) []TaskHistory {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var filtered []TaskHistory
	for i := len(s.history) - 1; i >= 0; i-- {
		h := s.history[i]
		if taskID == "" || h.TaskID == taskID {
			filtered = append(filtered, h)
			if limit > 0 && len(filtered) >= limit {
				break
			}
		}
	}

	return filtered
}

// IsRunning returns whether the scheduler is running
func (s *Scheduler) IsRunning() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.running
}

// Stats returns scheduler statistics
func (s *Scheduler) Stats() map[string]interface{} {
	s.mu.RLock()
	defer s.mu.RUnlock()

	totalTasks := len(s.tasks)
	enabledTasks := 0
	totalRuns := int64(0)
	totalFails := int64(0)

	for _, task := range s.tasks {
		if task.Enabled {
			enabledTasks++
		}
		totalRuns += task.RunCount
		totalFails += task.FailCount
	}

	return map[string]interface{}{
		"running":       s.running,
		"total_tasks":   totalTasks,
		"enabled_tasks": enabledTasks,
		"total_runs":    totalRuns,
		"total_fails":   totalFails,
		"history_count": len(s.history),
	}
}

// BuiltinTaskFuncs holds all built-in task functions per AI.md PART 26
type BuiltinTaskFuncs struct {
	// ssl.renewal - Daily, renew certs 7 days before expiry
	SSLRenewal TaskFunc
	// geoip.update - Weekly, update GeoIP databases
	GeoIPUpdate TaskFunc
	// blocklist.update - Daily, update IP/domain blocklists
	BlocklistUpdate TaskFunc
	// cve.update - Daily, update CVE/security databases
	CVEUpdate TaskFunc
	// session.cleanup - Every 15 minutes, remove expired sessions
	SessionCleanup TaskFunc
	// token.cleanup - Every 15 minutes, remove expired tokens
	TokenCleanup TaskFunc
	// log.rotation - Daily, rotate and compress logs
	LogRotation TaskFunc
	// backup.auto - Disabled by default, automatic backups
	BackupAuto TaskFunc
	// healthcheck.self - Every 5 minutes, self-health check
	HealthcheckSelf TaskFunc
	// tor.health - Every 10 minutes, check Tor connectivity
	TorHealth TaskFunc
	// cluster.heartbeat - Every 30 seconds, cluster heartbeat
	ClusterHeartbeat TaskFunc
}

// RegisterBuiltinTasks registers all built-in scheduled tasks per AI.md PART 26
func (s *Scheduler) RegisterBuiltinTasks(funcs BuiltinTaskFuncs) {
	// ssl.renewal - Daily (runs check, renews if within 7 days of expiry)
	if funcs.SSLRenewal != nil {
		s.RegisterTask("ssl.renewal", "SSL Certificate Renewal",
			"Check and renew SSL certificates if needed (7 days before expiry)",
			"daily", funcs.SSLRenewal)
	}

	// geoip.update - Weekly
	if funcs.GeoIPUpdate != nil {
		s.RegisterTask("geoip.update", "GeoIP Database Update",
			"Download and update GeoIP databases from sapics/ip-location-db",
			"weekly", funcs.GeoIPUpdate)
	}

	// blocklist.update - Daily
	if funcs.BlocklistUpdate != nil {
		s.RegisterTask("blocklist.update", "Blocklist Update",
			"Download and update IP/domain blocklists",
			"daily", funcs.BlocklistUpdate)
	}

	// cve.update - Daily
	if funcs.CVEUpdate != nil {
		s.RegisterTask("cve.update", "CVE Database Update",
			"Download and update CVE/security vulnerability databases",
			"daily", funcs.CVEUpdate)
	}

	// session.cleanup - Every 15 minutes per AI.md PART 19
	if funcs.SessionCleanup != nil {
		s.RegisterTask("session.cleanup", "Session Cleanup",
			"Remove expired user and admin sessions",
			"15m", funcs.SessionCleanup)
	}

	// token.cleanup - Every 15 minutes per AI.md PART 19
	if funcs.TokenCleanup != nil {
		s.RegisterTask("token.cleanup", "Token Cleanup",
			"Remove expired API tokens and reset tokens",
			"15m", funcs.TokenCleanup)
	}

	// log.rotation - Daily
	if funcs.LogRotation != nil {
		s.RegisterTask("log.rotation", "Log Rotation",
			"Rotate and compress old log files",
			"daily", funcs.LogRotation)
	}

	// backup.auto - Per AI.md PART 22: Daily at 02:00, disabled by default
	if funcs.BackupAuto != nil {
		s.RegisterTask("backup.auto", "Automatic Backup",
			"Create automatic backups of configuration and databases",
			"0 2 * * *", funcs.BackupAuto) // Cron: 02:00 daily per AI.md PART 22
		// Disable by default per AI.md PART 26
		s.DisableTask("backup.auto")
	}

	// healthcheck.self - Every 5 minutes
	if funcs.HealthcheckSelf != nil {
		s.RegisterTask("healthcheck.self", "Self Health Check",
			"Perform internal health verification",
			"5m", funcs.HealthcheckSelf)
	}

	// tor.health - Every 10 minutes (only when Tor is installed/enabled)
	if funcs.TorHealth != nil {
		s.RegisterTask("tor.health", "Tor Health Check",
			"Check Tor connectivity and restart if needed",
			"10m", funcs.TorHealth)
	}

	// cluster.heartbeat - Every 30 seconds (only in cluster mode)
	if funcs.ClusterHeartbeat != nil {
		s.RegisterTask("cluster.heartbeat", "Cluster Heartbeat",
			"Send heartbeat to cluster nodes",
			"30s", funcs.ClusterHeartbeat)
	}
}

// RegisterDefaultTasks is deprecated, use RegisterBuiltinTasks instead
// Kept for backwards compatibility
func (s *Scheduler) RegisterDefaultTasks(
	certRenewal TaskFunc,
	notificationCheck TaskFunc,
	cleanup TaskFunc,
	healthCheck TaskFunc,
) {
	funcs := BuiltinTaskFuncs{
		SSLRenewal:      certRenewal,
		SessionCleanup:  cleanup,
		HealthcheckSelf: healthCheck,
	}
	s.RegisterBuiltinTasks(funcs)
}
