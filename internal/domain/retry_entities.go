// Package domain defines retry and DLQ entities for resilient job processing.
package domain

import (
	"time"
)

// RetryStatus represents the retry state of a job
type RetryStatus string

const (
	// RetryStatusNone indicates no retries have been attempted
	RetryStatusNone RetryStatus = "none"
	// RetryStatusRetrying indicates the job is being retried
	RetryStatusRetrying RetryStatus = "retrying"
	// RetryStatusExhausted indicates all retries have been exhausted
	RetryStatusExhausted RetryStatus = "exhausted"
	// RetryStatusDLQ indicates the job has been moved to DLQ
	RetryStatusDLQ RetryStatus = "dlq"
)

// RetryConfig defines retry behavior for job processing
type RetryConfig struct {
	// MaxRetries is the maximum number of retry attempts
	MaxRetries int
	// InitialDelay is the initial delay before first retry
	InitialDelay time.Duration
	// MaxDelay is the maximum delay between retries
	MaxDelay time.Duration
	// Multiplier is the exponential backoff multiplier
	Multiplier float64
	// Jitter adds randomness to prevent thundering herd
	Jitter bool
	// RetryableErrors defines which errors should trigger retries
	RetryableErrors []string
	// NonRetryableErrors defines which errors should not trigger retries
	NonRetryableErrors []string
}

// DefaultRetryConfig returns a sensible default retry configuration
func DefaultRetryConfig() RetryConfig {
	return RetryConfig{
		MaxRetries:   3,
		InitialDelay: 2 * time.Second,
		MaxDelay:     30 * time.Second,
		Multiplier:   2.0,
		Jitter:       true,
		RetryableErrors: []string{
			"context deadline exceeded",
			"connection refused",
			"timeout",
			"temporary failure",
			"rate limited",
			"upstream timeout",
			"upstream rate limit",
		},
		NonRetryableErrors: []string{
			"invalid argument",
			"not found",
			"conflict",
			"schema invalid",
			"authentication failed",
			"authorization failed",
		},
	}
}

// RetryInfo tracks retry attempts for a job
type RetryInfo struct {
	// AttemptCount is the current retry attempt number
	AttemptCount int
	// MaxAttempts is the maximum number of retry attempts
	MaxAttempts int
	// LastAttemptAt is the timestamp of the last retry attempt
	LastAttemptAt time.Time
	// NextRetryAt is the timestamp when the next retry should occur
	NextRetryAt time.Time
	// RetryStatus is the current retry status
	RetryStatus RetryStatus
	// LastError is the error from the last attempt
	LastError string
	// ErrorHistory is the history of all errors encountered
	ErrorHistory []string
	// CreatedAt is when the retry info was created
	CreatedAt time.Time
	// UpdatedAt is when the retry info was last updated
	UpdatedAt time.Time
}

// ShouldRetry determines if a job should be retried based on the error and retry config
func (ri *RetryInfo) ShouldRetry(err error, config RetryConfig) bool {
	// Don't retry if max attempts reached
	if ri.AttemptCount >= config.MaxRetries {
		return false
	}

	// Don't retry if already in DLQ
	if ri.RetryStatus == RetryStatusDLQ {
		return false
	}

	// Check if error is retryable
	errorStr := err.Error()
	for _, retryableErr := range config.RetryableErrors {
		if contains(errorStr, retryableErr) {
			return true
		}
	}

	// Check if error is non-retryable
	for _, nonRetryableErr := range config.NonRetryableErrors {
		if contains(errorStr, nonRetryableErr) {
			return false
		}
	}

	// Default to retryable for unknown errors
	return true
}

// CalculateNextRetryDelay calculates the delay for the next retry attempt
func (ri *RetryInfo) CalculateNextRetryDelay(config RetryConfig) time.Duration {
	// Calculate exponential backoff
	delay := time.Duration(float64(config.InitialDelay) * pow(config.Multiplier, float64(ri.AttemptCount)))

	// Cap at max delay
	if delay > config.MaxDelay {
		delay = config.MaxDelay
	}

	// Add jitter if enabled
	if config.Jitter {
		jitter := time.Duration(float64(delay) * 0.1) // 10% jitter
		delay = delay + jitter
	}

	return delay
}

// UpdateRetryAttempt updates the retry info after an attempt
func (ri *RetryInfo) UpdateRetryAttempt(err error) {
	ri.AttemptCount++
	ri.LastAttemptAt = time.Now()
	ri.UpdatedAt = time.Now()

	if err != nil {
		ri.LastError = err.Error()
		ri.ErrorHistory = append(ri.ErrorHistory, err.Error())
	}
}

// MarkAsExhausted marks the retry info as exhausted
func (ri *RetryInfo) MarkAsExhausted() {
	ri.RetryStatus = RetryStatusExhausted
	ri.UpdatedAt = time.Now()
}

// MarkAsDLQ marks the retry info as moved to DLQ
func (ri *RetryInfo) MarkAsDLQ() {
	ri.RetryStatus = RetryStatusDLQ
	ri.UpdatedAt = time.Now()
}

// MarkAsRetrying marks the retry info as currently retrying
func (ri *RetryInfo) MarkAsRetrying() {
	ri.RetryStatus = RetryStatusRetrying
	ri.UpdatedAt = time.Now()
}

// DLQJob represents a job that has been moved to the Dead Letter Queue
type DLQJob struct {
	// JobID is the original job ID
	JobID string
	// OriginalPayload is the original job payload
	OriginalPayload EvaluateTaskPayload
	// RetryInfo is the retry information
	RetryInfo RetryInfo
	// FailureReason is the reason for DLQ placement
	FailureReason string
	// MovedToDLQAt is when the job was moved to DLQ
	MovedToDLQAt time.Time
	// CanBeReprocessed indicates if the job can be reprocessed
	CanBeReprocessed bool
}

// Helper functions

func contains(s, substr string) bool {
	return len(s) >= len(substr) && s[:len(substr)] == substr
}

func pow(base, exp float64) float64 {
	result := 1.0
	for i := 0; i < int(exp); i++ {
		result *= base
	}
	return result
}
