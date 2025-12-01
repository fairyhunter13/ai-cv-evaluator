package observability

import (
	"testing"
	"time"
)

func TestCircuitBreakerState_String(t *testing.T) {
	cases := []struct {
		state    CircuitBreakerState
		expected string
	}{
		{StateClosed, "closed"},
		{StateOpen, "open"},
		{StateHalfOpen, "half-open"},
		{CircuitBreakerState(99), "unknown"},
	}

	for _, tt := range cases {
		if got := tt.state.String(); got != tt.expected {
			t.Errorf("String() = %q, want %q", got, tt.expected)
		}
	}
}

func TestNewCircuitBreakerDefaults(t *testing.T) {
	maxFailures := 3
	to := 5 * time.Second
	threshold := 0.7

	cb := NewCircuitBreaker(maxFailures, to, threshold)

	if cb.maxFailures != maxFailures {
		t.Fatalf("maxFailures = %d, want %d", cb.maxFailures, maxFailures)
	}
	if cb.timeout != to {
		t.Fatalf("timeout = %v, want %v", cb.timeout, to)
	}
	if cb.successThreshold != threshold {
		t.Fatalf("successThreshold = %v, want %v", cb.successThreshold, threshold)
	}
	if cb.state != StateClosed {
		t.Fatalf("initial state = %v, want %v", cb.state, StateClosed)
	}
}

func TestCircuitBreaker_CanExecuteTransitions(t *testing.T) {
	cb := NewCircuitBreaker(1, 50*time.Millisecond, 0.5)

	// Closed state allows execution
	if !cb.CanExecute() {
		t.Fatal("expected CanExecute to be true in closed state")
	}

	// Move to open state and ensure it blocks until timeout passes
	cb.state = StateOpen
	cb.lastFailureTime = time.Now()
	if cb.CanExecute() {
		t.Fatal("expected CanExecute to be false while open and before timeout")
	}

	// After timeout, CanExecute should transition to half-open and allow execution
	cb.lastFailureTime = time.Now().Add(-100 * time.Millisecond)
	if !cb.CanExecute() {
		t.Fatal("expected CanExecute to be true after timeout expired")
	}
	if cb.state != StateHalfOpen {
		t.Fatalf("expected state to transition to half-open, got %v", cb.state)
	}
}

func TestCircuitBreaker_RecordSuccessAndFailure(t *testing.T) {
	cb := NewCircuitBreaker(2, time.Second, 0.5)

	// In closed state, failures up to threshold keep circuit closed
	cb.RecordFailure()
	if cb.state != StateClosed {
		t.Fatalf("expected state closed after first failure, got %v", cb.state)
	}

	// Hitting maxFailures should open the circuit
	cb.RecordFailure()
	if cb.state != StateOpen {
		t.Fatalf("expected state open after reaching maxFailures, got %v", cb.state)
	}

	// Transition to half-open via CanExecute and then close on success
	cb.lastFailureTime = time.Now().Add(-2 * cb.timeout)
	if !cb.CanExecute() {
		t.Fatal("expected CanExecute to allow in half-open transition")
	}
	if cb.state != StateHalfOpen {
		t.Fatalf("expected half-open after CanExecute, got %v", cb.state)
	}

	cb.RecordSuccess() // should close immediately based on threshold logic
	if cb.state != StateClosed {
		t.Fatalf("expected state closed after success in half-open, got %v", cb.state)
	}
}

func TestCircuitBreaker_RecordFailureFromHalfOpen(t *testing.T) {
	cb := NewCircuitBreaker(2, time.Second, 0.5)
	cb.state = StateHalfOpen

	cb.RecordFailure()
	if cb.state != StateOpen {
		t.Fatalf("expected state open after failure in half-open, got %v", cb.state)
	}
}

func TestCircuitBreaker_GetStateAndStatsAndReset(t *testing.T) {
	cb := NewCircuitBreaker(2, time.Second, 0.5)

	cb.RecordFailure()
	cb.RecordSuccess()

	if got := cb.GetState(); got != cb.state {
		t.Fatalf("GetState() = %v, want %v", got, cb.state)
	}

	stats := cb.GetStats()
	if stats["state"] == "" {
		t.Fatal("expected state in stats")
	}
	if stats["total_requests"].(int64) == 0 {
		t.Fatal("expected total_requests > 0 in stats")
	}

	cb.Reset()
	if cb.state != StateClosed {
		t.Fatalf("expected state closed after Reset, got %v", cb.state)
	}
	if cb.totalRequests != 0 || cb.totalFailures != 0 || cb.totalSuccesses != 0 {
		t.Fatalf("expected counters zero after Reset, got total=%d failures=%d successes=%d", cb.totalRequests, cb.totalFailures, cb.totalSuccesses)
	}
}
