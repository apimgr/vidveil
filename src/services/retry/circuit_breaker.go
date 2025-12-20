// SPDX-License-Identifier: MIT
// TEMPLATE.md PART 28: Circuit Breaker Pattern
package retry

import (
	"errors"
	"sync"
	"time"
)

// State represents the circuit breaker state
type State int

const (
	StateClosed   State = iota // Normal operation, requests pass through
	StateOpen                  // Circuit is open, requests fail immediately
	StateHalfOpen              // Testing if service recovered
)

func (s State) String() string {
	switch s {
	case StateClosed:
		return "closed"
	case StateOpen:
		return "open"
	case StateHalfOpen:
		return "half-open"
	default:
		return "unknown"
	}
}

// CircuitBreaker implements the circuit breaker pattern
type CircuitBreaker struct {
	mu sync.RWMutex

	name            string
	state           State
	failureCount    int
	successCount    int
	lastFailureTime time.Time

	// Configuration
	failureThreshold int           // Failures before opening circuit
	successThreshold int           // Successes in half-open before closing
	timeout          time.Duration // Time to wait before half-open
	onStateChange    func(name string, from, to State)
}

// CircuitBreakerConfig holds circuit breaker configuration
type CircuitBreakerConfig struct {
	Name             string
	FailureThreshold int           // Default: 5
	SuccessThreshold int           // Default: 2
	Timeout          time.Duration // Default: 30s
	OnStateChange    func(name string, from, to State)
}

// DefaultCircuitBreakerConfig returns default configuration
func DefaultCircuitBreakerConfig(name string) *CircuitBreakerConfig {
	return &CircuitBreakerConfig{
		Name:             name,
		FailureThreshold: 5,
		SuccessThreshold: 2,
		Timeout:          30 * time.Second,
	}
}

// NewCircuitBreaker creates a new circuit breaker
func NewCircuitBreaker(cfg *CircuitBreakerConfig) *CircuitBreaker {
	if cfg == nil {
		cfg = DefaultCircuitBreakerConfig("default")
	}

	return &CircuitBreaker{
		name:             cfg.Name,
		state:            StateClosed,
		failureThreshold: cfg.FailureThreshold,
		successThreshold: cfg.SuccessThreshold,
		timeout:          cfg.Timeout,
		onStateChange:    cfg.OnStateChange,
	}
}

// Errors
var (
	ErrCircuitOpen = errors.New("circuit breaker is open")
)

// Execute runs an operation through the circuit breaker
func (cb *CircuitBreaker) Execute(op func() error) error {
	if !cb.AllowRequest() {
		return ErrCircuitOpen
	}

	err := op()

	if err != nil {
		cb.RecordFailure()
	} else {
		cb.RecordSuccess()
	}

	return err
}

// ExecuteWithResult runs an operation that returns a result through the circuit breaker
func (cb *CircuitBreaker) ExecuteWithResult(op func() (interface{}, error)) (interface{}, error) {
	if !cb.AllowRequest() {
		return nil, ErrCircuitOpen
	}

	result, err := op()

	if err != nil {
		cb.RecordFailure()
	} else {
		cb.RecordSuccess()
	}

	return result, err
}

// AllowRequest checks if a request should be allowed
func (cb *CircuitBreaker) AllowRequest() bool {
	cb.mu.RLock()
	state := cb.state
	lastFailure := cb.lastFailureTime
	cb.mu.RUnlock()

	switch state {
	case StateClosed:
		return true

	case StateOpen:
		// Check if timeout has passed
		if time.Since(lastFailure) > cb.timeout {
			cb.transitionTo(StateHalfOpen)
			return true
		}
		return false

	case StateHalfOpen:
		return true

	default:
		return false
	}
}

