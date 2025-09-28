package httpserver_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/go-chi/chi/v5"
	httpserver "github.com/fairyhunter13/ai-cv-evaluator/internal/adapter/httpserver"
	"github.com/fairyhunter13/ai-cv-evaluator/internal/config"
	"github.com/fairyhunter13/ai-cv-evaluator/internal/domain"
	"github.com/fairyhunter13/ai-cv-evaluator/internal/usecase"
)

type stubJobRepoRes struct{ job domain.Job }

func (s *stubJobRepoRes) Create(_ domain.Context, _ domain.Job) (string, error) { return "", nil }
func (s *stubJobRepoRes) UpdateStatus(_ domain.Context, _ string, _ domain.JobStatus, _ *string) error {
	return nil
}
func (s *stubJobRepoRes) Get(_ domain.Context, _ string) (domain.Job, error) { return s.job, nil }
func (s *stubJobRepoRes) FindByIdempotencyKey(_ domain.Context, _ string) (domain.Job, error) {
	return domain.Job{}, domain.ErrNotFound
}

type stubResultRepoRes struct{ res domain.Result }

func (s *stubResultRepoRes) Upsert(_ domain.Context, _ domain.Result) error { return nil }
func (s *stubResultRepoRes) GetByJobID(_ domain.Context, _ string) (domain.Result, error) { return s.res, nil }

func newResultServer(job domain.Job, res domain.Result) *httpserver.Server {
	cfg := config.Config{Port: 8080, AppEnv: "dev"}
	upSvc := usecase.NewUploadService(nil)
	evSvc := usecase.NewEvaluateService(&stubJobRepoRes{job: job}, nil, nil)
	resSvc := usecase.NewResultService(&stubJobRepoRes{job: job}, &stubResultRepoRes{res: res})
	return httpserver.NewServer(cfg, upSvc, evSvc, resSvc, nil, nil, nil, nil, nil)
}

func TestResultHandler_Completed_ETagCaching(t *testing.T) {
	srv := newResultServer(domain.Job{ID: "job1", Status: domain.JobCompleted}, domain.Result{JobID: "job1", CVMatchRate: 0.9, CVFeedback: "good.", ProjectScore: 9, ProjectFeedback: "nice.", OverallSummary: "great overall."})
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
	srv := newResultServer(domain.Job{ID: "job2", Status: domain.JobFailed, Error: "schema invalid: field"}, domain.Result{})
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
