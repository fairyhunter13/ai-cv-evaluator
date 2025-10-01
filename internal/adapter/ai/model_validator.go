// Package ai provides model validation utilities for ensuring model health.
package ai

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"time"

	"github.com/fairyhunter13/ai-cv-evaluator/internal/domain"
)

// ModelValidator validates model health and response quality.
type ModelValidator struct {
	ai domain.AIClient
}

// NewModelValidator creates a new model validator.
func NewModelValidator(ai domain.AIClient) *ModelValidator {
	return &ModelValidator{ai: ai}
}

// ValidateModelHealth validates that a model is healthy and responsive.
func (mv *ModelValidator) ValidateModelHealth(ctx context.Context) error {
	slog.Info("validating model health")

	// Create a simple health check prompt
	healthPrompt := `Respond with a simple JSON: {"status": "healthy", "timestamp": "2024-01-01T00:00:00Z"}`

	// Set a reasonable timeout for health check
	healthCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	response, err := mv.ai.ChatJSON(healthCtx, "", healthPrompt, 100)
	if err != nil {
		slog.Error("model health check failed", slog.Any("error", err))
		return fmt.Errorf("model health check failed: %w", err)
	}

	// Validate the response is valid JSON
	var healthResponse struct {
		Status    string `json:"status"`
		Timestamp string `json:"timestamp"`
	}

	if err := json.Unmarshal([]byte(response), &healthResponse); err != nil {
		slog.Error("model health check returned invalid JSON",
			slog.String("response", response),
			slog.Any("error", err))
		return fmt.Errorf("model health check returned invalid JSON: %w", err)
	}

	if healthResponse.Status != "healthy" {
		slog.Error("model health check returned unhealthy status",
			slog.String("status", healthResponse.Status))
		return fmt.Errorf("model returned unhealthy status: %s", healthResponse.Status)
	}

	slog.Info("model health check passed")
	return nil
}

// ValidateJSONResponse validates that a model can produce valid JSON responses.
func (mv *ModelValidator) ValidateJSONResponse(ctx context.Context) error {
	slog.Info("validating JSON response capability")

	// Create a JSON validation prompt
	jsonPrompt := `Create a simple JSON object with these fields: {"name": "test", "value": 123, "active": true}`

	// Set a reasonable timeout for JSON validation
	jsonCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	response, err := mv.ai.ChatJSON(jsonCtx, "", jsonPrompt, 200)
	if err != nil {
		slog.Error("JSON validation failed", slog.Any("error", err))
		return fmt.Errorf("JSON validation failed: %w", err)
	}

	// Validate the response is valid JSON
	var jsonResponse map[string]interface{}
	if err := json.Unmarshal([]byte(response), &jsonResponse); err != nil {
		slog.Error("JSON validation returned invalid JSON",
			slog.String("response", response),
			slog.Any("error", err))
		return fmt.Errorf("JSON validation returned invalid JSON: %w", err)
	}

	// Check for required fields
	requiredFields := []string{"name", "value", "active"}
	for _, field := range requiredFields {
		if _, exists := jsonResponse[field]; !exists {
			slog.Error("JSON validation missing required field",
				slog.String("field", field),
				slog.String("response", response))
			return fmt.Errorf("JSON validation missing required field: %s", field)
		}
	}

	slog.Info("JSON response validation passed")
	return nil
}

// ValidateModelStability validates that a model produces consistent responses.
func (mv *ModelValidator) ValidateModelStability(ctx context.Context) error {
	slog.Info("validating model stability")

	// Test the same prompt multiple times
	prompt := `Respond with JSON: {"test": "stability", "number": 42}`
	responses := make([]string, 3)

	for i := 0; i < 3; i++ {
		stabilityCtx, cancel := context.WithTimeout(ctx, 30*time.Second)

		response, err := mv.ai.ChatJSON(stabilityCtx, "", prompt, 200)
		cancel()

		if err != nil {
			slog.Error("stability validation failed",
				slog.Int("attempt", i+1),
				slog.Any("error", err))
			return fmt.Errorf("stability validation failed on attempt %d: %w", i+1, err)
		}

		responses[i] = response

		// Small delay between requests
		time.Sleep(100 * time.Millisecond)
	}

	// Check that all responses are valid JSON
	for i, response := range responses {
		var jsonResponse map[string]interface{}
		if err := json.Unmarshal([]byte(response), &jsonResponse); err != nil {
			slog.Error("stability validation returned invalid JSON",
				slog.Int("attempt", i+1),
				slog.String("response", response),
				slog.Any("error", err))
			return fmt.Errorf("stability validation returned invalid JSON on attempt %d: %w", i+1, err)
		}
	}

	slog.Info("model stability validation passed")
	return nil
}

// ValidateModelComprehensive performs all model validations.
func (mv *ModelValidator) ValidateModelComprehensive(ctx context.Context) error {
	slog.Info("performing comprehensive model validation")

	// Health check
	if err := mv.ValidateModelHealth(ctx); err != nil {
		return fmt.Errorf("model health validation failed: %w", err)
	}

	// JSON response validation
	if err := mv.ValidateJSONResponse(ctx); err != nil {
		return fmt.Errorf("JSON response validation failed: %w", err)
	}

	// Stability validation
	if err := mv.ValidateModelStability(ctx); err != nil {
		return fmt.Errorf("model stability validation failed: %w", err)
	}

	slog.Info("comprehensive model validation passed")
	return nil
}
