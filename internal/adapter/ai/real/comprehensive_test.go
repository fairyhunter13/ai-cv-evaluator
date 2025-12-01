package real

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/fairyhunter13/ai-cv-evaluator/internal/config"
)

func TestChatJSONWithRetry_Success(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")

		if r.URL.Path == "/models" {
			_ = json.NewEncoder(w).Encode(map[string]any{
				"data": []map[string]any{
					{
						"id": "test-model-1",
						"pricing": map[string]string{
							"prompt":     "0",
							"completion": "0",
							"request":    "0",
							"image":      "0",
						},
					},
					{
						"id": "test-model-2",
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
				"model": "test-model-1",
				"choices": []map[string]any{
					{"message": map[string]any{"content": "The AI response for this test is a detailed and realistic one, designed to simulate a real-world conversation. It should not be flagged as a low-quality or refusal response by the detectRefusalWithValidation function."}},
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

	result, err := client.ChatJSONWithRetry(context.Background(), "system", "user", 100)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result != "The AI response for this test is a detailed and realistic one, designed to simulate a real-world conversation. It should not be flagged as a low-quality or refusal response by the detectRefusalWithValidation function." {
		t.Fatalf("unexpected result: %s", result)
	}
}

func TestChatJSONWithRetry_AllModelsFail(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")

		if r.URL.Path == "/models" {
			_ = json.NewEncoder(w).Encode(map[string]any{
				"data": []map[string]any{
					{
						"id": "test-model-1",
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
			w.WriteHeader(http.StatusInternalServerError)
			_ = json.NewEncoder(w).Encode(map[string]any{
				"error": "internal server error",
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

	_, err := client.ChatJSONWithRetry(context.Background(), "system", "user", 100)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "all models failed") {
		t.Fatalf("unexpected error message: %v", err)
	}
}

func TestCallOpenRouterWithModel_Success(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"model": "test-model",
			"choices": []map[string]any{
				{"message": map[string]any{"content": "test response"}},
			},
		})
	}))
	defer server.Close()

	cfg := config.Config{
		OpenRouterAPIKey:  "test-key",
		OpenRouterBaseURL: server.URL,
	}
	client := NewTestClient(cfg)

	result, err := client.callOpenRouterWithModel(context.Background(), "test-model", "system", "user", 100)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result != "test response" {
		t.Fatalf("unexpected result: %s", result)
	}
}

func TestCallOpenRouterWithModel_UsesBothAPIKeysWhenConfigured(t *testing.T) {
	authHeaders := make([]string, 0, 4)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/chat/completions" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		authHeaders = append(authHeaders, r.Header.Get("Authorization"))
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"model": "test-model",
			"choices": []map[string]any{
				{"message": map[string]any{"content": "test response"}},
			},
		})
	}))
	defer server.Close()

	cfg := config.Config{
		OpenRouterAPIKey:      "primary-key",
		OpenRouterAPIKey2:     "secondary-key",
		OpenRouterBaseURL:     server.URL,
		OpenRouterMinInterval: 0,
	}
	client := NewTestClient(cfg)

	for i := 0; i < 4; i++ {
		result, err := client.callOpenRouterWithModel(context.Background(), "test-model", "system", "user", 100)
		if err != nil {
			t.Fatalf("unexpected error on call %d: %v", i, err)
		}
		if result != "test response" {
			t.Fatalf("unexpected result on call %d: %s", i, result)
		}
	}

	if len(authHeaders) != 4 {
		t.Fatalf("expected 4 calls, got %d", len(authHeaders))
	}
	seenPrimary := false
	seenSecondary := false
	for _, h := range authHeaders {
		switch h {
		case "Bearer primary-key":
			seenPrimary = true
		case "Bearer secondary-key":
			seenSecondary = true
		default:
			t.Fatalf("unexpected Authorization header: %s", h)
		}
	}
	if !seenPrimary || !seenSecondary {
		t.Fatalf("expected both primary and secondary keys to be used, got %v", authHeaders)
	}
}

func TestCallOpenRouterWithModel_EmptyChoices(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"model":   "test-model",
			"choices": []map[string]any{},
		})
	}))
	defer server.Close()

	cfg := config.Config{
		OpenRouterAPIKey:  "test-key",
		OpenRouterBaseURL: server.URL,
	}
	client := NewTestClient(cfg)

	_, err := client.callOpenRouterWithModel(context.Background(), "test-model", "system", "user", 100)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "empty choices") {
		t.Fatalf("unexpected error message: %v", err)
	}
}

func TestCallOpenRouterWithModel_4xxError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"error": "bad request",
		})
	}))
	defer server.Close()

	cfg := config.Config{
		OpenRouterAPIKey:  "test-key",
		OpenRouterBaseURL: server.URL,
	}
	client := NewTestClient(cfg)

	_, err := client.callOpenRouterWithModel(context.Background(), "test-model", "system", "user", 100)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "chat status 400") {
		t.Fatalf("unexpected error message: %v", err)
	}
}

