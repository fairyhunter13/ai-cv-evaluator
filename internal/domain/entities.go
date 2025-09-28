// Package domain defines core entities, ports, and domain-specific errors.
package domain

import (
	"context"
	"errors"
	"time"
)

// Error taxonomy (sentinels)
var (
	ErrInvalidArgument   = errors.New("invalid argument")
	ErrNotFound          = errors.New("not found")
	ErrConflict          = errors.New("conflict")
	ErrRateLimited       = errors.New("rate limited")
	ErrUpstreamTimeout   = errors.New("upstream timeout")
	ErrUpstreamRateLimit = errors.New("upstream rate limit")
	ErrSchemaInvalid     = errors.New("schema invalid")
	ErrInternal          = errors.New("internal error")
)

// UploadType enumerates upload types
// UploadType is a string constant that represents the type of upload.
const (
	// UploadTypeCV is the type for CV uploads.
	UploadTypeCV = "cv"
	// UploadTypeProject is the type for project uploads.
	UploadTypeProject = "project"
)

// Upload represents stored text and metadata for CV or Project
// Invariants: Type in {cv, project}; Size <= Max; Text sanitized and non-empty
//go:generate mockery --name=UploadRepository --with-expecter --filename=upload_repository_mock.go
//go:generate mockery --name=JobRepository --with-expecter --filename=job_repository_mock.go
//go:generate mockery --name=ResultRepository --with-expecter --filename=result_repository_mock.go
//go:generate mockery --name=Queue --with-expecter --filename=queue_mock.go
//go:generate mockery --name=AIClient --with-expecter --filename=aiclient_mock.go
type Upload struct {
	// ID is the unique identifier for the upload.
	ID string
	// Type is the type of upload (cv or project).
	Type string
	// Text is the text content of the upload.
	Text string
	// Filename is the original filename of the upload.
	Filename string
	// MIME is the MIME type of the upload.
	MIME string
	// Size is the size of the upload in bytes.
	Size int64
	// CreatedAt is the timestamp when the upload was created.
	CreatedAt time.Time
}

// JobStatus captures the lifecycle state of an evaluation job.
// JobStatus is a string constant that represents the status of a job.
type JobStatus string

// Job status values.
const (
	// JobQueued is the status when a job is queued.
	JobQueued JobStatus = "queued"
	// JobProcessing is the status when a job is being processed.
	JobProcessing JobStatus = "processing"
	// JobCompleted is the status when a job is completed.
	JobCompleted JobStatus = "completed"
	// JobFailed is the status when a job fails.
	JobFailed JobStatus = "failed"
)

// Job is the domain model for an evaluation job.
type Job struct {
	// ID is the unique identifier for the job.
	ID string
	// Status is the current status of the job.
	Status JobStatus
	// Error is the error message if the job fails.
	Error string
	// CreatedAt is the timestamp when the job was created.
	CreatedAt time.Time
	// UpdatedAt is the timestamp when the job was last updated.
	UpdatedAt time.Time
	// CVID is the ID of the CV associated with the job.
	CVID string
	// ProjectID is the ID of the project associated with the job.
	ProjectID string
	// IdemKey is the idempotency key for the job.
	IdemKey *string
}

// Result stores the evaluation output for a job.
type Result struct {
	// JobID is the ID of the job that produced this result.
	JobID string
	// CVMatchRate is the match rate of the CV.
	CVMatchRate float64 // normalized fraction [0,1]
	// CVFeedback is the feedback for the CV.
	CVFeedback string
	// ProjectScore is the score of the project.
	ProjectScore float64 // [1,10]
	// ProjectFeedback is the feedback for the project.
	ProjectFeedback string
	// OverallSummary is the overall summary of the evaluation.
	OverallSummary string
	// CreatedAt is the timestamp when the result was created.
	CreatedAt time.Time
}

// Repositories (ports)

// UploadRepository is responsible for managing uploads.
type UploadRepository interface {
	// Create creates a new upload.
	Create(ctx Context, u Upload) (string, error)
	// Get retrieves an upload by ID.
	Get(ctx Context, id string) (Upload, error)
}

// JobRepository is responsible for managing jobs.
type JobRepository interface {
	// Create creates a new job.
	Create(ctx Context, j Job) (string, error)
	// UpdateStatus updates the status of a job.
	UpdateStatus(ctx Context, id string, status JobStatus, errMsg *string) error
	// Get retrieves a job by ID.
	Get(ctx Context, id string) (Job, error)
	// FindByIdempotencyKey finds a job by idempotency key.
	FindByIdempotencyKey(ctx Context, key string) (Job, error)
}

// ResultRepository is responsible for managing results.
type ResultRepository interface {
	// Upsert upserts a result.
	Upsert(ctx Context, r Result) error
	// GetByJobID retrieves a result by job ID.
	GetByJobID(ctx Context, jobID string) (Result, error)
}

// Queue (port)

// Queue is responsible for enqueuing tasks.
type Queue interface {
	// EnqueueEvaluate enqueues an evaluate task.
	EnqueueEvaluate(ctx Context, payload EvaluateTaskPayload) (string, error)
}

// AIClient (port)

// AIClient abstracts the AI provider used for embedding and chat JSON operations.
type AIClient interface {
	// Embed returns embedding vectors for texts; deterministic in mock mode
	Embed(ctx Context, texts []string) ([][]float32, error)
	// ChatJSON returns a JSON strictly matching provided schema instruction; deterministic in mock mode
	ChatJSON(ctx Context, systemPrompt, userPrompt string, maxTokens int) (string, error)
}

// TextExtractor (port)
// ExtractPath extracts text from a file at path with provided original filename.
// Implementations may call external services (e.g., Tika) or use local libraries.
type TextExtractor interface {
	// ExtractPath extracts text from a file at path with provided original filename.
	ExtractPath(ctx Context, fileName, path string) (string, error)
}

// EvaluateTaskPayload is the payload for the evaluate job enqueued to the background worker.
type EvaluateTaskPayload struct {
	JobID           string
	CVID            string
	ProjectID       string
	JobDescription  string
	StudyCaseBrief  string
}

// Context is an alias to allow decoupling from std context in domain
// Adapters and usecases should pass context.Context through
// Avoid importing std context here to keep domain pure if desired
// For practicality, we alias to any interface with Done/Err/Deadline methods via stdlib.
// But simplest: use type alias to context.Context; adapters convert where needed.

// Context is a type alias to stdlib context.Context for convenience across layers.
type Context = context.Context
