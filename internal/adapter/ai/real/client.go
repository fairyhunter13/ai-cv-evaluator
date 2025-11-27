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
	"sort"
	"strconv"
	"strings"
	"sync/atomic"
	"time"

	backoff "github.com/cenkalti/backoff/v4"

	"log/slog"

	aiadapter "github.com/fairyhunter13/ai-cv-evaluator/internal/adapter/ai"
	"github.com/fairyhunter13/ai-cv-evaluator/internal/adapter/observability"
	"github.com/fairyhunter13/ai-cv-evaluator/internal/config"
	"github.com/fairyhunter13/ai-cv-evaluator/internal/domain"
	"github.com/fairyhunter13/ai-cv-evaluator/internal/service/freemodels"
)

// Client implements domain.AIClient using OpenRouter (chat) and OpenAI (embeddings).
type Client struct {
	cfg                  config.Config
	chatHC               *http.Client
	embedHC              *http.Client
	freeModelsSvc        *freemodels.Service
	modelCounter         int64                     // Counter for round-robin model selection
	providerCounter      int64                     // Counter to balance load between Groq and OpenRouter when both are available
	rlc                  *aiadapter.RateLimitCache // Client-side rate-limit model cache
	lastORCall           atomic.Int64              // unix nano timestamp of last OpenRouter call (client-level throttle)
	lastGroqCall         atomic.Int64              // unix nano timestamp of last Groq call (client-level throttle)
	groqBlocked          atomic.Int64              // unix nano timestamp until which Groq is blocked due to 429
	openRouterBlocked    atomic.Int64              // unix nano timestamp until which OpenRouter is blocked (legacy provider-level block)
	openRouterKeyCounter int64
	openRouter1Blocked   atomic.Int64 // unix nano timestamp until which OpenRouter primary key is blocked due to 429
	openRouter2Blocked   atomic.Int64 // unix nano timestamp until which OpenRouter secondary key is blocked due to 429
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

	// Initialize free models service. Prefer primary OpenRouter key but
	// transparently fall back to OPENROUTER_API_KEY_2 when only the
	// secondary key is configured.
	openRouterKey := cfg.OpenRouterAPIKey
	if openRouterKey == "" {
		openRouterKey = cfg.OpenRouterAPIKey2
	}
	freeModelsSvc := freemodels.NewService(openRouterKey, cfg.OpenRouterBaseURL, cfg.FreeModelsRefresh)

	return &Client{
		cfg:           cfg,
		chatHC:        &http.Client{Timeout: chatTimeout},
		embedHC:       &http.Client{Timeout: embedTimeout},
		freeModelsSvc: freeModelsSvc,
		rlc:           aiadapter.NewRateLimitCache(),
	}
}

// getOpenRouterAPIKey returns an OpenRouter API key to use for this request.
// If both OPENROUTER_API_KEY and OPENROUTER_API_KEY_2 are configured, it
// distributes calls between them in a simple round-robin. Whitespace is
// trimmed and empty keys are ignored.
func (c *Client) getOpenRouterAPIKey() string {
	k1 := strings.TrimSpace(c.cfg.OpenRouterAPIKey)
	k2 := strings.TrimSpace(c.cfg.OpenRouterAPIKey2)
	// Only primary configured
	if k1 != "" && k2 == "" {
		return k1
	}
	// Only secondary configured
	if k2 != "" && k1 == "" {
		return k2
	}
	// Both configured: simple round-robin across the two accounts to spread
	// free-tier usage while still respecting global provider-level throttling.
	if k1 != "" && k2 != "" {
		if atomic.AddInt64(&c.openRouterKeyCounter, 1)%2 == 0 {
			return k1
		}
		return k2
	}
	// Neither configured
	return ""
}

