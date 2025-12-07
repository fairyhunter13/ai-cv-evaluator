package ai

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	domainmocks "github.com/fairyhunter13/ai-cv-evaluator/internal/domain/mocks"
)

func TestResponseValidator_PerformRefusalDetection_RefusalPath(t *testing.T) {
	mockAI := domainmocks.NewMockAIClient(t)
	analysisJSON := `{"is_refusal": true, "confidence": 0.9, "refusal_type": "policy_violation", "reason": "policy", "suggestions":["s1"]}`
	mockAI.On("ChatJSON", mock.Anything, "", mock.Anything, 500).
		Return(analysisJSON, nil).Once()

	v := NewResponseValidator(mockAI)
	res := &ValidationResult{}

	require.NoError(t, v.performRefusalDetection(context.Background(), "some response", res))
	assert.True(t, res.IsRefusal)
	if assert.NotNil(t, res.RefusalAnalysis) {
		assert.Equal(t, "policy_violation", res.RefusalAnalysis.RefusalType)
	}
	// Should record a critical issue
	found := false
	for _, iss := range res.Issues {
		if iss.Type == "refusal_detected" {
			found = true
		}
	}
	assert.True(t, found)
}

func TestResponseValidator_PerformResponseCleaning_FallbackPath(t *testing.T) {
	mockAI := domainmocks.NewMockAIClient(t)
	v := NewResponseValidator(mockAI)

	// This input will not become valid JSON after cleaning, exercising the
	// CleanAndValidateJSON error path and fallback to CleanJSONResponse.
	res := &ValidationResult{}
	err := v.performResponseCleaning("no json here", res)
	require.NoError(t, err)
	assert.NotEmpty(t, res.CleanedResponse)
}

func TestResponseValidator_ValidateResponse_AllPaths(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name          string
		response      string
		mockSetup     func(*domainmocks.MockAIClient)
		expectValid   bool
		expectRefusal bool
		expectIssues  bool
	}{
		{
			name:     "valid_json_response",
			response: `{"name": "test", "value": 123}`,
			mockSetup: func(ai *domainmocks.MockAIClient) {
				ai.On("ChatJSON", mock.Anything, "", mock.Anything, 500).
					Return(`{"is_refusal": false, "confidence": 0.1}`, nil).Maybe()
			},
			expectValid:   true,
			expectRefusal: false,
			expectIssues:  false,
		},
		{
			name:     "empty_response",
			response: "",
			mockSetup: func(ai *domainmocks.MockAIClient) {
				ai.On("ChatJSON", mock.Anything, "", mock.Anything, 500).
					Return(`{"is_refusal": false}`, nil).Maybe()
			},
			expectValid:   false,
			expectRefusal: false,
			expectIssues:  true,
		},
		{
			name:     "refusal_response",
			response: "I cannot help with that request.",
			mockSetup: func(ai *domainmocks.MockAIClient) {
				ai.On("ChatJSON", mock.Anything, "", mock.Anything, 500).
					Return(`{"is_refusal": true, "confidence": 0.95, "refusal_type": "policy", "reason": "policy violation"}`, nil).Maybe()
			},
			expectValid:   false,
			expectRefusal: true,
			expectIssues:  true,
		},
		{
			name:     "malformed_json_response",
			response: `{"name": "test", "value": }`,
			mockSetup: func(ai *domainmocks.MockAIClient) {
				ai.On("ChatJSON", mock.Anything, "", mock.Anything, 500).
					Return(`{"is_refusal": false}`, nil).Maybe()
			},
			expectValid:  false,
			expectIssues: true,
		},
		{
			name:     "json_with_markdown",
			response: "```json\n{\"name\": \"test\"}\n```",
			mockSetup: func(ai *domainmocks.MockAIClient) {
				ai.On("ChatJSON", mock.Anything, "", mock.Anything, 500).
					Return(`{"is_refusal": false}`, nil).Maybe()
			},
			expectValid:  true,
			expectIssues: false,
		},
		{
			name:     "ai_error_in_refusal_detection",
			response: `{"valid": true}`,
			mockSetup: func(ai *domainmocks.MockAIClient) {
				ai.On("ChatJSON", mock.Anything, "", mock.Anything, 500).
					Return("", assert.AnError).Maybe()
			},
			expectValid:  true,
			expectIssues: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockAI := domainmocks.NewMockAIClient(t)
			tt.mockSetup(mockAI)

			v := NewResponseValidator(mockAI)
			ctx := context.Background()

			result, err := v.ValidateResponse(ctx, tt.response)
			require.NoError(t, err)
			assert.NotNil(t, result)

			if tt.expectRefusal {
				assert.True(t, result.IsRefusal)
			}
			if tt.expectIssues {
				assert.Greater(t, len(result.Issues), 0)
			}
		})
	}
}

