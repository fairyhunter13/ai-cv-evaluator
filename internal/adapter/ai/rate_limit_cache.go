package ai

import (
	"sync"
	"time"

	"log/slog"
)

// RateLimitEntry represents a rate-limited model entry
type RateLimitEntry struct {
	ModelID       string
	BlockedUntil  time.Time
	FailureCount  int
	LastFailure   time.Time
	BlockDuration time.Duration
	MaxFailures   int
}

// IsBlocked checks if the model is currently blocked due to rate limiting
func (rle *RateLimitEntry) IsBlocked() bool {
	return time.Now().Before(rle.BlockedUntil)
}

// ShouldBlock checks if the model should be blocked based on failure count
func (rle *RateLimitEntry) ShouldBlock() bool {
	return rle.FailureCount >= rle.MaxFailures
}

// RecordFailure records a failure and potentially blocks the model
func (rle *RateLimitEntry) RecordFailure() {
	rle.FailureCount++
	rle.LastFailure = time.Now()

	// Block the model if it has exceeded the failure threshold
	if rle.ShouldBlock() {
		// Exponential backoff: increase block duration with each failure
		blockDuration := rle.BlockDuration
		if rle.FailureCount > 1 {
			// Exponential backoff: 10min, 20min, 40min, 80min, etc.
			// Use a safer calculation to avoid overflow
			multiplier := rle.FailureCount - 1
			if multiplier > 10 { // Cap multiplier to prevent overflow
				multiplier = 10
			}
			blockDuration = time.Duration(int64(blockDuration) * (1 << multiplier))
			// Cap at 2 hours maximum
			if blockDuration > 2*time.Hour {
				blockDuration = 2 * time.Hour
			}
		}

		rle.BlockedUntil = time.Now().Add(blockDuration)
		slog.Warn("model blocked due to rate limiting with exponential backoff",
			slog.String("model", rle.ModelID),
			slog.Int("failure_count", rle.FailureCount),
			slog.Duration("block_duration", blockDuration),
			slog.Time("blocked_until", rle.BlockedUntil))
	}
}

// RecordSuccess resets the failure count and unblocks the model
func (rle *RateLimitEntry) RecordSuccess() {
	if rle.FailureCount > 0 {
		slog.Info("model unblocked after successful request",
			slog.String("model", rle.ModelID),
			slog.Int("previous_failures", rle.FailureCount))
	}
	rle.FailureCount = 0
	rle.BlockedUntil = time.Time{} // Clear block
}

// GetTimeUntilUnblocked returns the duration until the model is unblocked
func (rle *RateLimitEntry) GetTimeUntilUnblocked() time.Duration {
	if !rle.IsBlocked() {
		return 0
	}
	return time.Until(rle.BlockedUntil)
}

// RateLimitCache manages rate-limited models with intelligent blocking
type RateLimitCache struct {
	mu              sync.RWMutex
	blockedModels   map[string]*RateLimitEntry
	defaultDuration time.Duration
	maxFailures     int
	cleanupInterval time.Duration
	stopCleanup     chan struct{}
}

// NewRateLimitCache creates a new rate limit cache
func NewRateLimitCache() *RateLimitCache {
	cache := &RateLimitCache{
		blockedModels:   make(map[string]*RateLimitEntry),
		defaultDuration: 20 * time.Second, // Short block for quick recovery
		maxFailures:     5,                // Require 5 consecutive failures before blocking
		cleanupInterval: 30 * time.Second, // Cleanup every 30 seconds
		stopCleanup:     make(chan struct{}),
	}

	// Start cleanup goroutine
	go cache.cleanupRoutine()

	return cache
}

// IsModelBlocked checks if a model is currently blocked due to rate limiting
func (rlc *RateLimitCache) IsModelBlocked(modelID string) bool {
	rlc.mu.RLock()
	defer rlc.mu.RUnlock()

	entry, exists := rlc.blockedModels[modelID]
	if !exists {
		return false
	}

	return entry.IsBlocked()
}

