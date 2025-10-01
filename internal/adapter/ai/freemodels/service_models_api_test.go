package freemodels

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/fairyhunter13/ai-cv-evaluator/internal/config"
)

// helper to load the sample models response JSON from repo root
func loadSampleModelsJSON(t *testing.T) []byte {
	t.Helper()
	p := filepath.Join("..", "..", "..", "..", "models.response.json")
	// #nosec G304 -- This is a test file reading a known test fixture
	b, err := os.ReadFile(p)
	if err == nil {
		return b
	}
	// fallback to a minimal inline sample to avoid filesystem dependency
	return []byte(`{"data":[{"id":"sample-free","name":"Sample Free","context_length":1024,"description":"sample","pricing":{"prompt":"0","completion":"0"}}]}`)
}

func TestService_FetchModels_ParsesOpenRouterSample(t *testing.T) {
	body := loadSampleModelsJSON(t)

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/models" {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write(body)
	}))
	defer ts.Close()

	svc := NewWithRefresh("", ts.URL, time.Hour)
	ctx := context.Background()

	models, err := svc.GetFreeModels(ctx)
	if err != nil {
		t.Fatalf("GetFreeModels returned error: %v", err)
	}
	if models == nil {
		t.Fatalf("expected non-nil models slice")
	}
	// We don't assert count because the sample may have no free models by our definition
	// The important part is that decoding succeeds (no panic / error) and returns a slice
}

func TestWrapper_SelectFreeModel_WithNoFreeModels_ReturnsError(t *testing.T) {
	// Serve an empty models list to guarantee no free models are available
	empty := []byte(`{"data":[]}`)
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/models" {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write(empty)
	}))
	defer ts.Close()

	cfg := struct {
		BaseURL string
		Refresh time.Duration
	}{
		BaseURL: ts.URL,
		Refresh: time.Hour,
	}

	// Construct wrapper using the same service but avoid full real client usage in test
	svc := NewWithRefresh("", cfg.BaseURL, cfg.Refresh)
	w := &FreeModelWrapper{
		client:        nil, // not used by selectFreeModel
		freeModelsSvc: svc,
		cfg:           config.Config{}, // zero value fine for this selection test
		modelFailures: make(map[string]int),
		maxFailures:   3,
	}

	ctx := context.Background()
	_, err := w.selectFreeModel(ctx)
	if err == nil {
		t.Fatalf("expected error when no free models available")
	}
}

func TestWrapper_SelectsZeroPricedModel_WhenPresent(t *testing.T) {
	// Load sample and force one model's prompt price to zero
	var root map[string]any
	if err := json.Unmarshal(loadSampleModelsJSON(t), &root); err != nil {
		t.Fatalf("unmarshal sample: %v", err)
	}
	arr, _ := root["data"].([]any)
	if len(arr) == 0 {
		t.Skip("sample has no data entries to modify")
	}
	first, _ := arr[0].(map[string]any)
	pricing, _ := first["pricing"].(map[string]any)
	if pricing == nil {
		pricing = map[string]any{}
		first["pricing"] = pricing
	}
	pricing["prompt"] = "0" // mark free

	mut, _ := json.Marshal(root)

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/models" {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write(mut)
	}))
	defer ts.Close()

	svc := NewWithRefresh("", ts.URL, time.Hour)
	w := &FreeModelWrapper{
		client:        nil,
		freeModelsSvc: svc,
		cfg:           config.Config{},
		modelFailures: make(map[string]int),
		maxFailures:   3,
	}
	ctx := context.Background()
	mid, err := w.selectFreeModel(ctx)
	if err != nil {
		t.Fatalf("expected a free model, got error: %v", err)
	}
	if mid == "" {
		t.Fatalf("expected a non-empty model id")
	}
}
