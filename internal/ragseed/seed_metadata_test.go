package ragseed_test

import (
	"context"
	"encoding/json"
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

type metaAI struct{}

func (s metaAI) Embed(_ domain.Context, texts []string) ([][]float32, error) {
	vecs := make([][]float32, len(texts))
	for i := range texts {
		vecs[i] = []float32{1, 2, 3}
	}
	return vecs, nil
}

func (s metaAI) ChatJSON(_ domain.Context, _ string, _ string, _ int) (string, error) {
	return "{}", nil
}
func (s metaAI) CleanCoTResponse(_ domain.Context, response string) (string, error) {
	return response, nil
}

func TestSeedFile_MetadataMapping(t *testing.T) {
	t.Setenv("RAGSEED_ALLOW_ABSPATHS", "1")
	var captured []map[string]any
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPut && r.URL.Path == "/collections/coll/points" {
			var payload struct {
				Points []map[string]any `json:"points"`
			}
			_ = json.NewDecoder(r.Body).Decode(&payload)
			captured = payload.Points
			w.WriteHeader(200)
			return
		}
		w.WriteHeader(200)
	}))
	defer ts.Close()
	q := qdrantcli.New(ts.URL, "")

	dir := t.TempDir()
	p := filepath.Join(dir, "seed.yaml")
	require.NoError(t, os.WriteFile(p, []byte(`
items: []
texts: []
data:
  - text: "Doc A"
    type: "doc"
    section: "intro"
    weight: 2.5
`), 0o600))
	if err := ragseed.SeedFile(context.Background(), q, metaAI{}, p, "coll"); err != nil {
		t.Fatalf("seed: %v", err)
	}
	if len(captured) != 1 {
		t.Fatalf("want 1 point, got %d", len(captured))
	}
	pt := captured[0]
	pl, _ := pt["payload"].(map[string]any)
	if pl == nil {
		t.Fatalf("missing payload: %v", pt)
	}
	if pl["type"] != "doc" || pl["section"] != "intro" {
		t.Fatalf("missing metadata: %v", pt)
	}
	if _, ok := pl["weight"]; !ok {
		t.Fatalf("missing weight")
	}
	if _, ok := pl["weight"]; !ok {
		t.Fatalf("missing weight")
	}
}
