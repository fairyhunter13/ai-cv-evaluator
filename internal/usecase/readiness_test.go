package usecase

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/fairyhunter13/ai-cv-evaluator/internal/domain"
	"github.com/fairyhunter13/ai-cv-evaluator/internal/domain/mocks"
)

// mockVectorDBHealthChecker is a simple mock for VectorDBHealthChecker
type mockVectorDBHealthChecker struct {
	pingErr error
}

func (m *mockVectorDBHealthChecker) Ping(_ domain.Context) error {
	return m.pingErr
}

func TestEvaluateService_Readiness(t *testing.T) {
	t.Run("nil dependencies", func(t *testing.T) {
		// Test with nil dependencies to verify error handling
		svc := NewEvaluateServiceWithHealthChecks(nil, nil, nil, nil, nil)
		checks := svc.Readiness(context.TODO())
		require.Len(t, checks, 4)
		// With nil dependencies, all checks should fail
		for _, c := range checks {
			assert.False(t, c.OK, "expected failure in %s with nil dependencies", c.Name)
		}
	})

	t.Run("successful health checks", func(t *testing.T) {
		// Create mocks that return success
		mockJobs := mocks.NewMockJobRepository(t)
		mockJobs.On("Count", mock.Anything).Return(int64(5), nil).Once()

		mockAI := mocks.NewMockAIClient(t)
		mockAI.On("Embed", mock.Anything, mock.Anything).Return([][]float32{{1.0, 2.0}}, nil).Once()

		mockVector := &mockVectorDBHealthChecker{pingErr: nil}

		mockUploads := mocks.NewMockUploadRepository(t)
		mockUploads.On("Count", mock.Anything).Return(int64(3), nil).Once()

		svc := NewEvaluateServiceWithHealthChecks(mockJobs, nil, mockUploads, mockAI, mockVector)
		checks := svc.Readiness(context.TODO())
		require.Len(t, checks, 4)

		// All checks should pass
		for _, c := range checks {
			assert.True(t, c.OK, "expected success in %s", c.Name)
		}
	})

	t.Run("database error", func(t *testing.T) {
		mockJobs := mocks.NewMockJobRepository(t)
		mockJobs.On("Count", mock.Anything).Return(int64(0), errors.New("database error")).Once()

		svc := NewEvaluateServiceWithHealthChecks(mockJobs, nil, nil, nil, nil)
		checks := svc.Readiness(context.TODO())
		require.Len(t, checks, 4)

		// Database check should fail
		dbCheck := checks[0]
		assert.Equal(t, "database", dbCheck.Name)
		assert.False(t, dbCheck.OK)
		assert.Contains(t, dbCheck.Details, "database error")
	})

	t.Run("AI service error", func(t *testing.T) {
		mockAI := mocks.NewMockAIClient(t)
		mockAI.On("Embed", mock.Anything, mock.Anything).Return(nil, errors.New("AI service error")).Once()

		svc := NewEvaluateServiceWithHealthChecks(nil, nil, nil, mockAI, nil)
		checks := svc.Readiness(context.TODO())
		require.Len(t, checks, 4)

		// AI service check should fail
		aiCheck := checks[1]
		assert.Equal(t, "ai_service", aiCheck.Name)
		assert.False(t, aiCheck.OK)
		assert.Contains(t, aiCheck.Details, "AI service error")
	})

	t.Run("vector database error", func(t *testing.T) {
		mockVector := &mockVectorDBHealthChecker{pingErr: errors.New("vector database error")}

		svc := NewEvaluateServiceWithHealthChecks(nil, nil, nil, nil, mockVector)
		checks := svc.Readiness(context.TODO())
		require.Len(t, checks, 4)

		// Vector database check should fail
		vectorCheck := checks[2]
		assert.Equal(t, "vector_database", vectorCheck.Name)
		assert.False(t, vectorCheck.OK)
		assert.Contains(t, vectorCheck.Details, "vector database error")
	})

	t.Run("upload repository error", func(t *testing.T) {
		mockUploads := mocks.NewMockUploadRepository(t)
		mockUploads.On("Count", mock.Anything).Return(int64(0), errors.New("upload repository error")).Once()

		svc := NewEvaluateServiceWithHealthChecks(nil, nil, mockUploads, nil, nil)
		checks := svc.Readiness(context.TODO())
		require.Len(t, checks, 4)

		// Upload repository check should fail
		uploadCheck := checks[3]
		assert.Equal(t, "upload_repository", uploadCheck.Name)
		assert.False(t, uploadCheck.OK)
		assert.Contains(t, uploadCheck.Details, "upload repository error")
	})
}
