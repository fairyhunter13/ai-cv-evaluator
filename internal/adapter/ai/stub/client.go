//go:build ignore

package stub

import (
	"encoding/json"
	"time"

	"github.com/fairyhunter13/ai-cv-evaluator/internal/domain"
)

// Client is a fast, deterministic AI client for local/testing.
// Disabled from builds via the build tag above. E2E uses live providers only.
type Client struct{}

func New() *Client { return &Client{} }

// Embed returns simple small vectors deterministically.
func (c *Client) Embed(_ domain.Context, texts []string) ([][]float32, error) {
	res := make([][]float32, len(texts))
	for i := range texts {
		res[i] = []float32{0.1, 0.2, 0.3}
	}
	return res, nil
}

// ChatJSON returns a compact JSON string matching the expected schema.
func (c *Client) ChatJSON(_ domain.Context, _ string, _ string, _ int) (string, error) {
	// Simulate a tiny bit of processing latency to resemble real work
	time.Sleep(50 * time.Millisecond)
	payload := map[string]any{
		"cv_match_rate":    0.87,
		"cv_feedback":      "Strong alignment with backend and cloud experience.",
		"project_score":    8.2,
		"project_feedback": "Good microservices design; observability in place.",
		"overall_summary":  "Solid fit for the role with relevant LLM workflow experience.",
	}
	b, _ := json.Marshal(payload)
	return string(b), nil
}
