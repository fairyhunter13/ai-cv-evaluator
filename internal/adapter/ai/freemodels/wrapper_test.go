// Package freemodels tests the free models wrapper.
package freemodels

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/fairyhunter13/ai-cv-evaluator/internal/config"
	domainmocks "github.com/fairyhunter13/ai-cv-evaluator/internal/domain/mocks"
	mock "github.com/stretchr/testify/mock"
)

func TestFreeModelWrapper_NewFreeModelWrapper(t *testing.T) {
	cfg := config.Config{
		OpenRouterAPIKey:  "test-key",
		OpenRouterBaseURL: "http://unused",
		FreeModelsRefresh: 30 * 60 * 1000000000, // 30 minutes in nanoseconds
	}

	wrapper := NewFreeModelWrapper(cfg)
	if wrapper == nil {
		t.Fatal("wrapper should not be nil")
	}

	if wrapper.client == nil {
		t.Error("client should not be nil")
	}

	if wrapper.freeModelsSvc == nil {
		t.Error("freeModelsSvc should not be nil")
	}

	if wrapper.cfg.OpenRouterAPIKey != cfg.OpenRouterAPIKey {
		t.Error("cfg should be set correctly")
	}

	if wrapper.maxFailures != 3 {
		t.Errorf("expected maxFailures 3, got %d", wrapper.maxFailures)
	}
}

func TestFreeModelWrapper_GetCurrentModel(t *testing.T) {
	cfg := config.Config{
		OpenRouterAPIKey:  "test-key",
		OpenRouterBaseURL: "http://unused",
	}

	wrapper := NewFreeModelWrapper(cfg)

	// Initially, current model should be empty
	if wrapper.GetCurrentModel() != "" {
		t.Error("current model should be empty initially")
	}

	// Set a model and test
	wrapper.lastModel = "test-model"
	if wrapper.GetCurrentModel() != "test-model" {
		t.Errorf("expected current model 'test-model', got %s", wrapper.GetCurrentModel())
	}
}

func TestFreeModelWrapper_GetModelStats(t *testing.T) {
	cfg := config.Config{
		OpenRouterAPIKey:  "test-key",
		OpenRouterBaseURL: "http://unused",
	}

	wrapper := NewFreeModelWrapper(cfg)

	// Initially, stats should be empty
	stats := wrapper.GetModelStats()
	if len(stats) != 0 {
		t.Errorf("expected empty stats, got %v", stats)
	}

	// Add some failures and test
	wrapper.modelFailures["model1"] = 2
	wrapper.modelFailures["model2"] = 1

	stats = wrapper.GetModelStats()
	if len(stats) != 2 {
		t.Errorf("expected 2 stats entries, got %d", len(stats))
	}

	if stats["model1"] != 2 {
		t.Errorf("expected model1 failures 2, got %d", stats["model1"])
	}

	if stats["model2"] != 1 {
		t.Errorf("expected model2 failures 1, got %d", stats["model2"])
	}
}

func TestFreeModelWrapper_GetFreeModelsInfo(t *testing.T) {
	// Serve 2 free models locally to avoid external network
	mock := map[string]any{
		"data": []map[string]any{
			{"id": "free-model-1", "pricing": map[string]any{"prompt": "0"}},
			{"id": "free-model-2", "pricing": map[string]any{"prompt": "0"}},
		},
	}
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/models" {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(mock)
	}))
	defer ts.Close()

	cfg := config.Config{}
	w := &FreeModelWrapper{
		client:        nil, // not used
		freeModelsSvc: NewWithRefresh("", ts.URL, time.Minute),
		cfg:           cfg,
		modelFailures: make(map[string]int),
		maxFailures:   3,
	}
	ctx := context.Background()

	models, err := w.GetFreeModelsInfo(ctx)
	if err != nil {
		t.Fatalf("GetFreeModelsInfo returned error: %v", err)
	}
	if len(models) != 2 {
		t.Fatalf("expected 2 model ids, got %d", len(models))
	}
}

