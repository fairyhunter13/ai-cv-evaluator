package httpserver_test

import (
	"bytes"
	"io"
	"mime/multipart"
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
)

type okExtractor struct{}

func (n *okExtractor) ExtractPath(_ domain.Context, _ string, _ string) (string, error) {
	return "text", nil
}

// Mock creation functions
func createMockUploadRepo2(t *testing.T) *domainmocks.UploadRepository {
	mockRepo := domainmocks.NewUploadRepository(t)
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

func createMockJobRepo2(t *testing.T) *domainmocks.JobRepository {
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

func createMockQueue2(t *testing.T) *domainmocks.Queue {
	mockRepo := domainmocks.NewQueue(t)
	mockRepo.EXPECT().EnqueueEvaluate(mock.Anything, mock.Anything).Return("t-1", nil).Maybe()
	return mockRepo
}

func newSrvWithExt(t *testing.T, ext domain.TextExtractor) *httpserver.Server {
	t.Helper()
	cfg := config.Config{MaxUploadMB: 5, Port: 8080, AppEnv: "dev"}
	upRepo := createMockUploadRepo2(t)
	jobRepo := createMockJobRepo2(t)
	queue := createMockQueue2(t)
	upSvc := usecase.NewUploadService(upRepo)
	evSvc := usecase.NewEvaluateService(jobRepo, queue, upRepo)
	resSvc := usecase.NewResultService(jobRepo, nil)
	return httpserver.NewServer(cfg, upSvc, evSvc, resSvc, ext, nil, nil, nil)
}

func buildMultipartWithNames2(t *testing.T, fields map[string][]byte, names map[string]string) (body *bytes.Buffer, contentType string) {
	t.Helper()
	buf := &bytes.Buffer{}
	w := multipart.NewWriter(buf)
	for name, data := range fields {
		filename := names[name]
		fw, err := w.CreateFormFile(name, filename)
		require.NoError(t, err)
		_, err = fw.Write(data)
		require.NoError(t, err)
	}
	require.NoError(t, w.Close())
	return buf, w.FormDataContentType()
}

func TestUploadHandler_406_NotAcceptable(t *testing.T) {
	srv := newSrvWithExt(t, &okExtractor{})
	body, ctype := buildMultipartWithNames2(t, map[string][]byte{"cv": []byte("cv"), "project": []byte("pr")}, map[string]string{"cv": "cv.txt", "project": "prj.txt"})
	r := httptest.NewRequest(http.MethodPost, "/v1/upload", bytes.NewReader(body.Bytes()))
	r.Header.Set("Content-Type", ctype)
	r.Header.Set("Accept", "text/html")
	w := httptest.NewRecorder()
	srv.UploadHandler()(w, r)
	if w.Result().StatusCode != http.StatusNotAcceptable {
		t.Fatalf("want 406")
	}
}

func TestUploadHandler_InvalidContentType(t *testing.T) {
	srv := newSrvWithExt(t, &okExtractor{})
	r := httptest.NewRequest(http.MethodPost, "/v1/upload", bytes.NewReader([]byte("not multipart")))
	r.Header.Set("Content-Type", "application/json")
	r.Header.Set("Accept", "application/json")
	w := httptest.NewRecorder()
	srv.UploadHandler()(w, r)
	if w.Result().StatusCode != http.StatusBadRequest {
		t.Fatalf("want 400 invalid argument")
	}
}

func TestUploadHandler_ProjectExtensionUnsupported(t *testing.T) {
	srv := newSrvWithExt(t, &okExtractor{})
	body, ctype := buildMultipartWithNames2(t, map[string][]byte{"cv": []byte("cv"), "project": []byte("pr")}, map[string]string{"cv": "cv.txt", "project": "prj.exe"})
	r := httptest.NewRequest(http.MethodPost, "/v1/upload", bytes.NewReader(body.Bytes()))
	r.Header.Set("Content-Type", ctype)
	r.Header.Set("Accept", "application/json")
	w := httptest.NewRecorder()
	srv.UploadHandler()(w, r)
	if w.Result().StatusCode != http.StatusUnsupportedMediaType {
		t.Fatalf("want 415")
	}
}

func TestUploadHandler_PDF_Extractor_Success(t *testing.T) {
	srv := newSrvWithExt(t, &okExtractor{})
	// Minimal headers for detection
	pdf := []byte("%PDF-1.4\n%")
	body, ctype := buildMultipartWithNames2(t, map[string][]byte{"cv": pdf, "project": pdf}, map[string]string{"cv": "cv.pdf", "project": "prj.pdf"})
	r := httptest.NewRequest(http.MethodPost, "/v1/upload", io.NopCloser(bytes.NewReader(body.Bytes())))
	r.Header.Set("Content-Type", ctype)
	r.Header.Set("Accept", "application/json")
	w := httptest.NewRecorder()
	srv.UploadHandler()(w, r)
	if w.Result().StatusCode != http.StatusOK {
		t.Fatalf("want 200, got %d", w.Result().StatusCode)
	}
}
