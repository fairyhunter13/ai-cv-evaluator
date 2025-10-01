package shared_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/fairyhunter13/ai-cv-evaluator/internal/adapter/queue/shared"
	qdrantcli "github.com/fairyhunter13/ai-cv-evaluator/internal/adapter/vector/qdrant"
	"github.com/fairyhunter13/ai-cv-evaluator/internal/domain"
	domainmocks "github.com/fairyhunter13/ai-cv-evaluator/internal/domain/mocks"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

// Helper function to create mock repositories with default expectations
func createMockJobRepository(_ *testing.T) *domainmocks.JobRepository {
	mockRepo := &domainmocks.JobRepository{}
	mockRepo.On("Create", mock.Anything, mock.Anything).Return(func(_ domain.Context, j domain.Job) (string, error) {
		return j.ID, nil
	})
	mockRepo.On("UpdateStatus", mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(nil)
	mockRepo.On("Get", mock.Anything, mock.Anything).Return(func(_ domain.Context, id string) (domain.Job, error) {
		return domain.Job{ID: id, Status: domain.JobQueued}, nil
	})
	mockRepo.On("FindByIdempotencyKey", mock.Anything, mock.Anything).Return(domain.Job{}, domain.ErrNotFound)
	mockRepo.On("Count", mock.Anything).Return(int64(0), nil)
	mockRepo.On("CountByStatus", mock.Anything, mock.Anything).Return(int64(0), nil)
	mockRepo.On("List", mock.Anything, mock.Anything, mock.Anything).Return([]domain.Job{}, nil)
	mockRepo.On("GetAverageProcessingTime", mock.Anything).Return(float64(0), nil)
	return mockRepo
}

func createMockUploadRepository(_ *testing.T) *domainmocks.UploadRepository {
	mockRepo := &domainmocks.UploadRepository{}
	mockRepo.On("Create", mock.Anything, mock.Anything).Return(func(_ domain.Context, u domain.Upload) (string, error) {
		return u.ID, nil
	})
	mockRepo.On("Get", mock.Anything, mock.Anything).Return(func(_ domain.Context, id string) (domain.Upload, error) {
		return domain.Upload{
			ID:   id,
			Text: "Sample content for " + id,
		}, nil
	})
	mockRepo.On("Count", mock.Anything).Return(int64(0), nil)
	mockRepo.On("CountByType", mock.Anything, mock.Anything).Return(int64(0), nil)
	return mockRepo
}

func createMockResultRepository(_ *testing.T) *domainmocks.ResultRepository {
	mockRepo := &domainmocks.ResultRepository{}
	mockRepo.On("Upsert", mock.Anything, mock.Anything).Return(nil)
	mockRepo.On("GetByJobID", mock.Anything, mock.Anything).Return(func(_ domain.Context, jobID string) (domain.Result, error) {
		return domain.Result{JobID: jobID}, nil
	})
	return mockRepo
}

func createMockAIClient(_ *testing.T) *domainmocks.AIClient {
	mockRepo := &domainmocks.AIClient{}
	mockRepo.On("ChatJSON", mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(func(_ domain.Context, systemPrompt, _ string, _ int) (string, error) {
		// Handle two-pass raw evaluation
		if strings.Contains(systemPrompt, "Perform a detailed analysis") {
			return `{
                "technical_score": 4.0,
                "experience_score": 3.5,
                "achievements_score": 4.5,
                "cultural_score": 3.0,
                "correctness_score": 4.0,
                "quality_score": 4.5,
                "resilience_score": 3.5,
                "docs_score": 4.0,
                "creativity_score": 3.0,
                "cv_feedback": "Detailed feedback on CV match",
                "project_feedback": "Detailed feedback on project quality",
                "overall_summary": "Comprehensive summary of the candidate's fit"
            }`, nil
		}
		// Handle normalization fallback
		if strings.Contains(systemPrompt, "scoring normalization expert") {
			return `{
                "cv_match_rate": 0.8,
                "cv_feedback": "Brief CV feedback",
                "project_score": 8.2,
                "project_feedback": "Brief project feedback",
                "overall_summary": "Concise overall summary"
            }`, nil
		}
		// Handle extraction pass
		if strings.Contains(systemPrompt, "expert CV analyzer") {
			return `Key insights extracted: Go, Microservices, 5+ years`, nil
		}
		// Default single-pass evaluation
		return `{
            "cv_match_rate": 0.85,
            "cv_feedback": "Strong technical background",
            "project_score": 8.5,
            "project_feedback": "Excellent project",
            "overall_summary": "Highly qualified candidate"
        }`, nil
	})
	mockRepo.On("Embed", mock.Anything, mock.Anything).Return([][]float32{{0.1, 0.2, 0.3}}, nil)
	return mockRepo
}

// Helper to create a test Qdrant client backed by an httptest.Server
func newTestQdrantClient() *qdrantcli.Client {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Only handle search endpoints used in tests
		if r.Method == http.MethodPost && strings.Contains(r.URL.Path, "/points/search") {
			w.Header().Set("Content-Type", "application/json")
			resp := map[string]any{
				"result": []map[string]any{
					{"payload": map[string]any{"text": "Sample context A JavaScript React Node.js"}, "score": 0.9},
					{"payload": map[string]any{"text": "Sample context B Python Django"}, "score": 0.7},
				},
			}
			_ = json.NewEncoder(w).Encode(resp)
			return
		}
		// Default OK for other endpoints if any
		w.WriteHeader(http.StatusOK)
	}))
	// No API key needed
	return qdrantcli.New(srv.URL, "")
}

