// SPDX-License-Identifier: MIT
//go:build windows

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
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)

	select {
	case sig := <-quit:
		return sig
	case <-ctx.Done():
		return syscall.SIGTERM
	}
}

// NotifyReload registers a reload signal handler.
// On Windows, there is no SIGHUP equivalent, so this is a no-op.
func NotifyReload(handler func()) {
	// Windows does not support SIGHUP
	// Reload must be triggered via API or service manager
}

// GetStopSignal returns the appropriate stop signal for this platform.
func GetStopSignal() os.Signal {
	return syscall.SIGTERM
}
