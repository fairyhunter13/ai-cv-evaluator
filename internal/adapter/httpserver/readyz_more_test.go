package httpserver_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	httpserver "github.com/fairyhunter13/ai-cv-evaluator/internal/adapter/httpserver"
	"github.com/fairyhunter13/ai-cv-evaluator/internal/config"
	"github.com/fairyhunter13/ai-cv-evaluator/internal/usecase"
)

func TestReadyzHandler_AllFail(t *testing.T) {
	cfg := config.Config{Port: 8080}
	s := httpserver.NewServer(cfg, usecase.NewUploadService(nil), usecase.NewEvaluateService(nil, nil, nil), usecase.NewResultService(nil, nil),
		nil,
		func(_ context.Context) error { return http.ErrServerClosed }, // db
		func(_ context.Context) error { return http.ErrServerClosed }, // qdrant
		func(_ context.Context) error { return http.ErrServerClosed }, // tika
	)
	rw := httptest.NewRecorder()
	s.ReadyzHandler()(rw, httptest.NewRequest("GET", "/readyz", nil))
	if rw.Result().StatusCode != http.StatusServiceUnavailable {
		t.Fatalf("want 503, got %d", rw.Result().StatusCode)
	}
}
