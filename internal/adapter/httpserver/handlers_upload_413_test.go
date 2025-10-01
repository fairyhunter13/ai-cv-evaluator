package httpserver_test

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"testing"

	"mime/multipart"

	httpserver "github.com/fairyhunter13/ai-cv-evaluator/internal/adapter/httpserver"
	"github.com/fairyhunter13/ai-cv-evaluator/internal/config"
	"github.com/fairyhunter13/ai-cv-evaluator/internal/usecase"
	"github.com/stretchr/testify/require"
)

func TestUploadHandler_413_PayloadTooLarge(t *testing.T) {
	// Set MaxUploadMB=1; handler caps body to 2MB for multipart
	cfg := config.Config{Port: 8080, MaxUploadMB: 1}
	s := httpserver.NewServer(cfg, usecase.NewUploadService(nil), usecase.NewEvaluateService(nil, nil, nil), usecase.NewResultService(nil, nil), nil, nil, nil, nil)
	// Build multipart > 2MB
	big1 := bytes.Repeat([]byte("A"), 1536*1024) // 1.5MB
	big2 := bytes.Repeat([]byte("B"), 800*1024)  // 0.78MB => total ~2.28MB
	buf := &bytes.Buffer{}
	w := multipart.NewWriter(buf)
	fw, err := w.CreateFormFile("cv", "cv.txt")
	require.NoError(t, err)
	_, err = fw.Write(big1)
	require.NoError(t, err)
	fw2, err := w.CreateFormFile("project", "prj.txt")
	require.NoError(t, err)
	_, err = fw2.Write(big2)
	require.NoError(t, err)
	require.NoError(t, w.Close())
	r := httptest.NewRequest(http.MethodPost, "/v1/upload", bytes.NewReader(buf.Bytes()))
	r.Header.Set("Content-Type", w.FormDataContentType())
	r.Header.Set("Accept", "application/json")
	rec := httptest.NewRecorder()
	s.UploadHandler()(rec, r)
	if rec.Result().StatusCode != http.StatusRequestEntityTooLarge {
		t.Fatalf("want 413, got %d", rec.Result().StatusCode)
	}
}