// BlockModel blocks a model for the specified duration
func (rlc *RateLimitCache) BlockModel(modelID string, duration time.Duration) {
	rlc.mu.Lock()
	defer rlc.mu.Unlock()

	entry := rlc.getOrCreateEntry(modelID)
	entry.BlockedUntil = time.Now().Add(duration)
	entry.RecordFailure()

	slog.Info("model blocked due to rate limiting",
		slog.String("model", modelID),
		slog.Duration("duration", duration),
		slog.Time("blocked_until", entry.BlockedUntil))
}

// RecordFailure records a failure for a model and potentially blocks it
func (rlc *RateLimitCache) RecordFailure(modelID string) {
	rlc.mu.Lock()
	defer rlc.mu.Unlock()

	entry := rlc.getOrCreateEntry(modelID)
	entry.RecordFailure()

	// Debug logging
	slog.Info("rate limit cache failure recorded",
		slog.String("model", modelID),
		slog.Int("failure_count", entry.FailureCount),
		slog.Bool("is_blocked", entry.IsBlocked()))
}

// RecordRateLimit records a rate limit event with a specific retry-after duration.
// If retryAfter is zero or negative, the default duration is used.
func (rlc *RateLimitCache) RecordRateLimit(modelID string, retryAfter time.Duration) {
	rlc.mu.Lock()
	defer rlc.mu.Unlock()

	entry := rlc.getOrCreateEntry(modelID)
	entry.FailureCount++
	entry.LastFailure = time.Now()

	blockFor := retryAfter
	if blockFor <= 0 {
		blockFor = rlc.defaultDuration
	}
	entry.BlockedUntil = time.Now().Add(blockFor)

	slog.Warn("model rate-limited; blocking until retry-after",
		slog.String("model", modelID),
		slog.Duration("retry_after", blockFor),
		slog.Time("blocked_until", entry.BlockedUntil),
		slog.Int("failure_count", entry.FailureCount))
}

// RecordSuccess records a success for a model and unblocks it
func (rlc *RateLimitCache) RecordSuccess(modelID string) {
	rlc.mu.Lock()
	defer rlc.mu.Unlock()

	entry := rlc.getOrCreateEntry(modelID)
	entry.RecordSuccess()
}

// GetBlockedModels returns a list of currently blocked models
func (rlc *RateLimitCache) GetBlockedModels() []string {
	rlc.mu.RLock()
	defer rlc.mu.RUnlock()

	var blocked []string
	for modelID, entry := range rlc.blockedModels {
		if entry.IsBlocked() {
			blocked = append(blocked, modelID)
		}
	}
	return blocked
}

// GetAvailableModels filters out blocked models from a list
func (rlc *RateLimitCache) GetAvailableModels(allModels []string) []string {
	rlc.mu.RLock()
	defer rlc.mu.RUnlock()

	var available []string
	var blocked []string
	for _, modelID := range allModels {
		entry, exists := rlc.blockedModels[modelID]
		if !exists || !entry.IsBlocked() {
			available = append(available, modelID)
		} else {
			blocked = append(blocked, modelID)
		}
	}

	// Debug logging
	slog.Info("rate limit cache filtering",
		slog.Int("total_models", len(allModels)),
		slog.Int("available_models", len(available)),
		slog.Int("blocked_models", len(blocked)),
		slog.Any("blocked_model_list", blocked))

	return available
}

// GetModelStatus returns the status of a specific model
func (rlc *RateLimitCache) GetModelStatus(modelID string) map[string]interface{} {
	rlc.mu.RLock()
	defer rlc.mu.RUnlock()

	entry, exists := rlc.blockedModels[modelID]
	if !exists {
		return map[string]interface{}{
			"model_id":      modelID,
			"blocked":       false,
			"failure_count": 0,
			"last_failure":  nil,
		}
	}

	return map[string]interface{}{
		"model_id":       modelID,
		"blocked":        entry.IsBlocked(),
		"failure_count":  entry.FailureCount,
		"last_failure":   entry.LastFailure,
		"blocked_until":  entry.BlockedUntil,
		"time_remaining": entry.GetTimeUntilUnblocked(),
	}
}

