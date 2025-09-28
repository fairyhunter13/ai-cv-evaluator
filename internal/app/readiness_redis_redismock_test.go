//go:build redismock

package app

import (
	"context"
	"testing"

	redismock "github.com/go-redis/redismock/v9"
	"github.com/redis/go-redis/v9"
	"github.com/fairyhunter13/ai-cv-evaluator/internal/config"
)

type statusAdapter struct{ s *redis.StatusCmd }
func (s statusAdapter) Err() error { return s.s.Err() }

// clientAdapter adapts *redis.Client into our RedisClient
type clientAdapter struct{ c *redis.Client }
func (c clientAdapter) Ping(ctx context.Context) RedisPingResult { return statusAdapter{c.c.Ping(ctx)} }

func TestBuildReadinessChecks_Redismock(t *testing.T) {
	client, mock := redismock.NewClientMock()
	mock.ExpectPing().SetVal("PONG")
	db, red, _, _ := BuildReadinessChecks(config.Config{QdrantURL: "http://localhost", TikaURL: "http://localhost"}, nil, clientAdapter{client})
	if err := red(context.Background()); err != nil { t.Fatalf("redis check: %v", err) }
	if err := db(context.Background()); err == nil { t.Fatalf("expected db not configured error") }
	if err := mock.ExpectationsWereMet(); err != nil { t.Fatalf("redis expectations: %v", err) }
}
