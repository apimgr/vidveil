// SPDX-License-Identifier: MIT
// TEMPLATE.md PART 26: Built-in Scheduler
package scheduler

import (
	"context"
	"fmt"
	"sort"
	"sync"
	"time"
)

// TaskFunc is a function that executes a scheduled task
type TaskFunc func(ctx context.Context) error

// Task represents a scheduled task
type Task struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description"`
	// Schedule format: hourly, daily, weekly, monthly, or cron expression
	Schedule   string    `json:"schedule"`
	Enabled    bool      `json:"enabled"`
	LastRun    time.Time `json:"last_run"`
	// LastResult: success, failure, running, or pending
	LastResult string    `json:"last_result"`
	LastError  string    `json:"last_error,omitempty"`
	NextRun    time.Time `json:"next_run"`
	RunCount   int64     `json:"run_count"`
	FailCount  int64     `json:"fail_count"`
	Interval   time.Duration `json:"-"`
	fn         TaskFunc
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

// Scheduler manages scheduled tasks per TEMPLATE.md PART 9
type Scheduler struct {
	tasks    map[string]*Task
	history  []TaskHistory
	mu       sync.RWMutex
	ctx      context.Context
	cancel   context.CancelFunc
	running  bool
	maxHist  int
}

// New creates a new scheduler
func New() *Scheduler {
	return &Scheduler{
		tasks:   make(map[string]*Task),
		history: make([]TaskHistory, 0),
		// Keep last 100 history entries
		maxHist: 100,
	}
}

// RegisterTask registers a new scheduled task
func (s *Scheduler) RegisterTask(id, name, description, schedule string, fn TaskFunc) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	interval, err := parseSchedule(schedule)
	if err != nil {
		return fmt.Errorf("invalid schedule '%s': %w", schedule, err)
	}

	task := &Task{
		ID:          id,
		Name:        name,
		Description: description,
		Schedule:    schedule,
		Enabled:     true,
		LastResult:  "pending",
		Interval:    interval,
		fn:          fn,
	}
	task.NextRun = time.Now().Add(interval)

	s.tasks[id] = task
	return nil
}

// parseSchedule converts schedule string to duration per TEMPLATE.md
func parseSchedule(schedule string) (time.Duration, error) {
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
	tasks := make([]*Task, 0, len(s.tasks))
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
func (s *Scheduler) runTask(task *Task) {
	s.mu.Lock()
	task.LastResult = "running"
	startTime := time.Now()
	s.mu.Unlock()

	// Create task context with timeout
	ctx, cancel := context.WithTimeout(s.ctx, 5*time.Minute)
	defer cancel()

	err := task.fn(ctx)

	s.mu.Lock()
	defer s.mu.Unlock()

	endTime := time.Now()
	duration := endTime.Sub(startTime)

	task.LastRun = startTime
	task.NextRun = startTime.Add(task.Interval)
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

	// Add to history
	s.history = append(s.history, hist)
	// Trim history if needed
	if len(s.history) > s.maxHist {
		s.history = s.history[len(s.history)-s.maxHist:]
	}
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
func (s *Scheduler) EnableTask(taskID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	task, ok := s.tasks[taskID]
	if !ok {
		return fmt.Errorf("task not found: %s", taskID)
	}

	task.Enabled = true
	if task.NextRun.Before(time.Now()) {
		task.NextRun = time.Now().Add(task.Interval)
	}
	return nil
}

// DisableTask disables a task
func (s *Scheduler) DisableTask(taskID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	task, ok := s.tasks[taskID]
	if !ok {
		return fmt.Errorf("task not found: %s", taskID)
	}

	task.Enabled = false
	return nil
}

// SetSchedule updates a task's schedule
func (s *Scheduler) SetSchedule(taskID, schedule string) error {
	interval, err := parseSchedule(schedule)
	if err != nil {
		return err
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	task, ok := s.tasks[taskID]
	if !ok {
		return fmt.Errorf("task not found: %s", taskID)
	}

	task.Schedule = schedule
	task.Interval = interval
	task.NextRun = time.Now().Add(interval)
	return nil
}

// GetTask returns a task by ID
func (s *Scheduler) GetTask(taskID string) (*Task, error) {
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
func (s *Scheduler) ListTasks() []*Task {
	s.mu.RLock()
	defer s.mu.RUnlock()

	tasks := make([]*Task, 0, len(s.tasks))
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

// BuiltinTaskFuncs holds all built-in task functions per TEMPLATE.md PART 26
type BuiltinTaskFuncs struct {
	// ssl.renewal - Daily, renew certs 7 days before expiry
	SSLRenewal TaskFunc
	// geoip.update - Weekly, update GeoIP databases
	GeoIPUpdate TaskFunc
	// blocklist.update - Daily, update IP/domain blocklists
	BlocklistUpdate TaskFunc
	// cve.update - Daily, update CVE/security databases
	CVEUpdate TaskFunc
	// session.cleanup - Hourly, remove expired sessions
	SessionCleanup TaskFunc
	// token.cleanup - Daily, remove expired tokens
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

// RegisterBuiltinTasks registers all built-in scheduled tasks per TEMPLATE.md PART 26
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

	// session.cleanup - Hourly
	if funcs.SessionCleanup != nil {
		s.RegisterTask("session.cleanup", "Session Cleanup",
			"Remove expired user and admin sessions",
			"hourly", funcs.SessionCleanup)
	}

	// token.cleanup - Daily
	if funcs.TokenCleanup != nil {
		s.RegisterTask("token.cleanup", "Token Cleanup",
			"Remove expired API tokens and reset tokens",
			"daily", funcs.TokenCleanup)
	}

	// log.rotation - Daily
	if funcs.LogRotation != nil {
		s.RegisterTask("log.rotation", "Log Rotation",
			"Rotate and compress old log files",
			"daily", funcs.LogRotation)
	}

	// backup.auto - Disabled by default (registered but disabled)
	if funcs.BackupAuto != nil {
		s.RegisterTask("backup.auto", "Automatic Backup",
			"Create automatic backups of configuration and databases",
			"daily", funcs.BackupAuto)
		// Disable by default per TEMPLATE.md PART 26
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
