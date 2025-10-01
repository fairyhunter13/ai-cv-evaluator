package httpserver_test

import (
	"net/http/httptest"
	"os"
	"testing"

	httpserver "github.com/fairyhunter13/ai-cv-evaluator/internal/adapter/httpserver"
	"github.com/fairyhunter13/ai-cv-evaluator/internal/config"
	"github.com/fairyhunter13/ai-cv-evaluator/internal/usecase"
)

func Test_OpenAPIServe_404_WhenMissing(t *testing.T) {
	cfg := config.Config{Port: 8080}
	s := httpserver.NewServer(cfg, usecase.NewUploadService(nil), usecase.NewEvaluateService(nil, nil, nil), usecase.NewResultService(nil, nil), nil, nil, nil, nil)
	_ = os.RemoveAll("api/openapi.yaml")
	_ = os.RemoveAll("api")
	rw := httptest.NewRecorder()
	s.OpenAPIServe()(rw, httptest.NewRequest("GET", "/openapi.yaml", nil))
	if rw.Result().StatusCode != 404 {
		t.Fatalf("want 404, got %d", rw.Result().StatusCode)
	}
}
