package redpanda

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/twmb/franz-go/pkg/kgo"

	"github.com/fairyhunter13/ai-cv-evaluator/internal/domain"
)

func TestConsumer_ProcessRecord_Success(t *testing.T) {
	ctx := context.Background()

	jobs := &fakeJobRepo{jobs: map[string]domain.Job{
		"job-1": {ID: "job-1", Status: domain.JobQueued},
	}}
	uploads := &fakeUploadRepo{uploads: map[string]domain.Upload{
		"cv-1":      {ID: "cv-1", Type: domain.UploadTypeCV, Text: "cv text"},
		"project-1": {ID: "project-1", Type: domain.UploadTypeProject, Text: "project text"},
	}}
	results := &fakeResultRepo{}
	ai := &stubAIForHandle{}

	c := &Consumer{
		jobs:    jobs,
		uploads: uploads,
		results: results,
		ai:      ai,
	}

	payload := domain.EvaluateTaskPayload{
		JobID:          "job-1",
		CVID:           "cv-1",
		ProjectID:      "project-1",
		JobDescription: "desc",
		StudyCaseBrief: "study",
		ScoringRubric:  "rubric",
	}
	value, err := json.Marshal(payload)
	require.NoError(t, err)

	rec := &kgo.Record{
		Topic:     "evaluate-jobs",
		Partition: 0,
		Offset:    1,
		Key:       []byte("job-1"),
		Value:     value,
	}

	require.NoError(t, c.processRecord(ctx, rec))

	// Ensure a result was written and job completed
	require.NotNil(t, results.stored)
	res, ok := results.stored["job-1"]
	require.True(t, ok)
	require.Equal(t, "job-1", res.JobID)

	job, err := jobs.Get(ctx, "job-1")
	require.NoError(t, err)
	require.Equal(t, domain.JobCompleted, job.Status)
}
