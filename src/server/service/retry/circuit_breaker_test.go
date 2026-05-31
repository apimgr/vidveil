// SPDX-License-Identifier: MIT
package retry

import (
	"errors"
	"testing"
	"time"
)

func TestNewCircuitBreaker(t *testing.T) {
	cb := NewCircuitBreaker(nil)

	if cb.GetState() != CircuitBreakerStateClosed {
		t.Errorf("Expected CircuitBreakerStateClosed, got %v", cb.GetState())
	}

	if cb.name != "default" {
		t.Errorf("Expected name 'default', got '%s'", cb.name)
	}
}

func TestCircuitBreakerConfig(t *testing.T) {
	cfg := &CircuitBreakerConfig{
		Name:             "test",
		FailureThreshold: 3,
		SuccessThreshold: 1,
		Timeout:          10 * time.Second,
	}

	cb := NewCircuitBreaker(cfg)

	if cb.name != "test" {
		t.Errorf("Expected name 'test', got '%s'", cb.name)
	}

	if cb.failureThreshold != 3 {
		t.Errorf("Expected failureThreshold 3, got %d", cb.failureThreshold)
	}

	if cb.successThreshold != 1 {
		t.Errorf("Expected successThreshold 1, got %d", cb.successThreshold)
	}

	if cb.timeout != 10*time.Second {
		t.Errorf("Expected timeout 10s, got %v", cb.timeout)
	}
}

func TestCircuitBreakerAllowRequest(t *testing.T) {
	cfg := &CircuitBreakerConfig{
		Name:             "test",
		FailureThreshold: 3,
		SuccessThreshold: 1,
		Timeout:          100 * time.Millisecond,
	}

	cb := NewCircuitBreaker(cfg)

	// Initially closed, requests allowed
	if !cb.AllowRequest() {
		t.Error("Closed circuit should allow requests")
	}

	// Record failures to open circuit
	cb.RecordFailure()
	cb.RecordFailure()
	cb.RecordFailure()

	// Circuit should now be open
	if cb.GetState() != CircuitBreakerStateOpen {
		t.Errorf("Expected CircuitBreakerStateOpen after threshold, got %v", cb.GetState())
	}

	// Open circuit should block requests
	if cb.AllowRequest() {
		t.Error("Open circuit should not allow requests")
	}
}

func TestCircuitBreakerHalfOpen(t *testing.T) {
	cfg := &CircuitBreakerConfig{
		Name:             "test",
		FailureThreshold: 1,
		SuccessThreshold: 2,
		Timeout:          50 * time.Millisecond,
	}

	cb := NewCircuitBreaker(cfg)

	// Open the circuit
	cb.RecordFailure()

	if cb.GetState() != CircuitBreakerStateOpen {
		t.Errorf("Expected CircuitBreakerStateOpen, got %v", cb.GetState())
	}

	// Wait for timeout
	time.Sleep(60 * time.Millisecond)

	// Request should be allowed and transition to half-open
	if !cb.AllowRequest() {
		t.Error("Should allow request after timeout (half-open)")
	}

	if cb.GetState() != CircuitBreakerStateHalfOpen {
		t.Errorf("Expected CircuitBreakerStateHalfOpen, got %v", cb.GetState())
	}
}

func TestCircuitBreakerRecovery(t *testing.T) {
	cfg := &CircuitBreakerConfig{
		Name:             "test",
		FailureThreshold: 1,
		SuccessThreshold: 2,
		Timeout:          50 * time.Millisecond,
	}

	cb := NewCircuitBreaker(cfg)

	// Open the circuit
	cb.RecordFailure()

	// Wait for timeout
	time.Sleep(60 * time.Millisecond)

	// Transition to half-open
	cb.AllowRequest()

	// Record successes to close circuit
	cb.RecordSuccess()
	cb.RecordSuccess()

	if cb.GetState() != CircuitBreakerStateClosed {
		t.Errorf("Expected CircuitBreakerStateClosed after recovery, got %v", cb.GetState())
	}

	// Requests should be allowed again
	if !cb.AllowRequest() {
		t.Error("Recovered circuit should allow requests")
	}
}

