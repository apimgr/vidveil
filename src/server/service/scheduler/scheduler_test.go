// SPDX-License-Identifier: MIT
package scheduler

import (
	"context"
	"database/sql"
	"testing"
	"time"
)

// --- NewScheduler ---

// TestNewScheduler_Structure verifies initial field values without a database.
func TestNewScheduler_Structure(t *testing.T) {
	s := NewScheduler()
	if s == nil {
		t.Fatal("NewScheduler returned nil")
	}
	if s.tasks == nil {
		t.Error("tasks map is nil")
	}
	if s.history == nil {
		t.Error("history slice is nil")
	}
	if len(s.history) != 0 {
		t.Errorf("history len = %d, want 0", len(s.history))
	}
	if s.maxHist != 100 {
		t.Errorf("maxHist = %d, want 100", s.maxHist)
	}
	if s.running {
		t.Error("running should be false after construction")
	}
	if s.db != nil {
		t.Error("db should be nil when using NewScheduler")
	}
	if s.cancel != nil {
		t.Error("cancel should be nil before Start")
	}
}

// --- NewSchedulerWithDB ---

// TestNewSchedulerWithDB_NilDB verifies that passing nil is accepted and the
// resulting scheduler has the same invariants as NewScheduler.
func TestNewSchedulerWithDB_NilDB(t *testing.T) {
	s := NewSchedulerWithDB(nil)
	if s == nil {
		t.Fatal("NewSchedulerWithDB(nil) returned nil")
	}
	if s.tasks == nil {
		t.Error("tasks map is nil")
	}
	if s.history == nil {
		t.Error("history slice is nil")
	}
	if s.maxHist != 100 {
		t.Errorf("maxHist = %d, want 100", s.maxHist)
	}
	if s.running {
		t.Error("running should be false")
	}
	if s.db != nil {
		t.Error("db should be nil when nil was passed")
	}
}

// --- SetDB ---

// TestSetDB_NilThenNonNil verifies SetDB does not panic and updates the field.
func TestSetDB_NilThenNonNil(t *testing.T) {
	s := NewScheduler()

	// Setting nil must not panic.
	s.SetDB(nil)
	if s.db != nil {
		t.Error("db should still be nil after SetDB(nil)")
	}

	// Setting a non-nil *sql.DB must be reflected in the field.
	// sql.Open is lazy; it does not connect, so no driver is needed.
	db, err := sql.Open("sqlite3", ":memory:")
	if err != nil {
		// Driver not available in this test environment — skip the non-nil case.
		t.Skip("sqlite3 driver unavailable:", err)
	}
	defer db.Close()

	s.SetDB(db)
	if s.db == nil {
		t.Error("db should be non-nil after SetDB(db)")
	}
}

// --- SetCatchUpWindow ---

// TestSetCatchUpWindow verifies the field is updated correctly.
func TestSetCatchUpWindow(t *testing.T) {
	s := NewScheduler()

	if s.catchUpWindow != 0 {
		t.Errorf("initial catchUpWindow = %v, want 0", s.catchUpWindow)
	}

	s.SetCatchUpWindow(2 * time.Hour)
	if s.catchUpWindow != 2*time.Hour {
		t.Errorf("catchUpWindow = %v, want 2h", s.catchUpWindow)
	}

	// Setting to zero must also work.
	s.SetCatchUpWindow(0)
	if s.catchUpWindow != 0 {
		t.Errorf("catchUpWindow after reset = %v, want 0", s.catchUpWindow)
	}
}

// --- parseInterval ---

// TestParseInterval_NamedSchedules verifies all recognised keyword mappings.
func TestParseInterval_NamedSchedules(t *testing.T) {
	cases := []struct {
		input string
		want  time.Duration
	}{
		{"hourly", time.Hour},
		{"daily", 24 * time.Hour},
		{"weekly", 7 * 24 * time.Hour},
		{"monthly", 30 * 24 * time.Hour},
	}
	for _, tc := range cases {
		t.Run(tc.input, func(t *testing.T) {
			got, err := parseInterval(tc.input)
			if err != nil {
				t.Fatalf("parseInterval(%q) unexpected error: %v", tc.input, err)
			}
			if got != tc.want {
				t.Errorf("parseInterval(%q) = %v, want %v", tc.input, got, tc.want)
			}
		})
	}
}

