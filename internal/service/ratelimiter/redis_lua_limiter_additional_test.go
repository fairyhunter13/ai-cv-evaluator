package ratelimiter

import (
	"context"
	"strconv"
	"testing"

	"github.com/redis/go-redis/v9"
)

func TestNewBucketConfigFromPerMinute(t *testing.T) {
	cfg := NewBucketConfigFromPerMinute(60)
	if cfg.Capacity != 60 {
		t.Fatalf("Capacity = %d, want 60", cfg.Capacity)
	}
	if cfg.RefillRate != 1.0 {
		t.Fatalf("RefillRate = %v, want 1.0", cfg.RefillRate)
	}

	zero := NewBucketConfigFromPerMinute(0)
	if zero.Capacity != 0 || zero.RefillRate != 0 {
		t.Fatalf("expected zero config for non-positive perMinute, got %+v", zero)
	}
}

func TestRedisLuaLimiter_SetBucketConfigNilSafe(_ *testing.T) {
	var limiter *RedisLuaLimiter
	limiter.SetBucketConfig("key", BucketConfig{Capacity: 1, RefillRate: 1})
}

func TestRedisLuaLimiter_MirrorToPostgresNilPool(_ *testing.T) {
	limiter := &RedisLuaLimiter{}
	limiter.mirrorToPostgres(context.Background(), "key", BucketConfig{Capacity: 1, RefillRate: 1}, 10, 123.45)
}

func TestToInt64AndToFloat64(t *testing.T) {
	if v := toInt64(int64(5)); v != 5 {
		t.Fatalf("toInt64(int64) = %d, want 5", v)
	}
	if v := toInt64(3); v != 3 {
		t.Fatalf("toInt64(int) = %d, want 3", v)
	}
	if v := toInt64(7.9); v != 7 {
		t.Fatalf("toInt64(float64) = %d, want 7", v)
	}
	if v := toInt64("not-a-number"); v != 0 {
		t.Fatalf("toInt64(string) = %d, want 0", v)
	}

	if v := toFloat64(float64(1.5)); v != 1.5 {
		t.Fatalf("toFloat64(float64) = %v, want 1.5", v)
	}
	if v := toFloat64(int64(2)); v != 2 {
		t.Fatalf("toFloat64(int64) = %v, want 2", v)
	}
	if v := toFloat64(3); v != 3 {
		t.Fatalf("toFloat64(int) = %v, want 3", v)
	}
	if v := toFloat64("nan"); !isNaN(v) {
		t.Fatalf("toFloat64(string) should return NaN, got %v", v)
	}
}

func isNaN(f float64) bool {
	return f != f
}

func TestAllow_ScriptError_FailOpen(t *testing.T) {
	ctx := context.Background()
	limiter, cleanup := newTestRedisLuaLimiter(t)
	// Close Redis before calling Allow so that the Lua script fails at runtime.
	cleanup()

	key := "bucket-script-error"
	limiter.SetBucketConfig(key, BucketConfig{Capacity: 1, RefillRate: 1})

	allowed, retryAfter, err := limiter.Allow(ctx, key, 1)
	if err == nil {
		t.Fatalf("expected error from script when redis is closed")
	}
	if !allowed {
		t.Fatalf("expected limiter to fail open on script error")
	}
	if retryAfter != 0 {
		t.Fatalf("expected zero retryAfter on script error, got %v", retryAfter)
	}
}

func TestAllow_UnexpectedScriptResult_FailOpen(t *testing.T) {
	ctx := context.Background()
	limiter, cleanup := newTestRedisLuaLimiter(t)
	defer cleanup()

	key := "bucket-unexpected-result"
	limiter.SetBucketConfig(key, BucketConfig{Capacity: 1, RefillRate: 1})

	// Force the script to return a single scalar instead of the expected 4-element array.
	limiter.script = redis.NewScript("return 1")

	allowed, retryAfter, err := limiter.Allow(ctx, key, 1)
	if err != nil {
		t.Fatalf("expected no error for unexpected script result, got %v", err)
	}
	if !allowed {
		t.Fatalf("expected limiter to fail open on unexpected script result")
	}
	if retryAfter != 0 {
		t.Fatalf("expected zero retryAfter on unexpected script result, got %v", retryAfter)
	}
}