func TestCircuitBreakerHalfOpenFailure(t *testing.T) {
	cfg := &CircuitBreakerConfig{
		Name:             "test",
		FailureThreshold: 1,
		SuccessThreshold: 2,
		Timeout:          50 * time.Millisecond,
	}

	cb := NewCircuitBreaker(cfg)

	// Open the circuit
	cb.RecordFailure()

	// Wait for timeout
	time.Sleep(60 * time.Millisecond)

	// Transition to half-open
	cb.AllowRequest()

	// Record one success
	cb.RecordSuccess()

	// Then a failure should reopen
	cb.RecordFailure()

	if cb.GetState() != CircuitBreakerStateOpen {
		t.Errorf("Expected CircuitBreakerStateOpen after half-open failure, got %v", cb.GetState())
	}
}

func TestCircuitBreakerReset(t *testing.T) {
	cfg := &CircuitBreakerConfig{
		Name:             "test",
		FailureThreshold: 1,
		SuccessThreshold: 1,
		// Long timeout for testing
		Timeout: 1 * time.Hour,
	}

	cb := NewCircuitBreaker(cfg)

	// Open the circuit
	cb.RecordFailure()

	if cb.GetState() != CircuitBreakerStateOpen {
		t.Errorf("Expected CircuitBreakerStateOpen, got %v", cb.GetState())
	}

	// Reset
	cb.Reset()

	if cb.GetState() != CircuitBreakerStateClosed {
		t.Errorf("Expected CircuitBreakerStateClosed after reset, got %v", cb.GetState())
	}

	if cb.FailureCount() != 0 {
		t.Errorf("Expected failure count 0 after reset, got %d", cb.FailureCount())
	}
}

func TestCircuitBreakerExecute(t *testing.T) {
	cfg := &CircuitBreakerConfig{
		Name:             "test",
		FailureThreshold: 2,
		SuccessThreshold: 1,
		Timeout:          100 * time.Millisecond,
	}

	cb := NewCircuitBreaker(cfg)

	// Successful execution
	err := cb.Execute(func() error {
		return nil
	})
	if err != nil {
		t.Errorf("Expected no error, got %v", err)
	}

	// Failed execution
	testErr := errors.New("test error")
	err = cb.Execute(func() error {
		return testErr
	})
	if err != testErr {
		t.Errorf("Expected test error, got %v", err)
	}

	// Another failure to open
	cb.Execute(func() error {
		return testErr
	})

	// Circuit open
	err = cb.Execute(func() error {
		return nil
	})
	if err != ErrCircuitOpen {
		t.Errorf("Expected ErrCircuitOpen, got %v", err)
	}
}

func TestCircuitBreakerExecuteWithResult(t *testing.T) {
	cfg := &CircuitBreakerConfig{
		Name:             "test",
		FailureThreshold: 5,
		SuccessThreshold: 1,
		Timeout:          100 * time.Millisecond,
	}

	cb := NewCircuitBreaker(cfg)

	// Successful execution
	result, err := cb.ExecuteWithResult(func() (interface{}, error) {
		return "success", nil
	})

	if err != nil {
		t.Errorf("Expected no error, got %v", err)
	}

	if result != "success" {
		t.Errorf("Expected 'success', got %v", result)
	}
}

func TestCircuitBreakerStateString(t *testing.T) {
	tests := []struct {
		state    CircuitBreakerState
		expected string
	}{
		{CircuitBreakerStateClosed, "closed"},
		{CircuitBreakerStateOpen, "open"},
		{CircuitBreakerStateHalfOpen, "half-open"},
		{CircuitBreakerState(99), "unknown"},
	}

	for _, tt := range tests {
		if tt.state.String() != tt.expected {
			t.Errorf("CircuitBreakerState(%d).String() = '%s', want '%s'", tt.state, tt.state.String(), tt.expected)
		}
	}
}

