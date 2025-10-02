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
	"sync"
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
	mu         sync.RWMutex
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
	s.mu.Lock()
	defer s.mu.Unlock()

	// Check if we need to refresh the models list
	needsRefresh := s.models == nil || time.Since(s.lastFetch) > s.refreshDur

	if needsRefresh {
		slog.Info("fetching free models from OpenRouter API",
			slog.String("base_url", s.baseURL),
			slog.Duration("refresh_interval", s.refreshDur))

		models, err := s.fetchModelsFromAPI(ctx)
		if err != nil {
			// If API fetch fails and we have cached models, use them
			if s.models != nil {
				slog.Warn("failed to fetch models from API, using cached models",
					slog.Any("error", err),
					slog.Int("cached_count", len(s.models)))
				return s.models, nil
			}
			return nil, fmt.Errorf("failed to fetch models from API: %w", err)
		}

		s.models = models
		s.lastFetch = time.Now()

		slog.Info("successfully fetched free models",
			slog.Int("count", len(models)),
			slog.Time("last_fetch", s.lastFetch))
	}

	return s.models, nil
}

// fetchModelsFromAPI fetches all models from OpenRouter API and filters free ones
func (s *Service) fetchModelsFromAPI(ctx context.Context) ([]Model, error) {
	url := s.baseURL + "/models"
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	if s.apiKey != "" {
		req.Header.Set("Authorization", "Bearer "+s.apiKey)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := s.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to make request: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("API request failed with status %d: %s", resp.StatusCode, string(body))
	}

	var apiResp OpenRouterResponse
	if err := json.NewDecoder(resp.Body).Decode(&apiResp); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	// Filter for free models only
	freeModels := s.filterFreeModels(apiResp.Data)

	slog.Info("filtered free models from OpenRouter API",
		slog.Int("total_models", len(apiResp.Data)),
		slog.Int("free_models", len(freeModels)))

	return freeModels, nil
}

// filterFreeModels filters models to only include free ones
func (s *Service) filterFreeModels(models []Model) []Model {
	var freeModels []Model

	for _, model := range models {
		if s.isFreeModel(model) {
			freeModels = append(freeModels, model)
		}
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
	s.mu.Lock()
	s.models = nil
	s.lastFetch = time.Time{}
	s.mu.Unlock()

	// Force refresh by calling the internal logic
	s.mu.Lock()
	defer s.mu.Unlock()

	slog.Info("fetching free models from OpenRouter API",
		slog.String("base_url", s.baseURL),
		slog.Duration("refresh_interval", s.refreshDur))

	models, err := s.fetchModelsFromAPI(ctx)
	if err != nil {
		return fmt.Errorf("failed to fetch models from API: %w", err)
	}

	s.models = models
	s.lastFetch = time.Now()

	slog.Info("successfully fetched free models",
		slog.Int("count", len(models)),
		slog.Time("last_fetch", s.lastFetch))

	return nil
}
