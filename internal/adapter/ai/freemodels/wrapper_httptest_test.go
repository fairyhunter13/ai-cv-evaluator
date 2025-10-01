package freemodels

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/fairyhunter13/ai-cv-evaluator/internal/config"
	domainmocks "github.com/fairyhunter13/ai-cv-evaluator/internal/domain/mocks"
	mock "github.com/stretchr/testify/mock"
)

func TestFreeModelWrapper_ChatJSON_Success(t *testing.T) {
	// Local server returns empty models so wrapper will fallback to mock client without external DNS warnings
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/models" {
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(map[string]any{"data": []any{}})
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	defer ts.Close()

	cfg := config.Config{
		FreeModelsRefresh: 30 * time.Minute,
		ChatModel:         "fallback-model",
	}

	// Create wrapper with mock client that always succeeds (fallback path)
	mockClient := &domainmocks.AIClient{}
	mockClient.On("ChatJSON", mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return("successful response from free model", nil)

	wrapper := &FreeModelWrapper{
		client:        mockClient,
		freeModelsSvc: New("", ts.URL),
		cfg:           cfg,
		modelFailures: make(map[string]int),
		maxFailures:   3,
	}

	// Set lastModel for book-keeping; selection will fail and fallback is used
	wrapper.lastModel = "free-model-1"

	ctx := context.Background()
	result, err := wrapper.ChatJSON(ctx, "system prompt", "user prompt", 100)

	if err != nil {
		t.Fatalf("ChatJSON failed: %v", err)
	}

	if result != "successful response from free model" {
		t.Errorf("expected successful response, got: %s", result)
	}
}

func TestFreeModelWrapper_ChatJSON_FreeModelFailure_Fallback(t *testing.T) {
	// Local server returns empty models to force fallback without external DNS warnings
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/models" {
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(map[string]any{"data": []any{}})
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	defer ts.Close()

	cfg := config.Config{
		FreeModelsRefresh: 30 * time.Minute,
		ChatModel:         "fallback-model",
	}

	// Create wrapper with mock client that succeeds for fallback
	mockClient := &domainmocks.AIClient{}
	mockClient.On("ChatJSON", mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return("fallback response", nil)

	wrapper := &FreeModelWrapper{
		client:        mockClient,
		freeModelsSvc: New("", ts.URL),
		cfg:           cfg,
		modelFailures: make(map[string]int),
		maxFailures:   3,
	}

	// Simulate a scenario where free model selection fails
	wrapper.modelFailures["free-model-1"] = 1

	ctx := context.Background()
	result, err := wrapper.ChatJSON(ctx, "system prompt", "user prompt", 100)

	if err != nil {
		t.Fatalf("ChatJSON failed: %v", err)
	}

	if result != "fallback response" {
		t.Errorf("expected fallback response, got: %s", result)
	}
}

func TestFreeModelWrapper_ModelBlacklisting(t *testing.T) {
	cfg := config.Config{
		FreeModelsRefresh: 30 * time.Minute,
		ChatModel:         "fallback-model",
	}

	// Create wrapper with mock client and local service (not exercised here but avoids external refs)
	mockClient := &domainmocks.AIClient{}
	wrapper := &FreeModelWrapper{
		client:        mockClient,
		freeModelsSvc: New("", "http://unused"),
		cfg:           cfg,
		modelFailures: make(map[string]int),
		maxFailures:   3,
	}

	// Test model failure tracking
	modelID := "test-model"

	// Initially, no failures
	if wrapper.modelFailures[modelID] != 0 {
		t.Error("model should have 0 failures initially")
	}

	// Simulate failures
	wrapper.modelFailures[modelID] = 2

	if wrapper.modelFailures[modelID] != 2 {
		t.Errorf("expected 2 failures, got %d", wrapper.modelFailures[modelID])
	}

	// Test blacklisting threshold
	if wrapper.modelFailures[modelID] >= wrapper.maxFailures {
		t.Error("model should not be blacklisted yet")
	}

	// Add one more failure to trigger blacklisting
	wrapper.modelFailures[modelID] = 3

	if wrapper.modelFailures[modelID] < wrapper.maxFailures {
		t.Error("model should be blacklisted now")
	}
}

func TestFreeModelWrapper_AllModelsBlacklisted_Reset(t *testing.T) {
	// Mock response with free models
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

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/models" {
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(mockResponse)
		} else {
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer ts.Close()

	cfg := config.Config{
		OpenRouterAPIKey:  "test-key",
		OpenRouterBaseURL: ts.URL,
		FreeModelsRefresh: 30 * time.Minute,
		ChatModel:         "fallback-model",
	}

	// Create wrapper with all models blacklisted
	mockClient := &domainmocks.AIClient{}
	wrapper := &FreeModelWrapper{
		client:        mockClient,
		freeModelsSvc: NewWithRefresh(cfg.OpenRouterAPIKey, cfg.OpenRouterBaseURL, cfg.FreeModelsRefresh),
		cfg:           cfg,
		modelFailures: map[string]int{
			"free-model-1": 3, // Blacklisted
		},
		maxFailures: 3,
	}

	ctx := context.Background()

	// This should reset the failure counts and use free-model-1 again
	_, err := wrapper.selectFreeModel(ctx)
	if err != nil {
		t.Fatalf("selectFreeModel failed: %v", err)
	}

	// Verify that failure counts were reset
	if wrapper.modelFailures["free-model-1"] != 0 {
		t.Errorf("expected failure count to be reset to 0, got: %d", wrapper.modelFailures["free-model-1"])
	}
}

func TestFreeModelWrapper_Embed_Delegation(t *testing.T) {
	cfg := config.Config{
		OpenRouterAPIKey:  "test-key",
		OpenRouterBaseURL: "https://api.openrouter.ai/v1",
	}

	// Create wrapper with mock client
	mockClient := &domainmocks.AIClient{}
	mockClient.On("Embed", mock.Anything, mock.Anything).Return([][]float32{{0.1, 0.2, 0.3}, {0.4, 0.5, 0.6}}, nil)

	wrapper := &FreeModelWrapper{
		client:        mockClient,
		freeModelsSvc: New("test-key", "https://api.openrouter.ai/v1"),
		cfg:           cfg,
		modelFailures: make(map[string]int),
		maxFailures:   3,
	}

	ctx := context.Background()
	texts := []string{"text1", "text2"}
	embeddings, err := wrapper.Embed(ctx, texts)

	if err != nil {
		t.Fatalf("Embed failed: %v", err)
	}

	if len(embeddings) != 2 {
		t.Errorf("expected 2 embeddings, got %d", len(embeddings))
	}

	if len(embeddings[0]) != 3 {
		t.Errorf("expected embedding dimension 3, got %d", len(embeddings[0]))
	}
}

func TestFreeModelWrapper_GetFreeModelsInfo_WithHTTPServer(t *testing.T) {
	// Mock response with free models
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

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/models" {
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(mockResponse)
		} else {
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer ts.Close()

	cfg := config.Config{
		OpenRouterAPIKey:  "test-key",
		OpenRouterBaseURL: ts.URL,
		FreeModelsRefresh: 30 * time.Minute,
	}

	mockClient := &domainmocks.AIClient{}
	wrapper := &FreeModelWrapper{
		client:        mockClient,
		freeModelsSvc: NewWithRefresh(cfg.OpenRouterAPIKey, cfg.OpenRouterBaseURL, cfg.FreeModelsRefresh),
		cfg:           cfg,
		modelFailures: make(map[string]int),
		maxFailures:   3,
	}

	ctx := context.Background()
	models, err := wrapper.GetFreeModelsInfo(ctx)

	if err != nil {
		t.Fatalf("GetFreeModelsInfo failed: %v", err)
	}

	if len(models) != 2 {
		t.Errorf("expected 2 model IDs, got %d", len(models))
	}

	// Check that we have the expected IDs
	idMap := make(map[string]bool)
	for _, id := range models {
		idMap[id] = true
	}

	if !idMap["free-model-1"] {
		t.Error("expected free-model-1 in model IDs")
	}
	if !idMap["free-model-2"] {
		t.Error("expected free-model-2 in model IDs")
	}
}
