package redpanda

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/fairyhunter13/ai-cv-evaluator/internal/domain"
)

type fakeAI struct {
	embedResp     [][]float32
	embedErr      error
	chatJSONResp  string
	chatJSONErr   error
	chatRetryResp string
	chatRetryErr  error
	cleanCoTResp  string
	cleanCoTErr   error
}

func (f *fakeAI) Embed(_ domain.Context, _ []string) ([][]float32, error) {
	if f.embedResp != nil || f.embedErr != nil {
		return f.embedResp, f.embedErr
	}
	return [][]float32{{0.1, 0.2, 0.3}}, nil
}

func (f *fakeAI) ChatJSON(_ domain.Context, _, _ string, _ int) (string, error) {
	if f.chatJSONResp != "" || f.chatJSONErr != nil {
		return f.chatJSONResp, f.chatJSONErr
	}
	return "{}", nil
}

func (f *fakeAI) ChatJSONWithRetry(_ domain.Context, _, _ string, _ int) (string, error) {
	if f.chatRetryResp != "" || f.chatRetryErr != nil {
		return f.chatRetryResp, f.chatRetryErr
	}
	return "{}", nil
}

func (f *fakeAI) CleanCoTResponse(_ domain.Context, response string) (string, error) {
	if f.cleanCoTResp != "" || f.cleanCoTErr != nil {
		return f.cleanCoTResp, f.cleanCoTErr
	}
	return response, nil
}

func TestTransformAIResponseToExpectedFormat_DerivesScoresAndFeedback(t *testing.T) {
	h := &IntegratedEvaluationHandler{}

	input := `{
		"technical_skills": ["go", "docker", "kafka"],
		"technologies": ["go", "kafka", "redis", "postgres"],
		"cv_feedback": "cv fb",
		"project_feedback": "proj fb",
		"overall_summary": "overall summary"
	}`

	out := h.transformAIResponseToExpectedFormat(input)
	if out == "" {
		t.Fatalf("expected non-empty transformed response")
	}

	var parsed struct {
		CVMatchRate     float64 `json:"cv_match_rate"`
		ProjectScore    float64 `json:"project_score"`
		CVFeedback      string  `json:"cv_feedback"`
		ProjectFeedback string  `json:"project_feedback"`
		OverallSummary  string  `json:"overall_summary"`
	}
	if err := json.Unmarshal([]byte(out), &parsed); err != nil {
		t.Fatalf("failed to unmarshal transformed JSON: %v", err)
	}

	if parsed.CVMatchRate <= 0 || parsed.CVMatchRate > 1 {
		t.Fatalf("cv_match_rate out of range: %v", parsed.CVMatchRate)
	}
	if parsed.ProjectScore < 1 || parsed.ProjectScore > 10 {
		t.Fatalf("project_score out of range: %v", parsed.ProjectScore)
	}
	if parsed.CVFeedback != "cv fb" || parsed.ProjectFeedback != "proj fb" || parsed.OverallSummary != "overall summary" {
		t.Fatalf("feedback fields not preserved: %+v", parsed)
	}
}

func TestTransformAIResponseToExpectedFormat_MissingFieldsReturnsEmpty(t *testing.T) {
	h := &IntegratedEvaluationHandler{}

	input := `{"unexpected":"shape"}`
	out := h.transformAIResponseToExpectedFormat(input)
	if out != "" {
		t.Fatalf("expected empty string for insufficient data, got %q", out)
	}
}

func TestCalculateCVMatchRateFromAnalysis(t *testing.T) {
	h := &IntegratedEvaluationHandler{}

	cases := []struct {
		name string
		data map[string]any
		min  float64
		max  float64
	}{
		{
			name: "skills_based",
			data: map[string]any{"technical_skills": []any{"a", "b", "c"}},
			min:  0.2,
			max:  1.0,
		},
		{
			name: "years_based",
			data: map[string]any{"experience_years": 3.0},
			min:  0.5,
			max:  1.0,
		},
		{
			name: "complexity_senior",
			data: map[string]any{"project_complexity": "senior"},
			min:  0.8,
			max:  0.9,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := h.calculateCVMatchRateFromAnalysis(tc.data)
			if got < tc.min || got > tc.max {
				t.Fatalf("calculateCVMatchRateFromAnalysis(%s) = %v, want between %v and %v", tc.name, got, tc.min, tc.max)
			}
		})
	}
}

func TestCalculateProjectScoreFromAnalysis(t *testing.T) {
	h := &IntegratedEvaluationHandler{}

	cases := []struct {
		name string
		data map[string]any
		min  float64
		max  float64
	}{
		{
			name: "technologies_based",
			data: map[string]any{"technologies": []any{"a", "b", "c", "d"}},
			min:  5,
			max:  10,
		},
		{
			name: "complexity_mid",
			data: map[string]any{"project_complexity": "mid"},
			min:  7,
			max:  7,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := h.calculateProjectScoreFromAnalysis(tc.data)
			if got < tc.min || got > tc.max {
				t.Fatalf("calculateProjectScoreFromAnalysis(%s) = %v, want between %v and %v", tc.name, got, tc.min, tc.max)
			}
		})
	}
}

func TestTruncateString(t *testing.T) {
	if got := truncateString("short", 10); got != "short" {
		t.Fatalf("truncateString short = %q, want %q", got, "short")
	}
	if got := truncateString("this is long", 4); got != "this..." {
		t.Fatalf("truncateString long = %q, want %q", got, "this...")
	}
}

