// SPDX-License-Identifier: MIT
package main

import (
	"errors"
	"fmt"

	"github.com/apimgr/vidveil/src/server/signal"
)

type restoreServiceController interface {
	GetServiceStatus() (string, error)
	Stop() error
	Start() error
}

func restoreWithServiceLifecycle(controller restoreServiceController, pidFile, binaryName string, restore func() error) error {
	status, statusErr := controller.GetServiceStatus()
	running, pid, pidErr := signal.CheckPIDFile(pidFile, binaryName)
	if pidErr != nil {
		return fmt.Errorf("check whether the server is running: %w", pidErr)
	}
	if running && status != "running" {
		return fmt.Errorf("server process %d is running outside the detected service manager; stop it before restoring", pid)
	}
	if statusErr != nil && status == "running" {
		return fmt.Errorf("cannot verify the running service state: %w", statusErr)
	}
	managedServerRunning := status == "running"
	if managedServerRunning {
		if err := controller.Stop(); err != nil {
			return fmt.Errorf("stop server before restore: %w", err)
		}
	}

	restoreErr := restore()
	if !managedServerRunning {
		return restoreErr
	}
	if err := controller.Start(); err != nil {
		return errors.Join(restoreErr, fmt.Errorf("restore finished but restarting the server failed: %w", err))
	}
	return restoreErr
}
