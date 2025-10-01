package observability

import (
	"fmt"
	"sync"
	"time"
)

// CircuitBreakerState represents the state of a circuit breaker.
type CircuitBreakerState int

const (
	// StateClosed means the circuit breaker is closed and requests are allowed.
	StateClosed CircuitBreakerState = iota
	// StateOpen means the circuit breaker is open and requests are blocked.
	StateOpen
	// StateHalfOpen means the circuit breaker is half-open and testing requests.
	StateHalfOpen
)

// CircuitBreaker implements the circuit breaker pattern for handling failures.
type CircuitBreaker struct {
	name         string
	maxFailures  int
	timeout      time.Duration
	state        CircuitBreakerState
	failures     int
	lastFailure  time.Time
	mu           sync.RWMutex
	successCount int
	halfOpenMax  int
}

// NewCircuitBreaker creates a new circuit breaker.
func NewCircuitBreaker(name string, maxFailures int, timeout time.Duration) *CircuitBreaker {
	return &CircuitBreaker{
		name:        name,
		maxFailures: maxFailures,
		timeout:     timeout,
		state:       StateClosed,
		halfOpenMax: 3, // Allow 3 test requests in half-open state
	}
}

// Call executes a function with circuit breaker protection.
func (cb *CircuitBreaker) Call(fn func() error) error {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	// Check if we need to transition from open to half-open
	if cb.state == StateOpen && time.Since(cb.lastFailure) >= cb.timeout {
		cb.state = StateHalfOpen
		cb.successCount = 0
	}

	// Check if circuit breaker should allow the request
	if !cb.shouldAllowRequest() {
		RecordCircuitBreakerStatus(cb.name, "call", int(cb.state))
		return fmt.Errorf("circuit breaker %s is %s", cb.name, cb.stateString())
	}

	// Execute the function
	err := fn()

	// Update circuit breaker state based on result
	cb.updateState(err)

	// Record metrics
	RecordCircuitBreakerStatus(cb.name, "call", int(cb.state))

	return err
}

// shouldAllowRequest determines if a request should be allowed.
func (cb *CircuitBreaker) shouldAllowRequest() bool {
	switch cb.state {
	case StateClosed:
		return true
	case StateOpen:
		return false
	case StateHalfOpen:
		// Allow limited requests in half-open state
		return cb.successCount < cb.halfOpenMax
	default:
		return false
	}
}

// updateState updates the circuit breaker state based on the result.
func (cb *CircuitBreaker) updateState(err error) {
	if err != nil {
		cb.failures++
		cb.lastFailure = time.Now()

		// Check if we should open the circuit
		if cb.failures >= cb.maxFailures {
			cb.state = StateOpen
		}
	} else {
		// Success - only reset failure count if in closed state
		if cb.state == StateClosed {
			cb.failures = 0
		}

		// If in half-open state, increment success count
		if cb.state == StateHalfOpen {
			cb.successCount++
			// If we've had enough successes, close the circuit
			if cb.successCount >= cb.halfOpenMax {
				cb.state = StateClosed
				cb.successCount = 0
				cb.failures = 0
			}
		}
	}
}

// stateString returns a string representation of the current state.
func (cb *CircuitBreaker) stateString() string {
	switch cb.state {
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

// GetState returns the current state of the circuit breaker.
func (cb *CircuitBreaker) GetState() CircuitBreakerState {
	cb.mu.RLock()
	defer cb.mu.RUnlock()
	return cb.state
}

// GetFailures returns the current failure count.
func (cb *CircuitBreaker) GetFailures() int {
	cb.mu.RLock()
	defer cb.mu.RUnlock()
	return cb.failures
}

// Reset resets the circuit breaker to closed state.
func (cb *CircuitBreaker) Reset() {
	cb.mu.Lock()
	defer cb.mu.Unlock()
	cb.state = StateClosed
	cb.failures = 0
	cb.successCount = 0
}

// IsOpen returns true if the circuit breaker is open.
func (cb *CircuitBreaker) IsOpen() bool {
	cb.mu.RLock()
	defer cb.mu.RUnlock()
	return cb.state == StateOpen
}

// IsClosed returns true if the circuit breaker is closed.
func (cb *CircuitBreaker) IsClosed() bool {
	cb.mu.RLock()
	defer cb.mu.RUnlock()
	return cb.state == StateClosed
}

// IsHalfOpen returns true if the circuit breaker is half-open.
func (cb *CircuitBreaker) IsHalfOpen() bool {
	cb.mu.RLock()
	defer cb.mu.RUnlock()
	return cb.state == StateHalfOpen
}

// CircuitBreakerManager manages multiple circuit breakers.
type CircuitBreakerManager struct {
	breakers map[string]*CircuitBreaker
	mu       sync.RWMutex
}

// NewCircuitBreakerManager creates a new circuit breaker manager.
func NewCircuitBreakerManager() *CircuitBreakerManager {
	return &CircuitBreakerManager{
		breakers: make(map[string]*CircuitBreaker),
	}
}

// GetOrCreate gets an existing circuit breaker or creates a new one.
func (cbm *CircuitBreakerManager) GetOrCreate(name string, maxFailures int, timeout time.Duration) *CircuitBreaker {
	cbm.mu.Lock()
	defer cbm.mu.Unlock()

	if cb, exists := cbm.breakers[name]; exists {
		return cb
	}

	cb := NewCircuitBreaker(name, maxFailures, timeout)
	cbm.breakers[name] = cb
	return cb
}

// Get gets an existing circuit breaker.
func (cbm *CircuitBreakerManager) Get(name string) (*CircuitBreaker, bool) {
	cbm.mu.RLock()
	defer cbm.mu.RUnlock()
	cb, exists := cbm.breakers[name]
	return cb, exists
}

// GetAll returns all circuit breakers.
func (cbm *CircuitBreakerManager) GetAll() map[string]*CircuitBreaker {
	cbm.mu.RLock()
	defer cbm.mu.RUnlock()

	result := make(map[string]*CircuitBreaker)
	for name, cb := range cbm.breakers {
		result[name] = cb
	}
	return result
}

// ResetAll resets all circuit breakers.
func (cbm *CircuitBreakerManager) ResetAll() {
	cbm.mu.Lock()
	defer cbm.mu.Unlock()

	for _, cb := range cbm.breakers {
		cb.Reset()
	}
}

// Global circuit breaker manager instance
var globalCBM = NewCircuitBreakerManager()

// GetCircuitBreaker gets or creates a circuit breaker with the given name.
func GetCircuitBreaker(name string, maxFailures int, timeout time.Duration) *CircuitBreaker {
	return globalCBM.GetOrCreate(name, maxFailures, timeout)
}

// GetCircuitBreakerState gets the state of a circuit breaker.
func GetCircuitBreakerState(name string) (CircuitBreakerState, bool) {
	cb, exists := globalCBM.Get(name)
	if !exists {
		return StateClosed, false
	}
	return cb.GetState(), true
}

// IsCircuitBreakerOpen checks if a circuit breaker is open.
func IsCircuitBreakerOpen(name string) bool {
	cb, exists := globalCBM.Get(name)
	if !exists {
		return false
	}
	return cb.IsOpen()
}

// ResetCircuitBreaker resets a circuit breaker.
func ResetCircuitBreaker(name string) {
	cb, exists := globalCBM.Get(name)
	if exists {
		cb.Reset()
	}
}

// ResetAllCircuitBreakers resets all circuit breakers.
func ResetAllCircuitBreakers() {
	globalCBM.ResetAll()
}
