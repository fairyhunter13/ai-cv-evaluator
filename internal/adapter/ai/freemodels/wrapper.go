// Package freemodels provides a wrapper for using free OpenRouter models.
package freemodels

import (
	"context"
	"log/slog"

	"github.com/fairyhunter13/ai-cv-evaluator/internal/adapter/ai/real"
	"github.com/fairyhunter13/ai-cv-evaluator/internal/config"
	"github.com/fairyhunter13/ai-cv-evaluator/internal/domain"
	"github.com/fairyhunter13/ai-cv-evaluator/internal/service/freemodels"
)

// FreeModelWrapper wraps the real AI client to use free models automatically.
type FreeModelWrapper struct {
	client        domain.AIClient
	freeModelsSvc *freemodels.Service
	cfg           config.Config
}

// NewFreeModelWrapper creates a new wrapper that automatically uses free models.
func NewFreeModelWrapper(cfg config.Config) *FreeModelWrapper {
	// Create the underlying real client
	realClient := real.New(cfg)

	// Create free models service with configurable refresh interval.
	// Prefer primary OpenRouter key but fall back to OPENROUTER_API_KEY_2 when
	// only the secondary key is configured.
	openRouterKey := cfg.OpenRouterAPIKey
	if openRouterKey == "" {
		openRouterKey = cfg.OpenRouterAPIKey2
	}
	freeModelsSvc := freemodels.NewService(openRouterKey, cfg.OpenRouterBaseURL, cfg.FreeModelsRefresh)

	return &FreeModelWrapper{
		client:        realClient,
		freeModelsSvc: freeModelsSvc,
		cfg:           cfg,
	}
}

// ChatJSON implements domain.AIClient using free models with automatic fallback.
func (w *FreeModelWrapper) ChatJSON(ctx context.Context, systemPrompt, userPrompt string, maxTokens int) (string, error) {
	// The real client now handles free model selection dynamically
	// Just delegate to the underlying client
	result, err := w.client.ChatJSON(ctx, systemPrompt, userPrompt, maxTokens)
	if err != nil {
		slog.Warn("free model request failed", slog.Any("error", err))
		return "", err
	}

	slog.Info("successfully used free model",
		slog.Int("result_length", len(result)))

	return result, nil
}

// ChatJSONWithRetry implements domain.AIClient using free models with retry logic and model fallback.
func (w *FreeModelWrapper) ChatJSONWithRetry(ctx context.Context, systemPrompt, userPrompt string, maxTokens int) (string, error) {
	// Use the enhanced retry method with model fallback
	result, err := w.client.ChatJSONWithRetry(ctx, systemPrompt, userPrompt, maxTokens)
	if err != nil {
		slog.Warn("free model request with retry failed", slog.Any("error", err))
		return "", err
	}

	slog.Info("successfully used free model with retry",
		slog.Int("result_length", len(result)))

	return result, nil
}

// Embed delegates to the underlying client (embeddings are typically free).
func (w *FreeModelWrapper) Embed(ctx context.Context, texts []string) ([][]float32, error) {
	return w.client.Embed(ctx, texts)
}

// CleanCoTResponse delegates to the underlying client for CoT cleaning.
func (w *FreeModelWrapper) CleanCoTResponse(ctx context.Context, response string) (string, error) {
	return w.client.CleanCoTResponse(ctx, response)
}
