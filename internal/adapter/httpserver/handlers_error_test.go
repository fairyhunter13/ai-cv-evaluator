package httpserver_test

import (
	"bytes"
	"encoding/json"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/httprate"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	httpserver "github.com/fairyhunter13/ai-cv-evaluator/internal/adapter/httpserver"
	"github.com/fairyhunter13/ai-cv-evaluator/internal/config"
	"github.com/fairyhunter13/ai-cv-evaluator/internal/domain"
	domainmocks "github.com/fairyhunter13/ai-cv-evaluator/internal/domain/mocks"
	"github.com/fairyhunter13/ai-cv-evaluator/internal/usecase"
)

// Mock creation functions
func createMockUploadRepoError(t *testing.T) *domainmocks.UploadRepository {
	mockRepo := domainmocks.NewUploadRepository(t)
	mockRepo.EXPECT().Create(mock.Anything, mock.Anything).Return("upload-1", nil).Maybe()
	mockRepo.EXPECT().Get(mock.Anything, mock.Anything).Return(domain.Upload{ID: "upload-1"}, nil).Maybe()
	mockRepo.EXPECT().Count(mock.Anything).Return(int64(0), nil).Maybe()
	mockRepo.EXPECT().CountByType(mock.Anything, mock.Anything).Return(int64(0), nil).Maybe()
	return mockRepo
}

