package httpserver_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/stretchr/testify/require"

	httpserver "github.com/fairyhunter13/ai-cv-evaluator/internal/adapter/httpserver"
	"github.com/fairyhunter13/ai-cv-evaluator/internal/config"
	"github.com/fairyhunter13/ai-cv-evaluator/internal/domain"
	"github.com/fairyhunter13/ai-cv-evaluator/internal/usecase"
)

// reuse helpers from handlers_result_test.go

func newAdminServerWithJob(t *testing.T, job domain.Job, res domain.Result) *httpserver.AdminServer {
	t.Helper()

	// Build a server wired with job and result repositories
	cfgServer := config.Config{Port: 8080, AppEnv: "dev"}
	jobRepo := createMockJobRepoRes(t, job)
	resultRepo := createMockResultRepoRes(t, res)
	upSvc := usecase.NewUploadService(nil)
	evSvc := usecase.NewEvaluateService(jobRepo, nil, nil)
	resSvc := usecase.NewResultService(jobRepo, resultRepo)
	srv := httpserver.NewServer(cfgServer, upSvc, evSvc, resSvc, nil, nil, nil, nil)

	// Admin server config with credentials and secret for JWT
	cfgAdmin := config.Config{AdminUsername: "admin", AdminPassword: "password", AdminSessionSecret: "secret"}
	admin, err := httpserver.NewAdminServer(cfgAdmin, srv)
	require.NoError(t, err)
	return admin
}

func getAdminToken(t *testing.T, admin *httpserver.AdminServer) string {
	t.Helper()
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/admin/token", nil)
	req.Form = map[string][]string{
		"username": {"admin"},
		"password": {"password"},
	}

	admin.AdminTokenHandler()(rec, req)

	resp := rec.Result()
	require.Equal(t, http.StatusOK, resp.StatusCode)
	defer resp.Body.Close() //nolint:errcheck

	var body map[string]any
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &body))
	return body["token"].(string)
}

func TestAdminJobDetailsHandler_Authorized_Success(t *testing.T) {
	job := domain.Job{ID: "job1", Status: domain.JobCompleted}
	res := domain.Result{JobID: "job1", CVMatchRate: 0.9}
	admin := newAdminServerWithJob(t, job, res)

	tok := getAdminToken(t, admin)

	r := chi.NewRouter()
	r.Get("/admin/api/jobs/{id}", admin.AdminJobDetailsHandler())

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/admin/api/jobs/job1", nil)
	req.Header.Set("Authorization", "Bearer "+tok)

	r.ServeHTTP(rec, req)

	resp := rec.Result()
	require.Equal(t, http.StatusOK, resp.StatusCode)
	defer resp.Body.Close() //nolint:errcheck

	var body map[string]any
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&body))
	require.Equal(t, "job1", body["id"])
}

func TestAdminJobDetailsHandler_InvalidID(t *testing.T) {
	job := domain.Job{ID: "job1", Status: domain.JobCompleted}
	res := domain.Result{JobID: "job1"}
	admin := newAdminServerWithJob(t, job, res)

	tok := getAdminToken(t, admin)

	r := chi.NewRouter()
	r.Get("/admin/api/jobs/{id}", admin.AdminJobDetailsHandler())

	rec := httptest.NewRecorder()
	// Use an invalid job ID (only special characters) that will be stripped to empty
	// by SanitizeJobID and then fail ValidateJobID
	req := httptest.NewRequest(http.MethodGet, "/admin/api/jobs/$$$$", nil)
	req.Header.Set("Authorization", "Bearer "+tok)

	r.ServeHTTP(rec, req)

	require.Equal(t, http.StatusBadRequest, rec.Result().StatusCode)
}

func TestAdminJobDetailsHandler_Unauthorized(t *testing.T) {
	job := domain.Job{ID: "job1", Status: domain.JobCompleted}
	res := domain.Result{JobID: "job1"}
	admin := newAdminServerWithJob(t, job, res)

	r := chi.NewRouter()
	r.Get("/admin/api/jobs/{id}", admin.AdminJobDetailsHandler())

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/admin/api/jobs/job1", nil)

	r.ServeHTTP(rec, req)

	require.Equal(t, http.StatusUnauthorized, rec.Result().StatusCode)
}

func TestAdminAuthRequired_DelegatesToSessionManager(t *testing.T) {
	cfgServer := config.Config{Port: 8080}
	srv := httpserver.NewServer(cfgServer, usecase.NewUploadService(nil), usecase.NewEvaluateService(nil, nil, nil), usecase.NewResultService(nil, nil), nil, nil, nil, nil)
	cfgAdmin := config.Config{AdminSessionSecret: "secret"}
	admin, err := httpserver.NewAdminServer(cfgAdmin, srv)
	require.NoError(t, err)

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/admin/api/protected", nil)

	called := false
	h := admin.AdminAuthRequired(func(w http.ResponseWriter, _ *http.Request) {
		called = true
		w.WriteHeader(http.StatusOK)
	})

	h(rec, req)

	require.True(t, called)
	require.Equal(t, http.StatusOK, rec.Result().StatusCode)
}
