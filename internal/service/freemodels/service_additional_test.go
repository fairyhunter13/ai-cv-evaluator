package freemodels

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestService_GetFreeModels(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name          string
		apiKey        string
		baseURL       string
		refreshDur    time.Duration
		mockSetup     func() *httptest.Server
		expectedCount int
		expectedError bool
	}{
		{
			name:       "successful_fetch",
			apiKey:     "test-key",
			baseURL:    "",
			refreshDur: 1 * time.Hour,
			mockSetup: func() *httptest.Server {
				return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
					w.Header().Set("Content-Type", "application/json")
					_ = json.NewEncoder(w).Encode(map[string]any{
						"data": []map[string]any{
							{
								"id":   "test-model-1",
								"name": "Test Model 1",
								"pricing": map[string]string{
									"prompt":     "0",
									"completion": "0",
									"request":    "0",
									"image":      "0",
								},
							},
							{
								"id":   "test-model-2",
								"name": "Test Model 2",
								"pricing": map[string]string{
									"prompt":     "0.01",
									"completion": "0.01",
									"request":    "0.01",
									"image":      "0.01",
								},
							},
						},
					})
				}))
			},
			expectedCount: 1, // Only the first model is free
			expectedError: false,
		},
		{
			name:       "api_error",
			apiKey:     "test-key",
			baseURL:    "",
			refreshDur: 1 * time.Hour,
			mockSetup: func() *httptest.Server {
				return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
					w.WriteHeader(http.StatusInternalServerError)
					_, _ = w.Write([]byte("Internal Server Error"))
				}))
			},
			expectedCount: 0,
			expectedError: true,
		},
		{
			name:       "no_models",
			apiKey:     "test-key",
			baseURL:    "",
			refreshDur: 1 * time.Hour,
			mockSetup: func() *httptest.Server {
				return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
					w.Header().Set("Content-Type", "application/json")
					_ = json.NewEncoder(w).Encode(map[string]any{
						"data": []map[string]any{},
					})
				}))
			},
			expectedCount: 0,
			expectedError: false,
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			server := tt.mockSetup()
			defer server.Close()

			service := NewService(tt.apiKey, server.URL, tt.refreshDur)
			models, err := service.GetFreeModels(context.Background())

			if tt.expectedError {
				assert.Error(t, err)
				assert.Nil(t, models)
			} else {
				assert.NoError(t, err)
				assert.Len(t, models, tt.expectedCount)
			}
		})
	}
}

func TestService_GetModelIDs(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name          string
		apiKey        string
		baseURL       string
		refreshDur    time.Duration
		mockSetup     func() *httptest.Server
		expectedIDs   []string
		expectedError bool
	}{
		{
			name:       "successful_fetch",
			apiKey:     "test-key",
			baseURL:    "",
			refreshDur: 1 * time.Hour,
			mockSetup: func() *httptest.Server {
				return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
					w.Header().Set("Content-Type", "application/json")
					_ = json.NewEncoder(w).Encode(map[string]any{
						"data": []map[string]any{
							{
								"id":   "test-model-1",
								"name": "Test Model 1",
								"pricing": map[string]string{
									"prompt":     "0",
									"completion": "0",
									"request":    "0",
									"image":      "0",
								},
							},
							{
								"id":   "test-model-2",
								"name": "Test Model 2",
								"pricing": map[string]string{
									"prompt":     "0",
									"completion": "0",
									"request":    "0",
									"image":      "0",
								},
							},
						},
					})
				}))
			},
			expectedIDs:   []string{"test-model-1", "test-model-2"},
			expectedError: false,
		},
		{
			name:       "api_error",
			apiKey:     "test-key",
			baseURL:    "",
			refreshDur: 1 * time.Hour,
			mockSetup: func() *httptest.Server {
				return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
					w.WriteHeader(http.StatusInternalServerError)
					_, _ = w.Write([]byte("Internal Server Error"))
				}))
			},
			expectedIDs:   nil,
			expectedError: true,
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			server := tt.mockSetup()
			defer server.Close()

			service := NewService(tt.apiKey, server.URL, tt.refreshDur)
			ids, err := service.GetModelIDs(context.Background())

			if tt.expectedError {
				assert.Error(t, err)
				assert.Nil(t, ids)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.expectedIDs, ids)
			}
		})
	}
}

