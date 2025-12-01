package ai

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"

	domainmocks "github.com/fairyhunter13/ai-cv-evaluator/internal/domain/mocks"
)

func TestNewRefusalDetector(t *testing.T) {
	mockAI := domainmocks.NewMockAIClient(t)
	rd := NewRefusalDetector(mockAI)
	assert.NotNil(t, rd)
}

func TestRefusalDetector_DetectRefusal_Success(t *testing.T) {
	t.Parallel()

	mockAI := domainmocks.NewMockAIClient(t)
	resp := `{"is_refusal": true, "confidence": 0.9, "refusal_type": "policy_violation", "reason": "policy", "suggestions":["s1"]}`
	mockAI.On("ChatJSON", mock.Anything, "", mock.Anything, 500).
		Return(resp, nil).Once()

	rd := NewRefusalDetector(mockAI)
	analysis, err := rd.DetectRefusal(context.Background(), "some response")
	assert.NoError(t, err)
	if assert.NotNil(t, analysis) {
		assert.True(t, analysis.IsRefusal)
		assert.Equal(t, "policy_violation", analysis.RefusalType)
		assert.Greater(t, analysis.Confidence, 0.0)
		assert.Len(t, analysis.Suggestions, 1)
	}
}

func TestRefusalDetector_DetectRefusal_Error(t *testing.T) {
	t.Parallel()

	mockAI := domainmocks.NewMockAIClient(t)
	mockAI.On("ChatJSON", mock.Anything, "", mock.Anything, 500).
		Return("", assert.AnError).Once()

	rd := NewRefusalDetector(mockAI)
	analysis, err := rd.DetectRefusal(context.Background(), "resp")
	assert.Error(t, err)
	assert.Nil(t, analysis)
}

func TestRefusalDetector_DetectRefusal_InvalidJSON(t *testing.T) {
	t.Parallel()

	mockAI := domainmocks.NewMockAIClient(t)
	mockAI.On("ChatJSON", mock.Anything, "", mock.Anything, 500).
		Return("not-json", nil).Once()

	rd := NewRefusalDetector(mockAI)
	analysis, err := rd.DetectRefusal(context.Background(), "resp")
	assert.Error(t, err)
	assert.Nil(t, analysis)
}

func TestRefusalDetector_DetectRefusalWithFallback_UsesAIResult(t *testing.T) {
	t.Parallel()

	mockAI := domainmocks.NewMockAIClient(t)
	resp := `{"is_refusal": false, "confidence": 0.2, "refusal_type": "", "reason": "", "suggestions":[]}`
	mockAI.On("ChatJSON", mock.Anything, "", mock.Anything, 500).
		Return(resp, nil).Once()

	rd := NewRefusalDetector(mockAI)
	analysis, err := rd.DetectRefusalWithFallback(context.Background(), "ok response")
	assert.NoError(t, err)
	if assert.NotNil(t, analysis) {
		assert.False(t, analysis.IsRefusal)
	}
}

func TestRefusalDetector_DetectRefusalWithFallback_FallbackCodeBased(t *testing.T) {
	t.Parallel()

	mockAI := domainmocks.NewMockAIClient(t)
	mockAI.On("ChatJSON", mock.Anything, "", mock.Anything, 500).
		Return("", assert.AnError).Once()

	rd := NewRefusalDetector(mockAI)
	analysis, err := rd.DetectRefusalWithFallback(context.Background(), "I'm sorry, but I cannot help with that")
	assert.NoError(t, err)
	if assert.NotNil(t, analysis) {
		assert.True(t, analysis.IsRefusal)
		assert.Equal(t, "code_detected", analysis.RefusalType)
		assert.InDelta(t, 0.7, analysis.Confidence, 0.0001)
	}
}

func TestIsRefusalResponseCodeBased(t *testing.T) {
	if !isRefusalResponseCodeBased("I'm sorry, I cannot do that due to policy") {
		t.Fatalf("expected code-based detector to flag refusal")
	}
	if isRefusalResponseCodeBased("Everything is fine, happy to help") {
		t.Fatalf("did not expect non-refusal text to be flagged")
	}
}

func TestGetRefusalHandlingSuggestions(t *testing.T) {
	var rd RefusalDetector

	security := rd.GetRefusalHandlingSuggestions("security_concerns")
	if len(security) == 0 {
		t.Fatalf("expected suggestions for security_concerns")
	}

	unknown := rd.GetRefusalHandlingSuggestions("unknown_type")
	if len(unknown) == 0 {
		t.Fatalf("expected default suggestions for unknown type")
	}
}
