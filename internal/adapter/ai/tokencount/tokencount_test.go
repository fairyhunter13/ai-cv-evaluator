package tokencount

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCountTokens(t *testing.T) {
	t.Parallel()

	counter := NewCounter()

	tests := []struct {
		name     string
		text     string
		model    string
		minCount int
		maxCount int
	}{
		{
			name:     "simple text with gpt-4",
			text:     "Hello, world!",
			model:    "gpt-4",
			minCount: 3,
			maxCount: 5,
		},
		{
			name:     "longer text",
			text:     "The quick brown fox jumps over the lazy dog.",
			model:    "gpt-3.5-turbo",
			minCount: 8,
			maxCount: 12,
		},
		{
			name:     "llama model (uses gpt-4 encoding)",
			text:     "Hello, world!",
			model:    "meta-llama/llama-3.1-8b-instruct:free",
			minCount: 3,
			maxCount: 5,
		},
		{
			name:     "groq model",
			text:     "Testing token counting",
			model:    "llama-3.1-8b-instant",
			minCount: 3,
			maxCount: 6,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			count, err := counter.CountTokens(tt.text, tt.model)
			require.NoError(t, err)
			assert.GreaterOrEqual(t, count, tt.minCount, "token count should be at least %d", tt.minCount)
			assert.LessOrEqual(t, count, tt.maxCount, "token count should be at most %d", tt.maxCount)
		})
	}
}

func TestCountChatTokens(t *testing.T) {
	t.Parallel()

	counter := NewCounter()

	systemPrompt := "You are a helpful assistant."
	userPrompt := "What is the capital of France?"

	count, err := counter.CountChatTokens(systemPrompt, userPrompt, "gpt-4")
	require.NoError(t, err)

	// Chat tokens include message overhead
	assert.Greater(t, count, 10, "chat tokens should include message overhead")
	assert.Less(t, count, 30, "chat tokens should be reasonable")
}

func TestCalculateUsage(t *testing.T) {
	t.Parallel()

	counter := NewCounter()

	systemPrompt := "You are a helpful assistant."
	userPrompt := "What is the capital of France?"
	completion := "The capital of France is Paris."

	usage, err := counter.CalculateUsage(systemPrompt, userPrompt, completion, "gpt-4", "openai")
	require.NoError(t, err)

	assert.Greater(t, usage.PromptTokens, 0, "prompt tokens should be positive")
	assert.Greater(t, usage.CompletionTokens, 0, "completion tokens should be positive")
	assert.Equal(t, usage.PromptTokens+usage.CompletionTokens, usage.TotalTokens, "total should equal sum")
	assert.Equal(t, "gpt-4", usage.Model)
	assert.Equal(t, "openai", usage.Provider)
}

func TestNormalizeModelName(t *testing.T) {
	t.Parallel()

	tests := []struct {
		input    string
		expected string
	}{
		{"gpt-4", "gpt-4"},
		{"gpt-4-turbo", "gpt-4"},
		{"gpt-3.5-turbo", "gpt-3.5-turbo"},
		{"meta-llama/llama-3.1-8b-instruct:free", "gpt-4"},
		{"mistralai/mistral-7b-instruct:free", "gpt-4"},
		{"google/gemma-7b-it:free", "gpt-4"},
		{"deepseek/deepseek-chat", "gpt-4"},
		{"anthropic/claude-3-haiku", "gpt-4"},
		{"unknown-model", "gpt-4"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result := normalizeModelName(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestDefaultCounter(t *testing.T) {
	t.Parallel()

	// Test that the default counter works
	count, err := CountTokensDefault("Hello, world!", "gpt-4")
	require.NoError(t, err)
	assert.Greater(t, count, 0)

	usage, err := CalculateUsageDefault("System", "User", "Response", "gpt-4", "openai")
	require.NoError(t, err)
	assert.Greater(t, usage.TotalTokens, 0)
}

func TestEncodingCache(t *testing.T) {
	t.Parallel()

	counter := NewCounter()

	// First call should create the encoding
	count1, err := counter.CountTokens("Hello", "gpt-4")
	require.NoError(t, err)

	// Second call should use cached encoding
	count2, err := counter.CountTokens("Hello", "gpt-4")
	require.NoError(t, err)

	assert.Equal(t, count1, count2, "cached encoding should produce same result")
}
