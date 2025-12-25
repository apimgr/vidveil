// SPDX-License-Identifier: MIT
//go:build !windows

package signal

import (
	"context"
	"os"
	"os/signal"
	"syscall"
)

// WaitForShutdown blocks until a shutdown signal is received.
// Returns the signal that was received.
func WaitForShutdown(ctx context.Context) os.Signal {
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM, syscall.SIGHUP)

	select {
	case sig := <-quit:
		return sig
	case <-ctx.Done():
		return syscall.SIGTERM
	}
}

// NotifyReload sends a reload signal handler.
// On Unix, this listens for SIGHUP.
func NotifyReload(handler func()) {
	reload := make(chan os.Signal, 1)
	signal.Notify(reload, syscall.SIGHUP)

	go func() {
		for range reload {
			handler()
		}
	}()
}

// GetStopSignal returns the appropriate stop signal for this platform.
func GetStopSignal() os.Signal {
	return syscall.SIGTERM
}
