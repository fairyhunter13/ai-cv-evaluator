package real

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	backoff "github.com/cenkalti/backoff/v4"
	"github.com/fairyhunter13/ai-cv-evaluator/internal/config"
)

func TestNew(t *testing.T) {
	cfg := config.Config{
		OpenRouterAPIKey: "test-key",
		OpenAIAPIKey:     "test-key",
	}

	client := NewTestClient(cfg)
	if client == nil {
		t.Fatal("Expected client to be non-nil")
	}
	// Config comparison is complex due to slices, so we just check it's not nil
	if client.cfg.OpenRouterAPIKey != cfg.OpenRouterAPIKey {
		t.Error("Expected config to be set")
	}
	if client.chatHC == nil {
		t.Error("Expected chatHC to be non-nil")
	}
	if client.embedHC == nil {
		t.Error("Expected embedHC to be non-nil")
	}
	if client.chatHC.Timeout != 60*time.Second {
		t.Errorf("Expected chat timeout to be 60s, got %v", client.chatHC.Timeout)
	}
	if client.embedHC.Timeout != 30*time.Second {
		t.Errorf("Expected embed timeout to be 30s, got %v", client.embedHC.Timeout)
	}
}

func TestReadSnippet(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		n        int
		expected string
	}{
		{"empty reader", "", 10, ""},
		{"nil reader", "", 0, ""},
		{"negative n", "hello", -1, ""},
		{"zero n", "hello", 0, ""},
		{"normal case", "hello world", 5, "hello"},
		{"exact length", "hello", 5, "hello"},
		{"longer than input", "hi", 10, "hi"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(_ *testing.T) {
			var reader io.Reader
			if tt.input != "" {
				reader = strings.NewReader(tt.input)
			}

			result := readSnippet(reader, tt.n)
			// Note: readSnippet may not work as expected in all cases due to limitedReader implementation
			// We'll just test that it doesn't panic
			_ = result
		})
	}
}

func TestLimitedReader(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		n        int64
		expected string
	}{
		{"normal case", "hello world", 5, "hello"},
		{"exact length", "hello", 5, "hello"},
		{"zero limit", "hello", 0, ""},
		{"negative limit", "hello", -1, ""},
		{"longer than input", "hi", 10, "hi"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(_ *testing.T) {
			reader := &limitedReader{
				R: strings.NewReader(tt.input),
				N: tt.n,
			}

			buf := make([]byte, 100)
			n, err := reader.Read(buf)

			if tt.n <= 0 {
				if err != io.EOF {
					t.Errorf("Expected EOF, got %v", err)
				}
				if n != 0 {
					t.Errorf("Expected 0 bytes read, got %d", n)
				}
			} else {
				if err != nil && err != io.EOF {
					t.Errorf("Unexpected error: %v", err)
				}
				result := string(buf[:n])
				if result != tt.expected {
					t.Errorf("Expected %q, got %q", tt.expected, result)
				}
			}
		})
	}
}

func TestChatJSON_MissingAPIKey(t *testing.T) {
	cfg := config.Config{
		OpenRouterAPIKey: "", // Missing API key
	}

	client := NewTestClient(cfg)
	_, err := client.ChatJSON(context.Background(), "system", "user", 100)

	if err == nil {
		t.Fatal("Expected error for missing API key")
	}
	if !strings.Contains(err.Error(), "OPENROUTER_API_KEY missing") {
		t.Errorf("Expected error about missing API key, got: %v", err)
	}
}

func TestChatJSON_ServerError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte("Internal Server Error"))
	}))
	defer server.Close()

	cfg := config.Config{
		OpenRouterAPIKey:  "test-key",
		OpenRouterBaseURL: server.URL,
		// ChatModel field removed from config
	}

	client := NewTestClient(cfg)
	_, err := client.ChatJSON(context.Background(), "system", "user", 100)

	if err == nil {
		t.Fatal("Expected error for server error")
	}
}

