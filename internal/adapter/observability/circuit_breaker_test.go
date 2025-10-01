package observability_test

import (
	"errors"
	"testing"
	"time"

	"github.com/fairyhunter13/ai-cv-evaluator/internal/adapter/observability"
	"github.com/stretchr/testify/assert"
)

func TestCircuitBreaker_NewCircuitBreaker(t *testing.T) {
	t.Parallel()

	cb := observability.NewCircuitBreaker("test", 3, 5*time.Second)

	// Test that the circuit breaker was created with correct parameters
	// We can't access private fields directly, so we test through behavior
	assert.Equal(t, observability.StateClosed, cb.GetState())
	assert.Equal(t, 0, cb.GetFailures())
	assert.True(t, cb.IsClosed())
	assert.False(t, cb.IsOpen())
	assert.False(t, cb.IsHalfOpen())
}

func TestCircuitBreaker_Call_Success(t *testing.T) {
	t.Parallel()

	cb := observability.NewCircuitBreaker("test", 2, 1*time.Second)

	err := cb.Call(func() error {
		return nil
	})

	assert.NoError(t, err)
	assert.Equal(t, observability.StateClosed, cb.GetState())
	assert.Equal(t, 0, cb.GetFailures())
}

func TestCircuitBreaker_Call_Failure(t *testing.T) {
	t.Parallel()

	cb := observability.NewCircuitBreaker("test", 2, 1*time.Second)
	testErr := errors.New("test error")

	err := cb.Call(func() error {
		return testErr
	})

	assert.Equal(t, testErr, err)
	assert.Equal(t, observability.StateClosed, cb.GetState())
	assert.Equal(t, 1, cb.GetFailures())
}

func TestCircuitBreaker_StateTransitions(t *testing.T) {
	t.Parallel()

	cb := observability.NewCircuitBreaker("test", 2, 100*time.Millisecond)

	// First failure - should stay closed
	err := cb.Call(func() error {
		return errors.New("failure 1")
	})
	assert.Error(t, err)
	assert.Equal(t, observability.StateClosed, cb.GetState())
	assert.Equal(t, 1, cb.GetFailures())

	// Second failure - should open circuit
	err = cb.Call(func() error {
		return errors.New("failure 2")
	})
	assert.Error(t, err)
	assert.Equal(t, observability.StateOpen, cb.GetState())
	assert.Equal(t, 2, cb.GetFailures())
	assert.True(t, cb.IsOpen())

	// Call while open should be blocked
	err = cb.Call(func() error {
		return nil
	})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "circuit breaker test is open")

	// Wait for timeout to transition to half-open
	time.Sleep(150 * time.Millisecond)

	// Call to trigger state transition from open to half-open
	err = cb.Call(func() error {
		return nil
	})
	assert.NoError(t, err)

	// Should now be half-open
	assert.Equal(t, observability.StateHalfOpen, cb.GetState())
	assert.True(t, cb.IsHalfOpen())

	// Success in half-open should close circuit (after enough successes)
	for i := 0; i < 2; i++ { // halfOpenMax is 3, we already had 1 success
		err = cb.Call(func() error {
			return nil
		})
		assert.NoError(t, err)
	}
	assert.Equal(t, observability.StateClosed, cb.GetState())
	assert.True(t, cb.IsClosed())
}

func TestCircuitBreaker_Reset(t *testing.T) {
	t.Parallel()

	cb := observability.NewCircuitBreaker("test", 1, 1*time.Second)

	// Open the circuit
	_ = cb.Call(func() error {
		return errors.New("failure")
	})
	assert.Equal(t, observability.StateOpen, cb.GetState())

	// Reset
	cb.Reset()
	assert.Equal(t, observability.StateClosed, cb.GetState())
	assert.Equal(t, 0, cb.GetFailures())
	assert.True(t, cb.IsClosed())
}

func TestCircuitBreakerManager_NewCircuitBreakerManager(t *testing.T) {
	t.Parallel()

	cbm := observability.NewCircuitBreakerManager()
	assert.NotNil(t, cbm)
	assert.Empty(t, cbm.GetAll())
}

func TestCircuitBreakerManager_GetOrCreate(t *testing.T) {
	t.Parallel()

	cbm := observability.NewCircuitBreakerManager()

	// Create new circuit breaker
	cb1 := cbm.GetOrCreate("test1", 2, 1*time.Second)
	assert.NotNil(t, cb1)

	// Get existing circuit breaker
	cb2 := cbm.GetOrCreate("test1", 5, 2*time.Second)
	assert.Equal(t, cb1, cb2) // Should be the same instance

	// Create another
	cb3 := cbm.GetOrCreate("test2", 3, 3*time.Second)
	assert.NotEqual(t, cb1, cb3)
}

func TestCircuitBreakerManager_Get(t *testing.T) {
	t.Parallel()

	cbm := observability.NewCircuitBreakerManager()

	// Get non-existent
	cb, exists := cbm.Get("nonexistent")
	assert.Nil(t, cb)
	assert.False(t, exists)

	// Create and get
	cbm.GetOrCreate("test", 2, 1*time.Second)
	cb, exists = cbm.Get("test")
	assert.NotNil(t, cb)
	assert.True(t, exists)
}

