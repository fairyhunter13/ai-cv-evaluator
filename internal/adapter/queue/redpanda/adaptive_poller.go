package redpanda

import (
	"math"
	"sync"
	"time"

	"log/slog"
)

// AdaptivePoller implements intelligent polling with dynamic intervals
type AdaptivePoller struct {
	mu                 sync.RWMutex
	baseInterval       time.Duration
	maxInterval        time.Duration
	minInterval        time.Duration
	backoffFactor      float64
	successCount       int
	failureCount       int
	consecutiveSuccess int
	consecutiveFailure int
	lastPollTime       time.Time
	lastSuccessTime    time.Time
	lastFailureTime    time.Time
	isHealthy          bool
}

// NewAdaptivePoller creates a new adaptive poller with improved timeout handling
func NewAdaptivePoller(baseInterval time.Duration) *AdaptivePoller {
	return &AdaptivePoller{
		baseInterval:  baseInterval,
		maxInterval:   10 * time.Second,       // Reduced from 30s to 10s maximum
		minInterval:   500 * time.Millisecond, // Increased from 100ms to 500ms minimum
		backoffFactor: 1.2,                    // Reduced from 1.5 to 1.2 (20% increase on failure)
	}
}

// GetNextInterval calculates the next polling interval based on success/failure patterns
func (ap *AdaptivePoller) GetNextInterval() time.Duration {
	ap.mu.Lock()
	defer ap.mu.Unlock()

	// Circuit breaker: If too many consecutive failures, use fixed interval
	if ap.consecutiveFailure >= 10 {
		ap.isHealthy = false
		slog.Warn("circuit breaker activated due to excessive failures",
			slog.Int("consecutive_failures", ap.consecutiveFailure),
			slog.Duration("fixed_interval", ap.maxInterval))
		return ap.maxInterval
	}

	// If we have more failures than successes, increase interval (backoff)
	if ap.failureCount > ap.successCount {
		// Exponential backoff with jitter, but capped to prevent excessive delays
		backoffMultiplier := math.Pow(ap.backoffFactor, float64(ap.consecutiveFailure))
		interval := float64(ap.baseInterval) * backoffMultiplier

		// Add jitter to prevent thundering herd
		jitter := interval * 0.1 * (0.5 - math.Mod(float64(time.Now().UnixNano()), 1.0))
		interval += jitter

		// Cap at maximum interval
		if interval > float64(ap.maxInterval) {
			interval = float64(ap.maxInterval)
		}
		slog.Debug("adaptive poller backoff",
			slog.Duration("interval", time.Duration(interval)),
			slog.Int("consecutive_failures", ap.consecutiveFailure),
			slog.Float64("backoff_multiplier", backoffMultiplier))

		return time.Duration(interval)
	}

	// If we're succeeding, decrease interval (speed up)
	successMultiplier := math.Max(0.5, 1.0/float64(ap.consecutiveSuccess+1))
	interval := float64(ap.baseInterval) * successMultiplier

	// Don't go below minimum interval
	if interval < float64(ap.minInterval) {
		interval = float64(ap.minInterval)
	}

	ap.isHealthy = true
	slog.Debug("adaptive poller speedup",
		slog.Duration("interval", time.Duration(interval)),
		slog.Int("consecutive_successes", ap.consecutiveSuccess),
		slog.Float64("success_multiplier", successMultiplier))
	return time.Duration(interval)
}

// RecordSuccess records a successful poll
func (ap *AdaptivePoller) RecordSuccess() {
	ap.mu.Lock()
	defer ap.mu.Unlock()

	ap.successCount++
	ap.consecutiveSuccess++
	ap.consecutiveFailure = 0
	ap.lastSuccessTime = time.Now()
	ap.lastPollTime = time.Now()
	ap.isHealthy = true

	// Reset circuit breaker if we've had enough successes
	if ap.consecutiveSuccess >= 3 {
		slog.Info("circuit breaker reset due to consecutive successes",
			slog.Int("consecutive_successes", ap.consecutiveSuccess))
	}

	slog.Debug("adaptive poller success recorded",
		slog.Int("success_count", ap.successCount),
		slog.Int("consecutive_success", ap.consecutiveSuccess),
		slog.Bool("is_healthy", ap.isHealthy))
}

// RecordFailure records a failed poll
func (ap *AdaptivePoller) RecordFailure() {
	ap.mu.Lock()
	defer ap.mu.Unlock()

	ap.failureCount++
	ap.consecutiveFailure++
	ap.consecutiveSuccess = 0
	ap.lastFailureTime = time.Now()
	ap.lastPollTime = time.Now()
	ap.isHealthy = false

	slog.Debug("adaptive poller failure recorded",
		slog.Int("failure_count", ap.failureCount),
		slog.Int("consecutive_failure", ap.consecutiveFailure),
		slog.Bool("is_healthy", ap.isHealthy))
}

