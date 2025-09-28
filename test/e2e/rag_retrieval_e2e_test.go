//go:build e2e

package e2e_test

import (
	"context"
	"net/http"
	"os"
	"testing"
	"time"
	"strings"

	"github.com/stretchr/testify/require"
	realai "github.com/fairyhunter13/ai-cv-evaluator/internal/adapter/ai/real"
	qdrantcli "github.com/fairyhunter13/ai-cv-evaluator/internal/adapter/vector/qdrant"
	"github.com/fairyhunter13/ai-cv-evaluator/internal/ragseed"
	"github.com/fairyhunter13/ai-cv-evaluator/internal/config"
)

func TestE2E_RAGRetrieval_ReturnsSeededChunks(t *testing.T) {
	qdrantURL := getenv("QDRANT_URL", "http://localhost:6333")
	cli := &http.Client{Timeout: 5 * time.Second}
	resp, err := cli.Get(qdrantURL + "/collections")
	if err != nil || resp.StatusCode < 200 || resp.StatusCode >= 300 {
		t.Skipf("qdrant not available at %s", qdrantURL)
	}
	if resp != nil { _ = resp.Body.Close() }

	// Require OpenAI API key for real embeddings
	if os.Getenv("OPENAI_API_KEY") == "" {
		t.Skip("OPENAI_API_KEY not set; skipping RAG retrieval E2E with real embeddings")
	}

	ctx := context.Background()
	q := qdrantcli.New(qdrantURL, os.Getenv("QDRANT_API_KEY"))
	cfg, _ := config.Load()
	ai := realai.New(cfg)

	// Seed default corpora
	require.NoError(t, ragseed.SeedDefault(ctx, q, ai))

	// Canonical queries (aligned with README/e2e)
	jobDesc := "Backend engineer building APIs, DBs, cloud, prompt design, chaining and RAG."
	brief := "Evaluate CV and project implementing LLM workflows, retries, and observability."
	vecs, err := ai.Embed(ctx, []string{jobDesc, brief})
	require.NoError(t, err)
	require.GreaterOrEqual(t, len(vecs), 2)

	// Search job description collection
	rsJob, err := q.Search(ctx, "job_description", vecs[0], 8)
	require.NoError(t, err)
	require.NotEmpty(t, rsJob)
	// At least one chunk must mention 'Product Engineer (Backend)' or 'AI prompts' to indicate correct seed hits
	foundJobSignal := false
	for _, r := range rsJob {
		if pl, ok := r["payload"].(map[string]any); ok {
			if t, ok2 := pl["text"].(string); ok2 {
				if containsAny(t, []string{"Product Engineer (Backend)", "AI prompts", "LLM chaining"}) { foundJobSignal = true; break }
			}
		}
	}
	require.True(t, foundJobSignal, "expected job description context not retrieved")

	// Search scoring rubric collection
	rsRub, err := q.Search(ctx, "scoring_rubric", vecs[1], 8)
	require.NoError(t, err)
	require.NotEmpty(t, rsRub)
	// Expect entries like 'Correctness (30%)' or 'Code Quality & Structure (25%)'
	foundRubricSignal := false
	for _, r := range rsRub {
		if pl, ok := r["payload"].(map[string]any); ok {
			if t, ok2 := pl["text"].(string); ok2 {
				if containsAny(t, []string{"Correctness (30%)", "Code Quality & Structure (25%)", "Resilience & Error Handling (20%)"}) { foundRubricSignal = true; break }
			}
		}
	}
	require.True(t, foundRubricSignal, "expected rubric context not retrieved")
}

func containsAny(s string, needles []string) bool {
    for _, n := range needles { if strings.Contains(s, n) { return true } }
    return false
}

func getenv(k, def string) string { if v := os.Getenv(k); v != "" { return v }; return def }