func TestService_Refresh(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name          string
		apiKey        string
		baseURL       string
		refreshDur    time.Duration
		mockSetup     func() *httptest.Server
		expectedError bool
	}{
		{
			name:       "successful_refresh",
			apiKey:     "test-key",
			baseURL:    "",
			refreshDur: 1 * time.Hour,
			mockSetup: func() *httptest.Server {
				return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
					w.Header().Set("Content-Type", "application/json")
					_ = json.NewEncoder(w).Encode(map[string]any{
						"data": []map[string]any{
							{
								"id":   "test-model-1",
								"name": "Test Model 1",
								"pricing": map[string]string{
									"prompt":     "0",
									"completion": "0",
									"request":    "0",
									"image":      "0",
								},
							},
						},
					})
				}))
			},
			expectedError: false,
		},
		{
			name:       "refresh_with_error",
			apiKey:     "test-key",
			baseURL:    "",
			refreshDur: 1 * time.Hour,
			mockSetup: func() *httptest.Server {
				return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
					w.WriteHeader(http.StatusInternalServerError)
					_, _ = w.Write([]byte("Internal Server Error"))
				}))
			},
			expectedError: true,
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			server := tt.mockSetup()
			defer server.Close()

			service := NewService(tt.apiKey, server.URL, tt.refreshDur)
			err := service.Refresh(context.Background())

			if tt.expectedError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestService_GetFreeModels_CacheBehavior(t *testing.T) {
	t.Parallel()

	// Test that models are cached and not refetched within refresh duration
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"data": []map[string]any{
				{
					"id":   "test-model-1",
					"name": "Test Model 1",
					"pricing": map[string]string{
						"prompt":     "0",
						"completion": "0",
						"request":    "0",
						"image":      "0",
					},
				},
			},
		})
	}))
	defer server.Close()

	service := NewService("test-key", server.URL, 1*time.Hour)

	// First call should fetch from API
	models1, err1 := service.GetFreeModels(context.Background())
	require.NoError(t, err1)
	require.Len(t, models1, 1)

	// Second call should use cache (no new API call)
	models2, err2 := service.GetFreeModels(context.Background())
	require.NoError(t, err2)
	require.Len(t, models2, 1)
	require.Equal(t, models1[0].ID, models2[0].ID)
}

func TestService_GetFreeModels_RefreshAfterDuration(t *testing.T) {
	t.Parallel()

	callCount := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		callCount++
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"data": []map[string]any{
				{
					"id":   "test-model-1",
					"name": "Test Model 1",
					"pricing": map[string]string{
						"prompt":     "0",
						"completion": "0",
						"request":    "0",
						"image":      "0",
					},
				},
			},
		})
	}))
	defer server.Close()

	// Use a very short refresh duration to test refresh behavior
	service := NewService("test-key", server.URL, 1*time.Millisecond)

	// First call
	models1, err1 := service.GetFreeModels(context.Background())
	require.NoError(t, err1)
	require.Len(t, models1, 1)
	require.Equal(t, 1, callCount)

	// Wait for refresh duration to pass
	time.Sleep(10 * time.Millisecond)

	// Second call should trigger refresh
	models2, err2 := service.GetFreeModels(context.Background())
	require.NoError(t, err2)
	require.Len(t, models2, 1)
	require.Equal(t, 2, callCount) // Should have made another API call
}