func TestCircuitBreakerRegistry(t *testing.T) {
	cfg := &CircuitBreakerConfig{
		FailureThreshold: 5,
		SuccessThreshold: 2,
		Timeout:          30 * time.Second,
	}

	registry := NewCircuitBreakerRegistry(cfg)

	// Get creates a new breaker
	cb1 := registry.Get("engine1")
	if cb1 == nil {
		t.Error("Expected circuit breaker, got nil")
	}

	if cb1.name != "engine1" {
		t.Errorf("Expected name 'engine1', got '%s'", cb1.name)
	}

	// Get same breaker
	cb1Again := registry.Get("engine1")
	if cb1 != cb1Again {
		t.Error("Expected same circuit breaker instance")
	}

	// Get different breaker
	cb2 := registry.Get("engine2")
	if cb1 == cb2 {
		t.Error("Expected different circuit breaker for different name")
	}
}

func TestCircuitBreakerRegistryGetAll(t *testing.T) {
	registry := NewCircuitBreakerRegistry(nil)

	registry.Get("engine1")
	registry.Get("engine2")
	registry.Get("engine3")

	all := registry.GetAll()

	if len(all) != 3 {
		t.Errorf("Expected 3 breakers, got %d", len(all))
	}

	if _, ok := all["engine1"]; !ok {
		t.Error("Expected engine1 in registry")
	}
}

func TestCircuitBreakerRegistryResetAll(t *testing.T) {
	cfg := &CircuitBreakerConfig{
		FailureThreshold: 1,
		SuccessThreshold: 1,
		Timeout:          1 * time.Hour,
	}

	registry := NewCircuitBreakerRegistry(cfg)

	cb1 := registry.Get("engine1")
	cb2 := registry.Get("engine2")

	// Open both circuits
	cb1.RecordFailure()
	cb2.RecordFailure()

	if cb1.GetState() != CircuitBreakerStateOpen {
		t.Error("cb1 should be open")
	}
	if cb2.GetState() != CircuitBreakerStateOpen {
		t.Error("cb2 should be open")
	}

	// Reset all
	registry.ResetAll()

	if cb1.GetState() != CircuitBreakerStateClosed {
		t.Error("cb1 should be closed after reset")
	}
	if cb2.GetState() != CircuitBreakerStateClosed {
		t.Error("cb2 should be closed after reset")
	}
}

func TestDefaultCircuitBreakerConfig(t *testing.T) {
	cfg := DefaultCircuitBreakerConfig("test-engine")

	if cfg.Name != "test-engine" {
		t.Errorf("Expected name 'test-engine', got '%s'", cfg.Name)
	}

	if cfg.FailureThreshold != 5 {
		t.Errorf("Expected FailureThreshold 5, got %d", cfg.FailureThreshold)
	}

	if cfg.SuccessThreshold != 2 {
		t.Errorf("Expected SuccessThreshold 2, got %d", cfg.SuccessThreshold)
	}

	if cfg.Timeout != 30*time.Second {
		t.Errorf("Expected Timeout 30s, got %v", cfg.Timeout)
	}
}

func TestCircuitBreakerSuccessResetFailureCount(t *testing.T) {
	cfg := &CircuitBreakerConfig{
		Name:             "test",
		FailureThreshold: 5,
		SuccessThreshold: 1,
		Timeout:          100 * time.Millisecond,
	}

	cb := NewCircuitBreaker(cfg)

	// Record some failures
	cb.RecordFailure()
	cb.RecordFailure()

	if cb.FailureCount() != 2 {
		t.Errorf("Expected failure count 2, got %d", cb.FailureCount())
	}

	// Success should reset failure count
	cb.RecordSuccess()

	if cb.FailureCount() != 0 {
		t.Errorf("Expected failure count 0 after success, got %d", cb.FailureCount())
	}
}

