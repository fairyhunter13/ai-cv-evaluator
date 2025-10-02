package usecase_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/fairyhunter13/ai-cv-evaluator/internal/domain"
	"github.com/fairyhunter13/ai-cv-evaluator/internal/domain/mocks"
	"github.com/fairyhunter13/ai-cv-evaluator/internal/usecase"
)

// Test helpers for creating mocks with minimal expectations
func setupMocks() (*mocks.MockJobRepository, *mocks.MockQueue, *mocks.MockUploadRepository) {
	jobRepo := &mocks.MockJobRepository{}
	queue := &mocks.MockQueue{}
	uploadRepo := &mocks.MockUploadRepository{}

	// Only set up expectations for methods that are actually called
	// The usecase only calls: FindByIdempotencyKey, Create, UpdateStatus on JobRepository
	// and EnqueueEvaluate on Queue. UploadRepository is not used in the Enqueue method.

	return jobRepo, queue, uploadRepo
}

func TestEvaluate_Enqueue_Success(t *testing.T) {
	t.Parallel()
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	jobRepo, queue, uploadRepo := setupMocks()

	// Set up specific expectations for this test
	jobRepo.On("Create", mock.Anything, mock.MatchedBy(func(j domain.Job) bool {
		return j.Status == domain.JobQueued
	})).Return("job-abc", nil)

	queue.On("EnqueueEvaluate", mock.Anything, mock.MatchedBy(func(p domain.EvaluateTaskPayload) bool {
		return p.CVID == "cv-1" && p.ProjectID == "pr-1"
	})).Return("t-1", nil)

	svc := usecase.NewEvaluateService(jobRepo, queue, uploadRepo)
	jobID, err := svc.Enqueue(ctx, "cv-1", "pr-1", "jd", "sc", "sr", "")
	require.NoError(t, err)
	assert.Equal(t, "job-abc", jobID)

	// Verify all expectations were met
	jobRepo.AssertExpectations(t)
	queue.AssertExpectations(t)
	uploadRepo.AssertExpectations(t)
}

func TestEvaluate_Enqueue_InvalidArgs(t *testing.T) {
	t.Parallel()
	jobRepo, queue, uploadRepo := setupMocks()
	svc := usecase.NewEvaluateService(jobRepo, queue, uploadRepo)
	_, err := svc.Enqueue(context.Background(), "", "pr-1", "jd", "sc", "sr", "")
	require.Error(t, err)
}

func TestEvaluate_Enqueue_QueueFail_UpdatesJobFailed(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	jobRepo, queue, uploadRepo := setupMocks()

	// Set up expectations for job creation and queue failure
	jobRepo.On("Create", mock.Anything, mock.MatchedBy(func(j domain.Job) bool {
		return j.Status == domain.JobQueued
	})).Return("job-abc", nil)

	queue.On("EnqueueEvaluate", mock.Anything, mock.Anything).Return("", errors.New("queue down"))

	// Expect job status to be updated to failed
	jobRepo.On("UpdateStatus", mock.Anything, "job-abc", domain.JobFailed, mock.AnythingOfType("*string")).Return(nil)

	svc := usecase.NewEvaluateService(jobRepo, queue, uploadRepo)
	_, err := svc.Enqueue(ctx, "cv-1", "pr-1", "jd", "sc", "sr", "")
	require.Error(t, err)

	// Verify all expectations were met
	jobRepo.AssertExpectations(t)
	queue.AssertExpectations(t)
	uploadRepo.AssertExpectations(t)
}
