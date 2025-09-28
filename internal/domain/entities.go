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
const (
	UploadTypeCV      = "cv"
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
	ID        string
	Type      string
	Text      string
	Filename  string
	MIME      string
	Size      int64
	CreatedAt time.Time
}

type JobStatus string

const (
	JobQueued     JobStatus = "queued"
	JobProcessing JobStatus = "processing"
	JobCompleted  JobStatus = "completed"
	JobFailed     JobStatus = "failed"
)

type Job struct {
	ID         string
	Status     JobStatus
	Error      string
	CreatedAt  time.Time
	UpdatedAt  time.Time
	CVID       string
	ProjectID  string
	IdemKey    *string
}

type Result struct {
	JobID            string
	CVMatchRate      float64 // normalized fraction [0,1]
	CVFeedback       string
	ProjectScore     float64 // [1,10]
	ProjectFeedback  string
	OverallSummary   string
	CreatedAt        time.Time
}

// Repositories (ports)

type UploadRepository interface {
	Create(ctx Context, u Upload) (string, error)
	Get(ctx Context, id string) (Upload, error)
}

type JobRepository interface {
	Create(ctx Context, j Job) (string, error)
	UpdateStatus(ctx Context, id string, status JobStatus, errMsg *string) error
	Get(ctx Context, id string) (Job, error)
	FindByIdempotencyKey(ctx Context, key string) (Job, error)
}

type ResultRepository interface {
	Upsert(ctx Context, r Result) error
	GetByJobID(ctx Context, jobID string) (Result, error)
}

// Queue (port)

type Queue interface {
	EnqueueEvaluate(ctx Context, payload EvaluateTaskPayload) (string, error)
}

// AIClient (port)

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
	ExtractPath(ctx Context, fileName, path string) (string, error)
}

// EvaluateTaskPayload

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

type Context = context.Context
