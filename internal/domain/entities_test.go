package domain

import (
	"testing"
	"time"
)

func TestUploadTypeConstants(t *testing.T) {
	tests := []struct {
		name     string
		constant string
		expected string
	}{
		{"UploadTypeCV", UploadTypeCV, "cv"},
		{"UploadTypeProject", UploadTypeProject, "project"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.constant != tt.expected {
				t.Errorf("Expected %s to be %q, got %q", tt.name, tt.expected, tt.constant)
			}
		})
	}
}

func TestJobStatusConstants(t *testing.T) {
	tests := []struct {
		name     string
		constant JobStatus
		expected string
	}{
		{"JobQueued", JobQueued, "queued"},
		{"JobProcessing", JobProcessing, "processing"},
		{"JobCompleted", JobCompleted, "completed"},
		{"JobFailed", JobFailed, "failed"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if string(tt.constant) != tt.expected {
				t.Errorf("Expected %s to be %q, got %q", tt.name, tt.expected, string(tt.constant))
			}
		})
	}
}

func TestUpload(t *testing.T) {
	now := time.Now()
	upload := Upload{
		ID:        "test-id",
		Type:      UploadTypeCV,
		Text:      "test content",
		Filename:  "test.txt",
		MIME:      "text/plain",
		Size:      1024,
		CreatedAt: now,
	}

	if upload.ID != "test-id" {
		t.Errorf("Expected ID to be 'test-id', got %q", upload.ID)
	}
	if upload.Type != UploadTypeCV {
		t.Errorf("Expected Type to be %q, got %q", UploadTypeCV, upload.Type)
	}
	if upload.Text != "test content" {
		t.Errorf("Expected Text to be 'test content', got %q", upload.Text)
	}
	if upload.Filename != "test.txt" {
		t.Errorf("Expected Filename to be 'test.txt', got %q", upload.Filename)
	}
	if upload.MIME != "text/plain" {
		t.Errorf("Expected MIME to be 'text/plain', got %q", upload.MIME)
	}
	if upload.Size != 1024 {
		t.Errorf("Expected Size to be 1024, got %d", upload.Size)
	}
	if !upload.CreatedAt.Equal(now) {
		t.Errorf("Expected CreatedAt to be %v, got %v", now, upload.CreatedAt)
	}
}

func TestJob(t *testing.T) {
	now := time.Now()
	idemKey := "test-key"
	job := Job{
		ID:        "job-123",
		Status:    JobQueued,
		Error:     "",
		CreatedAt: now,
		UpdatedAt: now,
		CVID:      "cv-456",
		ProjectID: "proj-789",
		IdemKey:   &idemKey,
	}

	if job.ID != "job-123" {
		t.Errorf("Expected ID to be 'job-123', got %q", job.ID)
	}
	if job.Status != JobQueued {
		t.Errorf("Expected Status to be %q, got %q", JobQueued, job.Status)
	}
	if job.Error != "" {
		t.Errorf("Expected Error to be empty, got %q", job.Error)
	}
	if !job.CreatedAt.Equal(now) {
		t.Errorf("Expected CreatedAt to be %v, got %v", now, job.CreatedAt)
	}
	if !job.UpdatedAt.Equal(now) {
		t.Errorf("Expected UpdatedAt to be %v, got %v", now, job.UpdatedAt)
	}
	if job.CVID != "cv-456" {
		t.Errorf("Expected CVID to be 'cv-456', got %q", job.CVID)
	}
	if job.ProjectID != "proj-789" {
		t.Errorf("Expected ProjectID to be 'proj-789', got %q", job.ProjectID)
	}
	if job.IdemKey == nil || *job.IdemKey != "test-key" {
		t.Errorf("Expected IdemKey to be 'test-key', got %v", job.IdemKey)
	}
}

func TestJobWithError(t *testing.T) {
	now := time.Now()
	job := Job{
		ID:        "job-123",
		Status:    JobFailed,
		Error:     "test error",
		CreatedAt: now,
		UpdatedAt: now,
		CVID:      "cv-456",
		ProjectID: "proj-789",
		IdemKey:   nil,
	}

	if job.Status != JobFailed {
		t.Errorf("Expected Status to be %q, got %q", JobFailed, job.Status)
	}
	if job.Error != "test error" {
		t.Errorf("Expected Error to be 'test error', got %q", job.Error)
	}
	if job.IdemKey != nil {
		t.Errorf("Expected IdemKey to be nil, got %v", job.IdemKey)
	}
}

func TestResult(t *testing.T) {
	now := time.Now()
	result := Result{
		JobID:           "job-123",
		CVMatchRate:     0.85,
		CVFeedback:      "Good CV",
		ProjectScore:    8.5,
		ProjectFeedback: "Good project",
		OverallSummary:  "Overall good candidate",
		CreatedAt:       now,
	}

	if result.JobID != "job-123" {
		t.Errorf("Expected JobID to be 'job-123', got %q", result.JobID)
	}
	if result.CVMatchRate != 0.85 {
		t.Errorf("Expected CVMatchRate to be 0.85, got %f", result.CVMatchRate)
	}
	if result.CVFeedback != "Good CV" {
		t.Errorf("Expected CVFeedback to be 'Good CV', got %q", result.CVFeedback)
	}
	if result.ProjectScore != 8.5 {
		t.Errorf("Expected ProjectScore to be 8.5, got %f", result.ProjectScore)
	}
	if result.ProjectFeedback != "Good project" {
		t.Errorf("Expected ProjectFeedback to be 'Good project', got %q", result.ProjectFeedback)
	}
	if result.OverallSummary != "Overall good candidate" {
		t.Errorf("Expected OverallSummary to be 'Overall good candidate', got %q", result.OverallSummary)
	}
	if !result.CreatedAt.Equal(now) {
		t.Errorf("Expected CreatedAt to be %v, got %v", now, result.CreatedAt)
	}
}

func TestEvaluateTaskPayload(t *testing.T) {
	payload := EvaluateTaskPayload{
		JobID:          "job-123",
		CVID:           "cv-456",
		ProjectID:      "proj-789",
		JobDescription: "Software Engineer",
		StudyCaseBrief: "Build a web app",
	}

	if payload.JobID != "job-123" {
		t.Errorf("Expected JobID to be 'job-123', got %q", payload.JobID)
	}
	if payload.CVID != "cv-456" {
		t.Errorf("Expected CVID to be 'cv-456', got %q", payload.CVID)
	}
	if payload.ProjectID != "proj-789" {
		t.Errorf("Expected ProjectID to be 'proj-789', got %q", payload.ProjectID)
	}
	if payload.JobDescription != "Software Engineer" {
		t.Errorf("Expected JobDescription to be 'Software Engineer', got %q", payload.JobDescription)
	}
	if payload.StudyCaseBrief != "Build a web app" {
		t.Errorf("Expected StudyCaseBrief to be 'Build a web app', got %q", payload.StudyCaseBrief)
	}
}
