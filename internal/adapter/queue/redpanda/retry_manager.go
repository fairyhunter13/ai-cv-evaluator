// Package redpanda implements retry and DLQ management for resilient job processing.
package redpanda

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/fairyhunter13/ai-cv-evaluator/internal/domain"
)

// RetryManager handles automatic retries and DLQ management
type RetryManager struct {
	producer    *Producer
	dlqProducer *Producer
	jobs        domain.JobRepository
	config      domain.RetryConfig
}

// NewRetryManager creates a new retry manager
func NewRetryManager(producer, dlqProducer *Producer, jobs domain.JobRepository, config domain.RetryConfig) *RetryManager {
	return &RetryManager{
		producer:    producer,
		dlqProducer: dlqProducer,
		jobs:        jobs,
		config:      config,
	}
}

// RetryJob attempts to retry a failed job
func (rm *RetryManager) RetryJob(ctx context.Context, jobID string, retryInfo *domain.RetryInfo, payload domain.EvaluateTaskPayload) error {
	// For upstream rate-limit and timeout failures, bypass immediate inline
	// retries and route the job directly to DLQ so that the DLQ consumer can
	// enforce a cooling window before requeueing. This prevents hammering AI
	// providers that have already signaled backpressure or long latencies.
	code := classifyFailureCode(retryInfo.LastError)
	if code == "UPSTREAM_RATE_LIMIT" || code == "UPSTREAM_TIMEOUT" {
		reason := retryInfo.LastError
		slog.Info("routing upstream failure to DLQ for cooldown",
			slog.String("job_id", jobID),
			slog.String("error_code", code),
			slog.String("last_error", retryInfo.LastError))
		return rm.moveToDLQ(ctx, jobID, payload, retryInfo, reason)
	}

	// Check if job should be retried under generic retry policy
	if !retryInfo.ShouldRetry(fmt.Errorf("%s", retryInfo.LastError), rm.config) {
		slog.Info("job should not be retried, moving to DLQ",
			slog.String("job_id", jobID),
			slog.String("last_error", retryInfo.LastError),
			slog.String("retry_status", string(retryInfo.RetryStatus)))
		return rm.moveToDLQ(ctx, jobID, payload, retryInfo, "job should not be retried")
	}

	// Check if max retries reached
	if retryInfo.AttemptCount >= rm.config.MaxRetries {
		slog.Info("max retries reached, moving to DLQ",
			slog.String("job_id", jobID),
			slog.Int("attempt_count", retryInfo.AttemptCount),
			slog.Int("max_retries", rm.config.MaxRetries))
		return rm.moveToDLQ(ctx, jobID, payload, retryInfo, "max retries reached")
	}

	// Calculate next retry delay
	delay := retryInfo.CalculateNextRetryDelay(rm.config)
	retryInfo.NextRetryAt = time.Now().Add(delay)

	// Update retry info
	retryInfo.MarkAsRetrying()
	retryInfo.UpdateRetryAttempt(nil) // No error for retry attempt

	// Update job status to queued for retry
	if err := rm.jobs.UpdateStatus(ctx, jobID, domain.JobQueued, nil); err != nil {
		slog.Error("failed to update job status for retry",
			slog.String("job_id", jobID),
			slog.Any("error", err))
		return fmt.Errorf("update job status for retry: %w", err)
	}

	// Schedule retry with delay
	go rm.scheduleRetry(ctx, jobID, payload, retryInfo)

	slog.Info("job scheduled for retry",
		slog.String("job_id", jobID),
		slog.Int("attempt", retryInfo.AttemptCount),
		slog.Duration("delay", delay),
		slog.Time("next_retry_at", retryInfo.NextRetryAt))

	return nil
}

// scheduleRetry schedules a job for retry after a delay
func (rm *RetryManager) scheduleRetry(ctx context.Context, jobID string, payload domain.EvaluateTaskPayload, retryInfo *domain.RetryInfo) {
	// Wait for the calculated delay
	delay := retryInfo.CalculateNextRetryDelay(rm.config)
	time.Sleep(delay)

	// Check if job is still eligible for retry
	job, err := rm.jobs.Get(ctx, jobID)
	if err != nil {
		slog.Error("failed to get job for retry",
			slog.String("job_id", jobID),
			slog.Any("error", err))
		return
	}

	// Don't retry if job is no longer in queued status
	if job.Status != domain.JobQueued {
		slog.Info("job status changed, skipping retry",
			slog.String("job_id", jobID),
			slog.String("current_status", string(job.Status)))
		return
	}

	// Enqueue the job for retry
	_, err = rm.producer.EnqueueEvaluate(ctx, payload)
	if err != nil {
		slog.Error("failed to enqueue job for retry",
			slog.String("job_id", jobID),
			slog.Any("error", err))

		// Mark as exhausted if we can't even enqueue
		retryInfo.MarkAsExhausted()
		_ = rm.jobs.UpdateStatus(ctx, jobID, domain.JobFailed, ptr("failed to enqueue for retry"))
		return
	}

	slog.Info("job enqueued for retry",
		slog.String("job_id", jobID),
		slog.Int("attempt", retryInfo.AttemptCount))
}

