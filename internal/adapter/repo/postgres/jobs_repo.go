// Package postgres provides PostgreSQL database adapters.
//
// It implements repository interfaces for data persistence.
// The package provides type-safe database operations with
// connection pooling and transaction support.
package postgres

import (
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"go.opentelemetry.io/otel"

	"github.com/fairyhunter13/ai-cv-evaluator/internal/domain"
)

// JobRepo persists and loads jobs from PostgreSQL using a minimal pgx pool.
type JobRepo struct{ Pool PgxPool }

// NewJobRepo constructs a JobRepo with the given pool.
func NewJobRepo(p PgxPool) *JobRepo { return &JobRepo{Pool: p} }

// Create inserts a new job and returns its id.
func (r *JobRepo) Create(ctx domain.Context, j domain.Job) (string, error) {
	tracer := otel.Tracer("repo.jobs")
	ctx, span := tracer.Start(ctx, "jobs.Create")
	defer span.End()
	id := j.ID
	if id == "" {
		id = uuid.New().String()
	}
	q := `INSERT INTO jobs (id, status, error, created_at, updated_at, cv_id, project_id, idempotency_key) VALUES ($1,$2,$3,$4,$5,$6,$7,$8)`
	_, err := r.Pool.Exec(ctx, q, id, j.Status, j.Error, time.Now().UTC(), time.Now().UTC(), j.CVID, j.ProjectID, j.IdemKey)
	if err != nil {
		return "", fmt.Errorf("op=job.create: %w", err)
	}
	return id, nil
}

// UpdateStatus updates a job's status and optional error message.
func (r *JobRepo) UpdateStatus(ctx domain.Context, id string, status domain.JobStatus, errMsg *string) error {
	tracer := otel.Tracer("repo.jobs")
	ctx, span := tracer.Start(ctx, "jobs.UpdateStatus")
	defer span.End()
	// Map nil errMsg to empty string to satisfy NOT NULL constraint on error column
	// CRITICAL: Pass empty string directly, not via COALESCE, to avoid NULL issues
	errVal := ""
	if errMsg != nil {
		errVal = *errMsg
	}
	q := `UPDATE jobs SET status=$2, error=$3, updated_at=$4 WHERE id=$1`
	_, err := r.Pool.Exec(ctx, q, id, status, errVal, time.Now().UTC())
	if err != nil {
		return fmt.Errorf("op=job.update_status: %w", err)
	}
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
		if err == pgx.ErrNoRows {
			return domain.Job{}, fmt.Errorf("op=job.find_idem: %w", domain.ErrNotFound)
		}
		return domain.Job{}, fmt.Errorf("op=job.find_idem: %w", err)
	}
	j.IdemKey = idem
	return j, nil
}

// Count returns the total number of jobs.
func (r *JobRepo) Count(ctx domain.Context) (int64, error) {
	tracer := otel.Tracer("repo.jobs")
	ctx, span := tracer.Start(ctx, "jobs.Count")
	defer span.End()
	q := `SELECT COUNT(*) FROM jobs`
	row := r.Pool.QueryRow(ctx, q)
	var count int64
	if err := row.Scan(&count); err != nil {
		return 0, fmt.Errorf("op=job.count: %w", err)
	}
	return count, nil
}

// CountByStatus returns the number of jobs by status.
func (r *JobRepo) CountByStatus(ctx domain.Context, status domain.JobStatus) (int64, error) {
	tracer := otel.Tracer("repo.jobs")
	ctx, span := tracer.Start(ctx, "jobs.CountByStatus")
	defer span.End()
	q := `SELECT COUNT(*) FROM jobs WHERE status = $1`
	row := r.Pool.QueryRow(ctx, q, status)
	var count int64
	if err := row.Scan(&count); err != nil {
		return 0, fmt.Errorf("op=job.count_by_status: %w", err)
	}
	return count, nil
}

// List returns a paginated list of jobs.
func (r *JobRepo) List(ctx domain.Context, offset, limit int) ([]domain.Job, error) {
	tracer := otel.Tracer("repo.jobs")
	ctx, span := tracer.Start(ctx, "jobs.List")
	defer span.End()
	q := `SELECT id, status, COALESCE(error,''), created_at, updated_at, cv_id, project_id, idempotency_key FROM jobs ORDER BY created_at DESC LIMIT $1 OFFSET $2`
	rows, err := r.Pool.Query(ctx, q, limit, offset)
	if err != nil {
		return nil, fmt.Errorf("op=job.list: %w", err)
	}
	defer rows.Close()

	var jobs []domain.Job
	for rows.Next() {
		var j domain.Job
		var idem *string
		if err := rows.Scan(&j.ID, &j.Status, &j.Error, &j.CreatedAt, &j.UpdatedAt, &j.CVID, &j.ProjectID, &idem); err != nil {
			return nil, fmt.Errorf("op=job.list_scan: %w", err)
		}
		j.IdemKey = idem
		jobs = append(jobs, j)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("op=job.list_rows: %w", err)
	}
	return jobs, nil
}