func TestCallOpenRouterWithModel_5xxError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"error": "internal server error",
		})
	}))
	defer server.Close()

	cfg := config.Config{
		OpenRouterAPIKey:  "test-key",
		OpenRouterBaseURL: server.URL,
	}
	client := NewTestClient(cfg)

	_, err := client.callOpenRouterWithModel(context.Background(), "test-model", "system", "user", 100)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "chat status 500") {
		t.Fatalf("unexpected error message: %v", err)
	}
}

func TestCallOpenRouterWithModel_RateLimit(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusTooManyRequests)
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"error": "rate limited",
		})
	}))
	defer server.Close()

	cfg := config.Config{
		OpenRouterAPIKey:  "test-key",
		OpenRouterBaseURL: server.URL,
	}
	client := NewTestClient(cfg)

	_, err := client.callOpenRouterWithModel(context.Background(), "test-model", "system", "user", 100)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "rate limited") {
		t.Fatalf("unexpected error message: %v", err)
	}
}

func TestCallOpenRouterWithModel_InvalidJSON(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte("invalid json"))
	}))
	defer server.Close()

	cfg := config.Config{
		OpenRouterAPIKey:  "test-key",
		OpenRouterBaseURL: server.URL,
	}
	client := NewTestClient(cfg)

	_, err := client.callOpenRouterWithModel(context.Background(), "test-model", "system", "user", 100)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "decode error") && !strings.Contains(err.Error(), "invalid character") {
		t.Fatalf("unexpected error message: %v", err)
	}
}

func TestCallOpenRouterWithModel_NetworkError(t *testing.T) {
	cfg := config.Config{
		OpenRouterAPIKey:  "test-key",
		OpenRouterBaseURL: "http://invalid-url-that-does-not-exist",
	}
	client := NewTestClient(cfg)

	_, err := client.callOpenRouterWithModel(context.Background(), "test-model", "system", "user", 100)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "failed") {
		t.Fatalf("unexpected error message: %v", err)
	}
}

func TestCallGroqChat_Success(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/chat/completions" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"choices": []map[string]any{
				{"message": map[string]any{"content": "groq test response"}},
			},
		})
	}))
	defer server.Close()

	cfg := config.Config{
		GroqAPIKey:  "test-groq-key",
		GroqBaseURL: server.URL,
	}
	client := NewTestClient(cfg)

	result, err := client.callGroqChat(context.Background(), cfg.GroqAPIKey, "system", "user", 100)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result != "groq test response" {
		t.Fatalf("unexpected result: %s", result)
	}
}

func TestCallGroqChat_TriesNextModelOn4xx(t *testing.T) {
	modelsTried := make([]string, 0, 2)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/chat/completions" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		var body map[string]any
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Fatalf("failed to decode request body: %v", err)
		}
		model, _ := body["model"].(string)
		modelsTried = append(modelsTried, model)

		switch model {
		case "llama-3.1-8b-instant":
			w.WriteHeader(http.StatusBadRequest)
			_ = json.NewEncoder(w).Encode(map[string]any{
				"error": "bad request",
			})
		case "llama-3.3-70b-versatile":
			_ = json.NewEncoder(w).Encode(map[string]any{
				"choices": []map[string]any{
					{"message": map[string]any{"content": "fallback groq response"}},
				},
			})
		default:
			t.Fatalf("unexpected model: %s", model)
		}
	}))
	defer server.Close()

	cfg := config.Config{
		GroqAPIKey:  "test-groq-key",
		GroqBaseURL: server.URL,
	}
	client := NewTestClient(cfg)

	result, err := client.callGroqChat(context.Background(), cfg.GroqAPIKey, "system", "user", 100)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result != "fallback groq response" {
		t.Fatalf("unexpected result: %s", result)
	}
	if len(modelsTried) != 2 {
		t.Fatalf("expected 2 models to be tried, got %d (%v)", len(modelsTried), modelsTried)
	}
	if modelsTried[0] == modelsTried[1] {
		t.Fatalf("expected different models to be tried, got %v", modelsTried)
	}
}