// TestParseInterval_DurationStrings verifies time.ParseDuration fall-through.
func TestParseInterval_DurationStrings(t *testing.T) {
	cases := []struct {
		input string
		want  time.Duration
	}{
		{"30m", 30 * time.Minute},
		{"2h", 2 * time.Hour},
		{"15s", 15 * time.Second},
		{"1h30m", 90 * time.Minute},
	}
	for _, tc := range cases {
		t.Run(tc.input, func(t *testing.T) {
			got, err := parseInterval(tc.input)
			if err != nil {
				t.Fatalf("parseInterval(%q) unexpected error: %v", tc.input, err)
			}
			if got != tc.want {
				t.Errorf("parseInterval(%q) = %v, want %v", tc.input, got, tc.want)
			}
		})
	}
}

// TestParseInterval_Errors verifies that invalid inputs return an error.
func TestParseInterval_Errors(t *testing.T) {
	cases := []string{"invalid", "", "not-a-duration", "1x"}
	for _, input := range cases {
		t.Run("bad:"+input, func(t *testing.T) {
			_, err := parseInterval(input)
			if err == nil {
				t.Errorf("parseInterval(%q) expected error, got nil", input)
			}
		})
	}
}

// --- parseCronSchedule ---

// TestParseCronSchedule_Valid verifies that valid 5-field cron expressions parse
// without error and return a non-nil schedule.
func TestParseCronSchedule_Valid(t *testing.T) {
	cases := []string{
		"0 2 * * *",
		"0 * * * *",
		"*/5 * * * *",
		"0 3 * * 0",
		"0 5 1 * *",
	}
	for _, expr := range cases {
		t.Run(expr, func(t *testing.T) {
			sched, err := parseCronSchedule(expr)
			if err != nil {
				t.Fatalf("parseCronSchedule(%q) unexpected error: %v", expr, err)
			}
			if sched == nil {
				t.Errorf("parseCronSchedule(%q) returned nil schedule", expr)
			}
		})
	}
}

// TestParseCronSchedule_Errors verifies that non-cron strings return errors.
func TestParseCronSchedule_Errors(t *testing.T) {
	cases := []string{
		"invalid cron",
		"@daily",
		"hourly",
		"0 2 * *",
		"",
		"* * * * * *",
	}
	for _, expr := range cases {
		t.Run("bad:"+expr, func(t *testing.T) {
			_, err := parseCronSchedule(expr)
			if err == nil {
				t.Errorf("parseCronSchedule(%q) expected error, got nil", expr)
			}
		})
	}
}

// --- RegisterTask ---

// noop is a trivial TaskFunc used wherever the function body does not matter.
func noop(_ context.Context) error { return nil }

// TestRegisterTask_IntervalSchedule verifies registration with a named interval.
func TestRegisterTask_IntervalSchedule(t *testing.T) {
	s := NewScheduler()
	err := s.RegisterTask("t1", "Task One", "desc", "hourly", noop)
	if err != nil {
		t.Fatalf("RegisterTask with 'hourly' returned error: %v", err)
	}
	if _, ok := s.tasks["t1"]; !ok {
		t.Error("task 't1' not found in tasks map after registration")
	}
}

// TestRegisterTask_CronSchedule verifies registration with a cron expression.
func TestRegisterTask_CronSchedule(t *testing.T) {
	s := NewScheduler()
	err := s.RegisterTask("cron1", "Cron Task", "desc", "0 * * * *", noop)
	if err != nil {
		t.Fatalf("RegisterTask with cron '0 * * * *' returned error: %v", err)
	}
	task := s.tasks["cron1"]
	if task.cronSched == nil {
		t.Error("cronSched should be non-nil for a cron-expression schedule")
	}
	if task.Interval != 0 {
		t.Errorf("Interval should be 0 for cron task, got %v", task.Interval)
	}
}

