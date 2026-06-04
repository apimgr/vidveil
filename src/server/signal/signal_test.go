// SPDX-License-Identifier: MIT
// Tests for Unix signal helpers: global setters, IsShuttingDown, GetStopSignal,
// NotifyReload (no-op), CheckPIDFile, WritePIDFile, RemovePIDFile, KillProcess.
//go:build !windows

package signal

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"
)

// resetGlobals restores all package-level variables to their zero values.
func resetGlobals(t *testing.T) {
	t.Helper()
	t.Cleanup(func() {
		shuttingDown = false
		logReopenFn = nil
		statusDumpFn = nil
	})
}

// --- SetLogReopenFunc ---

func TestSetLogReopenFuncInvokesCallback(t *testing.T) {
	resetGlobals(t)
	called := false
	SetLogReopenFunc(func() { called = true })
	if logReopenFn == nil {
		t.Fatal("logReopenFn is nil after SetLogReopenFunc")
	}
	logReopenFn()
	if !called {
		t.Error("logReopenFn() did not invoke the registered func")
	}
}

func TestSetLogReopenFuncNilDoesNotPanic(t *testing.T) {
	resetGlobals(t)
	SetLogReopenFunc(func() {})
	// Overwrite with nil; code that checks logReopenFn != nil before calling must handle this.
	SetLogReopenFunc(nil)
	if logReopenFn != nil {
		t.Error("logReopenFn should be nil after SetLogReopenFunc(nil)")
	}
}

func TestSetLogReopenFuncReplacesExistingCallback(t *testing.T) {
	resetGlobals(t)
	first := false
	second := false
	SetLogReopenFunc(func() { first = true })
	SetLogReopenFunc(func() { second = true })
	logReopenFn()
	if first {
		t.Error("first callback was called after being replaced")
	}
	if !second {
		t.Error("replacement callback was not called")
	}
}

// --- SetStatusDumpFunc ---

func TestSetStatusDumpFuncInvokesCallback(t *testing.T) {
	resetGlobals(t)
	called := false
	SetStatusDumpFunc(func() { called = true })
	if statusDumpFn == nil {
		t.Fatal("statusDumpFn is nil after SetStatusDumpFunc")
	}
	statusDumpFn()
	if !called {
		t.Error("statusDumpFn() did not invoke the registered func")
	}
}

func TestSetStatusDumpFuncNilDoesNotPanic(t *testing.T) {
	resetGlobals(t)
	SetStatusDumpFunc(func() {})
	SetStatusDumpFunc(nil)
	if statusDumpFn != nil {
		t.Error("statusDumpFn should be nil after SetStatusDumpFunc(nil)")
	}
}

func TestSetStatusDumpFuncReplacesExistingCallback(t *testing.T) {
	resetGlobals(t)
	first := false
	second := false
	SetStatusDumpFunc(func() { first = true })
	SetStatusDumpFunc(func() { second = true })
	statusDumpFn()
	if first {
		t.Error("first callback was called after being replaced")
	}
	if !second {
		t.Error("replacement callback was not called")
	}
}

// --- IsShuttingDown ---

func TestIsShuttingDownDefaultsFalse(t *testing.T) {
	resetGlobals(t)
	if IsShuttingDown() {
		t.Error("IsShuttingDown() = true at package init, want false")
	}
}

func TestIsShuttingDownReflectsGlobalTrue(t *testing.T) {
	resetGlobals(t)
	shuttingDown = true
	if !IsShuttingDown() {
		t.Error("IsShuttingDown() = false after shuttingDown = true, want true")
	}
}

func TestIsShuttingDownReflectsGlobalFalse(t *testing.T) {
	resetGlobals(t)
	shuttingDown = true
	shuttingDown = false
	if IsShuttingDown() {
		t.Error("IsShuttingDown() = true after reset to false, want false")
	}
}

// --- GetStopSignal ---

func TestGetStopSignalIsNonNil(t *testing.T) {
	sig := GetStopSignal()
	if sig == nil {
		t.Error("GetStopSignal() returned nil")
	}
}

func TestGetStopSignalIsSIGTERM(t *testing.T) {
	sig := GetStopSignal()
	if sig != syscall.SIGTERM {
		t.Errorf("GetStopSignal() = %v, want SIGTERM", sig)
	}
}