func TestCallGroqChat_StopsOn429AndDoesNotTryOtherModels(t *testing.T) {
	modelsTried := make([]string, 0, 2)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/chat/completions" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		var body map[string]any
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Fatalf("failed to decode request body: %v", err)
		}
		model, _ := body["model"].(string)
		modelsTried = append(modelsTried, model)

		w.WriteHeader(http.StatusTooManyRequests)
		_ = json.NewEncoder(w).Encode(map[string]any{
			"error": "rate limited",
		})
	}))
	defer server.Close()

	cfg := config.Config{
		GroqAPIKey:  "test-groq-key",
		GroqBaseURL: server.URL,
	}
	client := NewTestClient(cfg)

	_, err := client.callGroqChat(context.Background(), cfg.GroqAPIKey, "system", "user", 100)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "rate limited") {
		t.Fatalf("expected rate limit error, got %v", err)
	}
	if len(modelsTried) != 1 {
		t.Fatalf("expected only 1 model to be tried due to rate limiting, got %d (%v)", len(modelsTried), modelsTried)
	}
}

func TestChatJSONWithRetry_GroqPrimaryWhenNoOpenRouterKey(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/chat/completions" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"choices": []map[string]any{
				{"message": map[string]any{"content": "groq primary response"}},
			},
		})
	}))
	defer server.Close()

	cfg := config.Config{
		GroqAPIKey:  "test-groq-key",
		GroqBaseURL: server.URL,
	}
	client := NewTestClient(cfg)

	result, err := client.ChatJSONWithRetry(context.Background(), "system", "user", 100)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result != "groq primary response" {
		t.Fatalf("unexpected result: %s", result)
	}
}

