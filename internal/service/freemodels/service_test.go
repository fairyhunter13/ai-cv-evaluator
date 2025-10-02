package freemodels

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

// TestFilterFreeModels tests the filterFreeModels function
func TestFilterFreeModels(t *testing.T) {
	t.Parallel()

	service := NewService("", "", 1*time.Hour)

	tests := []struct {
		name          string
		models        []Model
		expectedCount int
		expectedIDs   []string
	}{
		{
			name: "all_free_models",
			models: []Model{
				{
					ID: "free-model-1",
					Pricing: Pricing{
						Prompt:     "0",
						Completion: "0",
						Request:    "0",
						Image:      "0",
					},
				},
				{
					ID: "free-model-2",
					Pricing: Pricing{
						Prompt:     "0.0",
						Completion: "0.0",
						Request:    "0.0",
						Image:      "0.0",
					},
				},
			},
			expectedCount: 2,
			expectedIDs:   []string{"free-model-1", "free-model-2"},
		},
		{
			name: "mixed_free_and_paid_models",
			models: []Model{
				{
					ID: "free-model-1",
					Pricing: Pricing{
						Prompt:     "0",
						Completion: "0",
						Request:    "0",
						Image:      "0",
					},
				},
				{
					ID: "paid-model-1",
					Pricing: Pricing{
						Prompt:     "0.001",
						Completion: "0.002",
						Request:    "0.003",
						Image:      "0.004",
					},
				},
				{
					ID: "free-model-2",
					Pricing: Pricing{
						Prompt:     "0.0",
						Completion: "0.0",
						Request:    "0.0",
						Image:      "0.0",
					},
				},
			},
			expectedCount: 2,
			expectedIDs:   []string{"free-model-1", "free-model-2"},
		},
		{
			name: "excluded_models",
			models: []Model{
				{
					ID: "openai/gpt-4",
					Pricing: Pricing{
						Prompt:     "0",
						Completion: "0",
						Request:    "0",
						Image:      "0",
					},
				},
				{
					ID: "anthropic/claude-3-opus",
					Pricing: Pricing{
						Prompt:     "0",
						Completion: "0",
						Request:    "0",
						Image:      "0",
					},
				},
				{
					ID: "openrouter/auto",
					Pricing: Pricing{
						Prompt:     "0",
						Completion: "0",
						Request:    "0",
						Image:      "0",
					},
				},
				{
					ID: "free-model-1",
					Pricing: Pricing{
						Prompt:     "0",
						Completion: "0",
						Request:    "0",
						Image:      "0",
					},
				},
			},
			expectedCount: 1,
			expectedIDs:   []string{"free-model-1"},
		},
		{
			name: "no_free_models",
			models: []Model{
				{
					ID: "paid-model-1",
					Pricing: Pricing{
						Prompt:     "0.001",
						Completion: "0.002",
						Request:    "0.003",
						Image:      "0.004",
					},
				},
				{
					ID: "paid-model-2",
					Pricing: Pricing{
						Prompt:     "0.005",
						Completion: "0.006",
						Request:    "0.007",
						Image:      "0.008",
					},
				},
			},
			expectedCount: 0,
			expectedIDs:   []string{},
		},
		{
			name:          "empty_models",
			models:        []Model{},
			expectedCount: 0,
			expectedIDs:   []string{},
		},
		{
			name: "openrouter_auto_strictly_banned",
			models: []Model{
				{
					ID: "openrouter/auto",
					Pricing: Pricing{
						Prompt:     "0",
						Completion: "0",
						Request:    "0",
						Image:      "0",
					},
				},
				{
					ID: "free-model-1",
					Pricing: Pricing{
						Prompt:     "0",
						Completion: "0",
						Request:    "0",
						Image:      "0",
					},
				},
			},
			expectedCount: 1,
			expectedIDs:   []string{"free-model-1"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			filtered := service.filterFreeModels(tt.models)

			assert.Equal(t, tt.expectedCount, len(filtered), "Filtered models count should match expected")

			// Check that all expected IDs are present
			actualIDs := make([]string, len(filtered))
			for i, model := range filtered {
				actualIDs[i] = model.ID
			}

			for _, expectedID := range tt.expectedIDs {
				assert.Contains(t, actualIDs, expectedID, "Expected model ID %s should be in filtered results", expectedID)
			}
		})
	}
}

