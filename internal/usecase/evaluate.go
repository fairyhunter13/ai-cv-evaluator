// Package usecase contains application business logic services.
package usecase

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"time"

	"github.com/fairyhunter13/ai-cv-evaluator/internal/domain"
)

// EvaluateService orchestrates job creation and queueing for evaluation.
type EvaluateService struct {
	Jobs    domain.JobRepository
	Queue   domain.Queue
	Uploads domain.UploadRepository
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

// Enqueue validates inputs, creates a job, and enqueues the evaluation task.
func (s EvaluateService) Enqueue(ctx domain.Context, cvID, projectID, jobDesc, studyCase, scoringRubric, idemKey string) (string, error) {
	if cvID == "" || projectID == "" {
		return "", fmt.Errorf("%w: ids required", domain.ErrInvalidArgument)
	}
	// Idempotency: if provided, try to find an existing job
	if idemKey != "" {
		if j, err := s.Jobs.FindByIdempotencyKey(ctx, idemKey); err == nil && j.ID != "" {
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
		return "", err
	}
	// Enqueue
	payload := domain.EvaluateTaskPayload{JobID: jobID, CVID: cvID, ProjectID: projectID, JobDescription: jobDesc, StudyCaseBrief: studyCase, ScoringRubric: scoringRubric}
	if _, err := s.Queue.EnqueueEvaluate(ctx, payload); err != nil {
		_ = s.Jobs.UpdateStatus(ctx, jobID, domain.JobFailed, ptr("enqueue failed"))
		return "", err
	}
	return jobID, nil
}

// Readiness returns static readiness checks; actual external checks are in internal/app.
func (s EvaluateService) Readiness(_ domain.Context) []ReadinessCheck {
	// Placeholder: In real impl, ping DB/Qdrant
	return []ReadinessCheck{{Name: "db", OK: true}, {Name: "qdrant", OK: true}}
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