// ListWithFilters returns a paginated list of jobs with search and status filtering.
func (r *JobRepo) ListWithFilters(ctx domain.Context, offset, limit int, search, status string) ([]domain.Job, error) {
	tracer := otel.Tracer("repo.jobs")
	ctx, span := tracer.Start(ctx, "jobs.ListWithFilters")
	defer span.End()

	// Build dynamic query based on filters
	baseQuery := `SELECT id, status, COALESCE(error,''), created_at, updated_at, cv_id, project_id, idempotency_key FROM jobs`
	whereClause := ""
	args := []interface{}{}
	argIndex := 1

	// Add status filter if provided
	if status != "" {
		whereClause += " WHERE status = $" + fmt.Sprintf("%d", argIndex)
		args = append(args, status)
		argIndex++
	}

	// Add search filter if provided
	if search != "" {
		if whereClause == "" {
			whereClause = " WHERE "
		} else {
			whereClause += " AND "
		}
		searchPattern := "%" + search + "%"
		whereClause += "(id ILIKE $" + fmt.Sprintf("%d", argIndex) + " OR cv_id ILIKE $" + fmt.Sprintf("%d", argIndex+1) + " OR project_id ILIKE $" + fmt.Sprintf("%d", argIndex+2) + ")"
		args = append(args, searchPattern, searchPattern, searchPattern)
		argIndex += 3
	}

	// Add ordering and pagination
	query := baseQuery + whereClause + " ORDER BY created_at DESC LIMIT $" + fmt.Sprintf("%d", argIndex) + " OFFSET $" + fmt.Sprintf("%d", argIndex+1)
	args = append(args, limit, offset)

	rows, err := r.Pool.Query(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("op=job.list_with_filters: %w", err)
	}
	defer rows.Close()

	var jobs []domain.Job
	for rows.Next() {
		var j domain.Job
		var idem *string
		if err := rows.Scan(&j.ID, &j.Status, &j.Error, &j.CreatedAt, &j.UpdatedAt, &j.CVID, &j.ProjectID, &idem); err != nil {
			return nil, fmt.Errorf("op=job.list_with_filters_scan: %w", err)
		}
		j.IdemKey = idem
		jobs = append(jobs, j)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("op=job.list_with_filters_rows: %w", err)
	}
	return jobs, nil
}

// CountWithFilters returns the total count of jobs with search and status filtering.
func (r *JobRepo) CountWithFilters(ctx domain.Context, search, status string) (int64, error) {
	tracer := otel.Tracer("repo.jobs")
	ctx, span := tracer.Start(ctx, "jobs.CountWithFilters")
	defer span.End()

	// Build dynamic query based on filters
	baseQuery := `SELECT COUNT(*) FROM jobs`
	whereClause := ""
	args := []interface{}{}
	argIndex := 1

	// Add status filter if provided
	if status != "" {
		whereClause += " WHERE status = $" + fmt.Sprintf("%d", argIndex)
		args = append(args, status)
		argIndex++
	}

	// Add search filter if provided
	if search != "" {
		if whereClause == "" {
			whereClause = " WHERE "
		} else {
			whereClause += " AND "
		}
		searchPattern := "%" + search + "%"
		whereClause += "(id ILIKE $" + fmt.Sprintf("%d", argIndex) + " OR cv_id ILIKE $" + fmt.Sprintf("%d", argIndex+1) + " OR project_id ILIKE $" + fmt.Sprintf("%d", argIndex+2) + ")"
		args = append(args, searchPattern, searchPattern, searchPattern)
	}

	query := baseQuery + whereClause
	row := r.Pool.QueryRow(ctx, query, args...)
	var count int64
	if err := row.Scan(&count); err != nil {
		return 0, fmt.Errorf("op=job.count_with_filters: %w", err)
	}
	return count, nil
}

// GetAverageProcessingTime returns the average processing time for completed jobs.
func (r *JobRepo) GetAverageProcessingTime(ctx domain.Context) (float64, error) {
	tracer := otel.Tracer("repo.jobs")
	ctx, span := tracer.Start(ctx, "jobs.GetAverageProcessingTime")
	defer span.End()
	q := `SELECT AVG(EXTRACT(EPOCH FROM (updated_at - created_at))) FROM jobs WHERE status = $1`
	row := r.Pool.QueryRow(ctx, q, domain.JobCompleted)
	var avgTime *float64
	if err := row.Scan(&avgTime); err != nil {
		return 0, fmt.Errorf("op=job.avg_processing_time: %w", err)
	}
	if avgTime == nil {
		return 0, nil // No completed jobs
	}
	return *avgTime, nil
}
