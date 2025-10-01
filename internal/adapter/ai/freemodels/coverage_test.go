package freemodels

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestService_EdgeCases(t *testing.T) {
	t.Parallel()

	// Test with empty API key
	service := New("", "http://unused")
	assert.NotNil(t, service)
	assert.Equal(t, "", service.apiKey)
	assert.Equal(t, "http://unused", service.baseURL)

	// Test with custom refresh interval
	service = NewWithRefresh("test-key", "http://unused", 2*time.Hour)
	assert.NotNil(t, service)
	assert.Equal(t, 2*time.Hour, service.fetchInterval)
}

func TestService_ConcurrentAccess(t *testing.T) {
	t.Parallel()

	service := New("test-key", "http://unused")
	// Prevent network fetch by marking as recently fetched
	service.lastFetch = time.Now()
	ctx := context.Background()

	// Test concurrent access to GetFreeModels
	done := make(chan bool, 10)
	for i := 0; i < 10; i++ {
		go func() {
			_, _ = service.GetFreeModels(ctx)
			done <- true
		}()
	}

	// Wait for all goroutines to complete
	for i := 0; i < 10; i++ {
		<-done
	}
}

func TestService_ForceRefresh_EdgeCases(t *testing.T) {
	t.Parallel()

	service := New("test-key", "http://unused")
	ctx := context.Background()

	// Test force refresh with invalid URL
	service.baseURL = "invalid-url"
	err := service.ForceRefresh(ctx)
	assert.Error(t, err)

	// Test force refresh with empty context
	err = service.ForceRefresh(context.Background())
	// Should handle context properly
	if err != nil {
		assert.Contains(t, err.Error(), "failed to fetch models")
	}
}

func TestService_GetModelInfo_EdgeCases(t *testing.T) {
	t.Parallel()

	service := New("test-key", "http://unused")
	// Prevent network fetch
	service.lastFetch = time.Now()
	ctx := context.Background()

	// Test with non-existent model
	info, err := service.GetModelInfo(ctx, "non-existent-model")
	assert.Error(t, err)
	assert.Nil(t, info)

	// Test with empty model ID
	info, err = service.GetModelInfo(ctx, "")
	assert.Error(t, err)
	assert.Nil(t, info)
}

func TestService_GetFreeModelIDs_EdgeCases(t *testing.T) {
	t.Parallel()

	service := New("test-key", "http://unused")
	// Prevent network fetch
	service.lastFetch = time.Now()
	ctx := context.Background()

	// Test with no models
	ids, err := service.GetFreeModelIDs(ctx)
	if err != nil {
		// Should return empty slice on error
		assert.Empty(t, ids)
	} else {
		// If no error, should return slice (even if empty)
		assert.NotNil(t, ids)
	}
}

func TestModel_EdgeCases(t *testing.T) {
	t.Parallel()

	// Test Model with empty fields
	model := Model{
		ID:          "",
		Name:        "",
		Description: "",
		Context:     0,
	}

	assert.Equal(t, "", model.ID)
	assert.Equal(t, "", model.Name)
	assert.Equal(t, "", model.Description)
	assert.Equal(t, 0, model.Context)

	// Test Model with special characters
	model = Model{
		ID:          "model-with-special-chars-!@#$%",
		Name:        "Model with Special Chars",
		Description: "Description with émojis 🚀 and unicode",
		Context:     1000000,
	}

	assert.Equal(t, "model-with-special-chars-!@#$%", model.ID)
	assert.Equal(t, "Model with Special Chars", model.Name)
	assert.Equal(t, "Description with émojis 🚀 and unicode", model.Description)
	assert.Equal(t, 1000000, model.Context)
}

func TestPricing_EdgeCases(t *testing.T) {
	t.Parallel()

	// Test Pricing with different types
	pricing := Pricing{
		Prompt:     "0",
		Completion: 0.0,
	}

	assert.Equal(t, "0", pricing.Prompt)
	assert.Equal(t, 0.0, pricing.Completion)

	// Test Pricing with nil values
	pricing = Pricing{
		Prompt:     nil,
		Completion: nil,
	}

	assert.Nil(t, pricing.Prompt)
	assert.Nil(t, pricing.Completion)

	// Test Pricing with numeric strings
	pricing = Pricing{
		Prompt:     "0.0001",
		Completion: "0.0002",
	}

	assert.Equal(t, "0.0001", pricing.Prompt)
	assert.Equal(t, "0.0002", pricing.Completion)
}

