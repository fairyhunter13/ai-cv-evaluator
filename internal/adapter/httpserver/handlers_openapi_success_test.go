package httpserver_test

import (
	"net/http/httptest"
	"os"
	"testing"

	httpserver "github.com/fairyhunter13/ai-cv-evaluator/internal/adapter/httpserver"
	"github.com/fairyhunter13/ai-cv-evaluator/internal/config"
	"github.com/fairyhunter13/ai-cv-evaluator/internal/usecase"
	"github.com/stretchr/testify/require"
)

func Test_OpenAPIServe_200_WhenPresent(t *testing.T) {
	require.NoError(t, os.MkdirAll("api", 0o750))
	require.NoError(t, os.WriteFile("api/openapi.yaml", []byte("openapi: 3.0.0\n"), 0o600))
	cfg := config.Config{Port: 8080}
	s := httpserver.NewServer(cfg, usecase.NewUploadService(nil), usecase.NewEvaluateService(nil, nil, nil), usecase.NewResultService(nil, nil), nil, nil, nil, nil)
	rw := httptest.NewRecorder()
	s.OpenAPIServe()(rw, httptest.NewRequest("GET", "/openapi.yaml", nil))
	if rw.Result().StatusCode != 200 {
		t.Fatalf("want 200, got %d", rw.Result().StatusCode)
	}
}
