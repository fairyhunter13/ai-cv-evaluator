package usecase_test

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/fairyhunter13/ai-cv-evaluator/internal/domain"
	domainmocks "github.com/fairyhunter13/ai-cv-evaluator/internal/domain/mocks"
	"github.com/fairyhunter13/ai-cv-evaluator/internal/usecase"
)

func TestResult_InProgress_NotModified(t *testing.T) {
	jobRepo := domainmocks.NewJobRepository(t)
	resultRepo := domainmocks.NewResultRepository(t)

	// Set up mock expectations
	jobRepo.On("Get", mock.Anything, "j1").Return(domain.Job{ID: "j1", Status: domain.JobProcessing, CreatedAt: time.Now()}, nil)
	// The service may mark stale jobs as failed
	jobRepo.On("UpdateStatus", mock.Anything, "j1", domain.JobFailed, mock.AnythingOfType("*string")).Return(nil).Maybe()

	svc := usecase.NewResultService(jobRepo, resultRepo)
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
	jobRepo := domainmocks.NewJobRepository(t)
	resultRepo := domainmocks.NewResultRepository(t)

	// Set up mock expectations
	jobRepo.On("Get", mock.Anything, "j2").Return(domain.Job{ID: "j2", Status: domain.JobCompleted}, nil)
	resultRepo.On("GetByJobID", mock.Anything, "j2").Return(domain.Result{JobID: "j2", CVMatchRate: 0.9, CVFeedback: "good", ProjectScore: 8, ProjectFeedback: "ok", OverallSummary: "sum", CreatedAt: time.Now()}, nil)

	svc := usecase.NewResultService(jobRepo, resultRepo)
	st, _, etag, err := svc.Fetch(context.Background(), "j2", "")
	require.NoError(t, err)
	assert.Equal(t, 200, st)

	st2, _, _, err := svc.Fetch(context.Background(), "j2", etag)
	require.NoError(t, err)
	assert.Equal(t, 304, st2)
}

func TestResult_ErrorCode_Mapping_Timeout(t *testing.T) {
	jobRepo := domainmocks.NewJobRepository(t)
	resultRepo := domainmocks.NewResultRepository(t)

	// Set up mock expectations
	jobRepo.On("Get", mock.Anything, "j3").Return(domain.Job{ID: "j3", Status: domain.JobFailed, Error: "upstream timeout while calling ai"}, nil)

	svc := usecase.NewResultService(jobRepo, resultRepo)
	st, body, _, err := svc.Fetch(context.Background(), "j3", "")
	require.NoError(t, err)
	assert.Equal(t, 200, st)
	errObj := body["error"].(map[string]any)
	assert.Equal(t, "UPSTREAM_TIMEOUT", errObj["code"]) //nolint:forcetypeassert
}
