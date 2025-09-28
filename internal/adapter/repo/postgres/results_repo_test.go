package postgres_test

import (
	"context"
	"testing"
	"time"

	pgxmock "github.com/pashagolub/pgxmock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/fairyhunter13/ai-cv-evaluator/internal/adapter/repo/postgres"
	"github.com/fairyhunter13/ai-cv-evaluator/internal/domain"
)

func TestResultRepo_Upsert_Get(t *testing.T) {
	m, err := pgxmock.NewPool()
	require.NoError(t, err)
	defer m.Close()
	repo := postgres.NewResultRepo(m)
	ctx := context.Background()
	res := domain.Result{JobID: "j1", CVMatchRate: 0.8, CVFeedback: "a.", ProjectScore: 9, ProjectFeedback: "b.", OverallSummary: "c."}

	// Upsert ok
	m.ExpectExec("INSERT INTO results").
		WithArgs(res.JobID, res.CVMatchRate, res.CVFeedback, res.ProjectScore, res.ProjectFeedback, res.OverallSummary, pgxmock.AnyArg()).
		WillReturnResult(pgxmock.NewResult("INSERT", 1))
	require.NoError(t, repo.Upsert(ctx, res))

	// Get ok
	fixed := time.Now().UTC()
	rows := pgxmock.NewRows([]string{"job_id","cv_match_rate","cv_feedback","project_score","project_feedback","overall_summary","created_at"}).
		AddRow(res.JobID, res.CVMatchRate, res.CVFeedback, res.ProjectScore, res.ProjectFeedback, res.OverallSummary, fixed)
	m.ExpectQuery(`SELECT job_id, cv_match_rate, cv_feedback, project_score, project_feedback, overall_summary, created_at FROM results WHERE job_id=\$1`).
		WithArgs(res.JobID).
		WillReturnRows(rows)
	got, err := repo.GetByJobID(ctx, res.JobID)
	require.NoError(t, err)
	assert.Equal(t, res.JobID, got.JobID)

	// Get DB error
	m.ExpectQuery(`SELECT job_id, cv_match_rate, cv_feedback, project_score, project_feedback, overall_summary, created_at FROM results WHERE job_id=\$1`).
		WithArgs("bad").
		WillReturnError(assert.AnError)
	_, err = repo.GetByJobID(ctx, "bad")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "op=result.get")

	require.NoError(t, m.ExpectationsWereMet())
}
