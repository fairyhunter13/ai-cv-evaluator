package ai

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"

	domainmocks "github.com/fairyhunter13/ai-cv-evaluator/internal/domain/mocks"
)

func TestEmbedCache_PassthroughMethods(t *testing.T) {
	mockAI := domainmocks.NewMockAIClient(t)
	client := NewEmbedCache(mockAI, 4)
	ctx := context.Background()

	// ChatJSON passthrough
	mockAI.On("ChatJSON", mock.Anything, "sys", "user", 10).
		Return(`{"ok":true}`, nil).Once()

	resp, err := client.ChatJSON(ctx, "sys", "user", 10)
	assert.NoError(t, err)
	assert.Equal(t, `{"ok":true}`, resp)

	// ChatJSONWithRetry passthrough
	mockAI.On("ChatJSONWithRetry", mock.Anything, "", "retry", 5).
		Return("retry-ok", nil).Once()

	resp, err = client.ChatJSONWithRetry(ctx, "", "retry", 5)
	assert.NoError(t, err)
	assert.Equal(t, "retry-ok", resp)

	// CleanCoTResponse passthrough
	mockAI.On("CleanCoTResponse", mock.Anything, "raw").
		Return("clean", nil).Once()

	cleaned, err := client.CleanCoTResponse(ctx, "raw")
	assert.NoError(t, err)
	assert.Equal(t, "clean", cleaned)
}