func TestAllow_NonPositiveCostNormalizesToOne(t *testing.T) {
	ctx := context.Background()
	limiter, cleanup := newTestRedisLuaLimiter(t)
	defer cleanup()

	key := "bucket-nonpositive-cost"
	limiter.SetBucketConfig(key, BucketConfig{Capacity: 1, RefillRate: 1})

	allowed, retryAfter, err := limiter.Allow(ctx, key, 0)
	if err != nil {
		t.Fatalf("unexpected error from Allow with non-positive cost: %v", err)
	}
	if !allowed {
		t.Fatalf("expected request with non-positive cost to be allowed")
	}
	if retryAfter != 0 {
		t.Fatalf("expected zero retryAfter for non-positive cost, got %v", retryAfter)
	}

	// The underlying Lua script should have seen cost=1 and consumed the only token.
	val, err := limiter.redis.HGet(ctx, "rate:"+key, "tokens").Result()
	if err != nil {
		t.Fatalf("failed to read tokens from redis: %v", err)
	}
	tokens, err := strconv.ParseFloat(val, 64)
	if err != nil {
		t.Fatalf("failed to parse tokens value %q: %v", val, err)
	}
	if tokens != 0 {
		t.Fatalf("expected tokens=0 after non-positive cost normalized to 1, got %v", tokens)
	}
}

func TestWarmFromPostgres_NilLimiter(t *testing.T) {
	var limiter *RedisLuaLimiter
	err := limiter.WarmFromPostgres(context.Background())
	if err != nil {
		t.Fatalf("expected nil error for nil limiter, got %v", err)
	}
}

func TestWarmFromPostgres_NilPool(t *testing.T) {
	limiter := &RedisLuaLimiter{}
	err := limiter.WarmFromPostgres(context.Background())
	if err != nil {
		t.Fatalf("expected nil error for nil pool, got %v", err)
	}
}

func TestSetBucketConfig_InitializesMap(t *testing.T) {
	limiter := &RedisLuaLimiter{}
	limiter.SetBucketConfig("test-key", BucketConfig{Capacity: 10, RefillRate: 1.0})

	if limiter.buckets == nil {
		t.Fatal("expected buckets map to be initialized")
	}
	cfg, ok := limiter.buckets["test-key"]
	if !ok {
		t.Fatal("expected bucket config to be set")
	}
	if cfg.Capacity != 10 {
		t.Fatalf("expected capacity 10, got %d", cfg.Capacity)
	}
}

func TestToInt64_AllTypes(t *testing.T) {
	tests := []struct {
		input    interface{}
		expected int64
	}{
		{int64(100), 100},
		{int(50), 50},
		{float64(75.9), 75},
		{"string", 0},
		{nil, 0},
	}

	for _, tt := range tests {
		result := toInt64(tt.input)
		if result != tt.expected {
			t.Errorf("toInt64(%v) = %d, want %d", tt.input, result, tt.expected)
		}
	}
}

func TestToFloat64_AllTypes(t *testing.T) {
	tests := []struct {
		input    interface{}
		expected float64
		isNaN    bool
	}{
		{float64(1.5), 1.5, false},
		{int64(2), 2.0, false},
		{int(3), 3.0, false},
		{"string", 0, true},
	}

	for _, tt := range tests {
		result := toFloat64(tt.input)
		if tt.isNaN {
			if !isNaN(result) {
				t.Errorf("toFloat64(%v) should be NaN, got %v", tt.input, result)
			}
		} else if result != tt.expected {
			t.Errorf("toFloat64(%v) = %v, want %v", tt.input, result, tt.expected)
		}
	}
}
