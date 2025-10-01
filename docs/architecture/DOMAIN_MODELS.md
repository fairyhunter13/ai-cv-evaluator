# Domain Models

This document describes the domain models and entities in the AI CV Evaluator system.

## Overview

The domain layer follows Clean Architecture principles and contains the core business logic, entities, and interfaces that define the system's behavior.

## Domain Entities

### 1. Job Entity

Represents an evaluation job in the system.

```go
type Job struct {
    ID          string    `json:"id"`
    Status      JobStatus `json:"status"`
    CreatedAt   time.Time `json:"created_at"`
    UpdatedAt   time.Time `json:"updated_at"`
    CompletedAt *time.Time `json:"completed_at,omitempty"`
}
```

#### Job Status Enum
```go
type JobStatus string

const (
    JobQueued     JobStatus = "queued"
    JobProcessing JobStatus = "processing"
    JobCompleted  JobStatus = "completed"
    JobFailed     JobStatus = "failed"
)
```

#### Job Status Transitions
```mermaid
stateDiagram-v2
    [*] --> Queued
    Queued --> Processing
    Processing --> Completed
    Processing --> Failed
    Completed --> [*]
    Failed --> [*]
```

### 2. Upload Entity

Represents uploaded files (CV and project).

```go
type Upload struct {
    ID        string    `json:"id"`
    Content   string    `json:"content"`
    Filename  string    `json:"filename"`
    CreatedAt time.Time `json:"created_at"`
}
```

### 3. Result Entity

Represents the evaluation result.

```go
type Result struct {
    JobID           string    `json:"job_id"`
    CVMatchRate     float64   `json:"cv_match_rate"`
    CVFeedback      string    `json:"cv_feedback"`
    ProjectScore    float64   `json:"project_score"`
    ProjectFeedback string    `json:"project_feedback"`
    OverallSummary  string    `json:"overall_summary"`
    CreatedAt       time.Time `json:"created_at"`
}
```

## Domain Interfaces (Ports)

### 1. Repository Interfaces

#### JobRepository
```go
type JobRepository interface {
    Upsert(ctx Context, job Job) error
    GetByID(ctx Context, id string) (Job, error)
    GetByStatus(ctx Context, status JobStatus) ([]Job, error)
    UpdateStatus(ctx Context, id string, status JobStatus) error
    GetAverageProcessingTime(ctx Context) (float64, error)
}
```

#### UploadRepository
```go
type UploadRepository interface {
    Upsert(ctx Context, upload Upload) error
    GetByID(ctx Context, id string) (Upload, error)
    Ingest(ctx Context, cvContent, projectContent, cvFilename, projectFilename string) (cvID, projectID string, err error)
}
```

#### ResultRepository
```go
type ResultRepository interface {
    Upsert(ctx Context, result Result) error
    GetByJobID(ctx Context, jobID string) (Result, error)
}
```

### 2. External Service Interfaces

#### AIClient
```go
type AIClient interface {
    Embed(ctx Context, texts []string) ([][]float32, error)
    ChatJSON(ctx Context, systemPrompt, userPrompt string, maxTokens int) (string, error)
    CleanCoTResponse(ctx Context, response string) (string, error)
}
```

#### TextExtractor
```go
type TextExtractor interface {
    ExtractPath(ctx Context, fileName, path string) (string, error)
}
```

#### Queue
```go
type Queue interface {
    EnqueueEvaluate(ctx Context, payload EvaluateTaskPayload) (string, error)
}
```

## Domain Services

### 1. Evaluation Service

Core business logic for evaluation processing.

```go
type EvaluationService struct {
    jobRepo    JobRepository
    uploadRepo UploadRepository
    resultRepo ResultRepository
    aiClient   AIClient
    queue      Queue
}
```

#### Key Methods
```go
func (s *EvaluationService) EnqueueEvaluation(
    ctx Context,
    cvID, projectID, jobDescription, studyCase, scoringRubric string,
) (string, error)

func (s *EvaluationService) ProcessEvaluation(
    ctx Context,
    jobID string,
    cvContent, projectContent, jobDescription, studyCase, scoringRubric string,
) (Result, error)
```

