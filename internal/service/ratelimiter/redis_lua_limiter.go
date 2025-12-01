package ratelimiter

import (
	"context"
	"log/slog"
	"math"
	"sync"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"
)

type Limiter interface {
	Allow(ctx context.Context, key string, cost int64) (allowed bool, retryAfter time.Duration, err error)
}

type BucketConfig struct {
	Capacity   int64
	RefillRate float64
}

func NewBucketConfigFromPerMinute(perMinute int) BucketConfig {
	if perMinute <= 0 {
		return BucketConfig{}
	}
	return BucketConfig{
		Capacity:   int64(perMinute),
		RefillRate: float64(perMinute) / 60.0,
	}
}

type RedisLuaLimiter struct {
	redis   *redis.Client
	pool    *pgxpool.Pool
	buckets map[string]BucketConfig
	script  *redis.Script
	mu      sync.RWMutex
}

func NewRedisLuaLimiter(rdb *redis.Client, pool *pgxpool.Pool, buckets map[string]BucketConfig) *RedisLuaLimiter {
	if rdb == nil {
		return nil
	}
	if buckets == nil {
		buckets = map[string]BucketConfig{}
	}
	return &RedisLuaLimiter{
		redis:   rdb,
		pool:    pool,
		buckets: buckets,
		script:  redis.NewScript(luaTokenBucketScript),
	}
}

const luaTokenBucketScript = `
local key = KEYS[1]
local capacity = tonumber(ARGV[1])
local refill_rate = tonumber(ARGV[2])
local now = tonumber(ARGV[3])
local cost = tonumber(ARGV[4])

local tokens = capacity
local last_refill = now

local data = redis.call("HMGET", key, "tokens", "last_refill")
if data[1] ~= false and data[1] ~= nil then
  tokens = tonumber(data[1])
end
if data[2] ~= false and data[2] ~= nil then
  last_refill = tonumber(data[2])
end

if last_refill == nil then
  last_refill = now
end

local delta = now - last_refill
if delta < 0 then
  delta = 0
end

tokens = math.min(capacity, tokens + delta * refill_rate)
last_refill = now

local allowed = 0
local retry_after = 0

if tokens >= cost then
  tokens = tokens - cost
  allowed = 1
else
  local shortage = cost - tokens
  if refill_rate > 0 then
    retry_after = shortage / refill_rate
  else
    retry_after = 0
  end
end

redis.call("HMSET", key, "tokens", tokens, "last_refill", last_refill)

return { allowed, tokens, last_refill, retry_after }
`

func (l *RedisLuaLimiter) Allow(ctx context.Context, key string, cost int64) (bool, time.Duration, error) {
	if l == nil || l.redis == nil {
		return true, 0, nil
	}
	l.mu.RLock()
	cfg, ok := l.buckets[key]
	l.mu.RUnlock()
	if !ok || cfg.Capacity <= 0 || cfg.RefillRate <= 0 {
		return true, 0, nil
	}
	if cost <= 0 {
		cost = 1
	}

	now := time.Now()
	nowSec := float64(now.UnixNano()) / 1e9

	redisKey := "rate:" + key
	res, err := l.script.Run(ctx, l.redis, []string{redisKey}, cfg.Capacity, cfg.RefillRate, nowSec, cost).Result()
	if err != nil {
		slog.Error("redis rate limiter script error", slog.String("key", key), slog.Any("error", err))
		// Fail open on Redis errors to avoid hard outages; provider 4xx/429 handling still applies separately.
		return true, 0, err
	}

	vals, ok := res.([]interface{})
	if !ok || len(vals) < 4 {
		slog.Error("redis rate limiter unexpected script result", slog.String("key", key), slog.Any("result", res))
		return true, 0, nil
	}

	allowed := toInt64(vals[0]) == 1
	tokens := toFloat64(vals[1])
	lastRefill := toFloat64(vals[2])
	retryAfterSec := toFloat64(vals[3])
	retryAfter := time.Duration(retryAfterSec * float64(time.Second))

	if l.pool != nil {
		l.mirrorToPostgres(ctx, key, cfg, tokens, lastRefill)
	}

	return allowed, retryAfter, nil
}

func (l *RedisLuaLimiter) mirrorToPostgres(ctx context.Context, key string, cfg BucketConfig, tokens, lastRefillSec float64) {
	if l.pool == nil {
		return
	}

	sec := int64(lastRefillSec)
	nsec := int64((lastRefillSec - float64(sec)) * 1e9)
	if nsec < 0 {
		nsec = 0
	}
	lastRefill := time.Unix(sec, nsec)

	_, err := l.pool.Exec(ctx,
		`INSERT INTO rate_limit_buckets (bucket_key, capacity, refill_rate, tokens, last_refill)
		 VALUES ($1, $2, $3, $4, $5)
		 ON CONFLICT (bucket_key) DO UPDATE SET
		   capacity = EXCLUDED.capacity,
		   refill_rate = EXCLUDED.refill_rate,
		   tokens = EXCLUDED.tokens,
		   last_refill = EXCLUDED.last_refill`,
		key, cfg.Capacity, cfg.RefillRate, tokens, lastRefill,
	)
	if err != nil {
		slog.Error("failed to mirror rate limit bucket to postgres", slog.String("key", key), slog.Any("error", err))
	}
}

func (l *RedisLuaLimiter) WarmFromPostgres(ctx context.Context) error {
	if l == nil || l.pool == nil || l.redis == nil {
		return nil
	}

	rows, err := l.pool.Query(ctx, `SELECT bucket_key, tokens, EXTRACT(EPOCH FROM last_refill) FROM rate_limit_buckets`)
	if err != nil {
		return err
	}
	defer rows.Close()

	for rows.Next() {
		var key string
		var tokens float64
		var lastRefillSec float64
		if err := rows.Scan(&key, &tokens, &lastRefillSec); err != nil {
			return err
		}
		redisKey := "rate:" + key
		sec := int64(lastRefillSec)
		nsec := int64((lastRefillSec - float64(sec)) * 1e9)
		if nsec < 0 {
			nsec = 0
		}
		// Store seconds-with-fraction in the same representation used by Lua script (seconds as float).
		storedLastRefill := float64(sec) + float64(nsec)/1e9
		if err := l.redis.HMSet(ctx, redisKey, "tokens", tokens, "last_refill", storedLastRefill).Err(); err != nil {
			slog.Error("failed to warm Redis bucket from postgres", slog.String("key", key), slog.Any("error", err))
		}
	}
	return rows.Err()
}

// SetBucketConfig updates or creates the bucket configuration for the given logical key.
// This allows callers (e.g. AI clients) to adjust capacity/refill dynamically based on
// provider rate limit headers. It is safe for concurrent use.
func (l *RedisLuaLimiter) SetBucketConfig(key string, cfg BucketConfig) {
	if l == nil {
		return
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.buckets == nil {
		l.buckets = map[string]BucketConfig{}
	}
	l.buckets[key] = cfg
}

func toInt64(v interface{}) int64 {
	switch t := v.(type) {
	case int64:
		return t
	case int:
		return int64(t)
	case float64:
		return int64(t)
	case string:
		return 0
	default:
		return 0
	}
}

func toFloat64(v interface{}) float64 {
	switch t := v.(type) {
	case float64:
		return t
	case int64:
		return float64(t)
	case int:
		return float64(t)
	default:
		return math.NaN()
	}
}
