package freemodels

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestService_FetchModels_Success(t *testing.T) {
	// Mock response with free and paid models
	mockResponse := map[string]interface{}{
		"data": []map[string]interface{}{
			{
				"id":             "free-model-1",
				"name":           "Free Model 1",
				"context_length": 4096,
				"description":    "A free model",
				"pricing": map[string]interface{}{
					"prompt":     "0",
					"completion": "0",
				},
			},
			{
				"id":             "paid-model-1",
				"name":           "Paid Model 1",
				"context_length": 8192,
				"description":    "A paid model",
				"pricing": map[string]interface{}{
					"prompt":     "0.001",
					"completion": "0.002",
				},
			},
			{
				"id":             "free-model-2",
				"name":           "Free Model 2",
				"context_length": 2048,
				"description":    "Another free model",
				"pricing": map[string]interface{}{
					"prompt":     "",
					"completion": "0",
				},
			},
		},
	}

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/models" {
			t.Errorf("expected path /models, got %s", r.URL.Path)
		}
		if r.Method != "GET" {
			t.Errorf("expected GET method, got %s", r.Method)
		}
		if r.Header.Get("Content-Type") != "application/json" {
			t.Errorf("expected Content-Type application/json, got %s", r.Header.Get("Content-Type"))
		}
		if r.Header.Get("Authorization") != "Bearer test-key" {
			t.Errorf("expected Authorization Bearer test-key, got %s", r.Header.Get("Authorization"))
		}

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(mockResponse)
	}))
	defer ts.Close()

	service := New("test-key", ts.URL)
	ctx := context.Background()

	// Force refresh to test the fetchModels method
	err := service.ForceRefresh(ctx)
	if err != nil {
		t.Fatalf("ForceRefresh failed: %v", err)
	}

	// Test GetFreeModels returns the free models
	models, err := service.GetFreeModels(ctx)
	if err != nil {
		t.Fatalf("GetFreeModels failed: %v", err)
	}

	if len(models) != 2 {
		t.Errorf("expected 2 free models, got %d", len(models))
	}

	// Verify the free models
	modelIDs := make(map[string]bool)
	for _, model := range models {
		modelIDs[model.ID] = true
	}

	if !modelIDs["free-model-1"] {
		t.Error("expected free-model-1 to be in results")
	}
	if !modelIDs["free-model-2"] {
		t.Error("expected free-model-2 to be in results")
	}
	if modelIDs["paid-model-1"] {
		t.Error("expected paid-model-1 to NOT be in results")
	}
}

func TestService_FetchModels_HTTPError(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte("Internal Server Error"))
	}))
	defer ts.Close()

	service := New("test-key", ts.URL)
	ctx := context.Background()

	err := service.ForceRefresh(ctx)
	if err == nil {
		t.Fatal("expected error for 500 status code")
	}

	if !strings.Contains(err.Error(), "API returned status 500") {
		t.Errorf("expected error about status 500, got: %v", err)
	}
}

func TestService_FetchModels_NotFound(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		_, _ = w.Write([]byte("Not Found"))
	}))
	defer ts.Close()

	service := New("test-key", ts.URL)
	ctx := context.Background()

	err := service.ForceRefresh(ctx)
	if err == nil {
		t.Fatal("expected error for 404 status code")
	}

	if !strings.Contains(err.Error(), "API returned status 404") {
		t.Errorf("expected error about status 404, got: %v", err)
	}
}

func TestService_FetchModels_Unauthorized(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = w.Write([]byte("Unauthorized"))
	}))
	defer ts.Close()

	service := New("test-key", ts.URL)
	ctx := context.Background()

	err := service.ForceRefresh(ctx)
	if err == nil {
		t.Fatal("expected error for 401 status code")
	}

	if !strings.Contains(err.Error(), "API returned status 401") {
		t.Errorf("expected error about status 401, got: %v", err)
	}
}

func TestService_FetchModels_InvalidJSON(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte("invalid json"))
	}))
	defer ts.Close()

	service := New("test-key", ts.URL)
	ctx := context.Background()

	err := service.ForceRefresh(ctx)
	if err == nil {
		t.Fatal("expected error for invalid JSON")
	}

	if !strings.Contains(err.Error(), "failed to decode response") {
		t.Errorf("expected JSON decode error, got: %v", err)
	}
}

func TestService_FetchModels_EmptyResponse(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]interface{}{"data": []interface{}{}})
	}))
	defer ts.Close()

	service := New("test-key", ts.URL)
	ctx := context.Background()

	err := service.ForceRefresh(ctx)
	if err != nil {
		t.Fatalf("ForceRefresh failed: %v", err)
	}

	models, err := service.GetFreeModels(ctx)
	if err != nil {
		t.Fatalf("GetFreeModels failed: %v", err)
	}

	if len(models) != 0 {
		t.Errorf("expected 0 models, got %d", len(models))
	}
}

func TestService_FetchModels_NoAPIKey(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "" {
			t.Errorf("expected no Authorization header, got %s", r.Header.Get("Authorization"))
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]interface{}{"data": []interface{}{}})
	}))
	defer ts.Close()

	service := New("", ts.URL) // No API key
	ctx := context.Background()

	err := service.ForceRefresh(ctx)
	if err != nil {
		t.Fatalf("ForceRefresh failed: %v", err)
	}
}

