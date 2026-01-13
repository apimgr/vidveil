// SPDX-License-Identifier: MIT
// AI.md PART 25: Windows Service Integration
//go:build windows

package system

import (
	"fmt"
	"os"
	"time"

	"golang.org/x/sys/windows/svc"
)

// WindowsServiceName is the service name for Windows
const WindowsServiceName = "vidveil"

// windowsService implements the Windows Service interface
type windowsService struct {
	stopChan chan struct{}
	runFunc  func() error
}

// RunAsWindowsService runs the application as a Windows service
// Per AI.md PART 25: Use golang.org/x/sys/windows/svc for Windows service integration
func RunAsWindowsService(runFunc func() error) error {
	ws := &windowsService{
		stopChan: make(chan struct{}),
		runFunc:  runFunc,
	}
	return svc.Run(WindowsServiceName, ws)
}

// Execute implements svc.Handler interface
// Per AI.md PART 25 specification
func (ws *windowsService) Execute(args []string, r <-chan svc.ChangeRequest, s chan<- svc.Status) (bool, uint32) {
	const cmdsAccepted = svc.AcceptStop | svc.AcceptShutdown

	// Notify SCM that we're starting
	s <- svc.Status{State: svc.StartPending}

	// Start the application in a goroutine
	errChan := make(chan error, 1)
	go func() {
		errChan <- ws.runFunc()
	}()

	// Small delay to allow startup
	time.Sleep(100 * time.Millisecond)

	// Notify SCM that we're running
	s <- svc.Status{State: svc.Running, Accepts: cmdsAccepted}

	// Handle service control requests
	for {
		select {
		case c := <-r:
			switch c.Cmd {
			case svc.Stop, svc.Shutdown:
				s <- svc.Status{State: svc.StopPending}
				close(ws.stopChan)
				// Give the application time to shutdown gracefully
				time.Sleep(5 * time.Second)
				return false, 0
			case svc.Interrogate:
				s <- c.CurrentStatus
			default:
				// Ignore unknown commands
			}
		case err := <-errChan:
			if err != nil {
				// Log error and exit with failure
				fmt.Fprintf(os.Stderr, "Service error: %v\n", err)
				return true, 1
			}
			return false, 0
		}
	}
}

// IsWindowsService returns true if the current process is running as a Windows service
func IsWindowsService() bool {
	// Check if stdin is attached - services don't have stdin
	inService, err := svc.IsWindowsService()
	if err != nil {
		return false
	}
	return inService
}

// StopChannel returns the channel that signals service stop
func (ws *windowsService) StopChannel() <-chan struct{} {
	return ws.stopChan
}
