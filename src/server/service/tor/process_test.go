// SPDX-License-Identifier: MIT
package tor

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/cretz/bine/control"
)

func TestWaitControlPortAcceptsLateCompleteFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "control_port")
	if err := os.WriteFile(path, []byte("PORT=127.0.0.1:"), 0600); err != nil {
		t.Fatal(err)
	}
	go func() {
		time.Sleep(30 * time.Millisecond)
		_ = os.WriteFile(path, []byte("PORT=127.0.0.1:65535\n"), 0600)
	}()
	process := &torProcess{done: make(chan struct{}), output: &processOutput{}}
	address, err := waitControlPort(context.Background(), process, path, time.Second)
	if err != nil {
		t.Fatalf("waitControlPort() error = %v", err)
	}
	if address != "127.0.0.1:65535" {
		t.Fatalf("waitControlPort() = %q", address)
	}
}

func TestWaitControlPortRejectsInvalidAndPartialLines(t *testing.T) {
	for _, line := range []string{"", "PORT=127.0.0.1:0\n", "PORT=0.0.0.0:9051\n", "PORT=127.0.0.1:1234"} {
		t.Run(line, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "control_port")
			if err := os.WriteFile(path, []byte(line), 0600); err != nil {
				t.Fatal(err)
			}
			process := &torProcess{done: make(chan struct{}), output: &processOutput{}}
			ctx, cancel := context.WithTimeout(context.Background(), 25*time.Millisecond)
			defer cancel()
			if address, err := waitControlPort(ctx, process, path, 25*time.Millisecond); err == nil || address != "" {
				t.Fatalf("waitControlPort() = (%q, %v), want timeout", address, err)
			}
		})
	}
}

func TestWaitControlPortFailsWhenChildExits(t *testing.T) {
	done := make(chan struct{})
	close(done)
	process := &torProcess{done: done, err: errors.New("bad torrc"), output: &processOutput{}}
	if _, err := waitControlPort(context.Background(), process, filepath.Join(t.TempDir(), "missing"), time.Second); err == nil {
		t.Fatal("waitControlPort() succeeded after the Tor child exited")
	}
}

func TestSpawnTorRemovesStaleControlPortBeforeStart(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "control_port")
	if err := os.WriteFile(path, []byte("PORT=127.0.0.1:1234\n"), 0600); err != nil {
		t.Fatal(err)
	}
	_, err := spawnTor(context.Background(), filepath.Join(dir, "missing-tor"), "", dir)
	if err == nil {
		t.Fatal("spawnTor() unexpectedly started a missing binary")
	}
	if _, err := os.Stat(path); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("stale control_port still exists: %v", err)
	}
}

func TestWaitDescriptorUploadFiltersAddressAndReportsFailure(t *testing.T) {
	events := make(chan control.Event, 3)
	events <- &control.HSDescEvent{Action: "FAILED", Address: "another-service", Reason: "UPLOAD_REJECTED"}
	events <- &control.HSDescEvent{Action: "FAILED", Address: "testaddress", Reason: "UPLOAD_REJECTED"}
	events <- &control.HSDescEvent{Action: "UPLOADED", Address: "testaddress"}
	var failures []string
	err := waitDescriptorUpload(context.Background(), events, "testaddress.onion", func(reason string) {
		failures = append(failures, reason)
	})
	if err != nil {
		t.Fatalf("waitDescriptorUpload() error = %v", err)
	}
	if len(failures) != 1 || failures[0] != "UPLOAD_REJECTED" {
		t.Fatalf("failure callbacks = %#v", failures)
	}
}

func TestWaitDescriptorUploadHonorsTimeout(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
	defer cancel()
	if err := waitDescriptorUpload(ctx, make(chan control.Event), "testaddress.onion", nil); !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("waitDescriptorUpload() error = %v, want deadline exceeded", err)
	}
}
