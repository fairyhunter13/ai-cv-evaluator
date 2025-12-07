package tokencount

import (
	"sync"
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

func TestCountTokensDefault(t *testing.T) {
	count, err := CountTokensDefault("Hello, world!", "gpt-4")
	require.NoError(t, err)
	assert.Greater(t, count, 0)
}

func TestCalculateUsageDefault(t *testing.T) {
	usage, err := CalculateUsageDefault("You are a helpful assistant.", "Hello!", "Hi there!", "gpt-4", "openai")
	require.NoError(t, err)
	assert.NotNil(t, usage)
	assert.Greater(t, usage.PromptTokens, 0)
	assert.Greater(t, usage.CompletionTokens, 0)
	assert.Equal(t, usage.TotalTokens, usage.PromptTokens+usage.CompletionTokens)
	assert.Equal(t, "gpt-4", usage.Model)
	assert.Equal(t, "openai", usage.Provider)
}

func TestCalculateUsageWithUnknownModel(t *testing.T) {
	counter := NewCounter()

	// Test with a completely unknown model that will trigger fallback estimation
	usage, err := counter.CalculateUsage(
		"System prompt for testing",
		"User prompt for testing",
		"Completion text for testing",
		"unknown-model-xyz-123",
		"unknown-provider",
	)
	require.NoError(t, err)
	assert.NotNil(t, usage)
	// Even with unknown model, should still return reasonable estimates
	assert.GreaterOrEqual(t, usage.PromptTokens, 0)
	assert.GreaterOrEqual(t, usage.CompletionTokens, 0)
	assert.Equal(t, usage.TotalTokens, usage.PromptTokens+usage.CompletionTokens)
}

func TestCountChatTokensWithUnknownModel(t *testing.T) {
	counter := NewCounter()

	// Test with unknown model - should fall back to cl100k_base
	count, err := counter.CountChatTokens("System", "User message", "unknown-model-xyz")
	require.NoError(t, err)
	assert.Greater(t, count, 0)
}

func TestCountCompletionTokensWithUnknownModel(t *testing.T) {
	counter := NewCounter()

	// Test with unknown model - should fall back to cl100k_base
	count, err := counter.CountCompletionTokens("This is a completion", "unknown-model-xyz")
	require.NoError(t, err)
	assert.Greater(t, count, 0)
}

func TestGetEncodingForModel_CacheHit(t *testing.T) {
	counter := NewCounter()

	// First call - cache miss
	count1, err := counter.CountTokens("Hello", "gpt-4")
	require.NoError(t, err)

	// Second call - cache hit
	count2, err := counter.CountTokens("Hello", "gpt-4")
	require.NoError(t, err)

	// Should get same result
	assert.Equal(t, count1, count2)
}

func TestGetEncodingForModel_ConcurrentAccess(t *testing.T) {
	counter := NewCounter()
	var wg sync.WaitGroup
	errors := make(chan error, 10)

	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, err := counter.CountTokens("Hello world", "gpt-4")
			if err != nil {
				errors <- err
			}
		}()
	}

	wg.Wait()
	close(errors)

	for err := range errors {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestCalculateUsage_AllModels(t *testing.T) {
	counter := NewCounter()

	models := []string{
		"gpt-4",
		"gpt-4-turbo",
		"gpt-3.5-turbo",
		"claude-3-opus",
		"claude-3-sonnet",
		"gemini-pro",
		"llama-3-70b",
		"mistral-large",
	}

	for _, model := range models {
		t.Run(model, func(t *testing.T) {
			usage, err := counter.CalculateUsage(
				"You are a helpful assistant.",
				"What is 2+2?",
				"The answer is 4.",
				model,
				"test-provider",
			)
			require.NoError(t, err)
			assert.NotNil(t, usage)
			assert.Greater(t, usage.PromptTokens, 0)
			assert.Greater(t, usage.CompletionTokens, 0)
			assert.Equal(t, usage.TotalTokens, usage.PromptTokens+usage.CompletionTokens)
			assert.Equal(t, model, usage.Model)
			assert.Equal(t, "test-provider", usage.Provider)
		})
	}
}

func TestNormalizeModelName_AllVariants(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"GPT-4", "gpt-4"},
		{"GPT-4-TURBO", "gpt-4"},
		{"gpt-4-0125-preview", "gpt-4"},
		{"gpt-4-turbo-2024-04-09", "gpt-4"},
		{"gpt-3.5-turbo-0125", "gpt-3.5-turbo"},
		{"claude-3-opus-20240229", "gpt-4"},                // Claude maps to gpt-4 for tokenization
		{"claude-3-sonnet-20240229", "gpt-4"},              // Claude maps to gpt-4 for tokenization
		{"claude-3-haiku-20240307", "gpt-4"},               // Claude maps to gpt-4 for tokenization
		{"gemini-1.5-pro-latest", "gpt-4"},                 // Gemini maps to gpt-4 for tokenization
		{"llama-3-70b-instruct", "gpt-4"},                  // Llama maps to gpt-4 for tokenization
		{"mistral-large-latest", "gpt-4"},                  // Mistral maps to gpt-4 for tokenization
		{"unknown-model", "gpt-4"},                         // Unknown models default to gpt-4
		{"meta-llama/llama-3.1-8b-instruct:free", "gpt-4"}, // OpenRouter format
		{"deepseek/deepseek-chat", "gpt-4"},                // DeepSeek
		{"google/gemma-2-9b-it:free", "gpt-4"},             // Gemma
		{"qwen/qwen-2-7b-instruct:free", "gpt-4"},          // Qwen
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result := normalizeModelName(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestCalculateUsage_AllScenarios(t *testing.T) {
	t.Parallel()

	counter := NewCounter()

	tests := []struct {
		name         string
		systemPrompt string
		userPrompt   string
		completion   string
		model        string
		provider     string
	}{
		{
			name:         "standard_usage",
			systemPrompt: "You are a helpful assistant.",
			userPrompt:   "What is the capital of France?",
			completion:   "The capital of France is Paris.",
			model:        "gpt-4",
			provider:     "openai",
		},
		{
			name:         "empty_system_prompt",
			systemPrompt: "",
			userPrompt:   "Hello world",
			completion:   "Hi there!",
			model:        "gpt-3.5-turbo",
			provider:     "openai",
		},
		{
			name:         "empty_completion",
			systemPrompt: "You are a bot.",
			userPrompt:   "Say nothing",
			completion:   "",
			model:        "gpt-4",
			provider:     "openai",
		},
		{
			name:         "all_empty",
			systemPrompt: "",
			userPrompt:   "",
			completion:   "",
			model:        "gpt-4",
			provider:     "openai",
		},
		{
			name:         "openrouter_model",
			systemPrompt: "You are a code reviewer.",
			userPrompt:   "Review this code.",
			completion:   "The code looks good.",
			model:        "meta-llama/llama-3.1-8b-instruct:free",
			provider:     "openrouter",
		},
		{
			name:         "groq_model",
			systemPrompt: "You are a translator.",
			userPrompt:   "Translate hello to Spanish.",
			completion:   "Hola",
			model:        "llama-3.1-8b-instant",
			provider:     "groq",
		},
		{
			name:         "long_prompts",
			systemPrompt: "You are an expert in software development with deep knowledge of Go, Python, JavaScript, and many other programming languages. Your task is to provide detailed code reviews.",
			userPrompt:   "Please review the following code and provide feedback on best practices, potential bugs, and performance improvements. The code is a REST API handler that processes user requests.",
			completion:   "I've reviewed the code and found several areas for improvement. First, the error handling could be more robust. Second, consider adding input validation. Third, the database queries could be optimized using prepared statements.",
			model:        "gpt-4",
			provider:     "openai",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			usage, err := counter.CalculateUsage(tt.systemPrompt, tt.userPrompt, tt.completion, tt.model, tt.provider)
			require.NoError(t, err)
			assert.NotNil(t, usage)
			assert.GreaterOrEqual(t, usage.PromptTokens, 0)
			assert.GreaterOrEqual(t, usage.CompletionTokens, 0)
			assert.Equal(t, usage.PromptTokens+usage.CompletionTokens, usage.TotalTokens)
			assert.Equal(t, tt.model, usage.Model)
			assert.Equal(t, tt.provider, usage.Provider)
		})
	}
}

func TestCountTokensDefault_Extended(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		text  string
		model string
	}{
		{"simple_text", "Hello world", "gpt-4"},
		{"empty_text", "", "gpt-4"},
		{"long_text", "This is a longer text that should have more tokens than a simple hello world.", "gpt-4"},
		{"special_chars", "Hello! @#$%^&*() World", "gpt-4"},
		{"unicode", "Hello 世界 🌍", "gpt-4"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			count, err := CountTokensDefault(tt.text, tt.model)
			require.NoError(t, err)
			assert.GreaterOrEqual(t, count, 0)
		})
	}
}

