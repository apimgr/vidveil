// SPDX-License-Identifier: MIT
// Package scheduler provides background task scheduling
// This is a re-export package that wraps src/server/service/scheduler
package scheduler

import "github.com/apimgr/vidveil/src/server/service/scheduler"

// Re-export main types and functions from server/service/scheduler
type Scheduler = scheduler.Scheduler
type ScheduledTask = scheduler.ScheduledTask

var (
	NewScheduler = scheduler.NewScheduler
)