// moveToDLQ moves a job to the Dead Letter Queue
func (rm *RetryManager) moveToDLQ(ctx context.Context, jobID string, payload domain.EvaluateTaskPayload, retryInfo *domain.RetryInfo, reason string) error {
	// Create DLQ job
	dlqJob := domain.DLQJob{
		JobID:            jobID,
		OriginalPayload:  payload,
		RetryInfo:        *retryInfo,
		FailureReason:    reason,
		MovedToDLQAt:     time.Now(),
		CanBeReprocessed: true,
	}

	// Mark retry info as DLQ
	retryInfo.MarkAsDLQ()

	// Serialize DLQ job
	dlqData, err := json.Marshal(dlqJob)
	if err != nil {
		slog.Error("failed to marshal DLQ job",
			slog.String("job_id", jobID),
			slog.Any("error", err))
		return fmt.Errorf("marshal DLQ job: %w", err)
	}

	// Send to DLQ topic
	if err := rm.dlqProducer.EnqueueDLQ(ctx, jobID, dlqData); err != nil {
		slog.Error("failed to enqueue job to DLQ",
			slog.String("job_id", jobID),
			slog.Any("error", err))
		return fmt.Errorf("enqueue to DLQ: %w", err)
	}

	// Update job status to failed
	if err := rm.jobs.UpdateStatus(ctx, jobID, domain.JobFailed, &reason); err != nil {
		slog.Error("failed to update job status to failed",
			slog.String("job_id", jobID),
			slog.Any("error", err))
	}

	slog.Info("job moved to DLQ",
		slog.String("job_id", jobID),
		slog.String("reason", reason),
		slog.Int("attempt_count", retryInfo.AttemptCount),
		slog.String("retry_status", string(retryInfo.RetryStatus)))

	return nil
}

// ProcessDLQJob processes a job from the Dead Letter Queue
func (rm *RetryManager) ProcessDLQJob(ctx context.Context, dlqJob domain.DLQJob) error {
	// Check if job can be reprocessed
	if !dlqJob.CanBeReprocessed {
		slog.Info("DLQ job cannot be reprocessed",
			slog.String("job_id", dlqJob.JobID),
			slog.String("failure_reason", dlqJob.FailureReason))
		return fmt.Errorf("DLQ job cannot be reprocessed")
	}

	// For upstream rate-limit and timeout failures, enforce a cooling window
	// before reprocessing. This prevents immediately hammering upstream
	// providers that have signaled temporary rate limiting or produced repeated
	// timeouts.
	loweredReason := strings.ToLower(dlqJob.FailureReason)
	loweredError := strings.ToLower(dlqJob.RetryInfo.LastError)
	combined := loweredReason + " " + loweredError
	isRateLimitOrTimeout := strings.Contains(combined, "rate limit") ||
		strings.Contains(combined, "timeout") ||
		strings.Contains(combined, "deadline exceeded")
	const rateLimitDLQCooldown = 30 * time.Second
	if isRateLimitOrTimeout {
		cooldownUntil := dlqJob.MovedToDLQAt.Add(rateLimitDLQCooldown)
		if delay := time.Until(cooldownUntil); delay > 0 {
			slog.Info("DLQ cooling in effect for upstream rate limit/timeout",
				slog.String("job_id", dlqJob.JobID),
				slog.Duration("cooling_remaining", delay))
			go func(job domain.DLQJob, d time.Duration) {
				time.Sleep(d)
				if err := rm.requeueFromDLQ(context.Background(), job); err != nil {
					slog.Error("failed to requeue cooled DLQ job",
						slog.String("job_id", job.JobID),
						slog.Any("error", err))
				}
			}(dlqJob, delay)
			return nil
		}
	}

	return rm.requeueFromDLQ(ctx, dlqJob)
}

// requeueFromDLQ updates job status and enqueues the original payload back to the
// main evaluate topic for reprocessing.
func (rm *RetryManager) requeueFromDLQ(ctx context.Context, dlqJob domain.DLQJob) error {
	if err := rm.jobs.UpdateStatus(ctx, dlqJob.JobID, domain.JobQueued, nil); err != nil {
		slog.Error("failed to update job status for DLQ reprocessing",
			slog.String("job_id", dlqJob.JobID),
			slog.Any("error", err))
		return fmt.Errorf("update job status for DLQ reprocessing: %w", err)
	}

	_, err := rm.producer.EnqueueEvaluate(ctx, dlqJob.OriginalPayload)
	if err != nil {
		slog.Error("failed to enqueue DLQ job for reprocessing",
			slog.String("job_id", dlqJob.JobID),
			slog.Any("error", err))
		return fmt.Errorf("enqueue DLQ job for reprocessing: %w", err)
	}

	slog.Info("DLQ job enqueued for reprocessing",
		slog.String("job_id", dlqJob.JobID),
		slog.String("original_failure_reason", dlqJob.FailureReason))

	return nil
}

// GetRetryStats returns retry statistics
func (rm *RetryManager) GetRetryStats(_ context.Context) (map[string]interface{}, error) {
	// This would typically query the database for retry statistics
	// For now, return a placeholder
	return map[string]interface{}{
		"total_retries":      0,
		"successful_retries": 0,
		"failed_retries":     0,
		"dlq_jobs":           0,
	}, nil
}

// Helper function to create a string pointer
func ptr(s string) *string {
	return &s
}