func TestCalculateUsageDefault_Extended(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name         string
		systemPrompt string
		userPrompt   string
		completion   string
		model        string
		provider     string
	}{
		{"basic", "You are a bot.", "Hello", "Hi there!", "gpt-4", "openai"},
		{"empty_system", "", "Hello", "Hi!", "gpt-4", "openai"},
		{"empty_all", "", "", "", "gpt-4", "openai"},
		{"different_provider", "Bot", "Hi", "Hello!", "llama-3.1-8b", "groq"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			usage, err := CalculateUsageDefault(tt.systemPrompt, tt.userPrompt, tt.completion, tt.model, tt.provider)
			require.NoError(t, err)
			assert.NotNil(t, usage)
			assert.GreaterOrEqual(t, usage.TotalTokens, 0)
		})
	}
}

func TestGetEncodingForModel_AllPaths(t *testing.T) {
	t.Parallel()

	counter := NewCounter()

	tests := []struct {
		name  string
		model string
	}{
		{"gpt4", "gpt-4"},
		{"gpt35", "gpt-3.5-turbo"},
		{"unknown_model", "totally-unknown-model-xyz"},
		{"openrouter_format", "meta-llama/llama-3.1-8b-instruct:free"},
		{"claude", "claude-3-opus-20240229"},
		{"gemini", "gemini-1.5-pro-latest"},
		{"mistral", "mistral-large-latest"},
		{"deepseek", "deepseek/deepseek-chat"},
		{"qwen", "qwen/qwen-2-7b-instruct:free"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// First call - should create encoding
			count1, err := counter.CountTokens("Hello world", tt.model)
			require.NoError(t, err)
			assert.GreaterOrEqual(t, count1, 0)

			// Second call - should use cached encoding
			count2, err := counter.CountTokens("Hello world", tt.model)
			require.NoError(t, err)
			assert.Equal(t, count1, count2)
		})
	}
}

