package shared_test

import (
	"context"
	"errors"
	"testing"

	"github.com/fairyhunter13/ai-cv-evaluator/internal/adapter/queue/shared"
	"github.com/fairyhunter13/ai-cv-evaluator/internal/domain"
	domainmocks "github.com/fairyhunter13/ai-cv-evaluator/internal/domain/mocks"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

// Erroring implementations to exercise failure branches

func createErrUploadRepo(_ *testing.T) *domainmocks.UploadRepository {
	mockRepo := &domainmocks.UploadRepository{}
	mockRepo.On("Get", mock.Anything, mock.Anything).Return(domain.Upload{}, errors.New("get upload failed"))
	return mockRepo
}

func createErrUploadRepoOnProject(_ *testing.T) *domainmocks.UploadRepository {
	mockRepo := &domainmocks.UploadRepository{}
	mockRepo.On("Get", mock.Anything, "cv-ok").Return(domain.Upload{ID: "cv-ok", Text: "cv content"}, nil)
	mockRepo.On("Get", mock.Anything, mock.Anything).Return(domain.Upload{}, errors.New("get project failed"))
	return mockRepo
}

func createErrAIClient(_ *testing.T) *domainmocks.AIClient {
	mockRepo := &domainmocks.AIClient{}
	mockRepo.On("ChatJSON", mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return("", errors.New("ai failed"))
	return mockRepo
}

func createErrResultRepo(_ *testing.T) *domainmocks.ResultRepository {
	mockRepo := &domainmocks.ResultRepository{}
	mockRepo.On("Upsert", mock.Anything, mock.Anything).Return(errors.New("upsert failed"))
	return mockRepo
}

func createErrJobRepoProcessing(_ *testing.T) *domainmocks.JobRepository {
	mockRepo := &domainmocks.JobRepository{}
	mockRepo.On("UpdateStatus", mock.Anything, mock.Anything, domain.JobProcessing, mock.Anything).Return(errors.New("update processing failed"))
	mockRepo.On("UpdateStatus", mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(nil)
	return mockRepo
}

func createErrJobRepoCompleted(_ *testing.T) *domainmocks.JobRepository {
	mockRepo := &domainmocks.JobRepository{}
	mockRepo.On("UpdateStatus", mock.Anything, mock.Anything, domain.JobCompleted, mock.Anything).Return(errors.New("update completed failed"))
	mockRepo.On("UpdateStatus", mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(nil)
	return mockRepo
}

func TestHandleEvaluate_FailToGetCV(t *testing.T) {
	t.Parallel()
	jobRepo := createMockJobRepository(t)
	uploadRepo := createErrUploadRepo(t)
	resultRepo := createMockResultRepository(t)
	aiClient := createMockAIClient(t)

	payload := domain.EvaluateTaskPayload{
		JobID:          "job1",
		CVID:           "cv-missing",
		ProjectID:      "project1",
		JobDescription: "Software Engineer",
		StudyCaseBrief: "Build a web app",
	}

	err := shared.HandleEvaluate(context.Background(), jobRepo, uploadRepo, resultRepo, aiClient, nil, payload)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "get upload failed")
}

func TestHandleEvaluate_FailToGetProject(t *testing.T) {
	t.Parallel()
	jobRepo := createMockJobRepository(t)
	uploadRepo := createErrUploadRepoOnProject(t)
	resultRepo := createMockResultRepository(t)
	aiClient := createMockAIClient(t)

	payload := domain.EvaluateTaskPayload{
		JobID:          "job1",
		CVID:           "cv-ok",
		ProjectID:      "project-missing",
		JobDescription: "Software Engineer",
		StudyCaseBrief: "Build a web app",
	}

	err := shared.HandleEvaluate(context.Background(), jobRepo, uploadRepo, resultRepo, aiClient, nil, payload)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "get project failed")
}

func TestHandleEvaluate_FailToUpdateProcessingStatus(t *testing.T) {
	t.Parallel()
	jobRepo := createErrJobRepoProcessing(t)
	uploadRepo := createMockUploadRepository(t)
	resultRepo := createMockResultRepository(t)
	aiClient := createMockAIClient(t)

	payload := domain.EvaluateTaskPayload{
		JobID:          "job1",
		CVID:           "cv-ok",
		ProjectID:      "project1",
		JobDescription: "Software Engineer",
		StudyCaseBrief: "Build a web app",
	}

	err := shared.HandleEvaluate(context.Background(), jobRepo, uploadRepo, resultRepo, aiClient, nil, payload)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "update processing failed")
}

func TestHandleEvaluate_FailToUpdateCompletedStatus(t *testing.T) {
	t.Parallel()
	jobRepo := createErrJobRepoCompleted(t)
	uploadRepo := createMockUploadRepository(t)
	resultRepo := createMockResultRepository(t)
	aiClient := createMockAIClient(t)

	payload := domain.EvaluateTaskPayload{
		JobID:          "job1",
		CVID:           "cv-ok",
		ProjectID:      "project1",
		JobDescription: "Software Engineer",
		StudyCaseBrief: "Build a web app",
	}

	err := shared.HandleEvaluate(context.Background(), jobRepo, uploadRepo, resultRepo, aiClient, nil, payload)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "update completed failed")
}

func TestHandleEvaluate_FailToUpsertResult(t *testing.T) {
	t.Parallel()
	jobRepo := createMockJobRepository(t)
	uploadRepo := createMockUploadRepository(t)
	resultRepo := createErrResultRepo(t)
	aiClient := createMockAIClient(t)

	payload := domain.EvaluateTaskPayload{
		JobID:          "job1",
		CVID:           "cv-ok",
		ProjectID:      "project1",
		JobDescription: "Software Engineer",
		StudyCaseBrief: "Build a web app",
	}

	err := shared.HandleEvaluate(context.Background(), jobRepo, uploadRepo, resultRepo, aiClient, nil, payload)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "upsert failed")
}

func TestHandleEvaluate_FailAIClient(t *testing.T) {
	t.Parallel()
	jobRepo := createMockJobRepository(t)
	uploadRepo := createMockUploadRepository(t)
	resultRepo := createMockResultRepository(t)
	aiClient := createErrAIClient(t)

	payload := domain.EvaluateTaskPayload{
		JobID:          "job1",
		CVID:           "cv-ok",
		ProjectID:      "project1",
		JobDescription: "Software Engineer",
		StudyCaseBrief: "Build a web app",
	}

	err := shared.HandleEvaluate(context.Background(), jobRepo, uploadRepo, resultRepo, aiClient, nil, payload)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "ai failed")
}