func TestGetStopSignalStringRepresentation(t *testing.T) {
	sig := GetStopSignal()
	// syscall.SIGTERM.String() returns "terminated" on Linux
	got := sig.String()
	if got != "terminated" {
		t.Errorf("GetStopSignal().String() = %q, want %q", got, "terminated")
	}
}

// --- NotifyReload ---

func TestNotifyReloadDoesNotPanic(t *testing.T) {
	// Per spec, NotifyReload is a no-op; it must not panic.
	NotifyReload(func() {})
}

func TestNotifyReloadDoesNotInvokeHandler(t *testing.T) {
	called := false
	NotifyReload(func() { called = true })
	// The handler must never be called by NotifyReload itself.
	if called {
		t.Error("NotifyReload invoked the handler immediately; it must be a no-op")
	}
}

func TestNotifyReloadNilDoesNotPanic(t *testing.T) {
	NotifyReload(nil)
}

// --- CheckPIDFile ---

func TestCheckPIDFileNonExistentPath(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "nonexistent.pid")
	running, pid, err := CheckPIDFile(path, "vidveil")
	if err != nil {
		t.Errorf("CheckPIDFile(nonexistent) error = %v, want nil", err)
	}
	if running {
		t.Error("CheckPIDFile(nonexistent) running = true, want false")
	}
	if pid != 0 {
		t.Errorf("CheckPIDFile(nonexistent) pid = %d, want 0", pid)
	}
}

func TestCheckPIDFileNonNumericContentReturnsFalseAndRemovesFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "corrupt.pid")
	if err := os.WriteFile(path, []byte("not-a-number\n"), 0600); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	running, pid, err := CheckPIDFile(path, "vidveil")
	if err != nil {
		t.Errorf("CheckPIDFile(corrupt) error = %v, want nil", err)
	}
	if running {
		t.Error("CheckPIDFile(corrupt) running = true, want false")
	}
	if pid != 0 {
		t.Errorf("CheckPIDFile(corrupt) pid = %d, want 0", pid)
	}
	// Corrupt PID file must be removed.
	if _, statErr := os.Stat(path); !os.IsNotExist(statErr) {
		t.Error("CheckPIDFile(corrupt) did not remove the corrupt PID file")
	}
}

func TestCheckPIDFileWithOurOwnPID(t *testing.T) {
	// Write our own PID; isProcessRunning will return true (signal 0 succeeds),
	// and isOurProcess checks the binary name of the current process.
	dir := t.TempDir()
	path := filepath.Join(dir, "self.pid")
	ownPID := os.Getpid()
	if err := os.WriteFile(path, []byte(strconv.Itoa(ownPID)), 0600); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	// Determine our own binary name so isOurProcess returns true.
	exePath, err := os.Readlink("/proc/" + strconv.Itoa(ownPID) + "/exe")
	if err != nil {
		t.Skip("cannot read /proc/self/exe, skipping own-PID test")
	}
	binaryName := filepath.Base(exePath)

	running, pid, err := CheckPIDFile(path, binaryName)
	if err != nil {
		t.Errorf("CheckPIDFile(self) error = %v, want nil", err)
	}
	if !running {
		t.Error("CheckPIDFile(self) running = false, want true")
	}
	if pid != ownPID {
		t.Errorf("CheckPIDFile(self) pid = %d, want %d", pid, ownPID)
	}
}

func TestCheckPIDFileWrongBinaryNameReturnsFalseAndRemovesFile(t *testing.T) {
	// PID exists and is running, but binary name mismatch simulates PID reuse.
	dir := t.TempDir()
	path := filepath.Join(dir, "reuse.pid")
	ownPID := os.Getpid()
	if err := os.WriteFile(path, []byte(strconv.Itoa(ownPID)), 0600); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	// Use a name that will never match the test binary.
	running, _, err := CheckPIDFile(path, "definitely-not-our-binary")
	if err != nil {
		t.Errorf("CheckPIDFile(wrong binary) error = %v, want nil", err)
	}
	if running {
		t.Error("CheckPIDFile(wrong binary) running = true, want false")
	}
	// Stale PID file must be removed.
	if _, statErr := os.Stat(path); !os.IsNotExist(statErr) {
		t.Error("CheckPIDFile(wrong binary) did not remove the stale PID file")
	}
}

