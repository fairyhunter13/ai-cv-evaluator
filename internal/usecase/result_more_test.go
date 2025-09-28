package usecase_test

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/fairyhunter13/ai-cv-evaluator/internal/domain"
	"github.com/fairyhunter13/ai-cv-evaluator/internal/usecase"
)

type stubJobRepoR2 struct{ job domain.Job; getErr error }
func (s *stubJobRepoR2) Create(_ domain.Context, _ domain.Job) (string, error) { return "", nil }
func (s *stubJobRepoR2) UpdateStatus(_ domain.Context, _ string, _ domain.JobStatus, _ *string) error { return nil }
func (s *stubJobRepoR2) Get(_ domain.Context, _ string) (domain.Job, error) { return s.job, s.getErr }
func (s *stubJobRepoR2) FindByIdempotencyKey(_ domain.Context, _ string) (domain.Job, error) { return domain.Job{}, domain.ErrNotFound }

type stubResultRepoR2 struct{ res domain.Result }
func (s *stubResultRepoR2) Upsert(_ domain.Context, _ domain.Result) error { return nil }
func (s *stubResultRepoR2) GetByJobID(_ domain.Context, _ string) (domain.Result, error) { return s.res, nil }

func TestResult_InProgress_NotModified(t *testing.T) {
	svc := usecase.NewResultService(&stubJobRepoR2{job: domain.Job{ID: "j1", Status: domain.JobProcessing}}, &stubResultRepoR2{})
	st, body, etag, err := svc.Fetch(context.Background(), "j1", "")
	require.NoError(t, err)
	assert.Equal(t, 200, st)
	require.NotEmpty(t, etag)
	require.NotNil(t, body)

	st2, _, _, err := svc.Fetch(context.Background(), "j1", etag)
	require.NoError(t, err)
	assert.Equal(t, 304, st2)
}

func TestResult_Completed_NotModified(t *testing.T) {
	jr := &stubJobRepoR2{job: domain.Job{ID: "j2", Status: domain.JobCompleted}}
	rr := &stubResultRepoR2{ res: domain.Result{JobID: "j2", CVMatchRate: 0.9, CVFeedback: "good", ProjectScore: 8, ProjectFeedback: "ok", OverallSummary: "sum", CreatedAt: time.Now()} }
	svc := usecase.NewResultService(jr, rr)
	st, _, etag, err := svc.Fetch(context.Background(), "j2", "")
	require.NoError(t, err)
	assert.Equal(t, 200, st)

	st2, _, _, err := svc.Fetch(context.Background(), "j2", etag)
	require.NoError(t, err)
	assert.Equal(t, 304, st2)
}

func TestResult_ErrorCode_Mapping_Timeout(t *testing.T) {
	svc := usecase.NewResultService(&stubJobRepoR2{job: domain.Job{ID: "j3", Status: domain.JobFailed, Error: "upstream timeout while calling ai"}}, &stubResultRepoR2{})
	st, body, _, err := svc.Fetch(context.Background(), "j3", "")
	require.NoError(t, err)
	assert.Equal(t, 200, st)
	errObj := body["error"].(map[string]any)
	assert.Equal(t, "UPSTREAM_TIMEOUT", errObj["code"]) //nolint:forcetypeassert
}
