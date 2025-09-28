package asynqadp_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/fairyhunter13/ai-cv-evaluator/internal/adapter/queue/asynq"
	"github.com/fairyhunter13/ai-cv-evaluator/internal/domain"
)

func TestNew(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		redisURL  string
		wantErr   bool
		errContains string
	}{
		{
			name:     "valid redis URL",
			redisURL: "redis://localhost:6379",
			wantErr:  false,
		},
		{
			name:     "valid redis URL with database",
			redisURL: "redis://localhost:6379/1",
			wantErr:  false,
		},
		{
			name:        "invalid redis URL",
			redisURL:    "invalid://url",
			wantErr:     true,
			errContains: "redis",
		},
		{
			name:        "empty URL",
			redisURL:    "",
			wantErr:     true,
			errContains: "redis",
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			q, err := asynqadp.New(tt.redisURL)
			
			if tt.wantErr {
				require.Error(t, err)
				if tt.errContains != "" {
					assert.Contains(t, err.Error(), tt.errContains)
				}
				assert.Nil(t, q)
			} else {
				require.NoError(t, err)
				assert.NotNil(t, q)
			}
		})
	}
}

func TestQueue_EnqueueEvaluate(t *testing.T) {
	// Skip if no Redis is available
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	q, err := asynqadp.New("redis://localhost:6379/15") // Use database 15 for testing
	if err != nil {
		t.Skip("Redis not available:", err)
	}

	tests := []struct {
		name    string
		payload domain.EvaluateTaskPayload
		wantErr bool
	}{
		{
			name: "valid payload",
			payload: domain.EvaluateTaskPayload{
				JobID:     "test-job-123",
				CVID:      "cv-456",
				ProjectID: "proj-789",
			},
			wantErr: false,
		},
		{
			name: "empty job ID",
			payload: domain.EvaluateTaskPayload{
				JobID:     "",
				CVID:      "cv-456",
				ProjectID: "proj-789",
			},
			wantErr: false, // Should still enqueue
		},
		{
			name: "all fields populated",
			payload: domain.EvaluateTaskPayload{
				JobID:     "full-job-001",
				CVID:      "full-cv-002",
				ProjectID: "full-proj-003",
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.Background()
			taskID, err := q.EnqueueEvaluate(ctx, tt.payload)
			
			if tt.wantErr {
				require.Error(t, err)
				assert.Empty(t, taskID)
			} else {
				require.NoError(t, err)
				assert.NotEmpty(t, taskID)
				
				// Task ID should be a valid format (typically UUID-like)
				assert.Greater(t, len(taskID), 10)
			}
		})
	}
}

func TestTaskConstant(t *testing.T) {
	// Ensure the task name constant is what we expect
	assert.Equal(t, "evaluate_job", asynqadp.TaskEvaluate)
}
