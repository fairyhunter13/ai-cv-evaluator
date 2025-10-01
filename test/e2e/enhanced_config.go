// Package e2e provides enhanced E2E test configuration for better reliability.
package e2e

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/fairyhunter13/ai-cv-evaluator/internal/adapter/ai"
	"github.com/fairyhunter13/ai-cv-evaluator/internal/domain"
)

// EnhancedE2EConfig provides configuration for enhanced E2E tests.
type EnhancedE2EConfig struct {
	// Timeouts
	JobTimeout         time.Duration
	APITimeout         time.Duration
	ModelHealthTimeout time.Duration

	// Retry Configuration
	MaxRetries        int
	RetryDelay        time.Duration
	BackoffMultiplier float64

	// Model Validation
	ValidateModelHealth    bool
	ValidateJSONResponse   bool
	ValidateModelStability bool

	// Parallel Execution
	MaxParallelJobs       int
	EnableModelValidation bool
}

// DefaultEnhancedE2EConfig returns a default configuration for enhanced E2E tests.
func DefaultEnhancedE2EConfig() *EnhancedE2EConfig {
	return &EnhancedE2EConfig{
		// Increased timeouts for free models
		JobTimeout:         5 * time.Minute,   // Increased from 2 minutes
		APITimeout:         120 * time.Second, // Increased from 30 seconds
		ModelHealthTimeout: 30 * time.Second,

		// Retry configuration
		MaxRetries:        5,               // Increased from 3
		RetryDelay:        2 * time.Second, // Increased from 1 second
		BackoffMultiplier: 1.5,             // Reduced from 2.0 for faster retries

		// Model validation
		ValidateModelHealth:    true,
		ValidateJSONResponse:   true,
		ValidateModelStability: false, // Disabled for faster tests

		// Parallel execution
		MaxParallelJobs:       2, // Reduced from 4 to reduce load
		EnableModelValidation: true,
	}
}

// ValidateModelBeforeTest validates model health before running E2E tests.
func ValidateModelBeforeTest(ctx context.Context, aiClient domain.AIClient) error {
	slog.Info("validating model before E2E test")

	validator := ai.NewModelValidator(aiClient)

	// Quick health check
	healthCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	if err := validator.ValidateModelHealth(healthCtx); err != nil {
		slog.Error("model health validation failed", slog.Any("error", err))
		return fmt.Errorf("model health validation failed: %w", err)
	}

	// JSON response validation
	jsonCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	if err := validator.ValidateJSONResponse(jsonCtx); err != nil {
		slog.Error("JSON response validation failed", slog.Any("error", err))
		return fmt.Errorf("JSON response validation failed: %w", err)
	}

	slog.Info("model validation passed, proceeding with E2E test")
	return nil
}

// WaitForModelStability waits for model to be stable before running tests.
func WaitForModelStability(ctx context.Context, aiClient domain.AIClient, maxWaitTime time.Duration) error {
	slog.Info("waiting for model stability", slog.Duration("max_wait", maxWaitTime))

	validator := ai.NewModelValidator(aiClient)
	start := time.Now()

	for time.Since(start) < maxWaitTime {
		// Quick stability check
		stabilityCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
		err := validator.ValidateModelStability(stabilityCtx)
		cancel()

		if err == nil {
			slog.Info("model stability achieved")
			return nil
		}

		slog.Warn("model stability check failed, retrying",
			slog.Any("error", err),
			slog.Duration("elapsed", time.Since(start)))

		// Wait before retry
		time.Sleep(2 * time.Second)
	}

	return fmt.Errorf("model stability not achieved within %v", maxWaitTime)
}

// EnhancedTestSetup performs enhanced setup for E2E tests.
func EnhancedTestSetup(ctx context.Context, aiClient domain.AIClient) error {
	slog.Info("performing enhanced E2E test setup")

	// Validate model health
	if err := ValidateModelBeforeTest(ctx, aiClient); err != nil {
		return fmt.Errorf("model validation failed: %w", err)
	}

	// Wait for model stability (optional)
	// if err := WaitForModelStability(ctx, aiClient, 30*time.Second); err != nil {
	// 	slog.Warn("model stability wait failed, continuing anyway", slog.Any("error", err))
	// }

	slog.Info("enhanced E2E test setup completed")
	return nil
}