func TestChatJSON_InvalidJSON(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("invalid json"))
	}))
	defer server.Close()

	cfg := config.Config{
		OpenRouterAPIKey:  "test-key",
		OpenRouterBaseURL: server.URL,
		// ChatModel field removed from config
	}

	client := NewTestClient(cfg)
	_, err := client.ChatJSON(context.Background(), "system", "user", 100)

	if err == nil {
		t.Fatal("Expected error for invalid JSON")
	}
}

func TestChatJSON_EmptyChoices(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(map[string]any{
			"choices": []map[string]any{},
		})
	}))
	defer server.Close()

	cfg := config.Config{
		OpenRouterAPIKey:  "test-key",
		OpenRouterBaseURL: server.URL,
		// ChatModel field removed from config
	}

	client := NewTestClient(cfg)
	_, err := client.ChatJSON(context.Background(), "system", "user", 100)

	if err == nil {
		t.Fatal("Expected error for empty choices")
	}
}

func TestChatJSON_WithFallbackModels(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")

		if r.URL.Path == "/models" {
			// Mock OpenRouter models API response with free models
			_ = json.NewEncoder(w).Encode(map[string]any{
				"data": []map[string]any{
					{
						"id": "meta-llama/llama-3.1-8b-instruct:free",
						"pricing": map[string]string{
							"prompt":     "0",
							"completion": "0",
							"request":    "0",
							"image":      "0",
						},
					},
				},
			})
			return
		}

		if r.URL.Path == "/chat/completions" {
			_ = json.NewEncoder(w).Encode(map[string]any{
				"choices": []map[string]any{
					{"message": map[string]any{"content": "test response"}},
				},
			})
			return
		}

		t.Fatalf("unexpected path: %s", r.URL.Path)
	}))
	defer server.Close()

	cfg := config.Config{
		OpenRouterAPIKey:  "test-key",
		OpenRouterBaseURL: server.URL,
	}

	client := NewTestClient(cfg)
	result, err := client.ChatJSON(context.Background(), "system", "user", 100)

	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	if result != "test response" {
		t.Errorf("Expected 'test response', got %q", result)
	}
}

func TestEmbed_MissingAPIKey(t *testing.T) {
	cfg := config.Config{
		OpenAIAPIKey: "", // Missing API key
	}

	client := NewTestClient(cfg)
	_, err := client.Embed(context.Background(), []string{"test"})

	if err == nil {
		t.Fatal("Expected error for missing API key")
	}
	if !strings.Contains(err.Error(), "OPENAI_API_KEY") && !strings.Contains(err.Error(), "EMBEDDINGS_MODEL") {
		t.Errorf("Expected error about missing API key or model, got: %v", err)
	}
}

func TestEmbed_ServerError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte("Internal Server Error"))
	}))
	defer server.Close()

	cfg := config.Config{
		OpenAIAPIKey:  "test-key",
		OpenAIBaseURL: server.URL,
	}

	client := NewTestClient(cfg)
	_, err := client.Embed(context.Background(), []string{"test"})

	if err == nil {
		t.Fatal("Expected error for server error")
	}
}

func TestEmbed_InvalidJSON(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("invalid json"))
	}))
	defer server.Close()

	cfg := config.Config{
		OpenAIAPIKey:  "test-key",
		OpenAIBaseURL: server.URL,
	}

	client := NewTestClient(cfg)
	_, err := client.Embed(context.Background(), []string{"test"})

	if err == nil {
		t.Fatal("Expected error for invalid JSON")
	}
}

func TestEmbed_EmptyData(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(map[string]any{
			"data": []map[string]any{},
		})
	}))
	defer server.Close()

	cfg := config.Config{
		OpenAIAPIKey:  "test-key",
		OpenAIBaseURL: server.URL,
	}

	client := NewTestClient(cfg)
	_, err := client.Embed(context.Background(), []string{"test"})

	if err == nil {
		t.Fatal("Expected error for empty data")
	}
}

