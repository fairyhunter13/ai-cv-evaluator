package app

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/fairyhunter13/ai-cv-evaluator/internal/config"
)

// Pinger is the minimal interface for a database pool capable of Ping.
type Pinger interface{ Ping(ctx context.Context) error }

// RedisPingResult is the minimal return type of a Redis client's Ping.
type RedisPingResult interface{ Err() error }
// RedisClient is the minimal interface for a Redis client needed for readiness.
type RedisClient interface{ Ping(ctx context.Context) RedisPingResult }

// BuildReadinessChecks returns four readiness checks: db, redis, qdrant, and tika.
func BuildReadinessChecks(cfg config.Config, pool Pinger, rdb RedisClient) (
	func(ctx context.Context) error,
	func(ctx context.Context) error,
	func(ctx context.Context) error,
	func(ctx context.Context) error,
) {
	dbCheck := func(ctx context.Context) error {
		if pool == nil { return fmt.Errorf("db not configured") }
		return pool.Ping(ctx)
	}
	redisCheck := func(ctx context.Context) error {
		if rdb == nil { return fmt.Errorf("redis not configured") }
		return rdb.Ping(ctx).Err()
	}
	qdrantCheck := func(ctx context.Context) error {
		client := &http.Client{Timeout: 2 * time.Second}
		req, _ := http.NewRequestWithContext(ctx, http.MethodGet, cfg.QdrantURL+"/collections", nil)
		if cfg.QdrantAPIKey != "" { req.Header.Set("api-key", cfg.QdrantAPIKey) }
		resp, err := client.Do(req)
		if err != nil { return err }
		defer func(){ _ = resp.Body.Close() }()
		if resp.StatusCode >= 200 && resp.StatusCode < 300 { return nil }
		return fmt.Errorf("qdrant status %d", resp.StatusCode)
	}
	tikaCheck := func(ctx context.Context) error {
		if cfg.TikaURL == "" { return fmt.Errorf("tika url not configured") }
		client := &http.Client{Timeout: 2 * time.Second}
		req, _ := http.NewRequestWithContext(ctx, http.MethodGet, cfg.TikaURL+"/version", nil)
		resp, err := client.Do(req)
		if err != nil { return err }
		defer func(){ _ = resp.Body.Close() }()
		if resp.StatusCode >= 200 && resp.StatusCode < 300 { return nil }
		return fmt.Errorf("tika status %d", resp.StatusCode)
	}
	return dbCheck, redisCheck, qdrantCheck, tikaCheck
}