func TestFreeModelWrapper_ChatJSON_NoFreeModels(t *testing.T) {
	// Local server returns empty list -> selectFreeModel fails -> fallback client used
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/models" {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{"data": []any{}})
	}))
	defer ts.Close()

	// Fallback client returns an error quickly (no network)
	mockClient := &domainmocks.AIClient{}
	mockClient.On("ChatJSON", mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return("", errors.New("fallback error"))

	cfg := config.Config{ChatModel: "fallback-model"}
	w := &FreeModelWrapper{
		client:        mockClient,
		freeModelsSvc: NewWithRefresh("", ts.URL, time.Minute),
		cfg:           cfg,
		modelFailures: make(map[string]int),
		maxFailures:   3,
	}

	ctx := context.Background()
	result, err := w.ChatJSON(ctx, "system", "user", 100)
	if err == nil {
		t.Fatal("expected error from fallback client")
	}
	if result != "" {
		t.Fatalf("expected empty result, got %q", result)
	}
}

func TestFreeModelWrapper_Embed(t *testing.T) {
	// Use a mock client to avoid network and backoff
	mockClient := &domainmocks.AIClient{}
	mockClient.On("Embed", mock.Anything, mock.Anything).Return(nil, errors.New("embed error"))

	w := &FreeModelWrapper{
		client:        mockClient,
		freeModelsSvc: NewWithRefresh("", "http://unused", time.Minute),
		cfg:           config.Config{},
		modelFailures: make(map[string]int),
		maxFailures:   3,
	}

	ctx := context.Background()
	texts := []string{"test text"}
	embeddings, err := w.Embed(ctx, texts)
	if err == nil {
		t.Fatal("expected error from mock embed")
	}
	if embeddings != nil {
		t.Fatal("expected nil embeddings on error")
	}
}

func TestFreeModelWrapper_ModelFailureTracking(t *testing.T) {
	cfg := config.Config{
		OpenRouterAPIKey:  "test-key",
		OpenRouterBaseURL: "http://unused",
	}

	wrapper := NewFreeModelWrapper(cfg)

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

	// Test blacklisting
	if wrapper.modelFailures[modelID] >= wrapper.maxFailures {
		t.Error("model should not be blacklisted yet")
	}

	// Add one more failure to trigger blacklisting
	wrapper.modelFailures[modelID] = 3

	if wrapper.modelFailures[modelID] < wrapper.maxFailures {
		t.Error("model should be blacklisted now")
	}
}

func TestFreeModelWrapper_SelectFreeModel(t *testing.T) {
	// Local server returns empty list to ensure deterministic error
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/models" {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{"data": []any{}})
	}))
	defer ts.Close()

	mockClient := &domainmocks.AIClient{}
	w := &FreeModelWrapper{
		client:        mockClient,
		freeModelsSvc: NewWithRefresh("", ts.URL, time.Minute),
		cfg:           config.Config{},
		modelFailures: make(map[string]int),
		maxFailures:   3,
	}
	ctx := context.Background()
	_, err := w.selectFreeModel(ctx)
	if err == nil {
		t.Error("expected error when no free models available")
	}
}

func TestFreeModelWrapper_ChatJSON_SuccessWithFreeModel(t *testing.T) {
	// Local server returns free models
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

	// Mock the real client to return success
	mockClient := &domainmocks.AIClient{}
	mockClient.On("ChatJSON", mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return("success response", nil)

	cfg := config.Config{
		OpenRouterAPIKey:  "test-key",
		OpenRouterBaseURL: ts.URL,
		ChatModel:         "fallback-model",
	}
	w := &FreeModelWrapper{
		client:        mockClient,
		freeModelsSvc: NewWithRefresh("", ts.URL, time.Minute),
		cfg:           cfg,
		modelFailures: make(map[string]int),
		maxFailures:   3,
	}

	ctx := context.Background()
	result, err := w.ChatJSON(ctx, "system", "user", 100)
	if err != nil {
		t.Fatalf("expected success, got error: %v", err)
	}
	if result != "success response" {
		t.Fatalf("expected 'success response', got %q", result)
	}
}

func TestFreeModelWrapper_ChatJSON_FreeModelFailure_FallbackSuccess(t *testing.T) {
	// Local server returns free models
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/models" {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"data": []map[string]any{
				{"id": "free-model-1", "pricing": map[string]any{"prompt": "0"}},
			},
		})
	}))
	defer ts.Close()

	// Mock the real client to return success on fallback
	mockClient := &domainmocks.AIClient{}
	mockClient.On("ChatJSON", mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return("fallback response", nil)

	cfg := config.Config{
		OpenRouterAPIKey:  "test-key",
		OpenRouterBaseURL: ts.URL,
		ChatModel:         "fallback-model",
	}
	w := &FreeModelWrapper{
		client:        mockClient,
		freeModelsSvc: NewWithRefresh("", ts.URL, time.Minute),
		cfg:           cfg,
		modelFailures: make(map[string]int),
		maxFailures:   3,
	}

	ctx := context.Background()
	result, err := w.ChatJSON(ctx, "system", "user", 100)
	if err != nil {
		t.Fatalf("expected success, got error: %v", err)
	}
	if result != "fallback response" {
		t.Fatalf("expected 'fallback response', got %q", result)
	}
}

