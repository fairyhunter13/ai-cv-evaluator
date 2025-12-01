package redpanda

import (
	"context"
	"testing"
	"time"

	"github.com/fairyhunter13/ai-cv-evaluator/internal/domain"
)

type fakeRetryProducer struct {
	enqueueEvaluateCalls []domain.EvaluateTaskPayload
	enqueueDLQCalls      []struct {
		jobID string
		data  []byte
	}
}

func (p *fakeRetryProducer) EnqueueEvaluate(_ context.Context, payload domain.EvaluateTaskPayload) (string, error) {
	p.enqueueEvaluateCalls = append(p.enqueueEvaluateCalls, payload)
	return payload.JobID, nil
}

func (p *fakeRetryProducer) EnqueueDLQ(_ context.Context, jobID string, dlqData []byte) error {
	p.enqueueDLQCalls = append(p.enqueueDLQCalls, struct {
		jobID string
		data  []byte
	}{jobID: jobID, data: dlqData})
	return nil
}

type fakeJobRepo struct {
	updated []struct {
		id     string
		status domain.JobStatus
		msg    *string
	}
	jobs map[string]domain.Job
}

func (r *fakeJobRepo) Create(_ domain.Context, j domain.Job) (string, error) { return j.ID, nil }
func (r *fakeJobRepo) UpdateStatus(_ domain.Context, id string, status domain.JobStatus, msg *string) error {
	r.updated = append(r.updated, struct {
		id     string
		status domain.JobStatus
		msg    *string
	}{id: id, status: status, msg: msg})
	if r.jobs != nil {
		job := r.jobs[id]
		job.Status = status
		r.jobs[id] = job
	}
	return nil
}
func (r *fakeJobRepo) Get(_ domain.Context, id string) (domain.Job, error) {
	if r.jobs != nil {
		if j, ok := r.jobs[id]; ok {
			return j, nil
		}
	}
	return domain.Job{ID: id}, nil
}
func (*fakeJobRepo) FindByIdempotencyKey(domain.Context, string) (domain.Job, error) {
	return domain.Job{}, nil
}
func (*fakeJobRepo) Count(domain.Context) (int64, error) { return 0, nil }
func (*fakeJobRepo) CountByStatus(domain.Context, domain.JobStatus) (int64, error) {
	return 0, nil
}
func (*fakeJobRepo) List(domain.Context, int, int) ([]domain.Job, error) { return nil, nil }
func (*fakeJobRepo) ListWithFilters(domain.Context, int, int, string, string) ([]domain.Job, error) {
	return nil, nil
}
func (*fakeJobRepo) CountWithFilters(domain.Context, string, string) (int64, error) {
	return 0, nil
}
func (*fakeJobRepo) GetAverageProcessingTime(domain.Context) (float64, error) { return 0, nil }

func TestRetryManager_MoveToDLQ_SetsStatusAndEnqueues(t *testing.T) {
	ctx := context.Background()
	prod := &fakeRetryProducer{}
	jobs := &fakeJobRepo{jobs: make(map[string]domain.Job)}
	cfg := domain.DefaultRetryConfig()
	rm := NewRetryManager(prod, prod, jobs, cfg)

	retryInfo := &domain.RetryInfo{
		AttemptCount: 1,
		MaxAttempts:  cfg.MaxRetries,
		LastError:    "temporary failure",
		ErrorHistory: []string{"temporary failure"},
	}
	payload := domain.EvaluateTaskPayload{JobID: "job-1"}

	if err := rm.moveToDLQ(ctx, "job-1", payload, retryInfo, "reason"); err != nil {
		t.Fatalf("moveToDLQ returned error: %v", err)
	}

	if retryInfo.RetryStatus != domain.RetryStatusDLQ {
		t.Fatalf("expected RetryStatusDLQ, got %v", retryInfo.RetryStatus)
	}
	if len(prod.enqueueDLQCalls) != 1 {
		t.Fatalf("expected 1 DLQ enqueue call, got %d", len(prod.enqueueDLQCalls))
	}
	if len(jobs.updated) == 0 || jobs.updated[0].status != domain.JobFailed {
		t.Fatalf("expected job status to be updated to failed, updates=%v", jobs.updated)
	}
}

