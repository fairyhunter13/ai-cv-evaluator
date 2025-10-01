// Package real implements a real AI client backed by OpenRouter and OpenAI APIs.
package real

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	backoff "github.com/cenkalti/backoff/v4"

	"log/slog"

	"github.com/fairyhunter13/ai-cv-evaluator/internal/adapter/observability"
	"github.com/fairyhunter13/ai-cv-evaluator/internal/config"
	"github.com/fairyhunter13/ai-cv-evaluator/internal/domain"
)

// Client implements domain.AIClient using OpenRouter (chat) and OpenAI (embeddings).
type Client struct {
	cfg     config.Config
	chatHC  *http.Client
	embedHC *http.Client
}

// readSnippet reads up to n bytes from r and returns it as a string, non-destructively where possible.
func readSnippet(r io.Reader, n int) string {
	if r == nil || n <= 0 {
		return ""
	}
	buf := make([]byte, n)
	m, _ := io.ReadAtLeast(&limitedReader{R: r, N: int64(n)}, buf, 0)
	return string(buf[:m])
}

type limitedReader struct {
	R io.Reader
	N int64
}

func (l *limitedReader) Read(p []byte) (int, error) {
	if l.N <= 0 {
		return 0, io.EOF
	}
	if int64(len(p)) > l.N {
		p = p[:l.N]
	}
	n, err := l.R.Read(p)
	l.N -= int64(n)
	return n, err
}

// New constructs a real AI client with sensible timeouts.
func New(cfg config.Config) *Client {
	return &Client{
		cfg:     cfg,
		chatHC:  &http.Client{Timeout: 15 * time.Second},
		embedHC: &http.Client{Timeout: 15 * time.Second},
	}
}

// getBackoffConfig returns a configured ExponentialBackOff based on the current environment.
func (c *Client) getBackoffConfig() *backoff.ExponentialBackOff {
	expo := backoff.NewExponentialBackOff()

	maxElapsedTime, initialInterval, maxInterval, multiplier := c.cfg.GetAIBackoffConfig()
	expo.MaxElapsedTime = maxElapsedTime
	expo.InitialInterval = initialInterval
	expo.MaxInterval = maxInterval
	expo.Multiplier = multiplier

	return expo
}