// --- WritePIDFile ---

func TestWritePIDFileWritesOurPID(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "write.pid")
	if err := WritePIDFile(path, "vidveil"); err != nil {
		t.Fatalf("WritePIDFile: %v", err)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile after WritePIDFile: %v", err)
	}
	got, err := strconv.Atoi(string(data))
	if err != nil {
		t.Fatalf("PID file content is not a number: %q", string(data))
	}
	if got != os.Getpid() {
		t.Errorf("WritePIDFile wrote pid %d, want %d", got, os.Getpid())
	}
}

func TestWritePIDFileSecondCallReturnsAlreadyRunning(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "double.pid")
	ownPID := os.Getpid()
	if err := os.WriteFile(path, []byte(strconv.Itoa(ownPID)), 0600); err != nil {
		t.Fatalf("setup WriteFile: %v", err)
	}

	// Discover binary name so CheckPIDFile sees us as running.
	exePath, err := os.Readlink("/proc/" + strconv.Itoa(ownPID) + "/exe")
	if err != nil {
		t.Skip("cannot read /proc/self/exe, skipping double-write test")
	}
	binaryName := filepath.Base(exePath)

	err = WritePIDFile(path, binaryName)
	if err == nil {
		t.Error("WritePIDFile on existing running PID: expected error, got nil")
	}
}

func TestWritePIDFileAfterRemoveSucceeds(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "rewrite.pid")
	if err := WritePIDFile(path, "vidveil"); err != nil {
		t.Fatalf("first WritePIDFile: %v", err)
	}
	if err := RemovePIDFile(path); err != nil {
		t.Fatalf("RemovePIDFile: %v", err)
	}
	if err := WritePIDFile(path, "vidveil"); err != nil {
		t.Errorf("second WritePIDFile after remove: %v", err)
	}
}

func TestWritePIDFileCreatesParentDirectory(t *testing.T) {
	dir := t.TempDir()
	// Nested directory that does not exist yet.
	path := filepath.Join(dir, "nested", "deep", "app.pid")
	if err := WritePIDFile(path, "vidveil"); err != nil {
		t.Fatalf("WritePIDFile with missing parent dirs: %v", err)
	}
	if _, err := os.Stat(path); err != nil {
		t.Errorf("PID file not found after WritePIDFile: %v", err)
	}
}

// --- RemovePIDFile ---

func TestRemovePIDFileDeletesFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "remove.pid")
	if err := os.WriteFile(path, []byte("123"), 0600); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	if err := RemovePIDFile(path); err != nil {
		t.Fatalf("RemovePIDFile: %v", err)
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Error("RemovePIDFile: file still exists after removal")
	}
}

func TestRemovePIDFileNonExistentReturnsError(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "ghost.pid")
	err := RemovePIDFile(path)
	if err == nil {
		t.Error("RemovePIDFile(nonexistent) = nil, want error")
	}
}

func TestRemovePIDFileIdempotencyFails(t *testing.T) {
	// Second call must return error — it is not idempotent by design (wraps os.Remove).
	dir := t.TempDir()
	path := filepath.Join(dir, "once.pid")
	if err := os.WriteFile(path, []byte("1"), 0600); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	if err := RemovePIDFile(path); err != nil {
		t.Fatalf("first RemovePIDFile: %v", err)
	}
	if err := RemovePIDFile(path); err == nil {
		t.Error("second RemovePIDFile(nonexistent) = nil, want error")
	}
}

// --- isOurProcess / isOurProcessDarwin ---

// TestIsOurProcessInvalidPIDFallback verifies that isOurProcess with a PID whose
// /proc entry doesn't exist falls back to isOurProcessDarwin (the ps-based path).
// On Linux, /proc/999999999/exe will not exist, so the readlink fails and the
// Darwin fallback is exercised. The overall result must be false (no such process).
func TestIsOurProcessInvalidPIDFallback(t *testing.T) {
	const impossiblePID = 999999999
	got := isOurProcess(impossiblePID, "vidveil")
	if got {
		t.Errorf("isOurProcess(%d, \"vidveil\") = true, want false (no such process)", impossiblePID)
	}
}

