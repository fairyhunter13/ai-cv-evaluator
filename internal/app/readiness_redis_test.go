package app

import (
	"context"
	"testing"
	"github.com/fairyhunter13/ai-cv-evaluator/internal/config"
)

type okPing struct{}
func (okPing) Err() error { return nil }
type errPing struct{ err error }
func (e errPing) Err() error { return e.err }
type fakeRedis struct{ ok bool; err error }
func (f fakeRedis) Ping(_ context.Context) RedisPingResult { if f.ok { return okPing{} }; return errPing{err: f.err} }

func TestBuildReadinessChecks_Redis_Success(t *testing.T) {
    client := fakeRedis{ok: true}
    db, red, _, _ := BuildReadinessChecks(config.Config{QdrantURL: "http://localhost", TikaURL: "http://localhost"}, nil, client)
    if err := red(context.Background()); err != nil { t.Fatalf("redis check: %v", err) }
    // db nil should error
    if err := db(context.Background()); err == nil { t.Fatalf("expected db not configured error") }
}

func TestBuildReadinessChecks_Redis_Error(t *testing.T) {
    client := fakeRedis{ok: false, err: context.DeadlineExceeded}
    _, red, _, _ := BuildReadinessChecks(config.Config{QdrantURL: "http://localhost", TikaURL: "http://localhost"}, nil, client)
    if err := red(context.Background()); err == nil { t.Fatalf("expected redis error") }
}
