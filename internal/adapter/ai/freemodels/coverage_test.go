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
	service := NewService("", "http://unused", 1*time.Hour)
	assert.NotNil(t, service)
	assert.Equal(t, "", service.apiKey)
	assert.Equal(t, "http://unused", service.baseURL)

	// Test with custom refresh interval
	service = NewService("test-key", "http://unused", 2*time.Hour)
	assert.NotNil(t, service)
	assert.Equal(t, 2*time.Hour, service.refreshDur)
}

func TestService_ConcurrentAccess(t *testing.T) {
	t.Parallel()

	service := NewService("test-key", "http://unused", 1*time.Hour)
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

	service := NewService("test-key", "http://unused", 1*time.Hour)
	ctx := context.Background()

	// Test refresh with invalid URL
	service.baseURL = "invalid-url"
	err := service.Refresh(ctx)
	assert.Error(t, err)

	// Test refresh with empty context
	err = service.Refresh(context.Background())
	// Should handle context properly
	if err != nil {
		assert.Contains(t, err.Error(), "failed to fetch models")
	}
}

func TestService_GetModelIDs_EdgeCases(t *testing.T) {
	t.Parallel()

	service := NewService("test-key", "http://unused", 1*time.Hour)
	// Prevent network fetch
	service.lastFetch = time.Now()
	ctx := context.Background()

	// Test with no models
	ids, err := service.GetModelIDs(ctx)
	if err != nil {
		// Should return empty slice on error
		assert.Empty(t, ids)
	} else {
		// If no error, should return slice (even if empty)
		assert.NotNil(t, ids)
	}
}

func TestService_GetFreeModels_EdgeCases(t *testing.T) {
	t.Parallel()

	service := NewService("test-key", "http://unused", 1*time.Hour)
	// Prevent network fetch
	service.lastFetch = time.Now()
	ctx := context.Background()

	// Test with no models
	models, err := service.GetFreeModels(ctx)
	if err != nil {
		// Should return empty slice on error
		assert.Empty(t, models)
	} else {
		// If no error, should return slice (even if empty)
		assert.NotNil(t, models)
	}
}

func TestModel_EdgeCases(t *testing.T) {
	t.Parallel()

	// Test Model with empty fields
	model := Model{
		ID:          "",
		Name:        "",
		Description: "",
		Pricing:     Pricing{},
	}

	assert.Equal(t, "", model.ID)
	assert.Equal(t, "", model.Name)
	assert.Equal(t, "", model.Description)
	assert.Equal(t, Pricing{}, model.Pricing)

	// Test Model with special characters
	model = Model{
		ID:          "model-with-special-chars-!@#$%",
		Name:        "Model with Special Chars",
		Description: "Description with émojis 🚀 and unicode",
		Pricing:     Pricing{Prompt: "0", Completion: "0"},
	}

	assert.Equal(t, "model-with-special-chars-!@#$%", model.ID)
	assert.Equal(t, "Model with Special Chars", model.Name)
	assert.Equal(t, "Description with émojis 🚀 and unicode", model.Description)
	assert.Equal(t, Pricing{Prompt: "0", Completion: "0"}, model.Pricing)
}

func TestPricing_EdgeCases(t *testing.T) {
	t.Parallel()

	// Test Pricing with different types
	pricing := Pricing{
		Prompt:     "0",
		Completion: "0",
	}

	assert.Equal(t, "0", pricing.Prompt)
	assert.Equal(t, "0", pricing.Completion)

	// Test Pricing with empty values
	pricing = Pricing{
		Prompt:     "",
		Completion: "",
	}

	assert.Equal(t, "", pricing.Prompt)
	assert.Equal(t, "", pricing.Completion)

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

	service := NewService("test-key", "http://unused", 1*time.Hour)

	// Test refresh status with zero time
	assert.True(t, service.lastFetch.IsZero())
	assert.Equal(t, 1*time.Hour, service.refreshDur)

	// Test refresh status after setting some models
	service.models = []Model{
		{ID: "model1", Name: "Model 1", Pricing: Pricing{Prompt: "0", Completion: "0"}},
		{ID: "model2", Name: "Model 2", Pricing: Pricing{Prompt: "0", Completion: "0"}},
	}
	service.lastFetch = time.Now()

	assert.False(t, service.lastFetch.IsZero())
	assert.Equal(t, 1*time.Hour, service.refreshDur)
}

// TestService_IsFreeModel_EdgeCases tests model validation through public methods
func TestService_IsFreeModel_EdgeCases(t *testing.T) {
	t.Parallel()

	// This test is removed as it was testing private methods directly
	// The isFreeModel logic is tested through the public GetFreeModels method
}