### 2. Upload Service

Handles file upload and processing.

```go
type UploadService struct {
    uploadRepo    UploadRepository
    textExtractor TextExtractor
}
```

#### Key Methods
```go
func (s *UploadService) ProcessUpload(
    ctx Context,
    cvContent, projectContent, cvFilename, projectFilename string,
) (cvID, projectID string, error)
```

## Domain Value Objects

### 1. EvaluateTaskPayload

Payload for evaluation tasks.

```go
type EvaluateTaskPayload struct {
    JobID          string `json:"job_id"`
    CVID           string `json:"cv_id"`
    ProjectID      string `json:"project_id"`
    JobDescription string `json:"job_description"`
    StudyCaseBrief string `json:"study_case_brief"`
    ScoringRubric  string `json:"scoring_rubric"`
}
```

### 2. EvaluationResult

Result of the evaluation process.

```go
type EvaluationResult struct {
    CVMatchRate     float64 `json:"cv_match_rate"`
    CVFeedback      string  `json:"cv_feedback"`
    ProjectScore    float64 `json:"project_score"`
    ProjectFeedback string  `json:"project_feedback"`
    OverallSummary  string  `json:"overall_summary"`
}
```

## Domain Errors

### Error Types
```go
var (
    ErrInvalidArgument   = errors.New("invalid argument")
    ErrNotFound         = errors.New("not found")
    ErrConflict         = errors.New("conflict")
    ErrRateLimited      = errors.New("rate limited")
    ErrUpstreamTimeout  = errors.New("upstream timeout")
    ErrUpstreamRateLimit = errors.New("upstream rate limit")
    ErrSchemaInvalid    = errors.New("schema invalid")
)
```

### Error Handling
```go
func (s *EvaluationService) ProcessEvaluation(ctx Context, jobID string, ...) (Result, error) {
    // Validate inputs
    if jobID == "" {
        return Result{}, fmt.Errorf("%w: job ID required", ErrInvalidArgument)
    }
    
    // Process evaluation
    result, err := s.performEvaluation(ctx, ...)
    if err != nil {
        return Result{}, fmt.Errorf("evaluation failed: %w", err)
    }
    
    return result, nil
}
```

## Domain Rules and Business Logic

### 1. Job Status Rules
- Jobs can only transition to valid states
- Completed jobs cannot be modified
- Failed jobs can be retried
- Processing jobs have timeout limits

### 2. Evaluation Rules
- CV match rate must be between 0.0 and 1.0
- Project score must be between 0.0 and 10.0
- All feedback fields must be non-empty
- Overall summary must be comprehensive

### 3. Upload Rules
- Files must be valid document types
- File size must be within limits
- Content must be extractable
- Duplicate uploads are handled gracefully

## Domain Events

### 1. Job Events
```go
type JobCreatedEvent struct {
    JobID    string    `json:"job_id"`
    CreatedAt time.Time `json:"created_at"`
}

type JobStatusChangedEvent struct {
    JobID     string    `json:"job_id"`
    OldStatus JobStatus `json:"old_status"`
    NewStatus JobStatus `json:"new_status"`
    ChangedAt time.Time `json:"changed_at"`
}
```

### 2. Evaluation Events
```go
type EvaluationStartedEvent struct {
    JobID     string    `json:"job_id"`
    StartedAt time.Time `json:"started_at"`
}

type EvaluationCompletedEvent struct {
    JobID      string    `json:"job_id"`
    Result     Result    `json:"result"`
    CompletedAt time.Time `json:"completed_at"`
}
```

## Domain Validation

### 1. Entity Validation
```go
func (j *Job) Validate() error {
    if j.ID == "" {
        return fmt.Errorf("job ID is required")
    }
    
    if j.Status == "" {
        return fmt.Errorf("job status is required")
    }
    
    if j.CreatedAt.IsZero() {
        return fmt.Errorf("created at is required")
    }
    
    return nil
}
```

