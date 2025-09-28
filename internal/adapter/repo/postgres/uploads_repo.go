package postgres

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"go.opentelemetry.io/otel"

	"github.com/fairyhunter13/ai-cv-evaluator/internal/domain"
)

type UploadRepo struct { Pool PgxPool }

type PgxPool interface { Exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error); QueryRow(ctx context.Context, sql string, args ...any) pgx.Row }

func NewUploadRepo(p PgxPool) *UploadRepo { return &UploadRepo{Pool: p} }

func (r *UploadRepo) Create(ctx domain.Context, u domain.Upload) (string, error) {
	tracer := otel.Tracer("repo.uploads")
	ctx, span := tracer.Start(ctx, "uploads.Create")
	defer span.End()
	id := u.ID
	if id == "" { id = uuid.New().String() }
	q := `INSERT INTO uploads (id, type, text, filename, mime, size, created_at) VALUES ($1,$2,$3,$4,$5,$6,$7)`
	_, err := r.Pool.Exec(ctx, q, id, u.Type, u.Text, u.Filename, u.MIME, u.Size, time.Now().UTC())
	if err != nil { return "", fmt.Errorf("op=upload.create: %w", err) }
	return id, nil
}

func (r *UploadRepo) Get(ctx domain.Context, id string) (domain.Upload, error) {
	tracer := otel.Tracer("repo.uploads")
	ctx, span := tracer.Start(ctx, "uploads.Get")
	defer span.End()
	q := `SELECT id, type, text, filename, mime, size, created_at FROM uploads WHERE id=$1`
	row := r.Pool.QueryRow(ctx, q, id)
	var u domain.Upload
	if err := row.Scan(&u.ID, &u.Type, &u.Text, &u.Filename, &u.MIME, &u.Size, &u.CreatedAt); err != nil {
		return domain.Upload{}, fmt.Errorf("op=upload.get: %w", err)
	}
	return u, nil
}
