package domain

import (
	"errors"
	"testing"
	"time"
)

func TestDefaultRetryConfigValues(t *testing.T) {
	cfg := DefaultRetryConfig()

	if cfg.MaxRetries != 3 {
		t.Fatalf("MaxRetries = %d, want 3", cfg.MaxRetries)
	}
	if cfg.InitialDelay != 2*time.Second {
		t.Fatalf("InitialDelay = %v, want 2s", cfg.InitialDelay)
	}
	if cfg.MaxDelay != 30*time.Second {
		t.Fatalf("MaxDelay = %v, want 30s", cfg.MaxDelay)
	}
	if cfg.Multiplier != 2.0 {
		t.Fatalf("Multiplier = %v, want 2.0", cfg.Multiplier)
	}
	if !cfg.Jitter {
		t.Fatalf("Jitter = false, want true")
	}
	if len(cfg.RetryableErrors) == 0 {
		t.Fatalf("RetryableErrors should not be empty")
	}
	if len(cfg.NonRetryableErrors) == 0 {
		t.Fatalf("NonRetryableErrors should not be empty")
	}
}

func TestRetryInfo_ShouldRetry_BasicDecisions(t *testing.T) {
	cfg := DefaultRetryConfig()

	ri := &RetryInfo{AttemptCount: cfg.MaxRetries}
	if ri.ShouldRetry(errors.New("timeout"), cfg) {
		t.Fatalf("ShouldRetry returned true when max retries reached")
	}

	ri = &RetryInfo{RetryStatus: RetryStatusDLQ}
	if ri.ShouldRetry(errors.New("timeout"), cfg) {
		t.Fatalf("ShouldRetry returned true when status is DLQ")
	}

	ri = &RetryInfo{}
	if !ri.ShouldRetry(errors.New("timeout while calling upstream"), cfg) {
		t.Fatalf("ShouldRetry returned false for retryable error")
	}

	ri = &RetryInfo{}
	if ri.ShouldRetry(errors.New("invalid argument: bad payload"), cfg) {
		t.Fatalf("ShouldRetry returned true for non-retryable error")
	}

	ri = &RetryInfo{}
	if !ri.ShouldRetry(errors.New("some unknown error"), cfg) {
		t.Fatalf("ShouldRetry returned false for unknown error")
	}
}

func TestRetryInfo_CalculateNextRetryDelay(t *testing.T) {
	cfg := RetryConfig{
		InitialDelay: 2 * time.Second,
		MaxDelay:     30 * time.Second,
		Multiplier:   2.0,
		Jitter:       false,
	}

	ri := &RetryInfo{AttemptCount: 2}
	delay := ri.CalculateNextRetryDelay(cfg)
	if delay != 8*time.Second {
		t.Fatalf("delay = %v, want 8s", delay)
	}
}

func TestRetryInfo_CalculateNextRetryDelay_WithCapAndJitter(t *testing.T) {
	cfg := RetryConfig{
		InitialDelay: 5 * time.Second,
		MaxDelay:     10 * time.Second,
		Multiplier:   10.0,
		Jitter:       true,
	}

	ri := &RetryInfo{AttemptCount: 3}
	delay := ri.CalculateNextRetryDelay(cfg)

	minRetryDelay := 10 * time.Second
	maxRetryDelay := 11 * time.Second
	if delay < minRetryDelay || delay > maxRetryDelay {
		t.Fatalf("delay = %v, want between %v and %v", delay, minRetryDelay, maxRetryDelay)
	}
}

func TestRetryInfo_UpdateAndStatusTransitions(t *testing.T) {
	ri := &RetryInfo{}
	if ri.AttemptCount != 0 {
		t.Fatalf("initial AttemptCount = %d, want 0", ri.AttemptCount)
	}

	err := errors.New("first")
	ri.UpdateRetryAttempt(err)
	if ri.AttemptCount != 1 {
		t.Fatalf("AttemptCount = %d, want 1", ri.AttemptCount)
	}
	if ri.LastError != err.Error() {
		t.Fatalf("LastError = %q, want %q", ri.LastError, err.Error())
	}
	if len(ri.ErrorHistory) != 1 {
		t.Fatalf("ErrorHistory len = %d, want 1", len(ri.ErrorHistory))
	}
	if ri.LastAttemptAt.IsZero() || ri.UpdatedAt.IsZero() {
		t.Fatalf("timestamps should be set after UpdateRetryAttempt")
	}

	before := ri.UpdatedAt
	ri.MarkAsRetrying()
	if ri.RetryStatus != RetryStatusRetrying {
		t.Fatalf("RetryStatus = %q, want %q", ri.RetryStatus, RetryStatusRetrying)
	}
	if !ri.UpdatedAt.After(before) && !ri.UpdatedAt.Equal(before) {
		t.Fatalf("UpdatedAt should be updated or equal after MarkAsRetrying")
	}

	ri.MarkAsExhausted()
	if ri.RetryStatus != RetryStatusExhausted {
		t.Fatalf("RetryStatus = %q, want %q", ri.RetryStatus, RetryStatusExhausted)
	}

	ri.MarkAsDLQ()
	if ri.RetryStatus != RetryStatusDLQ {
		t.Fatalf("RetryStatus = %q, want %q", ri.RetryStatus, RetryStatusDLQ)
	}
}

func TestContainsAndPowHelpers(t *testing.T) {
	if !contains("timeout while calling upstream", "timeout") {
		t.Fatalf("contains should return true for matching prefix")
	}
	if contains("timeout while calling upstream", "upstream") {
		t.Fatalf("contains should return false for non-prefix substring")
	}

	if got := pow(2, 0); got != 1 {
		t.Fatalf("pow(2,0) = %v, want 1", got)
	}
	if got := pow(2, 3); got != 8 {
		t.Fatalf("pow(2,3) = %v, want 8", got)
	}
}
