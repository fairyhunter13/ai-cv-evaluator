package usecase_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/fairyhunter13/ai-cv-evaluator/internal/domain"
	"github.com/fairyhunter13/ai-cv-evaluator/internal/usecase"
)

type stubJobRepoIdem struct { found domain.Job }
func (s *stubJobRepoIdem) Create(_ domain.Context, _ domain.Job) (string, error) { return "created", nil }
func (s *stubJobRepoIdem) UpdateStatus(_ domain.Context, _ string, _ domain.JobStatus, _ *string) error { return nil }
func (s *stubJobRepoIdem) Get(_ domain.Context, id string) (domain.Job, error) { return domain.Job{ID:id}, nil }
func (s *stubJobRepoIdem) FindByIdempotencyKey(_ domain.Context, _ string) (domain.Job, error) {
	if s.found.ID != "" { return s.found, nil }
	return domain.Job{}, domain.ErrNotFound
}

type stubQueueIdem struct { called bool }
func (q *stubQueueIdem) EnqueueEvaluate(_ domain.Context, _ domain.EvaluateTaskPayload) (string, error) { q.called = true; return "ok", nil }

type stubUploadRepoIdem struct{}
func (r *stubUploadRepoIdem) Create(_ domain.Context, _ domain.Upload) (string, error) { return "u", nil }
func (r *stubUploadRepoIdem) Get(_ domain.Context, id string) (domain.Upload, error) { return domain.Upload{ID:id}, nil }

func TestEvaluate_Idempotency_ReturnsExistingJob(t *testing.T) {
	jr := &stubJobRepoIdem{ found: domain.Job{ID: "existing"} }
	q := &stubQueueIdem{}
	u := &stubUploadRepoIdem{}
	svc := usecase.NewEvaluateService(jr, q, u)
	jobID, err := svc.Enqueue(context.Background(), "cv1", "pr1", "", "", "idem-1")
	require.NoError(t, err)
	assert.Equal(t, "existing", jobID)
	assert.False(t, q.called, "enqueue should not be called when idempotent job exists")
}
