package observability

import (
	"context"
	"errors"
	"testing"
	"time"
)

func TestNewAdaptiveTimeoutManagerDefaults(t *testing.T) {
	base := 60 * time.Second
	minTimeout := 10 * time.Second
	maxTimeout := 5 * time.Minute

	atm := NewAdaptiveTimeoutManager(base, minTimeout, maxTimeout)

	if atm.baseTimeout != base {
		t.Fatalf("baseTimeout = %v, want %v", atm.baseTimeout, base)
	}
	if atm.minTimeout != minTimeout {
		t.Fatalf("minTimeout = %v, want %v", atm.minTimeout, minTimeout)
	}
	if atm.maxTimeout != maxTimeout {
		t.Fatalf("maxTimeout = %v, want %v", atm.maxTimeout, maxTimeout)
	}
	if got := atm.GetTimeout(); got != base {
		t.Fatalf("GetTimeout() = %v, want %v", got, base)
	}
}

func TestAdaptiveTimeout_RecordSuccessReducesTimeout(t *testing.T) {
	base := 60 * time.Second
	minTimeout := 5 * time.Second
	atm := NewAdaptiveTimeoutManager(base, minTimeout, 5*time.Minute)

	before := atm.GetTimeout()
	atm.RecordSuccess(before / 4)
	after := atm.GetTimeout()

	if after >= before {
		t.Fatalf("expected timeout to be reduced, before=%v after=%v", before, after)
	}
	if after < minTimeout {
		t.Fatalf("timeout reduced below minTimeout, after=%v min=%v", after, minTimeout)
	}
}

func TestAdaptiveTimeout_RecordSuccessNoChangeWhenNotFast(t *testing.T) {
	base := 60 * time.Second
	atm := NewAdaptiveTimeoutManager(base, 10*time.Second, 5*time.Minute)

	before := atm.GetTimeout()
	atm.RecordSuccess(before)
	after := atm.GetTimeout()

	if after != before {
		t.Fatalf("expected timeout to stay the same, before=%v after=%v", before, after)
	}
}

func TestAdaptiveTimeout_RecordFailureIncreasesTimeout(t *testing.T) {
	base := 1 * time.Second
	maxTimeout := 10 * time.Second
	atm := NewAdaptiveTimeoutManager(base, base, maxTimeout)

	before := atm.GetTimeout()
	atm.RecordFailure(errors.New("boom"))
	after := atm.GetTimeout()

	if after <= before {
		t.Fatalf("expected timeout to increase on failure, before=%v after=%v", before, after)
	}
	if after > maxTimeout {
		t.Fatalf("timeout exceeded maxTimeout, after=%v max=%v", after, maxTimeout)
	}
}

func TestAdaptiveTimeout_RecordTimeoutIncreasesTimeout(t *testing.T) {
	base := 1 * time.Second
	maxTimeout := 10 * time.Second
	atm := NewAdaptiveTimeoutManager(base, base, maxTimeout)

	before := atm.GetTimeout()
	atm.RecordTimeout()
	after := atm.GetTimeout()

	if after <= before {
		t.Fatalf("expected timeout to increase on timeout, before=%v after=%v", before, after)
	}
	if after > maxTimeout {
		t.Fatalf("timeout exceeded maxTimeout, after=%v max=%v", after, maxTimeout)
	}
}

func TestAdaptiveTimeout_WithTimeoutUsesCurrentTimeout(t *testing.T) {
	base := 100 * time.Millisecond
	atm := NewAdaptiveTimeoutManager(base, base, base)

	ctx := context.Background()
	ctxWithTimeout, cancel := atm.WithTimeout(ctx)
	defer cancel()

	if ctxWithTimeout == nil {
		t.Fatal("expected non-nil context from WithTimeout")
	}

	deadline, ok := ctxWithTimeout.Deadline()
	if !ok {
		t.Fatal("expected context to have a deadline")
	}

	remaining := time.Until(deadline)
	if remaining <= 0 {
		t.Fatalf("expected deadline in the future, remaining=%v", remaining)
	}
}

func TestAdaptiveTimeout_GetStatsAndReset(t *testing.T) {
	base := 2 * time.Second
	atm := NewAdaptiveTimeoutManager(base, 1*time.Second, 10*time.Second)

	// Drive some activity
	atm.RecordSuccess(base / 4)
	atm.RecordFailure(errors.New("boom"))
	atm.RecordTimeout()

	stats := atm.GetStats()
	if stats["current_timeout"] == "" {
		t.Fatal("expected current_timeout in stats")
	}
	if stats["base_timeout"] == "" {
		t.Fatal("expected base_timeout in stats")
	}
	if _, ok := stats["success_count"]; !ok {
		t.Fatal("expected success_count in stats")
	}
	if _, ok := stats["failure_count"]; !ok {
		t.Fatal("expected failure_count in stats")
	}
	if _, ok := stats["timeout_count"]; !ok {
		t.Fatal("expected timeout_count in stats")
	}
	if _, ok := stats["success_rate"]; !ok {
		t.Fatal("expected success_rate in stats")
	}

	// Reset and ensure fields go back to baseline
	atm.Reset()

	if got := atm.GetTimeout(); got != base {
		t.Fatalf("after Reset, GetTimeout() = %v, want %v", got, base)
	}
	if atm.successCount != 0 || atm.failureCount != 0 || atm.timeoutCount != 0 {
		t.Fatalf("expected counters to be zero after Reset, got success=%d failure=%d timeout=%d", atm.successCount, atm.failureCount, atm.timeoutCount)
	}
}
