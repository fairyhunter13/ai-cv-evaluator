package usecase_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/fairyhunter13/ai-cv-evaluator/internal/domain"
	"github.com/fairyhunter13/ai-cv-evaluator/internal/domain/mocks"
	"github.com/fairyhunter13/ai-cv-evaluator/internal/usecase"
)

func TestEvaluate_Idempotency_ReturnsExistingJob(t *testing.T) {
	jobRepo := mocks.NewMockJobRepository(t)
	queue := mocks.NewMockQueue(t)
	uploadRepo := mocks.NewMockUploadRepository(t)

	// Set up mock expectations - job should be found by idempotency key
	jobRepo.On("FindByIdempotencyKey", mock.Anything, "idem-1").Return(domain.Job{ID: "existing"}, nil)

	svc := usecase.NewEvaluateService(jobRepo, queue, uploadRepo)
	jobID, err := svc.Enqueue(context.Background(), "cv1", "pr1", "", "", "", "idem-1")
	require.NoError(t, err)
	assert.Equal(t, "existing", jobID)

	// Verify that EnqueueEvaluate was not called
	queue.AssertNotCalled(t, "EnqueueEvaluate")
}