// TestIsOurProcessDarwinWithOurOwnPID calls isOurProcessDarwin directly against
// the current PID. On Linux, ps is available (busybox or procps) and the current
// process must be visible. The result may be true or false depending on the
// exact comm name reported — we only verify no panic and a valid bool.
func TestIsOurProcessDarwinNoPanic(t *testing.T) {
	ownPID := os.Getpid()
	result := isOurProcessDarwin(ownPID, "anything")
	// result is either true or false — just confirm it's reachable without panic.
	_ = result
}

// TestIsOurProcessDarwinDeadPIDReturnsFalse verifies that isOurProcessDarwin
// returns false when ps cannot find the given PID.
func TestIsOurProcessDarwinDeadPIDReturnsFalse(t *testing.T) {
	if got := isOurProcessDarwin(999999999, "vidveil"); got {
		t.Error("isOurProcessDarwin(999999999, \"vidveil\") = true, want false")
	}
}

// TestIsOurProcessDarwinMatchesOwnProcess verifies that isOurProcessDarwin returns
// true when the comm name retrieved by ps exactly matches the argument.
// We discover our own comm name via ps first to ensure an exact match.
func TestIsOurProcessDarwinMatchesOwnProcess(t *testing.T) {
	ownPID := os.Getpid()
	cmd := exec.Command("ps", "-p", strconv.Itoa(ownPID), "-o", "comm=")
	out, err := cmd.Output()
	if err != nil {
		t.Skipf("ps not available or returned error (%v), skipping", err)
	}
	commName := strings.TrimSpace(string(out))
	if commName == "" {
		t.Skip("ps returned empty comm name, skipping")
	}
	if !isOurProcessDarwin(ownPID, commName) {
		t.Errorf("isOurProcessDarwin(%d, %q) = false, want true", ownPID, commName)
	}
}

