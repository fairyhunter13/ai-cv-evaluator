package asynqadp_test

import (
	"context"
	"errors"
	"testing"

	"github.com/hibiken/asynq"

	asynqadp "github.com/fairyhunter13/ai-cv-evaluator/internal/adapter/queue/asynq"
	"github.com/fairyhunter13/ai-cv-evaluator/internal/domain"
)

type fakeClient struct{ wantErr bool }

func (f fakeClient) EnqueueContext(_ context.Context, _ *asynq.Task, _ ...asynq.Option) (*asynq.TaskInfo, error) {
	if f.wantErr { return nil, errors.New("enqueue fail") }
	return &asynq.TaskInfo{ID: "tid-123"}, nil
}

func TestQueue_EnqueueEvaluate_Unit(t *testing.T) {
	q := asynqadp.NewWithClient(fakeClient{})
	id, err := q.EnqueueEvaluate(context.Background(), domain.EvaluateTaskPayload{JobID: "j1", CVID: "c", ProjectID: "p"})
	if err != nil { t.Fatalf("unexpected err: %v", err) }
	if id == "" { t.Fatalf("expected id") }
}

func TestQueue_EnqueueEvaluate_Error(t *testing.T) {
	q := asynqadp.NewWithClient(fakeClient{wantErr: true})
	_, err := q.EnqueueEvaluate(context.Background(), domain.EvaluateTaskPayload{JobID: "j1"})
	if err == nil { t.Fatalf("expected error") }
	if err != nil && (err.Error() == "enqueue fail") { t.Fatalf("should be wrapped, got raw: %v", err) }
}
