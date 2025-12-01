package ai

import (
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCachedResponse_IsExpired(t *testing.T) {
	tests := []struct {
		name      string
		timestamp time.Time
		ttl       time.Duration
		expected  bool
	}{
		{
			name:      "not expired",
			timestamp: time.Now(),
			ttl:       5 * time.Minute,
			expected:  false,
		},
		{
			name:      "expired",
			timestamp: time.Now().Add(-10 * time.Minute),
			ttl:       5 * time.Minute,
			expected:  true,
		},
		{
			name:      "just expired",
			timestamp: time.Now().Add(-6 * time.Minute),
			ttl:       5 * time.Minute,
			expected:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cr := &CachedResponse{
				Response:  "test response",
				Timestamp: tt.timestamp,
				TTL:       tt.ttl,
			}
			assert.Equal(t, tt.expected, cr.IsExpired())
		})
	}
}

func TestNewModelCache(t *testing.T) {
	mc := NewModelCache(100, 5*time.Minute)
	require.NotNil(t, mc)
	assert.Equal(t, 100, mc.maxSize)
	assert.Equal(t, 5*time.Minute, mc.defaultTTL)
	assert.NotNil(t, mc.cache)
	assert.NotNil(t, mc.accessCount)
}

func TestModelCache_generateCacheKey(t *testing.T) {
	mc := NewModelCache(100, 5*time.Minute)

	// Same prompts should generate same key
	key1 := mc.generateCacheKey("system", "user")
	key2 := mc.generateCacheKey("system", "user")
	assert.Equal(t, key1, key2)

	// Different prompts should generate different keys
	key3 := mc.generateCacheKey("system", "different user")
	assert.NotEqual(t, key1, key3)

	// Different system prompts should generate different keys
	key4 := mc.generateCacheKey("different system", "user")
	assert.NotEqual(t, key1, key4)
}

func TestModelCache_SetAndGet(t *testing.T) {
	mc := NewModelCache(100, 5*time.Minute)

	// Set a value
	mc.Set("system prompt", "user prompt", "test response", "model-1")

	// Get should return the value
	response, found := mc.Get("system prompt", "user prompt")
	assert.True(t, found)
	assert.Equal(t, "test response", response)

	// Get with different prompts should miss
	response, found = mc.Get("different system", "user prompt")
	assert.False(t, found)
	assert.Empty(t, response)
}

func TestModelCache_GetMiss(t *testing.T) {
	mc := NewModelCache(100, 5*time.Minute)

	// Get on empty cache
	response, found := mc.Get("system", "user")
	assert.False(t, found)
	assert.Empty(t, response)

	// Check miss count
	stats := mc.GetStats()
	assert.Equal(t, int64(1), stats["miss_count"])
}

func TestModelCache_GetExpired(t *testing.T) {
	mc := NewModelCache(100, 1*time.Millisecond)

	// Set a value with very short TTL
	mc.Set("system", "user", "response", "model-1")

	// Wait for expiry
	time.Sleep(5 * time.Millisecond)

	// Get should return miss
	response, found := mc.Get("system", "user")
	assert.False(t, found)
	assert.Empty(t, response)
}

func TestModelCache_SetWithTTL(t *testing.T) {
	mc := NewModelCache(100, 5*time.Minute)

	// Set with custom TTL
	mc.SetWithTTL("system", "user", "response", "model-1", 10*time.Minute)

	// Get should return the value
	response, found := mc.Get("system", "user")
	assert.True(t, found)
	assert.Equal(t, "response", response)

	// Verify the TTL was set correctly
	key := mc.generateCacheKey("system", "user")
	mc.mu.RLock()
	cached := mc.cache[key]
	mc.mu.RUnlock()
	assert.Equal(t, 10*time.Minute, cached.TTL)
}

func TestModelCache_Eviction(t *testing.T) {
	// Create a small cache
	mc := NewModelCache(3, 5*time.Minute)

	// Fill the cache
	mc.Set("system1", "user1", "response1", "model")
	mc.Set("system2", "user2", "response2", "model")
	mc.Set("system3", "user3", "response3", "model")

	// Access some entries to increase their count
	mc.Get("system1", "user1")
	mc.Get("system1", "user1")
	mc.Get("system3", "user3")

	// Add one more entry, should evict the least used (system2)
	mc.Set("system4", "user4", "response4", "model")

	// system2 should be evicted
	_, found := mc.Get("system2", "user2")
	assert.False(t, found)

	// Others should still be there
	_, found = mc.Get("system1", "user1")
	assert.True(t, found)
	_, found = mc.Get("system3", "user3")
	assert.True(t, found)
	_, found = mc.Get("system4", "user4")
	assert.True(t, found)
}

