package httpserver_test

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/require"

	httpserver "github.com/fairyhunter13/ai-cv-evaluator/internal/adapter/httpserver"
	"github.com/fairyhunter13/ai-cv-evaluator/internal/config"
	"github.com/fairyhunter13/ai-cv-evaluator/internal/usecase"
)

func newHealthTestServer() *httpserver.Server {
	cfg := config.Config{Port: 8080}
	upSvc := usecase.NewUploadService(nil)
	evSvc := usecase.NewEvaluateService(nil, nil, nil)
	resSvc := usecase.NewResultService(nil, nil)
	return httpserver.NewServer(cfg, upSvc, evSvc, resSvc, nil, nil, nil, nil)
}

func TestHealthzHandler_AllHealthy(t *testing.T) {
	s := newHealthTestServer()
	s.DBCheck = func(_ context.Context) error { return nil }
	s.QdrantCheck = func(_ context.Context) error { return nil }
	s.TikaCheck = func(_ context.Context) error { return nil }

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/healthz", nil)

	h := s.HealthzHandler()
	h(rec, req)

	resp := rec.Result()
	require.Equal(t, http.StatusOK, resp.StatusCode)
	defer resp.Body.Close() //nolint:errcheck

	var body map[string]any
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&body))
	require.Equal(t, "healthy", body["status"])
	checks, ok := body["checks"].([]any)
	require.True(t, ok)
	require.NotEmpty(t, checks)
}

func TestHealthzHandler_WithDBFailure_Unhealthy(t *testing.T) {
	s := newHealthTestServer()
	s.DBCheck = func(_ context.Context) error { return errors.New("db down") }
	s.QdrantCheck = func(_ context.Context) error { return nil }
	s.TikaCheck = func(_ context.Context) error { return nil }

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/healthz", nil)

	h := s.HealthzHandler()
	h(rec, req)

	resp := rec.Result()
	require.Equal(t, http.StatusServiceUnavailable, resp.StatusCode)
	defer resp.Body.Close() //nolint:errcheck

	var body map[string]any
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&body))
	require.Equal(t, "unhealthy", body["status"])
}

func TestMetricsHandler_ReturnsHealthStatus(t *testing.T) {
	s := newHealthTestServer()

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/metrics", nil)

	h := s.MetricsHandler()
	h(rec, req)

	resp := rec.Result()
	require.Equal(t, http.StatusOK, resp.StatusCode)
	defer resp.Body.Close() //nolint:errcheck

	var body map[string]any
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&body))
	require.NotEmpty(t, body["timestamp"])
	require.Equal(t, "1.0.0", body["version"])
	require.NotNil(t, body["health_check"])
}
