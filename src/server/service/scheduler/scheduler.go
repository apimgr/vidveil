// SPDX-License-Identifier: MIT
// AI.md PART 18: Scheduler with Database Persistence
package scheduler

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"
)

// TaskFunc is a function that executes a scheduled task
type TaskFunc func(ctx context.Context) error

// ScheduledTask represents a scheduled task
type ScheduledTask struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description"`
	// Schedule format: hourly, daily, weekly, monthly, or cron expression (e.g., "0 2 * * *")
	Schedule string    `json:"schedule"`
	Enabled  bool      `json:"enabled"`
	LastRun  time.Time `json:"last_run"`
	// LastResult: success, failure, running, or pending
	LastResult string    `json:"last_result"`
	LastError  string    `json:"last_error,omitempty"`
	NextRun    time.Time `json:"next_run"`
	RunCount   int64     `json:"run_count"`
	FailCount  int64     `json:"fail_count"`
	// Interval is for simple duration-based schedules
	Interval time.Duration `json:"-"`
	// cronSched is for cron-expression schedules per AI.md PART 18
	cronSched cronSchedule `json:"-"`
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

// Scheduler manages scheduled tasks per AI.md PART 18
// Supports optional database persistence for task state survival across restarts
type Scheduler struct {
	tasks   map[string]*ScheduledTask
	history []TaskHistory
	mu      sync.RWMutex
	ctx     context.Context
	cancel  context.CancelFunc
	running bool
	maxHist int
	// Optional database for persistence per AI.md PART 18
	db *sql.DB
	// Catch-up window per AI.md PART 18: run missed tasks if within this duration
	catchUpWindow time.Duration
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

// NewSchedulerWithDB creates a new scheduler with database persistence per AI.md PART 18
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

// SetCatchUpWindow sets the catch-up window per AI.md PART 18
// Missed tasks within this window will run on startup
func (s *Scheduler) SetCatchUpWindow(window time.Duration) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.catchUpWindow = window
}

// Query timeout helpers per AI.md PART 10: All queries MUST have timeouts
func (s *Scheduler) execCtx(query string, args ...interface{}) (sql.Result, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	return s.db.ExecContext(ctx, query, args...)
}

func (s *Scheduler) queryCtx(query string, args ...interface{}) (*sql.Rows, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	return s.db.QueryContext(ctx, query, args...)
}

func (s *Scheduler) queryRowCtx(query string, args ...interface{}) *sql.Row {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	return s.db.QueryRowContext(ctx, query, args...)
}

