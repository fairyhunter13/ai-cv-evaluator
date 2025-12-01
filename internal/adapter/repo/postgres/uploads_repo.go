// Package postgres provides PostgreSQL database adapters.
//
// It implements repository interfaces for data persistence.
// The package provides type-safe database operations with
// connection pooling and transaction support.
package postgres

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"

	"github.com/fairyhunter13/ai-cv-evaluator/internal/domain"
)

//go:generate mockery --config=.mockery.yml
//go:generate mockery --config=.mockery-pgx.yml

// UploadRepo persists and loads uploads using a minimal pgx pool.
type UploadRepo struct{ Pool PgxPool }

// PgxPool is a minimal subset of pgxpool used by the repos for easy testing.
type PgxPool interface {
	Exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error)
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
	Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
	BeginTx(ctx context.Context, txOptions pgx.TxOptions) (pgx.Tx, error)
}

// NewUploadRepo constructs an UploadRepo with the given pool.
func NewUploadRepo(p PgxPool) *UploadRepo { return &UploadRepo{Pool: p} }

// Create stores a new upload and returns its id (generates one if empty).
func (r *UploadRepo) Create(ctx domain.Context, u domain.Upload) (string, error) {
	tracer := otel.Tracer("repo.uploads")
	ctx, span := tracer.Start(ctx, "uploads.Create")
	defer span.End()
	span.SetAttributes(
		attribute.String("db.system", "postgresql"),
		attribute.String("db.operation", "INSERT"),
		attribute.String("db.sql.table", "uploads"),
	)
	id := u.ID
	if id == "" {
		id = uuid.New().String()
	}
	q := `INSERT INTO uploads (id, type, text, filename, mime, size, created_at) VALUES ($1,$2,$3,$4,$5,$6,$7)`
	_, err := r.Pool.Exec(ctx, q, id, u.Type, u.Text, u.Filename, u.MIME, u.Size, time.Now().UTC())
	if err != nil {
		return "", fmt.Errorf("op=upload.create: %w", err)
	}
	return id, nil
}

// Get loads an upload by id or returns an error.
func (r *UploadRepo) Get(ctx domain.Context, id string) (domain.Upload, error) {
	tracer := otel.Tracer("repo.uploads")
	ctx, span := tracer.Start(ctx, "uploads.Get")
	defer span.End()
	span.SetAttributes(
		attribute.String("db.system", "postgresql"),
		attribute.String("db.operation", "SELECT"),
		attribute.String("db.sql.table", "uploads"),
	)
	q := `SELECT id, type, text, filename, mime, size, created_at FROM uploads WHERE id=$1`
	row := r.Pool.QueryRow(ctx, q, id)
	var u domain.Upload
	if err := row.Scan(&u.ID, &u.Type, &u.Text, &u.Filename, &u.MIME, &u.Size, &u.CreatedAt); err != nil {
		return domain.Upload{}, fmt.Errorf("op=upload.get: %w", err)
	}
	return u, nil
}

// Count returns the total number of uploads.
func (r *UploadRepo) Count(ctx domain.Context) (int64, error) {
	tracer := otel.Tracer("repo.uploads")
	ctx, span := tracer.Start(ctx, "uploads.Count")
	defer span.End()
	span.SetAttributes(
		attribute.String("db.system", "postgresql"),
		attribute.String("db.operation", "COUNT"),
		attribute.String("db.sql.table", "uploads"),
	)
	q := `SELECT COUNT(*) FROM uploads`
	row := r.Pool.QueryRow(ctx, q)
	var count int64
	if err := row.Scan(&count); err != nil {
		return 0, fmt.Errorf("op=upload.count: %w", err)
	}
	return count, nil
}

// CountByType returns the number of uploads by type.
func (r *UploadRepo) CountByType(ctx domain.Context, uploadType string) (int64, error) {
	tracer := otel.Tracer("repo.uploads")
	ctx, span := tracer.Start(ctx, "uploads.CountByType")
	defer span.End()
	span.SetAttributes(
		attribute.String("db.system", "postgresql"),
		attribute.String("db.operation", "COUNT"),
		attribute.String("db.sql.table", "uploads"),
	)
	q := `SELECT COUNT(*) FROM uploads WHERE type = $1`
	row := r.Pool.QueryRow(ctx, q, uploadType)
	var count int64
	if err := row.Scan(&count); err != nil {
		return 0, fmt.Errorf("op=upload.count_by_type: %w", err)
	}
	return count, nil
}
