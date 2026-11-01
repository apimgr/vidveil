// SPDX-License-Identifier: MIT
// AI.md PART 24: Windows Service Integration
//go:build windows

package system

import (
	"context"
	"fmt"
	"os"
	"time"

	"golang.org/x/sys/windows/svc"
)

// windowsServiceName is the service name for Windows
const windowsServiceName = "vidveil"

// windowsService implements the Windows Service interface
type windowsService struct {
	runFunc func(context.Context) error
}

// RunAsWindowsService runs the application as a Windows service
// Per AI.md PART 24: Use golang.org/x/sys/windows/svc for Windows service integration
func RunAsWindowsService(runFunc func(context.Context) error) error {
	ws := &windowsService{runFunc: runFunc}
	return svc.Run(windowsServiceName, ws)
}

// Execute implements svc.Handler interface
// Per AI.md PART 24 specification
func (ws *windowsService) Execute(args []string, r <-chan svc.ChangeRequest, s chan<- svc.Status) (bool, uint32) {
	const cmdsAccepted = svc.AcceptStop | svc.AcceptShutdown

	// Notify SCM that we're starting
	s <- svc.Status{State: svc.StartPending}

	// Start the application in a goroutine
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	errChan := make(chan error, 1)
	go func() {
		errChan <- ws.runFunc(ctx)
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
				cancel()
				return ws.waitForShutdown(errChan, s)
			case svc.Interrogate:
				s <- c.CurrentStatus
			default:
				// Ignore unknown commands
			}
		case err := <-errChan:
			return windowsServiceExit(err)
		}
	}
}

func (ws *windowsService) waitForShutdown(errChan <-chan error, statuses chan<- svc.Status) (bool, uint32) {
	checkpoint := uint32(1)
	status := svc.Status{State: svc.StopPending, WaitHint: 30000, CheckPoint: checkpoint}
	statuses <- status
	ticker := time.NewTicker(10 * time.Second)
	defer ticker.Stop()
	for {
		select {
		case err := <-errChan:
			return windowsServiceExit(err)
		case <-ticker.C:
			checkpoint++
			status.CheckPoint = checkpoint
			statuses <- status
		}
	}
}

func windowsServiceExit(err error) (bool, uint32) {
	if err != nil {
		fmt.Fprintf(os.Stderr, "Service error: %v\n", err)
		return true, 1
	}
	return false, 0
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