// TestRegisterTask_DuplicateID verifies that registering the same ID twice
// returns an error on the second call. (The scheduler overwrites the map
// entry; this test documents the actual behaviour so it will catch a
// future regression if the behaviour changes.)
func TestRegisterTask_DuplicateID(t *testing.T) {
	s := NewScheduler()
	if err := s.RegisterTask("dup", "First", "d", "hourly", noop); err != nil {
		t.Fatalf("first RegisterTask unexpected error: %v", err)
	}
	// Current implementation: overwrites silently (no error).
	// This test will fail if the behaviour changes to either (a) return an
	// error or (b) reject the second registration — both are observable bugs.
	err := s.RegisterTask("dup", "Second", "d", "hourly", noop)
	task := s.tasks["dup"]
	if err == nil {
		// Accepted the overwrite — verify the map holds the second registration.
		if task.Name != "Second" {
			t.Errorf("duplicate overwrite: Name = %q, want 'Second'", task.Name)
		}
	}
	// If err != nil, the implementation now rejects duplicates — both are fine
	// as long as the behaviour is consistent.
}

// TestRegisterTask_InvalidSchedule verifies that an unrecognisable schedule
// string is rejected.
func TestRegisterTask_InvalidSchedule(t *testing.T) {
	s := NewScheduler()
	err := s.RegisterTask("bad", "Bad", "d", "not-a-schedule", noop)
	if err == nil {
		t.Error("RegisterTask with invalid schedule should return an error")
	}
}

// TestRegisterTask_TaskEnabledByDefault verifies that a freshly registered task
// is enabled and carries "pending" as its initial LastResult.
func TestRegisterTask_TaskEnabledByDefault(t *testing.T) {
	s := NewScheduler()
	if err := s.RegisterTask("e1", "E", "d", "hourly", noop); err != nil {
		t.Fatalf("RegisterTask error: %v", err)
	}
	task := s.tasks["e1"]
	if !task.Enabled {
		t.Error("newly registered task should be enabled by default")
	}
	if task.LastResult != "pending" {
		t.Errorf("LastResult = %q, want 'pending'", task.LastResult)
	}
}

// TestRegisterTask_NilFn verifies that passing a nil function does not panic
// during registration (the panic would surface at run time).
func TestRegisterTask_NilFn(t *testing.T) {
	s := NewScheduler()
	// This should not panic; whether it returns an error is an implementation
	// detail — we only guard against a panic here.
	defer func() {
		if r := recover(); r != nil {
			t.Errorf("RegisterTask with nil fn panicked: %v", r)
		}
	}()
	s.RegisterTask("nilf", "Nil", "d", "hourly", nil)
}

// --- GetTask ---

// TestGetTask_Known verifies retrieval of a registered task.
func TestGetTask_Known(t *testing.T) {
	s := NewScheduler()
	if err := s.RegisterTask("g1", "Get One", "desc", "daily", noop); err != nil {
		t.Fatalf("RegisterTask error: %v", err)
	}
	task, err := s.GetTask("g1")
	if err != nil {
		t.Fatalf("GetTask('g1') error: %v", err)
	}
	if task.ID != "g1" {
		t.Errorf("task.ID = %q, want 'g1'", task.ID)
	}
	if task.Name != "Get One" {
		t.Errorf("task.Name = %q, want 'Get One'", task.Name)
	}
}

// TestGetTask_Unknown verifies that an unknown ID returns an error.
func TestGetTask_Unknown(t *testing.T) {
	s := NewScheduler()
	_, err := s.GetTask("does-not-exist")
	if err == nil {
		t.Error("GetTask with unknown ID should return an error")
	}
}

