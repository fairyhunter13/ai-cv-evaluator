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
	"os"
	"strings"
	"time"

	"go.opentelemetry.io/otel"

	adapterobs "github.com/fairyhunter13/ai-cv-evaluator/internal/adapter/observability"
	qdrantcli "github.com/fairyhunter13/ai-cv-evaluator/internal/adapter/vector/qdrant"
	"github.com/fairyhunter13/ai-cv-evaluator/internal/domain"
	obsctx "github.com/fairyhunter13/ai-cv-evaluator/internal/observability"
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

	lg := obsctx.LoggerFromContext(ctx)

	start := time.Now()

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
	if v := os.Getenv("E2E_AI_TIMEOUT"); v != "" {
		if d, err := time.ParseDuration(v); err == nil {
			timeoutDuration = d
		}
	}
	evalCtx, cancel := context.WithTimeout(ctx, timeoutDuration)
	defer cancel()

	// If the job is already in a terminal state, skip processing entirely. This
	// prevents re-delivered messages for completed/failed jobs from being
	// counted as additional failed evaluations in Prometheus metrics.
	job, err := jobs.Get(ctx, payload.JobID)
	if err == nil && (job.Status == domain.JobCompleted || job.Status == domain.JobFailed) {
		slog.Info("job already in terminal state; skipping evaluation",
			slog.String("job_id", payload.JobID),
			slog.String("status", string(job.Status)))
		return nil
	}

	// Track job processing lifecycle for Prometheus job-queue metrics. These
	// metrics live in the worker process and are scraped via the worker's
	// /metrics endpoint. We only start tracking after confirming the job is not
	// already in a terminal state so that re-deliveries do not skew success
	// rates.
	adapterobs.StartProcessingJob("evaluate")
	success := false
	defer func() {
		if success {
			adapterobs.CompleteJob("evaluate")
			return
		}

		adapterobs.FailJob("evaluate")

		if jobs == nil {
			return
		}

		j, err := jobs.Get(context.Background(), payload.JobID)
		if err != nil {
			lg.Error("failed to load job in deferred cleanup", slog.String("job_id", payload.JobID), slog.Any("error", err))
			return
		}
		if j.Status != domain.JobProcessing {
			return
		}
		msg := "job failed: evaluation did not reach a terminal state"
		if err := jobs.UpdateStatus(context.Background(), payload.JobID, domain.JobFailed, &msg); err != nil {
			lg.Error("failed to update job status to failed in deferred cleanup", slog.String("job_id", payload.JobID), slog.Any("error", err))
		} else {
			adapterobs.RecordJobFailureByCode("evaluate", classifyFailureCode(msg))
		}
	}()

	// Update job status to processing
	if err := jobs.UpdateStatus(evalCtx, payload.JobID, domain.JobProcessing, nil); err != nil {
		lg.Error("failed to update job status to processing", slog.String("job_id", payload.JobID), slog.Any("error", err))
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

		lg.Warn("job processing timeout exceeded, marking as failed",
			slog.String("job_id", payload.JobID),
			slog.Duration("timeout", timeoutDuration))

		// Try to update job status to failed
		timeoutMsg := fmt.Sprintf("job processing timeout after %v", timeoutDuration)
		if err := jobs.UpdateStatus(ctx, payload.JobID, domain.JobFailed, &timeoutMsg); err != nil {
			lg.Error("failed to update job status to failed after timeout",
				slog.String("job_id", payload.JobID),
				slog.Any("error", err))
			return
		}
		code := classifyFailureCode(timeoutMsg)
		adapterobs.RecordJobFailureByCode("evaluate", code)
		lg.Info("job failure recorded",
			slog.String("job_id", payload.JobID),
			slog.String("error_code", code))
	}()

	// Get CV content
	cvUpload, err := uploads.Get(evalCtx, payload.CVID)
	if err != nil {
		lg.Error("failed to get CV content", slog.String("job_id", payload.JobID), slog.String("cv_id", payload.CVID), slog.Any("error", err))
		msg := "failed to get CV content"
		_ = jobs.UpdateStatus(ctx, payload.JobID, domain.JobFailed, ptr(msg))
		adapterobs.RecordJobFailureByCode("evaluate", classifyFailureCode(msg))
		return fmt.Errorf("get CV content: %w", err)
	}

	// Get project content
	projectUpload, err := uploads.Get(evalCtx, payload.ProjectID)
	if err != nil {
		lg.Error("failed to get project content", slog.String("job_id", payload.JobID), slog.String("project_id", payload.ProjectID), slog.Any("error", err))
		msg := "failed to get project content"
		_ = jobs.UpdateStatus(ctx, payload.JobID, domain.JobFailed, ptr(msg))
		adapterobs.RecordJobFailureByCode("evaluate", classifyFailureCode(msg))
		return fmt.Errorf("get project content: %w", err)
	}

	// Perform enhanced AI evaluation with retry logic and model fallback
	lg.Info("performing enhanced AI evaluation with retry logic", slog.String("job_id", payload.JobID))
	handler := NewIntegratedEvaluationHandler(ai, q)

	// Retry evaluation with exponential backoff
	maxRetries := 3
	var result domain.Result
	var lastErr error

	for attempt := 1; attempt <= maxRetries; attempt++ {
		slog.Info("evaluation attempt", slog.String("job_id", payload.JobID), slog.Int("attempt", attempt), slog.Int("max_retries", maxRetries))

		result, lastErr = handler.PerformIntegratedEvaluation(evalCtx, cvUpload.Text, projectUpload.Text, payload.JobDescription, payload.StudyCaseBrief, payload.ScoringRubric, payload.JobID)
		if lastErr == nil {
			lg.Info("evaluation succeeded", slog.String("job_id", payload.JobID), slog.Int("attempt", attempt))
			break
		}

		lg.Warn("evaluation attempt failed",
			slog.String("job_id", payload.JobID),
			slog.Int("attempt", attempt),
			slog.Int("max_retries", maxRetries),
			slog.Any("error", lastErr))

		// If this is not the last attempt, wait before retrying
		if attempt < maxRetries {
			backoffDuration := time.Duration(attempt) * 2 * time.Second
			lg.Info("waiting before retry", slog.String("job_id", payload.JobID), slog.Duration("backoff", backoffDuration))
			time.Sleep(backoffDuration)
		}
	}

	if lastErr != nil {
		lg.Error("enhanced evaluation failed after all retries",
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
		code := classifyFailureCode(msg)
		adapterobs.RecordJobFailureByCode("evaluate", code)
		slog.Info("job failure recorded",
			slog.String("job_id", payload.JobID),
			slog.String("error_code", code))
		return fmt.Errorf("enhanced evaluation failed after %d attempts: %w", maxRetries, lastErr)
	}

	// Store the result FIRST
	lg.Info("storing evaluation result", slog.String("job_id", payload.JobID))
	if err := results.Upsert(ctx, result); err != nil {
		lg.Error("failed to store result", slog.String("job_id", payload.JobID), slog.Any("error", err))
		failMsg := "failed to store evaluation result"
		if jobs != nil {
			if errStatus := jobs.UpdateStatus(ctx, payload.JobID, domain.JobFailed, &failMsg); errStatus != nil {
				lg.Error("failed to update job status to failed after store error", slog.String("job_id", payload.JobID), slog.Any("error", errStatus))
			}
		}
		adapterobs.RecordJobFailureByCode("evaluate", classifyFailureCode(failMsg))
		return fmt.Errorf("store result: %w", err)
	}
	lg.Info("evaluation result stored successfully", slog.String("job_id", payload.JobID))

	// Update job status to completed AFTER storing result
	lg.Info("updating job status to completed", slog.String("job_id", payload.JobID))
	if err := jobs.UpdateStatus(ctx, payload.JobID, domain.JobCompleted, nil); err != nil {
		lg.Error("failed to update job status to completed", slog.String("job_id", payload.JobID), slog.Any("error", err))
		failMsg := "failed to mark job as completed after evaluation"
		if jobs != nil {
			if errStatus := jobs.UpdateStatus(ctx, payload.JobID, domain.JobFailed, &failMsg); errStatus != nil {
				lg.Error("failed to update job status to failed after completion error", slog.String("job_id", payload.JobID), slog.Any("error", errStatus))
			}
		}
		adapterobs.RecordJobFailureByCode("evaluate", classifyFailureCode(failMsg))
		return fmt.Errorf("update job status: %w", err)
	}
	lg.Info("job status updated to completed successfully", slog.String("job_id", payload.JobID))
	success = true
	lg.Info("job completed",
		slog.String("job_id", payload.JobID),
		slog.Duration("processing_duration", time.Since(start)))
	return nil
}

// ptr returns a pointer to the given string.
