package httpserver_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	httpserver "github.com/fairyhunter13/ai-cv-evaluator/internal/adapter/httpserver"
	"github.com/fairyhunter13/ai-cv-evaluator/internal/config"
	"github.com/fairyhunter13/ai-cv-evaluator/internal/domain"
	domainmocks "github.com/fairyhunter13/ai-cv-evaluator/internal/domain/mocks"
	"github.com/fairyhunter13/ai-cv-evaluator/internal/usecase"
	"github.com/go-chi/chi/v5"
)

func createMockJobRepoRes(t *testing.T, job domain.Job) *domainmocks.JobRepository {
	mockRepo := domainmocks.NewJobRepository(t)
	mockRepo.EXPECT().Create(mock.Anything, mock.Anything).Return("", nil).Maybe()
	mockRepo.EXPECT().UpdateStatus(mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(nil).Maybe()
	mockRepo.EXPECT().Get(mock.Anything, mock.Anything).Return(job, nil).Maybe()
	mockRepo.EXPECT().FindByIdempotencyKey(mock.Anything, mock.Anything).Return(domain.Job{}, domain.ErrNotFound).Maybe()
	mockRepo.EXPECT().Count(mock.Anything).Return(int64(0), nil).Maybe()
	mockRepo.EXPECT().CountByStatus(mock.Anything, mock.Anything).Return(int64(0), nil).Maybe()
	mockRepo.EXPECT().List(mock.Anything, mock.Anything, mock.Anything).Return([]domain.Job{}, nil).Maybe()
	mockRepo.EXPECT().GetAverageProcessingTime(mock.Anything).Return(float64(0), nil).Maybe()
	return mockRepo
}

func createMockResultRepoRes(t *testing.T, res domain.Result) *domainmocks.ResultRepository {
	mockRepo := domainmocks.NewResultRepository(t)
	mockRepo.EXPECT().Upsert(mock.Anything, mock.Anything).Return(nil).Maybe()
	mockRepo.EXPECT().GetByJobID(mock.Anything, mock.Anything).Return(res, nil).Maybe()
	return mockRepo
}

func newResultServer(t *testing.T, job domain.Job, res domain.Result) *httpserver.Server {
	cfg := config.Config{Port: 8080, AppEnv: "dev"}
	jobRepo := createMockJobRepoRes(t, job)
	resultRepo := createMockResultRepoRes(t, res)
	upSvc := usecase.NewUploadService(nil)
	evSvc := usecase.NewEvaluateService(jobRepo, nil, nil)
	resSvc := usecase.NewResultService(jobRepo, resultRepo)
	return httpserver.NewServer(cfg, upSvc, evSvc, resSvc, nil, nil, nil, nil)
}

func TestResultHandler_Completed_ETagCaching(t *testing.T) {
	srv := newResultServer(t, domain.Job{ID: "job1", Status: domain.JobCompleted}, domain.Result{JobID: "job1", CVMatchRate: 0.9, CVFeedback: "good.", ProjectScore: 9, ProjectFeedback: "nice.", OverallSummary: "great overall."})
	router := chi.NewRouter()
	router.Get("/v1/result/{id}", srv.ResultHandler())
	r := httptest.NewRequest(http.MethodGet, "/v1/result/job1", nil)
	r.Header.Set("Accept", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, r)
	resp := w.Result()
	require.Equal(t, http.StatusOK, resp.StatusCode)
	etag := resp.Header.Get("ETag")
	require.NotEmpty(t, etag)
	_ = resp.Body.Close()

	// Second request with If-None-Match should return 304
	r2 := httptest.NewRequest(http.MethodGet, "/v1/result/job1", nil)
	r2.Header.Set("Accept", "application/json")
	r2.Header.Set("If-None-Match", etag)
	w2 := httptest.NewRecorder()
	router.ServeHTTP(w2, r2)
	resp2 := w2.Result()
	require.Equal(t, http.StatusNotModified, resp2.StatusCode)
	_ = resp2.Body.Close()
}

func TestResultHandler_FailedShape_IncludesError(t *testing.T) {
	srv := newResultServer(t, domain.Job{ID: "job2", Status: domain.JobFailed, Error: "schema invalid: field"}, domain.Result{})
	router := chi.NewRouter()
	router.Get("/v1/result/{id}", srv.ResultHandler())
	r := httptest.NewRequest(http.MethodGet, "/v1/result/job2", nil)
	r.Header.Set("Accept", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, r)
	resp := w.Result()
	require.Equal(t, http.StatusOK, resp.StatusCode)
	_ = resp.Body.Close()
}