// GetAllStats returns statistics for all models
func (rlc *RateLimitCache) GetAllStats() map[string]interface{} {
	rlc.mu.RLock()
	defer rlc.mu.RUnlock()

	stats := make(map[string]interface{})
	for modelID, entry := range rlc.blockedModels {
		stats[modelID] = map[string]interface{}{
			"blocked":        entry.IsBlocked(),
			"failure_count":  entry.FailureCount,
			"last_failure":   entry.LastFailure,
			"blocked_until":  entry.BlockedUntil,
			"time_remaining": entry.GetTimeUntilUnblocked(),
		}
	}

	return stats
}

// Clear removes all blocked models
func (rlc *RateLimitCache) Clear() {
	rlc.mu.Lock()
	defer rlc.mu.Unlock()

	rlc.blockedModels = make(map[string]*RateLimitEntry)
	slog.Info("rate limit cache cleared")
}

// RemainingBlockDuration returns how long until a model becomes unblocked.
// Returns 0 if the model is not currently blocked or unknown.
func (rlc *RateLimitCache) RemainingBlockDuration(modelID string) time.Duration {
	rlc.mu.RLock()
	defer rlc.mu.RUnlock()

	entry, exists := rlc.blockedModels[modelID]
	if !exists || !entry.IsBlocked() {
		return 0
	}
	return entry.GetTimeUntilUnblocked()
}

// Stop stops the cleanup routine
func (rlc *RateLimitCache) Stop() {
	close(rlc.stopCleanup)
}

// getOrCreateEntry gets an existing entry or creates a new one
func (rlc *RateLimitCache) getOrCreateEntry(modelID string) *RateLimitEntry {
	entry, exists := rlc.blockedModels[modelID]
	if !exists {
		entry = &RateLimitEntry{
			ModelID:       modelID,
			BlockDuration: rlc.defaultDuration,
			MaxFailures:   rlc.maxFailures,
		}
		rlc.blockedModels[modelID] = entry
	}
	return entry
}

// cleanupRoutine periodically cleans up expired entries
func (rlc *RateLimitCache) cleanupRoutine() {
	ticker := time.NewTicker(rlc.cleanupInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			rlc.cleanup()
		case <-rlc.stopCleanup:
			return
		}
	}
}

// cleanup removes expired entries
func (rlc *RateLimitCache) cleanup() {
	rlc.mu.Lock()
	defer rlc.mu.Unlock()

	now := time.Now()
	expiredModels := make([]string, 0)

	for modelID, entry := range rlc.blockedModels {
		// Remove entries that are no longer blocked and have no recent failures
		if !entry.IsBlocked() && now.Sub(entry.LastFailure) > rlc.defaultDuration*2 {
			expiredModels = append(expiredModels, modelID)
		}
	}

	for _, modelID := range expiredModels {
		delete(rlc.blockedModels, modelID)
	}

	if len(expiredModels) > 0 {
		slog.Debug("cleaned up expired rate limit entries",
			slog.Int("count", len(expiredModels)))
	}
}

// SetBlockDuration sets the default block duration for new entries
func (rlc *RateLimitCache) SetBlockDuration(duration time.Duration) {
	rlc.mu.Lock()
	defer rlc.mu.Unlock()
	rlc.defaultDuration = duration
}

// SetMaxFailures sets the maximum failures before blocking
func (rlc *RateLimitCache) SetMaxFailures(maxFailures int) {
	rlc.mu.Lock()
	defer rlc.mu.Unlock()
	rlc.maxFailures = maxFailures
}