func createMockJobRepoError(t *testing.T) *domainmocks.JobRepository {
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

func createMockQueueError(t *testing.T) *domainmocks.Queue {
	mockRepo := domainmocks.NewQueue(t)
	mockRepo.EXPECT().EnqueueEvaluate(mock.Anything, mock.Anything).Return("t-1", nil).Maybe()
	return mockRepo
}

func createMockJobRepoNotFound(t *testing.T) *domainmocks.JobRepository {
	mockRepo := domainmocks.NewJobRepository(t)
	mockRepo.EXPECT().Create(mock.Anything, mock.Anything).Return("", nil).Maybe()
	mockRepo.EXPECT().UpdateStatus(mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(nil).Maybe()
	mockRepo.EXPECT().Get(mock.Anything, mock.Anything).Return(domain.Job{}, domain.ErrNotFound).Maybe()
	mockRepo.EXPECT().FindByIdempotencyKey(mock.Anything, mock.Anything).Return(domain.Job{}, domain.ErrNotFound).Maybe()
	mockRepo.EXPECT().Count(mock.Anything).Return(int64(0), nil).Maybe()
	mockRepo.EXPECT().CountByStatus(mock.Anything, mock.Anything).Return(int64(0), nil).Maybe()
	mockRepo.EXPECT().List(mock.Anything, mock.Anything, mock.Anything).Return([]domain.Job{}, nil).Maybe()
	mockRepo.EXPECT().GetAverageProcessingTime(mock.Anything).Return(float64(0), nil).Maybe()
	return mockRepo
}

func TestEvaluateHandler_400_InvalidJSON(t *testing.T) {
	cfg := config.Config{Port: 8080}
	upRepo := createMockUploadRepoError(t)
	jobRepo := createMockJobRepoError(t)
	queue := createMockQueueError(t)
	upSvc := usecase.NewUploadService(upRepo)
	evSvc := usecase.NewEvaluateService(jobRepo, queue, upRepo)
	resSvc := usecase.NewResultService(jobRepo, nil)
	srv := httpserver.NewServer(cfg, upSvc, evSvc, resSvc, nil, nil, nil, nil)
	r := httptest.NewRequest(http.MethodPost, "/v1/evaluate", strings.NewReader("{invalid json"))
	r.Header.Set("Content-Type", "application/json")
	r.Header.Set("Accept", "application/json")
	w := httptest.NewRecorder()
	h := srv.EvaluateHandler()
	h(w, r)
	resp := w.Result()
	require.Equal(t, http.StatusBadRequest, resp.StatusCode)
	b, _ := io.ReadAll(resp.Body)
	_ = resp.Body.Close()
	var m map[string]any
	require.NoError(t, json.Unmarshal(b, &m))
	errObj, ok := m["error"].(map[string]any)
	require.True(t, ok)
	require.Equal(t, "INVALID_ARGUMENT", errObj["code"])
}

func TestEvaluateHandler_400_ValidationFailed(t *testing.T) {
	cfg := config.Config{Port: 8080}
	upRepo := createMockUploadRepoError(t)
	jobRepo := createMockJobRepoError(t)
	queue := createMockQueueError(t)
	upSvc := usecase.NewUploadService(upRepo)
	evSvc := usecase.NewEvaluateService(jobRepo, queue, upRepo)
	resSvc := usecase.NewResultService(jobRepo, nil)
	srv := httpserver.NewServer(cfg, upSvc, evSvc, resSvc, nil, nil, nil, nil)
	// Missing required fields
	payload := map[string]any{"cv_id": "cv1"}
	b, _ := json.Marshal(payload)
	r := httptest.NewRequest(http.MethodPost, "/v1/evaluate", bytes.NewReader(b))
	r.Header.Set("Content-Type", "application/json")
	r.Header.Set("Accept", "application/json")
	w := httptest.NewRecorder()
	h := srv.EvaluateHandler()
	h(w, r)
	resp := w.Result()
	require.Equal(t, http.StatusBadRequest, resp.StatusCode)
	_ = resp.Body.Close()
}

func TestEvaluateHandler_413_PayloadTooLarge(t *testing.T) {
	cfg := config.Config{Port: 8080}
	upRepo := createMockUploadRepoError(t)
	jobRepo := createMockJobRepoError(t)
	queue := createMockQueueError(t)
	upSvc := usecase.NewUploadService(upRepo)
	evSvc := usecase.NewEvaluateService(jobRepo, queue, upRepo)
	resSvc := usecase.NewResultService(jobRepo, nil)
	srv := httpserver.NewServer(cfg, upSvc, evSvc, resSvc, nil, nil, nil, nil)
	// Create payload > 1MB
	largeStr := strings.Repeat("x", 1024*1024+1)
	payload := map[string]any{
		"cv_id":            "cv1",
		"project_id":       "pr1",
		"job_description":  largeStr,
		"study_case_brief": "brief",
	}
	b, _ := json.Marshal(payload)
	r := httptest.NewRequest(http.MethodPost, "/v1/evaluate", bytes.NewReader(b))
	r.Header.Set("Content-Type", "application/json")
	r.Header.Set("Accept", "application/json")
	w := httptest.NewRecorder()
	h := srv.EvaluateHandler()
	h(w, r)
	resp := w.Result()
	require.Equal(t, http.StatusBadRequest, resp.StatusCode)
	_ = resp.Body.Close()
}

func TestEvaluateHandler_406_NotAcceptable(t *testing.T) {
	cfg := config.Config{Port: 8080}
	upRepo := createMockUploadRepoError(t)
	jobRepo := createMockJobRepoError(t)
	queue := createMockQueueError(t)
	upSvc := usecase.NewUploadService(upRepo)
	evSvc := usecase.NewEvaluateService(jobRepo, queue, upRepo)
	resSvc := usecase.NewResultService(jobRepo, nil)
	srv := httpserver.NewServer(cfg, upSvc, evSvc, resSvc, nil, nil, nil, nil)
	r := httptest.NewRequest(http.MethodPost, "/v1/evaluate", strings.NewReader("{}"))
	r.Header.Set("Content-Type", "application/json")
	r.Header.Set("Accept", "text/html")
	w := httptest.NewRecorder()
	h := srv.EvaluateHandler()
	h(w, r)
	resp := w.Result()
	require.Equal(t, http.StatusNotAcceptable, resp.StatusCode)
	_ = resp.Body.Close()
}

func TestUploadHandler_415_UnsupportedMediaType(t *testing.T) {
	srv := newTestServer(t)
	// Upload with .exe extension
	body, ctype := buildMultipartWithNames(t, map[string][]byte{
		"cv":      []byte("text"),
		"project": []byte("text"),
	}, map[string]string{
		"cv":      "cv.exe",
		"project": "project.txt",
	})
	r := httptest.NewRequest(http.MethodPost, "/v1/upload", bytes.NewReader(body.Bytes()))
	r.Header.Set("Content-Type", ctype)
	r.Header.Set("Accept", "application/json")
	w := httptest.NewRecorder()
	h := srv.UploadHandler()
	h(w, r)
	resp := w.Result()
	require.Equal(t, http.StatusUnsupportedMediaType, resp.StatusCode)
	_ = resp.Body.Close()
}

func TestResultHandler_404_NotFound(t *testing.T) {
	jr := createMockJobRepoNotFound(t)
	cfg := config.Config{Port: 8080}
	upRepo := createMockUploadRepoError(t)
	jobRepo := createMockJobRepoError(t)
	queue := createMockQueueError(t)
	upSvc := usecase.NewUploadService(upRepo)
	evSvc := usecase.NewEvaluateService(jobRepo, queue, upRepo)
	resSvc := usecase.NewResultService(jr, nil)
	srv := httpserver.NewServer(cfg, upSvc, evSvc, resSvc, nil, nil, nil, nil)
	router := chi.NewRouter()
	router.Get("/v1/result/{id}", srv.ResultHandler())
	r := httptest.NewRequest(http.MethodGet, "/v1/result/nonexistent", nil)
	r.Header.Set("Accept", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, r)
	resp := w.Result()
	require.Equal(t, http.StatusNotFound, resp.StatusCode)
	_ = resp.Body.Close()
}

func TestRateLimiting_429(t *testing.T) {
	cfg := config.Config{Port: 8080, RateLimitPerMin: 2}
	upRepo := createMockUploadRepoError(t)
	jobRepo := createMockJobRepoError(t)
	queue := createMockQueueError(t)
	upSvc := usecase.NewUploadService(upRepo)
	evSvc := usecase.NewEvaluateService(jobRepo, queue, upRepo)
	resSvc := usecase.NewResultService(jobRepo, nil)
	srv := httpserver.NewServer(cfg, upSvc, evSvc, resSvc, nil, nil, nil, nil)
	router := chi.NewRouter()
	router.Use(httprate.LimitByIP(cfg.RateLimitPerMin, 1*time.Minute))
	router.Post("/v1/evaluate", srv.EvaluateHandler())

	// Send requests rapidly
	for i := 0; i < 3; i++ {
		r := httptest.NewRequest(http.MethodPost, "/v1/evaluate", strings.NewReader("{}"))
		r.Header.Set("Content-Type", "application/json")
		r.Header.Set("Accept", "application/json")
		r.RemoteAddr = "127.0.0.1:1234"
		w := httptest.NewRecorder()
		router.ServeHTTP(w, r)
		resp := w.Result()
		_ = resp.Body.Close()
		if i < 2 {
			require.NotEqual(t, http.StatusTooManyRequests, resp.StatusCode)
		} else {
			require.Equal(t, http.StatusTooManyRequests, resp.StatusCode)
		}
	}
}

func buildMultipartWithNames(t *testing.T, fields map[string][]byte, names map[string]string) (body *bytes.Buffer, contentType string) {
	t.Helper()
	buf := &bytes.Buffer{}
	mw := multipart.NewWriter(buf)
	for field, data := range fields {
		filename := names[field]
		if filename == "" {
			filename = field + ".txt"
		}
		fw, err := mw.CreateFormFile(field, filename)
		require.NoError(t, err)
		_, err = fw.Write(data)
		require.NoError(t, err)
	}
	require.NoError(t, mw.Close())
	return buf, mw.FormDataContentType()
}
