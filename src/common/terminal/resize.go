// SPDX-License-Identifier: MIT
//go:build !windows

// Package terminal provides terminal utilities
// See AI.md PART 7 for specification
package terminal

import (
	"os"
	"os/signal"
	"syscall"
)

// ResizeHandler is a callback for terminal resize events
type ResizeHandler func(size TerminalSize)

// WatchResize watches for terminal resize events (SIGWINCH).
// Returns a stop channel: close it (or call StopWatchResize) to stop watching.
// The goroutine exits as soon as the stop channel is closed.
func WatchResize(handler ResizeHandler) chan struct{} {
	stop := make(chan struct{})
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGWINCH)

	go func() {
		defer signal.Stop(sigChan)

		for {
			select {
			case <-sigChan:
				if handler != nil {
					handler(GetTerminalSize())
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
