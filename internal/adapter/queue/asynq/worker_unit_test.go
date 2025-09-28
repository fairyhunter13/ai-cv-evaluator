package asynqadp

import (
	"testing"

	"github.com/fairyhunter13/ai-cv-evaluator/internal/domain"
)

type stubJobs struct{}
func (s stubJobs) Create(_ domain.Context, _ domain.Job) (string, error) { return "j", nil }
func (s stubJobs) UpdateStatus(_ domain.Context, _ string, _ domain.JobStatus, _ *string) error { return nil }
func (s stubJobs) Get(_ domain.Context, id string) (domain.Job, error) { return domain.Job{ID: id}, nil }
func (s stubJobs) FindByIdempotencyKey(_ domain.Context, _ string) (domain.Job, error) { return domain.Job{}, nil }

type stubUploads struct{}
func (s stubUploads) Create(_ domain.Context, _ domain.Upload) (string, error) { return "id", nil }
func (s stubUploads) Get(_ domain.Context, id string) (domain.Upload, error) { return domain.Upload{ID: id, Text: "t"}, nil }

type stubResults struct{}
func (s stubResults) Upsert(_ domain.Context, _ domain.Result) error { return nil }
func (s stubResults) GetByJobID(_ domain.Context, jobID string) (domain.Result, error) { return domain.Result{JobID: jobID}, nil }

type stubAI struct{}
func (s stubAI) Embed(_ domain.Context, _ []string) ([][]float32, error) { return [][]float32{{1,2,3}}, nil }
func (s stubAI) ChatJSON(_ domain.Context, _ string, _ string, _ int) (string, error) { return `{"cv_match_rate":0.5,"cv_feedback":"ok.","project_score":5,"project_feedback":"ok.","overall_summary":"one. two. three."}`, nil }

func TestNewWorker_Basics(t *testing.T) {
	w, err := NewWorker("redis://localhost:6379/15", stubJobs{}, stubUploads{}, stubResults{}, stubAI{}, nil, false, false)
	if err != nil { t.Fatalf("new worker: %v", err) }
	if w == nil { t.Fatalf("worker nil") }
	w.Stop() // should not panic
}
