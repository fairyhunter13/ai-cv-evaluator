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

func TestCalculateUsageWithFallback(t *testing.T) {
	t.Parallel()

	counter := NewCounter()

	// Test with a model that might trigger fallback estimation
	systemPrompt := "You are a helpful assistant that provides detailed answers."
	userPrompt := "Please explain the concept of machine learning in simple terms."
	completion := "Machine learning is a type of artificial intelligence that allows computers to learn from data without being explicitly programmed."

	usage, err := counter.CalculateUsage(systemPrompt, userPrompt, completion, "unknown-model-xyz", "custom-provider")
	require.NoError(t, err)

	// Even with unknown model, should still produce reasonable estimates
	assert.Greater(t, usage.PromptTokens, 0, "prompt tokens should be positive")
	assert.Greater(t, usage.CompletionTokens, 0, "completion tokens should be positive")
	assert.Equal(t, usage.TotalTokens, usage.PromptTokens+usage.CompletionTokens)
	assert.Equal(t, "unknown-model-xyz", usage.Model)
	assert.Equal(t, "custom-provider", usage.Provider)
}

func TestCountCompletionTokens(t *testing.T) {
	t.Parallel()

	counter := NewCounter()

	tests := []struct {
		name       string
		completion string
		model      string
		minCount   int
		maxCount   int
	}{
		{
			name:       "short completion",
			completion: "Paris",
			model:      "gpt-4",
			minCount:   1,
			maxCount:   3,
		},
		{
			name:       "medium completion",
			completion: "The capital of France is Paris, which is known for the Eiffel Tower.",
			model:      "gpt-3.5-turbo",
			minCount:   10,
			maxCount:   20,
		},
		{
			name:       "completion with llama model",
			completion: "Machine learning is a subset of artificial intelligence.",
			model:      "llama-3.1-8b-instant",
			minCount:   8,
			maxCount:   15,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			count, err := counter.CountCompletionTokens(tt.completion, tt.model)
			require.NoError(t, err)
			assert.GreaterOrEqual(t, count, tt.minCount)
			assert.LessOrEqual(t, count, tt.maxCount)
		})
	}
}

func TestCountChatTokensWithDifferentModels(t *testing.T) {
	t.Parallel()

	counter := NewCounter()

	systemPrompt := "You are a code review assistant."
	userPrompt := "Review this Go function for best practices."

	models := []string{
		"gpt-4",
		"gpt-3.5-turbo",
		"meta-llama/llama-3.1-8b-instruct:free",
		"mistralai/mistral-7b-instruct:free",
		"google/gemma-7b-it:free",
		"qwen/qwen-2-7b-instruct:free",
		"deepseek/deepseek-chat",
		"anthropic/claude-3-haiku",
	}

	for _, model := range models {
		t.Run(model, func(t *testing.T) {
			count, err := counter.CountChatTokens(systemPrompt, userPrompt, model)
			require.NoError(t, err)
			assert.Greater(t, count, 0, "token count should be positive for model %s", model)
		})
	}
}

func TestEmptyInputs(t *testing.T) {
	t.Parallel()

	counter := NewCounter()

	// Empty text should return 0 tokens
	count, err := counter.CountTokens("", "gpt-4")
	require.NoError(t, err)
	assert.Equal(t, 0, count)

	// Empty prompts in chat should still have overhead tokens
	chatCount, err := counter.CountChatTokens("", "", "gpt-4")
	require.NoError(t, err)
	assert.Greater(t, chatCount, 0, "chat tokens should include message overhead even with empty prompts")
}

func TestLongText(t *testing.T) {
	t.Parallel()

	counter := NewCounter()

	// Generate a long text
	longText := ""
	for i := 0; i < 100; i++ {
		longText += "This is a test sentence to check token counting for longer texts. "
	}

	count, err := counter.CountTokens(longText, "gpt-4")
	require.NoError(t, err)
	assert.Greater(t, count, 1000, "long text should have many tokens")
}

func TestSpecialCharacters(t *testing.T) {
	t.Parallel()

	counter := NewCounter()

	tests := []struct {
		name string
		text string
	}{
		{"unicode", "Hello 世界 🌍"},
		{"code", "func main() { fmt.Println(\"Hello\") }"},
		{"json", `{"key": "value", "number": 123}`},
		{"markdown", "# Header\n\n- Item 1\n- Item 2"},
		{"newlines", "Line 1\nLine 2\nLine 3"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			count, err := counter.CountTokens(tt.text, "gpt-4")
			require.NoError(t, err)
			assert.Greater(t, count, 0, "should count tokens for %s", tt.name)
		})
	}
}

func TestCalculateUsageDefault(t *testing.T) {
	t.Parallel()

	usage, err := CalculateUsageDefault(
		"You are a helpful assistant.",
		"What is the capital of France?",
		"The capital of France is Paris.",
		"gpt-4",
		"openai",
	)
	require.NoError(t, err)
	assert.NotNil(t, usage)
	assert.Greater(t, usage.PromptTokens, 0)
	assert.Greater(t, usage.CompletionTokens, 0)
	assert.Equal(t, usage.TotalTokens, usage.PromptTokens+usage.CompletionTokens)
	assert.Equal(t, "gpt-4", usage.Model)
	assert.Equal(t, "openai", usage.Provider)
}

func TestCountTokensDefault(t *testing.T) {
	t.Parallel()

	count, err := CountTokensDefault("Hello, world!", "gpt-4")
	require.NoError(t, err)
	assert.Greater(t, count, 0)
}

func TestCalculateUsageWithUnsupportedModel(t *testing.T) {
	t.Parallel()

	counter := NewCounter()

	// Use a model that will trigger fallback estimation
	usage, err := counter.CalculateUsage(
		"System prompt for testing",
		"User prompt for testing",
		"Completion text for testing",
		"completely-unknown-model-xyz",
		"unknown-provider",
	)
	require.NoError(t, err)
	assert.NotNil(t, usage)
	// Should still calculate tokens (using fallback)
	assert.Greater(t, usage.PromptTokens, 0)
	assert.Greater(t, usage.CompletionTokens, 0)
	assert.Equal(t, "completely-unknown-model-xyz", usage.Model)
	assert.Equal(t, "unknown-provider", usage.Provider)
}

func TestCalculateUsageWithEmptyInputs(t *testing.T) {
	t.Parallel()

	counter := NewCounter()

	usage, err := counter.CalculateUsage("", "", "", "gpt-4", "openai")
	require.NoError(t, err)
	assert.NotNil(t, usage)
	// Empty completion should have 0 completion tokens
	assert.Equal(t, 0, usage.CompletionTokens)
	// Empty prompts still have message overhead
	assert.GreaterOrEqual(t, usage.PromptTokens, 0)
}

func TestCalculateUsageWithLongInputs(t *testing.T) {
	t.Parallel()

	counter := NewCounter()

	// Generate long inputs
	longSystem := ""
	longUser := ""
	longCompletion := ""
	for i := 0; i < 50; i++ {
		longSystem += "This is a long system prompt. "
		longUser += "This is a long user prompt. "
		longCompletion += "This is a long completion. "
	}

	usage, err := counter.CalculateUsage(longSystem, longUser, longCompletion, "gpt-4", "openai")
	require.NoError(t, err)
	assert.NotNil(t, usage)
	assert.Greater(t, usage.PromptTokens, 100)
	assert.Greater(t, usage.CompletionTokens, 100)
	assert.Greater(t, usage.TotalTokens, 200)
}
