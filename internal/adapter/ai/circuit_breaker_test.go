package ai

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewCircuitBreaker(t *testing.T) {
	cb := NewCircuitBreaker("test-model")
	assert.NotNil(t, cb)
	assert.Equal(t, "test-model", cb.modelID)
	assert.Equal(t, CircuitClosed, cb.state)
	assert.Equal(t, 3, cb.failureThreshold)
	assert.Equal(t, 30*time.Second, cb.recoveryTimeout)
}

func TestCircuitBreaker_ShouldAttempt(t *testing.T) {
	tests := []struct {
		name     string
		setup    func(*CircuitBreaker)
		expected bool
	}{
		{
			name:     "closed circuit allows attempts",
			setup:    func(cb *CircuitBreaker) {},
			expected: true,
		},
		{
			name: "open circuit blocks attempts when recovery timeout not passed",
			setup: func(cb *CircuitBreaker) {
				cb.state = CircuitOpen
				cb.lastFailureTime = time.Now()
			},
			expected: false,
		},
		{
			name: "open circuit allows attempts after recovery timeout",
			setup: func(cb *CircuitBreaker) {
				cb.state = CircuitOpen
				cb.lastFailureTime = time.Now().Add(-35 * time.Second)
			},
			expected: true,
		},
		{
			name: "half-open circuit allows attempts",
			setup: func(cb *CircuitBreaker) {
				cb.state = CircuitHalfOpen
			},
			expected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cb := NewCircuitBreaker("test-model")
			tt.setup(cb)
			assert.Equal(t, tt.expected, cb.ShouldAttempt())
		})
	}
}

func TestCircuitBreaker_RecordSuccess(t *testing.T) {
	t.Run("increments success count", func(t *testing.T) {
		cb := NewCircuitBreaker("test-model")
		cb.RecordSuccess()
		assert.Equal(t, 1, cb.successCount)
		assert.Equal(t, 1, cb.totalRequests)
		assert.Equal(t, 0, cb.failureCount)
	})

	t.Run("resets failure count on success", func(t *testing.T) {
		cb := NewCircuitBreaker("test-model")
		cb.failureCount = 2
		cb.RecordSuccess()
		assert.Equal(t, 0, cb.failureCount)
	})

	t.Run("closes circuit when successful in half-open state", func(t *testing.T) {
		cb := NewCircuitBreaker("test-model")
		cb.state = CircuitHalfOpen
		cb.RecordSuccess()
		assert.Equal(t, CircuitClosed, cb.state)
	})

	t.Run("closes circuit when successful in open state", func(t *testing.T) {
		cb := NewCircuitBreaker("test-model")
		cb.state = CircuitOpen
		cb.RecordSuccess()
		assert.Equal(t, CircuitClosed, cb.state)
	})
}

func TestCircuitBreaker_RecordFailure(t *testing.T) {
	t.Run("increments failure count", func(t *testing.T) {
		cb := NewCircuitBreaker("test-model")
		cb.RecordFailure()
		assert.Equal(t, 1, cb.failureCount)
		assert.Equal(t, 1, cb.totalFailures)
		assert.Equal(t, 1, cb.totalRequests)
	})

	t.Run("opens circuit when threshold reached", func(t *testing.T) {
		cb := NewCircuitBreaker("test-model")
		cb.RecordFailure()
		cb.RecordFailure()
		cb.RecordFailure()
		assert.Equal(t, CircuitOpen, cb.state)
	})

	t.Run("does not open circuit before threshold", func(t *testing.T) {
		cb := NewCircuitBreaker("test-model")
		cb.RecordFailure()
		cb.RecordFailure()
		assert.Equal(t, CircuitClosed, cb.state)
	})
}

func TestCircuitBreaker_GetState(t *testing.T) {
	cb := NewCircuitBreaker("test-model")
	assert.Equal(t, CircuitClosed, cb.GetState())
	cb.state = CircuitOpen
	assert.Equal(t, CircuitOpen, cb.GetState())
	cb.state = CircuitHalfOpen
	assert.Equal(t, CircuitHalfOpen, cb.GetState())
}