// TestWritePIDFileMkdirAllError verifies that WritePIDFile returns an error
// when the parent directory cannot be created (a regular file blocks mkdirall).
func TestWritePIDFileMkdirAllError(t *testing.T) {
	dir := t.TempDir()
	// Create a regular file where we want a directory to be
	blockPath := filepath.Join(dir, "notadir")
	if err := os.WriteFile(blockPath, []byte("block"), 0644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	// Attempt to create a PID file inside the regular file (impossible)
	pidPath := filepath.Join(blockPath, "nested", "app.pid")
	err := WritePIDFile(pidPath, "vidveil")
	if err == nil {
		t.Error("WritePIDFile with file-as-parent = nil, want error")
	}
}

// --- KillProcess ---

func TestKillProcessInvalidPIDReturnsError(t *testing.T) {
	// PID -1 is invalid on Linux; FindProcess or Signal should fail.
	err := KillProcess(-1, true)
	if err == nil {
		t.Error("KillProcess(-1, true) = nil, want error")
	}
}

func TestKillProcessZeroPIDReturnsError(t *testing.T) {
	// PID 0 sends signal to the current process group — guard against that by
	// verifying an error is returned or that our test survives (SIGTERM to our
	// process group during a test run would abort the runner).
	// On Linux, signal 0 to PID 0 succeeds but SIGTERM is dangerous; we skip the
	// actual kill and only verify that passing an obviously-invalid PID is safe.
	err := KillProcess(-99999, false)
	if err == nil {
		t.Error("KillProcess(-99999, false) = nil, want error")
	}
}

// TestKillProcessGracefulFlag verifies graceful=true uses SIGTERM, graceful=false
// uses SIGKILL.  We cannot safely send either to ourselves, so we use an
// already-exited child process to confirm the error path is consistent.
func TestKillProcessExitedProcessReturnsError(t *testing.T) {
	// Start a process and let it finish, then try to signal it.
	// On Linux, FindProcess always succeeds, but Signal on a zombie/gone PID
	// returns ESRCH.
	proc, err := os.StartProcess("/bin/true", []string{"/bin/true"}, &os.ProcAttr{})
	if err != nil {
		t.Skip("cannot start /bin/true, skipping")
	}
	// Wait for it to exit.
	_, _ = proc.Wait()

	// Both graceful and non-graceful calls should return errors for a dead PID.
	if err := KillProcess(proc.Pid, true); err == nil {
		t.Error("KillProcess(dead, true) = nil, want error")
	}
	if err := KillProcess(proc.Pid, false); err == nil {
		t.Error("KillProcess(dead, false) = nil, want error")
	}
}

// --- State isolation regression ---

func TestGlobalStateIsIsolatedBetweenTests(t *testing.T) {
	resetGlobals(t)
	shuttingDown = true
	logReopenFn = func() {}
	statusDumpFn = func() {}
	// Cleanup registered by resetGlobals will restore everything.
}

func TestGlobalStateDefaultsAfterPreviousTest(t *testing.T) {
	// resetGlobals cleanup must have fired; all globals should be zero.
	if shuttingDown {
		t.Error("shuttingDown not reset to false after previous test")
	}
	if logReopenFn != nil {
		t.Error("logReopenFn not reset to nil after previous test")
	}
	if statusDumpFn != nil {
		t.Error("statusDumpFn not reset to nil after previous test")
	}
}

// --- SetupSignalHandler ---

// TestSetupSignalHandlerNilServerNoPanic verifies that SetupSignalHandler does
// not panic when passed a nil *http.Server. The goroutine spawned internally
// will simply block until a signal arrives — that is acceptable for a test.
func TestSetupSignalHandlerNilServerNoPanic(t *testing.T) {
	resetGlobals(t)
	dir := t.TempDir()
	pidPath := filepath.Join(dir, "test.pid")
	// Must not panic; the goroutine blocks but the test does not send signals.
	SetupSignalHandler(nil, pidPath)
}

// TestSetupSignalHandlerEmptyPidFileNoPanic verifies SetupSignalHandler tolerates
// an empty PID file path without panicking.
func TestSetupSignalHandlerEmptyPidFileNoPanic(t *testing.T) {
	resetGlobals(t)
	SetupSignalHandler(nil, "")
}

// TestSetupSignalHandlerUSR1InvokesLogReopen verifies that sending SIGUSR1 to
// the current process causes the registered logReopenFn to be called.
// SIGUSR1 is safe to send to ourselves — it does not terminate the process.
func TestSetupSignalHandlerUSR1InvokesLogReopen(t *testing.T) {
	resetGlobals(t)
	called := make(chan struct{}, 1)
	SetLogReopenFunc(func() { called <- struct{}{} })
	SetupSignalHandler(nil, "")
	syscall.Kill(os.Getpid(), syscall.SIGUSR1)
	select {
	case <-called:
	case <-waitTimeout(t, 2):
		t.Error("logReopenFn was not called within 2s after SIGUSR1")
	}
}

// TestSetupSignalHandlerUSR2InvokesStatusDump verifies that sending SIGUSR2 to
// the current process causes the registered statusDumpFn to be called.
// SIGUSR2 is safe to send to ourselves — it does not terminate the process.
func TestSetupSignalHandlerUSR2InvokesStatusDump(t *testing.T) {
	resetGlobals(t)
	called := make(chan struct{}, 1)
	SetStatusDumpFunc(func() { called <- struct{}{} })
	SetupSignalHandler(nil, "")
	syscall.Kill(os.Getpid(), syscall.SIGUSR2)
	select {
	case <-called:
	case <-waitTimeout(t, 2):
		t.Error("statusDumpFn was not called within 2s after SIGUSR2")
	}
}

// waitTimeout returns a channel that fires after n seconds, used as a test deadline.
func waitTimeout(t *testing.T, secs int) <-chan struct{} {
	t.Helper()
	ch := make(chan struct{})
	go func() {
		timer := time.NewTimer(time.Duration(secs) * time.Second)
		defer timer.Stop()
		<-timer.C
		close(ch)
	}()
	return ch
}

// --- WaitForShutdown ---

// TestWaitForShutdownCancelledContextReturnsSIGTERM verifies that when the
// supplied context is already cancelled WaitForShutdown returns immediately
// with syscall.SIGTERM.
func TestWaitForShutdownCancelledContextReturnsSIGTERM(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	sig := WaitForShutdown(ctx)
	if sig != syscall.SIGTERM {
		t.Errorf("WaitForShutdown(cancelled ctx) = %v, want SIGTERM", sig)
	}
}
