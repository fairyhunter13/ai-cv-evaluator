package ai

import (
	"crypto/sha256"
	"encoding/hex"
	"sync"
	"time"

	"log/slog"
)

// CachedResponse represents a cached model response
type CachedResponse struct {
	Response    string
	Timestamp   time.Time
	ModelID     string
	AccessCount int
	TTL         time.Duration
}

// IsExpired checks if the cached response has expired
func (cr *CachedResponse) IsExpired() bool {
	return time.Since(cr.Timestamp) > cr.TTL
}

// ModelCache implements intelligent caching for AI model responses
type ModelCache struct {
	mu          sync.RWMutex
	cache       map[string]*CachedResponse
	maxSize     int
	defaultTTL  time.Duration
	accessCount map[string]int
	hitCount    int64
	missCount   int64
}

// NewModelCache creates a new model cache
func NewModelCache(maxSize int, defaultTTL time.Duration) *ModelCache {
	return &ModelCache{
		cache:       make(map[string]*CachedResponse),
		maxSize:     maxSize,
		defaultTTL:  defaultTTL,
		accessCount: make(map[string]int),
	}
}

// generateCacheKey creates a hash key for the input prompt
func (mc *ModelCache) generateCacheKey(systemPrompt, userPrompt string) string {
	// Combine system and user prompts for unique key
	combined := systemPrompt + "|" + userPrompt
	hash := sha256.Sum256([]byte(combined))
	return hex.EncodeToString(hash[:])
}

// Get retrieves a cached response if available and not expired
func (mc *ModelCache) Get(systemPrompt, userPrompt string) (string, bool) {
	key := mc.generateCacheKey(systemPrompt, userPrompt)

	mc.mu.RLock()
	cached, exists := mc.cache[key]
	mc.mu.RUnlock()

	if !exists {
		mc.mu.Lock()
		mc.missCount++
		mc.mu.Unlock()
		return "", false
	}

	// Check if expired
	if cached.IsExpired() {
		mc.mu.Lock()
		delete(mc.cache, key)
		mc.missCount++
		mc.mu.Unlock()
		return "", false
	}

	// Update access count and hit count
	mc.mu.Lock()
	cached.AccessCount++
	mc.accessCount[key]++
	mc.hitCount++
	mc.mu.Unlock()

	slog.Debug("cache hit",
		slog.String("key", key[:16]+"..."),
		slog.String("model", cached.ModelID),
		slog.Int("access_count", cached.AccessCount),
		slog.Duration("age", time.Since(cached.Timestamp)))

	return cached.Response, true
}

// Set stores a response in the cache
func (mc *ModelCache) Set(systemPrompt, userPrompt, response, modelID string) {
	key := mc.generateCacheKey(systemPrompt, userPrompt)

	mc.mu.Lock()
	defer mc.mu.Unlock()

	// Check if we need to evict entries
	if len(mc.cache) >= mc.maxSize {
		mc.evictLeastUsed()
	}

	// Store the response
	mc.cache[key] = &CachedResponse{
		Response:    response,
		Timestamp:   time.Now(),
		ModelID:     modelID,
		AccessCount: 1,
		TTL:         mc.defaultTTL,
	}

	slog.Debug("cache set",
		slog.String("key", key[:16]+"..."),
		slog.String("model", modelID),
		slog.Duration("ttl", mc.defaultTTL))
}

// SetWithTTL stores a response with a custom TTL
func (mc *ModelCache) SetWithTTL(systemPrompt, userPrompt, response, modelID string, ttl time.Duration) {
	key := mc.generateCacheKey(systemPrompt, userPrompt)

	mc.mu.Lock()
	defer mc.mu.Unlock()

	// Check if we need to evict entries
	if len(mc.cache) >= mc.maxSize {
		mc.evictLeastUsed()
	}

	// Store the response with custom TTL
	mc.cache[key] = &CachedResponse{
		Response:    response,
		Timestamp:   time.Now(),
		ModelID:     modelID,
		AccessCount: 1,
		TTL:         ttl,
	}

	slog.Debug("cache set with custom TTL",
		slog.String("key", key[:16]+"..."),
		slog.String("model", modelID),
		slog.Duration("ttl", ttl))
}

// evictLeastUsed removes the least recently used entry
func (mc *ModelCache) evictLeastUsed() {
	if len(mc.cache) == 0 {
		return
	}

	var oldestKey string
	var oldestTime time.Time
	var lowestAccessCount int

	// Find the entry with lowest access count and oldest timestamp
	for key, cached := range mc.cache {
		if oldestKey == "" ||
			cached.AccessCount < lowestAccessCount ||
			(cached.AccessCount == lowestAccessCount && cached.Timestamp.Before(oldestTime)) {
			oldestKey = key
			oldestTime = cached.Timestamp
			lowestAccessCount = cached.AccessCount
		}
	}

	if oldestKey != "" {
		delete(mc.cache, oldestKey)
		slog.Debug("evicted least used cache entry",
			slog.String("key", oldestKey[:16]+"..."),
			slog.Int("access_count", lowestAccessCount),
			slog.Duration("age", time.Since(oldestTime)))
	}
}

// Cleanup removes expired entries
func (mc *ModelCache) Cleanup() {
	mc.mu.Lock()
	defer mc.mu.Unlock()

	expiredKeys := make([]string, 0)
	for key, cached := range mc.cache {
		if cached.IsExpired() {
			expiredKeys = append(expiredKeys, key)
		}
	}

	for _, key := range expiredKeys {
		delete(mc.cache, key)
	}

	if len(expiredKeys) > 0 {
		slog.Debug("cleaned up expired cache entries",
			slog.Int("count", len(expiredKeys)))
	}
}

// GetStats returns cache statistics
func (mc *ModelCache) GetStats() map[string]interface{} {
	mc.mu.RLock()
	defer mc.mu.RUnlock()

	totalRequests := mc.hitCount + mc.missCount
	hitRate := 0.0
	if totalRequests > 0 {
		hitRate = float64(mc.hitCount) / float64(totalRequests)
	}

	return map[string]interface{}{
		"cache_size":     len(mc.cache),
		"max_size":       mc.maxSize,
		"hit_count":      mc.hitCount,
		"miss_count":     mc.missCount,
		"total_requests": totalRequests,
		"hit_rate":       hitRate,
		"default_ttl":    mc.defaultTTL,
	}
}

// Clear removes all cached entries
func (mc *ModelCache) Clear() {
	mc.mu.Lock()
	defer mc.mu.Unlock()

	mc.cache = make(map[string]*CachedResponse)
	mc.accessCount = make(map[string]int)
	mc.hitCount = 0
	mc.missCount = 0

	slog.Info("model cache cleared")
}
