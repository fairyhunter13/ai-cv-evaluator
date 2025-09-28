package httpserver_test

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	httpserver "github.com/fairyhunter13/ai-cv-evaluator/internal/adapter/httpserver"
	"github.com/fairyhunter13/ai-cv-evaluator/internal/config"
	"github.com/fairyhunter13/ai-cv-evaluator/internal/domain"
	"github.com/fairyhunter13/ai-cv-evaluator/internal/usecase"
)

type idemJobRepo struct{ found domain.Job }
func (n *idemJobRepo) Create(_ domain.Context, _ domain.Job) (string, error) { return "job-new", nil }
func (n *idemJobRepo) UpdateStatus(_ domain.Context, _ string, _ domain.JobStatus, _ *string) error { return nil }
func (n *idemJobRepo) Get(_ domain.Context, _ string) (domain.Job, error) { return n.found, nil }
func (n *idemJobRepo) FindByIdempotencyKey(_ domain.Context, _ string) (domain.Job, error) { if n.found.ID!="" { return n.found, nil }; return domain.Job{}, domain.ErrNotFound }

type noopQueueIdem struct{}
func (q *noopQueueIdem) EnqueueEvaluate(_ domain.Context, _ domain.EvaluateTaskPayload) (string, error) { return "t-1", nil }

type stubUploadRepoIdemHTTP struct{}
func (s *stubUploadRepoIdemHTTP) Create(_ domain.Context, u domain.Upload) (string, error) { if u.Type==domain.UploadTypeCV {return "cv", nil}; return "pr", nil }
func (s *stubUploadRepoIdemHTTP) Get(_ domain.Context, id string) (domain.Upload, error) { return domain.Upload{ID:id}, nil }

func TestEvaluateHandler_Idempotent_ReturnsExisting(t *testing.T) {
	cfg := config.Config{Port:8080}
	jr := &idemJobRepo{ found: domain.Job{ID: "existing"} }
	evSvc := usecase.NewEvaluateService(jr, &noopQueueIdem{}, &stubUploadRepoIdemHTTP{})
	s := httpserver.NewServer(cfg, usecase.NewUploadService(&stubUploadRepoIdemHTTP{}), evSvc, usecase.NewResultService(jr, nil), nil, nil, nil, nil, nil)
	payload := map[string]any{"cv_id":"cv","project_id":"pr","job_description":"jd","study_case_brief":"brief"}
	b,_ := json.Marshal(payload)
	r := httptest.NewRequest(http.MethodPost, "/v1/evaluate", bytes.NewReader(b))
	r.Header.Set("Content-Type","application/json")
	r.Header.Set("Accept","application/json")
	r.Header.Set("Idempotency-Key","idem-1")
	rw := httptest.NewRecorder()
	s.EvaluateHandler()(rw, r)
	if rw.Result().StatusCode != http.StatusOK { t.Fatalf("want 200, got %d", rw.Result().StatusCode) }
}
