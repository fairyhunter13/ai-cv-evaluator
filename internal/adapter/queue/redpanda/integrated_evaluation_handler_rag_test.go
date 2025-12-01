package redpanda

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	qdrantcli "github.com/fairyhunter13/ai-cv-evaluator/internal/adapter/vector/qdrant"
	"github.com/fairyhunter13/ai-cv-evaluator/internal/domain"
	"github.com/stretchr/testify/require"
)

// ragTestAI is a minimal AIClient implementation used for testing RAG context
// retrieval. It only needs to support Embed; chat methods are stubbed.
type ragTestAI struct{}

func (ragTestAI) Embed(_ domain.Context, texts []string) ([][]float32, error) {
	vecs := make([][]float32, len(texts))
	for i := range vecs {
		vecs[i] = []float32{0.1, 0.2, 0.3}
	}
	return vecs, nil
}

func (ragTestAI) ChatJSON(_ domain.Context, _ string, _ string, _ int) (string, error) {
	return "{}", nil
}

func (ragTestAI) ChatJSONWithRetry(_ domain.Context, _ string, _ string, _ int) (string, error) {
	return "{}", nil
}

func (ragTestAI) CleanCoTResponse(_ domain.Context, _ string) (string, error) {
	return "{}", nil
}

// TestIntegratedEvaluationHandler_RetrieveEnhancedRAGContext verifies that the
// handler can successfully retrieve and combine job description and scoring
// rubric context from Qdrant and format it into the expected string shape.
func TestIntegratedEvaluationHandler_RetrieveEnhancedRAGContext(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodPost && strings.HasPrefix(r.URL.Path, "/collections/job_description/points/search"):
			_, _ = w.Write([]byte(`{"result":[{"payload":{"text":"Job RAG snippet 1"}},{"payload":{"text":"Job RAG snippet 2"}}]}`))
		case r.Method == http.MethodPost && strings.HasPrefix(r.URL.Path, "/collections/scoring_rubric/points/search"):
			_, _ = w.Write([]byte(`{"result":[{"payload":{"text":"Rubric RAG snippet A"}}]}`))
		default:
			_, _ = w.Write([]byte(`{"result":[]}`))
		}
	}))
	defer ts.Close()

	q := qdrantcli.New(ts.URL, "")
	h := &IntegratedEvaluationHandler{
		ai: ragTestAI{},
		q:  q,
	}

	ctx := context.Background()
	ctxStr, err := h.retrieveEnhancedRAGContext(ctx, "sample query", "Job description text", "Study case brief")
	require.NoError(t, err)
	require.NotEmpty(t, ctxStr)
	require.Contains(t, ctxStr, "Job Context: Job RAG snippet 1")
	require.Contains(t, ctxStr, "Scoring Criteria: Rubric RAG snippet A")
}
