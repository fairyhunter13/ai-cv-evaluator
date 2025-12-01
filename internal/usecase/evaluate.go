// Package usecase contains application business logic services.
package usecase

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"log/slog"
	"time"

	"github.com/fairyhunter13/ai-cv-evaluator/internal/domain"
	obsctx "github.com/fairyhunter13/ai-cv-evaluator/internal/observability"
	"go.opentelemetry.io/otel"
)

// EvaluateService orchestrates job creation and queueing for evaluation.
type EvaluateService struct {
	Jobs    domain.JobRepository
	Queue   domain.Queue
	Uploads domain.UploadRepository
	AI      domain.AIClient
	Vector  VectorDBHealthChecker
}

// VectorDBHealthChecker interface for checking vector database health
type VectorDBHealthChecker interface {
	// Ping checks if the vector database is accessible
	Ping(ctx domain.Context) error
}

// ReadinessCheck represents a single readiness probe result used by handlers.
type ReadinessCheck struct {
	Name    string `json:"name"`
	OK      bool   `json:"ok"`
	Details string `json:"details"`
}

// NewEvaluateService constructs an EvaluateService with its dependencies.
func NewEvaluateService(j domain.JobRepository, q domain.Queue, u domain.UploadRepository) EvaluateService {
	return EvaluateService{Jobs: j, Queue: q, Uploads: u}
}

// NewEvaluateServiceWithHealthChecks constructs an EvaluateService with health check dependencies.
func NewEvaluateServiceWithHealthChecks(j domain.JobRepository, q domain.Queue, u domain.UploadRepository, ai domain.AIClient, vector VectorDBHealthChecker) EvaluateService {
	return EvaluateService{Jobs: j, Queue: q, Uploads: u, AI: ai, Vector: vector}
}

// Enqueue validates inputs, creates a job, and enqueues the evaluation task.
func (s EvaluateService) Enqueue(ctx domain.Context, cvID, projectID, jobDesc, studyCase, scoringRubric, idemKey string) (string, error) {
	tr := otel.Tracer("usecase.evaluate")
	ctx, span := tr.Start(ctx, "EvaluateService.Enqueue")
	defer span.End()

	lg := obsctx.LoggerFromContext(ctx)
	lg.Info("enqueue evaluate request",
		slog.String("cv_id", cvID),
		slog.String("project_id", projectID),
		slog.String("idempotency_key", idemKey),
		slog.String("request_id", obsctx.RequestIDFromContext(ctx)))

	if cvID == "" || projectID == "" {
		lg.Error("enqueue evaluate missing ids", slog.String("cv_id", cvID), slog.String("project_id", projectID))
		return "", fmt.Errorf("%w: ids required", domain.ErrInvalidArgument)
	}
	// Idempotency: if provided, try to find an existing job
	if idemKey != "" {
		if j, err := s.Jobs.FindByIdempotencyKey(ctx, idemKey); err == nil && j.ID != "" {
			lg.Info("enqueue evaluate idempotent hit", slog.String("job_id", j.ID), slog.String("cv_id", cvID), slog.String("project_id", projectID))
			return j.ID, nil
		}
	}
	// Create job
	j := domain.Job{Status: domain.JobQueued, CVID: cvID, ProjectID: projectID, CreatedAt: time.Now().UTC(), UpdatedAt: time.Now().UTC()}
	if idemKey != "" {
		j.IdemKey = &idemKey
	}
	jobID, err := s.Jobs.Create(ctx, j)
	if err != nil {
		lg.Error("enqueue evaluate failed to create job", slog.Any("error", err), slog.String("cv_id", cvID), slog.String("project_id", projectID))
		return "", err
	}
	lg.Info("enqueue evaluate job created", slog.String("job_id", jobID), slog.String("cv_id", cvID), slog.String("project_id", projectID))
	// Enqueue, propagating request_id to the background worker via payload
	requestID := obsctx.RequestIDFromContext(ctx)
	payload := domain.EvaluateTaskPayload{JobID: jobID, CVID: cvID, ProjectID: projectID, JobDescription: jobDesc, StudyCaseBrief: studyCase, ScoringRubric: scoringRubric, RequestID: requestID}
	if _, err := s.Queue.EnqueueEvaluate(ctx, payload); err != nil {
		_ = s.Jobs.UpdateStatus(ctx, jobID, domain.JobFailed, ptr("enqueue failed"))
		lg.Error("enqueue evaluate failed to enqueue", slog.String("job_id", jobID), slog.Any("error", err))
		return "", err
	}
	lg.Info("enqueue evaluate enqueued", slog.String("job_id", jobID))
	return jobID, nil
}

// Readiness returns comprehensive readiness checks for all dependencies.
func (s EvaluateService) Readiness(ctx domain.Context) []ReadinessCheck {
	checks := []ReadinessCheck{}

	// Check database connectivity
	dbCheck := ReadinessCheck{Name: "database", OK: false, Details: "Database connection check"}
	if s.Jobs != nil {
		if _, err := s.Jobs.Count(ctx); err != nil {
			dbCheck.Details = fmt.Sprintf("Database error: %v", err)
		} else {
			dbCheck.OK = true
			dbCheck.Details = "Database connection successful"
		}
	} else {
		dbCheck.Details = "Database not configured"
	}
	checks = append(checks, dbCheck)

	// Check AI service connectivity
	aiCheck := ReadinessCheck{Name: "ai_service", OK: false, Details: "AI service connection check"}
	if s.AI != nil {
		// Test AI service with a simple embedding request
		_, err := s.AI.Embed(ctx, []string{"health check"})
		if err != nil {
			aiCheck.Details = fmt.Sprintf("AI service error: %v", err)
		} else {
			aiCheck.OK = true
			aiCheck.Details = "AI service connection successful"
		}
	} else {
		aiCheck.Details = "AI service not configured"
	}
	checks = append(checks, aiCheck)

	// Check vector database connectivity
	vectorCheck := ReadinessCheck{Name: "vector_database", OK: false, Details: "Vector database connection check"}
	if s.Vector != nil {
		if err := s.Vector.Ping(ctx); err != nil {
			vectorCheck.Details = fmt.Sprintf("Vector database error: %v", err)
		} else {
			vectorCheck.OK = true
			vectorCheck.Details = "Vector database connection successful"
		}
	} else {
		vectorCheck.Details = "Vector database not configured"
	}
	checks = append(checks, vectorCheck)

	// Check upload repository
	uploadCheck := ReadinessCheck{Name: "upload_repository", OK: false, Details: "Upload repository check"}
	if s.Uploads != nil {
		if _, err := s.Uploads.Count(ctx); err != nil {
			uploadCheck.Details = fmt.Sprintf("Upload repository error: %v", err)
		} else {
			uploadCheck.OK = true
			uploadCheck.Details = "Upload repository connection successful"
		}
	} else {
		uploadCheck.Details = "Upload repository not configured"
	}
	checks = append(checks, uploadCheck)

	return checks
}

func hash(s string) string { h := sha256.Sum256([]byte(s)); return hex.EncodeToString(h[:]) }

// EvaluationResult is an adapter-usecase DTO for result response.
type EvaluationResult struct {
	CVMatchRate     float64
	CVFeedback      string
	ProjectScore    float64
	ProjectFeedback string
	OverallSummary  string
}

func ptr(s string) *string { return &s }