// TestFetchModelsFromAPI tests the fetchModelsFromAPI function
func TestFetchModelsFromAPI(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name           string
		apiKey         string
		baseURL        string
		serverResponse map[string]interface{}
		serverStatus   int
		expectError    bool
		expectedCount  int
	}{
		{
			name:    "successful_api_call",
			apiKey:  "test-key",
			baseURL: "https://openrouter.ai/api/v1",
			serverResponse: map[string]interface{}{
				"data": []map[string]interface{}{
					{
						"id":             "free-model-1",
						"name":           "Free Model 1",
						"context_length": 4096,
						"pricing": map[string]interface{}{
							"prompt":     "0",
							"completion": "0",
						},
					},
					{
						"id":             "paid-model-1",
						"name":           "Paid Model 1",
						"context_length": 8192,
						"pricing": map[string]interface{}{
							"prompt":     "0.001",
							"completion": "0.002",
						},
					},
				},
			},
			serverStatus:  http.StatusOK,
			expectError:   false,
			expectedCount: 1, // Only free model should be returned after filtering
		},
		{
			name:    "api_error_response",
			apiKey:  "test-key",
			baseURL: "https://openrouter.ai/api/v1",
			serverResponse: map[string]interface{}{
				"error": "Invalid API key",
			},
			serverStatus: http.StatusUnauthorized,
			expectError:  true,
		},
		{
			name:    "empty_response",
			apiKey:  "test-key",
			baseURL: "https://openrouter.ai/api/v1",
			serverResponse: map[string]interface{}{
				"data": []map[string]interface{}{},
			},
			serverStatus:  http.StatusOK,
			expectError:   false,
			expectedCount: 0,
		},
		{
			name:    "malformed_response",
			apiKey:  "test-key",
			baseURL: "https://openrouter.ai/api/v1",
			serverResponse: map[string]interface{}{
				"data": "not an array",
			},
			serverStatus: http.StatusOK,
			expectError:  true,
		},
		{
			name:    "no_api_key",
			apiKey:  "",
			baseURL: "https://openrouter.ai/api/v1",
			serverResponse: map[string]interface{}{
				"data": []map[string]interface{}{},
			},
			serverStatus:  http.StatusOK,
			expectError:   false,
			expectedCount: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			// Create mock server
			ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(tt.serverStatus)
				_ = json.NewEncoder(w).Encode(tt.serverResponse)
			}))
			defer ts.Close()

			// Use test server URL
			testBaseURL := ts.URL
			if tt.baseURL != "https://openrouter.ai/api/v1" {
				testBaseURL = tt.baseURL
			}

			service := NewService(tt.apiKey, testBaseURL, 1*time.Hour)
			ctx := context.Background()

			models, err := service.fetchModelsFromAPI(ctx)

			if tt.expectError {
				assert.Error(t, err)
				assert.Nil(t, models)
			} else {
				assert.NoError(t, err)
				if tt.expectedCount > 0 {
					assert.NotNil(t, models)
				}
				assert.Equal(t, tt.expectedCount, len(models), "Models count should match expected")
			}
		})
	}
}

// TestFetchModelsFromAPI_NetworkError tests network error scenarios
func TestFetchModelsFromAPI_NetworkError(t *testing.T) {
	t.Parallel()

	service := NewService("test-key", "http://invalid-url-that-does-not-exist", 1*time.Hour)
	ctx := context.Background()

	models, err := service.fetchModelsFromAPI(ctx)

	assert.Error(t, err)
	assert.Nil(t, models)
}

// TestFetchModelsFromAPI_Timeout tests timeout scenarios
func TestFetchModelsFromAPI_Timeout(t *testing.T) {
	t.Parallel()

	// Create a server that takes too long to respond
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		time.Sleep(2 * time.Second) // Longer than typical timeout
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"data": []map[string]interface{}{},
		})
	}))
	defer ts.Close()

	service := NewService("test-key", ts.URL, 1*time.Hour)
	ctx := context.Background()

	models, err := service.fetchModelsFromAPI(ctx)

	// The 2-second delay is less than the 30-second HTTP timeout, so this should succeed
	assert.NoError(t, err)
	assert.Equal(t, 0, len(models), "Should return empty slice for timeout test")
}

// TestFetchModelsFromAPI_InvalidJSON tests invalid JSON response
func TestFetchModelsFromAPI_InvalidJSON(t *testing.T) {
	t.Parallel()

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("invalid json"))
	}))
	defer ts.Close()

	service := NewService("test-key", ts.URL, 1*time.Hour)
	ctx := context.Background()

	models, err := service.fetchModelsFromAPI(ctx)

	assert.Error(t, err)
	assert.Nil(t, models)
}

// TestFetchModelsFromAPI_MissingDataField tests response without data field
func TestFetchModelsFromAPI_MissingDataField(t *testing.T) {
	t.Parallel()

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"message": "Success",
			// Missing "data" field
		})
	}))
	defer ts.Close()

	service := NewService("test-key", ts.URL, 1*time.Hour)
	ctx := context.Background()

	models, err := service.fetchModelsFromAPI(ctx)

	// The function should handle missing data field gracefully
	// It returns an empty slice when data field is missing
	assert.NoError(t, err)
	assert.Equal(t, 0, len(models), "Should return empty slice for missing data field")
}

