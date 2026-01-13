// SPDX-License-Identifier: MIT
// AI.md PART 8: Daemonization (Windows)
//go:build windows

package daemon

import (
	"fmt"
	"os"
)

// Daemonize on Windows is not supported per AI.md PART 8 lines 7939-7955
// Windows does not support traditional Unix daemonization
// Instead, use Windows Services (--service install/start)
func Daemonize() error {
	// On Windows, --daemon flag is ignored with a warning
	fmt.Fprintln(os.Stderr, "Warning: --daemon is not supported on Windows")
	fmt.Fprintln(os.Stderr, "Use --service --install && --service start for Windows Service")
	// Continue in foreground
	return nil
}