func TestService_FetchAllModelsFromAPI(t *testing.T) {
	t.Parallel()

	t.Run("success", func(t *testing.T) {
		t.Parallel()

		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(OpenRouterResponse{
				Data: []Model{
					{ID: "model-1", Name: "Model 1", Pricing: Pricing{Prompt: "0.01", Completion: "0.01", Request: "0.02"}},
					{ID: "model-2", Name: "Model 2", Pricing: Pricing{Prompt: "0.02", Completion: "0.02", Request: "0.04"}},
				},
			})
		}))
		defer server.Close()

		service := NewService("test-key", server.URL, 1*time.Hour)
		models, err := service.fetchAllModelsFromAPI(context.Background())
		require.NoError(t, err)
		require.Len(t, models, 2)
		assert.Equal(t, "model-1", models[0].ID)
		assert.Equal(t, "model-2", models[1].ID)
	})

	t.Run("non_200_status", func(t *testing.T) {
		t.Parallel()

		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusInternalServerError)
			_, _ = w.Write([]byte("internal error"))
		}))
		defer server.Close()

		service := NewService("test-key", server.URL, 1*time.Hour)
		models, err := service.fetchAllModelsFromAPI(context.Background())
		require.Error(t, err)
		assert.Nil(t, models)
	})

	t.Run("invalid_json", func(t *testing.T) {
		t.Parallel()

		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte("not-json"))
		}))
		defer server.Close()

		service := NewService("test-key", server.URL, 1*time.Hour)
		models, err := service.fetchAllModelsFromAPI(context.Background())
		require.Error(t, err)
		assert.Nil(t, models)
	})
}

func TestService_GetCheapestPaidModels(t *testing.T) {
	t.Parallel()

	t.Run("zero_limit_returns_nil", func(t *testing.T) {
		t.Parallel()

		service := NewService("test-key", "http://example", 1*time.Hour)
		models, err := service.GetCheapestPaidModels(context.Background(), 0)
		require.NoError(t, err)
		assert.Nil(t, models)
	})

	t.Run("fetch_error_propagated", func(t *testing.T) {
		t.Parallel()

		// Use an invalid base URL to force a request error
		service := NewService("test-key", "http://127.0.0.1:0", 1*time.Hour)
		models, err := service.GetCheapestPaidModels(context.Background(), 3)
		require.Error(t, err)
		assert.Nil(t, models)
	})

	t.Run("sorts_and_limits_paid_models", func(t *testing.T) {
		t.Parallel()

		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(OpenRouterResponse{
				Data: []Model{
					{
						ID:   "free-model",
						Name: "Free Model",
						Pricing: Pricing{
							Prompt:     "0",
							Completion: "0",
							Request:    "0",
							Image:      "0",
						},
					},
					{
						ID:   "paid-cheap",
						Name: "Paid Cheap",
						Pricing: Pricing{
							Prompt:     "0.001",
							Completion: "0.001",
							Request:    "0",
						},
					},
					{
						ID:   "paid-expensive",
						Name: "Paid Expensive",
						Pricing: Pricing{
							Request: "0.05",
						},
					},
					{
						ID:   "openrouter/auto",
						Name: "Auto Banned",
						Pricing: Pricing{
							Request: "0.000001",
						},
					},
				},
			})
		}))
		defer server.Close()

		service := NewService("test-key", server.URL, 1*time.Hour)
		ctx := context.Background()

		// limit larger than candidates exercises minInt when a >= b
		allPaid, err := service.GetCheapestPaidModels(ctx, 10)
		require.NoError(t, err)
		require.Len(t, allPaid, 2)
		assert.Equal(t, "paid-cheap", allPaid[0].ID)
		assert.Equal(t, "paid-expensive", allPaid[1].ID)

		// limit smaller than candidates exercises other minInt branch
		cheapestOnly, err := service.GetCheapestPaidModels(ctx, 1)
		require.NoError(t, err)
		require.Len(t, cheapestOnly, 1)
		assert.Equal(t, "paid-cheap", cheapestOnly[0].ID)
	})
}

func TestParsePrice_EmptyAndInvalid(t *testing.T) {
	t.Parallel()

	require.Equal(t, 0.0, parsePrice(""))
	require.Equal(t, 0.0, parsePrice("not-a-number"))
}