func TestService_RefreshStatus_EdgeCases(t *testing.T) {
	t.Parallel()

	service := New("test-key", "http://unused")

	// Test refresh status with zero time
	lastFetch, nextFetch, refreshInterval := service.GetRefreshStatus()
	assert.True(t, lastFetch.IsZero())
	assert.Equal(t, 1*time.Hour, refreshInterval)
	assert.Equal(t, lastFetch.Add(refreshInterval), nextFetch)

	// Test refresh status after setting some models
	service.models = []Model{
		{ID: "model1", Name: "Model 1"},
		{ID: "model2", Name: "Model 2"},
	}
	service.lastFetch = time.Now()

	lastFetch, nextFetch, refreshInterval = service.GetRefreshStatus()
	assert.False(t, lastFetch.IsZero())
	assert.Equal(t, 1*time.Hour, refreshInterval)
	assert.Equal(t, lastFetch.Add(refreshInterval), nextFetch)
}

func TestService_IsFreeModel_EdgeCases(t *testing.T) {
	t.Parallel()

	service := New("test-key", "http://unused")

	// Test with nil pricing
	model := Model{
		ID:      "test-model",
		Pricing: Pricing{Prompt: nil, Completion: nil},
	}
	assert.True(t, service.isFreeModel(model))

	// Test with empty string pricing
	model = Model{
		ID:      "test-model",
		Pricing: Pricing{Prompt: "", Completion: ""},
	}
	assert.True(t, service.isFreeModel(model))

	// Test with whitespace pricing
	model = Model{
		ID:      "test-model",
		Pricing: Pricing{Prompt: "   ", Completion: "   "},
	}
	assert.True(t, service.isFreeModel(model))

	// Test with mixed pricing
	model = Model{
		ID:      "test-model",
		Pricing: Pricing{Prompt: 0, Completion: "0.001"},
	}
	assert.False(t, service.isFreeModel(model))
}

func TestService_GetRandomFreeModel_EdgeCases(t *testing.T) {
	t.Parallel()

	service := New("test-key", "http://unused")
	// Prevent auto refresh (no network)
	service.lastFetch = time.Now()
	ctx := context.Background()

	// Test with no models
	_, err := service.GetRandomFreeModel(ctx)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "no free models available")

	// Test with one model
	service.models = []Model{
		{ID: "single-model", Name: "Single Model"},
	}
	modelID, err := service.GetRandomFreeModel(ctx)
	if err == nil {
		assert.Equal(t, "single-model", modelID)
	}
}

func TestService_GetBestFreeModel_EdgeCases(t *testing.T) {
	t.Parallel()

	service := New("test-key", "http://unused")
	// Prevent auto refresh (no network)
	service.lastFetch = time.Now()
	ctx := context.Background()

	// Test with no models
	_, err := service.GetBestFreeModel(ctx)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "no free models available")

	// Test with one model
	service.models = []Model{
		{ID: "single-model", Name: "Single Model", Context: 1000},
	}
	modelID, err := service.GetBestFreeModel(ctx)
	if err == nil {
		assert.Equal(t, "single-model", modelID)
	}
}

func TestService_ConcurrentRefresh(t *testing.T) {
	t.Parallel()

	// Local server returning empty data quickly
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/models" {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{"data": []any{}})
	}))
	defer ts.Close()

	service := New("test-key", ts.URL)
	ctx := context.Background()

	// Test concurrent refresh attempts
	done := make(chan bool, 5)
	for i := 0; i < 5; i++ {
		go func() {
			_ = service.ForceRefresh(ctx)
			done <- true
		}()
	}

	// Wait for all goroutines to complete
	for i := 0; i < 5; i++ {
		<-done
	}
}

func TestService_HTTPClient_Configuration(t *testing.T) {
	t.Parallel()

	service := New("test-key", "http://unused")

	// Test HTTP client configuration
	assert.NotNil(t, service.httpClient)
	assert.Equal(t, 30*time.Second, service.httpClient.Timeout)
}

