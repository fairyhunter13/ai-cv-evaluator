package redpanda

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestIntegratedEvaluationHandler_PerformFastPathEvaluation_Succeeds(t *testing.T) {
	ai := &fakeAI{chatRetryResp: `{"cv_match_rate":0.8,"cv_feedback":"ok","project_score":8,"project_feedback":"ok","overall_summary":"ok"}`}
	h := &IntegratedEvaluationHandler{ai: ai}

	ctx := context.Background()
	out, err := h.performFastPathEvaluation(ctx, "cv", "project", "jobDesc", "studyCase", "rubric", "job-1")
	require.NoError(t, err)
	require.NotEmpty(t, out)
}

func TestIntegratedEvaluationHandler_ExtractStructuredCVInfo_Succeeds(t *testing.T) {
	ai := &fakeAI{chatRetryResp: `{"technical_skills":["go"],"experience_years":3}`}
	h := &IntegratedEvaluationHandler{ai: ai}

	ctx := context.Background()
	out, err := h.extractStructuredCVInfo(ctx, "cv content", "job-1")
	require.NoError(t, err)
	require.NotEmpty(t, out)
}

func TestIntegratedEvaluationHandler_SummarizeProjectContent_Succeeds(t *testing.T) {
	ai := &fakeAI{chatRetryResp: "- item1\n- item2"}
	h := &IntegratedEvaluationHandler{ai: ai}

	ctx := context.Background()
	out, err := h.summarizeProjectContent(ctx, "project content", "job-1")
	require.NoError(t, err)
	require.NotEmpty(t, out)
}

func TestIntegratedEvaluationHandler_GenerateScoringPrompt_ContainsInputs(t *testing.T) {
	h := &IntegratedEvaluationHandler{}
	prompt := h.generateScoringPrompt("cvX", "projY", "jobZ", "studyQ", "rubricR")

	require.Contains(t, prompt, "cvX")
	require.Contains(t, prompt, "projY")
	require.Contains(t, prompt, "jobZ")
	require.Contains(t, prompt, "studyQ")
	require.Contains(t, prompt, "rubricR")
}

func TestIntegratedEvaluationHandler_GenerateProjectEvaluationPrompt_ContainsInputs(t *testing.T) {
	h := &IntegratedEvaluationHandler{}
	prompt := h.generateProjectEvaluationPrompt("projY", "studyQ", "rubricR")

	require.Contains(t, prompt, "projY")
	require.Contains(t, prompt, "studyQ")
	require.Contains(t, prompt, "rubricR")
}
