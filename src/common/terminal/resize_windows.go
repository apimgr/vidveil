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

// WatchResize watches for terminal resize events on Windows
// Windows doesn't have SIGWINCH, so we poll for size changes
// Returns a channel that will be closed when the watcher stops
func WatchResize(handler ResizeHandler) chan struct{} {
	done := make(chan struct{})

	go func() {
		defer close(done)

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
			case <-done:
				return
			}
		}
	}()

	return done
}

// StopWatchResize stops watching for resize events
func StopWatchResize(done chan struct{}) {
	select {
	case <-done:
		// Already closed
	default:
		close(done)
	}
}