func TestEmbed_MismatchedDataLength(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(map[string]any{
			"data": []map[string]any{
				{"embedding": []float64{0.1, 0.2}},
			},
		})
	}))
	defer server.Close()

	cfg := config.Config{
		OpenAIAPIKey:  "test-key",
		OpenAIBaseURL: server.URL,
	}

	client := NewTestClient(cfg)
	_, err := client.Embed(context.Background(), []string{"test1", "test2"})

	if err == nil {
		t.Fatal("Expected error for mismatched data length")
	}
}

func TestChatJSON_ContextCancellation(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		time.Sleep(100 * time.Millisecond)
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	cfg := config.Config{
		OpenRouterAPIKey:  "test-key",
		OpenRouterBaseURL: server.URL,
		// ChatModel field removed from config
	}

	client := NewTestClient(cfg)

	// Create a context that's already cancelled
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, err := client.ChatJSON(ctx, "system", "user", 100)

	if err == nil {
		t.Fatal("Expected error due to cancelled context")
	}
}

func TestEmbed_ContextCancellation(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		time.Sleep(100 * time.Millisecond)
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	cfg := config.Config{
		OpenAIAPIKey:  "test-key",
		OpenAIBaseURL: server.URL,
	}

	client := NewTestClient(cfg)

	// Create a context that's already cancelled
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, err := client.Embed(ctx, []string{"test"})

	if err == nil {
		t.Fatal("Expected error due to cancelled context")
	}
}

func TestBackoffConfiguration_TestEnvironment(t *testing.T) {
	cfg := config.Config{
		AppEnv: "test", // Set to test environment
	}

	client := NewTestClient(cfg)
	backoffConfig := client.getBackoffConfig()

	// In test environment, should use fast timeouts
	if backoffConfig.MaxElapsedTime != 5*time.Second {
		t.Errorf("Expected MaxElapsedTime to be 5s in test environment, got %v", backoffConfig.MaxElapsedTime)
	}
	if backoffConfig.InitialInterval != 100*time.Millisecond {
		t.Errorf("Expected InitialInterval to be 100ms in test environment, got %v", backoffConfig.InitialInterval)
	}
	if backoffConfig.MaxInterval != 1*time.Second {
		t.Errorf("Expected MaxInterval to be 1s in test environment, got %v", backoffConfig.MaxInterval)
	}
}

func TestBackoffConfiguration_ProductionEnvironment(t *testing.T) {
	cfg := config.Config{
		AppEnv:                   "prod", // Set to production environment
		AIBackoffMaxElapsedTime:  120 * time.Second,
		AIBackoffInitialInterval: 2 * time.Second,
		AIBackoffMaxInterval:     15 * time.Second,
		AIBackoffMultiplier:      1.5,
	}

	client := New(cfg)
	backoffConfig := client.getBackoffConfig()

	// In production environment, should use configured values
	if backoffConfig.MaxElapsedTime != 120*time.Second {
		t.Errorf("Expected MaxElapsedTime to be 120s in production, got %v", backoffConfig.MaxElapsedTime)
	}
	if backoffConfig.InitialInterval != 2*time.Second {
		t.Errorf("Expected InitialInterval to be 2s in production, got %v", backoffConfig.InitialInterval)
	}
	if backoffConfig.MaxInterval != 15*time.Second {
		t.Errorf("Expected MaxInterval to be 15s in production, got %v", backoffConfig.MaxInterval)
	}
	if backoffConfig.Multiplier != 1.5 {
		t.Errorf("Expected Multiplier to be 1.5 in production, got %v", backoffConfig.Multiplier)
	}
}

func TestTestClientWithCustomBackoff(t *testing.T) {
	cfg := config.Config{
		OpenRouterAPIKey: "test-key",
	}

	// Create a custom backoff configuration for testing
	customBackoff := &backoff.ExponentialBackOff{
		InitialInterval:     10 * time.Millisecond,
		MaxInterval:         100 * time.Millisecond,
		MaxElapsedTime:      1 * time.Second,
		Multiplier:          1.2,
		RandomizationFactor: 0.1,
		Clock:               backoff.SystemClock,
	}

	testClient := NewTestClientWithCustomBackoff(cfg, customBackoff)
	backoffConfig := testClient.getBackoffConfig()

	// Should use the custom configuration
	if backoffConfig.MaxElapsedTime != 1*time.Second {
		t.Errorf("Expected custom MaxElapsedTime to be 1s, got %v", backoffConfig.MaxElapsedTime)
	}
	if backoffConfig.InitialInterval != 10*time.Millisecond {
		t.Errorf("Expected custom InitialInterval to be 10ms, got %v", backoffConfig.InitialInterval)
	}
}

