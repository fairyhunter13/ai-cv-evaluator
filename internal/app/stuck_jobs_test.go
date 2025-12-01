package app

import (
	"context"
	"testing"
	"time"

	"github.com/fairyhunter13/ai-cv-evaluator/internal/domain"
)

type fakeJobRepo struct {
	jobs        []domain.Job
	updateCalls []struct {
		id     string
		status domain.JobStatus
		msg    *string
	}
	listErr   error
	updateErr error
}

func (r *fakeJobRepo) Create(context.Context, domain.Job) (string, error) { return "", nil }
func (r *fakeJobRepo) UpdateStatus(_ context.Context, id string, status domain.JobStatus, msg *string) error {
	if r.updateErr != nil {
		return r.updateErr
	}
	r.updateCalls = append(r.updateCalls, struct {
		id     string
		status domain.JobStatus
		msg    *string
	}{id: id, status: status, msg: msg})
	return nil
}
func (r *fakeJobRepo) Get(context.Context, string) (domain.Job, error) { return domain.Job{}, nil }
func (r *fakeJobRepo) FindByIdempotencyKey(context.Context, string) (domain.Job, error) {
	return domain.Job{}, nil
}
func (r *fakeJobRepo) Count(context.Context) (int64, error) { return 0, nil }
func (r *fakeJobRepo) CountByStatus(_ domain.Context, _ domain.JobStatus) (int64, error) {
	return 0, nil
}
func (r *fakeJobRepo) List(context.Context, int, int) ([]domain.Job, error) { return nil, nil }
func (r *fakeJobRepo) ListWithFilters(context.Context, int, int, string, string) ([]domain.Job, error) {
	if r.listErr != nil {
		return nil, r.listErr
	}
	return r.jobs, nil
}
func (r *fakeJobRepo) CountWithFilters(context.Context, string, string) (int64, error) {
	return int64(len(r.jobs)), nil
}
func (r *fakeJobRepo) GetAverageProcessingTime(context.Context) (float64, error) {
	return 0, nil
}

func TestNewStuckJobSweeperDefaults(t *testing.T) {
	repo := &fakeJobRepo{}
	s := NewStuckJobSweeper(repo, 0, 0)
	if s == nil {
		t.Fatalf("expected non-nil sweeper")
	}
	if s.maxProcessingAge <= 0 {
		t.Fatalf("maxProcessingAge should be set to default, got %v", s.maxProcessingAge)
	}
	if s.interval <= 0 {
		t.Fatalf("interval should be set to default, got %v", s.interval)
	}
}

func TestNewStuckJobSweeperNilRepo(t *testing.T) {
	if sweeper := NewStuckJobSweeper(nil, time.Minute, time.Minute); sweeper != nil {
		t.Fatalf("expected nil sweeper when repo is nil")
	}
}

func TestStuckJobSweeperSweepOnceMarksOldJobsFailed(t *testing.T) {
	now := time.Now()
	repo := &fakeJobRepo{
		jobs: []domain.Job{
			{ID: "old", Status: domain.JobProcessing, UpdatedAt: now.Add(-10 * time.Minute)},
			{ID: "recent", Status: domain.JobProcessing, UpdatedAt: now.Add(-1 * time.Minute)},
		},
	}
	s := &StuckJobSweeper{
		jobs:             repo,
		maxProcessingAge: 5 * time.Minute,
		interval:         time.Minute,
	}

	s.sweepOnce(context.Background())

	if len(repo.updateCalls) != 1 {
		t.Fatalf("expected 1 update call, got %d", len(repo.updateCalls))
	}
	call := repo.updateCalls[0]
	if call.id != "old" {
		t.Fatalf("expected job 'old' to be updated, got %q", call.id)
	}
	if call.status != domain.JobFailed {
		t.Fatalf("expected status %q, got %q", domain.JobFailed, call.status)
	}
	if call.msg == nil || *call.msg == "" {
		t.Fatalf("expected non-empty failure message")
	}
}

func TestStuckJobSweeperRunStopsOnContextDone(t *testing.T) {
	repo := &fakeJobRepo{}
	s := NewStuckJobSweeper(repo, time.Minute, 10*time.Millisecond)
	if s == nil {
		t.Fatalf("expected non-nil sweeper")
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	ch := make(chan struct{})
	go func() {
		s.Run(ctx)
		close(ch)
	}()

	time.Sleep(30 * time.Millisecond)
	cancel()

	select {
	case <-ch:
	case <-time.After(200 * time.Millisecond):
		t.Fatalf("Run did not exit after context cancellation")
	}
}
