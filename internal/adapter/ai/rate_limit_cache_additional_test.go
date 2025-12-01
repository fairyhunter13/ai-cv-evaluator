package ai

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestRateLimitCache_CleanupRemovesExpiredEntries(t *testing.T) {
	cache := NewRateLimitCache()
	defer cache.Stop()

	cache.mu.Lock()
	stale := &RateLimitEntry{
		ModelID:       "stale-model",
		BlockedUntil:  time.Now().Add(-time.Hour),
		LastFailure:   time.Now().Add(-cache.defaultDuration * 3),
		BlockDuration: cache.defaultDuration,
		MaxFailures:   cache.maxFailures,
	}
	cache.blockedModels["stale-model"] = stale
	cache.mu.Unlock()

	cache.cleanup()

	cache.mu.RLock()
	_, exists := cache.blockedModels["stale-model"]
	cache.mu.RUnlock()

	require.False(t, exists, "expected cleanup to remove expired entry")
}
