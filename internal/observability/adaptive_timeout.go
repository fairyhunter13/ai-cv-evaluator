// Package observability provides adaptive timeout management and comprehensive metrics.
package observability

import (
	"context"
	"fmt"
	"log/slog"
	"sync"
	"time"
)

// AdaptiveTimeoutManager manages dynamic timeouts based on system performance
type AdaptiveTimeoutManager struct {
	mu sync.RWMutex

	// Base timeout configuration
	baseTimeout time.Duration
	minTimeout  time.Duration
	maxTimeout  time.Duration

	// Performance tracking
	successCount int64
	failureCount int64
	timeoutCount int64

	// Adaptive factors
	successFactor float64
	failureFactor float64
	timeoutFactor float64

	// Current adaptive timeout
	currentTimeout time.Duration

	// Metrics
	lastUpdate     time.Time
	updateInterval time.Duration
}

// NewAdaptiveTimeoutManager creates a new adaptive timeout manager
func NewAdaptiveTimeoutManager(baseTimeout, minTimeout, maxTimeout time.Duration) *AdaptiveTimeoutManager {
	return &AdaptiveTimeoutManager{
		baseTimeout:    baseTimeout,
		minTimeout:     minTimeout,
		maxTimeout:     maxTimeout,
		currentTimeout: baseTimeout,
		successFactor:  0.95, // Reduce timeout by 5% on success
		failureFactor:  1.05, // Increase timeout by 5% on failure
		timeoutFactor:  1.10, // Increase timeout by 10% on timeout
		updateInterval: 30 * time.Second,
	}
}

// GetTimeout returns the current adaptive timeout
func (atm *AdaptiveTimeoutManager) GetTimeout() time.Duration {
	atm.mu.RLock()
	defer atm.mu.RUnlock()
	return atm.currentTimeout
}

// RecordSuccess records a successful operation and adjusts timeout
func (atm *AdaptiveTimeoutManager) RecordSuccess(duration time.Duration) {
	atm.mu.Lock()
	defer atm.mu.Unlock()

	atm.successCount++

	// If operation completed much faster than timeout, reduce timeout
	if duration < atm.currentTimeout/2 {
		newTimeout := time.Duration(float64(atm.currentTimeout) * atm.successFactor)
		if newTimeout >= atm.minTimeout {
			atm.currentTimeout = newTimeout
			slog.Info("adaptive timeout reduced due to fast success",
				slog.Duration("old_timeout", time.Duration(float64(atm.currentTimeout)/atm.successFactor)),
				slog.Duration("new_timeout", atm.currentTimeout),
				slog.Duration("operation_duration", duration))
		}
	}

	atm.lastUpdate = time.Now()
}

// RecordFailure records a failed operation and adjusts timeout
func (atm *AdaptiveTimeoutManager) RecordFailure(err error) {
	atm.mu.Lock()
	defer atm.mu.Unlock()

	atm.failureCount++

	// Increase timeout on failure
	newTimeout := time.Duration(float64(atm.currentTimeout) * atm.failureFactor)
	if newTimeout <= atm.maxTimeout {
		atm.currentTimeout = newTimeout
		slog.Info("adaptive timeout increased due to failure",
			slog.Duration("old_timeout", time.Duration(float64(atm.currentTimeout)/atm.failureFactor)),
			slog.Duration("new_timeout", atm.currentTimeout),
			slog.String("error", err.Error()))
	}

	atm.lastUpdate = time.Now()
}

// RecordTimeout records a timeout and adjusts timeout
func (atm *AdaptiveTimeoutManager) RecordTimeout() {
	atm.mu.Lock()
	defer atm.mu.Unlock()

	atm.timeoutCount++

	// Increase timeout on timeout
	newTimeout := time.Duration(float64(atm.currentTimeout) * atm.timeoutFactor)
	if newTimeout <= atm.maxTimeout {
		atm.currentTimeout = newTimeout
		slog.Info("adaptive timeout increased due to timeout",
			slog.Duration("old_timeout", time.Duration(float64(atm.currentTimeout)/atm.timeoutFactor)),
			slog.Duration("new_timeout", atm.currentTimeout))
	}

	atm.lastUpdate = time.Now()
}

// WithTimeout creates a context with adaptive timeout
func (atm *AdaptiveTimeoutManager) WithTimeout(ctx context.Context) (context.Context, context.CancelFunc) {
	timeout := atm.GetTimeout()
	return context.WithTimeout(ctx, timeout)
}

// GetStats returns current statistics
func (atm *AdaptiveTimeoutManager) GetStats() map[string]interface{} {
	atm.mu.RLock()
	defer atm.mu.RUnlock()

	total := atm.successCount + atm.failureCount + atm.timeoutCount
	successRate := float64(0)
	if total > 0 {
		successRate = float64(atm.successCount) / float64(total) * 100
	}

	return map[string]interface{}{
		"current_timeout": atm.currentTimeout.String(),
		"base_timeout":    atm.baseTimeout.String(),
		"min_timeout":     atm.minTimeout.String(),
		"max_timeout":     atm.maxTimeout.String(),
		"success_count":   atm.successCount,
		"failure_count":   atm.failureCount,
		"timeout_count":   atm.timeoutCount,
		"success_rate":    fmt.Sprintf("%.2f%%", successRate),
		"last_update":     atm.lastUpdate.Format(time.RFC3339),
	}
}

// Reset resets the adaptive timeout to base value
func (atm *AdaptiveTimeoutManager) Reset() {
	atm.mu.Lock()
	defer atm.mu.Unlock()

	atm.currentTimeout = atm.baseTimeout
	atm.successCount = 0
	atm.failureCount = 0
	atm.timeoutCount = 0
	atm.lastUpdate = time.Now()

	slog.Info("adaptive timeout reset to base value",
		slog.Duration("base_timeout", atm.baseTimeout))
}
