// Package postgres provides PostgreSQL database adapters.
//
// It implements repository interfaces for data persistence.
// The package provides type-safe database operations with
// connection pooling and transaction support.
package postgres

import (
	"context"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

// NewPool creates a pgx connection pool from the provided DSN and returns it.
// The pool is configured with sane defaults for this application.
func NewPool(ctx context.Context, dsn string) (*pgxpool.Pool, error) {
	cfg, err := pgxpool.ParseConfig(dsn)
	if err != nil {
		return nil, err
	}
	cfg.MaxConns = 10
	cfg.MaxConnIdleTime = 5 * time.Minute
	pool, err := pgxpool.NewWithConfig(ctx, cfg)
	if err != nil {
		return nil, err
	}
	return pool, nil
}
