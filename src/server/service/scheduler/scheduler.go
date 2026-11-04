// SPDX-License-Identifier: MIT
// AI.md PART 18: Scheduler with Database Persistence
package scheduler

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"os"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/apimgr/vidveil/src/server/service/logging"
	"github.com/apimgr/vidveil/src/server/service/metric"
)

// defaultSchedulerTimezone is applied when the TZ environment variable is not
// set, per AI.md PART 18 (default timezone: America/New_York).
const defaultSchedulerTimezone = "America/New_York"

// resolveSchedulerLocation returns the location used to evaluate cron
// schedules. When TZ is set, Go already reflects it in time.Local; otherwise
// the AI.md PART 18 default (America/New_York) is used, falling back to UTC if
// the zone database is unavailable.
func resolveSchedulerLocation() *time.Location {
	if os.Getenv("TZ") != "" {
		return time.Local
	}
	loc, err := time.LoadLocation(defaultSchedulerTimezone)
	if err != nil {
		return time.UTC
	}
	return loc
}

// TaskFunc is a function that executes a scheduled task
type TaskFunc func(ctx context.Context) error

// TaskFailureNotifier receives completed failures so the application can send
// configured operator notifications without coupling the scheduler to a channel.
type TaskFailureNotifier func(task ScheduledTask, history TaskHistory)

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
	// retryCount tracks consecutive failed attempts for exponential backoff per AI.md PART 18
	retryCount int
	// running marks a task whose execution is currently in flight, used to
	// enforce the PART 18 overlap policy (a still-running task is skipped, not
	// run concurrently). In-memory only, guarded by Scheduler.mu.
	running bool `json:"-"`
}

// Retry policy per AI.md PART 18: max 3 retries, 5m base delay, exponential backoff (5m, 10m, 20m)
const (
	schedulerMaxRetries = 3
	schedulerRetryDelay = 5 * time.Minute
)

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
	tasks      map[string]*ScheduledTask
	history    []TaskHistory
	mu         sync.RWMutex
	ctx        context.Context
	cancel     context.CancelFunc
	taskCtx    context.Context
	taskCancel context.CancelFunc
	taskWG     sync.WaitGroup
	running    bool
	stopping   bool
	maxHist    int
	// Optional database for persistence per AI.md PART 18
	db              *sql.DB
	logger          *logging.AppLogger
	failureNotifier TaskFailureNotifier
	// Catch-up window per AI.md PART 18: run missed tasks if within this duration
	catchUpWindow time.Duration
	// loc is the timezone used to evaluate cron schedules per AI.md PART 18
	loc *time.Location
}

// SetLogger routes scheduler activity to the configured log files. Scheduler
// tasks never write operational messages to the process console.
func (s *Scheduler) SetLogger(logger *logging.AppLogger) {
	s.mu.Lock()
	s.logger = logger
	s.mu.Unlock()
}

// SetFailureNotifier wires the application's configured operator notification
// delivery for failed task runs.
func (s *Scheduler) SetFailureNotifier(notifier TaskFailureNotifier) {
	s.mu.Lock()
	s.failureNotifier = notifier
	s.mu.Unlock()
}

func (s *Scheduler) logTaskAudit(task *ScheduledTask, history TaskHistory) {
	s.mu.RLock()
	logger := s.logger
	s.mu.RUnlock()
	if logger == nil {
		return
	}
	result := "success"
	if history.Result == "failure" {
		result = "failure"
	}
	details := map[string]interface{}{
		"task_id":     task.ID,
		"task_name":   task.Name,
		"started_at":  history.StartTime.UTC().Format(time.RFC3339Nano),
		"duration_ms": history.Duration.Milliseconds(),
		"status":      history.Result,
	}
	if history.Error != "" {
		details["error"] = history.Error
	}
	logger.Audit("scheduler.task", task.ID, "system", "", result, details)
}

func (s *Scheduler) logSchedulerError(message string, fields map[string]interface{}) {
	s.mu.RLock()
	logger := s.logger
	s.mu.RUnlock()
	if logger != nil {
		logger.Error(message, fields)
	}
}

