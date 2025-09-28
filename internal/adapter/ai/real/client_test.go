package real

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"context"

	"github.com/fairyhunter13/ai-cv-evaluator/internal/config"
)

type chatReq struct {
	Model   string              `json:"model"`
	Models  []string            `json:"models"`
	Messages []map[string]string `json:"messages"`
}

type embedReq struct {
	Model string     `json:"model"`
	Input []string   `json:"input"`
}

func TestChatJSON_UsesAutoAndFallbacks(t *testing.T) {
	// Chat test server
	chatTS := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/chat/completions" { t.Fatalf("unexpected path: %s", r.URL.Path) }
		var cr chatReq
		_ = json.NewDecoder(r.Body).Decode(&cr)
		if cr.Model != "openrouter/auto" { t.Fatalf("expected auto model, got %q", cr.Model) }
		if len(cr.Models) != 2 { t.Fatalf("expected 2 fallback models, got %d", len(cr.Models)) }
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(200)
		_ = json.NewEncoder(w).Encode(map[string]any{
			"choices": []map[string]any{{"message": map[string]any{"content": "{\"ok\":true}"}}},
		})
	}))
	defer chatTS.Close()

	cfg := config.Config{
		OpenRouterAPIKey:  "x",
		OpenRouterBaseURL: chatTS.URL,
		ChatModel:         "", // trigger auto
		ChatFallbackModels: []string{"openai/gpt-4o-mini", "gryphe/mythomax-l2-13b"},
	}
	c := New(cfg)
	out, err := c.ChatJSON(context.Background(), "sys", "user", 64)
	if err != nil { t.Fatalf("chat err: %v", err) }
	if out != "{\"ok\":true}" { t.Fatalf("unexpected chat out: %q", out) }
}

func TestEmbed_ConvertsFloats(t *testing.T) {
	embedTS := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/embeddings" { t.Fatalf("unexpected path: %s", r.URL.Path) }
		var er embedReq
		_ = json.NewDecoder(r.Body).Decode(&er)
		if er.Model == "" || len(er.Input) != 2 { t.Fatalf("unexpected embed req: %+v", er) }
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(200)
		_ = json.NewEncoder(w).Encode(map[string]any{
			"data": []map[string]any{{"embedding": []float64{0.1, 0.2, 0.3}}, {"embedding": []float64{0.4}}},
		})
	}))
	defer embedTS.Close()

	cfg := config.Config{
		OpenAIAPIKey:  "y",
		OpenAIBaseURL: embedTS.URL,
		EmbeddingsModel: "text-embedding-3-small",
	}
	c := New(cfg)
	vecs, err := c.Embed(context.Background(), []string{"a", "b"})
	if err != nil { t.Fatalf("embed err: %v", err) }
	if len(vecs) != 2 || len(vecs[0]) != 3 || len(vecs[1]) != 1 { t.Fatalf("unexpected vecs: %#v", vecs) }
}