func TestRetryManager_RequeueFromDLQ_UpdatesStatusAndEnqueues(t *testing.T) {
	ctx := context.Background()
	prod := &fakeRetryProducer{}
	jobs := &fakeJobRepo{jobs: map[string]domain.Job{"job-1": {ID: "job-1", Status: domain.JobQueued}}}
	cfg := domain.DefaultRetryConfig()
	rm := NewRetryManager(prod, prod, jobs, cfg)

	dlq := domain.DLQJob{JobID: "job-1", OriginalPayload: domain.EvaluateTaskPayload{JobID: "job-1"}}

	if err := rm.requeueFromDLQ(ctx, dlq); err != nil {
		t.Fatalf("requeueFromDLQ returned error: %v", err)
	}
	if len(prod.enqueueEvaluateCalls) != 1 {
		t.Fatalf("expected 1 enqueueEvaluate call, got %d", len(prod.enqueueEvaluateCalls))
	}
	if len(jobs.updated) == 0 || jobs.updated[0].status != domain.JobQueued {
		t.Fatalf("expected job status to be updated to queued, updates=%v", jobs.updated)
	}
}

func TestRetryManager_ProcessDLQJob_CannotReprocess(t *testing.T) {
	ctx := context.Background()
	rm := NewRetryManager(&fakeRetryProducer{}, &fakeRetryProducer{}, &fakeJobRepo{}, domain.DefaultRetryConfig())

	dlq := domain.DLQJob{JobID: "job-1", FailureReason: "permanent", CanBeReprocessed: false}

	if err := rm.ProcessDLQJob(ctx, dlq); err == nil {
		t.Fatalf("expected error for DLQ job that cannot be reprocessed")
	}
}

func TestRetryManager_ProcessDLQJob_RequeuesWhenEligibleAndNotRateLimited(t *testing.T) {
	ctx := context.Background()
	prod := &fakeRetryProducer{}
	jobs := &fakeJobRepo{jobs: map[string]domain.Job{"job-1": {ID: "job-1", Status: domain.JobQueued}}}
	cfg := domain.DefaultRetryConfig()
	rm := NewRetryManager(prod, prod, jobs, cfg)

	dlq := domain.DLQJob{
		JobID:         "job-1",
		FailureReason: "permanent failure",
		RetryInfo: domain.RetryInfo{
			LastError: "permanent failure",
		},
		MovedToDLQAt:     time.Now().Add(-time.Hour),
		CanBeReprocessed: true,
	}

	if err := rm.ProcessDLQJob(ctx, dlq); err != nil {
		t.Fatalf("ProcessDLQJob returned error: %v", err)
	}
	if len(prod.enqueueEvaluateCalls) != 1 {
		t.Fatalf("expected 1 enqueueEvaluate call, got %d", len(prod.enqueueEvaluateCalls))
	}
}

func TestRetryManager_RetryJob_RoutesUpstreamRateLimitToDLQ(t *testing.T) {
	ctx := context.Background()
	prod := &fakeRetryProducer{}
	jobs := &fakeJobRepo{jobs: make(map[string]domain.Job)}
	cfg := domain.DefaultRetryConfig()
	rm := NewRetryManager(prod, prod, jobs, cfg)

	retryInfo := &domain.RetryInfo{
		AttemptCount: 0,
		MaxAttempts:  cfg.MaxRetries,
		LastError:    "upstream rate limit",
		RetryStatus:  domain.RetryStatusNone,
	}
	payload := domain.EvaluateTaskPayload{JobID: "job-1"}

	if err := rm.RetryJob(ctx, "job-1", retryInfo, payload); err != nil {
		t.Fatalf("RetryJob returned error: %v", err)
	}
	if len(prod.enqueueDLQCalls) != 1 {
		t.Fatalf("expected 1 DLQ enqueue call, got %d", len(prod.enqueueDLQCalls))
	}
}

func TestRetryManager_GetRetryStats_ReturnsMap(t *testing.T) {
	rm := NewRetryManager(&fakeRetryProducer{}, &fakeRetryProducer{}, &fakeJobRepo{}, domain.DefaultRetryConfig())

	stats, err := rm.GetRetryStats(context.Background())
	if err != nil {
		t.Fatalf("GetRetryStats returned error: %v", err)
	}
	if _, ok := stats["total_retries"]; !ok {
		t.Fatalf("expected total_retries key in stats map")
	}
}
