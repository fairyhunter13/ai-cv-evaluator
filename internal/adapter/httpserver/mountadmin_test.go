package httpserver_test

import (
	"testing"

	"github.com/go-chi/chi/v5"

	httpserver "github.com/fairyhunter13/ai-cv-evaluator/internal/adapter/httpserver"
	"github.com/fairyhunter13/ai-cv-evaluator/internal/config"
	"github.com/fairyhunter13/ai-cv-evaluator/internal/usecase"
)

func Test_MountAdmin_Disabled_NoPanic(t *testing.T) {
	t.Helper()
	cfg := config.Config{Port: 8080}
	s := httpserver.NewServer(cfg, usecase.NewUploadService(nil), usecase.NewEvaluateService(nil, nil, nil), usecase.NewResultService(nil, nil), nil, nil, nil, nil)
	r := chi.NewRouter()
	s.MountAdmin(r)
}

func Test_MountAdmin_Enabled_UsesStub_NoPanic(t *testing.T) {
	t.Helper()
	cfg := config.Config{Port: 8080, AdminUsername: "a", AdminPassword: "b", AdminSessionSecret: "c"}
	s := httpserver.NewServer(cfg, usecase.NewUploadService(nil), usecase.NewEvaluateService(nil, nil, nil), usecase.NewResultService(nil, nil), nil, nil, nil, nil)
	r := chi.NewRouter()
	s.MountAdmin(r)
}