// IsHealthy returns whether the poller considers the system healthy
func (ap *AdaptivePoller) IsHealthy() bool {
	ap.mu.RLock()
	defer ap.mu.RUnlock()
	return ap.isHealthy
}

// GetStats returns polling statistics
func (ap *AdaptivePoller) GetStats() map[string]interface{} {
	ap.mu.RLock()
	defer ap.mu.RUnlock()

	totalPolls := ap.successCount + ap.failureCount
	successRate := 0.0
	if totalPolls > 0 {
		successRate = float64(ap.successCount) / float64(totalPolls)
	}

	return map[string]interface{}{
		"base_interval":       ap.baseInterval,
		"max_interval":        ap.maxInterval,
		"min_interval":        ap.minInterval,
		"success_count":       ap.successCount,
		"failure_count":       ap.failureCount,
		"consecutive_success": ap.consecutiveSuccess,
		"consecutive_failure": ap.consecutiveFailure,
		"total_polls":         totalPolls,
		"success_rate":        successRate,
		"is_healthy":          ap.isHealthy,
		"last_poll_time":      ap.lastPollTime,
		"last_success_time":   ap.lastSuccessTime,
		"last_failure_time":   ap.lastFailureTime,
	}
}

// Reset resets the poller statistics
func (ap *AdaptivePoller) Reset() {
	ap.mu.Lock()
	defer ap.mu.Unlock()

	ap.successCount = 0
	ap.failureCount = 0
	ap.consecutiveSuccess = 0
	ap.consecutiveFailure = 0
	ap.isHealthy = true

	slog.Info("adaptive poller reset")
}

// AdaptivePollingManager manages adaptive polling for multiple topics
type AdaptivePollingManager struct {
	mu       sync.RWMutex
	pollers  map[string]*AdaptivePoller
	cleanup  chan struct{}
	interval time.Duration
}

// NewAdaptivePollingManager creates a new adaptive polling manager
func NewAdaptivePollingManager(cleanupInterval time.Duration) *AdaptivePollingManager {
	manager := &AdaptivePollingManager{
		pollers:  make(map[string]*AdaptivePoller),
		cleanup:  make(chan struct{}),
		interval: cleanupInterval,
	}

	// Start cleanup routine
	go manager.cleanupRoutine()

	return manager
}

// GetPoller returns or creates an adaptive poller for a specific topic
func (apm *AdaptivePollingManager) GetPoller(topic string, baseInterval time.Duration) *AdaptivePoller {
	apm.mu.Lock()
	defer apm.mu.Unlock()

	if poller, exists := apm.pollers[topic]; exists {
		return poller
	}

	poller := NewAdaptivePoller(baseInterval)
	apm.pollers[topic] = poller
	return poller
}

// GetAllStats returns statistics for all pollers
func (apm *AdaptivePollingManager) GetAllStats() map[string]interface{} {
	apm.mu.RLock()
	defer apm.mu.RUnlock()

	stats := make(map[string]interface{})
	for topic, poller := range apm.pollers {
		stats[topic] = poller.GetStats()
	}
	return stats
}

// cleanupRoutine periodically cleans up old pollers
func (apm *AdaptivePollingManager) cleanupRoutine() {
	ticker := time.NewTicker(apm.interval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			apm.cleanupOldPollers()
		case <-apm.cleanup:
			return
		}
	}
}

// cleanupOldPollers removes pollers that haven't been used recently
func (apm *AdaptivePollingManager) cleanupOldPollers() {
	apm.mu.Lock()
	defer apm.mu.Unlock()

	cutoff := time.Now().Add(-1 * time.Hour) // Remove pollers unused for 1 hour
	removed := 0

	for topic, poller := range apm.pollers {
		stats := poller.GetStats()
		if lastPoll, ok := stats["last_poll_time"].(time.Time); ok {
			if lastPoll.Before(cutoff) {
				delete(apm.pollers, topic)
				removed++
			}
		}
	}

	if removed > 0 {
		slog.Debug("cleaned up old adaptive pollers",
			slog.Int("removed_count", removed),
			slog.Int("remaining_count", len(apm.pollers)))
	}
}

// Stop stops the adaptive polling manager
func (apm *AdaptivePollingManager) Stop() {
	close(apm.cleanup)
}