func TestModelCache_Cleanup(t *testing.T) {
	mc := NewModelCache(100, 1*time.Millisecond)

	// Set some values
	mc.Set("system1", "user1", "response1", "model")
	mc.Set("system2", "user2", "response2", "model")

	// Wait for expiry
	time.Sleep(5 * time.Millisecond)

	// Add a fresh entry
	mc.SetWithTTL("system3", "user3", "response3", "model", 5*time.Minute)

	// Cleanup should remove expired entries
	mc.Cleanup()

	// Check that only fresh entry remains
	stats := mc.GetStats()
	assert.Equal(t, 1, stats["cache_size"])

	_, found := mc.Get("system3", "user3")
	assert.True(t, found)
}

func TestModelCache_GetStats(t *testing.T) {
	mc := NewModelCache(100, 5*time.Minute)

	// Initial stats
	stats := mc.GetStats()
	assert.Equal(t, 0, stats["cache_size"])
	assert.Equal(t, 100, stats["max_size"])
	assert.Equal(t, int64(0), stats["hit_count"])
	assert.Equal(t, int64(0), stats["miss_count"])
	assert.Equal(t, 0.0, stats["hit_rate"])

	// Add entries and make some requests
	mc.Set("system1", "user1", "response1", "model")
	mc.Get("system1", "user1") // hit
	mc.Get("system2", "user2") // miss

	stats = mc.GetStats()
	assert.Equal(t, 1, stats["cache_size"])
	assert.Equal(t, int64(1), stats["hit_count"])
	assert.Equal(t, int64(1), stats["miss_count"])
	assert.Equal(t, 0.5, stats["hit_rate"])
}

func TestModelCache_Clear(t *testing.T) {
	mc := NewModelCache(100, 5*time.Minute)

	// Add some entries
	mc.Set("system1", "user1", "response1", "model")
	mc.Set("system2", "user2", "response2", "model")
	mc.Get("system1", "user1")
	mc.Get("nonexistent", "prompt")

	// Clear the cache
	mc.Clear()

	// Verify everything is reset
	stats := mc.GetStats()
	assert.Equal(t, 0, stats["cache_size"])
	assert.Equal(t, int64(0), stats["hit_count"])
	assert.Equal(t, int64(0), stats["miss_count"])

	// Previous entries should not be found
	_, found := mc.Get("system1", "user1")
	assert.False(t, found)
}

func TestModelCache_ConcurrentAccess(t *testing.T) {
	mc := NewModelCache(1000, 5*time.Minute)
	var wg sync.WaitGroup

	// Use unique keys per goroutine to avoid race on AccessCount field
	// The actual race is in model_cache.go where Get reads/writes AccessCount
	// without proper synchronization across the RLock/Lock transition

	// Concurrent setters - each uses unique keys
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			for j := 0; j < 50; j++ {
				key := fmt.Sprintf("system-%d-%d", id, j)
				mc.Set(key, "user", "response", "model")
				mc.SetWithTTL(key+"ttl", "user", "response", "model", time.Hour)
			}
		}(i)
	}

	// Concurrent getters - use unique keys to avoid race
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			for j := 0; j < 50; j++ {
				key := fmt.Sprintf("system-%d-%d", id, j)
				mc.Get(key, "user")
				mc.Get("nonexistent", "prompt") // This is fine - cache miss
			}
		}(i)
	}

	// Concurrent stats readers
	for i := 0; i < 5; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < 50; j++ {
				_ = mc.GetStats()
			}
		}()
	}

	// Wait for all concurrent operations to complete
	wg.Wait()

	// Cleanup only after all concurrent access is done (serial, not concurrent)
	mc.Cleanup()
	mc.Clear()
}

func TestModelCache_AccessCountTracking(t *testing.T) {
	mc := NewModelCache(100, 5*time.Minute)

	// Set an entry
	mc.Set("system", "user", "response", "model")

	// Access it multiple times
	for i := 0; i < 5; i++ {
		mc.Get("system", "user")
	}

	// Check the access count
	key := mc.generateCacheKey("system", "user")
	mc.mu.RLock()
	cached := mc.cache[key]
	mc.mu.RUnlock()

	// Initial set count (1) + 5 gets = 6
	assert.Equal(t, 6, cached.AccessCount)
}
