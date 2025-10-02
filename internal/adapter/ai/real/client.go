// Package real implements a real AI client backed by OpenRouter and OpenAI APIs.
package real

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync/atomic"
	"time"

	backoff "github.com/cenkalti/backoff/v4"

	"log/slog"

	"github.com/fairyhunter13/ai-cv-evaluator/internal/adapter/observability"
	"github.com/fairyhunter13/ai-cv-evaluator/internal/config"
	"github.com/fairyhunter13/ai-cv-evaluator/internal/domain"
	"github.com/fairyhunter13/ai-cv-evaluator/internal/service/freemodels"
)

// Client implements domain.AIClient using OpenRouter (chat) and OpenAI (embeddings).
type Client struct {
	cfg           config.Config
	chatHC        *http.Client
	embedHC       *http.Client
	freeModelsSvc *freemodels.Service
	modelCounter  int64 // Counter for round-robin model selection
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

	// Use more aggressive timeouts for E2E tests
	chatTimeout := 60 * time.Second  // Increased for free models
	embedTimeout := 30 * time.Second // Increased for embeddings

	// If running in dev environment, use reasonable timeouts for E2E tests
	if cfg.AppEnv == "dev" {
		chatTimeout = 300 * time.Second // Increased for free model reliability (5 minutes)
		embedTimeout = 60 * time.Second // Increased for embeddings
	}

	// Initialize free models service
	freeModelsSvc := freemodels.NewService(cfg.OpenRouterAPIKey, cfg.OpenRouterBaseURL, cfg.FreeModelsRefresh)

	return &Client{
		cfg:           cfg,
		chatHC:        &http.Client{Timeout: chatTimeout},
		embedHC:       &http.Client{Timeout: embedTimeout},
		freeModelsSvc: freeModelsSvc,
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
// This method implements retry logic with model fallback for better reliability.
func (c *Client) ChatJSON(ctx domain.Context, systemPrompt, userPrompt string, maxTokens int) (string, error) {
	if c.cfg.OpenRouterAPIKey == "" {
		slog.Error("OpenRouter API key missing", slog.String("provider", "openrouter"))
		return "", fmt.Errorf("%w: OPENROUTER_API_KEY missing", domain.ErrInvalidArgument)
	}

	// Get free models from the service with retry logic
	slog.Debug("calling free models service to get available models")
	freeModels, err := c.freeModelsSvc.GetFreeModels(ctx)
	if err != nil {
		slog.Error("failed to get free models from service",
			slog.Any("error", err),
			slog.String("service", "freemodels"))

		// Try to refresh models and retry once
		slog.Info("attempting to refresh free models and retry")
		if refreshErr := c.freeModelsSvc.Refresh(ctx); refreshErr != nil {
			slog.Error("failed to refresh free models", slog.Any("error", refreshErr))
			return "", fmt.Errorf("failed to get free models: %w", err)
		}

		// Retry after refresh
		freeModels, err = c.freeModelsSvc.GetFreeModels(ctx)
		if err != nil {
			slog.Error("failed to get free models after refresh",
				slog.Any("error", err),
				slog.String("service", "freemodels"))
			return "", fmt.Errorf("failed to get free models after refresh: %w", err)
		}
	}

	slog.Debug("free models service returned models",
		slog.Int("count", len(freeModels)))

	if len(freeModels) == 0 {
		slog.Error("no free models available", slog.String("provider", "openrouter"))
		return "", fmt.Errorf("no free models available from OpenRouter API")
	}

	// Select model using round-robin rotation with fallback
	modelIndex := atomic.AddInt64(&c.modelCounter, 1) % int64(len(freeModels))
	selectedModel := freeModels[modelIndex]
	model := selectedModel.ID

	// Log available models for debugging
	modelIDs := make([]string, len(freeModels))
	for i, m := range freeModels {
		modelIDs[i] = m.ID
	}
	slog.Debug("available free models", slog.Any("models", modelIDs))

	slog.Info("using free model from OpenRouter API (round-robin)",
		slog.String("model", model),
		slog.String("model_name", selectedModel.Name),
		slog.String("provider", "openrouter"),
		slog.Int("max_tokens", maxTokens),
		slog.Int("total_free_models", len(freeModels)),
		slog.Int64("model_index", modelIndex),
		slog.Int64("model_counter", c.modelCounter))

	// Prepare fallback models (limit to 3 as per OpenRouter API requirement)
	// Exclude the selected model and pick the next 3 models
	fallbackModels := make([]string, 0, 3)
	fallbackCount := 0
	for i := 0; i < len(freeModels) && fallbackCount < 3; i++ {
		if int64(i) != modelIndex { // Skip the selected model
			fallbackModels = append(fallbackModels, freeModels[i].ID)
			fallbackCount++
		}
	}

	slog.Info("calling OpenRouter API", slog.String("provider", "openrouter"), slog.String("model", model), slog.Int("max_tokens", maxTokens))
	body := map[string]any{
		"model":       model,
		"temperature": 0.2,
		"max_tokens":  maxTokens,
		"messages": []map[string]string{
			{"role": "system", "content": systemPrompt},
			{"role": "user", "content": userPrompt},
		},
	}

	// Add fallback models if available
	if len(fallbackModels) > 0 {
		body["models"] = fallbackModels
		slog.Debug("added fallback models", slog.String("fallback_models", fmt.Sprintf("%v", fallbackModels)))
	}
	b, _ := json.Marshal(body)
	slog.Debug("OpenRouter API request body", slog.String("body", string(b)))
	var out struct {
		Model   string `json:"model"`
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
	}
	op := func() error {
		start := time.Now()
		connectionStart := time.Now()
		// Recreate request each attempt to avoid reusing consumed bodies
		r, _ := http.NewRequestWithContext(ctx, http.MethodPost, c.cfg.OpenRouterBaseURL+"/chat/completions", bytes.NewReader(b))
		r.Header.Set("Authorization", "Bearer "+c.cfg.OpenRouterAPIKey)
		r.Header.Set("Content-Type", "application/json")

		// Log connection start
		slog.Debug("starting OpenRouter API connection",
			slog.String("model", model),
			slog.String("endpoint", c.cfg.OpenRouterBaseURL+"/chat/completions"),
			slog.Time("connection_start", connectionStart))

		resp, err := c.chatHC.Do(r)
		connectionDuration := time.Since(connectionStart)

		// Log connection duration
		slog.Info("OpenRouter API connection completed",
			slog.String("model", model),
			slog.Duration("connection_duration", connectionDuration),
			slog.Int("status_code", resp.StatusCode),
			slog.String("x_request_id", resp.Header.Get("X-Request-Id")))

		observability.AIRequestsTotal.WithLabelValues("openrouter", "chat").Inc()
		observability.AIRequestDuration.WithLabelValues("openrouter", "chat").Observe(time.Since(start).Seconds())
		if err != nil {
			return err
		}
		defer func() { _ = resp.Body.Close() }()

		// Read response body once and reuse it
		bodyBytes, err := io.ReadAll(resp.Body)
		if err != nil {
			slog.Error("failed to read response body", slog.String("provider", "openrouter"), slog.Any("error", err))
			return err
		}

		if resp.StatusCode == 429 {
			// Retryable: let backoff handle retries
			slog.Warn("ai provider rate limited", slog.String("provider", "openrouter"), slog.String("op", "chat"), slog.Int("status", resp.StatusCode), slog.String("x_request_id", resp.Header.Get("X-Request-Id")))
			return fmt.Errorf("rate limited: 429")
		}
		if resp.StatusCode >= 400 && resp.StatusCode < 500 {
			// Client error: non-retryable
			bodySnippet := string(bodyBytes)
			if len(bodySnippet) > 512 {
				bodySnippet = bodySnippet[:512]
			}
			slog.Warn("ai provider 4xx", slog.String("provider", "openrouter"), slog.String("op", "chat"), slog.Int("status", resp.StatusCode), slog.String("model", model), slog.String("endpoint", c.cfg.OpenRouterBaseURL+"/chat/completions"), slog.String("x_request_id", resp.Header.Get("X-Request-Id")), slog.String("body", bodySnippet))
			slog.Error("OpenRouter API 4xx error details", slog.String("response_body", bodySnippet), slog.String("request_body", string(b)))
			return backoff.Permanent(fmt.Errorf("chat status %d", resp.StatusCode))
		}
		if resp.StatusCode < 200 || resp.StatusCode >= 300 {
			// 5xx and others: retryable
			bodySnippet := string(bodyBytes)
			if len(bodySnippet) > 512 {
				bodySnippet = bodySnippet[:512]
			}
			slog.Error("ai provider non-2xx", slog.String("provider", "openrouter"), slog.String("op", "chat"), slog.Int("status", resp.StatusCode), slog.String("model", model), slog.String("endpoint", c.cfg.OpenRouterBaseURL+"/chat/completions"), slog.String("x_request_id", resp.Header.Get("X-Request-Id")), slog.String("body", bodySnippet))
			return fmt.Errorf("chat status %d", resp.StatusCode)
		}
		if err := json.Unmarshal(bodyBytes, &out); err != nil {
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

	// Log successful API call with model verification
	actualModel := "unknown"
	if len(out.Choices) > 0 && out.Choices[0].Message.Content != "" {
		// Check if the actual model used was different from requested
		if out.Model != "" && out.Model != model {
			slog.Warn("model substitution detected",
				slog.String("requested_model", model),
				slog.String("actual_model", out.Model),
				slog.String("provider", "openrouter"))
			actualModel = out.Model
		} else {
			actualModel = model
		}
	}

	slog.Info("OpenRouter API call successful",
		slog.String("provider", "openrouter"),
		slog.Int("choices_count", len(out.Choices)),
		slog.String("requested_model", model),
		slog.String("actual_model", actualModel))
	return out.Choices[0].Message.Content, nil
}

// ChatJSONWithRetry calls OpenRouter with retry logic and model fallback for better reliability.
func (c *Client) ChatJSONWithRetry(ctx domain.Context, systemPrompt, userPrompt string, maxTokens int) (string, error) {
	if c.cfg.OpenRouterAPIKey == "" {
		slog.Error("OpenRouter API key missing", slog.String("provider", "openrouter"))
		return "", fmt.Errorf("%w: OPENROUTER_API_KEY missing", domain.ErrInvalidArgument)
	}

	// Get free models from the service with retry logic
	slog.Debug("calling free models service to get available models")
	freeModels, err := c.freeModelsSvc.GetFreeModels(ctx)
	if err != nil {
		slog.Error("failed to get free models from service",
			slog.Any("error", err),
			slog.String("service", "freemodels"))

		// Try to refresh models and retry once
		slog.Info("attempting to refresh free models")
		if refreshErr := c.freeModelsSvc.Refresh(ctx); refreshErr != nil {
			slog.Error("failed to refresh free models", slog.Any("error", refreshErr))
			return "", fmt.Errorf("get free models: %w", err)
		}

		// Retry getting models after refresh
		freeModels, err = c.freeModelsSvc.GetFreeModels(ctx)
		if err != nil {
			slog.Error("failed to get free models after refresh", slog.Any("error", err))
			return "", fmt.Errorf("get free models after refresh: %w", err)
		}
	}

	slog.Info("retrieved free models from service",
		slog.String("provider", "openrouter"),
		slog.Int("count", len(freeModels)))

	if len(freeModels) == 0 {
		slog.Error("no free models available", slog.String("provider", "openrouter"))
		return "", fmt.Errorf("no free models available from OpenRouter API")
	}

	// Use enhanced model switching with timeout handling
	return c.chatJSONWithEnhancedModelSwitching(ctx, systemPrompt, userPrompt, maxTokens, freeModels)
}

// chatJSONWithEnhancedModelSwitching implements intelligent model switching with timeout handling.
func (c *Client) chatJSONWithEnhancedModelSwitching(ctx domain.Context, systemPrompt, userPrompt string, maxTokens int, freeModels []freemodels.Model) (string, error) {
	// Configuration for enhanced model switching
	const (
		maxRetriesPerModel      = 2
		modelTimeout            = 60 * time.Second // 60 seconds per model attempt
		circuitBreakerThreshold = 3                // Switch after 3 consecutive failures
	)

	// Track model performance for intelligent selection
	modelFailures := make(map[string]int)
	modelSuccesses := make(map[string]int)

	// Try each model with enhanced timeout and circuit breaker logic
	for modelIndex, model := range freeModels {
		modelID := model.ID
		modelName := model.Name

		// Skip models that have failed too many times (circuit breaker)
		if modelFailures[modelID] >= circuitBreakerThreshold {
			slog.Warn("model circuit breaker triggered, skipping model",
				slog.String("model", modelID),
				slog.String("model_name", modelName),
				slog.Int("failures", modelFailures[modelID]),
				slog.Int("threshold", circuitBreakerThreshold))
			continue
		}

		slog.Info("trying model with enhanced switching",
			slog.String("model", modelID),
			slog.String("model_name", modelName),
			slog.Int("model_index", modelIndex),
			slog.Int("total_models", len(freeModels)),
			slog.Int("previous_failures", modelFailures[modelID]),
			slog.Int("previous_successes", modelSuccesses[modelID]))

		// Try this model with retry logic and timeout handling
		for attempt := 1; attempt <= maxRetriesPerModel; attempt++ {
			slog.Info("model attempt with timeout",
				slog.String("model", modelID),
				slog.Int("attempt", attempt),
				slog.Int("max_retries", maxRetriesPerModel),
				slog.Duration("timeout", modelTimeout))

			// Create a timeout context for this specific model attempt
			modelCtx, cancel := context.WithTimeout(ctx, modelTimeout)

			// Use a channel to detect if the call completes or times out
			resultChan := make(chan struct {
				result string
				err    error
			}, 1)

			// Make the AI call in a goroutine to handle timeouts properly
			go func() {
				result, err := c.callOpenRouterWithModel(modelCtx, modelID, systemPrompt, userPrompt, maxTokens)
				resultChan <- struct {
					result string
					err    error
				}{result, err}
			}()

			// Wait for either completion or timeout
			select {
			case result := <-resultChan:
				cancel() // Clean up the timeout context

				if result.err == nil {
					// Enhanced refusal detection with comprehensive validation
					refusalDetected, refusalReason := c.detectRefusalWithValidation(ctx, result.result)
					if refusalDetected {
						slog.Warn("model returned refusal response, switching to next model",
							slog.String("model", modelID),
							slog.String("model_name", modelName),
							slog.String("refusal_reason", refusalReason),
							slog.String("response_preview", truncateString(result.result, 100)))
						modelFailures[modelID]++
						break // Skip to next model immediately
					}

					// Success! Update success counter and return
					modelSuccesses[modelID]++
					slog.Info("model succeeded with enhanced switching",
						slog.String("model", modelID),
						slog.String("model_name", modelName),
						slog.Int("attempt", attempt),
						slog.Int("response_length", len(result.result)),
						slog.Int("total_successes", modelSuccesses[modelID]))
					return result.result, nil
				}

				// Handle different types of errors
				slog.Warn("model attempt failed with enhanced switching",
					slog.String("model", modelID),
					slog.String("model_name", modelName),
					slog.Int("attempt", attempt),
					slog.Any("error", result.err))

				// Check if it's a timeout error
				if modelCtx.Err() == context.DeadlineExceeded {
					slog.Warn("model timeout exceeded",
						slog.String("model", modelID),
						slog.String("model_name", modelName),
						slog.Duration("timeout", modelTimeout))
					modelFailures[modelID]++
				} else {
					// Other types of errors
					modelFailures[modelID]++
				}

			case <-modelCtx.Done():
				// Timeout occurred
				cancel()
				modelFailures[modelID]++
				slog.Warn("model timeout exceeded, switching to next model",
					slog.String("model", modelID),
					slog.String("model_name", modelName),
					slog.Duration("timeout", modelTimeout),
					slog.Int("failures", modelFailures[modelID]))
			}

			// If this is not the last attempt for this model, wait before retrying
			if attempt < maxRetriesPerModel {
				backoffDuration := time.Duration(attempt) * 2 * time.Second
				slog.Info("waiting before model retry",
					slog.String("model", modelID),
					slog.Duration("backoff", backoffDuration))
				time.Sleep(backoffDuration)
			}
		}

		slog.Warn("model failed after all retries, trying next model",
			slog.String("model", modelID),
			slog.String("model_name", modelName),
			slog.Int("model_index", modelIndex),
			slog.Int("total_models", len(freeModels)),
			slog.Int("total_failures", modelFailures[modelID]),
			slog.Int("total_successes", modelSuccesses[modelID]))
	}

	// Log final statistics
	slog.Error("all models failed with enhanced switching",
		slog.Int("total_models_tried", len(freeModels)),
		slog.Any("model_failures", modelFailures),
		slog.Any("model_successes", modelSuccesses))

	return "", fmt.Errorf("all models failed after enhanced switching (tried %d models)", len(freeModels))
}

// callOpenRouterWithModel makes a single call to OpenRouter with a specific model.
func (c *Client) callOpenRouterWithModel(ctx domain.Context, model, systemPrompt, userPrompt string, maxTokens int) (string, error) {
	body := map[string]any{
		"model":       model,
		"temperature": 0.2,
		"max_tokens":  maxTokens,
		"messages": []map[string]string{
			{"role": "system", "content": systemPrompt},
			{"role": "user", "content": userPrompt},
		},
	}

	b, _ := json.Marshal(body)
	slog.Debug("OpenRouter API request body", slog.String("body", string(b)))

	var out struct {
		Model   string `json:"model"`
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
	}

	op := func() error {
		start := time.Now()
		connectionStart := time.Now()
		r, _ := http.NewRequestWithContext(ctx, http.MethodPost, c.cfg.OpenRouterBaseURL+"/chat/completions", bytes.NewReader(b))
		r.Header.Set("Authorization", "Bearer "+c.cfg.OpenRouterAPIKey)
		r.Header.Set("Content-Type", "application/json")

		// Log connection start for model switching
		slog.Debug("starting OpenRouter API connection (model switching)",
			slog.String("model", model),
			slog.String("endpoint", c.cfg.OpenRouterBaseURL+"/chat/completions"),
			slog.Time("connection_start", connectionStart))

		resp, err := c.chatHC.Do(r)
		connectionDuration := time.Since(connectionStart)

		// Log connection duration for model switching
		slog.Info("OpenRouter API connection completed (model switching)",
			slog.String("model", model),
			slog.Duration("connection_duration", connectionDuration),
			slog.Int("status_code", resp.StatusCode),
			slog.String("x_request_id", resp.Header.Get("X-Request-Id")))

		observability.AIRequestsTotal.WithLabelValues("openrouter", "chat_retry").Inc()
		observability.AIRequestDuration.WithLabelValues("openrouter", "chat_retry").Observe(time.Since(start).Seconds())
		if err != nil {
			return err
		}
		defer func() { _ = resp.Body.Close() }()

		bodyBytes, err := io.ReadAll(resp.Body)
		if err != nil {
			slog.Error("failed to read response body", slog.String("provider", "openrouter"), slog.Any("error", err))
			return err
		}

		if resp.StatusCode == 429 {
			slog.Warn("ai provider rate limited", slog.String("provider", "openrouter"), slog.String("op", "chat_retry"), slog.Int("status", resp.StatusCode))
			return fmt.Errorf("rate limited: 429")
		}
		if resp.StatusCode >= 400 && resp.StatusCode < 500 {
			bodySnippet := string(bodyBytes)
			if len(bodySnippet) > 512 {
				bodySnippet = bodySnippet[:512]
			}
			slog.Warn("ai provider 4xx", slog.String("provider", "openrouter"), slog.String("op", "chat_retry"), slog.Int("status", resp.StatusCode), slog.String("model", model), slog.String("endpoint", c.cfg.OpenRouterBaseURL+"/chat/completions"), slog.String("x_request_id", resp.Header.Get("X-Request-Id")), slog.String("body", bodySnippet))
			return backoff.Permanent(fmt.Errorf("chat status %d", resp.StatusCode))
		}
		if resp.StatusCode < 200 || resp.StatusCode >= 300 {
			bodySnippet := string(bodyBytes)
			if len(bodySnippet) > 512 {
				bodySnippet = bodySnippet[:512]
			}
			slog.Error("ai provider non-2xx", slog.String("provider", "openrouter"), slog.String("op", "chat_retry"), slog.Int("status", resp.StatusCode), slog.String("model", model), slog.String("endpoint", c.cfg.OpenRouterBaseURL+"/chat/completions"), slog.String("x_request_id", resp.Header.Get("X-Request-Id")), slog.String("body", bodySnippet))
			return fmt.Errorf("chat status %d", resp.StatusCode)
		}
		if err := json.Unmarshal(bodyBytes, &out); err != nil {
			slog.Error("ai provider decode error", slog.String("provider", "openrouter"), slog.String("op", "chat_retry"), slog.String("model", model), slog.String("endpoint", c.cfg.OpenRouterBaseURL+"/chat/completions"), slog.Any("error", err))
			return err
		}
		return nil
	}

	expo := c.getBackoffConfig()
	bo := backoff.WithContext(expo, ctx)

	if err := backoff.Retry(op, bo); err != nil {
		slog.Error("OpenRouter API failed after retries", slog.String("provider", "openrouter"), slog.String("model", model), slog.Any("error", err))
		return "", fmt.Errorf("openrouter api failed for model %s: %w", model, err)
	}

	if len(out.Choices) == 0 {
		slog.Error("OpenRouter API returned empty choices", slog.String("provider", "openrouter"), slog.String("model", model))
		return "", fmt.Errorf("openrouter api returned empty choices for model %s", model)
	}

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
		connectionStart := time.Now()
		// Recreate request each attempt to avoid reusing consumed bodies
		r, _ := http.NewRequestWithContext(ctx, http.MethodPost, c.cfg.OpenAIBaseURL+"/embeddings", bytes.NewReader(b))
		r.Header.Set("Authorization", "Bearer "+c.cfg.OpenAIAPIKey)
		r.Header.Set("Content-Type", "application/json")

		// Log connection start for embeddings
		slog.Debug("starting OpenAI API connection (embeddings)",
			slog.String("model", c.cfg.EmbeddingsModel),
			slog.String("endpoint", c.cfg.OpenAIBaseURL+"/embeddings"),
			slog.Time("connection_start", connectionStart))

		resp, err := c.embedHC.Do(r)
		connectionDuration := time.Since(connectionStart)

		// Log connection duration for embeddings
		slog.Info("OpenAI API connection completed (embeddings)",
			slog.String("model", c.cfg.EmbeddingsModel),
			slog.Duration("connection_duration", connectionDuration),
			slog.Int("status_code", resp.StatusCode),
			slog.String("x_request_id", resp.Header.Get("X-Request-Id")))

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

// CleanCoTResponse sends a response with CoT leakage back to OpenRouter for cleaning
func (c *Client) CleanCoTResponse(ctx domain.Context, originalResponse string) (string, error) {
	slog.Info("cleaning CoT leakage from response", slog.Int("original_length", len(originalResponse)))

	// Create a cleaning prompt
	cleaningPrompt := `You are a response cleaner. Remove all chain-of-thought reasoning, step-by-step analysis, and explanatory text from the following response. Return ONLY the clean JSON data without any reasoning, explanations, or step-by-step analysis.

CRITICAL: Respond with ONLY valid JSON. No explanations, reasoning, or chain-of-thought in your response.

Original response to clean:
` + originalResponse

	// Use a different model for cleaning to avoid the same CoT patterns
	// Get free models and select a different one for cleaning
	freeModels, err := c.freeModelsSvc.GetFreeModels(ctx)
	if err != nil {
		slog.Error("failed to get free models for cleaning", slog.Any("error", err))
		return "", fmt.Errorf("failed to get free models for cleaning: %w", err)
	}

	if len(freeModels) == 0 {
		slog.Error("no free models available for cleaning")
		return "", fmt.Errorf("no free models available for cleaning")
	}

	// Select a different model for cleaning (use a different index)
	cleaningModelIndex := (atomic.AddInt64(&c.modelCounter, 1) + 1) % int64(len(freeModels))
	cleaningModel := freeModels[cleaningModelIndex]

	slog.Info("using cleaning model",
		slog.String("model", cleaningModel.ID),
		slog.String("model_name", cleaningModel.Name),
		slog.Int64("model_index", cleaningModelIndex))

	// Prepare fallback models for cleaning
	fallbackModels := make([]string, 0, 3)
	fallbackCount := 0
	for i := 0; i < len(freeModels) && fallbackCount < 3; i++ {
		if int64(i) != cleaningModelIndex {
			fallbackModels = append(fallbackModels, freeModels[i].ID)
			fallbackCount++
		}
	}

	// Call OpenRouter API for cleaning
	body := map[string]any{
		"model":       cleaningModel.ID,
		"temperature": 0.1, // Lower temperature for more consistent cleaning
		"max_tokens":  1000,
		"messages": []map[string]string{
			{"role": "system", "content": cleaningPrompt},
			{"role": "user", "content": "Clean this response and return only the JSON data:"},
		},
	}

	// Add fallback models if available
	if len(fallbackModels) > 0 {
		body["models"] = fallbackModels
	}

	b, _ := json.Marshal(body)
	slog.Debug("CoT cleaning request body", slog.String("body", string(b)))

	var out struct {
		Model   string `json:"model"`
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
	}

	op := func() error {
		start := time.Now()
		r, _ := http.NewRequestWithContext(ctx, http.MethodPost, c.cfg.OpenRouterBaseURL+"/chat/completions", bytes.NewReader(b))
		r.Header.Set("Authorization", "Bearer "+c.cfg.OpenRouterAPIKey)
		r.Header.Set("Content-Type", "application/json")
		resp, err := c.chatHC.Do(r)
		observability.AIRequestsTotal.WithLabelValues("openrouter", "cot_cleaning").Inc()
		observability.AIRequestDuration.WithLabelValues("openrouter", "cot_cleaning").Observe(time.Since(start).Seconds())
		if err != nil {
			return err
		}
		defer func() { _ = resp.Body.Close() }()

		if resp.StatusCode == 429 {
			slog.Warn("ai provider rate limited during CoT cleaning", slog.String("provider", "openrouter"), slog.String("op", "cot_cleaning"), slog.Int("status", resp.StatusCode))
			return fmt.Errorf("rate limited: 429")
		}
		if resp.StatusCode >= 400 && resp.StatusCode < 500 {
			bodySnippet := readSnippet(resp.Body, 512)
			slog.Warn("ai provider 4xx during CoT cleaning", slog.String("provider", "openrouter"), slog.String("op", "cot_cleaning"), slog.Int("status", resp.StatusCode), slog.String("model", cleaningModel.ID), slog.String("body", bodySnippet))
			return backoff.Permanent(fmt.Errorf("cot cleaning status %d", resp.StatusCode))
		}
		if resp.StatusCode < 200 || resp.StatusCode >= 300 {
			bodySnippet := readSnippet(resp.Body, 512)
			slog.Error("ai provider non-2xx during CoT cleaning", slog.String("provider", "openrouter"), slog.String("op", "cot_cleaning"), slog.Int("status", resp.StatusCode), slog.String("model", cleaningModel.ID), slog.String("body", bodySnippet))
			return fmt.Errorf("cot cleaning status %d", resp.StatusCode)
		}
		if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
			slog.Error("ai provider decode error during CoT cleaning", slog.String("provider", "openrouter"), slog.String("op", "cot_cleaning"), slog.String("model", cleaningModel.ID), slog.Any("error", err))
			return err
		}
		return nil
	}

	expo := c.getBackoffConfig()
	bo := backoff.WithContext(expo, ctx)

	slog.Info("starting CoT cleaning retry logic", slog.String("provider", "openrouter"), slog.Duration("max_elapsed", expo.MaxElapsedTime))
	if err := backoff.Retry(op, bo); err != nil {
		slog.Error("CoT cleaning failed after retries", slog.String("provider", "openrouter"), slog.Any("error", err))
		return "", fmt.Errorf("cot cleaning failed: %w", err)
	}

	if len(out.Choices) == 0 {
		slog.Error("CoT cleaning returned empty choices", slog.String("provider", "openrouter"))
		return "", errors.New("empty choices from CoT cleaning")
	}

	cleanedResponse := out.Choices[0].Message.Content
	slog.Info("CoT cleaning successful",
		slog.String("provider", "openrouter"),
		slog.Int("original_length", len(originalResponse)),
		slog.Int("cleaned_length", len(cleanedResponse)),
		slog.String("cleaning_model", out.Model))

	return cleanedResponse, nil
}

// isRefusalResponse detects if the AI model response is a refusal to process the request.
func isRefusalResponse(response string) bool {
	// Enhanced refusal detection with multiple categories
	refusalPatterns := map[string][]string{
		"apology_phrases": {
			"i'm sorry", "i apologize", "i regret", "unfortunately",
			"i'm afraid", "i'm unable to help", "i cannot help",
		},
		"capability_denials": {
			"i cannot", "i can't", "i am unable", "i'm unable",
			"i am not able", "i'm not able", "i don't have access",
			"i don't have the ability", "i lack the capability",
		},
		"security_concerns": {
			"access or reveal", "internal system instructions",
			"system instructions", "security concerns",
			"potentially harmful", "safety guidelines",
			"content policy", "usage guidelines",
		},
		"request_rejections": {
			"legitimate query", "can't assist with", "cannot assist with",
			"i cannot fulfill", "i cannot provide", "i cannot generate",
			"i cannot create", "i cannot write", "i cannot analyze",
		},
		"ethical_concerns": {
			"ethical guidelines", "responsible ai", "ai safety",
			"harmful content", "inappropriate", "unethical",
			"violates guidelines", "against policy",
		},
		"technical_limitations": {
			"technical limitations", "system limitations",
			"processing error", "unable to process",
			"request too complex", "input too short",
		},
	}

	lowerResponse := strings.ToLower(response)

	// Check each category
	for category, phrases := range refusalPatterns {
		for _, phrase := range phrases {
			if strings.Contains(lowerResponse, phrase) {
				slog.Debug("refusal response detected",
					slog.String("category", category),
					slog.String("phrase", phrase),
					slog.String("response_preview", truncateString(response, 200)))
				return true
			}
		}
	}

	// Additional pattern-based detection
	if isRefusalByPattern(response) {
		return true
	}

	return false
}

// isRefusalByPattern detects refusal responses using advanced pattern matching
func isRefusalByPattern(response string) bool {
	// Very short responses (likely refusals)
	if len(strings.TrimSpace(response)) < 50 {
		lowerResponse := strings.ToLower(response)
		shortRefusalIndicators := []string{
			"i can't", "i cannot", "sorry", "unable", "no",
		}
		for _, indicator := range shortRefusalIndicators {
			if strings.Contains(lowerResponse, indicator) {
				return true
			}
		}
	}

	// Responses that start with apologies
	trimmed := strings.TrimSpace(response)
	if len(trimmed) > 10 {
		firstWords := strings.ToLower(trimmed[:minInt(50, len(trimmed))])
		apologyStarters := []string{
			"i'm sorry", "i apologize", "unfortunately", "i'm afraid",
		}
		for _, starter := range apologyStarters {
			if strings.HasPrefix(firstWords, starter) {
				return true
			}
		}
	}

	// Responses that contain policy/guideline references
	lowerResponse := strings.ToLower(response)
	policyIndicators := []string{
		"policy", "guidelines", "terms", "conditions", "rules",
		"restrictions", "limitations", "boundaries",
	}
	for _, indicator := range policyIndicators {
		if strings.Contains(lowerResponse, indicator) {
			return true
		}
	}

	return false
}

// min returns the minimum of two integers
func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// detectRefusalWithValidation performs comprehensive refusal detection using multiple methods.
func (c *Client) detectRefusalWithValidation(_ context.Context, response string) (bool, string) {
	// Method 1: Fast code-based detection (immediate)
	if isRefusalResponse(response) {
		return true, "code-based pattern detection"
	}

	// Method 2: AI-powered detection (more accurate but slower)
	// Note: This would require the RefusalDetector, but we'll keep it simple for now
	// In a full implementation, you would use the RefusalDetector here

	// Method 3: Response quality checks
	if c.isLowQualityResponse(response) {
		return true, "low quality response detected"
	}

	return false, ""
}

// isLowQualityResponse detects low-quality responses that might indicate refusal.
func (c *Client) isLowQualityResponse(response string) bool {
	// Very short responses
	if len(strings.TrimSpace(response)) < 30 {
		return true
	}

	// Responses that are mostly punctuation or whitespace
	trimmed := strings.TrimSpace(response)
	if len(trimmed) < 10 {
		return true
	}

	// Responses that contain only common words without substance
	words := strings.Fields(strings.ToLower(trimmed))
	if len(words) < 5 {
		return true
	}

	// Check for responses that are just repeated words
	if c.hasExcessiveRepetition(words) {
		return true
	}

	return false
}

// hasExcessiveRepetition checks if the response has excessive word repetition.
func (c *Client) hasExcessiveRepetition(words []string) bool {
	if len(words) < 10 {
		return false
	}

	wordCount := make(map[string]int)
	for _, word := range words {
		wordCount[word]++
	}

	// If any word appears more than 30% of the time, it's likely repetitive
	maxCount := 0
	for _, count := range wordCount {
		if count > maxCount {
			maxCount = count
		}
	}

	return maxCount > len(words)/3
}

// truncateString truncates a string to the specified length.
func truncateString(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}