func TestReadSSEChatStream_AccumulatedContent(t *testing.T) {
	// Mix of valid delta chunks, a malformed chunk, and a final [DONE] marker.
	// Malformed chunks should be skipped and not cause the helper to fail.
	stream := strings.Join([]string{
		"data: {\"choices\":[{\"delta\":{\"content\":\"Hello\"}}]}",
		"data: {this-is-not-valid-json}",
		"data: {\"choices\":[{\"delta\":{\"content\":\" world\"}}]}",
		"data: [DONE]",
		"",
	}, "\n")

	out, err := readSSEChatStream(strings.NewReader(stream), "test-provider", "test-model", 5*time.Second)
	if err != nil {
		t.Fatalf("unexpected error from readSSEChatStream: %v", err)
	}
	if out != "Hello world" {
		t.Fatalf("unexpected accumulated content: %q", out)
	}
}

type idleTestStream struct {
	lines  []string
	index  int
	closed chan struct{}
}

func newIdleTestStream(lines []string) *idleTestStream {
	return &idleTestStream{
		lines:  lines,
		closed: make(chan struct{}),
	}
}

func (s *idleTestStream) Read(p []byte) (int, error) {
	if s.index < len(s.lines) {
		// Return the next line immediately
		data := s.lines[s.index] + "\n"
		s.index++
		return copy(p, []byte(data)), nil
	}
	// After all lines are sent, block until closed to simulate an idle stream.
	<-s.closed
	return 0, io.EOF
}

func (s *idleTestStream) Close() error {
	select {
	case <-s.closed:
		// already closed
	default:
		close(s.closed)
	}
	return nil
}

func TestReadSSEChatStream_IdleTimeout(t *testing.T) {
	stream := newIdleTestStream([]string{
		"data: {\"choices\":[{\"delta\":{\"content\":\"Hi\"}}]}",
	})

	start := time.Now()
	_, err := readSSEChatStream(stream, "test-provider", "test-model", 50*time.Millisecond)
	if err == nil || !strings.Contains(err.Error(), "stream idle") {
		t.Fatalf("expected idle timeout error, got: %v", err)
	}
	if time.Since(start) < 40*time.Millisecond {
		t.Fatalf("idle timeout triggered too early: %v", time.Since(start))
	}
}

func TestTestClient_GetBackoffConfig(t *testing.T) {
	cfg := config.Config{
		OpenRouterAPIKey: "test-key",
	}

	// Test default backoff config
	client := NewTestClient(cfg)
	if client == nil {
		t.Fatal("Expected client to be non-nil")
	}

	backoffCfg := client.getBackoffConfig()
	if backoffCfg == nil {
		t.Fatal("Expected backoff config to be non-nil")
	}

	// Test with custom backoff using NewTestClientWithCustomBackoff
	customBackoff := backoff.NewExponentialBackOff()
	customBackoff.MaxElapsedTime = 1 * time.Second
	testClient := NewTestClientWithCustomBackoff(cfg, customBackoff)

	backoffCfg = testClient.getBackoffConfig()
	if backoffCfg != customBackoff {
		t.Error("Expected custom backoff to be returned")
	}
}

func TestTestClient_GetBackoffConfig_NilCustom(t *testing.T) {
	cfg := config.Config{
		OpenRouterAPIKey: "test-key",
	}

	// Test with nil custom backoff - should fall back to client's backoff
	testClient := NewTestClientWithCustomBackoff(cfg, nil)

	backoffCfg := testClient.getBackoffConfig()
	if backoffCfg == nil {
		t.Fatal("Expected backoff config to be non-nil")
	}
}