### 2. Business Rule Validation
```go
func (r *Result) Validate() error {
    if r.CVMatchRate < 0.0 || r.CVMatchRate > 1.0 {
        return fmt.Errorf("CV match rate must be between 0.0 and 1.0")
    }
    
    if r.ProjectScore < 0.0 || r.ProjectScore > 10.0 {
        return fmt.Errorf("project score must be between 0.0 and 10.0")
    }
    
    if r.CVFeedback == "" {
        return fmt.Errorf("CV feedback is required")
    }
    
    if r.ProjectFeedback == "" {
        return fmt.Errorf("project feedback is required")
    }
    
    if r.OverallSummary == "" {
        return fmt.Errorf("overall summary is required")
    }
    
    return nil
}
```

## Domain Services Implementation

### 1. Evaluation Service Implementation
```go
func (s *EvaluationService) EnqueueEvaluation(
    ctx Context,
    cvID, projectID, jobDescription, studyCase, scoringRubric string,
) (string, error) {
    // Create job
    job := Job{
        ID:        generateJobID(),
        Status:    JobQueued,
        CreatedAt: time.Now(),
    }
    
    // Save job
    if err := s.jobRepo.Upsert(ctx, job); err != nil {
        return "", fmt.Errorf("failed to create job: %w", err)
    }
    
    // Create evaluation payload
    payload := EvaluateTaskPayload{
        JobID:          job.ID,
        CVID:           cvID,
        ProjectID:      projectID,
        JobDescription: jobDescription,
        StudyCaseBrief: studyCase,
        ScoringRubric:  scoringRubric,
    }
    
    // Enqueue evaluation task
    if err := s.queue.EnqueueEvaluate(ctx, payload); err != nil {
        return "", fmt.Errorf("failed to enqueue evaluation: %w", err)
    }
    
    return job.ID, nil
}
```

### 2. Upload Service Implementation
```go
func (s *UploadService) ProcessUpload(
    ctx Context,
    cvContent, projectContent, cvFilename, projectFilename string,
) (cvID, projectID string, error) {
    // Extract text from CV
    cvText, err := s.textExtractor.ExtractPath(ctx, cvFilename, cvContent)
    if err != nil {
        return "", "", fmt.Errorf("failed to extract CV text: %w", err)
    }
    
    // Extract text from project
    projectText, err := s.textExtractor.ExtractPath(ctx, projectFilename, projectContent)
    if err != nil {
        return "", "", fmt.Errorf("failed to extract project text: %w", err)
    }
    
    // Create upload entities
    cvUpload := Upload{
        ID:        generateUploadID(),
        Content:   cvText,
        Filename:  cvFilename,
        CreatedAt: time.Now(),
    }
    
    projectUpload := Upload{
        ID:        generateUploadID(),
        Content:   projectText,
        Filename:  projectFilename,
        CreatedAt: time.Now(),
    }
    
    // Save uploads
    if err := s.uploadRepo.Upsert(ctx, cvUpload); err != nil {
        return "", "", fmt.Errorf("failed to save CV upload: %w", err)
    }
    
    if err := s.uploadRepo.Upsert(ctx, projectUpload); err != nil {
        return "", "", fmt.Errorf("failed to save project upload: %w", err)
    }
    
    return cvUpload.ID, projectUpload.ID, nil
}
```

## Domain Testing

### 1. Entity Testing
```go
func TestJob_Validate(t *testing.T) {
    tests := []struct {
        name    string
        job     Job
        wantErr bool
    }{
        {
            name: "valid job",
            job: Job{
                ID:        "job-1",
                Status:    JobQueued,
                CreatedAt: time.Now(),
            },
            wantErr: false,
        },
        {
            name: "missing ID",
            job: Job{
                Status:    JobQueued,
                CreatedAt: time.Now(),
            },
            wantErr: true,
        },
    }
    
    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            err := tt.job.Validate()
            if tt.wantErr {
                assert.Error(t, err)
            } else {
                assert.NoError(t, err)
            }
        })
    }
}
```

