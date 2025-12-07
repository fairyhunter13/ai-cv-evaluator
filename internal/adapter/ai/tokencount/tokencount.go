// Package tokencount provides accurate token counting for LLM API calls.
//
// It uses tiktoken-go, a Go port of OpenAI's official tiktoken library,
// to count tokens for various LLM models. This enables accurate tracking
// of token usage for cost estimation and monitoring.
package tokencount

import (
	"strings"
	"sync"

	"log/slog"

	tiktoken "github.com/pkoukk/tiktoken-go"
)

// TokenUsage represents token counts for an LLM API call.
type TokenUsage struct {
	PromptTokens     int    `json:"prompt_tokens"`
	CompletionTokens int    `json:"completion_tokens"`
	TotalTokens      int    `json:"total_tokens"`
	Model            string `json:"model"`
	Provider         string `json:"provider"`
}

// Counter provides thread-safe token counting for LLM models.
type Counter struct {
	encodingCache map[string]*tiktoken.Tiktoken
	mu            sync.RWMutex
}

// NewCounter creates a new token counter instance.
func NewCounter() *Counter {
	return &Counter{
		encodingCache: make(map[string]*tiktoken.Tiktoken),
	}
}

// DefaultCounter is a global token counter instance.
var DefaultCounter = NewCounter()

// getEncodingForModel returns the appropriate tiktoken encoding for a model.
// It caches encodings for performance.
func (c *Counter) getEncodingForModel(model string) (*tiktoken.Tiktoken, error) {
	// Normalize model name for lookup
	normalizedModel := normalizeModelName(model)

	c.mu.RLock()
	if enc, ok := c.encodingCache[normalizedModel]; ok {
		c.mu.RUnlock()
		return enc, nil
	}
	c.mu.RUnlock()

	c.mu.Lock()
	defer c.mu.Unlock()

	// Double-check after acquiring write lock
	if enc, ok := c.encodingCache[normalizedModel]; ok {
		return enc, nil
	}

	// Try to get encoding for the specific model
	enc, err := tiktoken.EncodingForModel(normalizedModel)
	if err != nil {
		// Fall back to cl100k_base which is used by GPT-4, GPT-3.5-turbo, and most modern models
		slog.Debug("falling back to cl100k_base encoding",
			slog.String("model", model),
			slog.String("normalized", normalizedModel),
			slog.Any("error", err))
		enc, err = tiktoken.GetEncoding("cl100k_base")
		if err != nil {
			return nil, err
		}
	}

	c.encodingCache[normalizedModel] = enc
	return enc, nil
}

// normalizeModelName converts model IDs to tiktoken-compatible names.
func normalizeModelName(model string) string {
	model = strings.ToLower(model)

	// OpenRouter model IDs often have provider prefixes
	// e.g., "meta-llama/llama-3.1-8b-instruct:free"
	if strings.Contains(model, "/") {
		parts := strings.Split(model, "/")
		model = parts[len(parts)-1]
	}

	// Remove :free suffix from OpenRouter models
	model = strings.TrimSuffix(model, ":free")

	// Map common model families to tiktoken-compatible names
	switch {
	case strings.Contains(model, "gpt-4"):
		return "gpt-4"
	case strings.Contains(model, "gpt-3.5"):
		return "gpt-3.5-turbo"
	case strings.Contains(model, "llama"):
		// Llama models use similar tokenization to GPT-4
		return "gpt-4"
	case strings.Contains(model, "mistral"):
		// Mistral models use similar tokenization
		return "gpt-4"
	case strings.Contains(model, "gemma"):
		// Gemma models use similar tokenization
		return "gpt-4"
	case strings.Contains(model, "qwen"):
		// Qwen models use similar tokenization
		return "gpt-4"
	case strings.Contains(model, "deepseek"):
		// DeepSeek models use similar tokenization
		return "gpt-4"
	case strings.Contains(model, "claude"):
		// Claude uses different tokenization but cl100k_base is a reasonable approximation
		return "gpt-4"
	default:
		// Default to GPT-4 encoding for unknown models
		return "gpt-4"
	}
}

// CountTokens counts the number of tokens in a text string for a given model.
func (c *Counter) CountTokens(text, model string) (int, error) {
	enc, err := c.getEncodingForModel(model)
	if err != nil {
		return 0, err
	}

	tokens := enc.Encode(text, nil, nil)
	return len(tokens), nil
}

// CountChatTokens counts tokens for a chat completion request.
// It accounts for the message structure overhead used by OpenAI-compatible APIs.
func (c *Counter) CountChatTokens(systemPrompt, userPrompt, model string) (int, error) {
	enc, err := c.getEncodingForModel(model)
	if err != nil {
		return 0, err
	}

	// Token overhead per message varies by model
	// For GPT-3.5-turbo and GPT-4: 3 tokens per message + 1 for role
	// See: https://github.com/openai/openai-cookbook/blob/main/examples/How_to_count_tokens_with_tiktoken.ipynb
	tokensPerMessage := 3
	tokensPerRole := 1

	numTokens := 0

	// System message
	numTokens += tokensPerMessage
	numTokens += len(enc.Encode("system", nil, nil))
	numTokens += len(enc.Encode(systemPrompt, nil, nil))
	numTokens += tokensPerRole

	// User message
	numTokens += tokensPerMessage
	numTokens += len(enc.Encode("user", nil, nil))
	numTokens += len(enc.Encode(userPrompt, nil, nil))
	numTokens += tokensPerRole

	// Every reply is primed with <|start|>assistant<|message|>
	numTokens += 3

	return numTokens, nil
}

// CountCompletionTokens counts tokens in a completion response.
func (c *Counter) CountCompletionTokens(completion, model string) (int, error) {
	return c.CountTokens(completion, model)
}

// CalculateUsage calculates full token usage for a chat completion.
func (c *Counter) CalculateUsage(systemPrompt, userPrompt, completion, model, provider string) (*TokenUsage, error) {
	promptTokens, err := c.CountChatTokens(systemPrompt, userPrompt, model)
	if err != nil {
		slog.Warn("failed to count prompt tokens, using estimate",
			slog.String("model", model),
			slog.Any("error", err))
		// Fall back to rough estimate: ~4 chars per token
		promptTokens = (len(systemPrompt) + len(userPrompt)) / 4
	}

	completionTokens, err := c.CountCompletionTokens(completion, model)
	if err != nil {
		slog.Warn("failed to count completion tokens, using estimate",
			slog.String("model", model),
			slog.Any("error", err))
		// Fall back to rough estimate: ~4 chars per token
		completionTokens = len(completion) / 4
	}

	return &TokenUsage{
		PromptTokens:     promptTokens,
		CompletionTokens: completionTokens,
		TotalTokens:      promptTokens + completionTokens,
		Model:            model,
		Provider:         provider,
	}, nil
}

// CountTokensDefault uses the default counter to count tokens.
func CountTokensDefault(text, model string) (int, error) {
	return DefaultCounter.CountTokens(text, model)
}

// CalculateUsageDefault uses the default counter to calculate usage.
func CalculateUsageDefault(systemPrompt, userPrompt, completion, model, provider string) (*TokenUsage, error) {
	return DefaultCounter.CalculateUsage(systemPrompt, userPrompt, completion, model, provider)
}
