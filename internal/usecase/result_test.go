package usecase_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/fairyhunter13/ai-cv-evaluator/internal/domain"
	domainmocks "github.com/fairyhunter13/ai-cv-evaluator/internal/domain/mocks"
	"github.com/fairyhunter13/ai-cv-evaluator/internal/usecase"
)

func TestResult_FailedShape_ReturnsErrorObject(t *testing.T) {
	jobRepo := domainmocks.NewJobRepository(t)
	resultRepo := domainmocks.NewResultRepository(t)

	// Set up mock expectations
	jobRepo.On("Get", mock.Anything, "job1").Return(domain.Job{ID: "job1", Status: domain.JobFailed, Error: "schema invalid: missing field"}, nil)

	svc := usecase.NewResultService(jobRepo, resultRepo)
	st, body, etag, err := svc.Fetch(context.Background(), "job1", "")
	require.NoError(t, err)
	assert.Equal(t, 200, st)
	require.NotEmpty(t, etag)
	// Expect id, status, error.code and error.message
	assert.Equal(t, "job1", body["id"])       //nolint:forcetypeassert
	assert.Equal(t, "failed", body["status"]) //nolint:forcetypeassert
	errObj, ok := body["error"].(map[string]any)
	require.True(t, ok)
	code, _ := errObj["code"].(string)
	msg, _ := errObj["message"].(string)
	assert.Equal(t, "SCHEMA_INVALID", code)
	require.Contains(t, msg, "schema invalid")
}