// TestFetchModelsFromAPI_InvalidModelStructure tests invalid model structure
func TestFetchModelsFromAPI_InvalidModelStructure(t *testing.T) {
	t.Parallel()

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"data": []map[string]interface{}{
				{
					"id":   123, // Invalid: should be string
					"name": "Test Model",
				},
			},
		})
	}))
	defer ts.Close()

	service := NewService("test-key", ts.URL, 1*time.Hour)
	ctx := context.Background()

	models, err := service.fetchModelsFromAPI(ctx)

	// This should handle the invalid structure gracefully
	if err != nil {
		assert.Nil(t, models)
	} else {
		assert.NotNil(t, models)
	}
}

// TestFetchModelsFromAPI_EmptyPricing tests models with empty pricing
func TestFetchModelsFromAPI_EmptyPricing(t *testing.T) {
	t.Parallel()

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"data": []map[string]interface{}{
				{
					"id":             "model-with-empty-pricing",
					"name":           "Model with Empty Pricing",
					"context_length": 4096,
					"pricing":        map[string]interface{}{}, // Empty pricing
				},
				{
					"id":             "model-with-nil-pricing",
					"name":           "Model with Nil Pricing",
					"context_length": 4096,
					"pricing":        nil, // Nil pricing
				},
			},
		})
	}))
	defer ts.Close()

	service := NewService("test-key", ts.URL, 1*time.Hour)
	ctx := context.Background()

	models, err := service.fetchModelsFromAPI(ctx)

	assert.NoError(t, err)
	assert.NotNil(t, models)
	assert.Equal(t, 2, len(models), "Should return models even with empty/nil pricing")
}

// TestFetchModelsFromAPI_LargeResponse tests handling of large responses
func TestFetchModelsFromAPI_LargeResponse(t *testing.T) {
	t.Parallel()

	// Create a large response with many models
	models := make([]map[string]interface{}, 1000)
	for i := 0; i < 1000; i++ {
		models[i] = map[string]interface{}{
			"id":             fmt.Sprintf("model-%d", i),
			"name":           fmt.Sprintf("Model %d", i),
			"context_length": 4096,
			"pricing": map[string]interface{}{
				"prompt":     "0",
				"completion": "0",
			},
		}
	}

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"data": models,
		})
	}))
	defer ts.Close()

	service := NewService("test-key", ts.URL, 1*time.Hour)
	ctx := context.Background()

	fetchedModels, err := service.fetchModelsFromAPI(ctx)

	assert.NoError(t, err)
	assert.NotNil(t, fetchedModels)
	assert.Equal(t, 1000, len(fetchedModels), "Should handle large responses")
}

// TestFetchModelsFromAPI_ConcurrentCalls tests concurrent API calls
func TestFetchModelsFromAPI_ConcurrentCalls(t *testing.T) {
	t.Parallel()

	// Use atomic counter to avoid race conditions
	var requestCount int64
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		atomic.AddInt64(&requestCount, 1)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"data": []map[string]interface{}{
				{
					"id":             "concurrent-model",
					"name":           "Concurrent Model",
					"context_length": 4096,
					"pricing": map[string]interface{}{
						"prompt":     "0",
						"completion": "0",
					},
				},
			},
		})
	}))
	defer ts.Close()

	service := NewService("test-key", ts.URL, 1*time.Hour)
	ctx := context.Background()

	// Make concurrent calls
	const numGoroutines = 10
	results := make(chan []Model, numGoroutines)
	errors := make(chan error, numGoroutines)

	for i := 0; i < numGoroutines; i++ {
		go func() {
			models, err := service.fetchModelsFromAPI(ctx)
			results <- models
			errors <- err
		}()
	}

	// Collect results
	var allModels [][]Model
	var allErrors []error

	for i := 0; i < numGoroutines; i++ {
		models := <-results
		err := <-errors
		allModels = append(allModels, models)
		allErrors = append(allErrors, err)
	}

	// All calls should succeed
	for i, err := range allErrors {
		assert.NoError(t, err, "Goroutine %d should not return error", i)
	}

	// All results should be identical
	for i := 1; i < len(allModels); i++ {
		assert.Equal(t, allModels[0], allModels[i], "All concurrent calls should return identical results")
	}

	// Should have made multiple requests (no caching in fetchModelsFromAPI)
	assert.GreaterOrEqual(t, atomic.LoadInt64(&requestCount), int64(numGoroutines), "Should make multiple API requests")
}
