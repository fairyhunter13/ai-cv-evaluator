package usecase_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/fairyhunter13/ai-cv-evaluator/internal/domain"
	"github.com/fairyhunter13/ai-cv-evaluator/internal/usecase"
)

type stubJobRepoR struct{ job domain.Job; getErr error }

func (s *stubJobRepoR) Create(ctx domain.Context, j domain.Job) (string, error) { return "", nil }
func (s *stubJobRepoR) UpdateStatus(ctx domain.Context, id string, status domain.JobStatus, errMsg *string) error { return nil }
func (s *stubJobRepoR) Get(ctx domain.Context, id string) (domain.Job, error) { return s.job, s.getErr }
func (s *stubJobRepoR) FindByIdempotencyKey(ctx domain.Context, key string) (domain.Job, error) { return domain.Job{}, domain.ErrNotFound }

type stubResultRepoR struct{}

func (s *stubResultRepoR) Upsert(ctx domain.Context, r domain.Result) error { return nil }
func (s *stubResultRepoR) GetByJobID(ctx domain.Context, jobID string) (domain.Result, error) { return domain.Result{}, nil }

func TestResult_FailedShape_ReturnsErrorObject(t *testing.T) {
	svc := usecase.NewResultService(&stubJobRepoR{job: domain.Job{ID: "job1", Status: domain.JobFailed, Error: "schema invalid: missing field"}}, &stubResultRepoR{})
	st, body, etag, err := svc.Fetch(context.Background(), "job1", "")
	require.NoError(t, err)
	assert.Equal(t, 200, st)
	require.NotEmpty(t, etag)
	// Expect id, status, error.code and error.message
	assert.Equal(t, "job1", body["id"]) //nolint:forcetypeassert
	assert.Equal(t, "failed", body["status"]) //nolint:forcetypeassert
	errObj, ok := body["error"].(map[string]any)
	require.True(t, ok)
	code, _ := errObj["code"].(string)
	msg, _ := errObj["message"].(string)
	assert.Equal(t, "SCHEMA_INVALID", code)
	require.Contains(t, msg, "schema invalid")
}