// loadTaskStateFromDB loads persisted task state from database
// Called during RegisterTask to restore run_count, fail_count, last_run, etc.
func (s *Scheduler) loadTaskStateFromDB(taskID string) (*ScheduledTask, error) {
	if s.db == nil {
		return nil, nil
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	row := s.db.QueryRowContext(ctx, `
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

	_, err := s.execCtx(`
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

	_, err := s.execCtx(`
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

	rows, err := s.queryCtx(`
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
// Per AI.md PART 18/22: Supports cron expressions or simple intervals
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

	// Try parsing as cron expression first (per AI.md PART 18)
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

	// Merge persisted state from database per AI.md PART 18
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

// cronSchedule is the interface for cron-expression schedules (replaces robfig/cron dependency)
type cronSchedule interface {
	Next(t time.Time) time.Time
}

// cronExpr is a parsed 5-field cron expression (minute hour dom month dow)
type cronExpr struct {
	minutes  []int
	hours    []int
	doms     []int
	months   []int
	dows     []int
}

// Next returns the next activation time after t per AI.md PART 18
func (c *cronExpr) Next(t time.Time) time.Time {
	// Advance by one second so we never return t itself
	t = t.Add(time.Second).Truncate(time.Second)
	// Search up to 4 years to avoid infinite loop on impossible expressions
	limit := t.Add(4 * 365 * 24 * time.Hour)
	for t.Before(limit) {
		if !inList(c.months, int(t.Month())) {
			t = time.Date(t.Year(), t.Month()+1, 1, 0, 0, 0, 0, t.Location())
			continue
		}
		if !inList(c.doms, t.Day()) || !inList(c.dows, int(t.Weekday())) {
			t = time.Date(t.Year(), t.Month(), t.Day()+1, 0, 0, 0, 0, t.Location())
			continue
		}
		if !inList(c.hours, t.Hour()) {
			t = time.Date(t.Year(), t.Month(), t.Day(), t.Hour()+1, 0, 0, 0, t.Location())
			continue
		}
		if !inList(c.minutes, t.Minute()) {
			t = t.Add(time.Minute).Truncate(time.Minute)
			continue
		}
		return t.Truncate(time.Minute)
	}
	return time.Time{}
}

// inList reports whether v is in the sorted list
func inList(list []int, v int) bool {
	for _, x := range list {
		if x == v {
			return true
		}
	}
	return false
}

// parseCronSchedule parses a cron expression per AI.md PART 18
// Supports standard 5-field cron format: minute hour day-of-month month day-of-week
// No external dependencies — built-in parser only.
func parseCronSchedule(schedule string) (cronSchedule, error) {
	// Must be exactly 5 space-separated fields
	fields := strings.Fields(schedule)
	if len(fields) != 5 {
		return nil, fmt.Errorf("not a cron expression")
	}

	ranges := [5][2]int{
		{0, 59},  // minute
		{0, 23},  // hour
		{1, 31},  // dom
		{1, 12},  // month
		{0, 6},   // dow
	}

	parsed := make([][]int, 5)
	for i, field := range fields {
		vals, err := parseCronField(field, ranges[i][0], ranges[i][1])
		if err != nil {
			return nil, fmt.Errorf("field %d (%s): %w", i, field, err)
		}
		parsed[i] = vals
	}

	return &cronExpr{
		minutes: parsed[0],
		hours:   parsed[1],
		doms:    parsed[2],
		months:  parsed[3],
		dows:    parsed[4],
	}, nil
}

// parseCronField expands one cron field (supports *, */n, n, n-m, n,m,...)
func parseCronField(field string, min, max int) ([]int, error) {
	set := make(map[int]struct{})

	for _, part := range strings.Split(field, ",") {
		var step int = 1
		// Handle step syntax: */n or range/n
		if idx := strings.Index(part, "/"); idx >= 0 {
			s, err := strconv.Atoi(part[idx+1:])
			if err != nil || s < 1 {
				return nil, fmt.Errorf("invalid step in %q", part)
			}
			step = s
			part = part[:idx]
		}

		var lo, hi int
		if part == "*" {
			lo, hi = min, max
		} else if idx := strings.Index(part, "-"); idx >= 0 {
			var err error
			lo, err = strconv.Atoi(part[:idx])
			if err != nil {
				return nil, fmt.Errorf("invalid range start in %q", part)
			}
			hi, err = strconv.Atoi(part[idx+1:])
			if err != nil {
				return nil, fmt.Errorf("invalid range end in %q", part)
			}
		} else {
			v, err := strconv.Atoi(part)
			if err != nil {
				return nil, fmt.Errorf("invalid value %q", part)
			}
			lo, hi = v, v
		}

		if lo < min || hi > max || lo > hi {
			return nil, fmt.Errorf("value out of range [%d-%d] in %q", min, max, part)
		}
		for v := lo; v <= hi; v += step {
			set[v] = struct{}{}
		}
	}

	result := make([]int, 0, len(set))
	for v := range set {
		result = append(result, v)
	}
	sort.Ints(result)
	return result, nil
}

// parseInterval converts schedule string to duration per AI.md PART 18.
// Supports: @every Xm / @every Xh / @hourly / @daily / @weekly / @monthly
// and bare Go duration strings (e.g. "15m") for backward compatibility.
func parseInterval(schedule string) (time.Duration, error) {
	// @every <duration> per AI.md PART 18 ("@every 15m", "@every 5m", "@every 1h", …)
	if strings.HasPrefix(schedule, "@every ") {
		d, err := time.ParseDuration(strings.TrimPrefix(schedule, "@every "))
		if err != nil {
			return 0, fmt.Errorf("invalid @every duration %q: %w", schedule, err)
		}
		return d, nil
	}
	switch schedule {
	case "hourly", "@hourly":
		return time.Hour, nil
	case "daily", "@daily":
		return 24 * time.Hour, nil
	case "weekly", "@weekly":
		return 7 * 24 * time.Hour, nil
	case "monthly", "@monthly":
		return 30 * 24 * time.Hour, nil
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
// Per AI.md PART 18: Checks for missed tasks within catch-up window and runs them
func (s *Scheduler) Start(ctx context.Context) {
	s.mu.Lock()
	if s.running {
		s.mu.Unlock()
		return
	}
	s.ctx, s.cancel = context.WithCancel(ctx)
	s.running = true
	catchUpWindow := s.catchUpWindow
	s.mu.Unlock()

	// Check for missed tasks within catch-up window per AI.md PART 18
	if catchUpWindow > 0 {
		s.runMissedTasks(catchUpWindow)
	}

	go s.run()
}

// runMissedTasks runs tasks that were missed while the server was down
// Per AI.md PART 18: Only runs if missed within catch_up_window
func (s *Scheduler) runMissedTasks(window time.Duration) {
	s.mu.RLock()
	tasks := make([]*ScheduledTask, 0, len(s.tasks))
	for _, task := range s.tasks {
		tasks = append(tasks, task)
	}
	s.mu.RUnlock()

	now := time.Now()
	cutoff := now.Add(-window)

	// Sort tasks by NextRun (oldest first) for proper ordering
	sort.Slice(tasks, func(i, j int) bool {
		return tasks[i].NextRun.Before(tasks[j].NextRun)
	})

	for _, task := range tasks {
		// Skip if not enabled or not missed
		if !task.Enabled {
			continue
		}
		// Task is missed if NextRun is in the past but after cutoff
		if task.NextRun.Before(now) && task.NextRun.After(cutoff) {
			log.Printf("running missed task: %s (was due at %s)", task.Name, task.NextRun.Format(time.RFC3339))
			go s.runTask(task)
		}
	}
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
// Per AI.md PART 18: Task state is persisted to database after each run
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

	// Persist state to database per AI.md PART 18
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
// Per AI.md PART 18: Enabled state is persisted to database
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
// Per AI.md PART 18: Enabled state is persisted to database
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
// Per AI.md PART 18/22: Schedule changes are persisted to database
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
	// token.cleanup - Every 15 minutes, remove expired tokens
	TokenCleanup TaskFunc
	// log.rotation - Daily, rotate and compress logs
	LogRotation TaskFunc
	// backup_daily - Daily at 02:00, full backup + daily incremental (enabled by default)
	BackupDaily TaskFunc
	// backup_hourly - Hourly incremental (disabled by default)
	BackupHourly TaskFunc
	// healthcheck.self - Every 5 minutes, self-health check
	HealthcheckSelf TaskFunc
	// tor.health - Every 10 minutes, check Tor connectivity
	TorHealth TaskFunc
	// update_check - Daily at 06:00 per AI.md PART 18/22: notify-only unless auto_install is true
	UpdateCheck TaskFunc
}

// RegisterBuiltinTasks registers all built-in scheduled tasks per AI.md
// PART 18. Task IDs use the spec-canonical underscore form (e.g.
// "ssl_renewal"). Existing rows in scheduled_tasks / task_history that
// were persisted under the legacy dot form ("ssl.renewal") are
// renamed in-place by migrateLegacyTaskIDs() before registration so
// run history and per-task config survive the rename.
func (s *Scheduler) RegisterBuiltinTasks(funcs BuiltinTaskFuncs) {
	s.migrateLegacyTaskIDs()

	// ssl_renewal - Daily at 03:00 per AI.md PART 18 (renews if within 7 days of expiry)
	if funcs.SSLRenewal != nil {
		s.RegisterTask("ssl_renewal", "SSL Certificate Renewal",
			"Check and renew SSL certificates if needed (7 days before expiry)",
			"0 3 * * *", funcs.SSLRenewal)
	}

	// geoip_update - Weekly (Sunday 03:00) per AI.md PART 18
	if funcs.GeoIPUpdate != nil {
		s.RegisterTask("geoip_update", "GeoIP Database Update",
			"Download and update GeoIP databases from sapics/ip-location-db",
			"0 3 * * 0", funcs.GeoIPUpdate)
	}

	// blocklist_update - Daily at 04:00 per AI.md PART 18
	if funcs.BlocklistUpdate != nil {
		s.RegisterTask("blocklist_update", "Blocklist Update",
			"Download and update IP/domain blocklists",
			"0 4 * * *", funcs.BlocklistUpdate)
	}

	// cve_update - Daily at 05:00 per AI.md PART 18
	if funcs.CVEUpdate != nil {
		s.RegisterTask("cve_update", "CVE Database Update",
			"Download and update CVE/security vulnerability databases",
			"0 5 * * *", funcs.CVEUpdate)
	}

	// token_cleanup - Every 15 minutes per AI.md PART 18
	if funcs.TokenCleanup != nil {
		s.RegisterTask("token_cleanup", "Token Cleanup",
			"Remove expired API tokens and reset tokens",
			"@every 15m", funcs.TokenCleanup)
	}

	// log_rotation - Daily at 00:00 per AI.md PART 18
	if funcs.LogRotation != nil {
		s.RegisterTask("log_rotation", "Log Rotation",
			"Rotate and compress old log files",
			"0 0 * * *", funcs.LogRotation)
	}

	// backup_daily - Per AI.md PART 18: Daily at 02:00, enabled by default
	if funcs.BackupDaily != nil {
		s.RegisterTask("backup_daily", "Daily Backup",
			"Create daily full backup of configuration and databases",
			"0 2 * * *", funcs.BackupDaily)
		// Enabled by default per AI.md PART 18 (Skippable: Yes = admin can disable)
	}

	// backup_hourly - Per AI.md PART 18: Hourly incremental, disabled by default
	if funcs.BackupHourly != nil {
		s.RegisterTask("backup_hourly", "Hourly Backup",
			"Create hourly incremental backup (disabled by default)",
			"@hourly", funcs.BackupHourly)
		// Disabled by default per AI.md PART 18
		s.DisableTask("backup_hourly")
	}

	// healthcheck_self - Every 5 minutes
	if funcs.HealthcheckSelf != nil {
		s.RegisterTask("healthcheck_self", "Self Health Check",
			"Perform internal health verification",
			"@every 5m", funcs.HealthcheckSelf)
	}

	// tor_health - Every 10 minutes (only when Tor is installed/enabled)
	if funcs.TorHealth != nil {
		s.RegisterTask("tor_health", "Tor Health Check",
			"Check Tor connectivity and restart if needed",
			"@every 10m", funcs.TorHealth)
	}

	// update_check - Daily at 06:00 per AI.md PART 18/22
	// Notify-only unless update.auto_install is true; honors update.defer_days
	if funcs.UpdateCheck != nil {
		s.RegisterTask("update_check", "Update Check",
			"Check release channel for a newer version; notify or auto-install per config",
			"0 6 * * *", funcs.UpdateCheck)
	}

}

// migrateLegacyTaskIDs renames built-in task IDs from the old "xxx.yyy"
// form to the spec-canonical "xxx_yyy" form (PART 18) in the persisted
// scheduled_tasks and task_history tables, so historical state and
// admin-tweaked schedules survive the upgrade. Idempotent — re-running
// it after migration is a no-op.
func (s *Scheduler) migrateLegacyTaskIDs() {
	if s.db == nil {
		return
	}
	rename := map[string]string{
		"ssl.renewal":      "ssl_renewal",
		"geoip.update":     "geoip_update",
		"blocklist.update": "blocklist_update",
		"cve.update":       "cve_update",
		"token.cleanup":    "token_cleanup",
		"log.rotation":     "log_rotation",
		"backup.auto":      "backup_daily",
		"backup_auto":      "backup_daily",
		"healthcheck.self": "healthcheck_self",
		"tor.health":       "tor_health",
	}
	for old, newID := range rename {
		// Update scheduled_tasks (state). UPDATE OR IGNORE conflict if
		// the new ID already has a row from a prior migration / a fresh
		// install — keep the new row in that case.
		if _, err := s.db.Exec(
			`UPDATE scheduled_tasks SET id = ? WHERE id = ? `+
				`AND NOT EXISTS (SELECT 1 FROM scheduled_tasks WHERE id = ?)`,
			newID, old, newID,
		); err != nil {
			log.Printf("scheduler: migrate task id %q->%q (scheduled_tasks): %v", old, newID, err)
		}
		if _, err := s.db.Exec(`DELETE FROM scheduled_tasks WHERE id = ?`, old); err != nil {
			log.Printf("scheduler: delete legacy task id %q: %v", old, err)
		}

		// Update task_history (run records).
		if _, err := s.db.Exec(`UPDATE task_history SET task_id = ? WHERE task_id = ?`, newID, old); err != nil {
			log.Printf("scheduler: migrate task id %q->%q (task_history): %v", old, newID, err)
		}
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
		TokenCleanup:    cleanup,
		HealthcheckSelf: healthCheck,
	}
	s.RegisterBuiltinTasks(funcs)
}
