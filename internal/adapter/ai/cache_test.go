package ai

import (
	"context"
	"testing"

	"github.com/fairyhunter13/ai-cv-evaluator/internal/domain"
)

type fakeAI struct{ embedCalls int }

func (f *fakeAI) Embed(_ domain.Context, texts []string) ([][]float32, error) {
	f.embedCalls++
	out := make([][]float32, len(texts))
	for i := range texts {
		out[i] = []float32{1, 2, 3}
	}
	return out, nil
}
func (f *fakeAI) ChatJSON(_ domain.Context, _ string, _ string, _ int) (string, error) {
	return "{}", nil
}

func (f *fakeAI) CleanCoTResponse(_ domain.Context, response string) (string, error) {
	return response, nil
}

func Test_NewEmbedCache_UsesCache(t *testing.T) {
	base := &fakeAI{}
	wrapped := NewEmbedCache(base, 8)
	ctx := context.Background()
	texts := []string{"hello", "world"}
	_, err := wrapped.Embed(ctx, texts)
	if err != nil {
		t.Fatalf("first embed: %v", err)
	}
	_, err = wrapped.Embed(ctx, texts)
	if err != nil {
		t.Fatalf("second embed: %v", err)
	}
	if base.embedCalls != 1 {
		t.Fatalf("expected 1 base embed call, got %d", base.embedCalls)
	}
}
