// Package shared provides shared evaluation logic for queue handlers.
package shared

import (
	"context"
	"fmt"
	"log/slog"

	"go.opentelemetry.io/otel"

	"github.com/fairyhunter13/ai-cv-evaluator/internal/adapter/observability"
	qdrantcli "github.com/fairyhunter13/ai-cv-evaluator/internal/adapter/vector/qdrant"
	"github.com/fairyhunter13/ai-cv-evaluator/internal/domain"
)

// SearchResult represents a search result from vector search.
type SearchResult struct {
	Text  string
	Score float64
}

// HandleEvaluate processes an evaluation task with the given dependencies.
// This is the shared evaluation logic that uses the enhanced AI evaluation system by default.
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

	// Update job status to processing
	if err := jobs.UpdateStatus(ctx, payload.JobID, domain.JobProcessing, nil); err != nil {
		slog.Error("failed to update job status to processing", slog.String("job_id", payload.JobID), slog.Any("error", err))
		return fmt.Errorf("update job status: %w", err)
	}

	// Get CV content
	cvUpload, err := uploads.Get(ctx, payload.CVID)
	if err != nil {
		slog.Error("failed to get CV content", slog.String("job_id", payload.JobID), slog.String("cv_id", payload.CVID), slog.Any("error", err))
		_ = jobs.UpdateStatus(ctx, payload.JobID, domain.JobFailed, ptr("failed to get CV content"))
		return fmt.Errorf("get CV content: %w", err)
	}

	// Get project content
	projectUpload, err := uploads.Get(ctx, payload.ProjectID)
	if err != nil {
		slog.Error("failed to get project content", slog.String("job_id", payload.JobID), slog.String("project_id", payload.ProjectID), slog.Any("error", err))
		_ = jobs.UpdateStatus(ctx, payload.JobID, domain.JobFailed, ptr("failed to get project content"))
		return fmt.Errorf("get project content: %w", err)
	}

	// Perform enhanced AI evaluation with full project.md conformance
	slog.Info("performing enhanced AI evaluation", slog.String("job_id", payload.JobID))
	handler := NewIntegratedEvaluationHandler(ai, q)
	result, err := handler.PerformIntegratedEvaluation(ctx, cvUpload.Text, projectUpload.Text, payload.JobDescription, payload.StudyCaseBrief, payload.ScoringRubric, payload.JobID)
	if err != nil {
		slog.Error("enhanced evaluation failed", slog.String("job_id", payload.JobID), slog.Any("error", err))
		_ = jobs.UpdateStatus(ctx, payload.JobID, domain.JobFailed, ptr("enhanced evaluation failed"))
		return fmt.Errorf("enhanced evaluation: %w", err)
	}

	// Update job status to completed BEFORE storing result
	if err := jobs.UpdateStatus(ctx, payload.JobID, domain.JobCompleted, nil); err != nil {
		slog.Error("failed to update job status to completed", slog.String("job_id", payload.JobID), slog.Any("error", err))
		return fmt.Errorf("update job status: %w", err)
	}

	// Store the result
	if err := results.Upsert(ctx, result); err != nil {
		slog.Error("failed to store result", slog.String("job_id", payload.JobID), slog.Any("error", err))
		return fmt.Errorf("store result: %w", err)
	}
	observability.CompleteJob("evaluate")
	slog.Info("job completed", slog.String("job_id", payload.JobID))
	return nil
}

// ptr returns a pointer to the given string.
func ptr(s string) *string { return &s }