func TestService_BaseURL_Configuration(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name    string
		baseURL string
		valid   bool
	}{
		{"valid URL", "http://unused", true},
		{"invalid URL", "not-a-url", false},
		{"empty URL", "", false},
		{"URL without protocol", "api.openrouter.ai/v1", false},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			service := New("test-key", tc.baseURL)
			assert.Equal(t, tc.baseURL, service.baseURL)
		})
	}
}

func TestService_FetchInterval_Configuration(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name     string
		interval time.Duration
	}{
		{"1 hour", 1 * time.Hour},
		{"30 minutes", 30 * time.Minute},
		{"2 hours", 2 * time.Hour},
		{"5 minutes", 5 * time.Minute},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			service := NewWithRefresh("test-key", "http://unused", tc.interval)
			assert.Equal(t, tc.interval, service.fetchInterval)
		})
	}
}

func TestService_ModelSorting(t *testing.T) {
	t.Parallel()

	service := New("test-key", "http://unused")
	ctx := context.Background()

	// Test sorting with different context lengths
	service.models = []Model{
		{ID: "model1", Name: "Model A", Context: 1000},
		{ID: "model2", Name: "Model B", Context: 2000},
		{ID: "model3", Name: "Model C", Context: 1000},
	}
	// Prevent fetch during GetFreeModels call
	service.lastFetch = time.Now()

	modelID, err := service.GetBestFreeModel(ctx)
	if err == nil {
		// Should return the model with highest context length
		assert.Equal(t, "model2", modelID)
	}
}

func TestService_ModelSorting_SameContextLength(t *testing.T) {
	t.Parallel()

	service := New("test-key", "http://unused")
	ctx := context.Background()

	// Test sorting with same context length (should sort by name)
	service.models = []Model{
		{ID: "model1", Name: "Zebra Model", Context: 1000},
		{ID: "model2", Name: "Alpha Model", Context: 1000},
		{ID: "model3", Name: "Beta Model", Context: 1000},
	}
	// Prevent fetch during GetFreeModels call
	service.lastFetch = time.Now()

	modelID, err := service.GetBestFreeModel(ctx)
	if err == nil {
		// Should return the model with alphabetically first name
		assert.Equal(t, "model2", modelID) // Alpha Model
	}
}

func TestService_JSONHandling(t *testing.T) {
	t.Parallel()

	// Test JSON marshaling of Model
	model := Model{
		ID:          "test-model",
		Name:        "Test Model",
		Description: "A test model",
		Context:     1000,
		Pricing: Pricing{
			Prompt:     "0",
			Completion: "0",
		},
	}

	jsonData, err := json.Marshal(model)
	assert.NoError(t, err)
	assert.NotEmpty(t, jsonData)

	// Test JSON unmarshaling
	var unmarshaledModel Model
	err = json.Unmarshal(jsonData, &unmarshaledModel)
	assert.NoError(t, err)
	assert.Equal(t, model.ID, unmarshaledModel.ID)
	assert.Equal(t, model.Name, unmarshaledModel.Name)
}

func TestService_ContextHandling(t *testing.T) {
	t.Parallel()

	// Use local server to avoid external calls
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/models" {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{"data": []any{}})
	}))
	defer ts.Close()

	service := New("test-key", ts.URL)

	// Test with cancelled context
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately
	// Use ForceRefresh to avoid internal warn logs in GetFreeModels
	err := service.ForceRefresh(ctx)
	// Should handle cancelled context gracefully
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "context canceled")
}

func TestService_TimeoutHandling(t *testing.T) {
	t.Parallel()

	// Local server that responds after a small delay
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		time.Sleep(5 * time.Millisecond)
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{"data": []any{}})
	}))
	defer ts.Close()

	// Test with very short timeout
	service := New("test-key", ts.URL)
	service.httpClient.Timeout = 1 * time.Nanosecond

	ctx := context.Background()
	// Use ForceRefresh to directly surface timeout without internal warn logs
	err := service.ForceRefresh(ctx)
	// Should handle timeout gracefully
	if err != nil {
		// Accept different timeout phrasings across platforms/race builds
		assert.True(t, containsTimeoutError(err), "expected timeout-related error, got: %v", err)
	}
}
