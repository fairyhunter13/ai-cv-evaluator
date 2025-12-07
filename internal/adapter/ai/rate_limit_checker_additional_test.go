package ai

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestRateLimitChecker_WaitForQuota_ImmediateSuccess(t *testing.T) {
	mockResponse := map[string]any{
		"data": map[string]any{
			"label":               "Test API Key",
			"usage":               1.0,
			"limit":               nil,
			"is_free_tier":        false,
			"limit_remaining":     10.0,
			"is_provisioning_key": false,
		},
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, "/key", r.URL.Path)
		w.Header().Set("Content-Type", "application/json")
		require.NoError(t, json.NewEncoder(w).Encode(mockResponse))
	}))
	defer server.Close()

	checker := NewRateLimitChecker("test-api-key", server.URL)
	ctx := context.Background()

	resp, err := checker.WaitForQuota(ctx, 1.0, 50*time.Millisecond)
	require.NoError(t, err)
	require.NotNil(t, resp)
}

func TestRateLimitChecker_GetQuotaInfo_LimitedAndUnlimited(t *testing.T) {
	limitedLimit := 100.0
	limitedRemaining := 40.0

	// First call: limited account
	limitedResp := map[string]any{
		"data": map[string]any{
			"label":               "Limited API Key",
			"usage":               60.0,
			"limit":               limitedLimit,
			"is_free_tier":        false,
			"limit_remaining":     limitedRemaining,
			"is_provisioning_key": false,
		},
	}

	// Second call: unlimited account (nil limit/remaining)
	unlimitedResp := map[string]any{
		"data": map[string]any{
			"label":               "Unlimited API Key",
			"usage":               10.0,
			"limit":               nil,
			"is_free_tier":        true,
			"limit_remaining":     nil,
			"is_provisioning_key": false,
		},
	}

	call := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if call == 0 {
			require.NoError(t, json.NewEncoder(w).Encode(limitedResp))
			call++
			return
		}
		require.NoError(t, json.NewEncoder(w).Encode(unlimitedResp))
	}))
	defer server.Close()

	checker := NewRateLimitChecker("test-api-key", server.URL)
	ctx := context.Background()

	// Limited account
	limit, usage, remaining, isFree, err := checker.GetQuotaInfo(ctx)
	require.NoError(t, err)
	require.Equal(t, limitedLimit, limit)
	require.Equal(t, 60.0, usage)
	require.Equal(t, limitedRemaining, remaining)
	require.False(t, isFree)

	// Unlimited account
	limit2, usage2, remaining2, isFree2, err := checker.GetQuotaInfo(ctx)
	require.NoError(t, err)
	require.Equal(t, -1.0, limit2)
	require.Equal(t, 10.0, usage2)
	require.Equal(t, -1.0, remaining2)
	require.True(t, isFree2)
}

func TestRateLimitChecker_WaitForQuota_Timeout(t *testing.T) {
	// Server that always returns insufficient quota
	mockResponse := map[string]any{
		"data": map[string]any{
			"label":               "Test API Key",
			"usage":               100.0,
			"limit":               100.0,
			"is_free_tier":        false,
			"limit_remaining":     0.0,
			"is_provisioning_key": false,
		},
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		require.NoError(t, json.NewEncoder(w).Encode(mockResponse))
	}))
	defer server.Close()

	checker := NewRateLimitChecker("test-api-key", server.URL)
	ctx := context.Background()

	// Should timeout since quota is always insufficient
	resp, err := checker.WaitForQuota(ctx, 10.0, 100*time.Millisecond)
	require.Error(t, err)
	require.Nil(t, resp)
	require.Contains(t, err.Error(), "insufficient quota")
}