func TestResponseValidator_PerformBasicChecks(t *testing.T) {
	t.Parallel()

	mockAI := domainmocks.NewMockAIClient(t)
	v := NewResponseValidator(mockAI)

	tests := []struct {
		name         string
		response     string
		expectIssues bool
	}{
		{"empty", "", true},
		{"whitespace_only", "   \n\t  ", true},
		{"valid_short", "OK", false},
		{"valid_long", "This is a valid response with enough content.", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			res := &ValidationResult{Issues: []ValidationIssue{}}
			err := v.performBasicChecks(tt.response, res)
			require.NoError(t, err)
			if tt.expectIssues {
				assert.Greater(t, len(res.Issues), 0)
			}
		})
	}
}

func TestResponseValidator_PerformJSONValidation(t *testing.T) {
	t.Parallel()

	mockAI := domainmocks.NewMockAIClient(t)
	v := NewResponseValidator(mockAI)

	tests := []struct {
		name        string
		response    string
		expectError bool
	}{
		{"valid_object", `{"key": "value"}`, false},
		{"valid_array", `[1, 2, 3]`, false},
		{"invalid_json", `{invalid}`, true},
		{"empty_object", `{}`, false},
		{"nested_object", `{"a": {"b": {"c": 1}}}`, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			res := &ValidationResult{Issues: []ValidationIssue{}}
			err := v.performJSONValidation(tt.response, res)
			if tt.expectError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestResponseValidator_ValidateResponse_EmptyResponse(t *testing.T) {
	t.Parallel()

	mockAI := domainmocks.NewMockAIClient(t)
	// Mock refusal detection call
	mockAI.On("ChatJSON", mock.Anything, "", mock.Anything, 500).
		Return(`{"is_refusal": false, "confidence": 0.1, "refusal_type": "", "reason": "", "suggestions": []}`, nil).Once()

	v := NewResponseValidator(mockAI)

	result, err := v.ValidateResponse(context.Background(), "")
	require.NoError(t, err)
	assert.NotNil(t, result)
	assert.Greater(t, len(result.Issues), 0) // Should have issues for empty response
}

func TestResponseValidator_ValidateResponse_RefusalDetectionError(t *testing.T) {
	t.Parallel()

	mockAI := domainmocks.NewMockAIClient(t)
	// Refusal detection fails but validation continues
	mockAI.On("ChatJSON", mock.Anything, "", mock.Anything, 500).
		Return("", assert.AnError).Once()

	v := NewResponseValidator(mockAI)
	result, err := v.ValidateResponse(context.Background(), `{"valid": "json"}`)
	require.NoError(t, err)
	assert.NotNil(t, result)
}

func TestResponseValidator_ContentQualityAssessment(t *testing.T) {
	t.Parallel()

	mockAI := domainmocks.NewMockAIClient(t)
	v := NewResponseValidator(mockAI)

	tests := []struct {
		name         string
		response     string
		expectIssues bool
	}{
		{"short_response", "OK", true},
		{"repetitive", "test test test test test test test test test test", true},
		{"incomplete_ellipsis", `{"data": "incomplete..."}`, true},
		{"good_response", `{"name": "John", "age": 30, "city": "New York"}`, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			res := &ValidationResult{Issues: []ValidationIssue{}, CleanedResponse: tt.response}
			v.performContentQualityAssessment(tt.response, res)
			if tt.expectIssues {
				assert.Greater(t, len(res.Issues), 0)
			}
		})
	}
}

func TestResponseValidator_DetermineOverallValidity_AllCases(t *testing.T) {
	t.Parallel()

	mockAI := domainmocks.NewMockAIClient(t)
	v := NewResponseValidator(mockAI)

	tests := []struct {
		name        string
		result      *ValidationResult
		expectValid bool
	}{
		{
			name:        "no_issues",
			result:      &ValidationResult{Issues: []ValidationIssue{}},
			expectValid: true,
		},
		{
			name: "low_severity_issue",
			result: &ValidationResult{Issues: []ValidationIssue{
				{Type: "test", Severity: "low"},
			}},
			expectValid: true,
		},
		{
			name: "critical_issue",
			result: &ValidationResult{Issues: []ValidationIssue{
				{Type: "test", Severity: "critical"},
			}},
			expectValid: false,
		},
		{
			name: "single_high_severity_issue",
			result: &ValidationResult{Issues: []ValidationIssue{
				{Type: "test", Severity: "high"},
			}},
			expectValid: true, // Only >2 high severity issues invalidate
		},
		{
			name: "multiple_high_severity_issues",
			result: &ValidationResult{Issues: []ValidationIssue{
				{Type: "test1", Severity: "high"},
				{Type: "test2", Severity: "high"},
				{Type: "test3", Severity: "high"},
			}},
			expectValid: false, // >2 high severity issues invalidate
		},
		{
			name:        "is_refusal",
			result:      &ValidationResult{Issues: []ValidationIssue{}, IsRefusal: true},
			expectValid: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.result.IsValid = true // Reset
			v.determineOverallValidity(tt.result)
			assert.Equal(t, tt.expectValid, tt.result.IsValid)
		})
	}
}
