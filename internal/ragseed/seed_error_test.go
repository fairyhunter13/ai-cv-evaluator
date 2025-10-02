package ragseed_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	qdrantcli "github.com/fairyhunter13/ai-cv-evaluator/internal/adapter/vector/qdrant"
	"github.com/fairyhunter13/ai-cv-evaluator/internal/domain"
	"github.com/fairyhunter13/ai-cv-evaluator/internal/ragseed"
	"github.com/stretchr/testify/require"
)

type errAI struct{}

func (s errAI) Embed(_ domain.Context, texts []string) ([][]float32, error) {
	vecs := make([][]float32, len(texts))
	for i := range texts {
		vecs[i] = []float32{1, 2, 3}
	}
	return vecs, nil
}
func (s errAI) ChatJSON(_ domain.Context, _ string, _ string, _ int) (string, error) {
	return "{}", nil
}
func (s errAI) ChatJSONWithRetry(_ domain.Context, _ string, _ string, _ int) (string, error) {
	return "{}", nil
}
func (s errAI) CleanCoTResponse(_ domain.Context, response string) (string, error) {
	return response, nil
}

func TestSeedFile_UpsertError(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPut {
			w.WriteHeader(500)
			return
		}
		w.WriteHeader(200)
	}))
	defer ts.Close()
	q := qdrantcli.New(ts.URL, "")

	dir := t.TempDir()
	p := filepath.Join(dir, "seed.yaml")
	require.NoError(t, os.WriteFile(p, []byte("items: [\"x\"]\n"), 0o600))
	if err := ragseed.SeedFile(context.Background(), q, errAI{}, p, "test"); err == nil {
		t.Fatalf("expected error due to upsert failure")
	}
}