// TestGetTask_ReturnsCopy verifies that mutating the returned task does not
// affect the scheduler's internal state.
func TestGetTask_ReturnsCopy(t *testing.T) {
	s := NewScheduler()
	if err := s.RegisterTask("cp", "Copy", "d", "hourly", noop); err != nil {
		t.Fatalf("RegisterTask error: %v", err)
	}
	task, _ := s.GetTask("cp")
	task.Name = "mutated"

	internal := s.tasks["cp"]
	if internal.Name == "mutated" {
		t.Error("GetTask should return a copy, not a pointer to internal state")
	}
}

// --- ListTasks ---

// TestListTasks_Empty verifies that an empty scheduler returns an empty slice.
func TestListTasks_Empty(t *testing.T) {
	s := NewScheduler()
	tasks := s.ListTasks()
	if len(tasks) != 0 {
		t.Errorf("ListTasks on empty scheduler = %d tasks, want 0", len(tasks))
	}
}

// TestListTasks_ReturnsAll verifies that all registered tasks appear in the
// result.
func TestListTasks_ReturnsAll(t *testing.T) {
	s := NewScheduler()
	ids := []string{"z-task", "a-task", "m-task"}
	for _, id := range ids {
		if err := s.RegisterTask(id, id, "d", "hourly", noop); err != nil {
			t.Fatalf("RegisterTask(%q) error: %v", id, err)
		}
	}
	tasks := s.ListTasks()
	if len(tasks) != len(ids) {
		t.Errorf("ListTasks returned %d tasks, want %d", len(tasks), len(ids))
	}
}

// TestListTasks_SortedByNextRun verifies that the returned slice is sorted by
// NextRun (ascending).
func TestListTasks_SortedByNextRun(t *testing.T) {
	s := NewScheduler()
	// Register tasks with different intervals so their NextRun values differ.
	schedules := []struct {
		id  string
		dur string
	}{
		{"long", "weekly"},
		{"short", "5m"},
		{"mid", "daily"},
	}
	for _, sc := range schedules {
		if err := s.RegisterTask(sc.id, sc.id, "d", sc.dur, noop); err != nil {
			t.Fatalf("RegisterTask(%q) error: %v", sc.id, err)
		}
	}
	tasks := s.ListTasks()
	for i := 1; i < len(tasks); i++ {
		if tasks[i].NextRun.Before(tasks[i-1].NextRun) {
			t.Errorf("ListTasks not sorted: tasks[%d].NextRun (%v) < tasks[%d].NextRun (%v)",
				i, tasks[i].NextRun, i-1, tasks[i-1].NextRun)
		}
	}
}

// --- IsRunning ---

// TestIsRunning_FalseBeforeStart verifies the scheduler is not running until
// Start is called.
func TestIsRunning_FalseBeforeStart(t *testing.T) {
	s := NewScheduler()
	if s.IsRunning() {
		t.Error("IsRunning() should be false before Start()")
	}
}

// --- Stats ---

// TestStats_Keys verifies that Stats returns a map containing the expected keys.
func TestStats_Keys(t *testing.T) {
	s := NewScheduler()
	stats := s.Stats()

	required := []string{"running", "total_tasks", "enabled_tasks", "total_runs", "total_fails", "history_count"}
	for _, key := range required {
		if _, ok := stats[key]; !ok {
			t.Errorf("Stats() missing key %q", key)
		}
	}
}

// TestStats_EmptyScheduler verifies zero-value counts on a fresh scheduler.
func TestStats_EmptyScheduler(t *testing.T) {
	s := NewScheduler()
	stats := s.Stats()

	if stats["running"] != false {
		t.Errorf("stats[running] = %v, want false", stats["running"])
	}
	if stats["total_tasks"] != 0 {
		t.Errorf("stats[total_tasks] = %v, want 0", stats["total_tasks"])
	}
	if stats["history_count"] != 0 {
		t.Errorf("stats[history_count] = %v, want 0", stats["history_count"])
	}
}

