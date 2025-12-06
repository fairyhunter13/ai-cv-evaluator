// Package postgres provides PostgreSQL database adapters.
//
// It implements repository interfaces for data persistence.
// The package provides type-safe database operations with
// connection pooling and transaction support.
package postgres

import (
	"context"
	"log/slog"
	"time"

	"github.com/exaring/otelpgx"
	"github.com/jackc/pgx/v5/pgxpool"
)

// NewPool creates a pgx connection pool from the provided DSN and returns it.
// The pool is configured with sane defaults for this application and includes
// OpenTelemetry tracing for distributed tracing visibility in Jaeger.
func NewPool(ctx context.Context, dsn string) (*pgxpool.Pool, error) {
	cfg, err := pgxpool.ParseConfig(dsn)
	if err != nil {
		return nil, err
	}
	cfg.MaxConns = 10
	cfg.MaxConnIdleTime = 5 * time.Minute

	// Add OpenTelemetry tracing to PostgreSQL connections
	cfg.ConnConfig.Tracer = otelpgx.NewTracer(
		otelpgx.WithTrimSQLInSpanName(),
	)

	pool, err := pgxpool.NewWithConfig(ctx, cfg)
	if err != nil {
		return nil, err
	}

	// Record connection pool stats for metrics
	if err := otelpgx.RecordStats(pool); err != nil {
		slog.Warn("failed to record pgx stats", slog.Any("error", err))
	}

	return pool, nil
}
