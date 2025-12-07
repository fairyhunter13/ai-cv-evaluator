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

func Test_NewEmbedCache_ZeroCapacity(t *testing.T) {
	base := &evictAI{}
	// Zero capacity should return base unmodified
	wrapped := NewEmbedCache(base, 0)
	if wrapped != base {
		t.Fatalf("expected base to be returned for zero capacity")
	}
}

func Test_NewEmbedCache_NegativeCapacity(t *testing.T) {
	base := &evictAI{}
	// Negative capacity should return base unmodified
	wrapped := NewEmbedCache(base, -1)
	if wrapped != base {
		t.Fatalf("expected base to be returned for negative capacity")
	}
}

func Test_NewEmbedCache_NilBase(t *testing.T) {
	// Nil base should return nil
	wrapped := NewEmbedCache(nil, 10)
	if wrapped != nil {
		t.Fatalf("expected nil to be returned for nil base")
	}
}

func Test_EmbedCache_MultipleMisses(t *testing.T) {
	base := &evictAI{}
	wrapped := NewEmbedCache(base, 10)
	ctx := context.Background()

	// Multiple texts in one call - all misses
	_, err := wrapped.Embed(ctx, []string{"a", "b", "c"})
	if err != nil {
		t.Fatalf("embed failed: %v", err)
	}
	if base.calls != 1 {
		t.Fatalf("expected 1 base call, got %d", base.calls)
	}

	// Same texts again - all hits
	_, err = wrapped.Embed(ctx, []string{"a", "b", "c"})
	if err != nil {
		t.Fatalf("embed failed: %v", err)
	}
	if base.calls != 1 {
		t.Fatalf("expected still 1 base call (cache hit), got %d", base.calls)
	}
}

func Test_EmbedCache_PartialHits(t *testing.T) {
	base := &evictAI{}
	wrapped := NewEmbedCache(base, 10)
	ctx := context.Background()

	// First call - all misses
	_, _ = wrapped.Embed(ctx, []string{"a", "b"})
	if base.calls != 1 {
		t.Fatalf("expected 1 base call, got %d", base.calls)
	}

	// Second call - partial hits (a, b are cached, c is miss)
	_, _ = wrapped.Embed(ctx, []string{"a", "c", "b"})
	if base.calls != 2 {
		t.Fatalf("expected 2 base calls (partial miss), got %d", base.calls)
	}
}

func Test_EmbedCache_UpdateExisting(t *testing.T) {
	base := &evictAI{}
	wrapped := NewEmbedCache(base, 10)
	ctx := context.Background()

	// First call - cache "a"
	_, _ = wrapped.Embed(ctx, []string{"a"})
	if base.calls != 1 {
		t.Fatalf("expected 1 base call, got %d", base.calls)
	}

	// Manually trigger put with same key to test update path
	// This is tested indirectly through the cache behavior
	_, _ = wrapped.Embed(ctx, []string{"a"})
	if base.calls != 1 {
		t.Fatalf("expected still 1 base call (cache hit), got %d", base.calls)
	}
}

func Test_ChatJSONWithRetry_Passthrough(t *testing.T) {
	base := &chatAI{}
	wrapped := NewEmbedCache(base, 4)
	result, err := wrapped.ChatJSONWithRetry(context.Background(), "sys", "user", 10)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result != "{}" {
		t.Fatalf("expected passthrough response, got %s", result)
	}
}

func Test_CleanCoTResponse_Passthrough(t *testing.T) {
	base := &chatAI{}
	wrapped := NewEmbedCache(base, 4)
	result, _ := wrapped.CleanCoTResponse(context.Background(), "test response")
	if result != "test response" {
		t.Fatalf("expected passthrough response, got %s", result)
	}
}