func TestCounter_ConcurrentAccess(t *testing.T) {
	t.Parallel()

	counter := NewCounter()
	models := []string{"gpt-4", "gpt-3.5-turbo", "claude-3-opus", "unknown-model"}

	var wg sync.WaitGroup
	for i := 0; i < 10; i++ {
		for _, model := range models {
			wg.Add(1)
			go func(m string) {
				defer wg.Done()
				_, err := counter.CountTokens("Hello world", m)
				assert.NoError(t, err)
			}(model)
		}
	}
	wg.Wait()
}

func TestCountChatTokens_AllModels(t *testing.T) {
	t.Parallel()

	counter := NewCounter()

	tests := []struct {
		name         string
		systemPrompt string
		userPrompt   string
		model        string
	}{
		{"gpt4_with_system", "You are a helpful assistant.", "Hello", "gpt-4"},
		{"gpt4_no_system", "", "Hello", "gpt-4"},
		{"gpt35_with_system", "You are a bot.", "Hi there", "gpt-3.5-turbo"},
		{"unknown_model", "System", "User", "unknown-model-xyz"},
		{"openrouter_model", "System", "User", "meta-llama/llama-3.1-8b:free"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			count, err := counter.CountChatTokens(tt.systemPrompt, tt.userPrompt, tt.model)
			require.NoError(t, err)
			assert.GreaterOrEqual(t, count, 0)
		})
	}
}

