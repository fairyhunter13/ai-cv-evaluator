package ragseed_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	qdrantcli "github.com/fairyhunter13/ai-cv-evaluator/internal/adapter/vector/qdrant"
	"github.com/fairyhunter13/ai-cv-evaluator/internal/domain"
	"github.com/fairyhunter13/ai-cv-evaluator/internal/ragseed"
	"github.com/stretchr/testify/require"
)

type sdAI struct{}

func (sdAI) Embed(_ domain.Context, texts []string) ([][]float32, error) {
	vecs := make([][]float32, len(texts))
	for i := range texts {
		vecs[i] = []float32{1, 2, 3}
	}
	return vecs, nil
}
func (sdAI) ChatJSON(_ domain.Context, _ string, _ string, _ int) (string, error) { return "{}", nil }
func (sdAI) CleanCoTResponse(_ domain.Context, response string) (string, error)   { return response, nil }

func TestSeedDefault_Smoke(t *testing.T) {
	// Ensure relative files exist for SeedDefault
	require.NoError(t, os.MkdirAll("configs/rag", 0o750))
	require.NoError(t, os.WriteFile("configs/rag/job_description.yaml", []byte("items: [\"a\"]\n"), 0o600))
	require.NoError(t, os.WriteFile("configs/rag/scoring_rubric.yaml", []byte("texts: [\"b\"]\n"), 0o600))
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPut && (r.URL.Path == "/collections/job_description/points" || r.URL.Path == "/collections/scoring_rubric/points") {
			w.WriteHeader(200)
			return
		}
		w.WriteHeader(200)
	}))
	defer ts.Close()
	q := qdrantcli.New(ts.URL, "")
	if err := ragseed.SeedDefault(context.Background(), q, sdAI{}); err != nil {
		t.Fatalf("seed default: %v", err)
	}
}
