package ai

import (
	"context"
	"testing"

	"github.com/fairyhunter13/ai-cv-evaluator/internal/domain"
)

type chatAI struct{ chatCalls int }

func (c *chatAI) Embed(_ domain.Context, texts []string) ([][]float32, error) {
	out := make([][]float32, len(texts))
	for i := range texts {
		out[i] = []float32{1, 2, 3}
	}
	return out, nil
}
func (c *chatAI) ChatJSON(_ domain.Context, _ string, _ string, _ int) (string, error) {
	c.chatCalls++
	return "{}", nil
}

func (c *chatAI) CleanCoTResponse(_ domain.Context, response string) (string, error) {
	return response, nil
}

func (c *chatAI) ChatJSONWithRetry(_ domain.Context, _ string, _ string, _ int) (string, error) {
	return "{}", nil
}

func Test_ChatJSON_Passthrough(t *testing.T) {
	base := &chatAI{}
	wrapped := NewEmbedCache(base, 4)
	_, _ = wrapped.ChatJSON(context.Background(), "sys", "user", 10)
	if base.chatCalls != 1 {
		t.Fatalf("expected passthrough chat call")
	}
}

type evictAI struct{ calls int }

func (e *evictAI) Embed(_ domain.Context, texts []string) ([][]float32, error) {
	e.calls++
	out := make([][]float32, len(texts))
	for i := range texts {
		out[i] = []float32{1, 2, 3}
	}
	return out, nil
}
func (e *evictAI) ChatJSON(_ domain.Context, _ string, _ string, _ int) (string, error) {
	return "{}", nil
}

func (e *evictAI) CleanCoTResponse(_ domain.Context, response string) (string, error) {
	return response, nil
}

func (e *evictAI) ChatJSONWithRetry(_ domain.Context, _ string, _ string, _ int) (string, error) {
	return "{}", nil
}

func Test_EmbedCache_EvictionFIFO(t *testing.T) {
	base := &evictAI{}
	wrapped := NewEmbedCache(base, 1)                         // small cap
	_, _ = wrapped.Embed(context.Background(), []string{"a"}) // miss, cached
	_, _ = wrapped.Embed(context.Background(), []string{"b"}) // evicts "a"
	_, _ = wrapped.Embed(context.Background(), []string{"b"}) // hit
	_, _ = wrapped.Embed(context.Background(), []string{"a"}) // miss again due to eviction
	if base.calls < 3 {
		t.Fatalf("expected >=3 base embeds due to eviction, got %d", base.calls)
	}
}
