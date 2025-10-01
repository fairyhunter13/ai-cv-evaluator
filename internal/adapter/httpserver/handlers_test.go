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

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	httpserver "github.com/fairyhunter13/ai-cv-evaluator/internal/adapter/httpserver"
	"github.com/fairyhunter13/ai-cv-evaluator/internal/config"
	"github.com/fairyhunter13/ai-cv-evaluator/internal/domain"
	domainmocks "github.com/fairyhunter13/ai-cv-evaluator/internal/domain/mocks"
	"github.com/fairyhunter13/ai-cv-evaluator/internal/usecase"
)

func createMockUploadRepo(t *testing.T) *domainmocks.UploadRepository {
	mockRepo := domainmocks.NewUploadRepository(t)
	mockRepo.EXPECT().Create(mock.Anything, mock.Anything).RunAndReturn(func(_ domain.Context, u domain.Upload) (string, error) {
		if u.Type == domain.UploadTypeCV {
			return "cv-" + strings.TrimSpace(u.Filename), nil
		}
		return "prj-" + strings.TrimSpace(u.Filename), nil
	}).Maybe()
	mockRepo.EXPECT().Get(mock.Anything, mock.Anything).RunAndReturn(func(_ domain.Context, id string) (domain.Upload, error) {
		return domain.Upload{ID: id}, nil
	}).Maybe()
	mockRepo.EXPECT().Count(mock.Anything).Return(int64(0), nil).Maybe()
	mockRepo.EXPECT().CountByType(mock.Anything, mock.Anything).Return(int64(0), nil).Maybe()
	return mockRepo
}

func createMockJobRepo(t *testing.T) *domainmocks.JobRepository {
	mockRepo := domainmocks.NewJobRepository(t)
	mockRepo.EXPECT().Create(mock.Anything, mock.Anything).Return("job-1", nil).Maybe()
	mockRepo.EXPECT().UpdateStatus(mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(nil).Maybe()
	mockRepo.EXPECT().Get(mock.Anything, mock.Anything).RunAndReturn(func(_ domain.Context, id string) (domain.Job, error) {
		return domain.Job{ID: id}, nil
	}).Maybe()
	mockRepo.EXPECT().FindByIdempotencyKey(mock.Anything, mock.Anything).Return(domain.Job{}, domain.ErrNotFound).Maybe()
	mockRepo.EXPECT().Count(mock.Anything).Return(int64(0), nil).Maybe()
	mockRepo.EXPECT().CountByStatus(mock.Anything, mock.Anything).Return(int64(0), nil).Maybe()
	mockRepo.EXPECT().List(mock.Anything, mock.Anything, mock.Anything).Return([]domain.Job{}, nil).Maybe()
	mockRepo.EXPECT().GetAverageProcessingTime(mock.Anything).Return(float64(0), nil).Maybe()
	return mockRepo
}

func createMockQueue(t *testing.T) *domainmocks.Queue {
	mockRepo := domainmocks.NewQueue(t)
	mockRepo.EXPECT().EnqueueEvaluate(mock.Anything, mock.Anything).Return("t-1", nil).Maybe()
	return mockRepo
}

func newTestServer(t *testing.T) *httpserver.Server {
	t.Helper()
	cfg := config.Config{MaxUploadMB: 5, Port: 8080, AppEnv: "dev"}
	uploadRepo := createMockUploadRepo(t)
	jobRepo := createMockJobRepo(t)
	queue := createMockQueue(t)
	upSvc := usecase.NewUploadService(uploadRepo)
	evSvc := usecase.NewEvaluateService(jobRepo, queue, uploadRepo)
	resSvc := usecase.NewResultService(jobRepo, nil)
	return httpserver.NewServer(cfg, upSvc, evSvc, resSvc, nil, nil, nil, nil)
}

func buildMultipart(t *testing.T, fields map[string][]byte) (body *bytes.Buffer, contentType string) {
	t.Helper()
	buf := &bytes.Buffer{}
	w := multipart.NewWriter(buf)
	for name, data := range fields {
		fw, err := w.CreateFormFile(name, name+".txt")
		require.NoError(t, err)
		_, err = fw.Write(data)
		require.NoError(t, err)
	}
	require.NoError(t, w.Close())
	return buf, w.FormDataContentType()
}

func TestUploadHandler_RejectsOctetStreamTxt(t *testing.T) {
	srv := newTestServer(t)
	r := httptest.NewRequest(http.MethodPost, "/v1/upload", nil)
	// Build multipart with binary-looking content for cv and valid text for project
	body, ctype := buildMultipart(t, map[string][]byte{
		"cv":      {0x00, 0x01, 0x02, 0x03},
		"project": []byte("hello world"),
	})
	r.Body = io.NopCloser(bytes.NewReader(body.Bytes()))
	r.Header.Set("Content-Type", ctype)
	r.Header.Set("Accept", "application/json")
	w := httptest.NewRecorder()
	h := srv.UploadHandler()
	h(w, r)
	resp := w.Result()
	require.Equal(t, http.StatusUnsupportedMediaType, resp.StatusCode)
	b, _ := io.ReadAll(resp.Body)
	_ = resp.Body.Close()
	var obj map[string]any
	require.NoError(t, json.Unmarshal(b, &obj))
	errObj, ok := obj["error"].(map[string]any)
	require.True(t, ok)
	// message should mention content rejection
	msg, _ := errObj["message"].(string)
	require.Contains(t, msg, "unsupported media type for cv (content)")
}

func TestUploadHandler_AcceptsText(t *testing.T) {
	srv := newTestServer(t)
	cv := []byte("this is a cv")
	pr := []byte("this is a project report")
	body, ctype := buildMultipart(t, map[string][]byte{"cv": cv, "project": pr})
	r := httptest.NewRequest(http.MethodPost, "/v1/upload", bytes.NewReader(body.Bytes()))
	r.Header.Set("Content-Type", ctype)
	r.Header.Set("Accept", "application/json")
	w := httptest.NewRecorder()
	h := srv.UploadHandler()
	h(w, r)
	resp := w.Result()
	// Should be 200 OK
	require.Equal(t, http.StatusOK, resp.StatusCode)
	b, _ := io.ReadAll(resp.Body)
	_ = resp.Body.Close()
	var obj map[string]string
	require.NoError(t, json.Unmarshal(b, &obj))
	require.NotEmpty(t, obj["cv_id"])
	require.NotEmpty(t, obj["project_id"])
}