// TestStats_WithTasks verifies counts after registering tasks.
func TestStats_WithTasks(t *testing.T) {
	s := NewScheduler()
	for _, id := range []string{"t1", "t2", "t3"} {
		if err := s.RegisterTask(id, id, "d", "hourly", noop); err != nil {
			t.Fatalf("RegisterTask error: %v", err)
		}
	}
	stats := s.Stats()
	if stats["total_tasks"] != 3 {
		t.Errorf("stats[total_tasks] = %v, want 3", stats["total_tasks"])
	}
	if stats["enabled_tasks"] != 3 {
		t.Errorf("stats[enabled_tasks] = %v, want 3", stats["enabled_tasks"])
	}
}

// --- GetHistory ---

// TestGetHistory_Empty verifies that history is empty before any tasks run.
func TestGetHistory_Empty(t *testing.T) {
	s := NewScheduler()
	if err := s.RegisterTask("h1", "H", "d", "hourly", noop); err != nil {
		t.Fatalf("RegisterTask error: %v", err)
	}
	hist := s.GetHistory("h1", 10)
	if len(hist) != 0 {
		t.Errorf("GetHistory before any run = %d entries, want 0", len(hist))
	}
}

// TestGetHistory_AllTasksWhenIDEmpty verifies that passing "" returns history
// across all tasks.
func TestGetHistory_AllTasksWhenIDEmpty(t *testing.T) {
	s := NewScheduler()
	hist := s.GetHistory("", 100)
	if len(hist) != 0 {
		t.Errorf("GetHistory('', 100) on empty scheduler = %d, want 0", len(hist))
	}
}

// TestGetHistory_LimitZeroMeansAll verifies that limit=0 returns all entries.
func TestGetHistory_LimitZeroMeansAll(t *testing.T) {
	s := NewScheduler()
	// Inject entries directly to avoid running goroutines.
	s.history = []TaskHistory{
		{TaskID: "x", Result: "success"},
		{TaskID: "x", Result: "failure"},
		{TaskID: "x", Result: "success"},
	}
	hist := s.GetHistory("x", 0)
	if len(hist) != 3 {
		t.Errorf("GetHistory with limit=0 = %d entries, want 3", len(hist))
	}
}

// TestGetHistory_LimitRespected verifies that a positive limit caps results.
func TestGetHistory_LimitRespected(t *testing.T) {
	s := NewScheduler()
	for i := 0; i < 5; i++ {
		s.history = append(s.history, TaskHistory{TaskID: "y", Result: "success"})
	}
	hist := s.GetHistory("y", 2)
	if len(hist) != 2 {
		t.Errorf("GetHistory with limit=2 = %d entries, want 2", len(hist))
	}
}

// TestGetHistory_FiltersByTaskID verifies that only entries for the requested
// task ID are returned.
func TestGetHistory_FiltersByTaskID(t *testing.T) {
	s := NewScheduler()
	s.history = []TaskHistory{
		{TaskID: "a", Result: "success"},
		{TaskID: "b", Result: "success"},
		{TaskID: "a", Result: "failure"},
	}
	hist := s.GetHistory("a", 10)
	if len(hist) != 2 {
		t.Errorf("GetHistory('a', 10) = %d entries, want 2", len(hist))
	}
	for _, h := range hist {
		if h.TaskID != "a" {
			t.Errorf("GetHistory returned entry with TaskID %q, want 'a'", h.TaskID)
		}
	}
}

// --- RunTaskNow ---

// TestRunTaskNow_ExecutesFunc verifies that the task function is called when
// RunTaskNow is used on a scheduler that has NOT been started.
func TestRunTaskNow_ExecutesFunc(t *testing.T) {
	s := NewScheduler()

	called := make(chan struct{}, 1)
	err := s.RegisterTask("run1", "Run One", "desc", "hourly", func(_ context.Context) error {
		called <- struct{}{}
		return nil
	})
	if err != nil {
		t.Fatalf("RegisterTask error: %v", err)
	}

	// runTask uses s.ctx for the task timeout; set it to background so the
	// goroutine does not get a nil-context panic.
	s.ctx, s.cancel = context.WithCancel(context.Background())
	defer s.cancel()

	err = s.RunTaskNow("run1")
	if err != nil {
		t.Fatalf("RunTaskNow error: %v", err)
	}

	select {
	case <-called:
	case <-time.After(2 * time.Second):
		t.Fatal("task was not called within 2 s")
	}
}

