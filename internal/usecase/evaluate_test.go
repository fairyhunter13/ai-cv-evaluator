package usecase_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/fairyhunter13/ai-cv-evaluator/internal/domain"
	"github.com/fairyhunter13/ai-cv-evaluator/internal/usecase"
)

type stubJobRepo struct {
	created domain.Job
	id      string
	status  domain.JobStatus
	errMsg  *string
	createErr error
}

func (s *stubJobRepo) Create(_ domain.Context, j domain.Job) (string, error) {
	s.created = j
	if s.createErr != nil { return "", s.createErr }
	if s.id == "" { s.id = "job-123" }
	return s.id, nil
}
func (s *stubJobRepo) UpdateStatus(_ domain.Context, _ string, status domain.JobStatus, errMsg *string) error {
	s.status, s.errMsg = status, errMsg
	return nil
}
func (s *stubJobRepo) Get(_ domain.Context, id string) (domain.Job, error) { return domain.Job{ID: id}, nil }
func (s *stubJobRepo) FindByIdempotencyKey(_ domain.Context, _ string) (domain.Job, error) { return domain.Job{}, domain.ErrNotFound }

type stubQueue struct { payload domain.EvaluateTaskPayload; err error }
func (q *stubQueue) EnqueueEvaluate(_ domain.Context, p domain.EvaluateTaskPayload) (string, error) { q.payload = p; return "t-1", q.err }

type stubUploadRepo struct{}
func (r *stubUploadRepo) Create(_ domain.Context, _ domain.Upload) (string, error) { return "", nil }
func (r *stubUploadRepo) Get(_ domain.Context, id string) (domain.Upload, error) { return domain.Upload{ID:id}, nil }

func TestEvaluate_Enqueue_Success(t *testing.T) {
	t.Parallel()
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	jr := &stubJobRepo{ id: "job-abc" }
	q := &stubQueue{}
	u := &stubUploadRepo{}
	svc := usecase.NewEvaluateService(jr, q, u)
	jobID, err := svc.Enqueue(ctx, "cv-1", "pr-1", "jd", "sc", "")
	require.NoError(t, err)
	assert.Equal(t, "job-abc", jobID)
	assert.Equal(t, domain.JobQueued, jr.created.Status)
	assert.Equal(t, "cv-1", q.payload.CVID)
	assert.Equal(t, "pr-1", q.payload.ProjectID)
}

func TestEvaluate_Enqueue_InvalidArgs(t *testing.T) {
	t.Parallel()
	svc := usecase.NewEvaluateService(&stubJobRepo{}, &stubQueue{}, &stubUploadRepo{})
	_, err := svc.Enqueue(context.Background(), "", "pr-1", "jd", "sc", "")
	require.Error(t, err)
}

func TestEvaluate_Enqueue_QueueFail_UpdatesJobFailed(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	jr := &stubJobRepo{}
	q := &stubQueue{ err: errors.New("queue down") }
	u := &stubUploadRepo{}
	svc := usecase.NewEvaluateService(jr, q, u)
	_, err := svc.Enqueue(ctx, "cv-1", "pr-1", "jd", "sc", "")
	require.Error(t, err)
	assert.Equal(t, domain.JobFailed, jr.status)
}