// TestCircuitBreakerLastFailureTime covers the LastFailureTime accessor.
func TestCircuitBreakerLastFailureTime(t *testing.T) {
	cb := NewCircuitBreaker(nil)

	// Before any failure, zero time
	if !cb.LastFailureTime().IsZero() {
		t.Error("Expected zero LastFailureTime before any failure")
	}

	before := time.Now()
	cb.RecordFailure()
	after := time.Now()

	ft := cb.LastFailureTime()
	if ft.Before(before) || ft.After(after) {
		t.Errorf("LastFailureTime %v not in range [%v, %v]", ft, before, after)
	}
}

// TestCircuitBreakerAllowRequestOpenNotElapsed covers the AllowRequest path where the
// circuit is open but the timeout has NOT yet elapsed (must return false without
// transitioning to half-open).
func TestCircuitBreakerAllowRequestOpenNotElapsed(t *testing.T) {
	cfg := &CircuitBreakerConfig{
		Name:             "test",
		FailureThreshold: 1,
		SuccessThreshold: 1,
		// Long timeout so it never elapses in this test
		Timeout: 1 * time.Hour,
	}

	cb := NewCircuitBreaker(cfg)
	cb.RecordFailure()

	if cb.GetState() != CircuitBreakerStateOpen {
		t.Fatal("Expected open state after failure")
	}

	// Timeout has not elapsed; AllowRequest must return false and leave state open
	if cb.AllowRequest() {
		t.Error("Open circuit with non-elapsed timeout must not allow requests")
	}

	if cb.GetState() != CircuitBreakerStateOpen {
		t.Errorf("State should remain open, got %v", cb.GetState())
	}
}

// TestCircuitBreakerExecuteWithResultOpen covers the early-return path of
// ExecuteWithResult when the circuit is open.
func TestCircuitBreakerExecuteWithResultOpen(t *testing.T) {
	cfg := &CircuitBreakerConfig{
		Name:             "test",
		FailureThreshold: 1,
		SuccessThreshold: 1,
		Timeout:          1 * time.Hour,
	}

	cb := NewCircuitBreaker(cfg)
	cb.RecordFailure()

	result, err := cb.ExecuteWithResult(func() (interface{}, error) {
		return "should not run", nil
	})

	if !errors.Is(err, ErrCircuitOpen) {
		t.Errorf("Expected ErrCircuitOpen, got %v", err)
	}

	if result != nil {
		t.Errorf("Expected nil result, got %v", result)
	}
}

// TestCircuitBreakerExecuteWithResultError covers the RecordFailure path inside
// ExecuteWithResult when the operation returns a non-nil error.
func TestCircuitBreakerExecuteWithResultError(t *testing.T) {
	cfg := &CircuitBreakerConfig{
		Name:             "test",
		FailureThreshold: 3,
		SuccessThreshold: 1,
		Timeout:          100 * time.Millisecond,
	}

	cb := NewCircuitBreaker(cfg)
	testErr := errors.New("operation failed")

	result, err := cb.ExecuteWithResult(func() (interface{}, error) {
		return nil, testErr
	})

	if !errors.Is(err, testErr) {
		t.Errorf("Expected testErr, got %v", err)
	}

	if result != nil {
		t.Errorf("Expected nil result, got %v", result)
	}

	// Failure must have been recorded
	if cb.FailureCount() != 1 {
		t.Errorf("Expected failure count 1, got %d", cb.FailureCount())
	}
}