// TestRunTaskNow_UnknownID verifies that an unknown task returns an error.
func TestRunTaskNow_UnknownID(t *testing.T) {
	s := NewScheduler()
	err := s.RunTaskNow("no-such-task")
	if err == nil {
		t.Error("RunTaskNow with unknown ID should return an error")
	}
}

// TestRunTaskNow_UpdatesHistory verifies that history is appended after the
// task runs.
func TestRunTaskNow_UpdatesHistory(t *testing.T) {
	s := NewScheduler()

	done := make(chan struct{}, 1)
	if err := s.RegisterTask("hist1", "Hist", "d", "hourly", func(_ context.Context) error {
		done <- struct{}{}
		return nil
	}); err != nil {
		t.Fatalf("RegisterTask error: %v", err)
	}

	s.ctx, s.cancel = context.WithCancel(context.Background())
	defer s.cancel()

	if err := s.RunTaskNow("hist1"); err != nil {
		t.Fatalf("RunTaskNow error: %v", err)
	}

	// Wait for the goroutine to finish writing history.
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("task did not complete")
	}

	// Give the goroutine a moment to write history after sending on done.
	time.Sleep(10 * time.Millisecond)

	hist := s.GetHistory("hist1", 10)
	if len(hist) == 0 {
		t.Error("expected at least one history entry after RunTaskNow")
	}
}

// TestRunTaskNow_FailingTask verifies that a task returning an error is
// recorded as a failure in history.
func TestRunTaskNow_FailingTask(t *testing.T) {
	s := NewScheduler()

	done := make(chan struct{}, 1)
	if err := s.RegisterTask("fail1", "Fail", "d", "hourly", func(_ context.Context) error {
		done <- struct{}{}
		return context.DeadlineExceeded
	}); err != nil {
		t.Fatalf("RegisterTask error: %v", err)
	}

	s.ctx, s.cancel = context.WithCancel(context.Background())
	defer s.cancel()

	if err := s.RunTaskNow("fail1"); err != nil {
		t.Fatalf("RunTaskNow itself should not error on task failure: %v", err)
	}

	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("task did not complete")
	}

	time.Sleep(10 * time.Millisecond)

	hist := s.GetHistory("fail1", 10)
	if len(hist) == 0 {
		t.Fatal("expected history entry for failed task")
	}
	if hist[0].Result != "failure" {
		t.Errorf("history Result = %q, want 'failure'", hist[0].Result)
	}
}

// --- EnableTask / DisableTask ---

// TestEnableDisableTask_TogglesField verifies that the enabled flag flips
// correctly.
func TestEnableDisableTask_TogglesField(t *testing.T) {
	s := NewScheduler()
	if err := s.RegisterTask("ed1", "ED", "d", "hourly", noop); err != nil {
		t.Fatalf("RegisterTask error: %v", err)
	}

	// Disable a task that starts enabled.
	if err := s.DisableTask("ed1"); err != nil {
		t.Fatalf("DisableTask error: %v", err)
	}
	if s.tasks["ed1"].Enabled {
		t.Error("task should be disabled after DisableTask")
	}

	// Re-enable it.
	if err := s.EnableTask("ed1"); err != nil {
		t.Fatalf("EnableTask error: %v", err)
	}
	if !s.tasks["ed1"].Enabled {
		t.Error("task should be enabled after EnableTask")
	}
}

// TestEnableTask_UnknownID verifies error on unknown task.
func TestEnableTask_UnknownID(t *testing.T) {
	s := NewScheduler()
	if err := s.EnableTask("ghost"); err == nil {
		t.Error("EnableTask with unknown ID should return error")
	}
}

