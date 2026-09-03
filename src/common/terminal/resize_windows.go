// SPDX-License-Identifier: MIT
//go:build windows

// Package terminal provides terminal utilities
// See AI.md PART 7 for specification
package terminal

import (
	"time"
)

// ResizeHandler is a callback for terminal resize events
type ResizeHandler func(size TerminalSize)

// WatchResize watches for terminal resize events on Windows.
// Windows doesn't have SIGWINCH, so we poll for size changes.
// Returns a stop channel: close it (or call StopWatchResize) to stop watching.
func WatchResize(handler ResizeHandler) chan struct{} {
	stop := make(chan struct{})

	go func() {
		lastSize := GetTerminalSize()
		ticker := time.NewTicker(500 * time.Millisecond)
		defer ticker.Stop()

		for {
			select {
			case <-ticker.C:
				newSize := GetTerminalSize()
				if newSize.Cols != lastSize.Cols || newSize.Rows != lastSize.Rows {
					lastSize = newSize
					if handler != nil {
						handler(newSize)
					}
				}
			case <-stop:
				return
			}
		}
	}()

	return stop
}

// StopWatchResize stops watching for resize events.
// Safe to call multiple times; subsequent calls are no-ops.
func StopWatchResize(done chan struct{}) {
	select {
	case <-done:
		// Already closed
	default:
		close(done)
	}
}