// TestCircuitBreakerSetStateWithCallback covers the setState path where onStateChange
// is non-nil, verifying the callback fires without deadlock.
func TestCircuitBreakerSetStateWithCallback(t *testing.T) {
	done := make(chan struct{})
	var cbFrom, cbTo CircuitBreakerState
	var cbName string

	cfg := &CircuitBreakerConfig{
		Name:             "cb-callback",
		FailureThreshold: 1,
		SuccessThreshold: 1,
		Timeout:          100 * time.Millisecond,
		OnStateChange: func(name string, from, to CircuitBreakerState) {
			cbName = name
			cbFrom = from
			cbTo = to
			close(done)
		},
	}

	cb := NewCircuitBreaker(cfg)

	// Trigger a state change: closed -> open
	cb.RecordFailure()

	select {
	case <-done:
	case <-time.After(500 * time.Millisecond):
		t.Fatal("onStateChange callback was never called")
	}

	if cbName != "cb-callback" {
		t.Errorf("Expected callback name 'cb-callback', got %q", cbName)
	}

	if cbFrom != CircuitBreakerStateClosed {
		t.Errorf("Expected from=closed, got %v", cbFrom)
	}

	if cbTo != CircuitBreakerStateOpen {
		t.Errorf("Expected to=open, got %v", cbTo)
	}
}

// TestCircuitBreakerTransitionToSameState covers the guard in transitionTo that
// skips setState when the new state equals the current state.
func TestCircuitBreakerTransitionToSameState(t *testing.T) {
	callbacks := 0
	cfg := &CircuitBreakerConfig{
		Name:             "no-dup",
		FailureThreshold: 5,
		SuccessThreshold: 2,
		Timeout:          50 * time.Millisecond,
		OnStateChange: func(_ string, _, _ CircuitBreakerState) {
			callbacks++
		},
	}

	cb := NewCircuitBreaker(cfg)

	// Trigger open state
	for i := 0; i < 5; i++ {
		cb.RecordFailure()
	}

	// Wait for any async callback
	time.Sleep(20 * time.Millisecond)
	countAfterOpen := callbacks

	// Wait for timeout to elapse so AllowRequest transitions to half-open
	time.Sleep(60 * time.Millisecond)

	// First AllowRequest transitions closed->open->half-open; second call must not
	// fire setState again (transitionTo guard prevents it)
	cb.AllowRequest()
	cb.AllowRequest()

	// Allow any async callbacks to fire
	time.Sleep(20 * time.Millisecond)

	// Only one additional state change (open->half-open) should have been recorded
	newCallbacks := callbacks - countAfterOpen
	if newCallbacks != 1 {
		t.Errorf("Expected exactly 1 new callback for open->half-open, got %d", newCallbacks)
	}
}

// TestCircuitBreakerRegistryGetDoubleCheck covers the double-check-after-write-lock
// branch in Registry.Get. The registryGetHook fires between the RLock miss and the
// WLock acquisition, letting a concurrent goroutine insert the key. When Get then
// acquires the write lock, the double-check finds the key and returns it directly.
func TestCircuitBreakerRegistryGetDoubleCheck(t *testing.T) {
	registry := NewCircuitBreakerRegistry(nil)
	const key = "double-check-key"

	inserted := make(chan *CircuitBreaker, 1)

	// The hook runs after RLock misses but before WLock — insert the key here.
	registryGetHook = func() {
		registry.mu.Lock()
		cb := NewCircuitBreaker(DefaultCircuitBreakerConfig(key))
		registry.breakers[key] = cb
		registry.mu.Unlock()
		inserted <- cb
	}
	t.Cleanup(func() { registryGetHook = nil })

	got := registry.Get(key)
	prebuilt := <-inserted

	if got != prebuilt {
		t.Error("double-check branch must return the instance inserted by the hook")
	}
}

// TestCircuitBreakerAllowRequestDefaultBranch covers the default (unknown state) branch
// in AllowRequest which must return false.
func TestCircuitBreakerAllowRequestDefaultBranch(t *testing.T) {
	cb := NewCircuitBreaker(nil)

	// Force an invalid state value directly (bypasses normal state machine)
	cb.mu.Lock()
	cb.state = CircuitBreakerState(99)
	cb.mu.Unlock()

	if cb.AllowRequest() {
		t.Error("Unknown state must not allow requests")
	}
}
