package redpanda

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/fairyhunter13/ai-cv-evaluator/internal/domain"
)

func TestHandleEvaluate_NilRepositories_Error(t *testing.T) {
	ctx := context.Background()
	payload := domain.EvaluateTaskPayload{JobID: "job-1", CVID: "cv-1", ProjectID: "project-1"}

	err := HandleEvaluate(ctx, nil, nil, nil, nil, nil, payload)
	require.Error(t, err)
}

func TestHandleEvaluate_UploadsError_UpdatesJobFailed(t *testing.T) {
	ctx := context.Background()
	jobs := &fakeJobRepo{jobs: map[string]domain.Job{
		"job-1": {ID: "job-1", Status: domain.JobQueued},
	}}
	// Missing CV upload to trigger error path
	uploads := &fakeUploadRepo{uploads: map[string]domain.Upload{
		"project-1": {ID: "project-1", Type: domain.UploadTypeProject, Text: "project text"},
	}}
	results := &fakeResultRepo{}
	ai := &stubAIForHandle{}

	payload := domain.EvaluateTaskPayload{
		JobID:          "job-1",
		CVID:           "cv-1",
		ProjectID:      "project-1",
		JobDescription: "desc",
		StudyCaseBrief: "study",
		ScoringRubric:  "rubric",
	}

	err := HandleEvaluate(ctx, jobs, uploads, results, ai, nil, payload)
	require.Error(t, err)

	job, err := jobs.Get(ctx, "job-1")
	require.NoError(t, err)
	require.Equal(t, domain.JobFailed, job.Status)
}
