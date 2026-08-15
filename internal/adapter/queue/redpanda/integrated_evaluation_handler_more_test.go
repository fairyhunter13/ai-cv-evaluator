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

func TestIntegratedEvaluationHandler_GenerateProjectEvaluationPrompt_ContainsInputs(t *testing.T) {
	h := &IntegratedEvaluationHandler{}
	prompt := h.generateProjectEvaluationPrompt("projY", "studyQ", "rubricR")

	require.Contains(t, prompt, "projY")
	require.Contains(t, prompt, "studyQ")
	require.Contains(t, prompt, "rubricR")
}
