// SPDX-License-Identifier: MIT
// AI.md PART 24: Windows service control must gracefully stop the application.
//go:build windows

package system

import (
	"context"
	"testing"
	"time"

	"golang.org/x/sys/windows/svc"
)

func TestWindowsServiceStopCancelsAndWaitsForApplication(t *testing.T) {
	started := make(chan struct{})
	service := &windowsService{runFunc: func(ctx context.Context) error {
		close(started)
		<-ctx.Done()
		return nil
	}}
	requests := make(chan svc.ChangeRequest)
	statuses := make(chan svc.Status, 4)
	done := make(chan struct {
		stop bool
		code uint32
	}, 1)
	go func() {
		stop, code := service.Execute(nil, requests, statuses)
		done <- struct {
			stop bool
			code uint32
		}{stop: stop, code: code}
	}()

	if got := <-statuses; got.State != svc.StartPending {
		t.Fatalf("first service state = %v, want start pending", got.State)
	}
	select {
	case <-started:
	case <-time.After(5 * time.Second):
		t.Fatal("application did not start")
	}
	if got := <-statuses; got.State != svc.Running {
		t.Fatalf("second service state = %v, want running", got.State)
	}
	requests <- svc.ChangeRequest{Cmd: svc.Stop}
	if got := <-statuses; got.State != svc.StopPending {
		t.Fatalf("third service state = %v, want stop pending", got.State)
	}
	select {
	case result := <-done:
		if result.stop || result.code != 0 {
			t.Fatalf("Execute() = (%t, %d), want (false, 0)", result.stop, result.code)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("service did not wait for the application to stop")
	}
}

func TestWindowsServiceRunErrorReportsFailure(t *testing.T) {
	service := &windowsService{runFunc: func(context.Context) error {
		return context.Canceled
	}}
	statuses := make(chan svc.Status, 4)
	stop, code := service.Execute(nil, make(chan svc.ChangeRequest), statuses)
	if !stop || code != 1 {
		t.Fatalf("Execute() = (%t, %d), want (true, 1)", stop, code)
	}
}