func TestCalculateUsage_Success(t *testing.T) {
	t.Parallel()

	counter := NewCounter()

	usage, err := counter.CalculateUsage(
		"You are a helpful assistant.",
		"Hello, how are you?",
		"I'm doing well, thank you for asking!",
		"gpt-4",
		"openai",
	)

	require.NoError(t, err)
	assert.NotNil(t, usage)
	assert.Greater(t, usage.PromptTokens, 0)
	assert.Greater(t, usage.CompletionTokens, 0)
	assert.Equal(t, usage.PromptTokens+usage.CompletionTokens, usage.TotalTokens)
	assert.Equal(t, "gpt-4", usage.Model)
	assert.Equal(t, "openai", usage.Provider)
}

func TestCalculateUsage_UnknownModel(t *testing.T) {
	t.Parallel()

	counter := NewCounter()

	// Unknown model should still work with fallback
	usage, err := counter.CalculateUsage(
		"System prompt",
		"User prompt",
		"Completion",
		"unknown-model-xyz",
		"unknown-provider",
	)

	require.NoError(t, err)
	assert.NotNil(t, usage)
	assert.GreaterOrEqual(t, usage.PromptTokens, 0)
	assert.GreaterOrEqual(t, usage.CompletionTokens, 0)
}

func TestCalculateUsageDefault_WithProvider(t *testing.T) {
	t.Parallel()

	usage, err := CalculateUsageDefault(
		"System",
		"User",
		"Completion",
		"gpt-4",
		"openai",
	)

	require.NoError(t, err)
	assert.NotNil(t, usage)
	assert.Equal(t, "openai", usage.Provider)
}

func TestCalculateUsage_AllFields(t *testing.T) {
	t.Parallel()

	counter := NewCounter()
	usage, err := counter.CalculateUsage(
		"You are a helpful assistant.",
		"Hello, how are you?",
		"I'm doing great, thank you for asking!",
		"gpt-4",
		"openrouter",
	)

	require.NoError(t, err)
	assert.NotNil(t, usage)
	assert.Greater(t, usage.PromptTokens, 0)
	assert.Greater(t, usage.CompletionTokens, 0)
	assert.Equal(t, usage.PromptTokens+usage.CompletionTokens, usage.TotalTokens)
	assert.Equal(t, "gpt-4", usage.Model)
	assert.Equal(t, "openrouter", usage.Provider)
}

func TestCalculateUsage_EmptyStrings(t *testing.T) {
	t.Parallel()

	counter := NewCounter()
	usage, err := counter.CalculateUsage(
		"",
		"",
		"",
		"gpt-4",
		"openai",
	)

	require.NoError(t, err)
	assert.NotNil(t, usage)
	assert.GreaterOrEqual(t, usage.PromptTokens, 0)
	assert.GreaterOrEqual(t, usage.CompletionTokens, 0)
}

func TestCountTokensDefault_Success(t *testing.T) {
	t.Parallel()

	count, err := CountTokensDefault("Hello, world!", "gpt-4")
	require.NoError(t, err)
	assert.Greater(t, count, 0)
}

func TestCountCompletionTokens_Success(t *testing.T) {
	t.Parallel()

	counter := NewCounter()
	count, err := counter.CountCompletionTokens("This is a test completion.", "gpt-4")
	require.NoError(t, err)
	assert.Greater(t, count, 0)
}