func TestCircuitBreakerManager_GetAll(t *testing.T) {
	t.Parallel()

	cbm := observability.NewCircuitBreakerManager()

	// Empty initially
	all := cbm.GetAll()
	assert.Empty(t, all)

	// Add some circuit breakers
	cbm.GetOrCreate("test1", 2, 1*time.Second)
	cbm.GetOrCreate("test2", 3, 2*time.Second)

	all = cbm.GetAll()
	assert.Len(t, all, 2)
	assert.Contains(t, all, "test1")
	assert.Contains(t, all, "test2")
}

func TestCircuitBreakerManager_ResetAll(t *testing.T) {
	t.Parallel()

	cbm := observability.NewCircuitBreakerManager()

	// Create and open some circuit breakers
	cb1 := cbm.GetOrCreate("test1", 1, 1*time.Second)
	cb2 := cbm.GetOrCreate("test2", 1, 1*time.Second)

	// Open them
	_ = cb1.Call(func() error { return errors.New("fail") })
	_ = cb2.Call(func() error { return errors.New("fail") })

	assert.True(t, cb1.IsOpen())
	assert.True(t, cb2.IsOpen())

	// Reset all
	cbm.ResetAll()

	assert.True(t, cb1.IsClosed())
	assert.True(t, cb2.IsClosed())
}

func TestGlobalCircuitBreakerFunctions(t *testing.T) {
	t.Parallel()

	// Reset global state
	observability.ResetAllCircuitBreakers()

	// Test GetCircuitBreaker
	cb := observability.GetCircuitBreaker("global-test", 2, 1*time.Second)
	assert.NotNil(t, cb)

	// Test GetCircuitBreakerState
	state, exists := observability.GetCircuitBreakerState("global-test")
	assert.True(t, exists)
	assert.Equal(t, observability.StateClosed, state)

	state, exists = observability.GetCircuitBreakerState("nonexistent")
	assert.False(t, exists)
	assert.Equal(t, observability.StateClosed, state)

	// Test IsCircuitBreakerOpen
	assert.False(t, observability.IsCircuitBreakerOpen("global-test"))
	assert.False(t, observability.IsCircuitBreakerOpen("nonexistent"))

	// Open the circuit breaker
	_ = cb.Call(func() error { return errors.New("fail") })
	_ = cb.Call(func() error { return errors.New("fail") })

	assert.True(t, observability.IsCircuitBreakerOpen("global-test"))

	// Test ResetCircuitBreaker
	observability.ResetCircuitBreaker("global-test")
	assert.False(t, observability.IsCircuitBreakerOpen("global-test"))

	// Test ResetAllCircuitBreakers
	_ = cb.Call(func() error { return errors.New("fail") })
	_ = cb.Call(func() error { return errors.New("fail") })
	assert.True(t, observability.IsCircuitBreakerOpen("global-test"))

	observability.ResetAllCircuitBreakers()
	assert.False(t, observability.IsCircuitBreakerOpen("global-test"))
}

func TestCircuitBreaker_HalfOpenToClosed(t *testing.T) {
	t.Parallel()

	cb := observability.NewCircuitBreaker("test", 1, 100*time.Millisecond)

	// Open the circuit
	_ = cb.Call(func() error { return errors.New("fail") })
	assert.True(t, cb.IsOpen())

	// Wait for timeout
	time.Sleep(150 * time.Millisecond)

	// Call to trigger state transition from open to half-open
	err := cb.Call(func() error { return nil })
	assert.NoError(t, err)
	assert.True(t, cb.IsHalfOpen())

	// Success should close the circuit (after enough successes)
	for i := 0; i < 2; i++ { // halfOpenMax is 3, we already had 1 success
		err := cb.Call(func() error { return nil })
		assert.NoError(t, err)
	}
	assert.True(t, cb.IsClosed())
}

func TestCircuitBreaker_HalfOpenToOpen(t *testing.T) {
	t.Parallel()

	cb := observability.NewCircuitBreaker("test", 1, 100*time.Millisecond)

	// Open the circuit
	_ = cb.Call(func() error { return errors.New("fail") })
	assert.True(t, cb.IsOpen())

	// Wait for timeout
	time.Sleep(150 * time.Millisecond)

	// Call to trigger state transition from open to half-open
	err := cb.Call(func() error { return nil })
	assert.NoError(t, err)
	assert.True(t, cb.IsHalfOpen())

	// Failure should open the circuit again
	err = cb.Call(func() error { return errors.New("fail again") })
	assert.Error(t, err)
	assert.True(t, cb.IsOpen())
}

func TestCircuitBreaker_ConcurrentAccess(t *testing.T) {
	t.Parallel()

	cb := observability.NewCircuitBreaker("test", 5, 100*time.Millisecond)

	// Run concurrent calls
	done := make(chan bool, 10)
	for i := 0; i < 10; i++ {
		go func() {
			_ = cb.Call(func() error {
				if time.Now().UnixNano()%2 == 0 {
					return errors.New("random failure")
				}
				return nil
			})
			done <- true
		}()
	}

	// Wait for all goroutines
	for i := 0; i < 10; i++ {
		<-done
	}

	// State should be consistent
	state := cb.GetState()
	assert.True(t, state == observability.StateClosed ||
		state == observability.StateOpen ||
		state == observability.StateHalfOpen)
}