### 2. Service Testing
```go
func TestEvaluationService_EnqueueEvaluation(t *testing.T) {
    // Setup mocks
    mockJobRepo := &MockJobRepository{}
    mockQueue := &MockQueue{}
    
    service := &EvaluationService{
        jobRepo: mockJobRepo,
        queue:   mockQueue,
    }
    
    // Test enqueue evaluation
    jobID, err := service.EnqueueEvaluation(ctx, "cv-1", "project-1", "job desc", "study case", "rubric")
    
    assert.NoError(t, err)
    assert.NotEmpty(t, jobID)
    assert.True(t, mockJobRepo.UpsertCalled)
    assert.True(t, mockQueue.EnqueueEvaluateCalled)
}
```

## Domain Configuration

### 1. Environment Variables
```bash
# Database configuration
DB_URL=postgres://user:pass@localhost:5432/db

# AI service configuration
OPENAI_API_KEY=your-api-key
OPENAI_BASE_URL=https://api.openai.com/v1

# Queue configuration
KAFKA_BROKERS=localhost:19092

# File processing
TIKA_URL=http://tika:9998
MAX_UPLOAD_MB=10
```

### 2. Domain Constants
```go
const (
    MaxUploadSizeMB = 10
    MaxJobTimeout   = 5 * time.Minute
    DefaultPageSize = 20
    MaxPageSize     = 100
)
```

## Domain Events and Integration

### 1. Event Publishing
```go
type EventPublisher interface {
    Publish(ctx Context, event interface{}) error
}

func (s *EvaluationService) ProcessEvaluation(ctx Context, jobID string, ...) (Result, error) {
    // Update job status
    if err := s.jobRepo.UpdateStatus(ctx, jobID, JobProcessing); err != nil {
        return Result{}, err
    }
    
    // Publish event
    event := JobStatusChangedEvent{
        JobID:     jobID,
        OldStatus: JobQueued,
        NewStatus: JobProcessing,
        ChangedAt: time.Now(),
    }
    
    if err := s.eventPublisher.Publish(ctx, event); err != nil {
        // Log error but don't fail the operation
        slog.Warn("failed to publish event", slog.Any("error", err))
    }
    
    // Continue with evaluation...
}
```

### 2. Event Handling
```go
type EventHandler interface {
    Handle(ctx Context, event interface{}) error
}

type JobStatusChangedHandler struct {
    // Dependencies
}

func (h *JobStatusChangedHandler) Handle(ctx Context, event interface{}) error {
    statusEvent, ok := event.(JobStatusChangedEvent)
    if !ok {
        return fmt.Errorf("invalid event type")
    }
    
    // Handle status change
    switch statusEvent.NewStatus {
    case JobCompleted:
        return h.handleJobCompleted(ctx, statusEvent)
    case JobFailed:
        return h.handleJobFailed(ctx, statusEvent)
    }
    
    return nil
}
```

## Domain Security

### 1. Input Validation
```go
func ValidateJobDescription(desc string) error {
    if len(desc) > 5000 {
        return fmt.Errorf("job description too long")
    }
    
    if strings.TrimSpace(desc) == "" {
        return fmt.Errorf("job description is required")
    }
    
    return nil
}
```

### 2. Access Control
```go
type AccessControl interface {
    CanAccessJob(ctx Context, userID, jobID string) (bool, error)
    CanModifyJob(ctx Context, userID, jobID string) (bool, error)
}
```

## Domain Performance

### 1. Caching
```go
type Cache interface {
    Get(ctx Context, key string) (interface{}, error)
    Set(ctx Context, key string, value interface{}, ttl time.Duration) error
    Delete(ctx Context, key string) error
}
```

### 2. Optimization
```go
func (s *EvaluationService) GetJobWithResult(ctx Context, jobID string) (Job, *Result, error) {
    // Use single query to get job and result
    job, result, err := s.jobRepo.GetWithResult(ctx, jobID)
    if err != nil {
        return Job{}, nil, err
    }
    
    return job, result, nil
}
```

## Domain Documentation

### 1. API Documentation
- OpenAPI specification
- Request/response schemas
- Error codes and messages

### 2. Business Logic Documentation
- Domain rules and constraints
- Business process flows
- Integration patterns

### 3. Testing Documentation
- Unit test coverage
- Integration test scenarios
- Performance test results
