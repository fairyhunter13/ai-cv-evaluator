package postgres_test

import (
	"context"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	pgxmock "github.com/pashagolub/pgxmock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/fairyhunter13/ai-cv-evaluator/internal/adapter/repo/postgres"
	"github.com/fairyhunter13/ai-cv-evaluator/internal/domain"
)

func TestJobRepo_Create_UpdateStatus_Get_FindIdem(t *testing.T) {
	t.Parallel()
	m, err := pgxmock.NewPool()
	require.NoError(t, err)
	defer m.Close()
	repo := postgres.NewJobRepo(m)
	ctx := context.Background()

	// Create
	m.ExpectExec("INSERT INTO jobs").
		WithArgs(pgxmock.AnyArg(), domain.JobQueued, "", pgxmock.AnyArg(), pgxmock.AnyArg(), "cv1", "pr1", nil).
		WillReturnResult(pgxmock.NewResult("INSERT", 1))
	id, err := repo.Create(ctx, domain.Job{Status: domain.JobQueued, CVID: "cv1", ProjectID: "pr1"})
	require.NoError(t, err)
	assert.NotEmpty(t, id)

	// UpdateStatus
	m.ExpectExec("UPDATE jobs SET status").
		WithArgs(id, domain.JobProcessing, nil, pgxmock.AnyArg()).
		WillReturnResult(pgxmock.NewResult("UPDATE", 1))
	require.NoError(t, repo.UpdateStatus(ctx, id, domain.JobProcessing, nil))

	// Get ok
	fixed := time.Now().UTC()
	rows := pgxmock.NewRows([]string{"id", "status", "error", "created_at", "updated_at", "cv_id", "project_id", "idempotency_key"}).
		AddRow(id, string(domain.JobProcessing), "", fixed, fixed, "cv1", "pr1", nil)
	m.ExpectQuery(`SELECT id, status, COALESCE\(error,''\), created_at, updated_at, cv_id, project_id, idempotency_key FROM jobs WHERE id=\$1`).
		WithArgs(id).
		WillReturnRows(rows)
	j, err := repo.Get(ctx, id)
	require.NoError(t, err)
	assert.Equal(t, id, j.ID)

	// Get not found
	m.ExpectQuery(`SELECT id, status, COALESCE\(error,''\), created_at, updated_at, cv_id, project_id, idempotency_key FROM jobs WHERE id=\$1`).
		WithArgs("missing").
		WillReturnError(pgx.ErrNoRows)
	_, err = repo.Get(ctx, "missing")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "op=job.get")

	// FindByIdempotencyKey ok
	rows2 := pgxmock.NewRows([]string{"id", "status", "error", "created_at", "updated_at", "cv_id", "project_id", "idempotency_key"}).
		AddRow(id, string(domain.JobQueued), "", fixed, fixed, "cv1", "pr1", nil)
	m.ExpectQuery(`SELECT id, status, COALESCE\(error,''\), created_at, updated_at, cv_id, project_id, idempotency_key FROM jobs WHERE idempotency_key=\$1 LIMIT 1`).
		WithArgs("idem1").
		WillReturnRows(rows2)
	j2, err := repo.FindByIdempotencyKey(ctx, "idem1")
	require.NoError(t, err)
	assert.Equal(t, id, j2.ID)

	// FindByIdempotencyKey not found
	m.ExpectQuery(`SELECT id, status, COALESCE\(error,''\), created_at, updated_at, cv_id, project_id, idempotency_key FROM jobs WHERE idempotency_key=\$1 LIMIT 1`).
		WithArgs("idem-miss").
		WillReturnError(pgx.ErrNoRows)
	_, err = repo.FindByIdempotencyKey(ctx, "idem-miss")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "op=job.find_idem")

	// UpdateStatus DB error
	m.ExpectExec("UPDATE jobs SET status").
		WithArgs(id, domain.JobFailed, nil, pgxmock.AnyArg()).
		WillReturnError(assert.AnError)
	require.Error(t, repo.UpdateStatus(ctx, id, domain.JobFailed, nil))

	require.NoError(t, m.ExpectationsWereMet())
}