func TestRateLimitChecker_WaitForQuota_ContextCancelled(t *testing.T) {
	// Server that returns insufficient quota
	mockResponse := map[string]any{
		"data": map[string]any{
			"label":               "Test API Key",
			"usage":               100.0,
			"limit":               100.0,
			"is_free_tier":        false,
			"limit_remaining":     0.0,
			"is_provisioning_key": false,
		},
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		require.NoError(t, json.NewEncoder(w).Encode(mockResponse))
	}))
	defer server.Close()

	checker := NewRateLimitChecker("test-api-key", server.URL)
	ctx, cancel := context.WithCancel(context.Background())

	// Cancel context after a short delay
	go func() {
		time.Sleep(50 * time.Millisecond)
		cancel()
	}()

	resp, err := checker.WaitForQuota(ctx, 10.0, 10*time.Second)
	require.Error(t, err)
	require.Nil(t, resp)
	require.ErrorIs(t, err, context.Canceled)
}

func TestRateLimitChecker_CheckFreeModelLimits_FreeTier(t *testing.T) {
	mockResponse := map[string]any{
		"data": map[string]any{
			"label":               "Free Tier Key",
			"usage":               0.0,
			"limit":               nil,
			"is_free_tier":        true,
			"limit_remaining":     nil,
			"is_provisioning_key": false,
		},
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		require.NoError(t, json.NewEncoder(w).Encode(mockResponse))
	}))
	defer server.Close()

	checker := NewRateLimitChecker("test-api-key", server.URL)
	ctx := context.Background()

	canUse, dailyLimit, err := checker.CheckFreeModelLimits(ctx)
	require.NoError(t, err)
	require.True(t, canUse)
	require.Equal(t, 50, dailyLimit) // Free tier gets 50 requests/day
}

func TestRateLimitChecker_CheckFreeModelLimits_PaidTier(t *testing.T) {
	mockResponse := map[string]any{
		"data": map[string]any{
			"label":               "Paid Tier Key",
			"usage":               50.0,
			"limit":               1000.0,
			"is_free_tier":        false,
			"limit_remaining":     950.0,
			"is_provisioning_key": false,
		},
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		require.NoError(t, json.NewEncoder(w).Encode(mockResponse))
	}))
	defer server.Close()

	checker := NewRateLimitChecker("test-api-key", server.URL)
	ctx := context.Background()

	canUse, dailyLimit, err := checker.CheckFreeModelLimits(ctx)
	require.NoError(t, err)
	require.True(t, canUse)
	require.Equal(t, 1000, dailyLimit) // Paid tier gets 1000 requests/day
}

func TestRateLimitChecker_CheckAccountStatus(t *testing.T) {
	mockResponse := map[string]any{
		"data": map[string]any{
			"label":               "Active Account",
			"usage":               25.0,
			"limit":               100.0,
			"is_free_tier":        false,
			"limit_remaining":     75.0,
			"is_provisioning_key": false,
		},
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		require.NoError(t, json.NewEncoder(w).Encode(mockResponse))
	}))
	defer server.Close()

	checker := NewRateLimitChecker("test-api-key", server.URL)
	ctx := context.Background()

	isActive, hasCredits, err := checker.CheckAccountStatus(ctx)
	require.NoError(t, err)
	require.True(t, isActive)
	require.True(t, hasCredits)
}

func TestRateLimitChecker_CheckAccountStatus_NoCredits(t *testing.T) {
	mockResponse := map[string]any{
		"data": map[string]any{
			"label":               "No Credits Account",
			"usage":               100.0,
			"limit":               100.0,
			"is_free_tier":        true,
			"limit_remaining":     0.0,
			"is_provisioning_key": false,
		},
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		require.NoError(t, json.NewEncoder(w).Encode(mockResponse))
	}))
	defer server.Close()

	checker := NewRateLimitChecker("test-api-key", server.URL)
	ctx := context.Background()

	isActive, hasCredits, err := checker.CheckAccountStatus(ctx)
	require.NoError(t, err)
	require.True(t, isActive)
	require.False(t, hasCredits) // No remaining credits
}
