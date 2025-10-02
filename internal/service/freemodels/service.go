// Package freemodels provides a service for managing free AI models from OpenRouter.
package freemodels

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strings"
	"time"
)

// Model represents a model from OpenRouter API
type Model struct {
	ID          string  `json:"id"`
	Name        string  `json:"name"`
	Description string  `json:"description"`
	Pricing     Pricing `json:"pricing"`
}

// Pricing represents the pricing information for a model
type Pricing struct {
	Prompt     string `json:"prompt"`
	Completion string `json:"completion"`
	Request    string `json:"request"`
	Image      string `json:"image"`
}

// OpenRouterResponse represents the response from OpenRouter API
type OpenRouterResponse struct {
	Data []Model `json:"data"`
}

// Service handles fetching and managing free models from OpenRouter
type Service struct {
	apiKey     string
	baseURL    string
	httpClient *http.Client
	models     []Model
	lastFetch  time.Time
	refreshDur time.Duration
}

// NewService creates a new free models service
func NewService(apiKey, baseURL string, refreshDur time.Duration) *Service {
	return &Service{
		apiKey:     apiKey,
		baseURL:    baseURL,
		httpClient: &http.Client{Timeout: 30 * time.Second},
		refreshDur: refreshDur,
	}
}

// GetFreeModels returns the list of free models, fetching from API if needed
func (s *Service) GetFreeModels(ctx context.Context) ([]Model, error) {
	slog.Debug("GetFreeModels called",
		slog.Bool("has_cached_models", s.models != nil),
		slog.Time("last_fetch", s.lastFetch),
		slog.Duration("time_since_fetch", time.Since(s.lastFetch)),
		slog.Duration("refresh_duration", s.refreshDur))

	// Check if we need to refresh the models list
	if s.models == nil || time.Since(s.lastFetch) > s.refreshDur {
		slog.Info("fetching free models from OpenRouter API",
			slog.String("base_url", s.baseURL),
			slog.Duration("refresh_interval", s.refreshDur),
			slog.Bool("has_cached_models", s.models != nil),
			slog.Int("cached_count", len(s.models)))

		models, err := s.fetchModelsFromAPI(ctx)
		if err != nil {
			slog.Error("failed to fetch models from OpenRouter API",
				slog.Any("error", err),
				slog.String("base_url", s.baseURL),
				slog.Bool("api_key_present", s.apiKey != ""))

			// If API fetch fails and we have cached models, use them
			if s.models != nil {
				slog.Warn("using cached models due to API failure",
					slog.Any("error", err),
					slog.Int("cached_count", len(s.models)))
				return s.models, nil
			}
			return nil, fmt.Errorf("failed to fetch models from API: %w", err)
		}

		s.models = models
		s.lastFetch = time.Now()
		slog.Info("successfully fetched free models from OpenRouter API",
			slog.Int("count", len(models)),
			slog.Time("last_fetch", s.lastFetch),
			slog.String("base_url", s.baseURL))

		// Log details about the fetched models
		for i, model := range models {
			slog.Debug("free model details",
				slog.Int("index", i),
				slog.String("id", model.ID),
				slog.String("name", model.Name),
				slog.String("pricing_prompt", model.Pricing.Prompt),
				slog.String("pricing_completion", model.Pricing.Completion))
		}
	} else {
		slog.Debug("using cached free models",
			slog.Int("count", len(s.models)),
			slog.Time("last_fetch", s.lastFetch))
	}

	return s.models, nil
}

