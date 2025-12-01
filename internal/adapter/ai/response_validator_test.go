package ai

import (
	"context"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"

	domainmocks "github.com/fairyhunter13/ai-cv-evaluator/internal/domain/mocks"
)

func TestNewResponseValidator(t *testing.T) {
	mockAI := domainmocks.NewMockAIClient(t)
	validator := NewResponseValidator(mockAI)
	assert.NotNil(t, validator)
	assert.NotNil(t, validator.refusalDetector)
	assert.NotNil(t, validator.responseCleaner)
}

func TestResponseValidator_PerformBasicChecks_EmptyAndLong(t *testing.T) {
	mockAI := domainmocks.NewMockAIClient(t)
	validator := NewResponseValidator(mockAI)

	res := &ValidationResult{IsValid: true}
	err := validator.performBasicChecks("   ", res)
	assert.NoError(t, err)
	assert.False(t, res.IsValid)
	if assert.Len(t, res.Issues, 1) {
		assert.Equal(t, "empty_response", res.Issues[0].Type)
		assert.Equal(t, "critical", res.Issues[0].Severity)
	}

	long := strings.Repeat("a", 10001)
	res2 := &ValidationResult{IsValid: true}
	err = validator.performBasicChecks(long, res2)
	assert.NoError(t, err)
	found := false
	for _, issue := range res2.Issues {
		if issue.Type == "long_response" {
			found = true
		}
	}
	assert.True(t, found, "expected long_response issue")
}

func TestResponseValidator_PerformJSONValidation_ValidAndInvalid(t *testing.T) {
	mockAI := domainmocks.NewMockAIClient(t)
	validator := NewResponseValidator(mockAI)

	res := &ValidationResult{}
	err := validator.performJSONValidation("not json", res)
	assert.Error(t, err)
	assert.Len(t, res.Issues, 1)
	assert.Equal(t, "invalid_json", res.Issues[0].Type)

	res2 := &ValidationResult{}
	err = validator.performJSONValidation(`{"ok": true}`, res2)
	assert.NoError(t, err)
	assert.Empty(t, res2.Issues)
}

func TestResponseValidator_PerformContentQualityAssessment(t *testing.T) {
	mockAI := domainmocks.NewMockAIClient(t)
	validator := NewResponseValidator(mockAI)

	// Craft a response that is repetitive, incomplete, and off-topic
	text := "lorem ipsum dolor lorem ipsum dolor lorem ipsum dolor ... this looks incomplete and off-topic"
	res := &ValidationResult{}
	validator.performContentQualityAssessment(text, res)

	var kinds []string
	for _, issue := range res.Issues {
		kinds = append(kinds, issue.Type)
	}

	assert.Contains(t, kinds, "repetitive_content")
	assert.Contains(t, kinds, "incomplete_content")
	assert.Contains(t, kinds, "off_topic_content")
}

func TestResponseValidator_DetermineOverallValidity(t *testing.T) {
	validator := &ResponseValidator{}

	// Critical issue makes result invalid
	res := &ValidationResult{
		IsValid: true,
		Issues:  []ValidationIssue{{Severity: "critical"}},
	}
	validator.determineOverallValidity(res)
	assert.False(t, res.IsValid)

	// Refusal makes result invalid
	res2 := &ValidationResult{
		IsValid:   true,
		IsRefusal: true,
	}
	validator.determineOverallValidity(res2)
	assert.False(t, res2.IsValid)

	// Too many high-severity issues makes result invalid
	res3 := &ValidationResult{
		IsValid: true,
		Issues:  []ValidationIssue{{Severity: "high"}, {Severity: "high"}, {Severity: "high"}},
	}
	validator.determineOverallValidity(res3)
	assert.False(t, res3.IsValid)

	// Otherwise remains valid
	res4 := &ValidationResult{
		IsValid: true,
		Issues:  []ValidationIssue{{Severity: "low"}, {Severity: "high"}},
	}
	validator.determineOverallValidity(res4)
	assert.True(t, res4.IsValid)
}

func TestResponseValidator_ValidateResponse_Integration(t *testing.T) {
	mockAI := domainmocks.NewMockAIClient(t)
	// Refusal detector will see a non-refusal analysis
	resp := `{"is_refusal": false, "confidence": 0.1, "refusal_type": "", "reason": "", "suggestions":[]}`
	mockAI.On("ChatJSON", mock.Anything, "", mock.Anything, 500).
		Return(resp, nil).Once()

	validator := NewResponseValidator(mockAI)
	out, err := validator.ValidateResponse(context.Background(), `{"message":"this is a reasonably sized model response"}`)
	assert.NoError(t, err)
	if assert.NotNil(t, out) {
		assert.False(t, out.IsRefusal)
	}
}
