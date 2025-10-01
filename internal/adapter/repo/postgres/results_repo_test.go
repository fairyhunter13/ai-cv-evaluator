package postgres_test

import (
	"context"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgconn"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/fairyhunter13/ai-cv-evaluator/internal/adapter/repo/postgres"
	"github.com/fairyhunter13/ai-cv-evaluator/internal/adapter/repo/postgres/mocks"
	"github.com/fairyhunter13/ai-cv-evaluator/internal/domain"
)

func TestResultRepo_Upsert_Success(t *testing.T) {
	pool := postgres.NewMockPgxPool(t)
	repo := postgres.NewResultRepo(pool)
	ctx := context.Background()
	res := domain.Result{JobID: "j1", CVMatchRate: 0.8, CVFeedback: "a.", ProjectScore: 9, ProjectFeedback: "b.", OverallSummary: "c."}

	// Upsert ok
	pool.EXPECT().Exec(mock.Anything, mock.Anything, mock.Anything).Return(pgconn.CommandTag{}, nil).Once()
	require.NoError(t, repo.Upsert(ctx, res))
}

func TestResultRepo_Get_Success(t *testing.T) {
	pool := postgres.NewMockPgxPool(t)
	repo := postgres.NewResultRepo(pool)
	ctx := context.Background()
	res := domain.Result{JobID: "j1", CVMatchRate: 0.8, CVFeedback: "a.", ProjectScore: 9, ProjectFeedback: "b.", OverallSummary: "c."}

	// Get ok
	fixed := time.Now().UTC()
	mockRow := mocks.NewMockRow(t)
	mockRow.On("Scan", mock.Anything).Run(func(args mock.Arguments) {
		dest := args[0].([]any)
		*(dest[0].(*string)) = res.JobID
		*(dest[1].(*float64)) = res.CVMatchRate
		*(dest[2].(*string)) = res.CVFeedback
		*(dest[3].(*float64)) = res.ProjectScore
		*(dest[4].(*string)) = res.ProjectFeedback
		*(dest[5].(*string)) = res.OverallSummary
		*(dest[6].(*time.Time)) = fixed
	}).Return(nil).Once()

	pool.EXPECT().QueryRow(mock.Anything, mock.Anything, mock.Anything).Return(mockRow).Once()
	got, err := repo.GetByJobID(ctx, res.JobID)
	require.NoError(t, err)
	assert.Equal(t, res.JobID, got.JobID)
}

func TestResultRepo_Get_Error(t *testing.T) {
	pool := postgres.NewMockPgxPool(t)
	repo := postgres.NewResultRepo(pool)
	ctx := context.Background()

	// Get DB error
	mockRowErr := mocks.NewMockRow(t)
	mockRowErr.On("Scan", mock.Anything).Return(assert.AnError).Once()
	pool.EXPECT().QueryRow(mock.Anything, mock.Anything, mock.Anything).Return(mockRowErr).Once()
	_, err := repo.GetByJobID(ctx, "bad")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "op=result.get")
}

func TestResultRepo_Upsert_DBError(t *testing.T) {
	pool := postgres.NewMockPgxPool(t)
	repo := postgres.NewResultRepo(pool)
	ctx := context.Background()
	res := domain.Result{JobID: "j1"}
	pool.EXPECT().Exec(mock.Anything, mock.Anything, mock.Anything).Return(pgconn.CommandTag{}, assert.AnError).Once()
	err := repo.Upsert(ctx, res)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "op=result.upsert")
}
