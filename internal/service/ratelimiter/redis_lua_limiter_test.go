package ratelimiter

import (
	"context"
	"testing"

	miniredis "github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
)

func newTestRedisLuaLimiter(t *testing.T) (*RedisLuaLimiter, func()) {
	mr, err := miniredis.Run()
	if err != nil {
		t.Fatalf("failed to start miniredis: %v", err)
	}

	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	limiter := NewRedisLuaLimiter(rdb, nil, nil)

	cleanup := func() {
		_ = rdb.Close()
		mr.Close()
	}

	return limiter, cleanup
}

func TestAllow_NilLimiter_FailOpen(t *testing.T) {
	ctx := context.Background()
	var limiter *RedisLuaLimiter

	allowed, retryAfter, err := limiter.Allow(ctx, "any", 1)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if !allowed {
		t.Fatalf("expected allowed to be true for nil limiter")
	}
	if retryAfter != 0 {
		t.Fatalf("expected zero retryAfter, got %v", retryAfter)
	}
}

func TestAllow_NoBucketConfig_FailOpen(t *testing.T) {
	ctx := context.Background()
	limiter, cleanup := newTestRedisLuaLimiter(t)
	defer cleanup()

	allowed, retryAfter, err := limiter.Allow(ctx, "unknown-bucket", 1)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if !allowed {
		t.Fatalf("expected allowed to be true when no bucket config is present")
	}
	if retryAfter != 0 {
		t.Fatalf("expected zero retryAfter, got %v", retryAfter)
	}
}

func TestAllow_WithBucket_RespectsCapacityAndRetryAfter(t *testing.T) {
	ctx := context.Background()
	limiter, cleanup := newTestRedisLuaLimiter(t)
	defer cleanup()

	key := "test-bucket"
	limiter.SetBucketConfig(key, BucketConfig{
		Capacity:   3,
		RefillRate: 0.000001,
	})

	for i := 0; i < 3; i++ {
		allowed, retryAfter, err := limiter.Allow(ctx, key, 1)
		if err != nil {
			t.Fatalf("unexpected error on allowed call %d: %v", i, err)
		}
		if !allowed {
			t.Fatalf("expected allowed=true on call %d", i)
		}
		if retryAfter != 0 {
			t.Fatalf("expected retryAfter=0 on allowed call %d, got %v", i, retryAfter)
		}
	}

	allowed, retryAfter, err := limiter.Allow(ctx, key, 1)
	if err == nil {
		// limiter fails open on Redis script errors, but with miniredis we expect a proper result
		if allowed {
			t.Fatalf("expected limiter to deny once capacity exhausted")
		}
		if retryAfter <= 0 {
			t.Fatalf("expected positive retryAfter when capacity exhausted, got %v", retryAfter)
		}
	} else {
		// Even if Redis errors, limiter must fail open without panicking
		if !allowed {
			t.Fatalf("expected allowed=true when script error occurs, got false with err=%v", err)
		}
	}
}

func TestWarmFromPostgres_NoPoolOrRedis_NoError(t *testing.T) {
	limiter := &RedisLuaLimiter{}
	if err := limiter.WarmFromPostgres(context.Background()); err != nil {
		t.Fatalf("expected no error from WarmFromPostgres with nil pool/redis, got %v", err)
	}
}