// getOpenRouterKeys returns the configured OpenRouter API keys (primary, secondary),
// both trimmed. Either value may be empty when not configured.
func (c *Client) getOpenRouterKeys() (string, string) {
	return strings.TrimSpace(c.cfg.OpenRouterAPIKey), strings.TrimSpace(c.cfg.OpenRouterAPIKey2)
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
// nolint:gocyclo // Function is intentionally complex due to robust retry, logging, and fallback logic.
func (c *Client) ChatJSON(ctx domain.Context, systemPrompt, userPrompt string, maxTokens int) (string, error) {
	groqKey := strings.TrimSpace(c.cfg.GroqAPIKey)
	hasGroq := groqKey != ""
	openRouterKey := c.getOpenRouterAPIKey()
	hasOpenRouter := openRouterKey != ""

	// 1) Primary: Groq when configured and not blocked
	var groqErr error
	if hasGroq && !c.isGroqBlocked() {
		res, err := c.callGroqChat(ctx, systemPrompt, userPrompt, maxTokens)
		if err == nil {
			return res, nil
		}
		groqErr = err
		slog.Warn("Groq ChatJSON primary attempt failed",
			slog.String("provider", "groq"),
			slog.Any("error", err))

		// If OpenRouter is not configured, surface Groq error directly.
		if !hasOpenRouter {
			return "", groqErr
		}
	} else if hasGroq && c.isGroqBlocked() {
		slog.Info("skipping Groq due to active rate limit block", slog.String("provider", "groq"))
		groqErr = errors.New("groq rate limited and blocked")
	}

	// 2) Secondary: OpenRouter free models
	if !hasOpenRouter {
		slog.Error("OpenRouter API key missing", slog.String("provider", "openrouter"))
		if hasGroq {
			// Normally unreachable because !hasOpenRouter with hasGroq returns above, but keep for safety.
			return "", groqErr
		}
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

	// Rate-limit-aware selection: prefer unblocked models, then shortest wait time
	available := make([]freemodels.Model, 0, len(freeModels))
	blocked := make([]freemodels.Model, 0)
	if c.rlc != nil {
		for _, m := range freeModels {
			if c.rlc.RemainingBlockDuration(m.ID) <= 0 {
				available = append(available, m)
			} else {
				blocked = append(blocked, m)
			}
		}
		// Sort blocked by remaining wait ascending
		sort.Slice(blocked, func(i, j int) bool {
			return c.rlc.RemainingBlockDuration(blocked[i].ID) < c.rlc.RemainingBlockDuration(blocked[j].ID)
		})
	} else {
		available = append(available, freeModels...)
	}

	var selectedModel freemodels.Model
	fallbackModels := make([]string, 0, 3)

	if len(available) > 0 {
		// Round-robin only among available
		counter := atomic.AddInt64(&c.modelCounter, 1)
		idx := int(counter % int64(len(available)))
		selectedModel = available[idx]
		// Fill fallbacks: remaining available (cyclic) then shortest-wait blocked
		for i := 0; i < len(available) && len(fallbackModels) < 3; i++ {
			pos := (idx + 1 + i) % len(available)
			cand := available[pos]
			if cand.ID != selectedModel.ID {
				fallbackModels = append(fallbackModels, cand.ID)
			}
		}
		for i := 0; i < len(blocked) && len(fallbackModels) < 3; i++ {
			fallbackModels = append(fallbackModels, blocked[i].ID)
		}
	} else {
		// All blocked: try cheap paid models first, then fall back to shortest-wait free models
		paidModels, err := c.freeModelsSvc.GetCheapestPaidModels(ctx, 2)
		if err == nil && len(paidModels) > 0 {
			// Prefer the cheapest paid as primary, then remaining paid, then shortest-wait free
			selectedModel = paidModels[0]
			slog.Info("using paid model as fallback (all free models blocked)",
				slog.String("model", selectedModel.ID),
				slog.String("model_name", selectedModel.Name),
				slog.Int("paid_models_available", len(paidModels)))
			for i := 1; i < len(paidModels) && len(fallbackModels) < 3; i++ {
				fallbackModels = append(fallbackModels, paidModels[i].ID)
			}
			// Append shortest-wait free models as additional fallbacks
			all := append([]freemodels.Model{}, freeModels...)
			sort.Slice(all, func(i, j int) bool {
				return c.rlc.RemainingBlockDuration(all[i].ID) < c.rlc.RemainingBlockDuration(all[j].ID)
			})
			for i := 0; i < len(all) && len(fallbackModels) < 3; i++ {
				fallbackModels = append(fallbackModels, all[i].ID)
			}
		} else {
			// No paid models available or error: use shortest-wait free models
			slog.Warn("paid model fallback unavailable, using shortest-wait free models",
				slog.Any("error", err),
				slog.Int("free_models_count", len(freeModels)))
			all := append([]freemodels.Model{}, freeModels...)
			sort.Slice(all, func(i, j int) bool {
				return c.rlc.RemainingBlockDuration(all[i].ID) < c.rlc.RemainingBlockDuration(all[j].ID)
			})
			selectedModel = all[0]
			for i := 1; i < len(all) && len(fallbackModels) < 3; i++ {
				fallbackModels = append(fallbackModels, all[i].ID)
			}
		}
	}

	model := selectedModel.ID

	// Log available models and selection details
	modelIDs := make([]string, len(freeModels))
	for i, m := range freeModels {
		modelIDs[i] = m.ID
	}
	slog.Debug("available free models", slog.Any("models", modelIDs))

	selectionLog := []any{
		slog.String("model", model),
		slog.String("model_name", selectedModel.Name),
		slog.String("provider", "openrouter"),
		slog.Int("max_tokens", maxTokens),
		slog.Int("total_free_models", len(freeModels)),
	}
	if c.rlc != nil {
		selectionLog = append(selectionLog,
			slog.Int("available_models", len(available)),
			slog.Int("blocked_models", len(blocked)))
	}
	slog.Info("using free model (rate-limit-aware round-robin)", selectionLog...)

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
	openRouterKey = c.getOpenRouterAPIKey()
	op := func() error {
		start := time.Now()
		connectionStart := time.Now()
		// Recreate request each attempt to avoid reusing consumed bodies
		// Client-level minimal spacing between OpenRouter calls to reduce 429s
		c.waitOpenRouterMinInterval()

		r, _ := http.NewRequestWithContext(ctx, http.MethodPost, c.cfg.OpenRouterBaseURL+"/chat/completions", bytes.NewReader(b))
		r.Header.Set("Authorization", "Bearer "+openRouterKey)
		r.Header.Set("Content-Type", "application/json")
		if ref := strings.TrimSpace(c.cfg.OpenRouterReferer); ref != "" {
			r.Header.Set("HTTP-Referer", ref)
		}
		if title := strings.TrimSpace(c.cfg.OpenRouterTitle); title != "" {
			r.Header.Set("X-Title", title)
		}

		// Log connection start
		slog.Debug("starting OpenRouter API connection",
			slog.String("model", model),
			slog.String("endpoint", c.cfg.OpenRouterBaseURL+"/chat/completions"),
			slog.Time("connection_start", connectionStart))

		resp, err := c.chatHC.Do(r)
		c.markOpenRouterCall()
		connectionDuration := time.Since(connectionStart)

		if err != nil {
			// Log without touching resp
			slog.Info("OpenRouter API connection attempt failed",
				slog.String("model", model),
				slog.Duration("connection_duration", connectionDuration))
			observability.AIRequestsTotal.WithLabelValues("openrouter", "chat").Inc()
			observability.AIRequestDuration.WithLabelValues("openrouter", "chat").Observe(time.Since(start).Seconds())
			return err
		}

		// Log connection duration
		slog.Info("OpenRouter API connection completed",
			slog.String("model", model),
			slog.Duration("connection_duration", connectionDuration),
			slog.Int("status_code", resp.StatusCode),
			slog.String("x_request_id", resp.Header.Get("X-Request-Id")))

		observability.AIRequestsTotal.WithLabelValues("openrouter", "chat").Inc()
		observability.AIRequestDuration.WithLabelValues("openrouter", "chat").Observe(time.Since(start).Seconds())
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
			retryAfter := parseRetryAfterHeader(resp.Header.Get("Retry-After"))
			if c.rlc != nil {
				c.rlc.RecordRateLimit(model, retryAfter)
			}
			c.blockOpenRouter(retryAfter)
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
			if c.rlc != nil {
				c.rlc.RecordFailure(model)
			}
			return backoff.Permanent(fmt.Errorf("chat status %d", resp.StatusCode))
		}
		if resp.StatusCode < 200 || resp.StatusCode >= 300 {
			// 5xx and others: retryable
			bodySnippet := string(bodyBytes)
			if len(bodySnippet) > 512 {
				bodySnippet = bodySnippet[:512]
			}
			slog.Error("ai provider non-2xx", slog.String("provider", "openrouter"), slog.String("op", "chat"), slog.Int("status", resp.StatusCode), slog.String("model", model), slog.String("endpoint", c.cfg.OpenRouterBaseURL+"/chat/completions"), slog.String("x_request_id", resp.Header.Get("X-Request-Id")), slog.String("body", bodySnippet))
			if c.rlc != nil {
				c.rlc.RecordFailure(model)
			}
			return fmt.Errorf("chat status %d", resp.StatusCode)
		}
		if err := json.Unmarshal(bodyBytes, &out); err != nil {
			slog.Error("ai provider decode error", slog.String("provider", "openrouter"), slog.String("op", "chat"), slog.String("model", model), slog.String("endpoint", c.cfg.OpenRouterBaseURL+"/chat/completions"), slog.Any("error", err))
			if c.rlc != nil {
				c.rlc.RecordFailure(model)
			}
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

	// Record success for actual model used
	if c.rlc != nil && actualModel != "" && actualModel != "unknown" {
		c.rlc.RecordSuccess(actualModel)
	}

	slog.Info("OpenRouter API call successful",
		slog.String("provider", "openrouter"),
		slog.Int("choices_count", len(out.Choices)),
		slog.String("requested_model", model),
		slog.String("actual_model", actualModel))
	return out.Choices[0].Message.Content, nil
}

// ChatJSONWithRetry performs chat with enhanced retry and model switching, preferring Groq
// when configured and falling back to OpenRouter free models when available.
func (c *Client) ChatJSONWithRetry(ctx domain.Context, systemPrompt, userPrompt string, maxTokens int) (string, error) {
	groqKey := strings.TrimSpace(c.cfg.GroqAPIKey)
	hasGroq := groqKey != ""
	orPrimary, orSecondary := c.getOpenRouterKeys()
	hasOR1 := orPrimary != ""
	hasOR2 := orSecondary != ""

	var groqErr error
	var orErr error
	var freeModels []freemodels.Model

	// 1) Primary: Groq, when configured and not blocked
	if hasGroq && !c.isGroqBlocked() {
		res, err := c.callGroqChat(ctx, systemPrompt, userPrompt, maxTokens)
		if err == nil {
			return res, nil
		}
		groqErr = err
		slog.Warn("Groq ChatJSONWithRetry attempt failed",
			slog.String("provider", "groq"),
			slog.Any("error", err))
	} else if hasGroq && c.isGroqBlocked() {
		slog.Info("skipping Groq due to active rate limit block", slog.String("provider", "groq"))
		groqErr = errors.New("groq rate limited and blocked")
	}

	// 2) Secondary: OpenRouter free models via primary account, then secondary account
	if hasOR1 || hasOR2 {
		// Get free models from the service with retry logic (shared across accounts)
		slog.Debug("calling free models service to get available models")
		models, err := c.freeModelsSvc.GetFreeModels(ctx)
		if err != nil {
			slog.Error("failed to get free models from service",
				slog.Any("error", err),
				slog.String("service", "freemodels"))

			// Try to refresh models and retry once
			slog.Info("attempting to refresh free models")
			if refreshErr := c.freeModelsSvc.Refresh(ctx); refreshErr != nil {
				slog.Error("failed to refresh free models", slog.Any("error", refreshErr))
				orErr = fmt.Errorf("openrouter free models unavailable: %w", err)
			} else {
				models, err = c.freeModelsSvc.GetFreeModels(ctx)
				if err != nil {
					slog.Error("failed to get free models after refresh", slog.Any("error", err))
					orErr = fmt.Errorf("openrouter free models unavailable after refresh: %w", err)
				}
			}
		} else {
			freeModels = models
		}

		if len(freeModels) == 0 && orErr == nil {
			slog.Error("no free models available", slog.String("provider", "openrouter"))
			orErr = fmt.Errorf("no free models available from OpenRouter API")
		}

		// Try OpenRouter primary account with enhanced switching
		if hasOR1 && len(freeModels) > 0 && !c.isOpenRouterAccountBlocked(orPrimary) {
			result, err := c.chatJSONWithEnhancedModelSwitchingForKey(ctx, orPrimary, systemPrompt, userPrompt, maxTokens, freeModels)
			if err == nil {
				return result, nil
			}
			if orErr == nil {
				orErr = err
			} else {
				orErr = fmt.Errorf("openrouter primary account failed: %v; %w", err, orErr)
			}
		}

		// Try OpenRouter secondary account with enhanced switching
		if hasOR2 && len(freeModels) > 0 && !c.isOpenRouterAccountBlocked(orSecondary) {
			result, err := c.chatJSONWithEnhancedModelSwitchingForKey(ctx, orSecondary, systemPrompt, userPrompt, maxTokens, freeModels)
			if err == nil {
				return result, nil
			}
			if orErr == nil {
				orErr = err
			} else {
				orErr = fmt.Errorf("openrouter secondary account failed: %v; %w", err, orErr)
			}
		}

		// If neither account produced an error but both are blocked, surface a generic error
		if orErr == nil && (hasOR1 || hasOR2) {
			orErr = fmt.Errorf("openrouter chat failed: all configured accounts are rate limited or blocked")
		}
	}

	// Aggregate final error based on which providers were configured
	if hasGroq && (hasOR1 || hasOR2) {
		return "", fmt.Errorf("groq chat failed: %v; openrouter chat failed: %w", groqErr, orErr)
	}
	if hasGroq {
		// Groq was configured but failed and OpenRouter is not available
		return "", groqErr
	}
	if hasOR1 || hasOR2 {
		return "", orErr
	}

	// Neither provider is configured
	slog.Error("no AI providers configured (Groq/OpenRouter)")
	return "", fmt.Errorf("%w: no AI providers configured", domain.ErrInvalidArgument)
}

// chatJSONWithEnhancedModelSwitching implements intelligent model switching with timeout handling.
func (c *Client) chatJSONWithEnhancedModelSwitching(ctx domain.Context, systemPrompt, userPrompt string, maxTokens int, freeModels []freemodels.Model) (string, error) {
	apiKey := c.getOpenRouterAPIKey()
	return c.chatJSONWithEnhancedModelSwitchingForKey(ctx, apiKey, systemPrompt, userPrompt, maxTokens, freeModels)
}

// chatJSONWithEnhancedModelSwitchingForKey implements intelligent model switching with timeout
// handling for a specific OpenRouter API key (account). This allows per-account rate-limit
// handling and fallback across multiple accounts.
func (c *Client) chatJSONWithEnhancedModelSwitchingForKey(ctx domain.Context, apiKey, systemPrompt, userPrompt string, maxTokens int, freeModels []freemodels.Model) (string, error) {
	// Configuration for enhanced model switching
	maxRetriesPerModel := 2
	modelTimeout := 60 * time.Second // default per-model timeout
	// In dev/E2E environments, allow more time per model attempt to accommodate
	// slower free models and heavier prompts used in comprehensive evaluations.
	if strings.EqualFold(c.cfg.AppEnv, "dev") {
		modelTimeout = 120 * time.Second
	}
	circuitBreakerThreshold := 3 // Switch after 3 consecutive failures

	// Track model performance for intelligent selection
	modelFailures := make(map[string]int)
	modelSuccesses := make(map[string]int)
	modelsTried := 0

	// Build rate-limit-aware order: unblocked first (RR offset), then blocked by shortest wait
	ordered := make([]freemodels.Model, 0, len(freeModels))
	blocked := make([]freemodels.Model, 0)
	if c.rlc != nil {
		for _, m := range freeModels {
			if c.rlc.RemainingBlockDuration(m.ID) <= 0 {
				ordered = append(ordered, m)
			} else {
				blocked = append(blocked, m)
			}
		}
		sort.Slice(blocked, func(i, j int) bool {
			return c.rlc.RemainingBlockDuration(blocked[i].ID) < c.rlc.RemainingBlockDuration(blocked[j].ID)
		})
	} else {
		ordered = append(ordered, freeModels...)
	}

	// Track the boundary between unblocked and blocked models
	unblockedCount := len(ordered)

	// Apply round-robin offset within unblocked bucket
	if len(ordered) > 1 {
		offset := int(atomic.AddInt64(&c.modelCounter, 1) % int64(len(ordered)))
		ordered = append(ordered[offset:], ordered[:offset]...)
	}
	ordered = append(ordered, blocked...)

	// If all models are blocked, we'll still try them - don't skip entirely
	allBlocked := unblockedCount == 0
	if allBlocked {
		slog.Warn("all OpenRouter models are blocked, will try blocked models with shortest wait",
			slog.Int("blocked_count", len(blocked)))
	}

	maxModelsToTry := 8
	if len(ordered) < maxModelsToTry {
		maxModelsToTry = len(ordered)
	}

	// Try each model with enhanced timeout and circuit breaker logic
	for modelIndex, model := range ordered {
		if modelIndex >= maxModelsToTry {
			slog.Warn("max model attempts reached in enhanced switching",
				slog.Int("max_models_to_try", maxModelsToTry),
				slog.Int("models_tried", modelsTried),
				slog.Int("total_models", len(freeModels)))
			break
		}

		if c.isOpenRouterAccountBlocked(apiKey) {
			slog.Warn("OpenRouter account blocked due to rate limiting, aborting remaining model attempts",
				slog.Int("models_tried", modelsTried),
				slog.Int("total_models", len(freeModels)))
			break
		}

		modelID := model.ID
		modelName := model.Name

		// Skip models that are currently blocked by rate-limit cache,
		// UNLESS all models are blocked (then we try anyway)
		if c.rlc != nil && c.rlc.IsModelBlocked(modelID) && !allBlocked {
			slog.Warn("skipping model due to active rate-limit block",
				slog.String("model", modelID),
				slog.String("model_name", modelName))
			continue
		}

		// Skip models that have failed too many times (circuit breaker)
		if modelFailures[modelID] >= circuitBreakerThreshold {
			slog.Warn("model circuit breaker triggered, skipping model",
				slog.String("model", modelID),
				slog.String("model_name", modelName),
				slog.Int("failures", modelFailures[modelID]),
				slog.Int("threshold", circuitBreakerThreshold))
			continue
		}

		modelsTried++

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
				result, err := c.callOpenRouterWithModelForKey(modelCtx, apiKey, modelID, systemPrompt, userPrompt, maxTokens)
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
					if c.rlc != nil {
						c.rlc.RecordSuccess(modelID)
					}
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
					if c.rlc != nil {
						c.rlc.RecordFailure(modelID)
					}
				} else {
					// Other types of errors
					modelFailures[modelID]++
					if c.rlc != nil {
						c.rlc.RecordFailure(modelID)
					}
				}

			case <-modelCtx.Done():
				// Timeout occurred
				cancel()
				modelFailures[modelID]++
				if c.rlc != nil {
					c.rlc.RecordFailure(modelID)
				}
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
		slog.Int("total_models_tried", modelsTried),
		slog.Int("total_models_available", len(freeModels)),
		slog.Any("model_failures", modelFailures),
		slog.Any("model_successes", modelSuccesses))

	return "", fmt.Errorf("all models failed after enhanced switching (tried %d models)", modelsTried)
}

// callOpenRouterWithModel makes a single call to OpenRouter with a specific model.
// callOpenRouterWithModel calls OpenRouter using whichever key is returned by getOpenRouterAPIKey.
// It preserves the legacy behaviour of distributing calls across accounts when both are configured.
func (c *Client) callOpenRouterWithModel(ctx domain.Context, model, systemPrompt, userPrompt string, maxTokens int) (string, error) {
	apiKey := c.getOpenRouterAPIKey()
	return c.callOpenRouterWithModelForKey(ctx, apiKey, model, systemPrompt, userPrompt, maxTokens)
}

// callOpenRouterWithModelForKey makes a single call to OpenRouter with a specific model and API key.
// This is used by enhanced switching to target a specific OpenRouter account.
func (c *Client) callOpenRouterWithModelForKey(ctx domain.Context, apiKey, model, systemPrompt, userPrompt string, maxTokens int) (string, error) {
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

	openRouterKey := strings.TrimSpace(apiKey)
	if openRouterKey == "" {
		slog.Error("OpenRouter API key missing for model switching", slog.String("provider", "openrouter"))
		return "", fmt.Errorf("%w: OPENROUTER_API_KEY missing", domain.ErrInvalidArgument)
	}

	op := func() error {
		start := time.Now()
		connectionStart := time.Now()
		// Client-level minimal spacing between OpenRouter calls to reduce 429s
		c.waitOpenRouterMinInterval()

		r, _ := http.NewRequestWithContext(ctx, http.MethodPost, c.cfg.OpenRouterBaseURL+"/chat/completions", bytes.NewReader(b))
		r.Header.Set("Authorization", "Bearer "+openRouterKey)
		r.Header.Set("Content-Type", "application/json")
		if ref := strings.TrimSpace(c.cfg.OpenRouterReferer); ref != "" {
			r.Header.Set("HTTP-Referer", ref)
		}
		if title := strings.TrimSpace(c.cfg.OpenRouterTitle); title != "" {
			r.Header.Set("X-Title", title)
		}

		// Log connection start for model switching
		slog.Debug("starting OpenRouter API connection (model switching)",
			slog.String("model", model),
			slog.String("endpoint", c.cfg.OpenRouterBaseURL+"/chat/completions"),
			slog.Time("connection_start", connectionStart))

		resp, err := c.chatHC.Do(r)
		c.markOpenRouterCall()
		connectionDuration := time.Since(connectionStart)

		if err != nil {
			// Log without touching resp
			slog.Info("OpenRouter API connection attempt failed (model switching)",
				slog.String("model", model),
				slog.Duration("connection_duration", connectionDuration))
			observability.AIRequestsTotal.WithLabelValues("openrouter", "chat_retry").Inc()
			observability.AIRequestDuration.WithLabelValues("openrouter", "chat_retry").Observe(time.Since(start).Seconds())
			return err
		}

		// Log connection duration for model switching
		slog.Info("OpenRouter API connection completed (model switching)",
			slog.String("model", model),
			slog.Duration("connection_duration", connectionDuration),
			slog.Int("status_code", resp.StatusCode),
			slog.String("x_request_id", resp.Header.Get("X-Request-Id")))

		observability.AIRequestsTotal.WithLabelValues("openrouter", "chat_retry").Inc()
		observability.AIRequestDuration.WithLabelValues("openrouter", "chat_retry").Observe(time.Since(start).Seconds())
		defer func() { _ = resp.Body.Close() }()

		bodyBytes, err := io.ReadAll(resp.Body)
		if err != nil {
			slog.Error("failed to read response body", slog.String("provider", "openrouter"), slog.Any("error", err))
			return err
		}

		if resp.StatusCode == 429 {
			slog.Warn("ai provider rate limited", slog.String("provider", "openrouter"), slog.String("op", "chat_retry"), slog.Int("status", resp.StatusCode))
			retryAfter := parseRetryAfterHeader(resp.Header.Get("Retry-After"))
			if c.rlc != nil {
				c.rlc.RecordRateLimit(model, retryAfter)
			}
			c.blockOpenRouterAccount(openRouterKey, retryAfter)
			return fmt.Errorf("rate limited: 429")
		}
		if resp.StatusCode >= 400 && resp.StatusCode < 500 {
			bodySnippet := string(bodyBytes)
			if len(bodySnippet) > 512 {
				bodySnippet = bodySnippet[:512]
			}
			slog.Warn("ai provider 4xx", slog.String("provider", "openrouter"), slog.String("op", "chat_retry"), slog.Int("status", resp.StatusCode), slog.String("model", model), slog.String("endpoint", c.cfg.OpenRouterBaseURL+"/chat/completions"), slog.String("x_request_id", resp.Header.Get("X-Request-Id")), slog.String("body", bodySnippet))
			if c.rlc != nil {
				c.rlc.RecordFailure(model)
			}
			return backoff.Permanent(fmt.Errorf("chat status %d", resp.StatusCode))
		}
		if resp.StatusCode < 200 || resp.StatusCode >= 300 {
			bodySnippet := string(bodyBytes)
			if len(bodySnippet) > 512 {
				bodySnippet = bodySnippet[:512]
			}
			slog.Error("ai provider non-2xx", slog.String("provider", "openrouter"), slog.String("op", "chat_retry"), slog.Int("status", resp.StatusCode), slog.String("model", model), slog.String("endpoint", c.cfg.OpenRouterBaseURL+"/chat/completions"), slog.String("x_request_id", resp.Header.Get("X-Request-Id")), slog.String("body", bodySnippet))
			if c.rlc != nil {
				c.rlc.RecordFailure(model)
			}
			return fmt.Errorf("chat status %d", resp.StatusCode)
		}
		if err := json.Unmarshal(bodyBytes, &out); err != nil {
			slog.Error("ai provider decode error", slog.String("provider", "openrouter"), slog.String("op", "chat_retry"), slog.String("model", model), slog.String("endpoint", c.cfg.OpenRouterBaseURL+"/chat/completions"), slog.Any("error", err))
			if c.rlc != nil {
				c.rlc.RecordFailure(model)
			}
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

// callGroqChat calls Groq's OpenAI-compatible chat completions API using an internal
// curated model list. It stops on provider-level 429 and falls back to OpenRouter via
// higher-level logic when Groq is blocked.
func (c *Client) callGroqChat(ctx domain.Context, systemPrompt, userPrompt string, maxTokens int) (string, error) {
	if strings.TrimSpace(c.cfg.GroqAPIKey) == "" {
		slog.Error("Groq API key missing", slog.String("provider", "groq"))
		return "", fmt.Errorf("%w: GROQ_API_KEY missing", domain.ErrInvalidArgument)
	}

	// Build ordered list of Groq models using an internal curated set. This keeps
	// Groq model selection automatic and independent of env configuration.
	models := []string{
		"llama-3.1-8b-instant",
		"llama-3.3-70b-versatile",
	}

	var lastErr error
	for _, model := range models {
		if c.isGroqBlocked() {
			slog.Info("skipping Groq models due to active rate limit block",
				slog.String("provider", "groq"))
			break
		}

		res, err := c.callGroqChatWithModel(ctx, model, systemPrompt, userPrompt, maxTokens)
		if err == nil {
			return res, nil
		}
		lastErr = err

		// If we hit a provider-level rate limit, stop trying further models and
		// let higher-level logic fall back to OpenRouter.
		if strings.Contains(err.Error(), "rate limited:") {
			slog.Warn("Groq provider rate limited; not attempting further Groq models",
				slog.String("provider", "groq"),
				slog.String("model", model),
				slog.Any("error", err))
			break
		}

		slog.Warn("Groq model attempt failed, trying next model if available",
			slog.String("provider", "groq"),
			slog.String("model", model),
			slog.Any("error", err))
	}

	if lastErr == nil {
		return "", fmt.Errorf("groq chat failed: no models attempted")
	}
	return "", lastErr
}

// callGroqChatWithModel performs the actual Groq API call for a specific model with
// existing backoff and rate-limit handling.
func (c *Client) callGroqChatWithModel(ctx domain.Context, model, systemPrompt, userPrompt string, maxTokens int) (string, error) {
	// Enforce minimum interval between Groq calls to avoid rate limiting
	c.waitGroqMinInterval()

	baseURL := strings.TrimSpace(c.cfg.GroqBaseURL)
	if baseURL == "" {
		baseURL = "https://api.groq.com/openai/v1"
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

	b, _ := json.Marshal(body)
	slog.Debug("Groq API request body", slog.String("body", string(b)))

	var out struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
	}

	op := func() error {
		start := time.Now()
		endpoint := strings.TrimRight(baseURL, "/") + "/chat/completions"
		r, _ := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(b))
		r.Header.Set("Authorization", "Bearer "+c.cfg.GroqAPIKey)
		r.Header.Set("Content-Type", "application/json")

		connectionStart := time.Now()
		resp, err := c.chatHC.Do(r)
		c.markGroqCall() // Mark the call timestamp for rate limiting
		connectionDuration := time.Since(connectionStart)
		if err != nil {
			slog.Info("Groq API connection attempt failed",
				slog.String("provider", "groq"),
				slog.Duration("connection_duration", connectionDuration))
			observability.AIRequestsTotal.WithLabelValues("groq", "chat").Inc()
			observability.AIRequestDuration.WithLabelValues("groq", "chat").Observe(time.Since(start).Seconds())
			return err
		}

		slog.Info("Groq API connection completed",
			slog.String("provider", "groq"),
			slog.Duration("connection_duration", connectionDuration),
			slog.Int("status_code", resp.StatusCode))

		observability.AIRequestsTotal.WithLabelValues("groq", "chat").Inc()
		observability.AIRequestDuration.WithLabelValues("groq", "chat").Observe(time.Since(start).Seconds())
		defer func() { _ = resp.Body.Close() }()

		bodyBytes, err := io.ReadAll(resp.Body)
		if err != nil {
			slog.Error("failed to read response body", slog.String("provider", "groq"), slog.Any("error", err))
			return err
		}

		if resp.StatusCode == http.StatusTooManyRequests {
			slog.Warn("ai provider rate limited", slog.String("provider", "groq"), slog.String("op", "chat"), slog.Int("status", resp.StatusCode))
			// Block Groq for 60 seconds (or Retry-After if provided) and fail fast
			retryAfter := parseRetryAfterHeader(resp.Header.Get("Retry-After"))
			c.blockGroq(retryAfter)
			// Return permanent error to stop retrying - let the caller fall back to OpenRouter
			return backoff.Permanent(fmt.Errorf("rate limited: %d", resp.StatusCode))
		}
		if resp.StatusCode >= 400 && resp.StatusCode < 500 {
			bodySnippet := string(bodyBytes)
			if len(bodySnippet) > 512 {
				bodySnippet = bodySnippet[:512]
			}
			slog.Warn("ai provider 4xx", slog.String("provider", "groq"), slog.String("op", "chat"), slog.Int("status", resp.StatusCode), slog.String("model", model), slog.String("endpoint", endpoint), slog.String("body", bodySnippet))
			return backoff.Permanent(fmt.Errorf("chat status %d", resp.StatusCode))
		}
		if resp.StatusCode < 200 || resp.StatusCode >= 300 {
			bodySnippet := string(bodyBytes)
			if len(bodySnippet) > 512 {
				bodySnippet = bodySnippet[:512]
			}
			slog.Error("ai provider non-2xx", slog.String("provider", "groq"), slog.String("op", "chat"), slog.Int("status", resp.StatusCode), slog.String("model", model), slog.String("endpoint", endpoint), slog.String("body", bodySnippet))
			return fmt.Errorf("chat status %d", resp.StatusCode)
		}
		if err := json.Unmarshal(bodyBytes, &out); err != nil {
			slog.Error("ai provider decode error", slog.String("provider", "groq"), slog.String("op", "chat"), slog.String("model", model), slog.String("endpoint", endpoint), slog.Any("error", err))
			return err
		}
		return nil
	}

	expo := c.getBackoffConfig()
	bo := backoff.WithContext(expo, ctx)

	slog.Info("starting Groq API retry logic", slog.String("provider", "groq"), slog.Duration("max_elapsed", expo.MaxElapsedTime))
	if err := backoff.Retry(op, bo); err != nil {
		slog.Error("Groq API failed after retries", slog.String("provider", "groq"), slog.Any("error", err))
		return "", fmt.Errorf("groq api failed: %w", err)
	}

	if len(out.Choices) == 0 {
		slog.Error("Groq API returned empty choices", slog.String("provider", "groq"))
		return "", errors.New("empty choices from Groq API")
	}

	return out.Choices[0].Message.Content, nil
}

// waitOpenRouterMinInterval enforces a minimal spacing between OpenRouter calls across this client instance.
func (c *Client) waitOpenRouterMinInterval() {
	minGap := c.cfg.OpenRouterMinInterval
	// If multiple worker processes are issuing OpenRouter requests, scale the
	// per-process gap so that aggregate QPS across all workers still respects
	// free-tier limits (e.g. 20 RPM for :free variants per OpenRouter docs).
	workerFactor := c.cfg.AIWorkerReplicas
	if workerFactor < 1 {
		workerFactor = 1
	}
	minGap = time.Duration(int64(minGap) * int64(workerFactor))
	if minGap <= 0 {
		return
	}
	for {
		prev := c.lastORCall.Load()
		now := time.Now()
		if prev == 0 {
			// First call wins the slot
			if c.lastORCall.CompareAndSwap(0, now.UnixNano()) {
				return
			}
			continue
		}
		last := time.Unix(0, prev)
		delta := now.Sub(last)
		if delta >= minGap {
			// Try to claim the next slot; if we lose CAS, retry
			if c.lastORCall.CompareAndSwap(prev, now.UnixNano()) {
				return
			}
			continue
		}
		time.Sleep(minGap - delta)
	}
}

// markOpenRouterCall updates the last OpenRouter call timestamp.
func (c *Client) markOpenRouterCall() {
	c.lastORCall.Store(time.Now().UnixNano())
}

func (c *Client) isOpenRouterBlocked() bool {
	blockedUntil := c.openRouterBlocked.Load()
	if blockedUntil == 0 {
		return false
	}
	return time.Now().UnixNano() < blockedUntil
}

func (c *Client) blockOpenRouter(d time.Duration) {
	if d <= 0 {
		d = 60 * time.Second
	}
	c.openRouterBlocked.Store(time.Now().Add(d).UnixNano())
	slog.Warn("blocking OpenRouter due to rate limit",
		slog.Duration("block_duration", d))
}

// isOpenRouterAccountBlocked returns true if the given OpenRouter API key is currently
// blocked due to a recent 429 response. This enables per-account fallback between
// primary and secondary OpenRouter keys.
func (c *Client) isOpenRouterAccountBlocked(apiKey string) bool {
	key := strings.TrimSpace(apiKey)
	if key == "" {
		return false
	}
	if key == strings.TrimSpace(c.cfg.OpenRouterAPIKey) {
		blockedUntil := c.openRouter1Blocked.Load()
		return blockedUntil != 0 && time.Now().UnixNano() < blockedUntil
	}
	if key == strings.TrimSpace(c.cfg.OpenRouterAPIKey2) {
		blockedUntil := c.openRouter2Blocked.Load()
		return blockedUntil != 0 && time.Now().UnixNano() < blockedUntil
	}
	return false
}

// blockOpenRouterAccount blocks a specific OpenRouter API key for the given duration
// after a 429 response, without affecting the other account. This allows sequential
// fallback from primary to secondary account when rate limits are hit.
func (c *Client) blockOpenRouterAccount(apiKey string, d time.Duration) {
	key := strings.TrimSpace(apiKey)
	if key == "" {
		return
	}
	if d <= 0 {
		d = 60 * time.Second
	}
	blockedUntil := time.Now().Add(d).UnixNano()
	if key == strings.TrimSpace(c.cfg.OpenRouterAPIKey) {
		c.openRouter1Blocked.Store(blockedUntil)
		slog.Warn("blocking OpenRouter primary account due to rate limit",
			slog.Duration("block_duration", d))
		return
	}
	if key == strings.TrimSpace(c.cfg.OpenRouterAPIKey2) {
		c.openRouter2Blocked.Store(blockedUntil)
		slog.Warn("blocking OpenRouter secondary account due to rate limit",
			slog.Duration("block_duration", d))
		return
	}
}

// waitGroqMinInterval enforces a minimal spacing between Groq calls to avoid rate limiting.
func (c *Client) waitGroqMinInterval() {
	// Groq has aggressive rate limits (e.g., 30 requests per minute on free tier).
	// Enforce a conservative minimal gap between calls per process and scale by
	// the approximate number of worker processes so aggregate QPS stays within
	// organizational limits.
	baseGap := 4 * time.Second
	workerFactor := c.cfg.AIWorkerReplicas
	if workerFactor < 1 {
		workerFactor = 1
	}
	minGap := time.Duration(int64(baseGap) * int64(workerFactor))
	for {
		prev := c.lastGroqCall.Load()
		now := time.Now()
		if prev == 0 {
			// First call wins the slot
			if c.lastGroqCall.CompareAndSwap(0, now.UnixNano()) {
				return
			}
			continue
		}
		last := time.Unix(0, prev)
		delta := now.Sub(last)
		if delta >= minGap {
			// Try to claim the next slot; if we lose CAS, retry
			if c.lastGroqCall.CompareAndSwap(prev, now.UnixNano()) {
				return
			}
			continue
		}
		time.Sleep(minGap - delta)
	}
}

// markGroqCall updates the last Groq call timestamp.
func (c *Client) markGroqCall() {
	c.lastGroqCall.Store(time.Now().UnixNano())
}

// isGroqBlocked checks if Groq is currently blocked due to a recent 429 response.
func (c *Client) isGroqBlocked() bool {
	blockedUntil := c.groqBlocked.Load()
	if blockedUntil == 0 {
		return false
	}
	return time.Now().UnixNano() < blockedUntil
}

// blockGroq blocks Groq for the specified duration after a 429 response.
func (c *Client) blockGroq(d time.Duration) {
	if d <= 0 {
		d = 60 * time.Second // Default 60 second cooldown
	}
	c.groqBlocked.Store(time.Now().Add(d).UnixNano())
	slog.Warn("blocking Groq due to rate limit",
		slog.Duration("block_duration", d))
}

// parseRetryAfterHeader parses Retry-After header into duration (delta-seconds or HTTP-date).
func parseRetryAfterHeader(v string) time.Duration {
	if v == "" {
		return 0
	}
	if secs, err := strconv.Atoi(strings.TrimSpace(v)); err == nil && secs >= 0 {
		return time.Duration(secs) * time.Second
	}
	if t, err := http.ParseTime(v); err == nil {
		d := time.Until(t)
		if d < 0 {
			return 0
		}
		return d
	}
	return 0
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

		// If the connection failed, do not access resp
		if err != nil {
			slog.Info("OpenAI API connection attempt failed (embeddings)",
				slog.String("model", c.cfg.EmbeddingsModel),
				slog.Duration("connection_duration", connectionDuration))
			observability.AIRequestsTotal.WithLabelValues("openai", "embed").Inc()
			observability.AIRequestDuration.WithLabelValues("openai", "embed").Observe(time.Since(start).Seconds())
			return err
		}

		// Log connection duration for embeddings with response details
		slog.Info("OpenAI API connection completed (embeddings)",
			slog.String("model", c.cfg.EmbeddingsModel),
			slog.Duration("connection_duration", connectionDuration),
			slog.Int("status_code", resp.StatusCode),
			slog.String("x_request_id", resp.Header.Get("X-Request-Id")))

		observability.AIRequestsTotal.WithLabelValues("openai", "embed").Inc()
		observability.AIRequestDuration.WithLabelValues("openai", "embed").Observe(time.Since(start).Seconds())
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
	openRouterKey := c.getOpenRouterAPIKey()

	op := func() error {
		start := time.Now()
		// Respect global OpenRouter client-level throttling to avoid 429s during cleaning
		c.waitOpenRouterMinInterval()
		r, _ := http.NewRequestWithContext(ctx, http.MethodPost, c.cfg.OpenRouterBaseURL+"/chat/completions", bytes.NewReader(b))
		r.Header.Set("Authorization", "Bearer "+openRouterKey)
		r.Header.Set("Content-Type", "application/json")
		resp, err := c.chatHC.Do(r)
		c.markOpenRouterCall()
		observability.AIRequestsTotal.WithLabelValues("openrouter", "cot_cleaning").Inc()
		observability.AIRequestDuration.WithLabelValues("openrouter", "cot_cleaning").Observe(time.Since(start).Seconds())
		if err != nil {
			return err
		}
		defer func() { _ = resp.Body.Close() }()

		if resp.StatusCode == 429 {
			slog.Warn("ai provider rate limited during CoT cleaning", slog.String("provider", "openrouter"), slog.String("op", "cot_cleaning"), slog.Int("status", resp.StatusCode))
			// Block the cleaning model briefly as well
			retryAfter := parseRetryAfterHeader(resp.Header.Get("Retry-After"))
			if c.rlc != nil {
				c.rlc.RecordRateLimit(cleaningModel.ID, retryAfter)
			}
			c.blockOpenRouter(retryAfter)
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
