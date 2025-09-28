package postgres_test

import (
    "context"
    "testing"
    "time"

    "github.com/jackc/pgx/v5"
    "github.com/stretchr/testify/assert"
    "github.com/stretchr/testify/require"

    "github.com/fairyhunter13/ai-cv-evaluator/internal/adapter/repo/postgres"
    "github.com/fairyhunter13/ai-cv-evaluator/internal/domain"
)

func TestJobRepo_Create_UpdateStatus_Get_FindIdem(t *testing.T) {
    t.Parallel()
    m := &poolStub{}
    repo := postgres.NewJobRepo(m)
    ctx := context.Background()

    // Create
    m.execErr = nil
    id, err := repo.Create(ctx, domain.Job{Status: domain.JobQueued, CVID: "cv1", ProjectID: "pr1"})
    require.NoError(t, err)
    assert.NotEmpty(t, id)

    // UpdateStatus
    m.execErr = nil
    require.NoError(t, repo.UpdateStatus(ctx, id, domain.JobProcessing, nil))

    // Get ok
    fixed := time.Now().UTC()
    m.row = rowStub{scan: func(dest ...any) error {
        *(dest[0].(*string)) = id
        *(dest[1].(*domain.JobStatus)) = domain.JobProcessing
        *(dest[2].(*string)) = ""
        *(dest[3].(*time.Time)) = fixed
        *(dest[4].(*time.Time)) = fixed
        *(dest[5].(*string)) = "cv1"
        *(dest[6].(*string)) = "pr1"
        // last is *string pointer (idempotency)
        *(dest[7].(**string)) = nil
        return nil
    }}
    j, err := repo.Get(ctx, id)
    require.NoError(t, err)
    assert.Equal(t, id, j.ID)

    // Get not found
    m.row = rowStub{scan: func(_ ...any) error { return pgx.ErrNoRows }}
    _, err = repo.Get(ctx, "missing")
    require.Error(t, err)
    assert.Contains(t, err.Error(), "op=job.get")

    // FindByIdempotencyKey ok
    m.row = rowStub{scan: func(dest ...any) error {
        *(dest[0].(*string)) = id
        *(dest[1].(*domain.JobStatus)) = domain.JobQueued
        *(dest[2].(*string)) = ""
        *(dest[3].(*time.Time)) = fixed
        *(dest[4].(*time.Time)) = fixed
        *(dest[5].(*string)) = "cv1"
        *(dest[6].(*string)) = "pr1"
        *(dest[7].(**string)) = nil
        return nil
    }}
    j2, err := repo.FindByIdempotencyKey(ctx, "idem1")
    require.NoError(t, err)
    assert.Equal(t, id, j2.ID)

    // FindByIdempotencyKey not found
    m.row = rowStub{scan: func(_ ...any) error { return pgx.ErrNoRows }}
    _, err = repo.FindByIdempotencyKey(ctx, "idem-miss")
    require.Error(t, err)
    assert.Contains(t, err.Error(), "op=job.find_idem")

    // UpdateStatus DB error
    m.execErr = assert.AnError
    require.Error(t, repo.UpdateStatus(ctx, id, domain.JobFailed, nil))
}

func TestJobRepo_Create_And_GetFind_OtherErrors(t *testing.T) {
    t.Parallel()
    m := &poolStub{}
    repo := postgres.NewJobRepo(m)
    ctx := context.Background()

    // Create DB error
    m.execErr = assert.AnError
    _, err := repo.Create(ctx, domain.Job{Status: domain.JobQueued, CVID: "cv", ProjectID: "pr"})
    require.Error(t, err)
    assert.Contains(t, err.Error(), "op=job.create")

    // Get other error (not pgx.ErrNoRows)
    m.row = rowStub{scan: func(_ ...any) error { return assert.AnError }}
    _, err = repo.Get(ctx, "id-1")
    require.Error(t, err)
    assert.Contains(t, err.Error(), "op=job.get")

    // FindByIdempotencyKey other error
    m.row = rowStub{scan: func(_ ...any) error { return assert.AnError }}
    _, err = repo.FindByIdempotencyKey(ctx, "k1")
    require.Error(t, err)
    assert.Contains(t, err.Error(), "op=job.find_idem")
}
