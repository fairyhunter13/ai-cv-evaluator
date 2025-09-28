package httpserver_test

import (
	"bytes"
	"encoding/json"
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/require"

	httpserver "github.com/fairyhunter13/ai-cv-evaluator/internal/adapter/httpserver"
	"github.com/fairyhunter13/ai-cv-evaluator/internal/config"
	"github.com/fairyhunter13/ai-cv-evaluator/internal/usecase"
)

func TestEvaluateHandler_200_OK(t *testing.T) {
	srv := newTestServer(t)
	payload := map[string]any{
		"cv_id": "cv-1", "project_id": "pr-1", "job_description": "jd", "study_case_brief": "brief",
	}
	b, _ := json.Marshal(payload)
	r := httptest.NewRequest(http.MethodPost, "/v1/evaluate", bytes.NewReader(b))
	r.Header.Set("Content-Type", "application/json")
	r.Header.Set("Accept", "application/json")
	w := httptest.NewRecorder()
	h := srv.EvaluateHandler()
	h(w, r)
	resp := w.Result()
	require.Equal(t, http.StatusOK, resp.StatusCode)
	_ = resp.Body.Close()
}

func TestReadyzHandler_OK_And_Unavailable(t *testing.T) {
	cfg := config.Config{Port: 8080}
	upSvc := usecase.NewUploadService(nil)
	evSvc := usecase.NewEvaluateService(nil, nil, nil)
	resSvc := usecase.NewResultService(nil, nil)
	s := httpserver.NewServer(cfg, upSvc, evSvc, resSvc,
		nil,
		func(_ context.Context) error { return nil },
		func(_ context.Context) error { return nil },
		func(_ context.Context) error { return nil },
		func(_ context.Context) error { return nil },
	)
	w := httptest.NewRecorder()
	s.ReadyzHandler()(w, httptest.NewRequest("GET", "/readyz", nil))
	if w.Result().StatusCode != http.StatusOK { t.Fatalf("want 200, got %d", w.Result().StatusCode) }

	// now make one check fail
	s = httpserver.NewServer(cfg, upSvc, evSvc, resSvc,
		nil,
		func(_ context.Context) error { return nil },
		func(_ context.Context) error { return nil },
		func(_ context.Context) error { return nil },
		func(_ context.Context) error { return http.ErrHandlerTimeout },
	)
	w2 := httptest.NewRecorder()
	s.ReadyzHandler()(w2, httptest.NewRequest("GET", "/readyz", nil))
	if w2.Result().StatusCode != http.StatusServiceUnavailable { t.Fatalf("want 503, got %d", w2.Result().StatusCode) }
}
