package ai

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRateLimitEntry_IsBlocked(t *testing.T) {
	tests := []struct {
		name         string
		blockedUntil time.Time
		expected     bool
	}{
		{
			name:         "not blocked - past time",
			blockedUntil: time.Now().Add(-1 * time.Hour),
			expected:     false,
		},
		{
			name:         "blocked - future time",
			blockedUntil: time.Now().Add(1 * time.Hour),
			expected:     true,
		},
		{
			name:         "not blocked - zero time",
			blockedUntil: time.Time{},
			expected:     false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			entry := &RateLimitEntry{
				ModelID:      "test-model",
				BlockedUntil: tt.blockedUntil,
			}
			assert.Equal(t, tt.expected, entry.IsBlocked())
		})
	}
}

func TestRateLimitEntry_ShouldBlock(t *testing.T) {
	tests := []struct {
		name         string
		failureCount int
		maxFailures  int
		expected     bool
	}{
		{
			name:         "should not block - below threshold",
			failureCount: 2,
			maxFailures:  5,
			expected:     false,
		},
		{
			name:         "should block - at threshold",
			failureCount: 5,
			maxFailures:  5,
			expected:     true,
		},
		{
			name:         "should block - above threshold",
			failureCount: 10,
			maxFailures:  5,
			expected:     true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			entry := &RateLimitEntry{
				ModelID:      "test-model",
				FailureCount: tt.failureCount,
				MaxFailures:  tt.maxFailures,
			}
			assert.Equal(t, tt.expected, entry.ShouldBlock())
		})
	}
}

func TestRateLimitEntry_RecordFailure(t *testing.T) {
	t.Run("increments failure count", func(t *testing.T) {
		entry := &RateLimitEntry{
			ModelID:       "test-model",
			MaxFailures:   5,
			BlockDuration: 10 * time.Second,
		}
		entry.RecordFailure()
		assert.Equal(t, 1, entry.FailureCount)
		assert.False(t, entry.IsBlocked())
	})

	t.Run("blocks when threshold reached", func(t *testing.T) {
		entry := &RateLimitEntry{
			ModelID:       "test-model",
			FailureCount:  4,
			MaxFailures:   5,
			BlockDuration: 10 * time.Second,
		}
		entry.RecordFailure()
		assert.Equal(t, 5, entry.FailureCount)
		assert.True(t, entry.IsBlocked())
	})

	t.Run("exponential backoff increases block duration", func(t *testing.T) {
		entry := &RateLimitEntry{
			ModelID:       "test-model",
			FailureCount:  4,
			MaxFailures:   5,
			BlockDuration: 10 * time.Second,
		}
		entry.RecordFailure() // First block
		firstBlockedUntil := entry.BlockedUntil

		entry.RecordFailure() // Second failure while blocked
		secondBlockedUntil := entry.BlockedUntil

		// Second block should be longer
		assert.True(t, secondBlockedUntil.After(firstBlockedUntil))
	})

	t.Run("caps block duration at 2 hours", func(t *testing.T) {
		entry := &RateLimitEntry{
			ModelID:       "test-model",
			FailureCount:  14, // Already exceeded
			MaxFailures:   5,
			BlockDuration: 1 * time.Hour,
		}
		entry.RecordFailure()

		// Block should be capped at 2 hours
		expectedMax := time.Now().Add(2*time.Hour + time.Minute)
		assert.True(t, entry.BlockedUntil.Before(expectedMax))
	})
}

func TestRateLimitEntry_RecordSuccess(t *testing.T) {
	t.Run("resets failure count", func(t *testing.T) {
		entry := &RateLimitEntry{
			ModelID:      "test-model",
			FailureCount: 3,
		}
		entry.RecordSuccess()
		assert.Equal(t, 0, entry.FailureCount)
	})

	t.Run("clears block", func(t *testing.T) {
		entry := &RateLimitEntry{
			ModelID:      "test-model",
			FailureCount: 5,
			BlockedUntil: time.Now().Add(1 * time.Hour),
		}
		entry.RecordSuccess()
		assert.False(t, entry.IsBlocked())
		assert.True(t, entry.BlockedUntil.IsZero())
	})
}

