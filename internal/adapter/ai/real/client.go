// Package real implements a real AI client backed by OpenRouter and OpenAI APIs.
package real

import (
	"bufio"
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	backoff "github.com/cenkalti/backoff/v4"
	tiktoken "github.com/pkoukk/tiktoken-go"
	tiktoken_loader "github.com/pkoukk/tiktoken-go-loader"
	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"

	aiadapter "github.com/fairyhunter13/ai-cv-evaluator/internal/adapter/ai"
	"github.com/fairyhunter13/ai-cv-evaluator/internal/adapter/observability"
	"github.com/fairyhunter13/ai-cv-evaluator/internal/config"
	"github.com/fairyhunter13/ai-cv-evaluator/internal/domain"
	intobs "github.com/fairyhunter13/ai-cv-evaluator/internal/observability"
	"github.com/fairyhunter13/ai-cv-evaluator/internal/service/freemodels"
	"github.com/fairyhunter13/ai-cv-evaluator/internal/service/ratelimiter"
)

func init() {
	// Use offline BPE loader to avoid downloading encoding files at runtime.
	// This is required for Docker containers that may not have internet access.
	tiktoken.SetBpeLoader(tiktoken_loader.NewOfflineLoader())
}

// recordTokenUsage records token usage metrics for AI provider calls.
// It records both prompt and completion tokens separately for detailed tracking.
func recordTokenUsage(provider, model string, promptTokens, completionTokens int) {
	slog.Debug("recordTokenUsage called",
		slog.String("provider", provider),
		slog.String("model", model),
		slog.Int("prompt_tokens", promptTokens),
		slog.Int("completion_tokens", completionTokens))
	if promptTokens > 0 {
		observability.RecordAITokenUsage(provider, "prompt", model, promptTokens)
	}
	if completionTokens > 0 {
		observability.RecordAITokenUsage(provider, "completion", model, completionTokens)
	}
}

// estimateTokenCount estimates the number of tokens in a text using tiktoken.
// It uses cl100k_base encoding which is compatible with most modern LLMs.
// Returns 0 if encoding fails.
func estimateTokenCount(text string) int {
	if text == "" {
		return 0
	}
	enc, err := tiktoken.GetEncoding("cl100k_base")
	if err != nil {
		slog.Error("failed to get tiktoken encoding", slog.Any("error", err))
		return 0
	}
	tokens := enc.Encode(text, nil, nil)
	return len(tokens)
}

// estimateChatTokens estimates the total tokens for a chat completion request.
// It accounts for message formatting overhead used by chat models.
func estimateChatTokens(systemPrompt, userPrompt, response string) (promptTokens, completionTokens int) {
	// Estimate prompt tokens (system + user messages with formatting overhead)
	// Chat models add ~4 tokens per message for formatting
	promptTokens = estimateTokenCount(systemPrompt) + estimateTokenCount(userPrompt) + 8
	completionTokens = estimateTokenCount(response)
	return promptTokens, completionTokens
}

