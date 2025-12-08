package real

import (
	"context"
	"net/http"
	"testing"
	"time"

	miniredis "github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"

	"github.com/fairyhunter13/ai-cv-evaluator/internal/service/ratelimiter"
)

func newTestLuaLimiter(t *testing.T) (*ratelimiter.RedisLuaLimiter, func()) {
	mr, err := miniredis.Run()
	if err != nil {
		t.Fatalf("failed to start miniredis: %v", err)
	}

	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	limiter := ratelimiter.NewRedisLuaLimiter(rdb, nil, nil)

	cleanup := func() {
		_ = rdb.Close()
		mr.Close()
	}

	return limiter, cleanup
}

func TestUpdateGroqLimiterFromHeaders_ConfiguresBucket(t *testing.T) {
	limiter, cleanup := newTestLuaLimiter(t)
	defer cleanup()

	c := &Client{limiter: limiter}

	h := http.Header{}
	h.Set("x-ratelimit-limit-requests", "2")

	apiKey := "test-groq-key" //nolint:gosec // Test credential.
	c.updateGroqLimiterFromHeaders(apiKey, h)

	ctx := context.Background()
	bucketKey := groqBucketKey(apiKey)

	for i := 0; i < 2; i++ {
		allowed, retryAfter, err := limiter.Allow(ctx, bucketKey, 1)
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

	allowed, retryAfter, err := limiter.Allow(ctx, bucketKey, 1)
	if err != nil {
		t.Fatalf("unexpected error on denied call: %v", err)
	}
	if allowed {
		t.Fatalf("expected limiter to deny once capacity exhausted")
	}
	if retryAfter <= 0 {
		t.Fatalf("expected positive retryAfter when capacity exhausted, got %v", retryAfter)
	}
}

func TestUpdateOpenRouterLimiterFromRetryAfter_ConfiguresBucket(t *testing.T) {
	limiter, cleanup := newTestLuaLimiter(t)
	defer cleanup()

	c := &Client{limiter: limiter}

	apiKey := "test-openrouter-key" //nolint:gosec // Test credential.
	d := 10 * time.Second
	c.updateOpenRouterLimiterFromRetryAfter(apiKey, d)

	ctx := context.Background()
	bucketKey := openRouterBucketKey(apiKey)

	allowed, retryAfter, err := limiter.Allow(ctx, bucketKey, 1)
	if err != nil {
		t.Fatalf("unexpected error on first allowed call: %v", err)
	}
	if !allowed {
		t.Fatalf("expected allowed=true on first call after configuration")
	}
	if retryAfter != 0 {
		t.Fatalf("expected retryAfter=0 on first call, got %v", retryAfter)
	}

	allowed, retryAfter, err = limiter.Allow(ctx, bucketKey, 1)
	if err != nil {
		t.Fatalf("unexpected error on second call: %v", err)
	}
	if allowed {
		t.Fatalf("expected limiter to deny second call after single-capacity bucket is spent")
	}
	if retryAfter <= 0 {
		t.Fatalf("expected positive retryAfter from OpenRouter-derived bucket, got %v", retryAfter)
	}
}