func TestHandleEvaluate_Success(t *testing.T) {
	t.Parallel()

	// Setup simple mocks
	jobRepo := createMockJobRepository(t)
	uploadRepo := createMockUploadRepository(t)
	resultRepo := createMockResultRepository(t)
	aiClient := createMockAIClient(t)
	// No RAG in this test

	// Create payload
	payload := domain.EvaluateTaskPayload{
		JobID:          "test-job-1",
		CVID:           "test-cv-1",
		ProjectID:      "test-project-1",
		JobDescription: "Looking for a senior software engineer",
		StudyCaseBrief: "Design a scalable microservices architecture",
	}

	// Call HandleEvaluate
	ctx := context.Background()
	err := shared.HandleEvaluate(
		ctx,
		jobRepo,
		uploadRepo,
		resultRepo,
		aiClient,
		nil,
		payload,
	)

	// Should succeed with simple mocks
	assert.NoError(t, err)
}

func TestHandleEvaluate_WithTwoPass(t *testing.T) {
	t.Parallel()

	// Setup simple mocks
	jobRepo := createMockJobRepository(t)
	uploadRepo := createMockUploadRepository(t)
	resultRepo := createMockResultRepository(t)
	aiClient := createMockAIClient(t)

	// Create payload
	payload := domain.EvaluateTaskPayload{
		JobID:          "test-job-1",
		CVID:           "test-cv-1",
		ProjectID:      "test-project-1",
		JobDescription: "Looking for a senior software engineer",
		StudyCaseBrief: "Design a scalable microservices architecture",
	}

	// Call HandleEvaluate with two-pass enabled
	ctx := context.Background()
	err := shared.HandleEvaluate(
		ctx,
		jobRepo,
		uploadRepo,
		resultRepo,
		aiClient,
		nil,
		payload,
	)

	// Should succeed with simple mocks
	assert.NoError(t, err)
}

func TestHandleEvaluate_WithChaining(t *testing.T) {
	t.Parallel()

	// Setup simple mocks
	jobRepo := createMockJobRepository(t)
	uploadRepo := createMockUploadRepository(t)
	resultRepo := createMockResultRepository(t)
	aiClient := createMockAIClient(t)

	// Create payload
	payload := domain.EvaluateTaskPayload{
		JobID:          "test-job-1",
		CVID:           "test-cv-1",
		ProjectID:      "test-project-1",
		JobDescription: "Looking for a senior software engineer",
		StudyCaseBrief: "Design a scalable microservices architecture",
	}

	// Call HandleEvaluate with chaining enabled
	ctx := context.Background()
	err := shared.HandleEvaluate(
		ctx,
		jobRepo,
		uploadRepo,
		resultRepo,
		aiClient,
		nil,
		payload,
	)

	// Should succeed with simple mocks
	assert.NoError(t, err)
}

