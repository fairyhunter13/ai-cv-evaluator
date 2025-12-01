package freemodels

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"

	domainmocks "github.com/fairyhunter13/ai-cv-evaluator/internal/domain/mocks"
)

func TestFreeModelWrapper_ChatJSONDelegates(t *testing.T) {
	mockAI := domainmocks.NewMockAIClient(t)
	w := &FreeModelWrapper{client: mockAI}

	mockAI.On("ChatJSON", mock.Anything, "sys", "user", 42).
		Return(`{"ok":true}`, nil).Once()

	ctx := context.Background()
	out, err := w.ChatJSON(ctx, "sys", "user", 42)
	assert.NoError(t, err)
	assert.Equal(t, `{"ok":true}`, out)
}

func TestFreeModelWrapper_ChatJSONWithRetryDelegates(t *testing.T) {
	mockAI := domainmocks.NewMockAIClient(t)
	w := &FreeModelWrapper{client: mockAI}

	mockAI.On("ChatJSONWithRetry", mock.Anything, "sys", "user", 21).
		Return("retry-ok", nil).Once()

	ctx := context.Background()
	out, err := w.ChatJSONWithRetry(ctx, "sys", "user", 21)
	assert.NoError(t, err)
	assert.Equal(t, "retry-ok", out)
}

func TestFreeModelWrapper_EmbedDelegates(t *testing.T) {
	mockAI := domainmocks.NewMockAIClient(t)
	w := &FreeModelWrapper{client: mockAI}

	vecs := [][]float32{{1, 2, 3}}
	mockAI.On("Embed", mock.Anything, []string{"text"}).
		Return(vecs, nil).Once()

	ctx := context.Background()
	out, err := w.Embed(ctx, []string{"text"})
	assert.NoError(t, err)
	assert.Equal(t, vecs, out)
}

func TestFreeModelWrapper_CleanCoTResponseDelegates(t *testing.T) {
	mockAI := domainmocks.NewMockAIClient(t)
	w := &FreeModelWrapper{client: mockAI}

	mockAI.On("CleanCoTResponse", mock.Anything, "raw").
		Return("clean", nil).Once()

	ctx := context.Background()
	out, err := w.CleanCoTResponse(ctx, "raw")
	assert.NoError(t, err)
	assert.Equal(t, "clean", out)
}
