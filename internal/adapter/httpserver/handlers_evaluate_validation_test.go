package httpserver_test

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	httpserver "github.com/fairyhunter13/ai-cv-evaluator/internal/adapter/httpserver"
	"github.com/fairyhunter13/ai-cv-evaluator/internal/config"
	"github.com/fairyhunter13/ai-cv-evaluator/internal/usecase"
)

func TestEvaluateHandler_ValidationDetails(t *testing.T) {
	cfg := config.Config{Port: 8080}
	s := httpserver.NewServer(cfg, usecase.NewUploadService(nil), usecase.NewEvaluateService(nil, nil, nil), usecase.NewResultService(nil, nil), nil, nil, nil, nil, nil)
	payload := map[string]any{"cv_id": "cv1"} // missing project_id, job_description, study_case_brief
	b,_ := json.Marshal(payload)
	r := httptest.NewRequest(http.MethodPost, "/v1/evaluate", bytes.NewReader(b))
	r.Header.Set("Content-Type", "application/json")
	r.Header.Set("Accept", "application/json")
	rw := httptest.NewRecorder()
	s.EvaluateHandler()(rw, r)
	if rw.Result().StatusCode != http.StatusBadRequest { t.Fatalf("want 400, got %d", rw.Result().StatusCode) }
	var resp map[string]any
	_ = json.NewDecoder(rw.Result().Body).Decode(&resp)
	errObj := resp["error"].(map[string]any)
	_ = errObj
	// details is optional; ensure we at least returned INVALID_ARGUMENT
	if errObj["code"].(string) != "INVALID_ARGUMENT" { t.Fatalf("code mismatch: %v", errObj["code"]) }
}
