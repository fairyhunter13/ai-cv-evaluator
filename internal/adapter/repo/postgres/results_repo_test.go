package postgres_test

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/fairyhunter13/ai-cv-evaluator/internal/adapter/repo/postgres"
	"github.com/fairyhunter13/ai-cv-evaluator/internal/domain"
)

func TestResultRepo_Upsert_Get(t *testing.T) {
	m := &poolStub{}
	repo := postgres.NewResultRepo(m)
	ctx := context.Background()
	res := domain.Result{JobID: "j1", CVMatchRate: 0.8, CVFeedback: "a.", ProjectScore: 9, ProjectFeedback: "b.", OverallSummary: "c."}

	// Upsert ok
	m.execErr = nil
	require.NoError(t, repo.Upsert(ctx, res))

	// Get ok
	fixed := time.Now().UTC()
	m.row = rowStub{scan: func(dest ...any) error {
		*(dest[0].(*string)) = res.JobID
		*(dest[1].(*float64)) = res.CVMatchRate
		*(dest[2].(*string)) = res.CVFeedback
		*(dest[3].(*float64)) = res.ProjectScore
		*(dest[4].(*string)) = res.ProjectFeedback
		*(dest[5].(*string)) = res.OverallSummary
		*(dest[6].(*time.Time)) = fixed
		return nil
	}}
	got, err := repo.GetByJobID(ctx, res.JobID)
	require.NoError(t, err)
	assert.Equal(t, res.JobID, got.JobID)

	// Get DB error
	m.row = rowStub{scan: func(_ ...any) error { return assert.AnError }}
	_, err = repo.GetByJobID(ctx, "bad")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "op=result.get")
}

func TestResultRepo_Upsert_DBError(t *testing.T) {
    m := &poolStub{}
    repo := postgres.NewResultRepo(m)
    ctx := context.Background()
    res := domain.Result{JobID: "j1"}
    m.execErr = assert.AnError
    err := repo.Upsert(ctx, res)
    require.Error(t, err)
    assert.Contains(t, err.Error(), "op=result.upsert")
}