func TestRateLimitEntry_GetTimeUntilUnblocked(t *testing.T) {
	t.Run("returns zero when not blocked", func(t *testing.T) {
		entry := &RateLimitEntry{
			ModelID:      "test-model",
			BlockedUntil: time.Now().Add(-1 * time.Hour),
		}
		assert.Equal(t, time.Duration(0), entry.GetTimeUntilUnblocked())
	})

	t.Run("returns remaining time when blocked", func(t *testing.T) {
		blockDuration := 1 * time.Hour
		entry := &RateLimitEntry{
			ModelID:      "test-model",
			BlockedUntil: time.Now().Add(blockDuration),
		}
		remaining := entry.GetTimeUntilUnblocked()
		assert.True(t, remaining > 59*time.Minute)
		assert.True(t, remaining <= blockDuration)
	})
}

func TestNewRateLimitCache(t *testing.T) {
	cache := NewRateLimitCache()
	require.NotNil(t, cache)
	assert.NotNil(t, cache.blockedModels)
	assert.Equal(t, 20*time.Second, cache.defaultDuration)
	assert.Equal(t, 5, cache.maxFailures)
	cache.Stop() // Cleanup
}

func TestRateLimitCache_IsModelBlocked(t *testing.T) {
	cache := NewRateLimitCache()
	defer cache.Stop()

	// Unknown model should not be blocked
	assert.False(t, cache.IsModelBlocked("unknown-model"))

	// Block a model
	cache.BlockModel("test-model", 1*time.Hour)
	assert.True(t, cache.IsModelBlocked("test-model"))

	// Clear and test again
	cache.Clear()
	assert.False(t, cache.IsModelBlocked("test-model"))
}

func TestRateLimitCache_BlockModel(t *testing.T) {
	cache := NewRateLimitCache()
	defer cache.Stop()

	cache.BlockModel("test-model", 1*time.Hour)
	assert.True(t, cache.IsModelBlocked("test-model"))
}

func TestRateLimitCache_RecordFailure(t *testing.T) {
	cache := NewRateLimitCache()
	defer cache.Stop()

	// Record failures below threshold
	for i := 0; i < 4; i++ {
		cache.RecordFailure("test-model")
		assert.False(t, cache.IsModelBlocked("test-model"))
	}

	// One more failure should trigger block
	cache.RecordFailure("test-model")
	assert.True(t, cache.IsModelBlocked("test-model"))
}

func TestRateLimitCache_RecordRateLimit(t *testing.T) {
	cache := NewRateLimitCache()
	defer cache.Stop()

	// Record rate limit with specific retry-after
	cache.RecordRateLimit("test-model", 30*time.Second)
	assert.True(t, cache.IsModelBlocked("test-model"))

	// Record rate limit with zero duration (should use default)
	cache.Clear()
	cache.RecordRateLimit("test-model-2", 0)
	assert.True(t, cache.IsModelBlocked("test-model-2"))
}

func TestRateLimitCache_RecordSuccess(t *testing.T) {
	cache := NewRateLimitCache()
	defer cache.Stop()

	// Block a model
	cache.BlockModel("test-model", 1*time.Hour)
	assert.True(t, cache.IsModelBlocked("test-model"))

	// Record success should unblock
	cache.RecordSuccess("test-model")
	assert.False(t, cache.IsModelBlocked("test-model"))
}

func TestRateLimitCache_GetBlockedModels(t *testing.T) {
	cache := NewRateLimitCache()
	defer cache.Stop()

	// Empty initially
	blocked := cache.GetBlockedModels()
	assert.Empty(t, blocked)

	// Block some models
	cache.BlockModel("model-1", 1*time.Hour)
	cache.BlockModel("model-2", 1*time.Hour)

	blocked = cache.GetBlockedModels()
	assert.Len(t, blocked, 2)
	assert.Contains(t, blocked, "model-1")
	assert.Contains(t, blocked, "model-2")
}

