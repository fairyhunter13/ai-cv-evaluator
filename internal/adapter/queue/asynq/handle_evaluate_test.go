package asynqadp

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	qdrantcli "github.com/fairyhunter13/ai-cv-evaluator/internal/adapter/vector/qdrant"
	"github.com/fairyhunter13/ai-cv-evaluator/internal/domain"
)

type heJobs struct{ statuses []domain.JobStatus }
func (h *heJobs) Create(_ domain.Context, _ domain.Job) (string, error) { return "j", nil }
func (h *heJobs) UpdateStatus(_ domain.Context, _ string, status domain.JobStatus, _ *string) error { h.statuses = append(h.statuses, status); return nil }
func (h *heJobs) Get(_ domain.Context, id string) (domain.Job, error) { return domain.Job{ID:id}, nil }
func (h *heJobs) FindByIdempotencyKey(_ domain.Context, _ string) (domain.Job, error) { return domain.Job{}, domain.ErrNotFound }

type heUploads struct{}
func (s heUploads) Create(_ domain.Context, _ domain.Upload) (string, error) { return "id", nil }
func (s heUploads) Get(_ domain.Context, id string) (domain.Upload, error) { return domain.Upload{ID:id, Text: "T", Filename: "f.txt"}, nil }

type heResults struct{ last domain.Result }
func (s *heResults) Upsert(_ domain.Context, r domain.Result) error { s.last = r; return nil }
func (s *heResults) GetByJobID(_ domain.Context, _ string) (domain.Result, error) { return s.last, nil }

type heAI struct{}
func (h heAI) Embed(_ domain.Context, _ []string) ([][]float32, error) { return [][]float32{{1,2,3},{4,5,6}}, nil }
func (h heAI) ChatJSON(_ domain.Context, systemPrompt, _ string, _ int) (string, error) {
	if systemPrompt == buildCVExtractSystemPrompt() {
		return `{"skills":["go"],"experiences":[],"projects":[],"summary":"ok."}`, nil
	}
	if systemPrompt == buildProjectExtractSystemPrompt() {
		return `{"requirements":["r1"],"architecture":[],"strengths":[],"risks":[],"summary":"ok"}`, nil
	}
	if systemPrompt == buildSystemPrompt() || systemPrompt == buildNormalizationSystemPrompt() {
		return `{"cv_match_rate":0.5,"cv_feedback":"ok.","project_score":5,"project_feedback":"ok.","overall_summary":"one. two. three."}`, nil
	}
	return `{"cv_match_rate":0.5,"cv_feedback":"ok.","project_score":5,"project_feedback":"ok.","overall_summary":"one. two. three."}`, nil
}

type heAIErr struct{}
func (heAIErr) Embed(_ domain.Context, _ []string) ([][]float32, error) { return [][]float32{{1,2,3}}, nil }
func (heAIErr) ChatJSON(_ domain.Context, _ string, _ string, _ int) (string, error) { return "", context.DeadlineExceeded }

func Test_handleEvaluate_Success_NoRAG(t *testing.T) {
	j := &heJobs{}
	u := heUploads{}
	r := &heResults{}
	ai := heAI{}
	p := domain.EvaluateTaskPayload{JobID: "j1", CVID: "c", ProjectID: "p", JobDescription: "jd", StudyCaseBrief: "br"}
	if err := handleEvaluate(context.Background(), j, u, r, ai, nil, p, false, false); err != nil { t.Fatalf("err: %v", err) }
}

func Test_handleEvaluate_TwoPass(t *testing.T) {
	j := &heJobs{}
	u := heUploads{}
	r := &heResults{}
	ai := heAI{}
	p := domain.EvaluateTaskPayload{JobID: "j2", CVID: "c", ProjectID: "p", JobDescription: "jd", StudyCaseBrief: "br"}
	if err := handleEvaluate(context.Background(), j, u, r, ai, nil, p, true, false); err != nil { t.Fatalf("err: %v", err) }
}

func Test_handleEvaluate_ChainWithRAG(t *testing.T) {
	// Minimal Qdrant search responder
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/collections/job_description/points/search" || r.URL.Path == "/collections/scoring_rubric/points/search" {
			_ = json.NewEncoder(w).Encode(map[string]any{"result": []map[string]any{{"payload": map[string]any{"text": "ctx1", "weight": 1.0}}}})
			return
		}
		w.WriteHeader(200)
	}))
	defer ts.Close()
	q := qdrantcli.New(ts.URL, "")
	j := &heJobs{}
	u := heUploads{}
	r := &heResults{}
	ai := heAI{}
	p := domain.EvaluateTaskPayload{JobID: "j3", CVID: "c", ProjectID: "p", JobDescription: "jd", StudyCaseBrief: "br"}
	if err := handleEvaluate(context.Background(), j, u, r, ai, q, p, false, true); err != nil { t.Fatalf("err: %v", err) }
}

func Test_handleEvaluate_ChatError_Fails(t *testing.T) {
	j := &heJobs{}
	u := heUploads{}
	r := &heResults{}
	ai := heAIErr{}
	p := domain.EvaluateTaskPayload{JobID: "j4", CVID: "c", ProjectID: "p", JobDescription: "jd", StudyCaseBrief: "br"}
	if err := handleEvaluate(context.Background(), j, u, r, ai, nil, p, false, false); err == nil { t.Fatalf("expected error") }
}
