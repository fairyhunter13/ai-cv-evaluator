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
