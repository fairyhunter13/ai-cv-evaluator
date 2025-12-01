package redpanda

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/fairyhunter13/ai-cv-evaluator/internal/domain"
)

// stubAIForHandle is a minimal AIClient stub that always returns a valid result JSON.
type stubAIForHandle struct{}

func (s *stubAIForHandle) Embed(domain.Context, []string) ([][]float32, error) {
	return [][]float32{{0.1, 0.2, 0.3}}, nil
}

func (s *stubAIForHandle) ChatJSON(domain.Context, string, string, int) (string, error) {
	return `{"ok":true}`, nil
}

func (s *stubAIForHandle) ChatJSONWithRetry(domain.Context, string, string, int) (string, error) {
	// Return a refined evaluation-like JSON payload that validateAndFinalizeResults accepts.
	return `{"cv_match_rate":0.8,"cv_feedback":"good","project_score":8.5,"project_feedback":"solid","overall_summary":"ok"}`, nil
}

func (s *stubAIForHandle) CleanCoTResponse(domain.Context, string) (string, error) {
	return `{"cv_match_rate":0.8,"cv_feedback":"good","project_score":8.5,"project_feedback":"solid","overall_summary":"ok"}`, nil
}

// fakeUploadRepo is a small UploadRepository implementation for HandleEvaluate tests.
type fakeUploadRepo struct {
	uploads map[string]domain.Upload
}

func (r *fakeUploadRepo) Create(domain.Context, domain.Upload) (string, error) { return "", nil }
func (r *fakeUploadRepo) Count(domain.Context) (int64, error)                  { return int64(len(r.uploads)), nil }
func (r *fakeUploadRepo) CountByType(domain.Context, string) (int64, error) {
	return int64(len(r.uploads)), nil
}

func (r *fakeUploadRepo) Get(_ domain.Context, id string) (domain.Upload, error) {
	if r.uploads == nil {
		return domain.Upload{}, domain.ErrNotFound
	}
	u, ok := r.uploads[id]
	if !ok {
		return domain.Upload{}, domain.ErrNotFound
	}
	return u, nil
}

// fakeResultRepo is a small ResultRepository implementation for HandleEvaluate tests.
type fakeResultRepo struct {
	stored map[string]domain.Result
}

func (r *fakeResultRepo) Upsert(_ domain.Context, res domain.Result) error {
	if r.stored == nil {
		r.stored = make(map[string]domain.Result)
	}
	r.stored[res.JobID] = res
	return nil
}

func (r *fakeResultRepo) GetByJobID(_ domain.Context, jobID string) (domain.Result, error) {
	if r.stored == nil {
		return domain.Result{}, domain.ErrNotFound
	}
	res, ok := r.stored[jobID]
	if !ok {
		return domain.Result{}, domain.ErrNotFound
	}
	return res, nil
}

func TestHandleEvaluate_SuccessPath_StoresResultAndCompletesJob(t *testing.T) {
	ctx := context.Background()

	jobs := &fakeJobRepo{jobs: map[string]domain.Job{
		"job-1": {ID: "job-1", Status: domain.JobQueued},
	},
	}
	uploads := &fakeUploadRepo{uploads: map[string]domain.Upload{
		"cv-1":      {ID: "cv-1", Type: domain.UploadTypeCV, Text: "cv text"},
		"project-1": {ID: "project-1", Type: domain.UploadTypeProject, Text: "project text"},
	},
	}
	results := &fakeResultRepo{}
	ai := &stubAIForHandle{}

	payload := domain.EvaluateTaskPayload{
		JobID:          "job-1",
		CVID:           "cv-1",
		ProjectID:      "project-1",
		JobDescription: "job desc",
		StudyCaseBrief: "study",
		ScoringRubric:  "rubric",
	}

	require.NoError(t, HandleEvaluate(ctx, jobs, uploads, results, ai, nil, payload))

	// Result should be stored for the job
	require.NotNil(t, results.stored)
	res, ok := results.stored["job-1"]
	require.True(t, ok)
	require.Equal(t, "job-1", res.JobID)

	// Job status should transition to completed
	job, err := jobs.Get(ctx, "job-1")
	require.NoError(t, err)
	require.Equal(t, domain.JobCompleted, job.Status)
}
