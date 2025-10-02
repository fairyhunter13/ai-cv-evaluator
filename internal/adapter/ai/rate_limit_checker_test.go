package ai

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRateLimitChecker_UnlimitedAccount(t *testing.T) {
	// Test with unlimited account (null limit and limit_remaining)
	mockResponse := map[string]interface{}{
		"data": map[string]interface{}{
			"label":               "Test API Key",
			"usage":               6.92,
			"limit":               nil,
			"is_free_tier":        false,
			"limit_remaining":     nil,
			"is_provisioning_key": false,
		},
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "GET", r.Method)
		assert.Equal(t, "/key", r.URL.Path)
		assert.Equal(t, "Bearer test-api-key", r.Header.Get("Authorization"))

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(mockResponse)
	}))
	defer server.Close()

	checker := NewRateLimitChecker("test-api-key", server.URL)

	ctx := context.Background()

	// Test 1: Check rate limit
	response, err := checker.CheckRateLimit(ctx)
	require.NoError(t, err)
	assert.Equal(t, "Test API Key", response.Data.Label)
	assert.Equal(t, 6.92, response.Data.Usage)
	assert.Nil(t, response.Data.Limit)          // Should be null for unlimited
	assert.Nil(t, response.Data.LimitRemaining) // Should be null for unlimited
	assert.False(t, response.Data.IsFreeTier)

	// Test 2: Check quota (should always return true for unlimited)
	hasQuota, _, err := checker.HasSufficientQuota(ctx, 1.0)
	require.NoError(t, err)
	assert.True(t, hasQuota, "Unlimited account should always have quota")

	// Test 3: Check account status
	isActive, hasCredits, err := checker.CheckAccountStatus(ctx)
	require.NoError(t, err)
	assert.True(t, isActive, "Unlimited account should be active")
	assert.True(t, hasCredits, "Unlimited account should have credits")

	// Test 4: Check free model limits
	canUseFreeModels, dailyLimit, err := checker.CheckFreeModelLimits(ctx)
	require.NoError(t, err)
	assert.True(t, canUseFreeModels)
	assert.Equal(t, 1000, dailyLimit, "Paid account should have 1000 daily free model requests")
}

func TestRateLimitChecker_LimitedAccount(t *testing.T) {
	// Test with limited account
	limitValue := 10.0
	remainingValue := 5.0

	mockResponse := map[string]interface{}{
		"data": map[string]interface{}{
			"label":               "Test API Key",
			"usage":               5.0,
			"limit":               &limitValue,
			"is_free_tier":        false,
			"limit_remaining":     &remainingValue,
			"is_provisioning_key": false,
		},
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(mockResponse)
	}))
	defer server.Close()

	checker := NewRateLimitChecker("test-api-key", server.URL)

	ctx := context.Background()

	// Test quota check with sufficient quota
	hasQuota, _, err := checker.HasSufficientQuota(ctx, 3.0)
	require.NoError(t, err)
	assert.True(t, hasQuota, "Should have sufficient quota")

	// Test quota check with insufficient quota
	hasQuota, _, err = checker.HasSufficientQuota(ctx, 10.0)
	require.NoError(t, err)
	assert.False(t, hasQuota, "Should not have sufficient quota")
}
