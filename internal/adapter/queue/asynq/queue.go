package asynqadp

import (
    "context"
    "encoding/json"
    "fmt"
    "time"

    "github.com/hibiken/asynq"

    "github.com/fairyhunter13/ai-cv-evaluator/internal/domain"
    "github.com/fairyhunter13/ai-cv-evaluator/internal/adapter/observability"
)

// TaskEvaluate is the task type name for evaluation jobs.
const TaskEvaluate = "evaluate_job"

// asynqClient abstracts the asynq client for testability.
type asynqClient interface {
    EnqueueContext(ctx context.Context, task *asynq.Task, opts ...asynq.Option) (*asynq.TaskInfo, error)
}

// Queue wraps an asynq client and implements domain.Queue.
type Queue struct { client asynqClient }

// New constructs a Queue backed by a Redis URI understood by asynq.
func New(redisURL string) (*Queue, error) {
    opt, err := asynq.ParseRedisURI(redisURL)
    if err != nil { return nil, fmt.Errorf("redis: %w", err) }
    return &Queue{client: asynq.NewClient(opt)}, nil
}

// NewWithClient allows injecting a custom asynq client (for unit tests).
func NewWithClient(c asynqClient) *Queue { return &Queue{client: c} }

// EnqueueEvaluate enqueues an evaluation task and returns its asynq task ID.
func (q *Queue) EnqueueEvaluate(ctx domain.Context, payload domain.EvaluateTaskPayload) (string, error) {
    b, _ := json.Marshal(payload)
    t := asynq.NewTask(TaskEvaluate, b)
    info, err := q.client.EnqueueContext(ctx, t, asynq.MaxRetry(5), asynq.Retention(24*time.Hour))
    if err != nil { return "", fmt.Errorf("op=queue.enqueue: %w", err) }
    observability.EnqueueJob("evaluate")
    return info.ID, nil
}
