// Package real implements a real AI client backed by OpenRouter and OpenAI APIs.
package real

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strings"
	"net/http"
	"time"

	backoff "github.com/cenkalti/backoff/v4"

	"github.com/fairyhunter13/ai-cv-evaluator/internal/config"
	"github.com/fairyhunter13/ai-cv-evaluator/internal/domain"
	"github.com/fairyhunter13/ai-cv-evaluator/internal/adapter/observability"
	"log/slog"
)

// Client implements domain.AIClient using OpenRouter (chat) and OpenAI (embeddings).
type Client struct {
	cfg        config.Config
	chatHC     *http.Client
	embedHC    *http.Client
}

// readSnippet reads up to n bytes from r and returns it as a string, non-destructively where possible.
func readSnippet(r io.Reader, n int) string {
    if r == nil || n <= 0 { return "" }
    buf := make([]byte, n)
    m, _ := io.ReadAtLeast(&limitedReader{R: r, N: int64(n)}, buf, 0)
    return string(buf[:m])
}

type limitedReader struct{ R io.Reader; N int64 }
func (l *limitedReader) Read(p []byte) (int, error) {
    if l.N <= 0 { return 0, io.EOF }
    if int64(len(p)) > l.N { p = p[:l.N] }
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

// ChatJSON calls OpenRouter (OpenAI-compatible) chat completions and returns the message content.
func (c *Client) ChatJSON(ctx domain.Context, systemPrompt, userPrompt string, maxTokens int) (string, error) {
	if c.cfg.OpenRouterAPIKey == "" {
		return "", fmt.Errorf("%w: OPENROUTER_API_KEY missing", domain.ErrInvalidArgument)
	}
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
		Choices []struct{ Message struct{ Content string `json:"content"` } `json:"message"` } `json:"choices"`
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
		if err != nil { return err }
		defer func(){ _ = resp.Body.Close() }()
		if resp.StatusCode == 429 {
			slog.Warn("ai provider rate limited", slog.String("provider", "openrouter"), slog.String("op", "chat"), slog.Int("status", resp.StatusCode), slog.String("x_request_id", resp.Header.Get("X-Request-Id")))
			return backoff.Permanent(fmt.Errorf("%w", domain.ErrUpstreamRateLimit))
		}
		if resp.StatusCode < 200 || resp.StatusCode >= 300 {
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
	expo := backoff.NewExponentialBackOff()
	expo.MaxElapsedTime = 20 * time.Second
	bo := backoff.WithContext(expo, ctx)
	if err := backoff.Retry(op, bo); err != nil { return "", err }
	if len(out.Choices) == 0 { return "", errors.New("empty choices") }
	return out.Choices[0].Message.Content, nil
}

// Embed calls OpenAI embeddings endpoint and returns vectors.
func (c *Client) Embed(ctx domain.Context, texts []string) ([][]float32, error) {
	if c.cfg.OpenAIAPIKey == "" || c.cfg.EmbeddingsModel == "" {
		return nil, fmt.Errorf("%w: OPENAI_API_KEY or EMBEDDINGS_MODEL missing", domain.ErrInvalidArgument)
	}
	body := map[string]any{
		"model": c.cfg.EmbeddingsModel,
		"input": texts,
	}
	b, _ := json.Marshal(body)
	var out struct {
		Data []struct{ Embedding []float64 `json:"embedding"` } `json:"data"`
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
		if err != nil { return err }
		defer func(){ _ = resp.Body.Close() }()
		if resp.StatusCode == 429 {
			slog.Warn("ai provider rate limited", slog.String("provider", "openai"), slog.String("op", "embed"), slog.Int("status", resp.StatusCode), slog.String("x_request_id", resp.Header.Get("X-Request-Id")), slog.String("openai_request_id", resp.Header.Get("Openai-Request-Id")))
			return backoff.Permanent(fmt.Errorf("%w", domain.ErrUpstreamRateLimit))
		}
		if resp.StatusCode < 200 || resp.StatusCode >= 300 {
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
	expo := backoff.NewExponentialBackOff()
	expo.MaxElapsedTime = 20 * time.Second
	bo := backoff.WithContext(expo, ctx)
	if err := backoff.Retry(op, bo); err != nil { return nil, err }
	res := make([][]float32, len(out.Data))
	for i := range out.Data {
		v := make([]float32, len(out.Data[i].Embedding))
		for j := range out.Data[i].Embedding { v[j] = float32(out.Data[i].Embedding[j]) }
		res[i] = v
	}
	return res, nil
}
