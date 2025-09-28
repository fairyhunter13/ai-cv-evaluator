package ragseed_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	qdrantcli "github.com/fairyhunter13/ai-cv-evaluator/internal/adapter/vector/qdrant"
	"github.com/fairyhunter13/ai-cv-evaluator/internal/domain"
	"github.com/fairyhunter13/ai-cv-evaluator/internal/ragseed"
	"github.com/stretchr/testify/require"
)

type stubAI struct{}
func (s stubAI) Embed(_ domain.Context, texts []string) ([][]float32, error) {
	vecs := make([][]float32, len(texts))
	for i := range texts { vecs[i] = []float32{1,2,3} }
	return vecs, nil
}
func (s stubAI) ChatJSON(_ domain.Context, _ string, _ string, _ int) (string, error) { return "{}", nil }

func TestSeedFile_ItemsTextsData(t *testing.T) {
	t.Setenv("RAGSEED_ALLOW_ABSPATHS", "1")
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPut && strings.HasSuffix(r.URL.Path, "/collections/test/points") {
			var payload struct{ Points []map[string]any `json:"points"` }
			_ = json.NewDecoder(r.Body).Decode(&payload)
			if len(payload.Points) != 4 {
				t.Fatalf("expected 4 points, got %d", len(payload.Points))
			}
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
items: ["a","b"]
texts: ["c"]
data:
  - text: "d"
    type: "doc"
    section: "s1"
    weight: 1.0
`), 0o600))

	if err := ragseed.SeedFile(context.Background(), q, stubAI{}, p, "test"); err != nil {
		t.Fatalf("seed err: %v", err)
	}
}

func TestSeedFile_ListFallback(t *testing.T) {
	t.Setenv("RAGSEED_ALLOW_ABSPATHS", "1")
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(200) }))
	defer ts.Close()
	q := qdrantcli.New(ts.URL, "")

	dir := t.TempDir()
	p := filepath.Join(dir, "seed.yaml")
	require.NoError(t, os.WriteFile(p, []byte("- one\n- two\n"), 0o600))
	if err := ragseed.SeedFile(context.Background(), q, stubAI{}, p, "test"); err != nil {
		t.Fatalf("seed err: %v", err)
	}
}

func TestSeedFile_NotFound(t *testing.T) {
	q := qdrantcli.New("http://127.0.0.1", "")
	if err := ragseed.SeedFile(context.Background(), q, stubAI{}, "/does/not/exist.yaml", "c"); err == nil {
		t.Fatalf("expected error for missing file")
	}
}
