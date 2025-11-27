// Package redpanda provides Redpanda/Kafka queue integration.
//
// It handles message publishing and consumption for job processing.
// The package provides reliable message delivery with exactly-once
// semantics and supports horizontal scaling of workers.
package redpanda

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"go.opentelemetry.io/otel"

	"github.com/fairyhunter13/ai-cv-evaluator/internal/adapter/observability"
	qdrantcli "github.com/fairyhunter13/ai-cv-evaluator/internal/adapter/vector/qdrant"
	"github.com/fairyhunter13/ai-cv-evaluator/internal/domain"
)

// HandleEvaluate processes an evaluation task with the given dependencies.
// This is the evaluation logic that uses the enhanced AI evaluation system by default.
func HandleEvaluate(
	ctx context.Context,
	jobs domain.JobRepository,
	uploads domain.UploadRepository,
	results domain.ResultRepository,
	ai domain.AIClient,
	q *qdrantcli.Client,
	payload domain.EvaluateTaskPayload,
) error {
	tracer := otel.Tracer("queue.handler")
	ctx, span := tracer.Start(ctx, "HandleEvaluate")
	defer span.End()

	// Check for nil dependencies
	if jobs == nil {
		return fmt.Errorf("job repository is nil")
	}
	if uploads == nil {
		return fmt.Errorf("upload repository is nil")
	}
	if results == nil {
		return fmt.Errorf("result repository is nil")
	}
	if ai == nil {
		return fmt.Errorf("AI client is nil")
	}

	// FIXED: Add timeout handling for stuck processing jobs
	// Create a timeout context for the entire evaluation process
	timeoutDuration := 5 * time.Minute // 5 minutes timeout for AI processing
	evalCtx, cancel := context.WithTimeout(ctx, timeoutDuration)
	defer cancel()

	job, err := jobs.Get(ctx, payload.JobID)
	if err == nil && job.Status == domain.JobCompleted {
		slog.Info("job already completed; skipping evaluation", slog.String("job_id", payload.JobID))
		return nil
	}

	// Update job status to processing
	if err := jobs.UpdateStatus(evalCtx, payload.JobID, domain.JobProcessing, nil); err != nil {
		slog.Error("failed to update job status to processing", slog.String("job_id", payload.JobID), slog.Any("error", err))
		return fmt.Errorf("update job status: %w", err)
	}

	// Monitor for timeout and update job status if needed.
	// When local fallback is enabled (E2E/dev), we use the same heuristic
	// fallback that the main evaluation flow uses after exhausted retries so
	// that jobs cannot remain stuck in processing even if upstream AI ignores
	// the context deadline.
	go func() {
		<-evalCtx.Done()
		if evalCtx.Err() != context.DeadlineExceeded {
			return
		}

		slog.Warn("job processing timeout exceeded, marking as failed",
			slog.String("job_id", payload.JobID),
			slog.Duration("timeout", timeoutDuration))

		// Try to update job status to failed
		timeoutMsg := fmt.Sprintf("job processing timeout after %v", timeoutDuration)
		if err := jobs.UpdateStatus(ctx, payload.JobID, domain.JobFailed, &timeoutMsg); err != nil {
			slog.Error("failed to update job status to failed after timeout",
				slog.String("job_id", payload.JobID),
				slog.Any("error", err))
		}
	}()

	// Get CV content
	cvUpload, err := uploads.Get(evalCtx, payload.CVID)
	if err != nil {
		slog.Error("failed to get CV content", slog.String("job_id", payload.JobID), slog.String("cv_id", payload.CVID), slog.Any("error", err))
		_ = jobs.UpdateStatus(ctx, payload.JobID, domain.JobFailed, ptr("failed to get CV content"))
		return fmt.Errorf("get CV content: %w", err)
	}

	// Get project content
	projectUpload, err := uploads.Get(evalCtx, payload.ProjectID)
	if err != nil {
		slog.Error("failed to get project content", slog.String("job_id", payload.JobID), slog.String("project_id", payload.ProjectID), slog.Any("error", err))
		_ = jobs.UpdateStatus(ctx, payload.JobID, domain.JobFailed, ptr("failed to get project content"))
		return fmt.Errorf("get project content: %w", err)
	}

	// Perform enhanced AI evaluation with retry logic and model fallback
	slog.Info("performing enhanced AI evaluation with retry logic", slog.String("job_id", payload.JobID))
	handler := NewIntegratedEvaluationHandler(ai, q)

	// Retry evaluation with exponential backoff
	maxRetries := 3
	var result domain.Result
	var lastErr error

	for attempt := 1; attempt <= maxRetries; attempt++ {
		slog.Info("evaluation attempt", slog.String("job_id", payload.JobID), slog.Int("attempt", attempt), slog.Int("max_retries", maxRetries))

		result, lastErr = handler.PerformIntegratedEvaluation(evalCtx, cvUpload.Text, projectUpload.Text, payload.JobDescription, payload.StudyCaseBrief, payload.ScoringRubric, payload.JobID)
		if lastErr == nil {
			slog.Info("evaluation succeeded", slog.String("job_id", payload.JobID), slog.Int("attempt", attempt))
			break
		}

		slog.Warn("evaluation attempt failed",
			slog.String("job_id", payload.JobID),
			slog.Int("attempt", attempt),
			slog.Int("max_retries", maxRetries),
			slog.Any("error", lastErr))

		// If this is not the last attempt, wait before retrying
		if attempt < maxRetries {
			backoffDuration := time.Duration(attempt) * 2 * time.Second
			slog.Info("waiting before retry", slog.String("job_id", payload.JobID), slog.Duration("backoff", backoffDuration))
			time.Sleep(backoffDuration)
		}
	}

	if lastErr != nil {
		slog.Error("enhanced evaluation failed after all retries",
			slog.String("job_id", payload.JobID),
			slog.Int("attempts", maxRetries),
			slog.Any("error", lastErr))

		// Build retry info and mark job as failed so that higher-level retry/DLQ
		// mechanisms can handle it. No synthetic results are created here; any
		// successful Result must come from the real AI evaluation pipeline.
		retryInfo := &domain.RetryInfo{
			AttemptCount:  maxRetries,
			MaxAttempts:   maxRetries,
			LastAttemptAt: time.Now(),
			RetryStatus:   domain.RetryStatusExhausted,
			LastError:     lastErr.Error(),
			ErrorHistory:  []string{lastErr.Error()},
			CreatedAt:     time.Now(),
			UpdatedAt:     time.Now(),
		}

		for attempt := 1; attempt <= maxRetries; attempt++ {
			retryInfo.ErrorHistory = append(retryInfo.ErrorHistory, fmt.Sprintf("attempt %d failed", attempt))
		}

		retryInfo.MarkAsExhausted()

		// Classify upstream AI rate-limit scenarios explicitly so that the
		// ResultService can map them to UPSTREAM_RATE_LIMIT instead of a generic
		// INTERNAL error. This is based on the error message returned from the
		// enhanced AI pipeline, which includes details about Groq/OpenRouter
		// provider blocking.
		msg := "enhanced evaluation failed after retries"
		lowerErr := strings.ToLower(lastErr.Error())
		if strings.Contains(lowerErr, "rate limit") {
			// When one or more upstream providers are rate limited, surface a
			// stable message that still contains the phrase "rate limit" so that
			// errorCodeFromJobError maps it to UPSTREAM_RATE_LIMIT.
			msg = "ai providers rate limited; one or more upstream AI providers temporarily unavailable"
			if strings.Contains(lowerErr, "groq chat failed") && strings.Contains(lowerErr, "openrouter chat failed") {
				msg = "ai providers rate limited; groq and openrouter temporarily unavailable"
			}
		}

		_ = jobs.UpdateStatus(ctx, payload.JobID, domain.JobFailed, ptr(msg))
		return fmt.Errorf("enhanced evaluation failed after %d attempts: %w", maxRetries, lastErr)
	}

	// Store the result FIRST
	slog.Info("storing evaluation result", slog.String("job_id", payload.JobID))
	if err := results.Upsert(ctx, result); err != nil {
		slog.Error("failed to store result", slog.String("job_id", payload.JobID), slog.Any("error", err))
		return fmt.Errorf("store result: %w", err)
	}
	slog.Info("evaluation result stored successfully", slog.String("job_id", payload.JobID))

	// Update job status to completed AFTER storing result
	slog.Info("updating job status to completed", slog.String("job_id", payload.JobID))
	if err := jobs.UpdateStatus(ctx, payload.JobID, domain.JobCompleted, nil); err != nil {
		slog.Error("failed to update job status to completed", slog.String("job_id", payload.JobID), slog.Any("error", err))
		return fmt.Errorf("update job status: %w", err)
	}
	slog.Info("job status updated to completed successfully", slog.String("job_id", payload.JobID))
	observability.CompleteJob("evaluate")
	slog.Info("job completed", slog.String("job_id", payload.JobID))
	return nil
}

// ptr returns a pointer to the given string.
