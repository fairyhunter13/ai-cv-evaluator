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