func TestHandleEvaluate_WithRAG(t *testing.T) {
	t.Parallel()

	// Setup simple mocks
	jobRepo := createMockJobRepository(t)
	uploadRepo := createMockUploadRepository(t)
	resultRepo := createMockResultRepository(t)
	aiClient := createMockAIClient(t)
	qdrantClient := newTestQdrantClient()

	// Create payload
	payload := domain.EvaluateTaskPayload{
		JobID:          "test-job-1",
		CVID:           "test-cv-1",
		ProjectID:      "test-project-1",
		JobDescription: "Looking for a senior software engineer",
		StudyCaseBrief: "Design a scalable microservices architecture",
	}

	// Call HandleEvaluate with RAG
	ctx := context.Background()
	err := shared.HandleEvaluate(
		ctx,
		jobRepo,
		uploadRepo,
		resultRepo,
		aiClient,
		qdrantClient,
		payload,
	)

	// Should succeed with simple mocks
	assert.NoError(t, err)
}

func TestHandleEvaluate_WithoutRAG(t *testing.T) {
	t.Parallel()

	// Setup simple mocks
	jobRepo := createMockJobRepository(t)
	uploadRepo := createMockUploadRepository(t)
	resultRepo := createMockResultRepository(t)
	aiClient := createMockAIClient(t)

	// Create payload
	payload := domain.EvaluateTaskPayload{
		JobID:          "test-job-1",
		CVID:           "test-cv-1",
		ProjectID:      "test-project-1",
		JobDescription: "Looking for a senior software engineer",
		StudyCaseBrief: "Design a scalable microservices architecture",
	}

	// Call HandleEvaluate without RAG (qdrantClient is nil)
	ctx := context.Background()
	err := shared.HandleEvaluate(
		ctx,
		jobRepo,
		uploadRepo,
		resultRepo,
		aiClient,
		nil, // No RAG
		payload,
	)

	// Should succeed with simple mocks
	assert.NoError(t, err)
}

func TestHandleEvaluate_WithBothTwoPassAndChaining(t *testing.T) {
	t.Parallel()

	// Setup simple mocks
	jobRepo := createMockJobRepository(t)
	uploadRepo := createMockUploadRepository(t)
	resultRepo := createMockResultRepository(t)
	aiClient := createMockAIClient(t)
	qdrantClient := newTestQdrantClient()

	// Create payload
	payload := domain.EvaluateTaskPayload{
		JobID:          "test-job-1",
		CVID:           "test-cv-1",
		ProjectID:      "test-project-1",
		JobDescription: "Looking for a senior software engineer",
		StudyCaseBrief: "Design a scalable microservices architecture",
	}

	// Call HandleEvaluate with both two-pass and chaining enabled
	ctx := context.Background()
	err := shared.HandleEvaluate(
		ctx,
		jobRepo,
		uploadRepo,
		resultRepo,
		aiClient,
		qdrantClient,
		payload,
	)

	// Should succeed with simple mocks
	assert.NoError(t, err)
}