// Client implements domain.AIClient using OpenRouter (chat) and OpenAI (embeddings).
type Client struct {
	cfg                  config.Config
	chatHC               *http.Client
	embedHC              *http.Client
	freeModelsSvc        *freemodels.Service
	modelCounter         int64                     // Counter for round-robin model selection
	providerCounter      int64                     // Counter to balance load between Groq and OpenRouter when both are available
	rlc                  *aiadapter.RateLimitCache // Client-side rate-limit model cache
	limiter              ratelimiter.Limiter
	lastORCall           atomic.Int64 // unix nano timestamp of last OpenRouter call (client-level throttle)
	lastGroqCall         atomic.Int64 // unix nano timestamp of last Groq call (client-level throttle)
	groq1Blocked         atomic.Int64 // unix nano timestamp until which Groq primary key is blocked due to 429
	groq2Blocked         atomic.Int64 // unix nano timestamp until which Groq secondary key is blocked due to 429
	openRouterBlocked    atomic.Int64 // unix nano timestamp until which OpenRouter is blocked (legacy provider-level block)
	openRouterKeyCounter int64
	openRouter1Blocked   atomic.Int64 // unix nano timestamp until which OpenRouter primary key is blocked due to 429
	openRouter2Blocked   atomic.Int64 // unix nano timestamp until which OpenRouter secondary key is blocked due to 429
	groqModels           []string     // Cached Groq chat-capable models ordered by capacity
	groqModelsLastFetch  time.Time    // Last time the Groq models cache was refreshed
	groqModelsMu         sync.RWMutex // Protects access to groqModels and groqModelsLastFetch

	// Integrated observability for external AI calls
	obsOpenRouterChat *intobs.IntegratedObservableClient
	obsGroqChat       *intobs.IntegratedObservableClient
	obsOpenAIEmbed    *intobs.IntegratedObservableClient
	obsCotClean       *intobs.IntegratedObservableClient
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

// readSSEChatStream parses a text/event-stream response from OpenAI-compatible
// chat completions and accumulates the content from each chunk. It supports
// both OpenAI-style {"choices":[{"delta":{"content":"..."}}]} and
// fallback to {"choices":[{"message":{"content":"..."}}]} payloads.
//
// It also enforces a sliding idle timeout: if no new SSE line is received
// within idleTimeout, the stream is considered idle and an error is returned.
func readSSEChatStream(r io.Reader, provider, model string, idleTimeout time.Duration) (string, error) {
	if idleTimeout <= 0 {
		idleTimeout = 20 * time.Second
	}

	scanner := bufio.NewScanner(r)
	// Allow reasonably large SSE lines (up to 1MB) to avoid truncation for
	// long prompts or safety messages.
	buf := make([]byte, 0, 64*1024)
	scanner.Buffer(buf, 1024*1024)

	type lineMsg struct {
		line string
		err  error
	}

	lines := make(chan lineMsg)

	// Goroutine to read lines from the SSE stream and forward them over a
	// channel so the main goroutine can apply a sliding idle timeout.
	go func() {
		defer close(lines)
		for scanner.Scan() {
			lines <- lineMsg{line: scanner.Text()}
		}
		if err := scanner.Err(); err != nil && !errors.Is(err, io.EOF) {
			lines <- lineMsg{err: err}
		}
	}()

	var sb strings.Builder
	timer := time.NewTimer(idleTimeout)
	defer timer.Stop()

	for {
		select {
		case msg, ok := <-lines:
			if !ok {
				// Stream ended normally
				return sb.String(), nil
			}
			if msg.err != nil {
				return "", msg.err
			}

			// Got activity: reset idle timer and parse the line
			if !timer.Stop() {
				select {
				case <-timer.C:
				default:
				}
			}
			_ = timer.Reset(idleTimeout)

			line := strings.TrimSpace(msg.line)
			if line == "" {
				continue
			}
			// Skip comment/heartbeat lines (e.g. ":keep-alive")
			if strings.HasPrefix(line, ":") {
				continue
			}
			if !strings.HasPrefix(line, "data:") {
				continue
			}
			data := strings.TrimSpace(strings.TrimPrefix(line, "data:"))
			if data == "" {
				continue
			}
			if data == "[DONE]" {
				return sb.String(), nil
			}

			var chunk struct {
				Choices []struct {
					Delta struct {
						Content string `json:"content"`
					} `json:"delta"`
					Message struct {
						Content string `json:"content"`
					} `json:"message"`
				} `json:"choices"`
			}

			if err := json.Unmarshal([]byte(data), &chunk); err != nil {
				slog.Error("failed to decode streaming chat chunk",
					slog.String("provider", provider),
					slog.String("model", model),
					slog.String("data", data),
					slog.Any("error", err))
				continue
			}
			if len(chunk.Choices) == 0 {
				continue
			}
			c := chunk.Choices[0]
			piece := c.Delta.Content
			if piece == "" {
				piece = c.Message.Content
			}
			if piece != "" {
				_, _ = sb.WriteString(piece)
			}

		case <-timer.C:
			// No activity within idleTimeout: treat as idle and abort the stream.
			if closer, ok := r.(io.Closer); ok {
				_ = closer.Close()
			}
			return "", fmt.Errorf("stream idle for %s", idleTimeout)
		}
	}
}

// New constructs a real AI client with sensible timeouts.
func New(cfg config.Config) *Client {
	return NewWithLimiter(cfg, nil)
}

func NewWithLimiter(cfg config.Config, lim ratelimiter.Limiter) *Client {

	// Use more aggressive timeouts by default.
	chatTimeout := 60 * time.Second  // Base timeout for chat completions
	embedTimeout := 30 * time.Second // Base timeout for embeddings

	// In dev (including E2E), allow a slightly higher chat timeout but keep it
	// well below the worker's 5-minute job-level SLA so stuck provider calls
	// fail fast and flow through our retry/timeout handling.
	if cfg.AppEnv == "dev" {
		chatTimeout = 90 * time.Second
		embedTimeout = 60 * time.Second
	}

	// Initialize free models service. Prefer primary OpenRouter key but
	// transparently fall back to OPENROUTER_API_KEY_2 when only the
	// secondary key is configured.
	openRouterKey := cfg.OpenRouterAPIKey
	if openRouterKey == "" {
		openRouterKey = cfg.OpenRouterAPIKey2
	}
	freeModelsSvc := freemodels.NewService(openRouterKey, cfg.OpenRouterBaseURL, cfg.FreeModelsRefresh)

	// Build integrated observable clients for AI providers
	openRouterObs := intobs.NewIntegratedObservableClient(
		intobs.ConnectionTypeAI,
		intobs.OperationTypeChat,
		"openrouter",
		"ai-client",
		chatTimeout,
		5*time.Second,
		2*chatTimeout,
	)
	groqObs := intobs.NewIntegratedObservableClient(
		intobs.ConnectionTypeAI,
		intobs.OperationTypeChat,
		"groq",
		"ai-client",
		chatTimeout,
		5*time.Second,
		2*chatTimeout,
	)
	embedObs := intobs.NewIntegratedObservableClient(
		intobs.ConnectionTypeAI,
		intobs.OperationTypeEmbed,
		"openai",
		"ai-client",
		embedTimeout,
		5*time.Second,
		2*embedTimeout,
	)
	cotCleanObs := intobs.NewIntegratedObservableClient(
		intobs.ConnectionTypeAI,
		intobs.OperationTypeChat,
		"openrouter",
		"ai-client",
		chatTimeout,
		5*time.Second,
		2*chatTimeout,
	)

	// Create HTTP clients with OpenTelemetry tracing for external AI calls
	chatTransport := otelhttp.NewTransport(http.DefaultTransport,
		otelhttp.WithSpanNameFormatter(func(_ string, r *http.Request) string {
			return fmt.Sprintf("AI %s %s", r.Method, r.URL.Host)
		}),
	)
	embedTransport := otelhttp.NewTransport(http.DefaultTransport,
		otelhttp.WithSpanNameFormatter(func(_ string, r *http.Request) string {
			return fmt.Sprintf("AI Embed %s %s", r.Method, r.URL.Host)
		}),
	)

	return &Client{
		cfg:               cfg,
		chatHC:            &http.Client{Timeout: chatTimeout, Transport: chatTransport},
		embedHC:           &http.Client{Timeout: embedTimeout, Transport: embedTransport},
		freeModelsSvc:     freeModelsSvc,
		rlc:               aiadapter.NewRateLimitCache(),
		limiter:           lim,
		obsOpenRouterChat: openRouterObs,
		obsGroqChat:       groqObs,
		obsOpenAIEmbed:    embedObs,
		obsCotClean:       cotCleanObs,
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

	lg := intobs.LoggerFromContext(ctx)

	// 1) Primary: Groq when configured and not blocked
	var groqErr error
	if hasGroq && !c.isGroqAccountBlocked(groqKey) {
		res, err := c.callGroqChat(ctx, groqKey, systemPrompt, userPrompt, maxTokens)
		if err == nil {
			return res, nil
		}
		groqErr = err
		lg.Warn("Groq ChatJSON primary attempt failed",
			slog.String("provider", "groq"),
			slog.Any("error", err))

		// If OpenRouter is not configured, surface Groq error directly.
		if !hasOpenRouter {
			return "", groqErr
		}
	} else if hasGroq && c.isGroqAccountBlocked(groqKey) {
		lg.Info("skipping Groq due to active rate limit block", slog.String("provider", "groq"))
		groqErr = errors.New("groq rate limited and blocked")
	}

	// 2) Secondary: OpenRouter free models
	if !hasOpenRouter {
		lg.Error("OpenRouter API key missing", slog.String("provider", "openrouter"))
		if hasGroq {
			// Normally unreachable because !hasOpenRouter with hasGroq returns above, but keep for safety.
			return "", groqErr
		}
		return "", fmt.Errorf("%w: OPENROUTER_API_KEY missing", domain.ErrInvalidArgument)
	}

	// Get free models from the service with retry logic
	lg.Debug("calling free models service to get available models")
	freeModels, err := c.freeModelsSvc.GetFreeModels(ctx)
	if err != nil {
		lg.Error("failed to get free models from service",
			slog.Any("error", err),
			slog.String("service", "freemodels"))

		// Try to refresh models and retry once
		lg.Info("attempting to refresh free models and retry")
		if refreshErr := c.freeModelsSvc.Refresh(ctx); refreshErr != nil {
			lg.Error("failed to refresh free models", slog.Any("error", refreshErr))
			return "", fmt.Errorf("failed to get free models: %w", err)
		}

		// Retry after refresh
		freeModels, err = c.freeModelsSvc.GetFreeModels(ctx)
		if err != nil {
			lg.Error("failed to get free models after refresh",
				slog.Any("error", err),
				slog.String("service", "freemodels"))
			return "", fmt.Errorf("failed to get free models after refresh: %w", err)
		}
	}

	lg.Debug("free models service returned models",
		slog.Int("count", len(freeModels)))
	if len(freeModels) == 0 {
		lg.Error("no free models available", slog.String("provider", "openrouter"))
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
			lg.Info("using paid model as fallback (all free models blocked)",
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
			lg.Warn("paid model fallback unavailable, using shortest-wait free models",
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
	lg.Debug("available free models", slog.Any("models", modelIDs))

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
	lg.Info("using free model (rate-limit-aware round-robin)", selectionLog...)

	lg.Info("calling OpenRouter API", slog.String("provider", "openrouter"), slog.String("model", model), slog.Int("max_tokens", maxTokens))
	body := map[string]any{
		"model":       model,
		"temperature": 0.2,
		"max_tokens":  maxTokens,
		"messages": []map[string]string{
			{"role": "system", "content": systemPrompt},
			{"role": "user", "content": userPrompt},
		},
	}
	// For non-test environments, request streaming responses so we can detect
	// inactivity and fail fast if the provider stops sending chunks.
	if !c.cfg.IsTest() {
		body["stream"] = true
	}

	// Add fallback models if available
	if len(fallbackModels) > 0 {
		body["models"] = fallbackModels
		lg.Debug("added fallback models", slog.String("fallback_models", fmt.Sprintf("%v", fallbackModels)))
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
		Usage struct {
			PromptTokens     int `json:"prompt_tokens"`
			CompletionTokens int `json:"completion_tokens"`
			TotalTokens      int `json:"total_tokens"`
		} `json:"usage"`
	}
	openRouterKey = c.getOpenRouterAPIKey()

	var result string
	err = c.obsOpenRouterChat.ExecuteWithMetrics(ctx, "chat", func(callCtx context.Context) error {
		expo := c.getBackoffConfig()
		// For non-test environments, enforce a stricter per-call timeout so that
		// if the provider does not send any response within ~20s, the call fails
		// and is retried via the existing backoff logic. Tests keep the original
		// behaviour to avoid flakiness.
		if !c.cfg.IsTest() {
			var cancel context.CancelFunc
			callCtx, cancel = context.WithTimeout(callCtx, 20*time.Second)
			defer cancel()
		}
		bo := backoff.WithContext(expo, callCtx)

		slog.Info("starting OpenRouter API retry logic", slog.String("provider", "openrouter"), slog.Duration("max_elapsed", expo.MaxElapsedTime))

		op := func() error {
			// Global limiter gate for OpenRouter account across workers
			if c.limiter != nil {
				allowed, retryAfter, err := c.limiter.Allow(callCtx, openRouterBucketKey(openRouterKey), 1)
				if err != nil {
					slog.Error("global rate limiter error for OpenRouter", slog.Any("error", err))
				} else if !allowed {
					slog.Warn("global rate limiter denied OpenRouter call",
						slog.String("provider", "openrouter"),
						slog.Duration("retry_after", retryAfter))
					c.blockOpenRouterAccount(openRouterKey, retryAfter)
					return backoff.Permanent(fmt.Errorf("rate limited: global limiter"))
				}
			}
			connectionStart := time.Now()
			// Recreate request each attempt to avoid reusing consumed bodies
			// Client-level minimal spacing between OpenRouter calls to reduce 429s
			c.waitOpenRouterMinInterval()

			r, _ := http.NewRequestWithContext(callCtx, http.MethodPost, c.cfg.OpenRouterBaseURL+"/chat/completions", bytes.NewReader(b))
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
				return err
			}

			// Log connection duration
			slog.Info("OpenRouter API connection completed",
				slog.String("model", model),
				slog.Duration("connection_duration", connectionDuration),
				slog.Int("status_code", resp.StatusCode),
				slog.String("x_request_id", resp.Header.Get("X-Request-Id")))

			defer func() { _ = resp.Body.Close() }()

			if resp.StatusCode == 429 {
				// Retryable: let backoff handle retries. We don't need the body content
				// here, but we keep the branch structure consistent with other status
				// handlers for logging and rate-limit bookkeeping.
				slog.Warn("ai provider rate limited", slog.String("provider", "openrouter"), slog.String("op", "chat"), slog.Int("status", resp.StatusCode), slog.String("x_request_id", resp.Header.Get("X-Request-Id")))
				retryAfter := parseRetryAfterHeader(resp.Header.Get("Retry-After"))
				if c.rlc != nil {
					c.rlc.RecordRateLimit(model, retryAfter)
				}
				c.blockOpenRouter(retryAfter)
				c.updateOpenRouterLimiterFromRetryAfter(openRouterKey, retryAfter)
				return fmt.Errorf("rate limited: 429")
			}
			if resp.StatusCode >= 400 && resp.StatusCode < 500 {
				// Client error: non-retryable
				bodyBytes, _ := io.ReadAll(resp.Body)
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
				bodyBytes, _ := io.ReadAll(resp.Body)
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
			// At this point we have a successful 2xx status code. Prefer SSE
			// parsing when the provider returns a streaming content type.
			contentType := strings.ToLower(resp.Header.Get("Content-Type"))
			isStream := strings.Contains(contentType, "text/event-stream") && !c.cfg.IsTest()
			if isStream {
				content, err := readSSEChatStream(resp.Body, "openrouter", model, 20*time.Second)
				if err != nil {
					slog.Error("failed to read OpenRouter streaming response", slog.String("provider", "openrouter"), slog.String("model", model), slog.Any("error", err))
					if c.rlc != nil {
						c.rlc.RecordFailure(model)
					}
					return err
				}
				if content == "" {
					slog.Error("OpenRouter streaming response produced empty content", slog.String("provider", "openrouter"), slog.String("model", model))
					if c.rlc != nil {
						c.rlc.RecordFailure(model)
					}
					return errors.New("empty content from OpenRouter streaming response")
				}
				// Populate out so downstream logic (model substitution, success
				// recording) can remain unchanged.
				out.Model = model
				out.Choices = []struct {
					Message struct {
						Content string `json:"content"`
					} `json:"message"`
				}{
					{Message: struct {
						Content string `json:"content"`
					}{Content: content}},
				}
				// For streaming responses, estimate tokens using tiktoken since API doesn't return usage
				promptTokens, completionTokens := estimateChatTokens(systemPrompt, userPrompt, content)
				recordTokenUsage("openrouter", model, promptTokens, completionTokens)
				slog.Info("OpenRouter streaming token usage estimated",
					slog.String("provider", "openrouter"),
					slog.String("model", model),
					slog.Int("prompt_tokens", promptTokens),
					slog.Int("completion_tokens", completionTokens),
					slog.Int("total_tokens", promptTokens+completionTokens))
				return nil
			}

			// Non-streaming JSON response
			bodyBytes, err := io.ReadAll(resp.Body)
			if err != nil {
				slog.Error("failed to read response body", slog.String("provider", "openrouter"), slog.Any("error", err))
				return err
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

		if err := backoff.Retry(op, bo); err != nil {
			slog.Error("OpenRouter API failed after retries", slog.String("provider", "openrouter"), slog.Any("error", err))
			return fmt.Errorf("openrouter api failed: %w", err)
		}

		if len(out.Choices) == 0 {
			slog.Error("OpenRouter API returned empty choices", slog.String("provider", "openrouter"))
			return errors.New("empty choices from OpenRouter API")
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

		// Record token usage from API response
		slog.Info("OpenRouter API response usage info",
			slog.String("provider", "openrouter"),
			slog.String("model", actualModel),
			slog.Int("prompt_tokens", out.Usage.PromptTokens),
			slog.Int("completion_tokens", out.Usage.CompletionTokens),
			slog.Int("total_tokens", out.Usage.TotalTokens))
		if out.Usage.TotalTokens > 0 {
			recordTokenUsage("openrouter", actualModel, out.Usage.PromptTokens, out.Usage.CompletionTokens)
			slog.Info("OpenRouter token usage recorded",
				slog.String("provider", "openrouter"),
				slog.String("model", actualModel),
				slog.Int("prompt_tokens", out.Usage.PromptTokens),
				slog.Int("completion_tokens", out.Usage.CompletionTokens),
				slog.Int("total_tokens", out.Usage.TotalTokens))
		} else {
			slog.Warn("OpenRouter API response did not include usage info, estimating tokens",
				slog.String("provider", "openrouter"),
				slog.String("model", actualModel))
			// Fallback to estimation if API doesn't return usage
			responseContent := out.Choices[0].Message.Content
			promptTokens, completionTokens := estimateChatTokens(systemPrompt, userPrompt, responseContent)
			recordTokenUsage("openrouter", actualModel, promptTokens, completionTokens)
			slog.Info("OpenRouter token usage estimated",
				slog.String("provider", "openrouter"),
				slog.String("model", actualModel),
				slog.Int("prompt_tokens", promptTokens),
				slog.Int("completion_tokens", completionTokens))
		}

		slog.Info("OpenRouter API call successful",
			slog.String("provider", "openrouter"),
			slog.Int("choices_count", len(out.Choices)),
			slog.String("requested_model", model),
			slog.String("actual_model", actualModel))

		result = out.Choices[0].Message.Content
		return nil
	})
	if err != nil {
		return "", err
	}

	return result, nil
}

// ChatJSONWithRetry performs chat with enhanced retry and model switching, preferring Groq
// when configured and falling back to OpenRouter free models when available.
func (c *Client) ChatJSONWithRetry(ctx domain.Context, systemPrompt, userPrompt string, maxTokens int) (string, error) {
	lg := intobs.LoggerFromContext(ctx)

	groqPrimary := strings.TrimSpace(c.cfg.GroqAPIKey)
	groqSecondary := strings.TrimSpace(c.cfg.GroqAPIKey2)
	hasGroq1 := groqPrimary != ""
	hasGroq2 := groqSecondary != ""
	hasAnyGroq := hasGroq1 || hasGroq2
	orPrimary, orSecondary := c.getOpenRouterKeys()
	hasOR1 := orPrimary != ""
	hasOR2 := orSecondary != ""

	var groqErr error
	var orErr error
	var freeModels []freemodels.Model

	// 1) Primary/secondary: Groq accounts, when configured and not individually blocked
	hasUnblockedGroq1 := hasGroq1 && !c.isGroqAccountBlocked(groqPrimary)
	hasUnblockedGroq2 := hasGroq2 && !c.isGroqAccountBlocked(groqSecondary)
	if hasUnblockedGroq1 || hasUnblockedGroq2 {
		// Groq account 1
		if hasUnblockedGroq1 {
			res, err := c.callGroqChat(ctx, groqPrimary, systemPrompt, userPrompt, maxTokens)
			if err == nil {
				return res, nil
			}
			groqErr = err
			lg.Warn("Groq ChatJSONWithRetry primary account attempt failed",
				slog.String("provider", "groq"),
				slog.Any("error", err))
		}

		// Groq account 2
		if hasUnblockedGroq2 {
			res, err := c.callGroqChat(ctx, groqSecondary, systemPrompt, userPrompt, maxTokens)
			if err == nil {
				return res, nil
			}
			if groqErr == nil {
				groqErr = err
			} else {
				groqErr = fmt.Errorf("groq secondary account failed: %v; %w", err, groqErr)
			}
			lg.Warn("Groq ChatJSONWithRetry secondary account attempt failed",
				slog.String("provider", "groq"),
				slog.Any("error", err))
		}
	} else if hasAnyGroq {
		lg.Info("skipping Groq due to active rate limit block on all accounts", slog.String("provider", "groq"))
		groqErr = errors.New("groq rate limited and all accounts blocked")
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
				} else {
					freeModels = models
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
	if hasAnyGroq && (hasOR1 || hasOR2) {
		return "", fmt.Errorf("groq chat failed: %v; openrouter chat failed: %w", groqErr, orErr)
	}
	if hasAnyGroq {
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

	var result string
	err := c.obsOpenRouterChat.ExecuteWithMetrics(ctx, "chat_retry", func(callCtx context.Context) error {
		expo := c.getBackoffConfig()
		if !c.cfg.IsTest() {
			var cancel context.CancelFunc
			callCtx, cancel = context.WithTimeout(callCtx, 20*time.Second)
			defer cancel()
		}
		bo := backoff.WithContext(expo, callCtx)

		op := func() error {
			// Global limiter gate for OpenRouter account across workers
			if c.limiter != nil {
				allowed, retryAfter, err := c.limiter.Allow(callCtx, openRouterBucketKey(openRouterKey), 1)
				if err != nil {
					slog.Error("global rate limiter error for OpenRouter (model switching)", slog.Any("error", err))
				} else if !allowed {
					slog.Warn("global rate limiter denied OpenRouter call (model switching)",
						slog.String("provider", "openrouter"),
						slog.Duration("retry_after", retryAfter))
					c.blockOpenRouterAccount(openRouterKey, retryAfter)
					return backoff.Permanent(fmt.Errorf("rate limited: global limiter"))
				}
			}
			connectionStart := time.Now()
			// Client-level minimal spacing between OpenRouter calls to reduce 429s
			c.waitOpenRouterMinInterval()

			r, _ := http.NewRequestWithContext(callCtx, http.MethodPost, c.cfg.OpenRouterBaseURL+"/chat/completions", bytes.NewReader(b))
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
				return err
			}

			// Log connection duration for model switching
			slog.Info("OpenRouter API connection completed (model switching)",
				slog.String("model", model),
				slog.Duration("connection_duration", connectionDuration),
				slog.Int("status_code", resp.StatusCode),
				slog.String("x_request_id", resp.Header.Get("X-Request-Id")))

			defer func() { _ = resp.Body.Close() }()

			if resp.StatusCode == 429 {
				// Rate limit: log and record without needing the body content.
				slog.Warn("ai provider rate limited", slog.String("provider", "openrouter"), slog.String("op", "chat_retry"), slog.Int("status", resp.StatusCode))
				retryAfter := parseRetryAfterHeader(resp.Header.Get("Retry-After"))
				if c.rlc != nil {
					c.rlc.RecordRateLimit(model, retryAfter)
				}
				c.blockOpenRouterAccount(openRouterKey, retryAfter)
				c.updateOpenRouterLimiterFromRetryAfter(openRouterKey, retryAfter)
				return fmt.Errorf("rate limited: 429")
			}
			if resp.StatusCode >= 400 && resp.StatusCode < 500 {
				bodyBytes, _ := io.ReadAll(resp.Body)
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
				bodyBytes, _ := io.ReadAll(resp.Body)
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
			contentType := strings.ToLower(resp.Header.Get("Content-Type"))
			isStream := strings.Contains(contentType, "text/event-stream") && !c.cfg.IsTest()
			if isStream {
				content, err := readSSEChatStream(resp.Body, "openrouter", model, 20*time.Second)
				if err != nil {
					slog.Error("failed to read OpenRouter streaming response (model switching)", slog.String("provider", "openrouter"), slog.String("model", model), slog.Any("error", err))
					if c.rlc != nil {
						c.rlc.RecordFailure(model)
					}
					return err
				}
				if content == "" {
					slog.Error("OpenRouter streaming response produced empty content (model switching)", slog.String("provider", "openrouter"), slog.String("model", model))
					if c.rlc != nil {
						c.rlc.RecordFailure(model)
					}
					return fmt.Errorf("openrouter streaming response empty for model %s", model)
				}
				out.Model = model
				out.Choices = []struct {
					Message struct {
						Content string `json:"content"`
					} `json:"message"`
				}{
					{Message: struct {
						Content string `json:"content"`
					}{Content: content}},
				}
				// For streaming responses, estimate tokens using tiktoken since API doesn't return usage
				promptTokens, completionTokens := estimateChatTokens(systemPrompt, userPrompt, content)
				recordTokenUsage("openrouter", model, promptTokens, completionTokens)
				slog.Info("OpenRouter streaming token usage estimated (model switching)",
					slog.String("provider", "openrouter"),
					slog.String("model", model),
					slog.Int("prompt_tokens", promptTokens),
					slog.Int("completion_tokens", completionTokens),
					slog.Int("total_tokens", promptTokens+completionTokens))
				return nil
			}

			bodyBytes, err := io.ReadAll(resp.Body)
			if err != nil {
				slog.Error("failed to read response body", slog.String("provider", "openrouter"), slog.Any("error", err))
				return err
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

		if err := backoff.Retry(op, bo); err != nil {
			slog.Error("OpenRouter API failed after retries", slog.String("provider", "openrouter"), slog.String("model", model), slog.Any("error", err))
			return fmt.Errorf("openrouter api failed for model %s: %w", model, err)
		}

		if len(out.Choices) == 0 {
			slog.Error("OpenRouter API returned empty choices", slog.String("provider", "openrouter"), slog.String("model", model))
			return fmt.Errorf("openrouter api returned empty choices for model %s", model)
		}

		result = out.Choices[0].Message.Content
		return nil
	})
	if err != nil {
		return "", err
	}

	return result, nil
}

// callGroqChat calls Groq's OpenAI-compatible chat completions API using an internal
// curated model list. It stops on provider-level 429 and falls back to OpenRouter via
// higher-level logic when Groq is blocked.
func (c *Client) callGroqChat(ctx domain.Context, apiKey, systemPrompt, userPrompt string, maxTokens int) (string, error) {
	lg := intobs.LoggerFromContext(ctx)

	trimmedKey := strings.TrimSpace(apiKey)
	if trimmedKey == "" {
		lg.Error("Groq API key missing", slog.String("provider", "groq"))
		return "", fmt.Errorf("%w: GROQ_API_KEY missing", domain.ErrInvalidArgument)
	}

	var models []string
	if c.cfg.IsTest() {
		// In unit tests, preserve the original deterministic behaviour and avoid
		// hitting the Groq /models endpoint, since test servers only stub
		// /chat/completions. This keeps the expectation that we try exactly these
		// two models in order.
		models = []string{
			"llama-3.1-8b-instant",
			"llama-3.3-70b-versatile",
		}
	} else {
		models = c.getGroqModels(ctx, trimmedKey)
		if len(models) == 0 {
			models = []string{
				"llama-3.1-8b-instant",
				"llama-3.3-70b-versatile",
			}
		}
	}

	var lastErr error
	for _, model := range models {
		res, err := c.callGroqChatWithModel(ctx, trimmedKey, model, systemPrompt, userPrompt, maxTokens)
		if err == nil {
			return res, nil
		}
		lastErr = err

		// If we hit a provider-level rate limit, stop trying further models and
		// let higher-level logic fall back to OpenRouter.
		if strings.Contains(err.Error(), "rate limited:") {
			lg.Warn("Groq provider rate limited; not attempting further Groq models",
				slog.String("provider", "groq"),
				slog.String("model", model),
				slog.Any("error", err))
			break
		}

		lg.Warn("Groq model attempt failed, trying next model if available",
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
func (c *Client) callGroqChatWithModel(ctx domain.Context, apiKey, model, systemPrompt, userPrompt string, maxTokens int) (string, error) {
	lg := intobs.LoggerFromContext(ctx)

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
	lg.Debug("Groq API request body", slog.String("body", string(b)))

	var out struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
		Usage struct {
			PromptTokens     int `json:"prompt_tokens"`
			CompletionTokens int `json:"completion_tokens"`
			TotalTokens      int `json:"total_tokens"`
		} `json:"usage"`
		Model string `json:"model"`
	}

	var result string
	err := c.obsGroqChat.ExecuteWithMetrics(ctx, "chat", func(callCtx context.Context) error {
		expo := c.getBackoffConfig()
		if !c.cfg.IsTest() {
			var cancel context.CancelFunc
			callCtx, cancel = context.WithTimeout(callCtx, 20*time.Second)
			defer cancel()
		}
		bo := backoff.WithContext(expo, callCtx)

		op := func() error {
			endpoint := strings.TrimRight(baseURL, "/") + "/chat/completions"
			// Global limiter gate for Groq account across workers
			if c.limiter != nil {
				allowed, retryAfter, err := c.limiter.Allow(callCtx, groqBucketKey(apiKey), 1)
				if err != nil {
					lg.Error("global rate limiter error for Groq", slog.Any("error", err))
				} else if !allowed {
					lg.Warn("global rate limiter denied Groq call",
						slog.String("provider", "groq"),
						slog.Duration("retry_after", retryAfter))
					c.blockGroqAccount(apiKey, retryAfter)
					return backoff.Permanent(fmt.Errorf("rate limited: global limiter"))
				}
			}
			r, _ := http.NewRequestWithContext(callCtx, http.MethodPost, endpoint, bytes.NewReader(b))
			r.Header.Set("Authorization", "Bearer "+apiKey)
			r.Header.Set("Content-Type", "application/json")

			connectionStart := time.Now()
			resp, err := c.chatHC.Do(r)
			c.markGroqCall() // Mark the call timestamp for rate limiting
			connectionDuration := time.Since(connectionStart)
			if err != nil {
				lg.Info("Groq API connection attempt failed",
					slog.String("provider", "groq"),
					slog.Duration("connection_duration", connectionDuration))
				return err
			}

			lg.Info("Groq API connection completed",
				slog.String("provider", "groq"),
				slog.Duration("connection_duration", connectionDuration),
				slog.Int("status_code", resp.StatusCode))

			defer func() { _ = resp.Body.Close() }()
			// Update global limiter configuration from Groq rate-limit headers when present
			c.updateGroqLimiterFromHeaders(apiKey, resp.Header)

			if resp.StatusCode == http.StatusTooManyRequests {
				lg.Warn("ai provider rate limited", slog.String("provider", "groq"), slog.String("op", "chat"), slog.Int("status", resp.StatusCode))
				// Block Groq for 60 seconds (or Retry-After if provided) and fail fast
				retryAfter := parseRetryAfterHeader(resp.Header.Get("Retry-After"))
				c.blockGroqAccount(apiKey, retryAfter)
				// Return permanent error to stop retrying - let the caller fall back to OpenRouter
				return backoff.Permanent(fmt.Errorf("rate limited: %d", resp.StatusCode))
			}
			if resp.StatusCode >= 400 && resp.StatusCode < 500 {
				bodyBytes, _ := io.ReadAll(resp.Body)
				bodySnippet := string(bodyBytes)
				if len(bodySnippet) > 512 {
					bodySnippet = bodySnippet[:512]
				}
				lg.Warn("ai provider 4xx", slog.String("provider", "groq"), slog.String("op", "chat"), slog.Int("status", resp.StatusCode), slog.String("model", model), slog.String("endpoint", endpoint), slog.String("body", bodySnippet))
				return backoff.Permanent(fmt.Errorf("chat status %d", resp.StatusCode))
			}
			if resp.StatusCode < 200 || resp.StatusCode >= 300 {
				bodyBytes, _ := io.ReadAll(resp.Body)
				bodySnippet := string(bodyBytes)
				if len(bodySnippet) > 512 {
					bodySnippet = bodySnippet[:512]
				}
				lg.Error("ai provider non-2xx", slog.String("provider", "groq"), slog.String("op", "chat"), slog.Int("status", resp.StatusCode), slog.String("model", model), slog.String("endpoint", endpoint), slog.String("body", bodySnippet))
				return fmt.Errorf("chat status %d", resp.StatusCode)
			}
			contentType := strings.ToLower(resp.Header.Get("Content-Type"))
			isStream := strings.Contains(contentType, "text/event-stream") && !c.cfg.IsTest()
			if isStream {
				content, err := readSSEChatStream(resp.Body, "groq", model, 20*time.Second)
				if err != nil {
					lg.Error("failed to read Groq streaming response", slog.String("provider", "groq"), slog.String("model", model), slog.Any("error", err))
					return err
				}
				if content == "" {
					lg.Error("Groq streaming response produced empty content", slog.String("provider", "groq"), slog.String("model", model))
					return errors.New("empty content from Groq streaming response")
				}
				out.Choices = []struct {
					Message struct {
						Content string `json:"content"`
					} `json:"message"`
				}{
					{Message: struct {
						Content string `json:"content"`
					}{Content: content}},
				}
				// For streaming responses, estimate tokens using tiktoken since API doesn't return usage
				promptTokens, completionTokens := estimateChatTokens(systemPrompt, userPrompt, content)
				recordTokenUsage("groq", model, promptTokens, completionTokens)
				lg.Info("Groq streaming token usage estimated",
					slog.String("provider", "groq"),
					slog.String("model", model),
					slog.Int("prompt_tokens", promptTokens),
					slog.Int("completion_tokens", completionTokens),
					slog.Int("total_tokens", promptTokens+completionTokens))
				return nil
			}

			bodyBytes, err := io.ReadAll(resp.Body)
			if err != nil {
				lg.Error("failed to read response body", slog.String("provider", "groq"), slog.Any("error", err))
				return err
			}
			if err := json.Unmarshal(bodyBytes, &out); err != nil {
				lg.Error("ai provider decode error", slog.String("provider", "groq"), slog.String("op", "chat"), slog.String("model", model), slog.String("endpoint", endpoint), slog.Any("error", err))
				return err
			}
			return nil
		}

		lg.Info("starting Groq API retry logic", slog.String("provider", "groq"), slog.Duration("max_elapsed", expo.MaxElapsedTime))
		if err := backoff.Retry(op, bo); err != nil {
			lg.Error("Groq API failed after retries", slog.String("provider", "groq"), slog.Any("error", err))
			return fmt.Errorf("groq api failed: %w", err)
		}

		if len(out.Choices) == 0 {
			lg.Error("Groq API returned empty choices", slog.String("provider", "groq"))
			return errors.New("empty choices from Groq API")
		}

		// Record token usage from API response
		lg.Info("Groq API response usage info",
			slog.String("provider", "groq"),
			slog.String("model", model),
			slog.Int("prompt_tokens", out.Usage.PromptTokens),
			slog.Int("completion_tokens", out.Usage.CompletionTokens),
			slog.Int("total_tokens", out.Usage.TotalTokens))
		if out.Usage.TotalTokens > 0 {
			actualModel := out.Model
			if actualModel == "" {
				actualModel = model
			}
			recordTokenUsage("groq", actualModel, out.Usage.PromptTokens, out.Usage.CompletionTokens)
			lg.Info("Groq token usage recorded",
				slog.String("provider", "groq"),
				slog.String("model", actualModel),
				slog.Int("prompt_tokens", out.Usage.PromptTokens),
				slog.Int("completion_tokens", out.Usage.CompletionTokens),
				slog.Int("total_tokens", out.Usage.TotalTokens))
		} else {
			lg.Warn("Groq API response did not include usage info, estimating tokens",
				slog.String("provider", "groq"),
				slog.String("model", model))
			// Fallback to estimation if API doesn't return usage
			responseContent := out.Choices[0].Message.Content
			promptTokens, completionTokens := estimateChatTokens(systemPrompt, userPrompt, responseContent)
			recordTokenUsage("groq", model, promptTokens, completionTokens)
			lg.Info("Groq token usage estimated",
				slog.String("provider", "groq"),
				slog.String("model", model),
				slog.Int("prompt_tokens", promptTokens),
				slog.Int("completion_tokens", completionTokens))
		}

		result = out.Choices[0].Message.Content
		return nil
	})
	if err != nil {
		return "", err
	}

	return result, nil
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

// isGroqAccountBlocked checks if a specific Groq API key is currently blocked due to a recent 429 response.
func (c *Client) isGroqAccountBlocked(apiKey string) bool {
	key := strings.TrimSpace(apiKey)
	if key == "" {
		return false
	}
	if key == strings.TrimSpace(c.cfg.GroqAPIKey) {
		blockedUntil := c.groq1Blocked.Load()
		return blockedUntil != 0 && time.Now().UnixNano() < blockedUntil
	}
	if key == strings.TrimSpace(c.cfg.GroqAPIKey2) {
		blockedUntil := c.groq2Blocked.Load()
		return blockedUntil != 0 && time.Now().UnixNano() < blockedUntil
	}
	return false
}

// blockGroqAccount blocks a specific Groq API key for the given duration after a 429 response.
func (c *Client) blockGroqAccount(apiKey string, d time.Duration) {
	key := strings.TrimSpace(apiKey)
	if key == "" {
		return
	}
	if d <= 0 {
		d = 60 * time.Second // Default 60 second cooldown
	}
	blockedUntil := time.Now().Add(d).UnixNano()
	if key == strings.TrimSpace(c.cfg.GroqAPIKey) {
		c.groq1Blocked.Store(blockedUntil)
		slog.Warn("blocking Groq primary account due to rate limit",
			slog.Duration("block_duration", d))
		return
	}
	if key == strings.TrimSpace(c.cfg.GroqAPIKey2) {
		c.groq2Blocked.Store(blockedUntil)
		slog.Warn("blocking Groq secondary account due to rate limit",
			slog.Duration("block_duration", d))
		return
	}
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

func accountBucketKey(provider, apiKey string) string {
	key := strings.TrimSpace(apiKey)
	if key == "" {
		return provider + ":default"
	}
	sum := sha256.Sum256([]byte(key))
	// Use first 8 bytes for a short stable suffix
	short := hex.EncodeToString(sum[:8])
	return provider + ":" + short
}

func groqBucketKey(apiKey string) string {
	return accountBucketKey("groq", apiKey)
}

func openRouterBucketKey(apiKey string) string {
	return accountBucketKey("openrouter", apiKey)
}

func (c *Client) updateGroqLimiterFromHeaders(apiKey string, h http.Header) {
	if c == nil || c.limiter == nil {
		return
	}
	lua, ok := c.limiter.(*ratelimiter.RedisLuaLimiter)
	if !ok {
		return
	}
	limitStr := h.Get("x-ratelimit-limit-requests")
	if limitStr == "" {
		return
	}
	limitVal, err := strconv.ParseInt(limitStr, 10, 64)
	if err != nil || limitVal <= 0 {
		return
	}
	const daySeconds = 24 * 60 * 60
	cfg := ratelimiter.BucketConfig{
		Capacity:   limitVal,
		RefillRate: float64(limitVal) / daySeconds,
	}
	lua.SetBucketConfig(groqBucketKey(apiKey), cfg)
}

func (c *Client) updateOpenRouterLimiterFromRetryAfter(apiKey string, d time.Duration) {
	if c == nil || c.limiter == nil || d <= 0 {
		return
	}
	lua, ok := c.limiter.(*ratelimiter.RedisLuaLimiter)
	if !ok {
		return
	}
	seconds := d.Seconds()
	if seconds <= 0 {
		return
	}
	cfg := ratelimiter.BucketConfig{
		Capacity:   1,
		RefillRate: 1.0 / seconds,
	}
	lua.SetBucketConfig(openRouterBucketKey(apiKey), cfg)
}

type groqModelLimit struct {
	RPM int
	TPM int
}

var groqModelLimits = map[string]groqModelLimit{
	"llama-3.1-8b-instant":                          {RPM: 30, TPM: 6000},
	"llama-3.3-70b-versatile":                       {RPM: 30, TPM: 12000},
	"openai/gpt-oss-20b":                            {RPM: 30, TPM: 8000},
	"openai/gpt-oss-120b":                           {RPM: 30, TPM: 8000},
	"qwen/qwen3-32b":                                {RPM: 60, TPM: 6000},
	"meta-llama/llama-4-maverick-17b-128e-instruct": {RPM: 30, TPM: 6000},
	"meta-llama/llama-4-scout-17b-16e-instruct":     {RPM: 30, TPM: 30000},
}

func (c *Client) getGroqModels(ctx domain.Context, apiKey string) []string {
	c.groqModelsMu.RLock()
	if len(c.groqModels) > 0 && !c.groqModelsLastFetch.IsZero() && time.Since(c.groqModelsLastFetch) < c.cfg.FreeModelsRefresh {
		models := make([]string, len(c.groqModels))
		copy(models, c.groqModels)
		c.groqModelsMu.RUnlock()
		return models
	}
	c.groqModelsMu.RUnlock()

	c.groqModelsMu.Lock()
	defer c.groqModelsMu.Unlock()
	if len(c.groqModels) > 0 && !c.groqModelsLastFetch.IsZero() && time.Since(c.groqModelsLastFetch) < c.cfg.FreeModelsRefresh {
		models := make([]string, len(c.groqModels))
		copy(models, c.groqModels)
		return models
	}

	models, err := c.fetchGroqModelsFromAPI(ctx, apiKey)
	if err != nil || len(models) == 0 {
		fallback := make([]string, 0, len(groqModelLimits))
		for id := range groqModelLimits {
			fallback = append(fallback, id)
		}
		if len(fallback) > 1 {
			sort.SliceStable(fallback, func(i, j int) bool {
				li := groqModelLimits[fallback[i]]
				lj := groqModelLimits[fallback[j]]
				if li.TPM != lj.TPM {
					return li.TPM > lj.TPM
				}
				return li.RPM > lj.RPM
			})
		}
		c.groqModels = fallback
		c.groqModelsLastFetch = time.Now()
		models = make([]string, len(fallback))
		copy(models, fallback)
		return models
	}

	c.groqModels = models
	c.groqModelsLastFetch = time.Now()
	out := make([]string, len(models))
	copy(out, models)
	return out
}

func (c *Client) fetchGroqModelsFromAPI(ctx domain.Context, apiKey string) ([]string, error) {
	trimmedKey := strings.TrimSpace(apiKey)
	baseURL := strings.TrimSpace(c.cfg.GroqBaseURL)
	if baseURL == "" {
		baseURL = "https://api.groq.com/openai/v1"
	}
	url := strings.TrimRight(baseURL, "/") + "/models"

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("create groq models request: %w", err)
	}
	if trimmedKey != "" {
		req.Header.Set("Authorization", "Bearer "+trimmedKey)
	}

	resp, err := c.chatHC.Do(req)
	if err != nil {
		return nil, fmt.Errorf("groq models request failed: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("groq models status %d: %s", resp.StatusCode, string(b))
	}

	var out struct {
		Data []struct {
			ID string `json:"id"`
		} `json:"data"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return nil, fmt.Errorf("decode groq models response: %w", err)
	}

	models := make([]string, 0, len(out.Data))
	for _, m := range out.Data {
		id := strings.TrimSpace(m.ID)
		if id == "" {
			continue
		}
		if _, ok := groqModelLimits[id]; ok {
			models = append(models, id)
		}
	}

	if len(models) > 1 {
		sort.SliceStable(models, func(i, j int) bool {
			li := groqModelLimits[models[i]]
			lj := groqModelLimits[models[j]]
			if li.TPM != lj.TPM {
				return li.TPM > lj.TPM
			}
			return li.RPM > lj.RPM
		})
	}

	return models, nil
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
		Usage struct {
			PromptTokens int `json:"prompt_tokens"`
			TotalTokens  int `json:"total_tokens"`
		} `json:"usage"`
		Model string `json:"model"`
	}
	op := func(callCtx context.Context) error {
		connectionStart := time.Now()
		// Recreate request each attempt to avoid reusing consumed bodies
		r, _ := http.NewRequestWithContext(callCtx, http.MethodPost, c.cfg.OpenAIBaseURL+"/embeddings", bytes.NewReader(b))
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
			return err
		}

		// Log connection duration for embeddings with response details
		slog.Info("OpenAI API connection completed (embeddings)",
			slog.String("model", c.cfg.EmbeddingsModel),
			slog.Duration("connection_duration", connectionDuration),
			slog.Int("status_code", resp.StatusCode),
			slog.String("x_request_id", resp.Header.Get("X-Request-Id")))

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

	err := c.obsOpenAIEmbed.ExecuteWithMetrics(ctx, "embed", func(callCtx context.Context) error {
		expo := c.getBackoffConfig()
		bo := backoff.WithContext(expo, callCtx)

		slog.Info("starting OpenAI API retry logic", slog.String("provider", "openai"), slog.Duration("max_elapsed", expo.MaxElapsedTime))
		if err := backoff.Retry(func() error { return op(callCtx) }, bo); err != nil {
			slog.Error("OpenAI API failed after retries", slog.String("provider", "openai"), slog.Any("error", err))
			return fmt.Errorf("openai api failed: %w", err)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}

	if len(out.Data) == 0 {
		slog.Error("OpenAI API returned empty data", slog.String("provider", "openai"))
		return nil, errors.New("empty data from OpenAI API")
	}

	// Record token usage from API response (embeddings only have prompt tokens)
	if out.Usage.TotalTokens > 0 {
		actualModel := out.Model
		if actualModel == "" {
			actualModel = c.cfg.EmbeddingsModel
		}
		recordTokenUsage("openai", actualModel, out.Usage.PromptTokens, 0)
		slog.Info("OpenAI embeddings token usage recorded",
			slog.String("provider", "openai"),
			slog.String("model", actualModel),
			slog.Int("prompt_tokens", out.Usage.PromptTokens),
			slog.Int("total_tokens", out.Usage.TotalTokens))
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

	op := func(callCtx context.Context) error {
		// Respect global OpenRouter client-level throttling to avoid 429s during cleaning
		c.waitOpenRouterMinInterval()
		r, _ := http.NewRequestWithContext(callCtx, http.MethodPost, c.cfg.OpenRouterBaseURL+"/chat/completions", bytes.NewReader(b))
		r.Header.Set("Authorization", "Bearer "+openRouterKey)
		r.Header.Set("Content-Type", "application/json")
		resp, err := c.chatHC.Do(r)
		c.markOpenRouterCall()
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
			c.updateOpenRouterLimiterFromRetryAfter(openRouterKey, retryAfter)
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

	err = c.obsCotClean.ExecuteWithMetrics(ctx, "cot_cleaning", func(callCtx context.Context) error {
		expo := c.getBackoffConfig()
		bo := backoff.WithContext(expo, callCtx)

		slog.Info("starting CoT cleaning retry logic", slog.String("provider", "openrouter"), slog.Duration("max_elapsed", expo.MaxElapsedTime))
		if err := backoff.Retry(func() error { return op(callCtx) }, bo); err != nil {
			slog.Error("CoT cleaning failed after retries", slog.String("provider", "openrouter"), slog.Any("error", err))
			return fmt.Errorf("cot cleaning failed: %w", err)
		}
		return nil
	})
	if err != nil {
		return "", err
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
