// Package freemodels provides a wrapper for using free OpenRouter models.
package freemodels

import (
	"context"
	"fmt"
	"log/slog"
	"sync"

	"github.com/fairyhunter13/ai-cv-evaluator/internal/adapter/ai/real"
	"github.com/fairyhunter13/ai-cv-evaluator/internal/config"
	"github.com/fairyhunter13/ai-cv-evaluator/internal/domain"
)

// FreeModelWrapper wraps the real AI client to use free models automatically.
type FreeModelWrapper struct {
	client        domain.AIClient
	freeModelsSvc *Service
	cfg           config.Config
	lastModel     string
	modelFailures map[string]int
	maxFailures   int
	roundRobinIdx int
	mu            sync.Mutex // Protects roundRobinIdx and modelFailures
}

// NewFreeModelWrapper creates a new wrapper that automatically uses free models.
func NewFreeModelWrapper(cfg config.Config) *FreeModelWrapper {
	// Create the underlying real client
	realClient := real.New(cfg)

	// Create free models service with configurable refresh interval
	freeModelsSvc := NewWithRefresh(cfg.OpenRouterAPIKey, cfg.OpenRouterBaseURL, cfg.FreeModelsRefresh)

	return &FreeModelWrapper{
		client:        realClient,
		freeModelsSvc: freeModelsSvc,
		cfg:           cfg,
		modelFailures: make(map[string]int),
		maxFailures:   3, // Max failures before blacklisting a model
	}
}

// ChatJSON implements domain.AIClient using free models with automatic fallback.
func (w *FreeModelWrapper) ChatJSON(ctx domain.Context, systemPrompt, userPrompt string, maxTokens int) (string, error) {
	// Try to get a free model
	modelID, err := w.selectFreeModel(ctx)
	if err != nil {
		slog.Warn("failed to get free model, falling back to configured model", slog.Any("error", err))
		return w.client.ChatJSON(ctx, systemPrompt, userPrompt, maxTokens)
	}

	// Check if we just reset the failure counts (all models were blacklisted)
	w.mu.Lock()
	wasReset := len(w.modelFailures) == 0
	w.mu.Unlock()

	// Create a modified config with the free model
	freeModelCfg := w.cfg
	freeModelCfg.ChatModel = modelID
	freeModelCfg.ChatFallbackModels = []string{} // Clear fallbacks to avoid conflicts

	// Create a temporary client with the free model
	tempClient := real.New(freeModelCfg)

	// Try the free model
	result, err := tempClient.ChatJSON(ctx, systemPrompt, userPrompt, maxTokens)
	if err != nil {
		// Track failures for this model (thread-safe) - but not if we just reset
		if !wasReset {
			w.mu.Lock()
			w.modelFailures[modelID]++
			failureCount := w.modelFailures[modelID]
			w.mu.Unlock()

			slog.Warn("free model failed, will try another",
				slog.String("model", modelID),
				slog.Int("failures", failureCount),
				slog.Any("error", err))

			// If this model has failed too many times, it's blacklisted
			if failureCount >= w.maxFailures {
				slog.Warn("model blacklisted due to repeated failures", slog.String("model", modelID))
			}
		} else {
			slog.Warn("free model failed after reset, not tracking failure",
				slog.String("model", modelID),
				slog.Any("error", err))
		}

		// Fallback to the original client with configured model
		return w.client.ChatJSON(ctx, systemPrompt, userPrompt, maxTokens)
	}

	// Success - reset failure count (thread-safe)
	w.mu.Lock()
	w.modelFailures[modelID] = 0
	w.lastModel = modelID
	w.mu.Unlock()

	slog.Info("successfully used free model",
		slog.String("model", modelID),
		slog.String("provider", "openrouter"))

	return result, nil
}

// Embed delegates to the underlying client (embeddings are typically free).
func (w *FreeModelWrapper) Embed(ctx domain.Context, texts []string) ([][]float32, error) {
	return w.client.Embed(ctx, texts)
}

// selectFreeModel selects an appropriate free model using round-robin.
func (w *FreeModelWrapper) selectFreeModel(ctx context.Context) (string, error) {
	// Get available free models
	models, err := w.freeModelsSvc.GetFreeModels(ctx)
	if err != nil {
		return "", fmt.Errorf("failed to get free models: %w", err)
	}

	if len(models) == 0 {
		return "", fmt.Errorf("no free models available")
	}

	// Thread-safe access to filter out blacklisted models and select next model
	w.mu.Lock()
	defer w.mu.Unlock()

	// Filter out blacklisted models
	var availableModels []string
	for _, model := range models {
		if w.modelFailures[model.ID] < w.maxFailures {
			availableModels = append(availableModels, model.ID)
		}
	}

	if len(availableModels) == 0 {
		// Reset failure counts if all models are blacklisted
		slog.Warn("all models blacklisted, resetting failure counts")
		w.modelFailures = make(map[string]int)
		availableModels = make([]string, len(models))
		for i, model := range models {
			availableModels[i] = model.ID
		}
		// Reset round-robin index when all models are reset
		w.roundRobinIdx = 0
	}

	// Use round-robin selection to distribute load across different models
	selectedModel := availableModels[w.roundRobinIdx%len(availableModels)]

	// Increment round-robin index for next selection
	w.roundRobinIdx++

	slog.Debug("selected model using round-robin",
		slog.String("model", selectedModel),
		slog.Int("index", w.roundRobinIdx-1),
		slog.Int("total_available", len(availableModels)))

	return selectedModel, nil
}

// GetFreeModelsInfo returns information about available free models.
func (w *FreeModelWrapper) GetFreeModelsInfo(ctx context.Context) ([]string, error) {
	return w.freeModelsSvc.GetFreeModelIDs(ctx)
}

// GetCurrentModel returns the currently selected model.
func (w *FreeModelWrapper) GetCurrentModel() string {
	return w.lastModel
}

// GetRoundRobinIndex returns the current round-robin index for debugging.
func (w *FreeModelWrapper) GetRoundRobinIndex() int {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.roundRobinIdx
}

// GetModelStats returns statistics about model usage and failures.
func (w *FreeModelWrapper) GetModelStats() map[string]int {
	w.mu.Lock()
	defer w.mu.Unlock()

	stats := make(map[string]int)
	for model, failures := range w.modelFailures {
		stats[model] = failures
	}
	return stats
}