func TestService_GetRandomFreeModel_Success(t *testing.T) {
	mockResponse := map[string]interface{}{
		"data": []map[string]interface{}{
			{
				"id":             "free-model-1",
				"name":           "Free Model 1",
				"context_length": 4096,
				"description":    "A free model",
				"pricing": map[string]interface{}{
					"prompt": "0",
				},
			},
			{
				"id":             "free-model-2",
				"name":           "Free Model 2",
				"context_length": 2048,
				"description":    "Another free model",
				"pricing": map[string]interface{}{
					"prompt": "0",
				},
			},
		},
	}

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(mockResponse)
	}))
	defer ts.Close()

	service := New("test-key", ts.URL)
	ctx := context.Background()

	// Test multiple calls to ensure we get valid model IDs
	for i := 0; i < 10; i++ {
		modelID, err := service.GetRandomFreeModel(ctx)
		if err != nil {
			t.Fatalf("GetRandomFreeModel failed: %v", err)
		}

		if modelID != "free-model-1" && modelID != "free-model-2" {
			t.Errorf("expected model ID to be free-model-1 or free-model-2, got %s", modelID)
		}
	}
}

func TestService_GetBestFreeModel_Success(t *testing.T) {
	mockResponse := map[string]interface{}{
		"data": []map[string]interface{}{
			{
				"id":             "free-model-1",
				"name":           "Free Model 1",
				"context_length": 2048,
				"description":    "A free model",
				"pricing": map[string]interface{}{
					"prompt": "0",
				},
			},
			{
				"id":             "free-model-2",
				"name":           "Free Model 2",
				"context_length": 4096,
				"description":    "Another free model",
				"pricing": map[string]interface{}{
					"prompt": "0",
				},
			},
			{
				"id":             "free-model-3",
				"name":           "Free Model 3",
				"context_length": 4096,
				"description":    "Another free model with same context",
				"pricing": map[string]interface{}{
					"prompt": "0",
				},
			},
		},
	}

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(mockResponse)
	}))
	defer ts.Close()

	service := New("test-key", ts.URL)
	ctx := context.Background()

	modelID, err := service.GetBestFreeModel(ctx)
	if err != nil {
		t.Fatalf("GetBestFreeModel failed: %v", err)
	}

	// Should return the model with highest context length
	// If context lengths are equal, should return the one with lexicographically smaller name
	if modelID != "free-model-2" {
		t.Errorf("expected free-model-2 (highest context), got %s", modelID)
	}
}

func TestService_GetModelInfo_Success(t *testing.T) {
	mockResponse := map[string]interface{}{
		"data": []map[string]interface{}{
			{
				"id":             "free-model-1",
				"name":           "Free Model 1",
				"context_length": 4096,
				"description":    "A free model",
				"pricing": map[string]interface{}{
					"prompt": "0",
				},
			},
		},
	}

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(mockResponse)
	}))
	defer ts.Close()

	service := New("test-key", ts.URL)
	ctx := context.Background()

	model, err := service.GetModelInfo(ctx, "free-model-1")
	if err != nil {
		t.Fatalf("GetModelInfo failed: %v", err)
	}

	if model.ID != "free-model-1" {
		t.Errorf("expected model ID free-model-1, got %s", model.ID)
	}

	if model.Name != "Free Model 1" {
		t.Errorf("expected model name 'Free Model 1', got %s", model.Name)
	}
}

func TestService_GetModelInfo_NotFound(t *testing.T) {
	mockResponse := map[string]interface{}{
		"data": []map[string]interface{}{
			{
				"id":             "free-model-1",
				"name":           "Free Model 1",
				"context_length": 4096,
				"description":    "A free model",
				"pricing": map[string]interface{}{
					"prompt": "0",
				},
			},
		},
	}

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(mockResponse)
	}))
	defer ts.Close()

	service := New("test-key", ts.URL)
	ctx := context.Background()

	_, err := service.GetModelInfo(ctx, "non-existent-model")
	if err == nil {
		t.Fatal("expected error for non-existent model")
	}

	if !strings.Contains(err.Error(), "model non-existent-model not found") {
		t.Errorf("expected error about model not found, got: %v", err)
	}
}

func TestService_GetFreeModelIDs_Success(t *testing.T) {
	mockResponse := map[string]interface{}{
		"data": []map[string]interface{}{
			{
				"id":             "free-model-1",
				"name":           "Free Model 1",
				"context_length": 4096,
				"description":    "A free model",
				"pricing": map[string]interface{}{
					"prompt": "0",
				},
			},
			{
				"id":             "free-model-2",
				"name":           "Free Model 2",
				"context_length": 2048,
				"description":    "Another free model",
				"pricing": map[string]interface{}{
					"prompt": "0",
				},
			},
		},
	}

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(mockResponse)
	}))
	defer ts.Close()

	service := New("test-key", ts.URL)
	ctx := context.Background()

	ids, err := service.GetFreeModelIDs(ctx)
	if err != nil {
		t.Fatalf("GetFreeModelIDs failed: %v", err)
	}

	if len(ids) != 2 {
		t.Errorf("expected 2 model IDs, got %d", len(ids))
	}

	// Check that we have the expected IDs
	idMap := make(map[string]bool)
	for _, id := range ids {
		idMap[id] = true
	}

	if !idMap["free-model-1"] {
		t.Error("expected free-model-1 in IDs")
	}
	if !idMap["free-model-2"] {
		t.Error("expected free-model-2 in IDs")
	}
}

func TestService_RefreshInterval(t *testing.T) {
	service := New("test-key", "http://unused")

	// Test that refresh interval is set correctly
	_, _, interval := service.GetRefreshStatus()
	expectedInterval := 1 * time.Hour
	if interval != expectedInterval {
		t.Errorf("expected refresh interval %v, got %v", expectedInterval, interval)
	}
}

func TestService_RefreshInterval_Custom(t *testing.T) {
	customInterval := 30 * time.Minute
	service := NewWithRefresh("test-key", "http://unused", customInterval)

	// Test that refresh interval is set correctly
	_, _, interval := service.GetRefreshStatus()
	if interval != customInterval {
		t.Errorf("expected refresh interval %v, got %v", customInterval, interval)
	}
}
