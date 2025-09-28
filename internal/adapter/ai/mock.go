//go:build ignore
// Mock AI client is disabled: project uses real providers (OpenAI/OpenRouter) only.
package ai

import (
	"crypto/sha1"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"math"
	"strings"

	"github.com/fairyhunter13/ai-cv-evaluator/internal/domain"
	"github.com/fairyhunter13/ai-cv-evaluator/internal/usecase"
)

// MockClient implements domain.AIClient deterministically for offline/mock mode.
type MockClient struct{}

// NewMockClient constructs a deterministic mock AI client.
func NewMockClient() domain.AIClient { return &MockClient{} }

// Embed returns a deterministic vector of size 1536 for each input text.
func (m *MockClient) Embed(ctx domain.Context, texts []string) ([][]float32, error) {
	const dims = 1536
	out := make([][]float32, len(texts))
	for i, t := range texts {
		out[i] = embedDeterministic(t, dims)
	}
	return out, nil
}

// ChatJSON returns compact JSON derived from a deterministic evaluation.
func (m *MockClient) ChatJSON(ctx domain.Context, systemPrompt, userPrompt string, maxTokens int) (string, error) {
	// Best-effort extraction from our prompt templates; fallback to hashing entire prompt.
	jd := pickFirstNonEmpty(
		segment(userPrompt, "Job Description (User input):", "Study Case Brief"),
		segment(userPrompt, "Job Description:", "Study Case Brief"),
	)
	brief := pickFirstNonEmpty(
		segment(userPrompt, "Study Case Brief (User input):", "Candidate CV Text"),
		segment(userPrompt, "Study Case Brief:", "Candidate CV Text"),
	)
	cv := pickFirstNonEmpty(segment(userPrompt, "Candidate CV Text:", "Project Report Text"))
	pr := pickFirstNonEmpty(segment(userPrompt, "Project Report Text:", "Return JSON"))
	if jd == "" && brief == "" && cv == "" && pr == "" {
		jd = userPrompt
	}
	res := EvaluateMock(cv, pr, jd, brief)
	payload := map[string]any{
		"cv_match_rate":    res.CVMatchRate,
		"cv_feedback":      res.CVFeedback,
		"project_score":    res.ProjectScore,
		"project_feedback": res.ProjectFeedback,
		"overall_summary":  res.OverallSummary,
	}
	b, err := json.Marshal(payload)
	if err != nil { return "", fmt.Errorf("mock marshal: %w", err) }
	s := string(b)
	if maxTokens > 0 && len(s) > maxTokens*4 { // very rough guard
		s = s[:maxTokens*4]
	}
	return s, nil
}

// EvaluateMock deterministically produces an evaluation based on hashed inputs.
func EvaluateMock(cvText, prText, jobDesc, brief string) usecase.EvaluationResult {
	s := hashToFloat(cvText+"|"+jobDesc) // 0..1
	p := hashToFloat(prText+"|"+brief)
	// cv fraction 0..1 (2 decimals)
	cvMatch := round2(0.4*s + 0.3*hashToFloat(cvText) + 0.3*hashToFloat(jobDesc))
	if cvMatch < 0 { cvMatch = 0 }
	if cvMatch > 1 { cvMatch = 1 }
	proj := round1(10.0 * (0.3*p + 0.3*hashToFloat(prText) + 0.4*hashToFloat(brief)))
	if proj > 10 { proj = 10 }
	if proj < 1 { proj = 1 }
	return usecase.EvaluationResult{
		CVMatchRate:     cvMatch,
		CVFeedback:      short("CV covers "+topWords(cvText, 3)+", aligns with role."),
		ProjectScore:    proj,
		ProjectFeedback: short("Project demonstrates "+topWords(prText, 3)+" with clear outcomes."),
		OverallSummary:  "Solid fit with strengths in " + topWords(cvText+" "+prText, 5) + ".",
	}
}

func embedDeterministic(s string, dims int) []float32 {
	// Simple LCG seeded by sha1(s)
	h := sha1.Sum([]byte(s))
	x := binary.BigEndian.Uint32(h[:4])
	const a = 1664525
	const c = 1013904223
	vec := make([]float32, dims)
	for i := 0; i < dims; i++ {
		x = uint32(uint64(a)*uint64(x) + uint64(c))
		// map to [-1,1] range approximately
		v := float32(x) / float32(^uint32(0))
		vec[i] = 2*v - 1
	}
	return vec
}

func pickFirstNonEmpty(ss ...string) string {
	for _, s := range ss { if strings.TrimSpace(s) != "" { return strings.TrimSpace(s) } }
	return ""
}

func segment(s, startMarker, nextMarker string) string {
	i := strings.Index(s, startMarker)
	if i == -1 { return "" }
	s2 := s[i+len(startMarker):]
	j := strings.Index(s2, nextMarker)
	if j == -1 { return strings.TrimSpace(s2) }
	return strings.TrimSpace(s2[:j])
}

func hashToFloat(s string) float64 {
	h := sha1.Sum([]byte(s))
	u := binary.BigEndian.Uint32(h[:4])
	return float64(u%1000) / 1000.0
}

func clamp01(v float64) float64 { if v < 0 { return 0 }; if v > 1 { return 1 }; return v }
func clamp10(v float64) float64 { if v < 0 { return 0 }; if v > 10 { return 10 }; return v }
func round2(v float64) float64 { return math.Round(v*100) / 100 }
func round1(v float64) float64 { return math.Round(v*10) / 10 }

func topWords(s string, n int) string {
	parts := strings.Fields(s)
	if len(parts) > n { parts = parts[:n] }
	return strings.Join(parts, ", ")
}

func short(s string) string {
	if len(s) <= 180 { return s }
	return s[:177] + "..."
}
