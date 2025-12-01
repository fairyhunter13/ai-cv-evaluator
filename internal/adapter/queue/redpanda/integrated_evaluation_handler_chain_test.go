package redpanda

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/fairyhunter13/ai-cv-evaluator/internal/domain"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// chainTestAI is a lightweight AIClient implementation used to verify that the
// multi-step chain is executed end-to-end. It records which kind of prompt was
// seen and always returns a stable JSON payload.
type chainTestAI struct {
	calls []string
}

func (a *chainTestAI) Embed(_ domain.Context, _ []string) ([][]float32, error) {
	// Not used in these unit tests; return a single dummy vector.
	return [][]float32{{0.1, 0.2, 0.3}}, nil
}

func (a *chainTestAI) ChatJSON(_ domain.Context, _ string, _ string, _ int) (string, error) {
	return `{"cv_match_rate":0.7,"cv_feedback":"ok","project_score":8.2,"project_feedback":"ok","overall_summary":"ok"}`, nil
}

func (a *chainTestAI) ChatJSONWithRetry(_ domain.Context, systemPrompt, _ string, _ int) (string, error) {
	label := "unknown"
	switch {
	case strings.Contains(systemPrompt, "You are an HR specialist and recruitment expert. Evaluate the candidate's CV against the job requirements using the standardized scoring rubric"):
		label = "cv_evaluate"
	case strings.Contains(systemPrompt, "You are summarizing a backend and AI-enabled project implementation"):
		label = "summarize"
	case strings.Contains(systemPrompt, "You are a technical reviewer evaluating a candidate's project deliverables"):
		label = "project_evaluate"
	case strings.Contains(systemPrompt, "You are a technical reviewer. Refine the evaluation results into final scores and feedback"):
		label = "refine"
	case strings.Contains(systemPrompt, "You are a senior technical recruiter evaluating a candidate's CV and project."):
		label = "fast"
	}
	a.calls = append(a.calls, label)

	// Always return a stable final JSON payload that matches parseRefinedEvaluationResponse expectations.
	return `{"cv_match_rate":0.7,"cv_feedback":"ok","project_score":8.2,"project_feedback":"ok","overall_summary":"ok"}`, nil
}

func (a *chainTestAI) CleanCoTResponse(_ domain.Context, response string) (string, error) {
	// Pass-through; not exercised in this test.
	return response, nil
}

// cotFallbackAI is a focused AI stub that only implements CoT cleaning. It is
// used to verify that cleanJSONResponseWithCoTFallback and
// parseRefinedEvaluationResponse correctly call CleanCoTResponse when primary
// JSON cleaning fails.
type cotFallbackAI struct {
	cleanCalls int
}

func (c *cotFallbackAI) Embed(_ domain.Context, _ []string) ([][]float32, error) {
	return nil, nil
}

func (c *cotFallbackAI) ChatJSON(_ domain.Context, _ string, _ string, _ int) (string, error) {
	return "", nil
}

func (c *cotFallbackAI) ChatJSONWithRetry(_ domain.Context, _ string, _ string, _ int) (string, error) {
	// Not used in these specific CoT tests.
	return "", nil
}

func (c *cotFallbackAI) CleanCoTResponse(_ domain.Context, _ string) (string, error) {
	c.cleanCalls++
	// Return a fully-formed evaluation JSON object that parseRefinedEvaluationResponse
	// can consume.
	return `{"cv_match_rate":0.9,"cv_feedback":"clean","project_score":9.5,"project_feedback":"clean","overall_summary":"clean"}`, nil
}

// TestIntegratedEvaluationHandler_PerformIntegratedEvaluation_MultiStepChainPreferred
// verifies that the default path runs the explicit multi-step chain (CV
// evaluation -> project evaluation -> refinement) rather than the single
// fast-path prompt when all steps succeed.
func TestIntegratedEvaluationHandler_PerformIntegratedEvaluation_MultiStepChainPreferred(t *testing.T) {
	t.Parallel()

	ai := &chainTestAI{}
	h := NewIntegratedEvaluationHandler(ai, nil)

	ctx := context.Background()
	result, err := h.PerformIntegratedEvaluation(ctx,
		"sample cv content",
		"sample project content",
		"sample job description",
		"sample study case",
		"sample scoring rubric",
		"job-123",
	)
	require.NoError(t, err)

	// The result should reflect the stable JSON returned by the AI stub.
	assert.InDelta(t, 0.7, result.CVMatchRate, 0.0001)
	assert.InDelta(t, 8.2, result.ProjectScore, 0.0001)
	assert.NotEmpty(t, result.CVFeedback)
	assert.NotEmpty(t, result.ProjectFeedback)
	assert.NotEmpty(t, result.OverallSummary)

	// Confirm that the chain steps were invoked at least once.
	assert.Contains(t, ai.calls, "cv_evaluate")
	assert.Contains(t, ai.calls, "project_evaluate")
	assert.Contains(t, ai.calls, "refine")

	// The fast-path fallback should NOT be used in the success scenario.
	for _, label := range ai.calls {
		assert.NotEqual(t, "fast", label)
	}
}

// TestIntegratedEvaluationHandler_CleanJSONResponseWithCoTFallback verifies
// that when the primary JSON cleaning fails, the handler calls
// AIClient.CleanCoTResponse and successfully parses the cleaned output.
func TestIntegratedEvaluationHandler_CleanJSONResponseWithCoTFallback(t *testing.T) {
	t.Parallel()

	ai := &cotFallbackAI{}
	h := NewIntegratedEvaluationHandler(ai, nil)
	ctx := context.Background()

	// Deliberately provide a response that has no JSON object so that the first
	// cleaning attempt fails and triggers CoT cleaning.
	raw := "chain-of-thought and explanations without any JSON structure"

	cleaned, err := h.cleanJSONResponseWithCoTFallback(ctx, raw, "job-cot-1")
	require.NoError(t, err)
	assert.NotEmpty(t, cleaned)
	assert.Equal(t, 1, ai.cleanCalls, "expected exactly one CoT cleaning call")

	// The cleaned output must be valid JSON.
	var payload map[string]any
	require.NoError(t, json.Unmarshal([]byte(cleaned), &payload))
}

// TestIntegratedEvaluationHandler_ParseRefinedEvaluationResponse_UsesCoTFallback
// exercises the full parseRefinedEvaluationResponse path with a non-JSON
// response and verifies that CoT cleaning is used to recover a valid
// domain.Result.
func TestIntegratedEvaluationHandler_ParseRefinedEvaluationResponse_UsesCoTFallback(t *testing.T) {
	t.Parallel()

	ai := &cotFallbackAI{}
	h := NewIntegratedEvaluationHandler(ai, nil)
	ctx := context.Background()

	// Non-JSON response forces cleanJSONResponse to fail, so the handler must
	// call CleanCoTResponse and then successfully parse the cleaned JSON.
	raw := "reasoning steps and analysis without any JSON payload"

	res, err := h.parseRefinedEvaluationResponse(ctx, raw, "job-cot-2")
	require.NoError(t, err)
	assert.Equal(t, 1, ai.cleanCalls, "expected exactly one CoT cleaning call")

	assert.InDelta(t, 0.9, res.CVMatchRate, 0.0001)
	assert.InDelta(t, 9.5, res.ProjectScore, 0.0001)
	assert.Equal(t, "clean", res.CVFeedback)
	assert.Equal(t, "clean", res.ProjectFeedback)
	assert.Equal(t, "clean", res.OverallSummary)
}