func TestFreeModelWrapper_ChatJSON_ModelBlacklisting(t *testing.T) {
	// Local server returns free models
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/models" {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"data": []map[string]any{
				{"id": "failing-model", "pricing": map[string]any{"prompt": "0"}},
				{"id": "working-model", "pricing": map[string]any{"prompt": "0"}},
			},
		})
	}))
	defer ts.Close()

	// Mock the real client to return success on fallback
	mockClient := &domainmocks.AIClient{}
	mockClient.On("ChatJSON", mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return("fallback response", nil)

	cfg := config.Config{
		OpenRouterAPIKey:  "test-key",
		OpenRouterBaseURL: ts.URL,
		ChatModel:         "fallback-model",
	}
	w := &FreeModelWrapper{
		client:        mockClient,
		freeModelsSvc: NewWithRefresh("", ts.URL, time.Minute),
		cfg:           cfg,
		modelFailures: make(map[string]int),
		maxFailures:   3,
	}

	// Pre-blacklist a model
	w.modelFailures["failing-model"] = 3

	ctx := context.Background()
	result, err := w.ChatJSON(ctx, "system", "user", 100)
	if err != nil {
		t.Fatalf("expected success, got error: %v", err)
	}
	if result != "fallback response" {
		t.Fatalf("expected 'fallback response', got %q", result)
	}
}

func TestFreeModelWrapper_ChatJSON_AllModelsBlacklisted_Reset(t *testing.T) {
	// Local server returns free models
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/models" {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"data": []map[string]any{
				{"id": "model-1", "pricing": map[string]any{"prompt": "0"}},
				{"id": "model-2", "pricing": map[string]any{"prompt": "0"}},
			},
		})
	}))
	defer ts.Close()

	// Mock the real client to return success on fallback
	mockClient := &domainmocks.AIClient{}
	mockClient.On("ChatJSON", mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return("fallback response", nil)

	cfg := config.Config{
		OpenRouterAPIKey:  "test-key",
		OpenRouterBaseURL: ts.URL,
		ChatModel:         "fallback-model",
	}
	w := &FreeModelWrapper{
		client:        mockClient,
		freeModelsSvc: NewWithRefresh("", ts.URL, time.Minute),
		cfg:           cfg,
		modelFailures: make(map[string]int),
		maxFailures:   3,
	}

	// Blacklist all models
	w.modelFailures["model-1"] = 3
	w.modelFailures["model-2"] = 3

	ctx := context.Background()
	result, err := w.ChatJSON(ctx, "system", "user", 100)
	if err != nil {
		t.Fatalf("expected success, got error: %v", err)
	}
	if result != "fallback response" {
		t.Fatalf("expected 'fallback response', got %q", result)
	}

	// Verify that failure counts were reset
	if len(w.modelFailures) != 0 {
		t.Errorf("expected failure counts to be reset, got %v", w.modelFailures)
	}
}

func TestFreeModelWrapper_GetRoundRobinIndex(t *testing.T) {
	cfg := config.Config{
		OpenRouterAPIKey:  "test-key",
		OpenRouterBaseURL: "http://unused",
	}

	wrapper := NewFreeModelWrapper(cfg)

	// Initially, index should be 0
	if wrapper.GetRoundRobinIndex() != 0 {
		t.Errorf("expected initial index 0, got %d", wrapper.GetRoundRobinIndex())
	}

	// Manually set index and test
	wrapper.roundRobinIdx = 5
	if wrapper.GetRoundRobinIndex() != 5 {
		t.Errorf("expected index 5, got %d", wrapper.GetRoundRobinIndex())
	}
}