func TestCircuitBreaker_GetStats(t *testing.T) {
	cb := NewCircuitBreaker("test-model")
	cb.RecordSuccess()
	cb.RecordFailure()

	stats := cb.GetStats()
	assert.Equal(t, "test-model", stats["model_id"])
	assert.Equal(t, "closed", stats["state"])
	assert.Equal(t, 1, stats["failure_count"])
	assert.Equal(t, 1, stats["success_count"])
	assert.Equal(t, 2, stats["total_requests"])
	assert.Equal(t, 1, stats["total_failures"])
}

func TestCircuitState_String(t *testing.T) {
	tests := []struct {
		state    CircuitState
		expected string
	}{
		{CircuitClosed, "closed"},
		{CircuitOpen, "open"},
		{CircuitHalfOpen, "half-open"},
		{CircuitState(99), "unknown"},
	}

	for _, tt := range tests {
		t.Run(tt.expected, func(t *testing.T) {
			assert.Equal(t, tt.expected, tt.state.String())
		})
	}
}

func TestCircuitBreaker_SuccessAndFailureRates(t *testing.T) {
	cb := NewCircuitBreaker("test-model")

	// Initially both rates should be 0
	stats := cb.GetStats()
	assert.Equal(t, 0.0, stats["success_rate"])
	assert.Equal(t, 0.0, stats["failure_rate"])

	// After 2 successes and 2 failures, rates should be 0.5
	cb.RecordSuccess()
	cb.RecordSuccess()
	cb.RecordFailure()
	cb.RecordFailure()

	stats = cb.GetStats()
	assert.Equal(t, 0.5, stats["success_rate"])
	assert.Equal(t, 0.5, stats["failure_rate"])
}

func TestNewCircuitBreakerManager(t *testing.T) {
	cbm := NewCircuitBreakerManager()
	require.NotNil(t, cbm)
	assert.NotNil(t, cbm.breakers)
	assert.NotNil(t, cbm.globalStats)
}

func TestCircuitBreakerManager_GetBreaker(t *testing.T) {
	cbm := NewCircuitBreakerManager()

	// First call should create a new breaker
	breaker1 := cbm.GetBreaker("model-1")
	require.NotNil(t, breaker1)
	assert.Equal(t, "model-1", breaker1.modelID)

	// Second call should return the same breaker
	breaker2 := cbm.GetBreaker("model-1")
	assert.Same(t, breaker1, breaker2)

	// Different model should get different breaker
	breaker3 := cbm.GetBreaker("model-2")
	assert.NotSame(t, breaker1, breaker3)
}

func TestCircuitBreakerManager_GetAllStats(t *testing.T) {
	cbm := NewCircuitBreakerManager()

	// Create and use some breakers
	breaker1 := cbm.GetBreaker("model-1")
	breaker1.RecordSuccess()
	breaker2 := cbm.GetBreaker("model-2")
	breaker2.RecordFailure()

	stats := cbm.GetAllStats()
	assert.Len(t, stats, 2)
	assert.Contains(t, stats, "model-1")
	assert.Contains(t, stats, "model-2")
}

func TestCircuitBreakerManager_GetHealthyModels(t *testing.T) {
	cbm := NewCircuitBreakerManager()

	// Create breakers
	breaker1 := cbm.GetBreaker("model-1")
	breaker2 := cbm.GetBreaker("model-2")
	breaker3 := cbm.GetBreaker("model-3")

	// Set one to open state
	breaker2.state = CircuitOpen

	// Keep others closed
	_ = breaker1
	_ = breaker3

	healthy := cbm.GetHealthyModels()
	assert.Len(t, healthy, 2)
	assert.Contains(t, healthy, "model-1")
	assert.Contains(t, healthy, "model-3")
	assert.NotContains(t, healthy, "model-2")
}

func TestCircuitBreaker_ConcurrentAccess(t *testing.T) {
	cb := NewCircuitBreaker("test-model")
	done := make(chan bool)

	// Concurrent readers
	for i := 0; i < 10; i++ {
		go func() {
			for j := 0; j < 100; j++ {
				_ = cb.ShouldAttempt()
				_ = cb.GetState()
				_ = cb.GetStats()
			}
			done <- true
		}()
	}

	// Concurrent writers
	for i := 0; i < 5; i++ {
		go func() {
			for j := 0; j < 100; j++ {
				cb.RecordSuccess()
				cb.RecordFailure()
			}
			done <- true
		}()
	}

	// Wait for all goroutines
	for i := 0; i < 15; i++ {
		<-done
	}

	// Should not panic or have data races
	assert.True(t, true)
}
