package asynqadp

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/hibiken/asynq"

	"github.com/fairyhunter13/ai-cv-evaluator/internal/domain"
	"github.com/fairyhunter13/ai-cv-evaluator/internal/adapter/observability"
)

const TaskEvaluate = "evaluate_job"

type Queue struct { client *asynq.Client }

func New(redisURL string) (*Queue, error) {
	opt, err := asynq.ParseRedisURI(redisURL)
	if err != nil { return nil, fmt.Errorf("redis: %w", err) }
	return &Queue{client: asynq.NewClient(opt)}, nil
}

func (q *Queue) EnqueueEvaluate(ctx domain.Context, payload domain.EvaluateTaskPayload) (string, error) {
	b, _ := json.Marshal(payload)
	t := asynq.NewTask(TaskEvaluate, b)
	info, err := q.client.EnqueueContext(ctx, t, asynq.MaxRetry(5), asynq.Retention(24*time.Hour))
	if err != nil { return "", fmt.Errorf("op=queue.enqueue: %w", err) }
	observability.EnqueueJob("evaluate")
	return info.ID, nil
}
