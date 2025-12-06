package real

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestEstimateTokenCount(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		text     string
		minCount int
		maxCount int
	}{
		{
			name:     "empty string",
			text:     "",
			minCount: 0,
			maxCount: 0,
		},
		{
			name:     "simple text",
			text:     "Hello, world!",
			minCount: 1,
			maxCount: 10,
		},
		{
			name:     "longer text",
			text:     "This is a longer piece of text that should have more tokens than the simple example.",
			minCount: 10,
			maxCount: 30,
		},
		{
			name:     "JSON content",
			text:     `{"name": "John", "age": 30, "city": "New York"}`,
			minCount: 10,
			maxCount: 30,
		},
		{
			name:     "code snippet",
			text:     `func main() { fmt.Println("Hello, World!") }`,
			minCount: 5,
			maxCount: 25,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			count := estimateTokenCount(tt.text)
			assert.GreaterOrEqual(t, count, tt.minCount, "token count should be at least %d", tt.minCount)
			assert.LessOrEqual(t, count, tt.maxCount, "token count should be at most %d", tt.maxCount)
		})
	}
}

func TestEstimateChatTokens(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name                string
		systemPrompt        string
		userPrompt          string
		response            string
		minPromptTokens     int
		maxPromptTokens     int
		minCompletionTokens int
		maxCompletionTokens int
	}{
		{
			name:                "empty prompts and response",
			systemPrompt:        "",
			userPrompt:          "",
			response:            "",
			minPromptTokens:     8, // overhead tokens
			maxPromptTokens:     8,
			minCompletionTokens: 0,
			maxCompletionTokens: 0,
		},
		{
			name:                "simple chat",
			systemPrompt:        "You are a helpful assistant.",
			userPrompt:          "Hello!",
			response:            "Hi there! How can I help you today?",
			minPromptTokens:     10,
			maxPromptTokens:     30,
			minCompletionTokens: 5,
			maxCompletionTokens: 20,
		},
		{
			name:                "longer chat",
			systemPrompt:        "You are an expert CV evaluator. Analyze the following CV and provide a detailed assessment.",
			userPrompt:          "Please evaluate this CV: John Doe, Software Engineer with 5 years of experience in Go and Python.",
			response:            `{"score": 8, "summary": "Strong technical background with relevant experience."}`,
			minPromptTokens:     30,
			maxPromptTokens:     80,
			minCompletionTokens: 10,
			maxCompletionTokens: 40,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			promptTokens, completionTokens := estimateChatTokens(tt.systemPrompt, tt.userPrompt, tt.response)

			assert.GreaterOrEqual(t, promptTokens, tt.minPromptTokens, "prompt tokens should be at least %d", tt.minPromptTokens)
			assert.LessOrEqual(t, promptTokens, tt.maxPromptTokens, "prompt tokens should be at most %d", tt.maxPromptTokens)
			assert.GreaterOrEqual(t, completionTokens, tt.minCompletionTokens, "completion tokens should be at least %d", tt.minCompletionTokens)
			assert.LessOrEqual(t, completionTokens, tt.maxCompletionTokens, "completion tokens should be at most %d", tt.maxCompletionTokens)
		})
	}
}

func TestRecordTokenUsage(t *testing.T) {
	t.Parallel()

	// Test that recordTokenUsage doesn't panic with various inputs
	tests := []struct {
		name             string
		provider         string
		model            string
		promptTokens     int
		completionTokens int
	}{
		{
			name:             "groq with tokens",
			provider:         "groq",
			model:            "llama3-8b-8192",
			promptTokens:     100,
			completionTokens: 50,
		},
		{
			name:             "openrouter with tokens",
			provider:         "openrouter",
			model:            "meta-llama/llama-3-8b-instruct:free",
			promptTokens:     200,
			completionTokens: 100,
		},
		{
			name:             "openai embeddings",
			provider:         "openai",
			model:            "text-embedding-3-small",
			promptTokens:     500,
			completionTokens: 0,
		},
		{
			name:             "zero tokens",
			provider:         "test",
			model:            "test-model",
			promptTokens:     0,
			completionTokens: 0,
		},
		{
			name:             "only prompt tokens",
			provider:         "test",
			model:            "test-model",
			promptTokens:     100,
			completionTokens: 0,
		},
		{
			name:             "only completion tokens",
			provider:         "test",
			model:            "test-model",
			promptTokens:     0,
			completionTokens: 50,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(_ *testing.T) {
			// This should not panic
			recordTokenUsage(tt.provider, tt.model, tt.promptTokens, tt.completionTokens)
		})
	}
}

func TestEstimateTokenCount_Consistency(t *testing.T) {
	t.Parallel()

	// Test that the same input always produces the same output
	text := "This is a test string for consistency checking."

	count1 := estimateTokenCount(text)
	count2 := estimateTokenCount(text)
	count3 := estimateTokenCount(text)

	assert.Equal(t, count1, count2, "token count should be consistent")
	assert.Equal(t, count2, count3, "token count should be consistent")
}

func TestEstimateChatTokens_IncludesOverhead(t *testing.T) {
	t.Parallel()

	// Test that the overhead is included in prompt tokens
	promptTokens, _ := estimateChatTokens("", "", "")

	// Should include the 8-token overhead for message formatting
	assert.Equal(t, 8, promptTokens, "empty prompts should still have overhead tokens")
}
