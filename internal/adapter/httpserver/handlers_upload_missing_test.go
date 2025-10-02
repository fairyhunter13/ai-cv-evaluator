package httpserver_test

import (
	"bytes"
	"errors"
	"io"
	"mime/multipart"
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

func createMockTextExtractorError(t *testing.T) *domainmocks.MockTextExtractor {
	mockExtractor := domainmocks.NewMockTextExtractor(t)
	mockExtractor.EXPECT().ExtractPath(mock.Anything, mock.Anything, mock.Anything).Return("", errors.New("extract fail")).Maybe()
	return mockExtractor
}

// Mock creation functions
func createMockUploadRepo3(t *testing.T) *domainmocks.MockUploadRepository {
	mockRepo := domainmocks.NewMockUploadRepository(t)
	mockRepo.EXPECT().Create(mock.Anything, mock.Anything).RunAndReturn(func(_ domain.Context, u domain.Upload) (string, error) {
		if u.Type == domain.UploadTypeCV {
			return "cv-1", nil
		}
		return "pr-1", nil
	}).Maybe()
	mockRepo.EXPECT().Get(mock.Anything, mock.Anything).RunAndReturn(func(_ domain.Context, id string) (domain.Upload, error) {
		return domain.Upload{ID: id}, nil
	}).Maybe()
	mockRepo.EXPECT().Count(mock.Anything).Return(int64(0), nil).Maybe()
	mockRepo.EXPECT().CountByType(mock.Anything, mock.Anything).Return(int64(0), nil).Maybe()
	return mockRepo
}

func createMockJobRepo3(t *testing.T) *domainmocks.MockJobRepository {
	mockRepo := domainmocks.NewMockJobRepository(t)
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

func createMockQueue3(t *testing.T) *domainmocks.MockQueue {
	mockRepo := domainmocks.NewMockQueue(t)
	mockRepo.EXPECT().EnqueueEvaluate(mock.Anything, mock.Anything).Return("t-1", nil).Maybe()
	return mockRepo
}

func newSrv(t *testing.T, ext domain.TextExtractor) *httpserver.Server {
	t.Helper()
	cfg := config.Config{MaxUploadMB: 5, Port: 8080, AppEnv: "dev"}
	upRepo := createMockUploadRepo3(t)
	jobRepo := createMockJobRepo3(t)
	queue := createMockQueue3(t)
	upSvc := usecase.NewUploadService(upRepo)
	evSvc := usecase.NewEvaluateService(jobRepo, queue, upRepo)
	resSvc := usecase.NewResultService(jobRepo, nil)
	return httpserver.NewServer(cfg, upSvc, evSvc, resSvc, ext, nil, nil, nil)
}

func TestUploadHandler_MissingCV(t *testing.T) {
	srv := newSrv(t, createMockTextExtractorError(t))
	// Only project part
	buf := &bytes.Buffer{}
	w := multipart.NewWriter(buf)
	fw, _ := w.CreateFormFile("project", "prj.txt")
	_, _ = fw.Write([]byte("pr"))
	_ = w.Close()
	r := httptest.NewRequest(http.MethodPost, "/v1/upload", bytes.NewReader(buf.Bytes()))
	r.Header.Set("Content-Type", w.FormDataContentType())
	r.Header.Set("Accept", "application/json")
	rec := httptest.NewRecorder()
	srv.UploadHandler()(rec, r)
	if rec.Result().StatusCode != http.StatusBadRequest {
		t.Fatalf("want 400")
	}
}

func TestUploadHandler_MissingProject(t *testing.T) {
	srv := newSrv(t, createMockTextExtractorError(t))
	buf := &bytes.Buffer{}
	w := multipart.NewWriter(buf)
	fw, _ := w.CreateFormFile("cv", "cv.txt")
	_, _ = fw.Write([]byte("cv"))
	_ = w.Close()
	r := httptest.NewRequest(http.MethodPost, "/v1/upload", io.NopCloser(bytes.NewReader(buf.Bytes())))
	r.Header.Set("Content-Type", w.FormDataContentType())
	r.Header.Set("Accept", "application/json")
	rec := httptest.NewRecorder()
	srv.UploadHandler()(rec, r)
	if rec.Result().StatusCode != http.StatusBadRequest {
		t.Fatalf("want 400")
	}
}

func TestUploadHandler_PDF_ExtractorError(t *testing.T) {
	srv := newSrv(t, createMockTextExtractorError(t))
	pdf := []byte("%PDF-1.7\n%")
	body, ctype := func() (*bytes.Buffer, string) {
		buf := &bytes.Buffer{}
		w := multipart.NewWriter(buf)
		fw, _ := w.CreateFormFile("cv", "cv.pdf")
		_, _ = fw.Write(pdf)
		fw2, _ := w.CreateFormFile("project", "prj.pdf")
		_, _ = fw2.Write(pdf)
		_ = w.Close()
		return buf, w.FormDataContentType()
	}()
	r := httptest.NewRequest(http.MethodPost, "/v1/upload", io.NopCloser(bytes.NewReader(body.Bytes())))
	r.Header.Set("Content-Type", ctype)
	r.Header.Set("Accept", "application/json")
	rec := httptest.NewRecorder()
	srv.UploadHandler()(rec, r)
	if rec.Result().StatusCode != http.StatusBadRequest {
		t.Fatalf("want 400")
	}
}