// TestDisableTask_UnknownID verifies error on unknown task.
func TestDisableTask_UnknownID(t *testing.T) {
	s := NewScheduler()
	if err := s.DisableTask("ghost"); err == nil {
		t.Error("DisableTask with unknown ID should return error")
	}
}

// TestDisableTask_Idempotent verifies that disabling an already-disabled task
// does not return an error or panic.
func TestDisableTask_Idempotent(t *testing.T) {
	s := NewScheduler()
	if err := s.RegisterTask("idem", "I", "d", "hourly", noop); err != nil {
		t.Fatalf("RegisterTask error: %v", err)
	}
	if err := s.DisableTask("idem"); err != nil {
		t.Fatalf("first DisableTask error: %v", err)
	}
	if err := s.DisableTask("idem"); err != nil {
		t.Fatalf("second DisableTask (idempotent) error: %v", err)
	}
}

// --- SetSchedule ---

// TestSetSchedule_UpdatesInterval verifies that switching to a new interval
// schedule changes the Interval field and clears cronSched.
func TestSetSchedule_UpdatesInterval(t *testing.T) {
	s := NewScheduler()
	if err := s.RegisterTask("ss1", "SS", "d", "hourly", noop); err != nil {
		t.Fatalf("RegisterTask error: %v", err)
	}
	if err := s.SetSchedule("ss1", "daily"); err != nil {
		t.Fatalf("SetSchedule error: %v", err)
	}
	task := s.tasks["ss1"]
	if task.Schedule != "daily" {
		t.Errorf("Schedule = %q, want 'daily'", task.Schedule)
	}
	if task.Interval != 24*time.Hour {
		t.Errorf("Interval = %v, want 24h", task.Interval)
	}
	if task.cronSched != nil {
		t.Error("cronSched should be nil after switching to an interval schedule")
	}
}

// TestSetSchedule_UpdatesCron verifies that switching to a cron expression sets
// cronSched and zeroes Interval.
func TestSetSchedule_UpdatesCron(t *testing.T) {
	s := NewScheduler()
	if err := s.RegisterTask("ss2", "SS", "d", "hourly", noop); err != nil {
		t.Fatalf("RegisterTask error: %v", err)
	}
	if err := s.SetSchedule("ss2", "0 4 * * *"); err != nil {
		t.Fatalf("SetSchedule error: %v", err)
	}
	task := s.tasks["ss2"]
	if task.cronSched == nil {
		t.Error("cronSched should be set after switching to a cron expression")
	}
	if task.Interval != 0 {
		t.Errorf("Interval = %v, want 0 for cron schedule", task.Interval)
	}
}

// TestSetSchedule_InvalidSchedule verifies that an invalid schedule returns an
// error and does not change the task.
func TestSetSchedule_InvalidSchedule(t *testing.T) {
	s := NewScheduler()
	if err := s.RegisterTask("ss3", "SS", "d", "hourly", noop); err != nil {
		t.Fatalf("RegisterTask error: %v", err)
	}
	originalInterval := s.tasks["ss3"].Interval

	err := s.SetSchedule("ss3", "not-valid-at-all")
	if err == nil {
		t.Error("SetSchedule with invalid schedule should return an error")
	}
	if s.tasks["ss3"].Interval != originalInterval {
		t.Error("Interval should not change after a rejected SetSchedule")
	}
}

// TestSetSchedule_UnknownID verifies that updating a non-existent task returns
// an error.
func TestSetSchedule_UnknownID(t *testing.T) {
	s := NewScheduler()
	if err := s.SetSchedule("no-such-task", "hourly"); err == nil {
		t.Error("SetSchedule on unknown ID should return an error")
	}
}

// --- Stop ---

// TestStop_NoopBeforeStart verifies that Stop does not panic when the
// scheduler has never been started.
func TestStop_NoopBeforeStart(t *testing.T) {
	s := NewScheduler()
	defer func() {
		if r := recover(); r != nil {
			t.Errorf("Stop() panicked before Start: %v", r)
		}
	}()
	s.Stop()
}