// ChatJSON calls OpenRouter (OpenAI-compatible) chat completions and returns the message content.
func (c *Client) ChatJSON(ctx domain.Context, systemPrompt, userPrompt string, maxTokens int) (string, error) {
	if c.cfg.OpenRouterAPIKey == "" {
		slog.Error("OpenRouter API key missing", slog.String("provider", "openrouter"))
		return "", fmt.Errorf("%w: OPENROUTER_API_KEY missing", domain.ErrInvalidArgument)
	}
	slog.Info("calling OpenRouter API", slog.String("provider", "openrouter"), slog.String("model", c.cfg.ChatModel), slog.Int("max_tokens", maxTokens))
	model := c.cfg.ChatModel
	if strings.TrimSpace(model) == "" {
		// Allow auto router when CHAT_MODEL is unspecified
		model = "openrouter/auto"
	}
	body := map[string]any{
		"model":       model,
		"temperature": 0.2,
		"max_tokens":  maxTokens,
		"messages": []map[string]string{
			{"role": "system", "content": systemPrompt},
			{"role": "user", "content": userPrompt},
		},
	}
	if len(c.cfg.ChatFallbackModels) > 0 {
		body["models"] = c.cfg.ChatFallbackModels
	}
	b, _ := json.Marshal(body)
	var out struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
	}
	op := func() error {
		start := time.Now()
		// Recreate request each attempt to avoid reusing consumed bodies
		r, _ := http.NewRequestWithContext(ctx, http.MethodPost, c.cfg.OpenRouterBaseURL+"/chat/completions", bytes.NewReader(b))
		r.Header.Set("Authorization", "Bearer "+c.cfg.OpenRouterAPIKey)
		r.Header.Set("Content-Type", "application/json")
		resp, err := c.chatHC.Do(r)
		observability.AIRequestsTotal.WithLabelValues("openrouter", "chat").Inc()
		observability.AIRequestDuration.WithLabelValues("openrouter", "chat").Observe(time.Since(start).Seconds())
		if err != nil {
			return err
		}
		defer func() { _ = resp.Body.Close() }()
		if resp.StatusCode == 429 {
			// Retryable: let backoff handle retries
			slog.Warn("ai provider rate limited", slog.String("provider", "openrouter"), slog.String("op", "chat"), slog.Int("status", resp.StatusCode), slog.String("x_request_id", resp.Header.Get("X-Request-Id")))
			return fmt.Errorf("rate limited: 429")
		}
		if resp.StatusCode >= 400 && resp.StatusCode < 500 {
			// Client error: non-retryable
			bodySnippet := readSnippet(resp.Body, 512)
			slog.Warn("ai provider 4xx", slog.String("provider", "openrouter"), slog.String("op", "chat"), slog.Int("status", resp.StatusCode), slog.String("model", model), slog.String("endpoint", c.cfg.OpenRouterBaseURL+"/chat/completions"), slog.String("x_request_id", resp.Header.Get("X-Request-Id")), slog.String("body", bodySnippet))
			return backoff.Permanent(fmt.Errorf("chat status %d", resp.StatusCode))
		}
		if resp.StatusCode < 200 || resp.StatusCode >= 300 {
			// 5xx and others: retryable
			bodySnippet := readSnippet(resp.Body, 512)
			slog.Error("ai provider non-2xx", slog.String("provider", "openrouter"), slog.String("op", "chat"), slog.Int("status", resp.StatusCode), slog.String("model", model), slog.String("endpoint", c.cfg.OpenRouterBaseURL+"/chat/completions"), slog.String("x_request_id", resp.Header.Get("X-Request-Id")), slog.String("body", bodySnippet))
			return fmt.Errorf("chat status %d", resp.StatusCode)
		}
		if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
			slog.Error("ai provider decode error", slog.String("provider", "openrouter"), slog.String("op", "chat"), slog.String("model", model), slog.String("endpoint", c.cfg.OpenRouterBaseURL+"/chat/completions"), slog.Any("error", err))
			return err
		}
		return nil
	}
	expo := c.getBackoffConfig()
	bo := backoff.WithContext(expo, ctx)

	slog.Info("starting OpenRouter API retry logic", slog.String("provider", "openrouter"), slog.Duration("max_elapsed", expo.MaxElapsedTime))
	if err := backoff.Retry(op, bo); err != nil {
		slog.Error("OpenRouter API failed after retries", slog.String("provider", "openrouter"), slog.Any("error", err))
		return "", fmt.Errorf("openrouter api failed: %w", err)
	}

	if len(out.Choices) == 0 {
		slog.Error("OpenRouter API returned empty choices", slog.String("provider", "openrouter"))
		return "", errors.New("empty choices from OpenRouter API")
	}

	slog.Info("OpenRouter API call successful", slog.String("provider", "openrouter"), slog.Int("choices_count", len(out.Choices)))
	return out.Choices[0].Message.Content, nil
}