func (s *Scheduler) logSchedulerInfo(message string, fields map[string]interface{}) {
	s.mu.RLock()
	logger := s.logger
	s.mu.RUnlock()
	if logger != nil {
		logger.Info(message, fields)
	}
}

// now returns the current time in the scheduler's configured timezone so cron
// expressions evaluate against the AI.md PART 18 timezone.
func (s *Scheduler) now() time.Time {
	if s.loc == nil {
		return time.Now()
	}
	return time.Now().In(s.loc)
}

// NewScheduler creates a new scheduler without database persistence
func NewScheduler() *Scheduler {
	return &Scheduler{
		tasks:   make(map[string]*ScheduledTask),
		history: make([]TaskHistory, 0),
		// Keep last 100 history entries in memory
		maxHist: 100,
		loc:     resolveSchedulerLocation(),
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
		loc:     resolveSchedulerLocation(),
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
// execCtx runs a statement whose execution completes before this helper
// returns, so scoping the timeout context to the helper is safe. Query
// helpers that return lazily-scanned *sql.Rows / *sql.Row are intentionally
// omitted: their context must outlive the caller's Scan, so callers create
// the context inline and defer cancel() in the scanning function.
func (s *Scheduler) execCtx(query string, args ...interface{}) (sql.Result, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	return s.db.ExecContext(ctx, query, args...)
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

	// Context must remain live through the row iteration below, so it is
	// created here and cancelled on function return (not inside a helper
	// that returns before Scan runs) per AI.md PART 10.
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	rows, err := s.db.QueryContext(ctx, `
		SELECT task_id, start_time, end_time, duration_ms, result, error
		FROM task_history
		ORDER BY start_time DESC
		LIMIT ?`, limit)
	if err != nil {
		return fmt.Errorf("failed to load history: %w", err)
	}
	defer func() { _ = rows.Close() }()

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
	existingState, err := s.loadTaskStateFromDB(id)
	if err != nil {
		return fmt.Errorf("load persisted state for task %q: %w", id, err)
	}

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
		task.NextRun = cronSched.Next(s.now())
	} else {
		// Fall back to simple interval
		interval, err := parseInterval(schedule)
		if err != nil {
			return fmt.Errorf("invalid schedule '%s': %w", schedule, err)
		}
		task.Interval = interval
		task.NextRun = s.now().Add(interval)
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
		if !existingState.NextRun.IsZero() {
			// Preserve overdue state so Start can apply the configured catch-up
			// window instead of silently advancing missed work.
			task.NextRun = existingState.NextRun
		} else if !existingState.LastRun.IsZero() {
			if task.cronSched != nil {
				nextFromLast := task.cronSched.Next(existingState.LastRun)
				task.NextRun = nextFromLast
			} else {
				nextFromLast := existingState.LastRun.Add(task.Interval)
				task.NextRun = nextFromLast
			}
		}
	}

	// Persist initial state to database
	if err := s.saveTaskStateToDB(task); err != nil {
		return fmt.Errorf("persist initial state for task %q: %w", id, err)
	}
	s.tasks[id] = task

	return nil
}

// cronSchedule is the interface for cron-expression schedules (replaces robfig/cron dependency)
type cronSchedule interface {
	Next(t time.Time) time.Time
}

// cronExpr is a parsed 5-field cron expression (minute hour dom month dow)
type cronExpr struct {
	minutes []int
	hours   []int
	doms    []int
	months  []int
	dows    []int
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

	// field order: minute, hour, day-of-month, month, day-of-week
	ranges := [5][2]int{
		{0, 59},
		{0, 23},
		{1, 31},
		{1, 12},
		{0, 6},
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
		step := 1
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
	s.taskCtx, s.taskCancel = context.WithCancel(context.Background())
	s.stopping = false
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
	now := s.now()
	cutoff := now.Add(-window)

	// Snapshot the due tasks' schedule fields under the read lock so we never
	// read Enabled/NextRun concurrently with the writers that mutate them.
	type missedTask struct {
		task    *ScheduledTask
		nextRun time.Time
	}
	s.mu.RLock()
	candidates := make([]missedTask, 0, len(s.tasks))
	for _, task := range s.tasks {
		if task.Enabled && task.NextRun.Before(now) && task.NextRun.After(cutoff) {
			candidates = append(candidates, missedTask{task: task, nextRun: task.NextRun})
		}
	}
	s.mu.RUnlock()

	// Sort candidates by NextRun (oldest first) for proper ordering
	sort.Slice(candidates, func(i, j int) bool {
		return candidates[i].nextRun.Before(candidates[j].nextRun)
	})

	for _, c := range candidates {
		s.logSchedulerInfo("running missed scheduled task", map[string]interface{}{
			"task_id":      c.task.ID,
			"task_name":    c.task.Name,
			"scheduled_at": c.nextRun.Format(time.RFC3339),
		})
		go s.runTask(c.task)
	}
}

// Stop stops the scheduler
func (s *Scheduler) Stop() {
	s.mu.Lock()
	if s.cancel != nil {
		s.cancel()
	}
	s.running = false
	s.stopping = true
	taskCancel := s.taskCancel
	s.taskCtx = nil
	s.taskCancel = nil
	s.mu.Unlock()

	completed := make(chan struct{})
	go func() {
		s.taskWG.Wait()
		close(completed)
	}()
	select {
	case <-completed:
	case <-time.After(30 * time.Second):
		s.logSchedulerError("scheduler shutdown timed out waiting for tasks", map[string]interface{}{"timeout": "30s"})
	}
	if taskCancel != nil {
		taskCancel()
	}
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
	now := s.now()
	s.mu.RLock()
	due := make([]*ScheduledTask, 0, len(s.tasks))
	for _, task := range s.tasks {
		if task.Enabled && now.After(task.NextRun) {
			due = append(due, task)
		}
	}
	s.mu.RUnlock()

	for _, task := range due {
		go s.runTask(task)
	}
}

// runTask executes a single task
// Per AI.md PART 18: Task state is persisted to database after each run
func (s *Scheduler) runTask(task *ScheduledTask) {
	s.mu.Lock()
	// Overlap guard per AI.md PART 18: if a prior execution of this task is
	// still in flight, do not run it again — record a skipped result and move
	// its next run forward so the trigger does not tight-loop.
	if task.running {
		s.mu.Unlock()
		s.recordSkip(task)
		return
	}
	if s.stopping {
		s.mu.Unlock()
		return
	}
	task.running = true
	s.taskWG.Add(1)
	task.LastResult = "running"
	startTime := s.now()
	s.mu.Unlock()

	// Always clear the in-flight flag, even on panic, so a crashed task never
	// permanently blocks its own future triggers.
	defer func() {
		s.mu.Lock()
		task.running = false
		s.mu.Unlock()
		s.taskWG.Done()
	}()

	// Emit Prometheus scheduler metrics per AI.md PART 20
	metric.SchedulerTasksRunning.WithLabelValues(task.ID).Inc()
	metric.SchedulerLastRunTimestamp.WithLabelValues(task.ID).Set(float64(startTime.Unix()))
	defer metric.SchedulerTasksRunning.WithLabelValues(task.ID).Dec()

	// Create task context with timeout
	parentCtx := s.taskCtx
	if parentCtx == nil {
		parentCtx = context.Background()
	}
	ctx, cancel := context.WithTimeout(parentCtx, 5*time.Minute)
	defer cancel()

	err := invokeTask(ctx, task.fn)

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

	status := "success"
	var retryLog map[string]interface{}
	if err != nil {
		status = "failure"
		task.LastResult = "failure"
		task.LastError = err.Error()
		task.FailCount++
		hist.Result = "failure"
		hist.Error = err.Error()
		// Retry policy per AI.md PART 18: schedule a retry with exponential
		// backoff (5m, 10m, 20m) until max_retries is reached, then fall back
		// to the task's normal schedule computed above.
		if task.retryCount < schedulerMaxRetries {
			task.retryCount++
			backoff := schedulerRetryDelay * time.Duration(1<<(task.retryCount-1))
			task.NextRun = startTime.Add(backoff)
			retryLog = map[string]interface{}{
				"task_id":     task.ID,
				"retry":       task.retryCount,
				"max_retries": schedulerMaxRetries,
				"retry_delay": backoff.String(),
				"error":       err.Error(),
			}
		} else {
			task.retryCount = 0
		}
	} else {
		task.LastResult = "success"
		task.LastError = ""
		hist.Result = "success"
		task.retryCount = 0
	}

	// Record execution count and duration per AI.md PART 20
	metric.SchedulerTasksTotal.WithLabelValues(task.ID, status).Inc()
	metric.SchedulerTaskDuration.WithLabelValues(task.ID).Observe(duration.Seconds())

	// Add to in-memory history
	s.history = append(s.history, hist)
	// Trim in-memory history if needed
	if len(s.history) > s.maxHist {
		s.history = s.history[len(s.history)-s.maxHist:]
	}

	// Make a copy of task for DB operations outside lock
	taskCopy := *task
	s.mu.Unlock()
	if retryLog != nil {
		s.logSchedulerError("scheduled task failed; retry scheduled", retryLog)
	}

	// Persist state to database per AI.md PART 18
	// Done outside lock to avoid blocking other operations
	if err := s.saveTaskStateToDB(&taskCopy); err != nil {
		s.logSchedulerError("failed to persist scheduled task state", map[string]interface{}{"task_id": task.ID, "error": err.Error()})
	}
	if err := s.saveHistoryToDB(hist); err != nil {
		s.logSchedulerError("failed to persist scheduled task history", map[string]interface{}{"task_id": task.ID, "error": err.Error()})
	}
	s.logTaskAudit(&taskCopy, hist)
	if hist.Result == "failure" {
		s.mu.RLock()
		notifyFailure := s.failureNotifier
		s.mu.RUnlock()
		if notifyFailure != nil {
			notifyFailure(taskCopy, hist)
		}
	}
}

func invokeTask(ctx context.Context, fn TaskFunc) (err error) {
	defer func() {
		if recovered := recover(); recovered != nil {
			err = errors.New("scheduled task panicked")
		}
	}()
	return fn(ctx)
}

// recordSkip records a skipped run for a task whose prior execution is still in
// flight, per the AI.md PART 18 overlap policy. It advances NextRun (without
// touching run/fail/retry counters) and appends a "skipped" history entry.
func (s *Scheduler) recordSkip(task *ScheduledTask) {
	s.mu.Lock()
	skipTime := s.now()
	task.LastResult = "skipped"
	if task.cronSched != nil {
		task.NextRun = task.cronSched.Next(skipTime)
	} else {
		task.NextRun = skipTime.Add(task.Interval)
	}

	hist := TaskHistory{
		TaskID:    task.ID,
		StartTime: skipTime,
		EndTime:   skipTime,
		Duration:  0,
		Result:    "skipped",
	}
	s.history = append(s.history, hist)
	if len(s.history) > s.maxHist {
		s.history = s.history[len(s.history)-s.maxHist:]
	}

	taskCopy := *task
	s.mu.Unlock()

	s.logSchedulerInfo("scheduled task skipped because its previous run is still active", map[string]interface{}{"task_id": task.ID})
	if err := s.saveTaskStateToDB(&taskCopy); err != nil {
		s.logSchedulerError("failed to persist skipped task state", map[string]interface{}{"task_id": task.ID, "error": err.Error()})
	}
	if err := s.saveHistoryToDB(hist); err != nil {
		s.logSchedulerError("failed to persist skipped task history", map[string]interface{}{"task_id": task.ID, "error": err.Error()})
	}
	s.logTaskAudit(&taskCopy, hist)
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

	previousEnabled := task.Enabled
	previousNextRun := task.NextRun
	task.Enabled = true
	if task.NextRun.Before(s.now()) {
		// Calculate next run: use cron schedule if available, else use interval
		if task.cronSched != nil {
			task.NextRun = task.cronSched.Next(s.now())
		} else {
			task.NextRun = s.now().Add(task.Interval)
		}
	}
	taskCopy := *task
	if err := s.saveTaskStateToDB(&taskCopy); err != nil {
		task.Enabled = previousEnabled
		task.NextRun = previousNextRun
		s.mu.Unlock()
		return fmt.Errorf("persist enabled state for task %q: %w", taskID, err)
	}
	s.mu.Unlock()
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

	previousEnabled := task.Enabled
	task.Enabled = false
	taskCopy := *task
	if err := s.saveTaskStateToDB(&taskCopy); err != nil {
		task.Enabled = previousEnabled
		s.mu.Unlock()
		return fmt.Errorf("persist disabled state for task %q: %w", taskID, err)
	}
	s.mu.Unlock()
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
	previousSchedule := task.Schedule
	previousInterval := task.Interval
	previousCron := task.cronSched
	previousNextRun := task.NextRun

	// Try parsing as cron expression first
	if cronSched, err := parseCronSchedule(schedule); err == nil {
		task.cronSched = cronSched
		task.Interval = 0
		task.NextRun = cronSched.Next(s.now())
	} else {
		// Fall back to simple interval
		interval, err := parseInterval(schedule)
		if err != nil {
			s.mu.Unlock()
			return fmt.Errorf("invalid schedule '%s': %w", schedule, err)
		}
		task.cronSched = nil
		task.Interval = interval
		task.NextRun = s.now().Add(interval)
	}

	task.Schedule = schedule
	taskCopy := *task
	if err := s.saveTaskStateToDB(&taskCopy); err != nil {
		task.Schedule = previousSchedule
		task.Interval = previousInterval
		task.cronSched = previousCron
		task.NextRun = previousNextRun
		s.mu.Unlock()
		return fmt.Errorf("persist schedule for task %q: %w", taskID, err)
	}
	s.mu.Unlock()
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
	// i2p_health - Every 10 minutes per AI.md PART 18: registered whenever I2P
	// is opted in so the task can retry provider discovery; nil otherwise.
	I2PHealth TaskFunc
	// update_check - Daily at 06:00 per AI.md PART 18/22: notify-only unless auto_install is true
	UpdateCheck TaskFunc
}

// RegisterBuiltinTasks registers all built-in scheduled tasks per AI.md
// PART 18. Task IDs use the spec-canonical underscore form (e.g.
// "ssl_renewal"). Existing rows in scheduled_tasks / task_history that
// were persisted under the legacy dot form ("ssl.renewal") are
// renamed in-place by migrateLegacyTaskIDs() before registration so
// run history and per-task config survive the rename.
func (s *Scheduler) RegisterBuiltinTasks(funcs BuiltinTaskFuncs) error {
	if err := s.migrateLegacyTaskIDs(); err != nil {
		return err
	}
	register := func(id, name, description, schedule string, fn TaskFunc) error {
		if fn == nil {
			return nil
		}
		if err := s.RegisterTask(id, name, description, schedule, fn); err != nil {
			return fmt.Errorf("register scheduled task %s: %w", id, err)
		}
		return nil
	}

	// ssl_renewal - Daily at 03:00 per AI.md PART 18 (renews if within 7 days of expiry)
	if err := register("ssl_renewal", "SSL Certificate Renewal",
		"Check and renew SSL certificates if needed (7 days before expiry)",
		"0 3 * * *", funcs.SSLRenewal); err != nil {
		return err
	}

	// geoip_update - Weekly (Sunday 03:00) per AI.md PART 18
	if err := register("geoip_update", "GeoIP Database Update",
		"Download and update GeoIP databases from sapics/ip-location-db",
		"0 3 * * 0", funcs.GeoIPUpdate); err != nil {
		return err
	}

	// blocklist_update - Daily at 04:00 per AI.md PART 18
	if err := register("blocklist_update", "Blocklist Update",
		"Download and update IP/domain blocklists",
		"0 4 * * *", funcs.BlocklistUpdate); err != nil {
		return err
	}

	// cve_update - Daily at 05:00 per AI.md PART 18
	if err := register("cve_update", "CVE Database Update",
		"Download and update CVE/security vulnerability databases",
		"0 5 * * *", funcs.CVEUpdate); err != nil {
		return err
	}

	// token_cleanup - Every 15 minutes per AI.md PART 18
	if err := register("token_cleanup", "Token Cleanup",
		"Remove expired API tokens and reset tokens",
		"@every 15m", funcs.TokenCleanup); err != nil {
		return err
	}

	// log_rotation - Daily at 00:00 per AI.md PART 18
	if err := register("log_rotation", "Log Rotation",
		"Rotate and compress old log files",
		"0 0 * * *", funcs.LogRotation); err != nil {
		return err
	}

	// backup_daily - Per AI.md PART 18: Daily at 02:00, enabled by default
	if err := register("backup_daily", "Daily Backup",
		"Create daily full backup of configuration and databases",
		"0 2 * * *", funcs.BackupDaily); err != nil {
		return err
	}

	// backup_hourly - Per AI.md PART 18: Hourly incremental, disabled by default
	if funcs.BackupHourly != nil {
		if err := register("backup_hourly", "Hourly Backup",
			"Create hourly incremental backup (disabled by default)",
			"@hourly", funcs.BackupHourly); err != nil {
			return err
		}
		// Disabled by default per AI.md PART 18
		if err := s.DisableTask("backup_hourly"); err != nil {
			return err
		}
	}

	// healthcheck_self - Every 5 minutes
	if err := register("healthcheck_self", "Self Health Check",
		"Perform internal health verification",
		"@every 5m", funcs.HealthcheckSelf); err != nil {
		return err
	}

	// tor_health - Every 10 minutes (only when Tor is installed/enabled)
	if err := register("tor_health", "Tor Health Check",
		"Check Tor connectivity and restart if needed",
		"@every 10m", funcs.TorHealth); err != nil {
		return err
	}
	if err := register("i2p_health", "I2P Health Check",
		"Check I2P connectivity and restart if needed",
		"@every 10m", funcs.I2PHealth); err != nil {
		return err
	}

	// update_check - Daily at 06:00 per AI.md PART 18/22
	// Notify-only unless update.auto_install is true; honors update.defer_days
	if err := register("update_check", "Update Check",
		"Check release channel for a newer version; notify or auto-install per config",
		"0 6 * * *", funcs.UpdateCheck); err != nil {
		return err
	}
	return nil
}

// migrateLegacyTaskIDs renames built-in task IDs from the old "xxx.yyy"
// form to the spec-canonical "xxx_yyy" form (PART 18) in the persisted
// scheduled_tasks and task_history tables, so historical state and
// admin-tweaked schedules survive the upgrade. Idempotent — re-running
// it after migration is a no-op.
func (s *Scheduler) migrateLegacyTaskIDs() error {
	if s.db == nil {
		return nil
	}
	var migrationErr error
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
		if _, err := s.execCtx(
			`UPDATE scheduled_tasks SET id = ? WHERE id = ? `+
				`AND NOT EXISTS (SELECT 1 FROM scheduled_tasks WHERE id = ?)`,
			newID, old, newID,
		); err != nil {
			s.logSchedulerError("failed to migrate legacy scheduler task ID", map[string]interface{}{"old_id": old, "new_id": newID, "error": err.Error()})
			migrationErr = errors.Join(migrationErr, fmt.Errorf("migrate task ID %q to %q: %w", old, newID, err))
			continue
		}
		if _, err := s.execCtx(`DELETE FROM scheduled_tasks WHERE id = ?`, old); err != nil {
			s.logSchedulerError("failed to delete legacy scheduler task ID", map[string]interface{}{"task_id": old, "error": err.Error()})
			migrationErr = errors.Join(migrationErr, fmt.Errorf("delete legacy task ID %q: %w", old, err))
		}

		// Update task_history (run records).
		if _, err := s.execCtx(`UPDATE task_history SET task_id = ? WHERE task_id = ?`, newID, old); err != nil {
			s.logSchedulerError("failed to migrate legacy scheduler task history", map[string]interface{}{"old_id": old, "new_id": newID, "error": err.Error()})
			migrationErr = errors.Join(migrationErr, fmt.Errorf("migrate task history ID %q to %q: %w", old, newID, err))
		}
	}
	return migrationErr
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
	if err := s.RegisterBuiltinTasks(funcs); err != nil {
		s.logSchedulerError("failed to register default scheduled tasks", map[string]interface{}{"error": err.Error()})
	}
}