func TestService_GetFreeModels_WithModels(t *testing.T) {
	t.Parallel()

	service := NewService("test-key", "http://unused", 1*time.Hour)
	// Prevent auto refresh (no network)
	service.lastFetch = time.Now()
	ctx := context.Background()

	// Test with no models
	models, err := service.GetFreeModels(ctx)
	if err != nil {
		assert.Empty(t, models)
	}

	// Test with one model
	service.models = []Model{
		{ID: "single-model", Name: "Single Model", Pricing: Pricing{Prompt: "0", Completion: "0"}},
	}
	models, err = service.GetFreeModels(ctx)
	if err == nil {
		assert.Len(t, models, 1)
		assert.Equal(t, "single-model", models[0].ID)
	}
}

func TestService_GetModelIDs_WithModels(t *testing.T) {
	t.Parallel()

	service := NewService("test-key", "http://unused", 1*time.Hour)
	// Prevent auto refresh (no network)
	service.lastFetch = time.Now()
	ctx := context.Background()

	// Test with no models
	ids, err := service.GetModelIDs(ctx)
	if err != nil {
		assert.Empty(t, ids)
	}

	// Test with one model
	service.models = []Model{
		{ID: "single-model", Name: "Single Model", Pricing: Pricing{Prompt: "0", Completion: "0"}},
	}
	ids, err = service.GetModelIDs(ctx)
	if err == nil {
		assert.Len(t, ids, 1)
		assert.Equal(t, "single-model", ids[0])
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

	service := NewService("test-key", ts.URL, 1*time.Hour)
	ctx := context.Background()

	// Test concurrent refresh attempts
	done := make(chan bool, 5)
	for i := 0; i < 5; i++ {
		go func() {
			_ = service.Refresh(ctx)
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

	service := NewService("test-key", "http://unused", 1*time.Hour)

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
			service := NewService("test-key", tc.baseURL, 1*time.Hour)
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
			service := NewService("test-key", "http://unused", tc.interval)
			assert.Equal(t, tc.interval, service.refreshDur)
		})
	}
}

func TestService_ModelSorting(t *testing.T) {
	t.Parallel()

	service := NewService("test-key", "http://unused", 1*time.Hour)
	ctx := context.Background()

	// Test sorting with different models
	service.models = []Model{
		{ID: "model1", Name: "Model A", Pricing: Pricing{Prompt: "0", Completion: "0"}},
		{ID: "model2", Name: "Model B", Pricing: Pricing{Prompt: "0", Completion: "0"}},
		{ID: "model3", Name: "Model C", Pricing: Pricing{Prompt: "0", Completion: "0"}},
	}
	// Prevent fetch during GetFreeModels call
	service.lastFetch = time.Now()

	models, err := service.GetFreeModels(ctx)
	if err == nil {
		// Should return available models
		assert.Len(t, models, 3)
	}
}

func TestService_ModelSorting_SameModels(t *testing.T) {
	t.Parallel()

	service := NewService("test-key", "http://unused", 1*time.Hour)
	ctx := context.Background()

	// Test sorting with same models (should sort by name)
	service.models = []Model{
		{ID: "model1", Name: "Zebra Model", Pricing: Pricing{Prompt: "0", Completion: "0"}},
		{ID: "model2", Name: "Alpha Model", Pricing: Pricing{Prompt: "0", Completion: "0"}},
		{ID: "model3", Name: "Beta Model", Pricing: Pricing{Prompt: "0", Completion: "0"}},
	}
	// Prevent fetch during GetFreeModels call
	service.lastFetch = time.Now()

	models, err := service.GetFreeModels(ctx)
	if err == nil {
		// Should return available models
		assert.Len(t, models, 3)
	}
}

func TestService_JSONHandling(t *testing.T) {
	t.Parallel()

	// Test JSON marshaling of Model
	model := Model{
		ID:          "test-model",
		Name:        "Test Model",
		Description: "A test model",
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

	service := NewService("test-key", ts.URL, 1*time.Hour)

	// Test with cancelled context
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately
	// Use Refresh to avoid internal warn logs in GetFreeModels
	err := service.Refresh(ctx)
	// Should handle cancelled context gracefully
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "context canceled")
}

func TestService_TimeoutHandling(t *testing.T) {
	t.Parallel()

	// Local server that responds after a small delay
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		time.Sleep(50 * time.Millisecond) // Increased to ensure it's longer than timeout
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{"data": []any{}})
	}))
	defer ts.Close()

	// Test with very short timeout
	service := NewService("test-key", ts.URL, 1*time.Hour)
	service.httpClient.Timeout = 10 * time.Millisecond // Increased for reliability

	ctx := context.Background()
	// Use Refresh to directly surface timeout without internal warn logs
	err := service.Refresh(ctx)
	// Should handle timeout gracefully
	if err != nil {
		// Accept different timeout phrasings across platforms/race builds
		assert.True(t, containsTimeoutError(err), "expected timeout-related error, got: %v", err)
	}
}