func TestParsePrice_CurrencyAndSuffix(t *testing.T) {
	t.Parallel()

	// Should handle currency prefix and suffix like "/1k tokens".
	got := parsePrice("$0.0123/1k tokens")
	require.InDelta(t, 0.0123, got, 1e-6)
}

func TestEffectivePrice_RequestPreferred(t *testing.T) {
	t.Parallel()

	p := Pricing{
		Request:    "0.002",
		Prompt:     "0.01",
		Completion: "0.02",
	}

	require.InDelta(t, 0.002, effectivePrice(p), 1e-9)
}

func TestEffectivePrice_FallbackPromptAndCompletion(t *testing.T) {
	t.Parallel()

	p := Pricing{
		Request:    "",
		Prompt:     "0.01",
		Completion: "0.02",
	}

	require.InDelta(t, 0.03, effectivePrice(p), 1e-9)
}

func TestCapacityScore_PerRequestAndContext(t *testing.T) {
	t.Parallel()

	// With explicit per-request limits
	m := Model{
		PerRequestLimits: &PerRequestLimits{PromptTokens: 1000, CompletionTokens: 2000},
		ContextLength:    4096,
	}
	require.InDelta(t, 3000.0, capacityScore(m), 1e-6)

	// With no per-request limits falls back to context length
	m2 := Model{
		PerRequestLimits: nil,
		ContextLength:    8192,
	}
	require.InDelta(t, 8192.0, capacityScore(m2), 1e-6)
}

func TestGetFreeModels_CacheUsedOnAPIFailure(t *testing.T) {
	t.Parallel()

	callCount := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		callCount++
		if callCount == 1 {
			// First call succeeds
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(map[string]any{
				"data": []map[string]any{
					{
						"id":   "cached-model",
						"name": "Cached Model",
						"pricing": map[string]string{
							"prompt":     "0",
							"completion": "0",
							"request":    "0",
							"image":      "0",
						},
					},
				},
			})
		} else {
			// Subsequent calls fail
			w.WriteHeader(http.StatusInternalServerError)
		}
	}))
	defer server.Close()

	svc := NewService("test-key", server.URL, 1*time.Millisecond)
	ctx := context.Background()

	// First call - should succeed and cache
	models, err := svc.GetFreeModels(ctx)
	require.NoError(t, err)
	require.Len(t, models, 1)

	// Wait for cache to expire
	time.Sleep(5 * time.Millisecond)

	// Second call - API fails but should return cached models
	models, err = svc.GetFreeModels(ctx)
	require.NoError(t, err)
	require.Len(t, models, 1)
	require.Equal(t, "cached-model", models[0].ID)
}

func TestIsFreeModel_ExcludedPatterns(t *testing.T) {
	t.Parallel()

	svc := &Service{}

	tests := []struct {
		name     string
		modelID  string
		pricing  Pricing
		expected bool
	}{
		{
			name:     "openrouter_auto_excluded",
			modelID:  "openrouter/auto",
			pricing:  Pricing{Prompt: "0", Completion: "0", Request: "0", Image: "0"},
			expected: false,
		},
		{
			name:     "openrouter_prefix_excluded",
			modelID:  "openrouter/gpt-4-turbo",
			pricing:  Pricing{Prompt: "0", Completion: "0", Request: "0", Image: "0"},
			expected: false,
		},
		{
			name:     "free_model_included",
			modelID:  "meta-llama/llama-3.1-8b-instruct:free",
			pricing:  Pricing{Prompt: "0", Completion: "0", Request: "0", Image: "0"},
			expected: true,
		},
		{
			name:     "paid_model_excluded",
			modelID:  "openai/gpt-4",
			pricing:  Pricing{Prompt: "0.01", Completion: "0.02", Request: "0", Image: "0"},
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			model := Model{ID: tt.modelID, Pricing: tt.pricing}
			result := svc.isFreeModel(model)
			require.Equal(t, tt.expected, result)
		})
	}
}
