package shared

import (
	"context"
	"testing"

	domainmocks "github.com/fairyhunter13/ai-cv-evaluator/internal/domain/mocks"
	"github.com/stretchr/testify/assert"
	mock "github.com/stretchr/testify/mock"
)

func TestDirectAICall(t *testing.T) {
	// Create mock AI client
	mockAI := &domainmocks.AIClient{}

	// Mock a simple response
	response := `{"test": "response"}`
	mockAI.On("ChatJSON", mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(response, nil)

	// Test direct AI call
	ctx := context.Background()
	result, err := mockAI.ChatJSON(ctx, "", "test prompt", 100)

	// Assertions
	assert.NoError(t, err)
	assert.Equal(t, response, result)

	// Verify mock call
	mockAI.AssertExpectations(t)
}

func TestIntegratedHandlerCreation(t *testing.T) {
	// Create mock AI client
	mockAI := &domainmocks.AIClient{}

	// Test handler creation
	handler := NewIntegratedEvaluationHandler(mockAI, nil)
	assert.NotNil(t, handler)
	assert.Equal(t, mockAI, handler.ai)
	assert.Nil(t, handler.q)
}
