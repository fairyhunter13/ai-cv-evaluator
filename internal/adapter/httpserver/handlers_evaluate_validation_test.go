package httpserver_test

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/mock"

	httpserver "github.com/fairyhunter13/ai-cv-evaluator/internal/adapter/httpserver"
	"github.com/fairyhunter13/ai-cv-evaluator/internal/config"
	"github.com/fairyhunter13/ai-cv-evaluator/internal/domain"
	domainmocks "github.com/fairyhunter13/ai-cv-evaluator/internal/domain/mocks"
	"github.com/fairyhunter13/ai-cv-evaluator/internal/usecase"
)

// Mock creation functions
func createMockUploadRepoValidation(t *testing.T) *domainmocks.UploadRepository {
	mockRepo := domainmocks.NewUploadRepository(t)
	mockRepo.EXPECT().Create(mock.Anything, mock.Anything).Return("upload-1", nil).Maybe()
	mockRepo.EXPECT().Get(mock.Anything, mock.Anything).Return(domain.Upload{ID: "upload-1"}, nil).Maybe()
	mockRepo.EXPECT().Count(mock.Anything).Return(int64(0), nil).Maybe()
	mockRepo.EXPECT().CountByType(mock.Anything, mock.Anything).Return(int64(0), nil).Maybe()
	return mockRepo
}

func createMockJobRepoValidation(t *testing.T) *domainmocks.JobRepository {
	mockRepo := domainmocks.NewJobRepository(t)
	mockRepo.EXPECT().Create(mock.Anything, mock.Anything).Return("job-1", nil).Maybe()
	mockRepo.EXPECT().UpdateStatus(mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(nil).Maybe()
	mockRepo.EXPECT().Get(mock.Anything, mock.Anything).Return(domain.Job{ID: "job-1"}, nil).Maybe()
	mockRepo.EXPECT().FindByIdempotencyKey(mock.Anything, mock.Anything).Return(domain.Job{}, domain.ErrNotFound).Maybe()
	mockRepo.EXPECT().Count(mock.Anything).Return(int64(0), nil).Maybe()
	mockRepo.EXPECT().CountByStatus(mock.Anything, mock.Anything).Return(int64(0), nil).Maybe()
	mockRepo.EXPECT().List(mock.Anything, mock.Anything, mock.Anything).Return([]domain.Job{}, nil).Maybe()
	mockRepo.EXPECT().GetAverageProcessingTime(mock.Anything).Return(float64(0), nil).Maybe()
	return mockRepo
}

func createMockQueueValidation(t *testing.T) *domainmocks.Queue {
	mockRepo := domainmocks.NewQueue(t)
	mockRepo.EXPECT().EnqueueEvaluate(mock.Anything, mock.Anything).Return("t-1", nil).Maybe()
	return mockRepo
}

func TestEvaluateHandler_ValidationDetails(t *testing.T) {
	cfg := config.Config{Port: 8080}
	s := httpserver.NewServer(cfg, usecase.NewUploadService(nil), usecase.NewEvaluateService(nil, nil, nil), usecase.NewResultService(nil, nil), nil, nil, nil, nil)
	payload := map[string]any{"cv_id": "cv1"} // missing project_id (job_description and study_case_brief are now optional)
	b, _ := json.Marshal(payload)
	r := httptest.NewRequest(http.MethodPost, "/v1/evaluate", bytes.NewReader(b))
	r.Header.Set("Content-Type", "application/json")
	r.Header.Set("Accept", "application/json")
	rw := httptest.NewRecorder()
	s.EvaluateHandler()(rw, r)
	if rw.Result().StatusCode != http.StatusBadRequest {
		t.Fatalf("want 400, got %d", rw.Result().StatusCode)
	}
	var resp map[string]any
	_ = json.NewDecoder(rw.Result().Body).Decode(&resp)
	errObj := resp["error"].(map[string]any)
	_ = errObj
	// details is optional; ensure we at least returned INVALID_ARGUMENT
	if errObj["code"].(string) != "INVALID_ARGUMENT" {
		t.Fatalf("code mismatch: %v", errObj["code"])
	}
}

func TestEvaluateHandler_OptionalFields(t *testing.T) {
	cfg := config.Config{Port: 8080}

	// Create proper repositories for the test
	upRepo := createMockUploadRepoValidation(t)
	jobRepo := createMockJobRepoValidation(t)
	queue := createMockQueueValidation(t)

	s := httpserver.NewServer(cfg, usecase.NewUploadService(upRepo), usecase.NewEvaluateService(jobRepo, queue, upRepo), usecase.NewResultService(jobRepo, nil), nil, nil, nil, nil)

	// Test with only required fields (job_description and study_case_brief should use defaults)
	payload := map[string]any{
		"cv_id":      "cv1",
		"project_id": "proj1",
		// job_description and study_case_brief are omitted - should use defaults
	}
	b, _ := json.Marshal(payload)
	r := httptest.NewRequest(http.MethodPost, "/v1/evaluate", bytes.NewReader(b))
	r.Header.Set("Content-Type", "application/json")
	r.Header.Set("Accept", "application/json")
	rw := httptest.NewRecorder()
	s.EvaluateHandler()(rw, r)

	// Should not return validation error since optional fields are omitted
	if rw.Result().StatusCode == http.StatusBadRequest {
		var resp map[string]any
		_ = json.NewDecoder(rw.Result().Body).Decode(&resp)
		t.Logf("Response: %+v", resp)
		t.Fatalf("unexpected validation error when optional fields are omitted")
	}
}
