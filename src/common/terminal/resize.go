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

// WatchResize watches for terminal resize events (SIGWINCH)
// Returns a channel that will be closed when the watcher stops
func WatchResize(handler ResizeHandler) chan struct{} {
	done := make(chan struct{})
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGWINCH)

	go func() {
		defer close(done)
		defer signal.Stop(sigChan)

		for {
			select {
			case <-sigChan:
				if handler != nil {
					handler(GetTerminalSize())
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
