package real

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/fairyhunter13/ai-cv-evaluator/internal/config"
)

type chatReq struct {
	Model    string              `json:"model"`
	Models   []string            `json:"models"`
	Messages []map[string]string `json:"messages"`
}

type embedReq struct {
	Model string   `json:"model"`
	Input []string `json:"input"`
}

func TestChatJSON_UsesAutoAndFallbacks(t *testing.T) {
	// Test server that handles both /models and /chat/completions
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")

		if r.URL.Path == "/models" {
			// Mock OpenRouter models API response
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
			var cr chatReq
			_ = json.NewDecoder(r.Body).Decode(&cr)
			if cr.Model != "meta-llama/llama-3.1-8b-instruct:free" {
				t.Fatalf("expected free model, got %q", cr.Model)
			}
			_ = json.NewEncoder(w).Encode(map[string]any{
				"choices": []map[string]any{{"message": map[string]any{"content": "{\"ok\":true}"}}},
			})
			return
		}

		t.Fatalf("unexpected path: %s", r.URL.Path)
	}))
	defer server.Close()

	cfg := config.Config{
		OpenRouterAPIKey:  "x",
		OpenRouterBaseURL: server.URL,
	}
	c := NewTestClient(cfg)
	out, err := c.ChatJSON(context.Background(), "sys", "user", 64)
	if err != nil {
		t.Fatalf("chat err: %v", err)
	}
	if out != "{\"ok\":true}" {
		t.Fatalf("unexpected chat out: %q", out)
	}
}

func TestEmbed_ConvertsFloats(t *testing.T) {
	embedTS := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/embeddings" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		var er embedReq
		_ = json.NewDecoder(r.Body).Decode(&er)
		if er.Model == "" || len(er.Input) != 2 {
			t.Fatalf("unexpected embed req: %+v", er)
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(200)
		_ = json.NewEncoder(w).Encode(map[string]any{
			"data": []map[string]any{{"embedding": []float64{0.1, 0.2, 0.3}}, {"embedding": []float64{0.4}}},
		})
	}))
	defer embedTS.Close()

	cfg := config.Config{
		OpenAIAPIKey:    "y",
		OpenAIBaseURL:   embedTS.URL,
		EmbeddingsModel: "text-embedding-3-small",
	}
	c := NewTestClient(cfg)
	vecs, err := c.Embed(context.Background(), []string{"a", "b"})
	if err != nil {
		t.Fatalf("embed err: %v", err)
	}
	if len(vecs) != 2 || len(vecs[0]) != 3 || len(vecs[1]) != 1 {
		t.Fatalf("unexpected vecs: %#v", vecs)
	}
}