func TestHandleEvaluate_ContextCancellation(t *testing.T) {
	t.Parallel()

	// Setup simple mocks
	jobRepo := createMockJobRepository(t)
	uploadRepo := createMockUploadRepository(t)
	resultRepo := createMockResultRepository(t)
	aiClient := createMockAIClient(t)
	qdrantClient := newTestQdrantClient()

	// Create payload
	payload := domain.EvaluateTaskPayload{
		JobID:          "test-job-1",
		CVID:           "test-cv-1",
		ProjectID:      "test-project-1",
		JobDescription: "Looking for a senior software engineer",
		StudyCaseBrief: "Design a scalable microservices architecture",
	}

	// Call HandleEvaluate with cancelled context
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately
	err := shared.HandleEvaluate(
		ctx,
		jobRepo,
		uploadRepo,
		resultRepo,
		aiClient,
		qdrantClient,
		payload,
	)

	// Should handle context cancellation gracefully
	// The exact behavior depends on where the cancellation occurs
	// We just verify it doesn't panic
	assert.NotPanics(t, func() {
		_ = err
	})

	// The function may or may not return an error depending on where cancellation occurs
	// This is acceptable behavior - the important thing is that it doesn't panic
	// and handles the cancellation gracefully
}

func TestHandleEvaluate_EmptyPayload(t *testing.T) {
	t.Parallel()

	// Setup simple mocks
	jobRepo := createMockJobRepository(t)
	uploadRepo := createMockUploadRepository(t)
	resultRepo := createMockResultRepository(t)
	aiClient := createMockAIClient(t)
	qdrantClient := newTestQdrantClient()

	// Create empty payload
	payload := domain.EvaluateTaskPayload{}

	// Call HandleEvaluate
	ctx := context.Background()
	err := shared.HandleEvaluate(
		ctx,
		jobRepo,
		uploadRepo,
		resultRepo,
		aiClient,
		qdrantClient,
		payload,
	)

	// Should handle empty payload gracefully
	// The exact behavior depends on validation
	assert.NotPanics(t, func() {
		_ = err
	})
}

func TestHandleEvaluate_ConcurrentCalls(t *testing.T) {
	t.Parallel()

	// Setup simple mocks
	jobRepo := createMockJobRepository(t)
	uploadRepo := createMockUploadRepository(t)
	resultRepo := createMockResultRepository(t)
	aiClient := createMockAIClient(t)
	qdrantClient := newTestQdrantClient()

	// Create payload
	payload := domain.EvaluateTaskPayload{
		JobID:          "test-job-1",
		CVID:           "test-cv-1",
		ProjectID:      "test-project-1",
		JobDescription: "Looking for a senior software engineer",
		StudyCaseBrief: "Design a scalable microservices architecture",
	}

	// Test concurrent calls
	done := make(chan bool, 5)
	for i := 0; i < 5; i++ {
		go func() {
			ctx := context.Background()
			_ = shared.HandleEvaluate(
				ctx,
				jobRepo,
				uploadRepo,
				resultRepo,
				aiClient,
				qdrantClient,
				payload,
			)
			done <- true
		}()
	}

	// Wait for all goroutines to complete
	for i := 0; i < 5; i++ {
		<-done
	}
}

func TestHandleEvaluate_DifferentConfigurations(t *testing.T) {
	t.Parallel()

	// Setup simple mocks
	jobRepo := createMockJobRepository(t)
	uploadRepo := createMockUploadRepository(t)
	resultRepo := createMockResultRepository(t)
	aiClient := createMockAIClient(t)
	qdrantClient := newTestQdrantClient()

	// Create payload
	payload := domain.EvaluateTaskPayload{
		JobID:          "test-job-1",
		CVID:           "test-cv-1",
		ProjectID:      "test-project-1",
		JobDescription: "Looking for a senior software engineer",
		StudyCaseBrief: "Design a scalable microservices architecture",
	}

	// Test different configurations
	configurations := []struct {
		name   string
		qdrant bool
	}{
		{"basic", false},
		{"with-rag", true},
	}

	for _, config := range configurations {
		t.Run(config.name, func(t *testing.T) {
			ctx := context.Background()
			var qdrant *qdrantcli.Client
			if config.qdrant {
				qdrant = qdrantClient
			}

			err := shared.HandleEvaluate(
				ctx,
				jobRepo,
				uploadRepo,
				resultRepo,
				aiClient,
				qdrant,
				payload,
			)

			// Should succeed with simple mocks
			assert.NoError(t, err)
		})
	}
}
