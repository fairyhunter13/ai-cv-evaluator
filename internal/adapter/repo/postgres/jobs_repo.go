package postgres

import (
	"fmt"
	"time"

	"github.com/google/uuid"
	"go.opentelemetry.io/otel"
	"github.com/jackc/pgx/v5"

	"github.com/fairyhunter13/ai-cv-evaluator/internal/domain"
)

// JobRepo persists and loads jobs from PostgreSQL using a minimal pgx pool.
type JobRepo struct { Pool PgxPool }

// NewJobRepo constructs a JobRepo with the given pool.
func NewJobRepo(p PgxPool) *JobRepo { return &JobRepo{Pool: p} }

// Create inserts a new job and returns its id.
func (r *JobRepo) Create(ctx domain.Context, j domain.Job) (string, error) {
	tracer := otel.Tracer("repo.jobs")
	ctx, span := tracer.Start(ctx, "jobs.Create")
	defer span.End()
	id := j.ID
	if id == "" { id = uuid.New().String() }
	q := `INSERT INTO jobs (id, status, error, created_at, updated_at, cv_id, project_id, idempotency_key) VALUES ($1,$2,$3,$4,$5,$6,$7,$8)`
	_, err := r.Pool.Exec(ctx, q, id, j.Status, j.Error, time.Now().UTC(), time.Now().UTC(), j.CVID, j.ProjectID, j.IdemKey)
	if err != nil { return "", fmt.Errorf("op=job.create: %w", err) }
	return id, nil
}

// UpdateStatus updates a job's status and optional error message.
func (r *JobRepo) UpdateStatus(ctx domain.Context, id string, status domain.JobStatus, errMsg *string) error {
	tracer := otel.Tracer("repo.jobs")
	ctx, span := tracer.Start(ctx, "jobs.UpdateStatus")
	defer span.End()
	q := `UPDATE jobs SET status=$2, error=$3, updated_at=$4 WHERE id=$1`
	// Map nil errMsg to empty string to satisfy NOT NULL constraint on error column
	errVal := ""
	if errMsg != nil { errVal = *errMsg }
	_, err := r.Pool.Exec(ctx, q, id, status, errVal, time.Now().UTC())
	if err != nil { return fmt.Errorf("op=job.update_status: %w", err) }
	return nil
}

// Get loads a job by id.
func (r *JobRepo) Get(ctx domain.Context, id string) (domain.Job, error) {
	tracer := otel.Tracer("repo.jobs")
	ctx, span := tracer.Start(ctx, "jobs.Get")
	defer span.End()
	q := `SELECT id, status, COALESCE(error,''), created_at, updated_at, cv_id, project_id, idempotency_key FROM jobs WHERE id=$1`
	row := r.Pool.QueryRow(ctx, q, id)
	var j domain.Job
	var idem *string
	if err := row.Scan(&j.ID, &j.Status, &j.Error, &j.CreatedAt, &j.UpdatedAt, &j.CVID, &j.ProjectID, &idem); err != nil {
		if err == pgx.ErrNoRows {
			return domain.Job{}, fmt.Errorf("op=job.get: %w", domain.ErrNotFound)
		}
		return domain.Job{}, fmt.Errorf("op=job.get: %w", err)
	}
	j.IdemKey = idem
	return j, nil
}

// FindByIdempotencyKey loads a job by idempotency key.
func (r *JobRepo) FindByIdempotencyKey(ctx domain.Context, key string) (domain.Job, error) {
	tracer := otel.Tracer("repo.jobs")
	ctx, span := tracer.Start(ctx, "jobs.FindByIdempotencyKey")
	defer span.End()
	q := `SELECT id, status, COALESCE(error,''), created_at, updated_at, cv_id, project_id, idempotency_key FROM jobs WHERE idempotency_key=$1 LIMIT 1`
	row := r.Pool.QueryRow(ctx, q, key)
	var j domain.Job
	var idem *string
	if err := row.Scan(&j.ID, &j.Status, &j.Error, &j.CreatedAt, &j.UpdatedAt, &j.CVID, &j.ProjectID, &idem); err != nil {
		if err == pgx.ErrNoRows { return domain.Job{}, fmt.Errorf("op=job.find_idem: %w", domain.ErrNotFound) }
		return domain.Job{}, fmt.Errorf("op=job.find_idem: %w", err)
	}
	j.IdemKey = idem
	return j, nil
}