func TestCleanJSONResponse_Variants(t *testing.T) {
	h := &IntegratedEvaluationHandler{}

	base := `{"cv_match_rate":0.9,"cv_feedback":"a","project_score":9,"project_feedback":"b","overall_summary":"c"}`

	// Plain JSON
	clean, err := h.cleanJSONResponse(base)
	if err != nil {
		t.Fatalf("cleanJSONResponse plain returned error: %v", err)
	}
	if clean != base {
		t.Fatalf("expected cleaned JSON to equal base, got %q", clean)
	}

	// Code fenced JSON
	fenced := "```json\n" + base + "\n```"
	clean, err = h.cleanJSONResponse(fenced)
	if err != nil {
		t.Fatalf("cleanJSONResponse fenced returned error: %v", err)
	}
	if clean != base {
		t.Fatalf("expected cleaned fenced JSON to equal base, got %q", clean)
	}

	// Prefixed JSON
	prefixed := "Here's the evaluation result: " + base
	clean, err = h.cleanJSONResponse(prefixed)
	if err != nil {
		t.Fatalf("cleanJSONResponse prefixed returned error: %v", err)
	}
	if clean != base {
		t.Fatalf("expected cleaned prefixed JSON to equal base, got %q", clean)
	}

	// No JSON object should error
	if _, err := h.cleanJSONResponse("no json here"); err == nil {
		t.Fatalf("expected error for input without JSON object")
	}

	// Invalid JSON exercises transform branch and still returns error
	if _, err := h.cleanJSONResponse("{invalid json}"); err == nil {
		t.Fatalf("expected error for invalid JSON input")
	}
}

func TestParseRefinedEvaluationResponse_CoTFallbackAndClamping(t *testing.T) {
	ctx := context.Background()
	ai := &fakeAI{
		cleanCoTResp: `{"cv_match_rate": 1.7, "cv_feedback": "cf", "project_score": 0, "project_feedback": "pf", "overall_summary": "sum"}`,
	}
	h := &IntegratedEvaluationHandler{ai: ai}

	// Initial response is not JSON so cleanJSONResponse will fail and CoT fallback will be used.
	resp := "not-json-response"

	res, err := h.parseRefinedEvaluationResponse(ctx, resp, "job-1")
	if err != nil {
		t.Fatalf("parseRefinedEvaluationResponse returned error: %v", err)
	}

	if res.CVMatchRate < 0 || res.CVMatchRate > 1 {
		t.Fatalf("expected clamped CVMatchRate in [0,1], got %v", res.CVMatchRate)
	}
	if res.ProjectScore < 1 || res.ProjectScore > 10 {
		t.Fatalf("expected clamped ProjectScore in [1,10], got %v", res.ProjectScore)
	}
	if res.CVFeedback == "" || res.ProjectFeedback == "" || res.OverallSummary == "" {
		t.Fatalf("expected non-empty feedback fields in result: %+v", res)
	}
}

func TestIntegratedEvaluationHandler_PerformStableEvaluation_UsesAI(t *testing.T) {
	ai := &fakeAI{chatRetryResp: "{\"ok\":true}"}
	h := &IntegratedEvaluationHandler{ai: ai}

	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	out, err := h.performStableEvaluation(ctx, "prompt", "job-1")
	if err != nil {
		t.Fatalf("performStableEvaluation returned error: %v", err)
	}
	if out == "" {
		t.Fatalf("expected non-empty response from performStableEvaluation")
	}
}

func TestIntegratedEvaluationHandler_EvaluateCVMatch_UsesFastPathWithoutRAG(t *testing.T) {
	ai := &fakeAI{chatRetryResp: `{"cv_match_rate":0.8,"cv_feedback":"ok","project_score":8,"project_feedback":"ok","overall_summary":"ok"}`}
	h := &IntegratedEvaluationHandler{ai: ai}

	ctx := context.Background()

	resp, err := h.evaluateCVMatch(ctx, "cv", "job desc", "rubric", "job-1")
	if err != nil {
		t.Fatalf("evaluateCVMatch returned error: %v", err)
	}
	if resp == "" {
		t.Fatalf("expected non-empty response from evaluateCVMatch")
	}
}

func TestIntegratedEvaluationHandler_EvaluateProjectDeliverables_UsesFastPathWithoutRAG(t *testing.T) {
	ai := &fakeAI{chatRetryResp: `{"cv_match_rate":0.8,"cv_feedback":"ok","project_score":8,"project_feedback":"ok","overall_summary":"ok"}`}
	h := &IntegratedEvaluationHandler{ai: ai}

	ctx := context.Background()

	resp, err := h.evaluateProjectDeliverables(ctx, "project", "study", "rubric", "job-1")
	if err != nil {
		t.Fatalf("evaluateProjectDeliverables returned error: %v", err)
	}
	if resp == "" {
		t.Fatalf("expected non-empty response from evaluateProjectDeliverables")
	}
}

func TestIntegratedEvaluationHandler_CompareWithJobRequirements_NoRAG(t *testing.T) {
	ai := &fakeAI{chatRetryResp: `{"cv_match_rate":0.8,"cv_feedback":"ok","project_score":8,"project_feedback":"ok","overall_summary":"ok"}`}
	h := &IntegratedEvaluationHandler{ai: ai}

	ctx := context.Background()

	resp, err := h.compareWithJobRequirements(ctx, "extracted", "job", "job-1")
	if err != nil {
		t.Fatalf("compareWithJobRequirements returned error: %v", err)
	}
	if resp == "" {
		t.Fatalf("expected non-empty response from compareWithJobRequirements")
	}
}
