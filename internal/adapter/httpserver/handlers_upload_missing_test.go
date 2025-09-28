package httpserver_test

import (
	"bytes"
	"errors"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"testing"

	httpserver "github.com/fairyhunter13/ai-cv-evaluator/internal/adapter/httpserver"
	"github.com/fairyhunter13/ai-cv-evaluator/internal/config"
	"github.com/fairyhunter13/ai-cv-evaluator/internal/domain"
	"github.com/fairyhunter13/ai-cv-evaluator/internal/usecase"
)

type errExtractor struct{}
func (e *errExtractor) ExtractPath(_ domain.Context, _ string, _ string) (string, error) { return "", errors.New("extract fail") }

type stubUploadRepo3 struct{}
func (s *stubUploadRepo3) Create(_ domain.Context, u domain.Upload) (string, error) { if u.Type==domain.UploadTypeCV {return "cv-1", nil}; return "pr-1", nil }
func (s *stubUploadRepo3) Get(_ domain.Context, id string) (domain.Upload, error) { return domain.Upload{ID:id}, nil }

type noopJobRepo3 struct{}
func (n *noopJobRepo3) Create(_ domain.Context, _ domain.Job) (string, error) { return "job-1", nil }
func (n *noopJobRepo3) UpdateStatus(_ domain.Context, _ string, _ domain.JobStatus, _ *string) error { return nil }
func (n *noopJobRepo3) Get(_ domain.Context, id string) (domain.Job, error) { return domain.Job{ID:id}, nil }
func (n *noopJobRepo3) FindByIdempotencyKey(_ domain.Context, _ string) (domain.Job, error) { return domain.Job{}, domain.ErrNotFound }

type noopQueue3 struct{}
func (q *noopQueue3) EnqueueEvaluate(_ domain.Context, _ domain.EvaluateTaskPayload) (string, error) { return "t-1", nil }

func newSrv(t *testing.T, ext domain.TextExtractor) *httpserver.Server {
	t.Helper()
	cfg := config.Config{MaxUploadMB: 5, Port: 8080, AppEnv: "dev"}
	upSvc := usecase.NewUploadService(&stubUploadRepo3{})
	evSvc := usecase.NewEvaluateService(&noopJobRepo3{}, &noopQueue3{}, &stubUploadRepo3{})
	resSvc := usecase.NewResultService(&noopJobRepo3{}, nil)
	return httpserver.NewServer(cfg, upSvc, evSvc, resSvc, ext, nil, nil, nil, nil)
}

func TestUploadHandler_MissingCV(t *testing.T) {
	srv := newSrv(t, &errExtractor{})
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
	if rec.Result().StatusCode != http.StatusBadRequest { t.Fatalf("want 400") }
}

func TestUploadHandler_MissingProject(t *testing.T) {
	srv := newSrv(t, &errExtractor{})
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
	if rec.Result().StatusCode != http.StatusBadRequest { t.Fatalf("want 400") }
}

func TestUploadHandler_PDF_ExtractorError(t *testing.T) {
	srv := newSrv(t, &errExtractor{})
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
	if rec.Result().StatusCode != http.StatusBadRequest { t.Fatalf("want 400") }
}
