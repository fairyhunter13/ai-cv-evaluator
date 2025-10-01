// Package freemodels tests the free models service.
package freemodels

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestService_GetFreeModels(t *testing.T) {
	// Serve an empty list of models to avoid external HTTP calls
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

	// Test that service initializes correctly
	if service == nil {
		t.Fatal("service should not be nil")
	}

	// Test that we can get models (empty)
	ctx := context.Background()
	models, err := service.GetFreeModels(ctx)
	if err != nil {
		t.Fatalf("GetFreeModels returned error: %v", err)
	}

	if models == nil || len(models) != 0 {
		t.Errorf("expected empty models slice, got %#v", models)
	}
}

func TestService_GetRandomFreeModel(t *testing.T) {
	// No models served -> expect error without external HTTP
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

	// Test with no models available
	_, err := service.GetRandomFreeModel(ctx)
	if err == nil {
		t.Error("expected error when no models available")
	}
}

func TestService_GetBestFreeModel(t *testing.T) {
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

	// Test with no models available
	_, err := service.GetBestFreeModel(ctx)
	if err == nil {
		t.Error("expected error when no models available")
	}
}

func TestService_IsFreeModel(t *testing.T) {
	service := New("test-key", "https://api.openrouter.ai/v1")

	tests := []struct {
		name     string
		model    Model
		expected bool
	}{
		{
			name: "free model with empty prompt price",
			model: Model{
				ID: "test-model",
				Pricing: Pricing{
					Prompt: "",
				},
			},
			expected: true,
		},
		{
			name: "free model with zero prompt price",
			model: Model{
				ID: "test-model",
				Pricing: Pricing{
					Prompt: "0",
				},
			},
			expected: true,
		},
		{
			name: "free model with zero decimal prompt price",
			model: Model{
				ID: "test-model",
				Pricing: Pricing{
					Prompt: "0.0",
				},
			},
			expected: true,
		},
		{
			name: "paid model",
			model: Model{
				ID: "test-model",
				Pricing: Pricing{
					Prompt: "0.001",
				},
			},
			expected: false,
		},
		{
			name: "model with whitespace prompt price",
			model: Model{
				ID: "test-model",
				Pricing: Pricing{
					Prompt: "  ",
				},
			},
			expected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := service.isFreeModel(tt.model)
			if result != tt.expected {
				t.Errorf("isFreeModel() = %v, want %v", result, tt.expected)
			}
		})
	}
}

func TestService_NewWithRefresh(t *testing.T) {
	refreshInterval := 30 * time.Minute
	service := NewWithRefresh("test-key", "https://api.openrouter.ai/v1", refreshInterval)

	if service == nil {
		t.Fatal("service should not be nil")
	}

	// Test that refresh interval is set correctly
	lastFetch, nextFetch, interval := service.GetRefreshStatus()
	if interval != refreshInterval {
		t.Errorf("expected refresh interval %v, got %v", refreshInterval, interval)
	}

	// Test that lastFetch is zero initially
	if !lastFetch.IsZero() {
		t.Error("lastFetch should be zero initially")
	}

	// Test that nextFetch is calculated correctly
	expectedNext := lastFetch.Add(interval)
	if !nextFetch.Equal(expectedNext) {
		t.Errorf("expected nextFetch %v, got %v", expectedNext, nextFetch)
	}
}

func TestService_GetRefreshStatus(t *testing.T) {
	service := New("test-key", "https://api.openrouter.ai/v1")

	lastFetch, nextFetch, interval := service.GetRefreshStatus()

	// Initially, lastFetch should be zero
	if !lastFetch.IsZero() {
		t.Error("lastFetch should be zero initially")
	}

	// Interval should be 1 hour by default
	expectedInterval := 1 * time.Hour
	if interval != expectedInterval {
		t.Errorf("expected interval %v, got %v", expectedInterval, interval)
	}

	// Next fetch should be calculated from zero time
	expectedNext := lastFetch.Add(interval)
	if !nextFetch.Equal(expectedNext) {
		t.Errorf("expected nextFetch %v, got %v", expectedNext, nextFetch)
	}
}

func TestService_ForceRefresh(t *testing.T) {
	// Return 401 to simulate unauthorized without hitting external network
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = w.Write([]byte("Unauthorized"))
	}))
	defer ts.Close()

	service := New("test-key", ts.URL)
	ctx := context.Background()

	err := service.ForceRefresh(ctx)
	if err == nil {
		t.Error("expected error when forcing refresh without valid API key")
	}
}

func TestService_GetModelInfo(t *testing.T) {
	// Serve a single model, then query a non-existent one
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/models" {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"data": []map[string]any{
				{"id": "free-model-1", "name": "Free Model 1", "pricing": map[string]any{"prompt": "0"}},
			},
		})
	}))
	defer ts.Close()

	service := New("test-key", ts.URL)
	ctx := context.Background()

	// Test with non-existent model
	_, err := service.GetModelInfo(ctx, "non-existent-model")
	if err == nil {
		t.Error("expected error for non-existent model")
	}
}

func TestService_GetFreeModelIDs(t *testing.T) {
	// Serve two free models and verify IDs without external HTTP
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/models" {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"data": []map[string]any{
				{"id": "free-model-1", "pricing": map[string]any{"prompt": "0"}},
				{"id": "free-model-2", "pricing": map[string]any{"prompt": "0"}},
			},
		})
	}))
	defer ts.Close()

	service := New("test-key", ts.URL)
	ctx := context.Background()

	ids, err := service.GetFreeModelIDs(ctx)
	if err != nil {
		t.Fatalf("GetFreeModelIDs returned error: %v", err)
	}

	if len(ids) != 2 {
		t.Errorf("expected 2 ids, got %d", len(ids))
	}
}

func TestModel_String(t *testing.T) {
	model := Model{
		ID:          "test-model",
		Name:        "Test Model",
		Context:     4096,
		Description: "A test model",
		TopProvider: json.RawMessage(`"test-provider"`),
		Pricing: Pricing{
			Prompt:     "0",
			Completion: "0.001",
		},
	}

	// Test that model fields are accessible
	if model.ID != "test-model" {
		t.Errorf("expected ID 'test-model', got %s", model.ID)
	}

	if model.Name != "Test Model" {
		t.Errorf("expected Name 'Test Model', got %s", model.Name)
	}

	if model.Context != 4096 {
		t.Errorf("expected Context 4096, got %d", model.Context)
	}
}

func TestPricing_String(t *testing.T) {
	pricing := Pricing{
		Prompt:     "0",
		Completion: "0.001",
	}

	if s, ok := pricing.Prompt.(string); !ok || s != "0" {
		t.Errorf("expected Prompt '0', got %#v", pricing.Prompt)
	}

	if s, ok := pricing.Completion.(string); !ok || s != "0.001" {
		t.Errorf("expected Completion '0.001', got %#v", pricing.Completion)
	}
}
