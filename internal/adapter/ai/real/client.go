// Package real implements a real AI client backed by OpenRouter and OpenAI APIs.
package real

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"net/http"
	"time"

	backoff "github.com/cenkalti/backoff/v4"

	"github.com/fairyhunter13/ai-cv-evaluator/internal/config"
	"github.com/fairyhunter13/ai-cv-evaluator/internal/domain"
	"github.com/fairyhunter13/ai-cv-evaluator/internal/adapter/observability"
)

// Client implements domain.AIClient using OpenRouter (chat) and OpenAI (embeddings).
type Client struct {
	cfg        config.Config
	chatHC     *http.Client
	embedHC    *http.Client
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
	req, _ := http.NewRequestWithContext(ctx, http.MethodPost, c.cfg.OpenRouterBaseURL+"/chat/completions", bytes.NewReader(b))
	req.Header.Set("Authorization", "Bearer "+c.cfg.OpenRouterAPIKey)
	req.Header.Set("Content-Type", "application/json")
	var out struct {
		Choices []struct{ Message struct{ Content string `json:"content"` } `json:"message"` } `json:"choices"`
	}
	op := func() error {
		start := time.Now()
		resp, err := c.chatHC.Do(req)
		observability.AIRequestsTotal.WithLabelValues("openrouter", "chat").Inc()
		observability.AIRequestDuration.WithLabelValues("openrouter", "chat").Observe(time.Since(start).Seconds())
		if err != nil { return err }
		defer func(){ _ = resp.Body.Close() }()
		if resp.StatusCode == 429 { return backoff.Permanent(fmt.Errorf("%w", domain.ErrUpstreamRateLimit)) }
		if resp.StatusCode < 200 || resp.StatusCode >= 300 { return fmt.Errorf("chat status %d", resp.StatusCode) }
		if err := json.NewDecoder(resp.Body).Decode(&out); err != nil { return err }
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
	req, _ := http.NewRequestWithContext(ctx, http.MethodPost, c.cfg.OpenAIBaseURL+"/embeddings", bytes.NewReader(b))
	req.Header.Set("Authorization", "Bearer "+c.cfg.OpenAIAPIKey)
	req.Header.Set("Content-Type", "application/json")
	var out struct {
		Data []struct{ Embedding []float64 `json:"embedding"` } `json:"data"`
	}
	op := func() error {
		start := time.Now()
		resp, err := c.embedHC.Do(req)
		observability.AIRequestsTotal.WithLabelValues("openai", "embed").Inc()
		observability.AIRequestDuration.WithLabelValues("openai", "embed").Observe(time.Since(start).Seconds())
		if err != nil { return err }
		defer func(){ _ = resp.Body.Close() }()
		if resp.StatusCode == 429 { return backoff.Permanent(fmt.Errorf("%w", domain.ErrUpstreamRateLimit)) }
		if resp.StatusCode < 200 || resp.StatusCode >= 300 { return fmt.Errorf("embed status %d", resp.StatusCode) }
		if err := json.NewDecoder(resp.Body).Decode(&out); err != nil { return err }
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
