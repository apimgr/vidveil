// SPDX-License-Identifier: MIT
package main

import (
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strconv"
	"testing"
)

type fakeRestoreServiceController struct {
	status    string
	statusErr error
	stopErr   error
	startErr  error
	events    []string
}

func (f *fakeRestoreServiceController) GetServiceStatus() (string, error) {
	f.events = append(f.events, "status")
	return f.status, f.statusErr
}

func (f *fakeRestoreServiceController) Stop() error {
	f.events = append(f.events, "stop")
	return f.stopErr
}

func (f *fakeRestoreServiceController) Start() error {
	f.events = append(f.events, "start")
	return f.startErr
}

func TestRestoreWithServiceLifecycleManagedRunning(t *testing.T) {
	controller := &fakeRestoreServiceController{status: "running"}
	controllerEvents := &controller.events
	err := restoreWithServiceLifecycle(controller, filepath.Join(t.TempDir(), "missing.pid"), "vidveil", func() error {
		*controllerEvents = append(*controllerEvents, "restore")
		return nil
	})
	if err != nil {
		t.Fatalf("restoreWithServiceLifecycle: %v", err)
	}
	want := []string{"status", "stop", "restore", "start"}
	if !reflect.DeepEqual(controller.events, want) {
		t.Fatalf("events = %v, want %v", controller.events, want)
	}
}

func TestRestoreWithServiceLifecycleStopped(t *testing.T) {
	controller := &fakeRestoreServiceController{status: "stopped"}
	restored := false
	err := restoreWithServiceLifecycle(controller, filepath.Join(t.TempDir(), "missing.pid"), "vidveil", func() error {
		restored = true
		return nil
	})
	if err != nil {
		t.Fatalf("restoreWithServiceLifecycle: %v", err)
	}
	if !restored {
		t.Fatal("restore callback was not called")
	}
	if !reflect.DeepEqual(controller.events, []string{"status"}) {
		t.Fatalf("events = %v, want [status]", controller.events)
	}
}

func TestRestoreWithServiceLifecycleStopsOnControllerError(t *testing.T) {
	controller := &fakeRestoreServiceController{status: "unknown", statusErr: errors.New("no service manager")}
	if err := restoreWithServiceLifecycle(controller, filepath.Join(t.TempDir(), "missing.pid"), "vidveil", func() error { return nil }); err != nil {
		t.Fatalf("offline restore with unavailable service manager: %v", err)
	}
	if !reflect.DeepEqual(controller.events, []string{"status"}) {
		t.Fatalf("events = %v, want [status]", controller.events)
	}
}

func TestRestoreWithServiceLifecycleRestartsAfterRestoreFailure(t *testing.T) {
	controller := &fakeRestoreServiceController{status: "running"}
	restoreErr := errors.New("invalid archive")
	err := restoreWithServiceLifecycle(controller, filepath.Join(t.TempDir(), "missing.pid"), "vidveil", func() error {
		return restoreErr
	})
	if !errors.Is(err, restoreErr) {
		t.Fatalf("error = %v, want restore error", err)
	}
	want := []string{"status", "stop", "start"}
	if !reflect.DeepEqual(controller.events, want) {
		t.Fatalf("events = %v, want %v", controller.events, want)
	}
}

func TestRestoreWithServiceLifecycleDoesNotRestoreWhenStopFails(t *testing.T) {
	controller := &fakeRestoreServiceController{status: "running", stopErr: errors.New("permission denied")}
	restored := false
	err := restoreWithServiceLifecycle(controller, filepath.Join(t.TempDir(), "missing.pid"), "vidveil", func() error {
		restored = true
		return nil
	})
	if err == nil {
		t.Fatal("restore succeeded despite failure to stop the running server")
	}
	if restored {
		t.Fatal("restore ran after service stop failed")
	}
	want := []string{"status", "stop"}
	if !reflect.DeepEqual(controller.events, want) {
		t.Fatalf("events = %v, want %v", controller.events, want)
	}
}

func TestRestoreWithServiceLifecycleRejectsUnmanagedRunningServer(t *testing.T) {
	pidFile := filepath.Join(t.TempDir(), "vidveil.pid")
	if err := os.WriteFile(pidFile, []byte(strconv.Itoa(os.Getpid())), 0600); err != nil {
		t.Fatalf("write current pid: %v", err)
	}
	controller := &fakeRestoreServiceController{status: "unknown", statusErr: errors.New("no service manager")}
	restored := false
	err := restoreWithServiceLifecycle(controller, pidFile, filepath.Base(os.Args[0]), func() error {
		restored = true
		return nil
	})
	if err == nil {
		t.Fatal("restore succeeded over an unmanaged running server")
	}
	if restored {
		t.Fatal("restore ran while the unmanaged server process was active")
	}
}
