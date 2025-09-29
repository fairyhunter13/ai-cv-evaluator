package httpserver_test

import (
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
	}, usecase.NewUploadService(nil), usecase.NewEvaluateService(nil, nil, nil), usecase.NewResultService(nil, nil), nil, nil, nil, nil, nil)
	admin, err := httpserver.NewAdminServer(srv.Cfg, srv)
	if err != nil {
		t.Fatalf("new admin: %v", err)
	}
	r := chi.NewRouter()
	admin.MountRoutes(r)
	return admin, r
}

func loginAndGetCookies(t *testing.T, r *chi.Mux) []*http.Cookie {
	t.Helper()
	rw := httptest.NewRecorder()
	req := httptest.NewRequest("POST", "/admin/login", nil)
	req.Form = map[string][]string{"username": {"admin"}, "password": {"secret"}}
	r.ServeHTTP(rw, req)
	if rw.Result().StatusCode != http.StatusSeeOther {
		t.Fatalf("login status: %d", rw.Result().StatusCode)
	}
	return rw.Result().Cookies()
}

func Test_Admin_Protected_Pages(t *testing.T) {
	_, r := newAdminSrv(t)
	cookies := loginAndGetCookies(t, r)

	// GET /admin/upload
	rw1 := httptest.NewRecorder()
	req1 := httptest.NewRequest("GET", "/admin/upload", nil)
	for _, c := range cookies { req1.AddCookie(c) }
	r.ServeHTTP(rw1, req1)
	if rw1.Result().StatusCode != http.StatusOK { t.Fatalf("/admin/upload: %d", rw1.Result().StatusCode) }

	// GET /admin/evaluate
	rw2 := httptest.NewRecorder()
	req2 := httptest.NewRequest("GET", "/admin/evaluate", nil)
	for _, c := range cookies { req2.AddCookie(c) }
	r.ServeHTTP(rw2, req2)
	if rw2.Result().StatusCode != http.StatusOK { t.Fatalf("/admin/evaluate: %d", rw2.Result().StatusCode) }

	// GET /admin/result
	rw3 := httptest.NewRecorder()
	req3 := httptest.NewRequest("GET", "/admin/result", nil)
	for _, c := range cookies { req3.AddCookie(c) }
	r.ServeHTTP(rw3, req3)
	if rw3.Result().StatusCode != http.StatusOK { t.Fatalf("/admin/result: %d", rw3.Result().StatusCode) }
}
