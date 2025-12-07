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
