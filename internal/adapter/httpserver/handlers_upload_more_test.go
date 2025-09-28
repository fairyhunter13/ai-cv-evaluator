package httpserver_test

import (
	"bytes"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/require"

	httpserver "github.com/fairyhunter13/ai-cv-evaluator/internal/adapter/httpserver"
	"github.com/fairyhunter13/ai-cv-evaluator/internal/config"
	"github.com/fairyhunter13/ai-cv-evaluator/internal/domain"
	"github.com/fairyhunter13/ai-cv-evaluator/internal/usecase"
)

type okExtractor struct{}
func (n *okExtractor) ExtractPath(_ domain.Context, _ string, _ string) (string, error) { return "text", nil }

type stubUploadRepo2 struct{ domain.UploadRepository }
func (s *stubUploadRepo2) Create(_ domain.Context, u domain.Upload) (string, error) { if u.Type==domain.UploadTypeCV {return "cv-1", nil}; return "pr-1", nil }
func (s *stubUploadRepo2) Get(_ domain.Context, id string) (domain.Upload, error) { return domain.Upload{ID:id}, nil }

type noopJobRepo2 struct{}
func (n *noopJobRepo2) Create(_ domain.Context, _ domain.Job) (string, error) { return "job-1", nil }
func (n *noopJobRepo2) UpdateStatus(_ domain.Context, _ string, _ domain.JobStatus, _ *string) error { return nil }
func (n *noopJobRepo2) Get(_ domain.Context, id string) (domain.Job, error) { return domain.Job{ID:id}, nil }
func (n *noopJobRepo2) FindByIdempotencyKey(_ domain.Context, _ string) (domain.Job, error) { return domain.Job{}, domain.ErrNotFound }

type noopQueue2 struct{}
func (q *noopQueue2) EnqueueEvaluate(_ domain.Context, _ domain.EvaluateTaskPayload) (string, error) { return "t-1", nil }

func newSrvWithExt(t *testing.T, ext domain.TextExtractor) *httpserver.Server {
	t.Helper()
	cfg := config.Config{MaxUploadMB: 5, Port: 8080, AppEnv: "dev"}
	upSvc := usecase.NewUploadService(&stubUploadRepo2{})
	evSvc := usecase.NewEvaluateService(&noopJobRepo2{}, &noopQueue2{}, &stubUploadRepo2{})
	resSvc := usecase.NewResultService(&noopJobRepo2{}, nil)
	return httpserver.NewServer(cfg, upSvc, evSvc, resSvc, ext, nil, nil, nil, nil)
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
	body, ctype := buildMultipartWithNames2(t, map[string][]byte{"cv": []byte("cv"), "project": []byte("pr")}, map[string]string{"cv":"cv.txt","project":"prj.txt"})
	r := httptest.NewRequest(http.MethodPost, "/v1/upload", bytes.NewReader(body.Bytes()))
	r.Header.Set("Content-Type", ctype)
	r.Header.Set("Accept", "text/html")
	w := httptest.NewRecorder()
	srv.UploadHandler()(w, r)
	if w.Result().StatusCode != http.StatusNotAcceptable { t.Fatalf("want 406") }
}

func TestUploadHandler_InvalidContentType(t *testing.T) {
	srv := newSrvWithExt(t, &okExtractor{})
	r := httptest.NewRequest(http.MethodPost, "/v1/upload", bytes.NewReader([]byte("not multipart")))
	r.Header.Set("Content-Type", "application/json")
	r.Header.Set("Accept", "application/json")
	w := httptest.NewRecorder()
	srv.UploadHandler()(w, r)
	if w.Result().StatusCode != http.StatusBadRequest { t.Fatalf("want 400 invalid argument") }
}

func TestUploadHandler_ProjectExtensionUnsupported(t *testing.T) {
	srv := newSrvWithExt(t, &okExtractor{})
	body, ctype := buildMultipartWithNames2(t, map[string][]byte{"cv": []byte("cv"), "project": []byte("pr")}, map[string]string{"cv":"cv.txt","project":"prj.exe"})
	r := httptest.NewRequest(http.MethodPost, "/v1/upload", bytes.NewReader(body.Bytes()))
	r.Header.Set("Content-Type", ctype)
	r.Header.Set("Accept", "application/json")
	w := httptest.NewRecorder()
	srv.UploadHandler()(w, r)
	if w.Result().StatusCode != http.StatusUnsupportedMediaType { t.Fatalf("want 415") }
}

func TestUploadHandler_PDF_Extractor_Success(t *testing.T) {
	srv := newSrvWithExt(t, &okExtractor{})
	// Minimal headers for detection
	pdf := []byte("%PDF-1.4\n%")
	body, ctype := buildMultipartWithNames2(t, map[string][]byte{"cv": pdf, "project": pdf}, map[string]string{"cv":"cv.pdf","project":"prj.pdf"})
	r := httptest.NewRequest(http.MethodPost, "/v1/upload", io.NopCloser(bytes.NewReader(body.Bytes())))
	r.Header.Set("Content-Type", ctype)
	r.Header.Set("Accept", "application/json")
	w := httptest.NewRecorder()
	srv.UploadHandler()(w, r)
	if w.Result().StatusCode != http.StatusOK { t.Fatalf("want 200, got %d", w.Result().StatusCode) }
}
