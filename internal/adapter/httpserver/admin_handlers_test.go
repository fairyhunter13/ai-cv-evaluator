//go:build adminui

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

func newAdminEnabledServer() *httpserver.Server {
	cfg := config.Config{
		AppEnv:             "dev",
		Port:               8080,
		AdminUsername:      "admin",
		AdminPassword:      "secret",
		AdminSessionSecret: "abcd",
	}
	upSvc := usecase.NewUploadService(nil)
	evSvc := usecase.NewEvaluateService(nil, nil, nil)
	resSvc := usecase.NewResultService(nil, nil)
	return httpserver.NewServer(cfg, upSvc, evSvc, resSvc, nil, nil, nil, nil, nil)
}

func Test_Admin_Login_Flow(t *testing.T) {
	srv := newAdminEnabledServer()
	admin, err := httpserver.NewAdminServer(srv.Cfg, srv)
	if err != nil { t.Fatalf("new admin: %v", err) }
	r := chi.NewRouter()
	admin.MountRoutes(r)

	// GET /admin/login should be 200
	rw := httptest.NewRecorder()
	r.ServeHTTP(rw, httptest.NewRequest("GET", "/admin/login", nil))
	if rw.Result().StatusCode != http.StatusOK { t.Fatalf("login page status: %d", rw.Result().StatusCode) }

	// POST /admin/login wrong creds => 303
	rw2 := httptest.NewRecorder()
	req2 := httptest.NewRequest("POST", "/admin/login", nil)
	req2.Form = map[string][]string{"username": {"wrong"}, "password": {"creds"}}
	r.ServeHTTP(rw2, req2)
	if rw2.Result().StatusCode != http.StatusSeeOther { t.Fatalf("wrong creds status: %d", rw2.Result().StatusCode) }

	// POST /admin/login correct creds => 303 and sets cookie
	rw3 := httptest.NewRecorder()
	req3 := httptest.NewRequest("POST", "/admin/login", nil)
	req3.Form = map[string][]string{"username": {"admin"}, "password": {"secret"}}
	r.ServeHTTP(rw3, req3)
	if rw3.Result().StatusCode != http.StatusSeeOther { t.Fatalf("correct creds status: %d", rw3.Result().StatusCode) }
	cookies := rw3.Result().Cookies()
	if len(cookies) == 0 { t.Fatalf("expected session cookie") }

	// GET /admin/ with cookie => 200
	rw4 := httptest.NewRecorder()
	req4 := httptest.NewRequest("GET", "/admin/", nil)
	for _, c := range cookies { req4.AddCookie(c) }
	r.ServeHTTP(rw4, req4)
	if rw4.Result().StatusCode != http.StatusOK { t.Fatalf("dashboard status: %d", rw4.Result().StatusCode) }
}

func Test_Admin_Auth_Redirect_When_No_Session(t *testing.T) {
	srv := newAdminEnabledServer()
	admin, err := httpserver.NewAdminServer(srv.Cfg, srv)
	if err != nil { t.Fatalf("new admin: %v", err) }
	r := chi.NewRouter()
	admin.MountRoutes(r)

	rw := httptest.NewRecorder()
	r.ServeHTTP(rw, httptest.NewRequest("GET", "/admin/", nil))
	if rw.Result().StatusCode != http.StatusSeeOther { t.Fatalf("expected redirect for missing session, got %d", rw.Result().StatusCode) }
}
