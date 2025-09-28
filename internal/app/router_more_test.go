package app_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	httpserver "github.com/fairyhunter13/ai-cv-evaluator/internal/adapter/httpserver"
	qdrantcli "github.com/fairyhunter13/ai-cv-evaluator/internal/adapter/vector/qdrant"
	"github.com/fairyhunter13/ai-cv-evaluator/internal/app"
	"github.com/fairyhunter13/ai-cv-evaluator/internal/config"
	"github.com/fairyhunter13/ai-cv-evaluator/internal/usecase"
)

func TestBuildRouter_Healthz_And_Readyz(t *testing.T) {
	cfg := config.Config{Port: 8080}
	upSvc := usecase.NewUploadService(nil)
	evSvc := usecase.NewEvaluateService(nil, nil, nil)
	resSvc := usecase.NewResultService(nil, nil)
	srv := httpserver.NewServer(cfg, upSvc, evSvc, resSvc,
		nil,
		func(_ context.Context) error { return nil },
		func(_ context.Context) error { return nil },
		func(_ context.Context) error { return nil },
		func(_ context.Context) error { return nil },
	)
	h := app.BuildRouter(cfg, srv)

	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/healthz", nil))
	if rec.Result().StatusCode != http.StatusOK {
		t.Fatalf("/healthz: want 200, got %d", rec.Result().StatusCode)
	}

	rec2 := httptest.NewRecorder()
	h.ServeHTTP(rec2, httptest.NewRequest(http.MethodGet, "/readyz", nil))
	if rec2.Result().StatusCode != http.StatusOK {
		t.Fatalf("/readyz: want 200, got %d", rec2.Result().StatusCode)
	}
}

func TestEnsureDefaultCollections_NoPanic(t *testing.T) {
	// Minimal Qdrant endpoints to satisfy EnsureCollection GET calls
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer ts.Close()
	q := qdrantcli.New(ts.URL, "")
	app.EnsureDefaultCollections(context.Background(), q, nil)
}