// Embed calls OpenAI embeddings endpoint and returns vectors.
func (c *Client) Embed(ctx domain.Context, texts []string) ([][]float32, error) {
	if c.cfg.OpenAIAPIKey == "" || c.cfg.EmbeddingsModel == "" {
		// Do not log secrets; only indicate presence
		slog.Error("OpenAI API key or model missing", slog.String("provider", "openai"), slog.Bool("has_api_key", c.cfg.OpenAIAPIKey != ""), slog.String("model", c.cfg.EmbeddingsModel))
		return nil, fmt.Errorf("%w: OPENAI_API_KEY or EMBEDDINGS_MODEL missing", domain.ErrInvalidArgument)
	}
	slog.Info("calling OpenAI API for embeddings", slog.String("provider", "openai"), slog.String("model", c.cfg.EmbeddingsModel), slog.Int("text_count", len(texts)))
	body := map[string]any{
		"model": c.cfg.EmbeddingsModel,
		"input": texts,
	}
	b, _ := json.Marshal(body)
	var out struct {
		Data []struct {
			Embedding []float64 `json:"embedding"`
		} `json:"data"`
	}
	op := func() error {
		start := time.Now()
		// Recreate request each attempt to avoid reusing consumed bodies
		r, _ := http.NewRequestWithContext(ctx, http.MethodPost, c.cfg.OpenAIBaseURL+"/embeddings", bytes.NewReader(b))
		r.Header.Set("Authorization", "Bearer "+c.cfg.OpenAIAPIKey)
		r.Header.Set("Content-Type", "application/json")
		resp, err := c.embedHC.Do(r)
		observability.AIRequestsTotal.WithLabelValues("openai", "embed").Inc()
		observability.AIRequestDuration.WithLabelValues("openai", "embed").Observe(time.Since(start).Seconds())
		if err != nil {
			return err
		}
		defer func() { _ = resp.Body.Close() }()
		if resp.StatusCode == 429 {
			// Retryable: let backoff handle retries
			slog.Warn("ai provider rate limited", slog.String("provider", "openai"), slog.String("op", "embed"), slog.Int("status", resp.StatusCode), slog.String("x_request_id", resp.Header.Get("X-Request-Id")), slog.String("openai_request_id", resp.Header.Get("Openai-Request-Id")))
			return fmt.Errorf("rate limited: 429")
		}
		if resp.StatusCode >= 400 && resp.StatusCode < 500 {
			// Client error: non-retryable
			bodySnippet := readSnippet(resp.Body, 512)
			slog.Warn("ai provider 4xx", slog.String("provider", "openai"), slog.String("op", "embed"), slog.Int("status", resp.StatusCode), slog.String("model", c.cfg.EmbeddingsModel), slog.String("endpoint", c.cfg.OpenAIBaseURL+"/embeddings"), slog.String("x_request_id", resp.Header.Get("X-Request-Id")), slog.String("openai_request_id", resp.Header.Get("Openai-Request-Id")), slog.String("body", bodySnippet))
			return backoff.Permanent(fmt.Errorf("embed status %d", resp.StatusCode))
		}
		if resp.StatusCode < 200 || resp.StatusCode >= 300 {
			// 5xx and others: retryable
			bodySnippet := readSnippet(resp.Body, 512)
			slog.Error("ai provider non-2xx", slog.String("provider", "openai"), slog.String("op", "embed"), slog.Int("status", resp.StatusCode), slog.String("model", c.cfg.EmbeddingsModel), slog.String("endpoint", c.cfg.OpenAIBaseURL+"/embeddings"), slog.String("x_request_id", resp.Header.Get("X-Request-Id")), slog.String("openai_request_id", resp.Header.Get("Openai-Request-Id")), slog.String("body", bodySnippet))
			return fmt.Errorf("embed status %d", resp.StatusCode)
		}
		if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
			slog.Error("ai provider decode error", slog.String("provider", "openai"), slog.String("op", "embed"), slog.String("model", c.cfg.EmbeddingsModel), slog.String("endpoint", c.cfg.OpenAIBaseURL+"/embeddings"), slog.Any("error", err))
			return err
		}
		return nil
	}
	expo := c.getBackoffConfig()
	bo := backoff.WithContext(expo, ctx)

	slog.Info("starting OpenAI API retry logic", slog.String("provider", "openai"), slog.Duration("max_elapsed", expo.MaxElapsedTime))
	if err := backoff.Retry(op, bo); err != nil {
		slog.Error("OpenAI API failed after retries", slog.String("provider", "openai"), slog.Any("error", err))
		return nil, fmt.Errorf("openai api failed: %w", err)
	}

	if len(out.Data) == 0 {
		slog.Error("OpenAI API returned empty data", slog.String("provider", "openai"))
		return nil, errors.New("empty data from OpenAI API")
	}

	slog.Info("OpenAI API call successful", slog.String("provider", "openai"), slog.Int("data_count", len(out.Data)))
	res := make([][]float32, len(out.Data))
	for i := range out.Data {
		v := make([]float32, len(out.Data[i].Embedding))
		for j := range out.Data[i].Embedding {
			v[j] = float32(out.Data[i].Embedding[j])
		}
		res[i] = v
	}
	return res, nil
}
