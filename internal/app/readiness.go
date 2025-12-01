// Package app wires application components and startup helpers.
//
// It provides dependency injection and application initialization.
// The package coordinates between different layers and provides
// a clean application bootstrap process.
package app

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/fairyhunter13/ai-cv-evaluator/internal/config"
	"go.opentelemetry.io/otel/attribute"
)

// Pinger is the minimal interface for a database pool capable of Ping.
type Pinger interface {
	Ping(ctx context.Context) error
}

// BuildReadinessChecks returns three readiness checks: db, qdrant, and tika.
func BuildReadinessChecks(cfg config.Config, pool Pinger) (
	func(ctx context.Context) error,
	func(ctx context.Context) error,
	func(ctx context.Context) error,
) {
	dbCheck := func(ctx context.Context) error {
		if pool == nil {
			return fmt.Errorf("db not configured")
		}
		return pool.Ping(ctx)
	}
	qdrantCheck := func(ctx context.Context) error {
		client := &http.Client{Timeout: 2 * time.Second}
		req, _ := http.NewRequestWithContext(ctx, http.MethodGet, cfg.QdrantURL+"/collections", nil)
		if cfg.QdrantAPIKey != "" {
			req.Header.Set("api-key", cfg.QdrantAPIKey)
		}
		resp, err := client.Do(req)
		if err != nil {
			return err
		}
		defer func() { _ = resp.Body.Close() }()
		if resp.StatusCode >= 200 && resp.StatusCode < 300 {
			return nil
		}
		if span := ctx.Value("otel.span"); span != nil {
			// Best-effort: attach status code as attribute if span is present in context
			if s, ok := span.(interface{ SetAttributes(...attribute.KeyValue) }); ok {
				s.SetAttributes(attribute.Int("readiness.qdrant.status_code", resp.StatusCode))
			}
		}
		return fmt.Errorf("qdrant status %d", resp.StatusCode)
	}
	tikaCheck := func(ctx context.Context) error {
		if cfg.TikaURL == "" {
			return fmt.Errorf("tika url not configured")
		}
		client := &http.Client{Timeout: 2 * time.Second}
		req, _ := http.NewRequestWithContext(ctx, http.MethodGet, cfg.TikaURL+"/version", nil)
		resp, err := client.Do(req)
		if err != nil {
			return err
		}
		defer func() { _ = resp.Body.Close() }()
		if resp.StatusCode >= 200 && resp.StatusCode < 300 {
			return nil
		}
		if span := ctx.Value("otel.span"); span != nil {
			if s, ok := span.(interface{ SetAttributes(...attribute.KeyValue) }); ok {
				s.SetAttributes(attribute.Int("readiness.tika.status_code", resp.StatusCode))
			}
		}
		return fmt.Errorf("tika status %d", resp.StatusCode)
	}
	return dbCheck, qdrantCheck, tikaCheck
}
