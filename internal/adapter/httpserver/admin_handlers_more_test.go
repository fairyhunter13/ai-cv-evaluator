package httpserver_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi/v5"

	httpserver "github.com/fairyhunter13/ai-cv-evaluator/internal/adapter/httpserver"
	"github.com/fairyhunter13/ai-cv-evaluator/internal/config"
	"github.com/fairyhunter13/ai-cv-evaluator/internal/usecase"
)

func newAdminSrv(t *testing.T) (*httpserver.AdminServer, *chi.Mux) {
	t.Helper()
	srv := httpserver.NewServer(config.Config{
		AppEnv:             "dev",
		Port:               8080,
		AdminUsername:      "admin",
		AdminPassword:      "secret",
		AdminSessionSecret: "abcd",
	}, usecase.NewUploadService(nil), usecase.NewEvaluateService(nil, nil, nil), usecase.NewResultService(nil, nil), nil, nil, nil, nil)
	admin, err := httpserver.NewAdminServer(srv.Cfg, srv)
	if err != nil {
		t.Fatalf("new admin: %v", err)
	}
	r := chi.NewRouter()
	// Mount API routes
	r.Post("/admin/token", admin.AdminTokenHandler())
	r.Get("/admin/api/status", admin.AdminStatusHandler())
	return admin, r
}

func loginAndGetToken(t *testing.T, r *chi.Mux) string {
	t.Helper()
	rw := httptest.NewRecorder()
	req := httptest.NewRequest("POST", "/admin/token", nil)
	req.Form = map[string][]string{"username": {"admin"}, "password": {"secret"}}
	r.ServeHTTP(rw, req)
	if rw.Result().StatusCode != http.StatusOK {
		t.Fatalf("login status: %d", rw.Result().StatusCode)
	}
	var body struct {
		Token string `json:"token"`
	}
	if err := json.Unmarshal(rw.Body.Bytes(), &body); err != nil {
		t.Fatalf("parse token response: %v", err)
	}
	if body.Token == "" {
		t.Fatalf("empty token in response")
	}
	return body.Token
}

func Test_Admin_API_Endpoints(t *testing.T) {
	_, r := newAdminSrv(t)
	token := loginAndGetToken(t, r)

	// GET /admin/api/status (protected endpoint, JWT bearer auth)
	rw := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/admin/api/status", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	r.ServeHTTP(rw, req)
	if rw.Result().StatusCode != http.StatusOK {
		t.Fatalf("/admin/api/status: %d", rw.Result().StatusCode)
	}
}

func Test_AdminJobsHandler_Unauthorized(t *testing.T) {
	admin, r := newAdminSrv(t)
	r.Get("/admin/api/jobs", admin.AdminJobsHandler())

	// No auth header
	rw := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/admin/api/jobs", nil)
	r.ServeHTTP(rw, req)
	if rw.Result().StatusCode != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", rw.Result().StatusCode)
	}

	// Invalid bearer token
	rw = httptest.NewRecorder()
	req = httptest.NewRequest("GET", "/admin/api/jobs", nil)
	req.Header.Set("Authorization", "Bearer invalid-token")
	r.ServeHTTP(rw, req)
	if rw.Result().StatusCode != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", rw.Result().StatusCode)
	}
}

func Test_AdminJobsHandler_InvalidPagination(t *testing.T) {
	admin, r := newAdminSrv(t)
	r.Get("/admin/api/jobs", admin.AdminJobsHandler())
	token := loginAndGetToken(t, r)

	// Invalid page
	rw := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/admin/api/jobs?page=-1", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	r.ServeHTTP(rw, req)
	if rw.Result().StatusCode != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", rw.Result().StatusCode)
	}

	// Invalid limit
	rw = httptest.NewRecorder()
	req = httptest.NewRequest("GET", "/admin/api/jobs?limit=abc", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	r.ServeHTTP(rw, req)
	if rw.Result().StatusCode != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", rw.Result().StatusCode)
	}
}

func Test_AdminJobsHandler_InvalidStatus(t *testing.T) {
	admin, r := newAdminSrv(t)
	r.Get("/admin/api/jobs", admin.AdminJobsHandler())
	token := loginAndGetToken(t, r)

	// Invalid status - should return 400 before hitting the repo
	rw := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/admin/api/jobs?status=invalid-status", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	r.ServeHTTP(rw, req)
	if rw.Result().StatusCode != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", rw.Result().StatusCode)
	}
}
