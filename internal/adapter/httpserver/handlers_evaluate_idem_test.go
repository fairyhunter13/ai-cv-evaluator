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
func createMockJobRepoIdem(t *testing.T, found domain.Job) *domainmocks.MockJobRepository {
	mockRepo := domainmocks.NewMockJobRepository(t)
	mockRepo.EXPECT().Create(mock.Anything, mock.Anything).Return("job-new", nil).Maybe()
	mockRepo.EXPECT().UpdateStatus(mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(nil).Maybe()
	mockRepo.EXPECT().Get(mock.Anything, mock.Anything).Return(found, nil).Maybe()
	mockRepo.EXPECT().FindByIdempotencyKey(mock.Anything, mock.Anything).RunAndReturn(func(_ domain.Context, _ string) (domain.Job, error) {
		if found.ID != "" {
			return found, nil
		}
		return domain.Job{}, domain.ErrNotFound
	}).Maybe()
	mockRepo.EXPECT().Count(mock.Anything).Return(int64(0), nil).Maybe()
	mockRepo.EXPECT().CountByStatus(mock.Anything, mock.Anything).Return(int64(0), nil).Maybe()
	mockRepo.EXPECT().List(mock.Anything, mock.Anything, mock.Anything).Return([]domain.Job{}, nil).Maybe()
	mockRepo.EXPECT().GetAverageProcessingTime(mock.Anything).Return(float64(0), nil).Maybe()
	return mockRepo
}

func createMockQueueIdem(t *testing.T) *domainmocks.MockQueue {
	mockRepo := domainmocks.NewMockQueue(t)
	mockRepo.EXPECT().EnqueueEvaluate(mock.Anything, mock.Anything).Return("t-1", nil).Maybe()
	return mockRepo
}

func createMockUploadRepoIdem(t *testing.T) *domainmocks.MockUploadRepository {
	mockRepo := domainmocks.NewMockUploadRepository(t)
	mockRepo.EXPECT().Create(mock.Anything, mock.Anything).RunAndReturn(func(_ domain.Context, u domain.Upload) (string, error) {
		if u.Type == domain.UploadTypeCV {
			return "cv", nil
		}
		return "pr", nil
	}).Maybe()
	mockRepo.EXPECT().Get(mock.Anything, mock.Anything).RunAndReturn(func(_ domain.Context, id string) (domain.Upload, error) {
		return domain.Upload{ID: id}, nil
	}).Maybe()
	mockRepo.EXPECT().Count(mock.Anything).Return(int64(0), nil).Maybe()
	mockRepo.EXPECT().CountByType(mock.Anything, mock.Anything).Return(int64(0), nil).Maybe()
	return mockRepo
}

func TestEvaluateHandler_Idempotent_ReturnsExisting(t *testing.T) {
	cfg := config.Config{Port: 8080}
	jr := createMockJobRepoIdem(t, domain.Job{ID: "existing"})
	queue := createMockQueueIdem(t)
	upRepo := createMockUploadRepoIdem(t)
	evSvc := usecase.NewEvaluateService(jr, queue, upRepo)
	s := httpserver.NewServer(cfg, usecase.NewUploadService(upRepo), evSvc, usecase.NewResultService(jr, nil), nil, nil, nil, nil)
	payload := map[string]any{"cv_id": "cv", "project_id": "pr", "job_description": "jd", "study_case_brief": "brief"}
	b, _ := json.Marshal(payload)
	r := httptest.NewRequest(http.MethodPost, "/v1/evaluate", bytes.NewReader(b))
	r.Header.Set("Content-Type", "application/json")
	r.Header.Set("Accept", "application/json")
	r.Header.Set("Idempotency-Key", "idem-1")
	rw := httptest.NewRecorder()
	s.EvaluateHandler()(rw, r)
	if rw.Result().StatusCode != http.StatusOK {
		t.Fatalf("want 200, got %d", rw.Result().StatusCode)
	}
}
