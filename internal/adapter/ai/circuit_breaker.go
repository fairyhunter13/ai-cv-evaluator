package ai

import (
	"log/slog"
	"sync"
	"time"
)

// CircuitState represents the state of a circuit breaker
type CircuitState int

const (
	// CircuitClosed indicates the circuit is allowing requests to pass through.
	CircuitClosed CircuitState = iota
	// CircuitOpen indicates the circuit is blocking requests due to failures.
	CircuitOpen
	// CircuitHalfOpen indicates the circuit is probing recovery with limited requests.
	CircuitHalfOpen
)

// CircuitBreaker implements an adaptive circuit breaker pattern for AI models
type CircuitBreaker struct {
	mu               sync.RWMutex
	modelID          string
	failureThreshold int
	recoveryTimeout  time.Duration
	state            CircuitState
	failureCount     int
	successCount     int
	lastFailureTime  time.Time
	lastSuccessTime  time.Time
	totalRequests    int
	totalFailures    int
}

// NewCircuitBreaker creates a new circuit breaker for a specific model
func NewCircuitBreaker(modelID string) *CircuitBreaker {
	return &CircuitBreaker{
		modelID:          modelID,
		failureThreshold: 3,                // Open circuit after 3 consecutive failures
		recoveryTimeout:  30 * time.Second, // Try recovery after 30 seconds
		state:            CircuitClosed,
	}
}

// ShouldAttempt determines if a request should be attempted based on circuit state
func (cb *CircuitBreaker) ShouldAttempt() bool {
	cb.mu.RLock()
	defer cb.mu.RUnlock()

	switch cb.state {
	case CircuitClosed:
		return true
	case CircuitOpen:
		// Check if recovery timeout has passed
		return time.Since(cb.lastFailureTime) > cb.recoveryTimeout
	case CircuitHalfOpen:
		// Allow one attempt to test recovery
		return true
	default:
		return false
	}
}

// RecordSuccess records a successful request
func (cb *CircuitBreaker) RecordSuccess() {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	cb.successCount++
	cb.lastSuccessTime = time.Now()
	cb.totalRequests++

	// Reset failure count on success
	cb.failureCount = 0

	// Transition states based on success
	switch cb.state {
	case CircuitHalfOpen:
		// Successful request in half-open state, close the circuit
		cb.state = CircuitClosed
		slog.Info("circuit breaker closed after successful recovery",
			slog.String("model", cb.modelID),
			slog.Int("success_count", cb.successCount),
			slog.Float64("success_rate", cb.getSuccessRate()))
	case CircuitOpen:
		// This shouldn't happen, but handle gracefully
		cb.state = CircuitClosed
		slog.Warn("circuit breaker closed unexpectedly after success",
			slog.String("model", cb.modelID))
	}
}

// RecordFailure records a failed request
func (cb *CircuitBreaker) RecordFailure() {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	cb.failureCount++
	cb.totalFailures++
	cb.totalRequests++
	cb.lastFailureTime = time.Now()

	// Check if we should open the circuit
	if cb.failureCount >= cb.failureThreshold {
		cb.state = CircuitOpen
		slog.Warn("circuit breaker opened due to consecutive failures",
			slog.String("model", cb.modelID),
			slog.Int("failure_count", cb.failureCount),
			slog.Int("threshold", cb.failureThreshold),
			slog.Float64("failure_rate", cb.getFailureRate()))
	}
}

// GetState returns the current circuit state
func (cb *CircuitBreaker) GetState() CircuitState {
	cb.mu.RLock()
	defer cb.mu.RUnlock()
	return cb.state
}

// GetStats returns circuit breaker statistics
func (cb *CircuitBreaker) GetStats() map[string]interface{} {
	cb.mu.RLock()
	defer cb.mu.RUnlock()

	return map[string]interface{}{
		"model_id":       cb.modelID,
		"state":          cb.state.String(),
		"failure_count":  cb.failureCount,
		"success_count":  cb.successCount,
		"total_requests": cb.totalRequests,
		"total_failures": cb.totalFailures,
		"success_rate":   cb.getSuccessRate(),
		"failure_rate":   cb.getFailureRate(),
		"last_failure":   cb.lastFailureTime,
		"last_success":   cb.lastSuccessTime,
	}
}

// getSuccessRate calculates the success rate (thread-safe)
func (cb *CircuitBreaker) getSuccessRate() float64 {
	if cb.totalRequests == 0 {
		return 0.0
	}
	return float64(cb.successCount) / float64(cb.totalRequests)
}

// getFailureRate calculates the failure rate (thread-safe)
func (cb *CircuitBreaker) getFailureRate() float64 {
	if cb.totalRequests == 0 {
		return 0.0
	}
	return float64(cb.totalFailures) / float64(cb.totalRequests)
}

// String returns a string representation of the circuit state
func (cs CircuitState) String() string {
	switch cs {
	case CircuitClosed:
		return "closed"
	case CircuitOpen:
		return "open"
	case CircuitHalfOpen:
		return "half-open"
	default:
		return "unknown"
	}
}

// CircuitBreakerManager manages multiple circuit breakers for different models
type CircuitBreakerManager struct {
	mu          sync.RWMutex
	breakers    map[string]*CircuitBreaker
	globalStats map[string]interface{}
}

// NewCircuitBreakerManager creates a new circuit breaker manager
func NewCircuitBreakerManager() *CircuitBreakerManager {
	return &CircuitBreakerManager{
		breakers:    make(map[string]*CircuitBreaker),
		globalStats: make(map[string]interface{}),
	}
}

// GetBreaker returns or creates a circuit breaker for a specific model
func (cbm *CircuitBreakerManager) GetBreaker(modelID string) *CircuitBreaker {
	cbm.mu.Lock()
	defer cbm.mu.Unlock()

	if breaker, exists := cbm.breakers[modelID]; exists {
		return breaker
	}

	breaker := NewCircuitBreaker(modelID)
	cbm.breakers[modelID] = breaker
	return breaker
}

// GetAllStats returns statistics for all circuit breakers
func (cbm *CircuitBreakerManager) GetAllStats() map[string]interface{} {
	cbm.mu.RLock()
	defer cbm.mu.RUnlock()

	stats := make(map[string]interface{})
	for modelID, breaker := range cbm.breakers {
		stats[modelID] = breaker.GetStats()
	}
	return stats
}

// GetHealthyModels returns models that are not in open circuit state
func (cbm *CircuitBreakerManager) GetHealthyModels() []string {
	cbm.mu.RLock()
	defer cbm.mu.RUnlock()

	var healthyModels []string
	for modelID, breaker := range cbm.breakers {
		if breaker.GetState() != CircuitOpen {
			healthyModels = append(healthyModels, modelID)
		}
	}
	return healthyModels
}
