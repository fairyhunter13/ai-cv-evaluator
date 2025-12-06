package ai

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
)

// OpenRouterAPIKeyResponse represents the response from the OpenRouter API key endpoint
// Based on: https://openrouter.ai/docs/api-reference/limits
type OpenRouterAPIKeyResponse struct {
	Data struct {
		Label             string   `json:"label"`
		Usage             float64  `json:"usage"`               // Number of credits used
		Limit             *float64 `json:"limit"`               // Credit limit, null if unlimited
		IsFreeTier        bool     `json:"is_free_tier"`        // Whether user has paid for credits
		LimitRemaining    *float64 `json:"limit_remaining"`     // Remaining credits, null if unlimited
		IsProvisioningKey bool     `json:"is_provisioning_key"` // Whether this is a provisioning key
	} `json:"data"`
}

// RateLimitChecker provides functionality to check OpenRouter API rate limits
type RateLimitChecker struct {
	apiKey     string
	baseURL    string
	httpClient *http.Client
}

// NewRateLimitChecker creates a new rate limit checker with OpenTelemetry tracing
func NewRateLimitChecker(apiKey, baseURL string) *RateLimitChecker {
	// Use otelhttp transport for distributed tracing
	transport := otelhttp.NewTransport(http.DefaultTransport,
		otelhttp.WithSpanNameFormatter(func(_ string, r *http.Request) string {
			return fmt.Sprintf("RateLimit %s %s", r.Method, r.URL.Host)
		}),
	)
	return &RateLimitChecker{
		apiKey:  apiKey,
		baseURL: baseURL,
		httpClient: &http.Client{
			Timeout:   30 * time.Second,
			Transport: transport,
		},
	}
}

// CheckRateLimit checks the current API key usage and remaining quota
func (r *RateLimitChecker) CheckRateLimit(ctx context.Context) (*OpenRouterAPIKeyResponse, error) {
	url := fmt.Sprintf("%s/key", r.baseURL)

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Authorization", "Bearer "+r.apiKey)
	req.Header.Set("Content-Type", "application/json")

	resp, err := r.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to make request: %w", err)
	}
	defer func() {
		_ = resp.Body.Close()
	}()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("API key check failed with status %d", resp.StatusCode)
	}

	var apiKeyResp OpenRouterAPIKeyResponse
	if err := json.NewDecoder(resp.Body).Decode(&apiKeyResp); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return &apiKeyResp, nil
}

// HasSufficientQuota checks if there's enough quota remaining for the operation
func (r *RateLimitChecker) HasSufficientQuota(ctx context.Context, requiredQuota float64) (bool, *OpenRouterAPIKeyResponse, error) {
	apiKeyResp, err := r.CheckRateLimit(ctx)
	if err != nil {
		return false, nil, err
	}

	// Check if we have sufficient quota remaining
	// For unlimited accounts (limit_remaining is null), always return true
	var hasQuota bool
	if apiKeyResp.Data.LimitRemaining == nil {
		// Unlimited account - always has quota
		hasQuota = true
	} else {
		// Limited account - check remaining quota
		hasQuota = *apiKeyResp.Data.LimitRemaining >= requiredQuota
	}

	return hasQuota, apiKeyResp, nil
}

// WaitForQuota waits for sufficient quota to become available
func (r *RateLimitChecker) WaitForQuota(ctx context.Context, requiredQuota float64, maxWaitTime time.Duration) (*OpenRouterAPIKeyResponse, error) {
	start := time.Now()

	for time.Since(start) < maxWaitTime {
		hasQuota, apiKeyResp, err := r.HasSufficientQuota(ctx, requiredQuota)
		if err != nil {
			return nil, err
		}

		if hasQuota {
			return apiKeyResp, nil
		}

		// Wait before checking again
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-time.After(5 * time.Second):
			// Continue checking
		}
	}

	return nil, fmt.Errorf("insufficient quota after waiting %v", maxWaitTime)
}

// GetQuotaInfo returns current quota information
func (r *RateLimitChecker) GetQuotaInfo(ctx context.Context) (limit, usage, remaining float64, isFreeTier bool, err error) {
	apiKeyResp, err := r.CheckRateLimit(ctx)
	if err != nil {
		return 0, 0, 0, false, err
	}

	// Handle null limit (unlimited)
	var limitValue float64
	if apiKeyResp.Data.Limit != nil {
		limitValue = *apiKeyResp.Data.Limit
	} else {
		limitValue = -1 // -1 indicates unlimited
	}

	// Handle null remaining (unlimited)
	var remainingValue float64
	if apiKeyResp.Data.LimitRemaining != nil {
		remainingValue = *apiKeyResp.Data.LimitRemaining
	} else {
		remainingValue = -1 // -1 indicates unlimited
	}

	return limitValue, apiKeyResp.Data.Usage, remainingValue, apiKeyResp.Data.IsFreeTier, nil
}

// CheckFreeModelLimits checks if the account can use free models based on OpenRouter limits
// Based on: https://openrouter.ai/docs/api-reference/limits
func (r *RateLimitChecker) CheckFreeModelLimits(ctx context.Context) (canUseFreeModels bool, dailyLimit int, err error) {
	apiKeyResp, err := r.CheckRateLimit(ctx)
	if err != nil {
		return false, 0, err
	}

	// Free model limits based on documentation:
	// - 20 requests per minute for free models
	// - Daily limits: 50 requests if <10 credits purchased, 1000 requests if >=10 credits purchased

	if apiKeyResp.Data.IsFreeTier {
		// User has never purchased credits - 50 requests per day
		return true, 50, nil
	}
	// User has purchased credits - 1000 requests per day
	return true, 1000, nil
}

// CheckAccountStatus checks if the account is in good standing
func (r *RateLimitChecker) CheckAccountStatus(ctx context.Context) (isActive bool, hasCredits bool, err error) {
	apiKeyResp, err := r.CheckRateLimit(ctx)
	if err != nil {
		return false, false, err
	}

	// Check if account has credits (not negative balance)
	// For unlimited accounts (limit is null), consider it as having credits
	hasCredits = (apiKeyResp.Data.LimitRemaining != nil && *apiKeyResp.Data.LimitRemaining > 0.0) || apiKeyResp.Data.Limit == nil

	// Account is active if it has credits or is free tier
	isActive = hasCredits || apiKeyResp.Data.IsFreeTier

	return isActive, hasCredits, nil
}
