package usecase

import (
	"crypto/sha1"
	"encoding/hex"
	"fmt"
	"time"

	"github.com/fairyhunter13/ai-cv-evaluator/internal/domain"
)

type EvaluateService struct {
	Jobs    domain.JobRepository
	Queue  domain.Queue
	Uploads domain.UploadRepository
}

type ReadinessCheck struct { Name string `json:"name"`; OK bool `json:"ok"`; Details string `json:"details"` }

func NewEvaluateService(j domain.JobRepository, q domain.Queue, u domain.UploadRepository) EvaluateService {
	return EvaluateService{Jobs: j, Queue: q, Uploads: u}
}

func (s EvaluateService) Enqueue(ctx domain.Context, cvID, projectID, jobDesc, studyCase, idemKey string) (string, error) {
	if cvID == "" || projectID == "" { return "", fmt.Errorf("%w: ids required", domain.ErrInvalidArgument) }
	// Idempotency: if provided, try to find an existing job
	if idemKey != "" {
		if j, err := s.Jobs.FindByIdempotencyKey(ctx, idemKey); err == nil && j.ID != "" {
			return j.ID, nil
		}
	}
	// Create job
	j := domain.Job{Status: domain.JobQueued, CVID: cvID, ProjectID: projectID, CreatedAt: time.Now().UTC(), UpdatedAt: time.Now().UTC()}
	if idemKey != "" { j.IdemKey = &idemKey }
	jobID, err := s.Jobs.Create(ctx, j)
	if err != nil { return "", err }
	// Enqueue
	payload := domain.EvaluateTaskPayload{JobID: jobID, CVID: cvID, ProjectID: projectID, JobDescription: jobDesc, StudyCaseBrief: studyCase}
	if _, err := s.Queue.EnqueueEvaluate(ctx, payload); err != nil {
		_ = s.Jobs.UpdateStatus(ctx, jobID, domain.JobFailed, ptr("enqueue failed"))
		return "", err
	}
	return jobID, nil
}

func (s EvaluateService) Readiness(ctx domain.Context) []ReadinessCheck {
	// Placeholder: In real impl, ping DB/Redis/Qdrant
	return []ReadinessCheck{{Name: "db", OK: true}, {Name: "redis", OK: true}, {Name: "qdrant", OK: true}}
}

func hash(s string) string { h := sha1.Sum([]byte(s)); return hex.EncodeToString(h[:]) }

// EvaluationResult is an adapter-usecase DTO for result response.
type EvaluationResult struct {
	CVMatchRate     float64
	CVFeedback      string
	ProjectScore    float64
	ProjectFeedback string
	OverallSummary  string
}

func ptr(s string) *string { return &s }