// fetchModelsFromAPI fetches all models from OpenRouter API and filters free ones
func (s *Service) fetchModelsFromAPI(ctx context.Context) ([]Model, error) {
	url := s.baseURL + "/models"
	slog.Debug("creating HTTP request to OpenRouter API",
		slog.String("url", url),
		slog.Bool("api_key_present", s.apiKey != ""))

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		slog.Error("failed to create HTTP request",
			slog.Any("error", err),
			slog.String("url", url))
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	if s.apiKey != "" {
		req.Header.Set("Authorization", "Bearer "+s.apiKey)
		slog.Debug("added authorization header to request")
	} else {
		slog.Warn("no API key provided for OpenRouter request")
	}
	req.Header.Set("Content-Type", "application/json")

	slog.Debug("making HTTP request to OpenRouter API",
		slog.String("url", url),
		slog.String("method", "GET"))

	start := time.Now()
	resp, err := s.httpClient.Do(req)
	duration := time.Since(start)

	if err != nil {
		slog.Error("HTTP request to OpenRouter API failed",
			slog.Any("error", err),
			slog.String("url", url),
			slog.Duration("duration", duration))
		return nil, fmt.Errorf("failed to make request: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	slog.Debug("received response from OpenRouter API",
		slog.Int("status_code", resp.StatusCode),
		slog.Duration("duration", duration),
		slog.String("url", url))

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		slog.Error("OpenRouter API returned non-200 status",
			slog.Int("status_code", resp.StatusCode),
			slog.String("response_body", string(body)),
			slog.String("url", url))

		// Handle specific error cases
		if resp.StatusCode == 429 {
			return nil, fmt.Errorf("OpenRouter API rate limited (429): %s", string(body))
		}
		if resp.StatusCode == 401 {
			return nil, fmt.Errorf("OpenRouter API authentication failed (401): %s", string(body))
		}

		return nil, fmt.Errorf("API request failed with status %d: %s", resp.StatusCode, string(body))
	}

	var apiResp OpenRouterResponse
	if err := json.NewDecoder(resp.Body).Decode(&apiResp); err != nil {
		slog.Error("failed to decode OpenRouter API response",
			slog.Any("error", err),
			slog.String("url", url))
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	slog.Info("successfully fetched models from OpenRouter API",
		slog.Int("total_models", len(apiResp.Data)),
		slog.Duration("duration", duration),
		slog.String("url", url))

	// Filter for free models only
	freeModels := s.filterFreeModels(apiResp.Data)

	slog.Info("filtered free models from OpenRouter API",
		slog.Int("total_models", len(apiResp.Data)),
		slog.Int("free_models", len(freeModels)),
		slog.Duration("duration", duration))

	return freeModels, nil
}

// filterFreeModels filters models to only include free ones
func (s *Service) filterFreeModels(models []Model) []Model {
	var freeModels []Model
	var paidModels []string

	slog.Debug("filtering models for free ones",
		slog.Int("total_models", len(models)))

	for _, model := range models {
		if s.isFreeModel(model) {
			freeModels = append(freeModels, model)
			slog.Debug("model identified as free",
				slog.String("id", model.ID),
				slog.String("name", model.Name),
				slog.String("pricing_prompt", model.Pricing.Prompt),
				slog.String("pricing_completion", model.Pricing.Completion))
		} else {
			paidModels = append(paidModels, model.ID)
		}
	}

	slog.Info("model filtering completed",
		slog.Int("total_models", len(models)),
		slog.Int("free_models", len(freeModels)),
		slog.Int("paid_models", len(paidModels)))

	if len(paidModels) > 0 {
		slog.Debug("paid models excluded",
			slog.String("paid_model_ids", fmt.Sprintf("%v", paidModels)))
	}

	return freeModels
}

// isFreeModel checks if a model is free based on its pricing
func (s *Service) isFreeModel(model Model) bool {
	// A model is considered free if all pricing fields are "0" or empty
	pricing := model.Pricing

	// Check if all pricing fields indicate free usage
	isPromptFree := pricing.Prompt == "0" || pricing.Prompt == "" || pricing.Prompt == "0.0"
	isCompletionFree := pricing.Completion == "0" || pricing.Completion == "" || pricing.Completion == "0.0"
	isRequestFree := pricing.Request == "0" || pricing.Request == "" || pricing.Request == "0.0"
	isImageFree := pricing.Image == "0" || pricing.Image == "" || pricing.Image == "0.0"

	// All pricing must be free
	allFree := isPromptFree && isCompletionFree && isRequestFree && isImageFree

	// STRICT BAN: Explicitly exclude openrouter/auto and other problematic models
	modelID := strings.ToLower(model.ID)

	// First check: explicit ban on openrouter/auto
	if modelID == "openrouter/auto" {
		slog.Warn("BANNED MODEL DETECTED: openrouter/auto is strictly prohibited",
			slog.String("model_id", model.ID),
			slog.String("reason", "openrouter/auto causes API timeouts and failures"))
		return false
	}

	// Additional check: exclude known paid model patterns
	excludedPatterns := []string{
		"gpt-4", "gpt-5", "claude-3", "gemini-pro", "mistral-large",
		"mixtral-8x", "llama-2-70b", "llama-2-13b", "command-",
		"auto", // openrouter/auto and similar auto-select models
	}

	for _, pattern := range excludedPatterns {
		if strings.Contains(modelID, pattern) {
			slog.Debug("excluding model with paid/problematic pattern",
				slog.String("model_id", model.ID),
				slog.String("pattern", pattern))
			return false
		}
	}

	if allFree {
		slog.Debug("identified free model",
			slog.String("model_id", model.ID),
			slog.String("name", model.Name),
			slog.String("pricing", fmt.Sprintf("prompt:%s,completion:%s,request:%s,image:%s",
				pricing.Prompt, pricing.Completion, pricing.Request, pricing.Image)))
	}

	return allFree
}

// GetModelIDs returns just the model IDs for easy use
func (s *Service) GetModelIDs(ctx context.Context) ([]string, error) {
	models, err := s.GetFreeModels(ctx)
	if err != nil {
		return nil, err
	}

	ids := make([]string, len(models))
	for i, model := range models {
		ids[i] = model.ID
	}

	return ids, nil
}

// Refresh forces a refresh of the models list
func (s *Service) Refresh(ctx context.Context) error {
	s.models = nil
	s.lastFetch = time.Time{}
	_, err := s.GetFreeModels(ctx)
	return err
}