func TestCleanCoTResponse_Success(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")

		if r.URL.Path == "/models" {
			_ = json.NewEncoder(w).Encode(map[string]any{
				"data": []map[string]any{
					{
						"id": "test-model",
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
				"model": "test-model",
				"choices": []map[string]any{
					{"message": map[string]any{"content": "cleaned response"}},
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

	result, err := client.CleanCoTResponse(context.Background(), "original response with reasoning")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result != "cleaned response" {
		t.Fatalf("unexpected result: %s", result)
	}
}

func TestCleanCoTResponse_NoModels(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")

		if r.URL.Path == "/models" {
			_ = json.NewEncoder(w).Encode(map[string]any{
				"data": []map[string]any{},
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

	_, err := client.CleanCoTResponse(context.Background(), "original response")
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "no free models available") {
		t.Fatalf("unexpected error message: %v", err)
	}
}

func TestCleanCoTResponse_EmptyChoices(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")

		if r.URL.Path == "/models" {
			_ = json.NewEncoder(w).Encode(map[string]any{
				"data": []map[string]any{
					{
						"id": "test-model",
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
				"model":   "test-model",
				"choices": []map[string]any{},
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

	_, err := client.CleanCoTResponse(context.Background(), "original response")
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "empty choices") {
		t.Fatalf("unexpected error message: %v", err)
	}
}

func TestCleanCoTResponse_4xxError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")

		if r.URL.Path == "/models" {
			_ = json.NewEncoder(w).Encode(map[string]any{
				"data": []map[string]any{
					{
						"id": "test-model",
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
			w.WriteHeader(http.StatusBadRequest)
			_ = json.NewEncoder(w).Encode(map[string]any{
				"error": "bad request",
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

	_, err := client.CleanCoTResponse(context.Background(), "original response")
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "cot cleaning status 400") {
		t.Fatalf("unexpected error message: %v", err)
	}
}

func TestCleanCoTResponse_5xxError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")

		if r.URL.Path == "/models" {
			_ = json.NewEncoder(w).Encode(map[string]any{
				"data": []map[string]any{
					{
						"id": "test-model",
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
			w.WriteHeader(http.StatusInternalServerError)
			_ = json.NewEncoder(w).Encode(map[string]any{
				"error": "internal server error",
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

	_, err := client.CleanCoTResponse(context.Background(), "original response")
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "cot cleaning status 500") {
		t.Fatalf("unexpected error message: %v", err)
	}
}

func TestCleanCoTResponse_RateLimit(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")

		if r.URL.Path == "/models" {
			_ = json.NewEncoder(w).Encode(map[string]any{
				"data": []map[string]any{
					{
						"id": "test-model",
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
			w.WriteHeader(http.StatusTooManyRequests)
			_ = json.NewEncoder(w).Encode(map[string]any{
				"error": "rate limited",
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

	_, err := client.CleanCoTResponse(context.Background(), "original response")
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "rate limited") {
		t.Fatalf("unexpected error message: %v", err)
	}
}

func TestCleanCoTResponse_InvalidJSON(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")

		if r.URL.Path == "/models" {
			_ = json.NewEncoder(w).Encode(map[string]any{
				"data": []map[string]any{
					{
						"id": "test-model",
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
			_, _ = w.Write([]byte("invalid json"))
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

	_, err := client.CleanCoTResponse(context.Background(), "original response")
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "decode error") && !strings.Contains(err.Error(), "invalid character") {
		t.Fatalf("unexpected error message: %v", err)
	}
}

func TestChatJSON_ModelSubstitution(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")

		if r.URL.Path == "/models" {
			_ = json.NewEncoder(w).Encode(map[string]any{
				"data": []map[string]any{
					{
						"id": "requested-model",
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
				"model": "actual-model", // Different from requested
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
		t.Fatalf("unexpected error: %v", err)
	}
	if result != "test response" {
		t.Fatalf("unexpected result: %s", result)
	}
}

func TestEmbed_EmptyData_Comprehensive(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"data": []map[string]any{},
		})
	}))
	defer server.Close()

	cfg := config.Config{
		OpenAIAPIKey:    "test-key",
		OpenAIBaseURL:   server.URL,
		EmbeddingsModel: "text-embedding-3-small",
	}
	client := NewTestClient(cfg)

	_, err := client.Embed(context.Background(), []string{"test"})
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "empty data") {
		t.Fatalf("unexpected error message: %v", err)
	}
}

func TestEmbed_4xxError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"error": "bad request",
		})
	}))
	defer server.Close()

	cfg := config.Config{
		OpenAIAPIKey:    "test-key",
		OpenAIBaseURL:   server.URL,
		EmbeddingsModel: "text-embedding-3-small",
	}
	client := NewTestClient(cfg)

	_, err := client.Embed(context.Background(), []string{"test"})
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "embed status 400") {
		t.Fatalf("unexpected error message: %v", err)
	}
}

func TestEmbed_5xxError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"error": "internal server error",
		})
	}))
	defer server.Close()

	cfg := config.Config{
		OpenAIAPIKey:    "test-key",
		OpenAIBaseURL:   server.URL,
		EmbeddingsModel: "text-embedding-3-small",
	}
	client := NewTestClient(cfg)

	_, err := client.Embed(context.Background(), []string{"test"})
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "embed status 500") {
		t.Fatalf("unexpected error message: %v", err)
	}
}

func TestEmbed_RateLimit(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusTooManyRequests)
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"error": "rate limited",
		})
	}))
	defer server.Close()

	cfg := config.Config{
		OpenAIAPIKey:    "test-key",
		OpenAIBaseURL:   server.URL,
		EmbeddingsModel: "text-embedding-3-small",
	}
	client := NewTestClient(cfg)

	_, err := client.Embed(context.Background(), []string{"test"})
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "rate limited") {
		t.Fatalf("unexpected error message: %v", err)
	}
}

func TestEmbed_InvalidJSON_Comprehensive(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte("invalid json"))
	}))
	defer server.Close()

	cfg := config.Config{
		OpenAIAPIKey:    "test-key",
		OpenAIBaseURL:   server.URL,
		EmbeddingsModel: "text-embedding-3-small",
	}
	client := NewTestClient(cfg)

	_, err := client.Embed(context.Background(), []string{"test"})
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "decode error") && !strings.Contains(err.Error(), "invalid character") {
		t.Fatalf("unexpected error message: %v", err)
	}
}

func TestEmbed_NetworkError(t *testing.T) {
	cfg := config.Config{
		OpenAIAPIKey:    "test-key",
		OpenAIBaseURL:   "http://invalid-url-that-does-not-exist",
		EmbeddingsModel: "text-embedding-3-small",
	}
	client := NewTestClient(cfg)

	_, err := client.Embed(context.Background(), []string{"test"})
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "failed") {
		t.Fatalf("unexpected error message: %v", err)
	}
}

func TestGetBackoffConfig(t *testing.T) {
	cfg := config.Config{
		AppEnv: "test",
	}
	client := NewTestClient(cfg)

	backoff := client.getBackoffConfig()
	if backoff == nil {
		t.Fatal("expected backoff config, got nil")
	}
	if backoff.MaxElapsedTime != 5*time.Second {
		t.Fatalf("expected MaxElapsedTime to be 5s, got %v", backoff.MaxElapsedTime)
	}
}

func TestNew_DevEnvironment(t *testing.T) {
	cfg := config.Config{
		AppEnv: "dev",
	}
	client := New(cfg)

	if client.chatHC.Timeout != 90*time.Second {
		t.Fatalf("expected chat timeout to be 90s in dev, got %v", client.chatHC.Timeout)
	}
	if client.embedHC.Timeout != 60*time.Second {
		t.Fatalf("expected embed timeout to be 60s in dev, got %v", client.embedHC.Timeout)
	}
}

func TestNew_ProductionEnvironment(t *testing.T) {
	cfg := config.Config{
		AppEnv: "production",
	}
	client := New(cfg)

	if client.chatHC.Timeout != 60*time.Second {
		t.Fatalf("expected chat timeout to be 60s in production, got %v", client.chatHC.Timeout)
	}
	if client.embedHC.Timeout != 30*time.Second {
		t.Fatalf("expected embed timeout to be 30s in production, got %v", client.embedHC.Timeout)
	}
}
