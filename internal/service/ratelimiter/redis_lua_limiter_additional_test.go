package ratelimiter

import (
	"context"
	"testing"
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
