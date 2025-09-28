package httpserver_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	httpserver "github.com/fairyhunter13/ai-cv-evaluator/internal/adapter/httpserver"
	"github.com/fairyhunter13/ai-cv-evaluator/internal/config"
	"github.com/fairyhunter13/ai-cv-evaluator/internal/usecase"
)

func TestResultHandler_406_NotAcceptable(t *testing.T) {
	cfg := config.Config{Port: 8080}
	upSvc := usecase.NewUploadService(nil)
	evSvc := usecase.NewEvaluateService(nil, nil, nil)
	resSvc := usecase.NewResultService(nil, nil)
	s := httpserver.NewServer(cfg, upSvc, evSvc, resSvc, nil, nil, nil, nil, nil)
	rw := httptest.NewRecorder()
	r := httptest.NewRequest("GET", "/v1/result/job1", nil)
	r.Header.Set("Accept", "text/plain")
	h := s.ResultHandler()
	h(rw, r)
	if rw.Result().StatusCode != http.StatusNotAcceptable { t.Fatalf("want 406, got %d", rw.Result().StatusCode) }
}

func TestResultHandler_400_MissingID(t *testing.T) {
	cfg := config.Config{Port: 8080}
	s := httpserver.NewServer(cfg, usecase.NewUploadService(nil), usecase.NewEvaluateService(nil, nil, nil), usecase.NewResultService(nil, nil), nil, nil, nil, nil, nil)
	rw := httptest.NewRecorder()
	r := httptest.NewRequest("GET", "/v1/result/", nil)
	r.Header.Set("Accept", "application/json")
	s.ResultHandler()(rw, r) // direct handler without chi URLParam set
	if rw.Result().StatusCode != http.StatusBadRequest { t.Fatalf("want 400, got %d", rw.Result().StatusCode) }
}

func TestAccessLog_EmitsAndPassesThrough(t *testing.T) {
	rec := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/x", nil)
	h := httpserver.AccessLog()(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(204) }))
	h.ServeHTTP(rec, r)
	if rec.Result().StatusCode != 204 { t.Fatalf("want 204") }
}

func TestLoggerFrom_WithRequestID(t *testing.T) {
	rec := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/x", nil)
	h := httpserver.RequestID()(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		lg := httpserver.LoggerFrom(r)
		if lg == nil { t.Fatalf("logger nil") }
		w.WriteHeader(204)
	}))
	h.ServeHTTP(rec, r)
	if rec.Result().StatusCode != 204 { t.Fatalf("want 204") }
}