// RecordSuccess records a successful operation
func (cb *CircuitBreaker) RecordSuccess() {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	switch cb.state {
	case StateClosed:
		// Reset failure count on success
		cb.failureCount = 0

	case StateHalfOpen:
		cb.successCount++
		if cb.successCount >= cb.successThreshold {
			cb.setState(StateClosed)
			cb.failureCount = 0
			cb.successCount = 0
		}
	}
}

// RecordFailure records a failed operation
func (cb *CircuitBreaker) RecordFailure() {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	cb.lastFailureTime = time.Now()

	switch cb.state {
	case StateClosed:
		cb.failureCount++
		if cb.failureCount >= cb.failureThreshold {
			cb.setState(StateOpen)
		}

	case StateHalfOpen:
		// Any failure in half-open goes back to open
		cb.setState(StateOpen)
		cb.successCount = 0
	}
}

// State returns the current state
func (cb *CircuitBreaker) State() State {
	cb.mu.RLock()
	defer cb.mu.RUnlock()
	return cb.state
}

// FailureCount returns the current failure count
func (cb *CircuitBreaker) FailureCount() int {
	cb.mu.RLock()
	defer cb.mu.RUnlock()
	return cb.failureCount
}

// Reset resets the circuit breaker to closed state
func (cb *CircuitBreaker) Reset() {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	cb.state = StateClosed
	cb.failureCount = 0
	cb.successCount = 0
}

// transitionTo transitions to a new state (thread-safe with lock upgrade)
func (cb *CircuitBreaker) transitionTo(newState State) {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	if cb.state != newState {
		cb.setState(newState)
	}
}

// setState sets the state and calls the callback (must hold lock)
func (cb *CircuitBreaker) setState(newState State) {
	oldState := cb.state
	cb.state = newState

	if cb.onStateChange != nil {
		// Call callback without lock to prevent deadlocks
		go cb.onStateChange(cb.name, oldState, newState)
	}
}

// CircuitBreakerRegistry manages multiple circuit breakers
type CircuitBreakerRegistry struct {
	mu       sync.RWMutex
	breakers map[string]*CircuitBreaker
	config   *CircuitBreakerConfig
}

// NewCircuitBreakerRegistry creates a new registry
func NewCircuitBreakerRegistry(defaultConfig *CircuitBreakerConfig) *CircuitBreakerRegistry {
	if defaultConfig == nil {
		defaultConfig = DefaultCircuitBreakerConfig("")
	}

	return &CircuitBreakerRegistry{
		breakers: make(map[string]*CircuitBreaker),
		config:   defaultConfig,
	}
}

// Get returns a circuit breaker by name, creating if necessary
func (r *CircuitBreakerRegistry) Get(name string) *CircuitBreaker {
	r.mu.RLock()
	cb, exists := r.breakers[name]
	r.mu.RUnlock()

	if exists {
		return cb
	}

	// Create new circuit breaker
	r.mu.Lock()
	defer r.mu.Unlock()

	// Double-check after acquiring write lock
	if cb, exists = r.breakers[name]; exists {
		return cb
	}

	cfg := &CircuitBreakerConfig{
		Name:             name,
		FailureThreshold: r.config.FailureThreshold,
		SuccessThreshold: r.config.SuccessThreshold,
		Timeout:          r.config.Timeout,
		OnStateChange:    r.config.OnStateChange,
	}

	cb = NewCircuitBreaker(cfg)
	r.breakers[name] = cb

	return cb
}

// GetAll returns all circuit breakers
func (r *CircuitBreakerRegistry) GetAll() map[string]*CircuitBreaker {
	r.mu.RLock()
	defer r.mu.RUnlock()

	result := make(map[string]*CircuitBreaker, len(r.breakers))
	for k, v := range r.breakers {
		result[k] = v
	}

	return result
}

// ResetAll resets all circuit breakers
func (r *CircuitBreakerRegistry) ResetAll() {
	r.mu.RLock()
	defer r.mu.RUnlock()

	for _, cb := range r.breakers {
		cb.Reset()
	}
}
