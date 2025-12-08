// Package observability provides circuit breaker implementation for external connections.
package observability

import (
	"log/slog"
	"sync"
	"time"
)

// CircuitBreakerState represents the state of the circuit breaker
type CircuitBreakerState int

const (
	// StateClosed indicates the circuit is closed and operations are allowed.
	StateClosed CircuitBreakerState = iota
	// StateOpen indicates the circuit is open and operations are blocked for a timeout period.
	StateOpen
	// StateHalfOpen indicates a trial state where limited operations are allowed to test recovery.
	StateHalfOpen
)

func (s CircuitBreakerState) String() string {
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

	// Configuration
	maxFailures      int
	timeout          time.Duration
	successThreshold float64

	// State
	state           CircuitBreakerState
	failureCount    int
	successCount    int
	lastFailureTime time.Time

	// Metrics
	totalRequests  int64
	totalFailures  int64
	totalSuccesses int64
	stateChanges   int64
}

// NewCircuitBreaker creates a new circuit breaker
func NewCircuitBreaker(maxFailures int, timeout time.Duration, successThreshold float64) *CircuitBreaker {
	return &CircuitBreaker{
		maxFailures:      maxFailures,
		timeout:          timeout,
		successThreshold: successThreshold,
		state:            StateClosed,
	}
}

// CanExecute returns true if the circuit breaker allows execution
func (cb *CircuitBreaker) CanExecute() bool {
	cb.mu.RLock()
	defer cb.mu.RUnlock()

	switch cb.state {
	case StateClosed:
		return true
	case StateOpen:
		// Check if timeout has passed
		if time.Since(cb.lastFailureTime) >= cb.timeout {
			cb.mu.RUnlock()
			cb.mu.Lock()
			cb.state = StateHalfOpen
			cb.failureCount = 0
			cb.successCount = 0
			cb.stateChanges++
			cb.mu.Unlock()
			cb.mu.RLock()

			slog.Info("circuit breaker transitioning to half-open",
				slog.Duration("timeout", cb.timeout),
				slog.Time("last_failure", cb.lastFailureTime))

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

	cb.totalRequests++
	cb.totalSuccesses++
	cb.successCount++

	if cb.state == StateHalfOpen {
		// Check if we have enough successes to close the circuit
		if cb.successCount >= int(float64(cb.successCount+cb.failureCount)*cb.successThreshold) {
			cb.state = StateClosed
			cb.failureCount = 0
			cb.successCount = 0
			cb.stateChanges++

			slog.Info("circuit breaker closed due to success threshold",
				slog.Int("success_count", cb.successCount),
				slog.Float64("success_threshold", cb.successThreshold))
		}
	}
}

// RecordFailure records a failed operation
func (cb *CircuitBreaker) RecordFailure() {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	cb.totalRequests++
	cb.totalFailures++
	cb.failureCount++
	cb.lastFailureTime = time.Now()

	switch cb.state {
	case StateClosed:
		// Check if we should open the circuit
		if cb.failureCount >= cb.maxFailures {
			cb.state = StateOpen
			cb.stateChanges++

			slog.Warn("circuit breaker opened due to failure threshold",
				slog.Int("failure_count", cb.failureCount),
				slog.Int("max_failures", cb.maxFailures))
		}
	case StateHalfOpen:
		// Any failure in half-open state opens the circuit
		cb.state = StateOpen
		cb.stateChanges++

		slog.Warn("circuit breaker opened due to failure in half-open state",
			slog.Int("failure_count", cb.failureCount))
	}
}

// GetState returns the current state
func (cb *CircuitBreaker) GetState() CircuitBreakerState {
	cb.mu.RLock()
	defer cb.mu.RUnlock()
	return cb.state
}

// GetStats returns circuit breaker statistics
func (cb *CircuitBreaker) GetStats() map[string]interface{} {
	cb.mu.RLock()
	defer cb.mu.RUnlock()

	successRate := float64(0)
	if cb.totalRequests > 0 {
		successRate = float64(cb.totalSuccesses) / float64(cb.totalRequests) * 100
	}

	return map[string]interface{}{
		"state":             cb.state.String(),
		"max_failures":      cb.maxFailures,
		"timeout":           cb.timeout.String(),
		"success_threshold": cb.successThreshold,
		"failure_count":     cb.failureCount,
		"success_count":     cb.successCount,
		"total_requests":    cb.totalRequests,
		"total_failures":    cb.totalFailures,
		"total_successes":   cb.totalSuccesses,
		"success_rate":      successRate,
		"state_changes":     cb.stateChanges,
		"last_failure":      cb.lastFailureTime.Format(time.RFC3339),
	}
}

// Reset resets the circuit breaker to closed state
func (cb *CircuitBreaker) Reset() {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	cb.state = StateClosed
	cb.failureCount = 0
	cb.successCount = 0
	cb.totalRequests = 0
	cb.totalFailures = 0
	cb.totalSuccesses = 0
	cb.stateChanges = 0
	cb.lastFailureTime = time.Time{}

	slog.Info("circuit breaker reset to closed state")
}
