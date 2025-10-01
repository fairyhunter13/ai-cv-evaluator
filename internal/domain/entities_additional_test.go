package domain

import (
	"testing"
	"time"
)

func TestUpload_EdgeCases(t *testing.T) {
	// Test with zero values
	upload := Upload{}
	if upload.ID != "" {
		t.Errorf("Expected empty ID, got %q", upload.ID)
	}
	if upload.Type != "" {
		t.Errorf("Expected empty Type, got %q", upload.Type)
	}
	if upload.Text != "" {
		t.Errorf("Expected empty Text, got %q", upload.Text)
	}
	if upload.Filename != "" {
		t.Errorf("Expected empty Filename, got %q", upload.Filename)
	}
	if upload.MIME != "" {
		t.Errorf("Expected empty MIME, got %q", upload.MIME)
	}
	if upload.Size != 0 {
		t.Errorf("Expected zero Size, got %d", upload.Size)
	}
	if !upload.CreatedAt.IsZero() {
		t.Errorf("Expected zero CreatedAt, got %v", upload.CreatedAt)
	}
}

func TestJob_EdgeCases(t *testing.T) {
	// Test with zero values
	job := Job{}
	if job.ID != "" {
		t.Errorf("Expected empty ID, got %q", job.ID)
	}
	if job.Status != "" {
		t.Errorf("Expected empty Status, got %q", job.Status)
	}
	if job.Error != "" {
		t.Errorf("Expected empty Error, got %q", job.Error)
	}
	if !job.CreatedAt.IsZero() {
		t.Errorf("Expected zero CreatedAt, got %v", job.CreatedAt)
	}
	if !job.UpdatedAt.IsZero() {
		t.Errorf("Expected zero UpdatedAt, got %v", job.UpdatedAt)
	}
	if job.CVID != "" {
		t.Errorf("Expected empty CVID, got %q", job.CVID)
	}
	if job.ProjectID != "" {
		t.Errorf("Expected empty ProjectID, got %q", job.ProjectID)
	}
	if job.IdemKey != nil {
		t.Errorf("Expected nil IdemKey, got %v", job.IdemKey)
	}
}

func TestResult_EdgeCases(t *testing.T) {
	// Test with zero values
	result := Result{}
	if result.JobID != "" {
		t.Errorf("Expected empty JobID, got %q", result.JobID)
	}
	if result.CVMatchRate != 0 {
		t.Errorf("Expected zero CVMatchRate, got %f", result.CVMatchRate)
	}
	if result.CVFeedback != "" {
		t.Errorf("Expected empty CVFeedback, got %q", result.CVFeedback)
	}
	if result.ProjectScore != 0 {
		t.Errorf("Expected zero ProjectScore, got %f", result.ProjectScore)
	}
	if result.ProjectFeedback != "" {
		t.Errorf("Expected empty ProjectFeedback, got %q", result.ProjectFeedback)
	}
	if result.OverallSummary != "" {
		t.Errorf("Expected empty OverallSummary, got %q", result.OverallSummary)
	}
	if !result.CreatedAt.IsZero() {
		t.Errorf("Expected zero CreatedAt, got %v", result.CreatedAt)
	}
}

func TestEvaluateTaskPayload_EdgeCases(t *testing.T) {
	// Test with zero values
	payload := EvaluateTaskPayload{}
	if payload.JobID != "" {
		t.Errorf("Expected empty JobID, got %q", payload.JobID)
	}
	if payload.CVID != "" {
		t.Errorf("Expected empty CVID, got %q", payload.CVID)
	}
	if payload.ProjectID != "" {
		t.Errorf("Expected empty ProjectID, got %q", payload.ProjectID)
	}
	if payload.JobDescription != "" {
		t.Errorf("Expected empty JobDescription, got %q", payload.JobDescription)
	}
	if payload.StudyCaseBrief != "" {
		t.Errorf("Expected empty StudyCaseBrief, got %q", payload.StudyCaseBrief)
	}
}

func TestJobStatus_StringConversion(t *testing.T) {
	tests := []struct {
		status   JobStatus
		expected string
	}{
		{JobQueued, "queued"},
		{JobProcessing, "processing"},
		{JobCompleted, "completed"},
		{JobFailed, "failed"},
		{"", ""},
		{"custom", "custom"},
	}

	for _, tt := range tests {
		t.Run(string(tt.status), func(t *testing.T) {
			if string(tt.status) != tt.expected {
				t.Errorf("Expected %q, got %q", tt.expected, string(tt.status))
			}
		})
	}
}

func TestUploadType_StringConversion(t *testing.T) {
	tests := []struct {
		uploadType string
		expected   string
	}{
		{UploadTypeCV, "cv"},
		{UploadTypeProject, "project"},
		{"", ""},
		{"custom", "custom"},
	}

	for _, tt := range tests {
		t.Run(tt.uploadType, func(t *testing.T) {
			if tt.uploadType != tt.expected {
				t.Errorf("Expected %q, got %q", tt.expected, tt.uploadType)
			}
		})
	}
}

func TestUpload_WithNilIdemKey(t *testing.T) {
	now := time.Now()
	job := Job{
		ID:        "job-123",
		Status:    JobQueued,
		Error:     "",
		CreatedAt: now,
		UpdatedAt: now,
		CVID:      "cv-456",
		ProjectID: "proj-789",
		IdemKey:   nil, // Explicitly nil
	}

	if job.IdemKey != nil {
		t.Errorf("Expected nil IdemKey, got %v", job.IdemKey)
	}
}

func TestResult_WithFloatValues(t *testing.T) {
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

	// Test that float values are preserved
	if result.CVMatchRate != 0.85 {
		t.Errorf("Expected CVMatchRate to be 0.85, got %f", result.CVMatchRate)
	}
	if result.ProjectScore != 8.5 {
		t.Errorf("Expected ProjectScore to be 8.5, got %f", result.ProjectScore)
	}
}