func TestRateLimitCache_GetAvailableModels(t *testing.T) {
	cache := NewRateLimitCache()
	defer cache.Stop()

	allModels := []string{"model-1", "model-2", "model-3"}

	// All available initially
	available := cache.GetAvailableModels(allModels)
	assert.Len(t, available, 3)

	// Block one model
	cache.BlockModel("model-2", 1*time.Hour)
	available = cache.GetAvailableModels(allModels)
	assert.Len(t, available, 2)
	assert.Contains(t, available, "model-1")
	assert.Contains(t, available, "model-3")
	assert.NotContains(t, available, "model-2")
}

func TestRateLimitCache_GetModelStatus(t *testing.T) {
	cache := NewRateLimitCache()
	defer cache.Stop()

	// Unknown model status
	status := cache.GetModelStatus("unknown")
	assert.Equal(t, "unknown", status["model_id"])
	assert.False(t, status["blocked"].(bool))
	assert.Equal(t, 0, status["failure_count"])

	// Blocked model status
	cache.BlockModel("test-model", 1*time.Hour)
	status = cache.GetModelStatus("test-model")
	assert.Equal(t, "test-model", status["model_id"])
	assert.True(t, status["blocked"].(bool))
	assert.NotNil(t, status["blocked_until"])
}

func TestRateLimitCache_GetAllStats(t *testing.T) {
	cache := NewRateLimitCache()
	defer cache.Stop()

	// Empty initially
	stats := cache.GetAllStats()
	assert.Empty(t, stats)

	// Add some entries
	cache.RecordFailure("model-1")
	cache.BlockModel("model-2", 1*time.Hour)

	stats = cache.GetAllStats()
	assert.Len(t, stats, 2)
	assert.Contains(t, stats, "model-1")
	assert.Contains(t, stats, "model-2")
}

func TestRateLimitCache_Clear(t *testing.T) {
	cache := NewRateLimitCache()
	defer cache.Stop()

	cache.BlockModel("model-1", 1*time.Hour)
	cache.BlockModel("model-2", 1*time.Hour)

	cache.Clear()

	assert.False(t, cache.IsModelBlocked("model-1"))
	assert.False(t, cache.IsModelBlocked("model-2"))
	assert.Empty(t, cache.GetBlockedModels())
}

func TestRateLimitCache_RemainingBlockDuration(t *testing.T) {
	cache := NewRateLimitCache()
	defer cache.Stop()

	// Unknown model
	assert.Equal(t, time.Duration(0), cache.RemainingBlockDuration("unknown"))

	// Blocked model
	cache.BlockModel("test-model", 1*time.Hour)
	remaining := cache.RemainingBlockDuration("test-model")
	assert.True(t, remaining > 59*time.Minute)
}

func TestRateLimitCache_SetBlockDuration(t *testing.T) {
	cache := NewRateLimitCache()
	defer cache.Stop()

	cache.SetBlockDuration(5 * time.Minute)
	assert.Equal(t, 5*time.Minute, cache.defaultDuration)
}

func TestRateLimitCache_SetMaxFailures(t *testing.T) {
	cache := NewRateLimitCache()
	defer cache.Stop()

	cache.SetMaxFailures(10)
	assert.Equal(t, 10, cache.maxFailures)
}

func TestRateLimitCache_ConcurrentAccess(t *testing.T) {
	cache := NewRateLimitCache()
	defer cache.Stop()

	done := make(chan bool)

	// Concurrent writers
	for i := 0; i < 10; i++ {
		go func(id int) {
			for j := 0; j < 50; j++ {
				cache.RecordFailure("model-concurrent")
				cache.RecordSuccess("model-concurrent")
				cache.RecordRateLimit("model-rate", time.Second)
			}
			done <- true
		}(i)
	}

	// Concurrent readers
	for i := 0; i < 10; i++ {
		go func(id int) {
			for j := 0; j < 50; j++ {
				cache.IsModelBlocked("model-concurrent")
				cache.GetBlockedModels()
				cache.GetAllStats()
				cache.GetModelStatus("model-concurrent")
			}
			done <- true
		}(i)
	}

	// Wait for all
	for i := 0; i < 20; i++ {
		<-done
	}
}
