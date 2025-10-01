// Package freemodels implements a service to fetch and manage free OpenRouter models.
package freemodels

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"sort"
	"strings"
	"sync"
	"time"
)

// Model represents an OpenRouter model with pricing information.
type Model struct {
	ID          string      `json:"id"`
	Name        string      `json:"name"`
	Pricing     Pricing     `json:"pricing"`
	Context     int         `json:"context_length"`
	Description string      `json:"description"`
	TopProvider json.RawMessage `json:"top_provider,omitempty"`
}

// Pricing contains the cost information for a model.
type Pricing struct {
	Prompt     any `json:"prompt"`     // Price per token for prompts
	Completion any `json:"completion"` // Price per token for completions
}

// Service manages free OpenRouter models with automatic fetching and rotation.
type Service struct {
	httpClient    *http.Client
	models        []Model
	modelsMutex   sync.RWMutex
	lastFetch     time.Time
	fetchInterval time.Duration
	apiKey        string
	baseURL       string
}

// New creates a new free models service.
func New(apiKey, baseURL string) *Service {
	return &Service{
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
		fetchInterval: 1 * time.Hour, // Default refresh every hour
		apiKey:        apiKey,
		baseURL:       baseURL,
	}
}

// NewWithRefresh creates a new free models service with custom refresh interval.
func NewWithRefresh(apiKey, baseURL string, refreshInterval time.Duration) *Service {
	return &Service{
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
		fetchInterval: refreshInterval,
		apiKey:        apiKey,
		baseURL:       baseURL,
	}
}

// GetFreeModels returns a list of free models, fetching from OpenRouter if needed.
func (s *Service) GetFreeModels(ctx context.Context) ([]Model, error) {
	s.modelsMutex.RLock()
	needsRefresh := s.lastFetch.IsZero() || time.Since(s.lastFetch) > s.fetchInterval
	s.modelsMutex.RUnlock()

	if needsRefresh {
		if err := s.fetchModels(ctx); err != nil {
			slog.Warn("failed to fetch fresh models, using cached", slog.Any("error", err))
		}
	}

	s.modelsMutex.RLock()
	defer s.modelsMutex.RUnlock()

	// Return a copy to avoid race conditions
	result := make([]Model, len(s.models))
	copy(result, s.models)
	return result, nil
}

// GetRandomFreeModel returns a random free model for load balancing.
func (s *Service) GetRandomFreeModel(ctx context.Context) (string, error) {
	models, err := s.GetFreeModels(ctx)
	if err != nil {
		return "", err
	}

	if len(models) == 0 {
		return "", fmt.Errorf("no free models available")
	}

	// Use current time as seed for pseudo-random selection
	index := int(time.Now().UnixNano()) % len(models)
	return models[index].ID, nil
}

// GetBestFreeModel returns the best free model based on context length and reliability.
func (s *Service) GetBestFreeModel(ctx context.Context) (string, error) {
	models, err := s.GetFreeModels(ctx)
	if err != nil {
		return "", err
	}

	if len(models) == 0 {
		return "", fmt.Errorf("no free models available")
	}

	// Sort by context length (descending) and then by name for consistency
	sort.Slice(models, func(i, j int) bool {
		if models[i].Context != models[j].Context {
			return models[i].Context > models[j].Context
		}
		return models[i].Name < models[j].Name
	})

	return models[0].ID, nil
}

// fetchModels fetches the latest models from OpenRouter API.
func (s *Service) fetchModels(ctx context.Context) error {
	req, err := http.NewRequestWithContext(ctx, "GET", s.baseURL+"/models", nil)
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	if s.apiKey != "" {
		req.Header.Set("Authorization", "Bearer "+s.apiKey)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := s.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("failed to fetch models: %w", err)
	}
	defer func() {
		if closeErr := resp.Body.Close(); closeErr != nil {
			slog.Warn("failed to close response body", slog.Any("error", closeErr))
		}
	}()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("API returned status %d", resp.StatusCode)
	}

	var response struct {
		Data []Model `json:"data"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&response); err != nil {
		return fmt.Errorf("failed to decode response: %w", err)
	}

	// Filter for free models (prompt price is zero across possible formats)
	var freeModels []Model
	for _, model := range response.Data {
		if s.isFreeModel(model) {
			freeModels = append(freeModels, model)
		}
	}

	s.modelsMutex.Lock()
	s.models = freeModels
	s.lastFetch = time.Now()
	s.modelsMutex.Unlock()

	slog.Info("fetched free models from OpenRouter",
		slog.Int("total_models", len(response.Data)),
		slog.Int("free_models", len(freeModels)))

	return nil
}

// isFreeModel checks if a model is free based on its pricing.
func (s *Service) isFreeModel(model Model) bool {
	// Check if prompt pricing is free across flexible shapes.
	return priceIsFree(model.Pricing.Prompt)
}

// priceIsFree determines whether a pricing value represents a free price.
// It supports strings ("", "0", "0.0"), numbers (0), and nested objects (any zero-like value).
func priceIsFree(v any) bool {
	switch t := v.(type) {
	case nil:
		// Treat missing as free to match previous behavior (empty => free)
		return true
	case string:
		s := strings.TrimSpace(t)
		return s == "" || s == "0" || s == "0.0"
	case float64:
		return t == 0
	case map[string]any:
		// Consider free if any nested value indicates zero price.
		for _, vv := range t {
			if priceIsFree(vv) {
				return true
			}
		}
		return false
	default:
		return false
	}
}

// GetModelInfo returns information about a specific model.
func (s *Service) GetModelInfo(ctx context.Context, modelID string) (*Model, error) {
	models, err := s.GetFreeModels(ctx)
	if err != nil {
		return nil, err
	}

	for _, model := range models {
		if model.ID == modelID {
			return &model, nil
		}
	}

	return nil, fmt.Errorf("model %s not found in free models", modelID)
}

// GetFreeModelIDs returns just the model IDs for easy use in configuration.
func (s *Service) GetFreeModelIDs(ctx context.Context) ([]string, error) {
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

// GetRefreshStatus returns information about the last refresh and next refresh time.
func (s *Service) GetRefreshStatus() (lastFetch time.Time, nextFetch time.Time, refreshInterval time.Duration) {
	s.modelsMutex.RLock()
	defer s.modelsMutex.RUnlock()

	lastFetch = s.lastFetch
	refreshInterval = s.fetchInterval
	nextFetch = s.lastFetch.Add(s.fetchInterval)

	return lastFetch, nextFetch, refreshInterval
}

// ForceRefresh forces an immediate refresh of the models list.
func (s *Service) ForceRefresh(ctx context.Context) error {
	return s.fetchModels(ctx)
}
