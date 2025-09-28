//go:build ignore
// Mock client tests disabled: project uses real providers only.
package ai_test

import (
	"context"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/fairyhunter13/ai-cv-evaluator/internal/adapter/ai"
	"github.com/fairyhunter13/ai-cv-evaluator/internal/usecase"
)

func TestMockClient_Embed(t *testing.T) {
	t.Parallel()
	client := ai.NewMockClient()

	tests := []struct {
		name  string
		texts []string
		want  int
	}{
		{
			name:  "single text",
			texts: []string{"test text"},
			want:  1,
		},
		{
			name:  "multiple texts",
			texts: []string{"text1", "text2", "text3"},
			want:  3,
		},
		{
			name:  "empty text",
			texts: []string{""},
			want:  1,
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			ctx := context.Background()
			vectors, err := client.Embed(ctx, tt.texts)
			require.NoError(t, err)
			assert.Len(t, vectors, tt.want)
			
			// Each vector should be 1536 dimensions
			for _, vec := range vectors {
				assert.Len(t, vec, 1536)
				// Check values are in expected range [-1, 1]
				for _, v := range vec {
					assert.GreaterOrEqual(t, v, float32(-1.0))
					assert.LessOrEqual(t, v, float32(1.0))
				}
			}
			
			// Deterministic: same input should produce same output
			if len(tt.texts) > 0 {
				vectors2, err := client.Embed(ctx, tt.texts)
				require.NoError(t, err)
				assert.Equal(t, vectors, vectors2)
			}
		})
	}
}

func TestMockClient_ChatJSON(t *testing.T) {
	t.Parallel()
	client := ai.NewMockClient()

	tests := []struct {
		name         string
		systemPrompt string
		userPrompt   string
		maxTokens    int
		checkFields  []string
	}{
		{
			name:         "full evaluation",
			systemPrompt: "You are an evaluator",
			userPrompt: `Job Description: Backend Developer
Study Case Brief: Build an API
Candidate CV Text: 5 years experience in backend
Project Report Text: Built REST API with authentication
Return JSON`,
			maxTokens:   1000,
			checkFields: []string{"cv_match_rate", "cv_feedback", "project_score", "project_feedback", "overall_summary"},
		},
		{
			name:         "with markers",
			systemPrompt: "Evaluate candidate",
			userPrompt: `Job Description (User input): Senior Backend Engineer
Study Case Brief (User input): Create microservices
Candidate CV Text: Expert in Go, Python, microservices
Project Report Text: Implemented service mesh
Return JSON`,
			maxTokens:   500,
			checkFields: []string{"cv_match_rate", "cv_feedback", "project_score"},
		},
		{
			name:         "minimal input",
			systemPrompt: "Evaluate",
			userPrompt:   "Simple test",
			maxTokens:    100,
			checkFields: []string{"cv_match_rate", "project_score"},
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			ctx := context.Background()
			result, err := client.ChatJSON(ctx, tt.systemPrompt, tt.userPrompt, tt.maxTokens)
			require.NoError(t, err)
			assert.NotEmpty(t, result)
			
			// Check it's valid JSON
			assert.True(t, strings.HasPrefix(result, "{"))
			assert.True(t, strings.HasSuffix(result, "}"))
			
			// Check expected fields are present
			for _, field := range tt.checkFields {
				assert.Contains(t, result, field)
			}
			
			// Deterministic: same input should produce same output
			result2, err := client.ChatJSON(ctx, tt.systemPrompt, tt.userPrompt, tt.maxTokens)
			require.NoError(t, err)
			assert.Equal(t, result, result2)
			
			// Check max tokens constraint
			if tt.maxTokens > 0 {
				assert.LessOrEqual(t, len(result), tt.maxTokens*4)
			}
		})
	}
}

func TestEvaluateMock(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		cvText   string
		prText   string
		jobDesc  string
		brief    string
		validate func(t *testing.T, result usecase.EvaluationResult)
	}{
		{
			name:    "normal evaluation",
			cvText:  "10 years backend experience, expert in Go, AWS, Docker",
			prText:  "Built scalable microservices with 99.9% uptime",
			jobDesc: "Looking for senior backend engineer",
			brief:   "Build reliable API",
			validate: func(t *testing.T, result usecase.EvaluationResult) {
				assert.GreaterOrEqual(t, result.CVMatchRate, 0.0)
				assert.LessOrEqual(t, result.CVMatchRate, 1.0)
				assert.GreaterOrEqual(t, result.ProjectScore, 1.0)
				assert.LessOrEqual(t, result.ProjectScore, 10.0)
				assert.NotEmpty(t, result.CVFeedback)
				assert.NotEmpty(t, result.ProjectFeedback)
				assert.NotEmpty(t, result.OverallSummary)
			},
		},
		{
			name:    "empty inputs",
			cvText:  "",
			prText:  "",
			jobDesc: "",
			brief:   "",
			validate: func(t *testing.T, result usecase.EvaluationResult) {
				assert.GreaterOrEqual(t, result.CVMatchRate, 0.0)
				assert.LessOrEqual(t, result.CVMatchRate, 1.0)
				assert.GreaterOrEqual(t, result.ProjectScore, 1.0)
				assert.LessOrEqual(t, result.ProjectScore, 10.0)
			},
		},
		{
			name:    "deterministic output",
			cvText:  "Python developer with ML experience",
			prText:  "Created neural network for image classification",
			jobDesc: "ML Engineer position",
			brief:   "Build ML pipeline",
			validate: func(t *testing.T, result usecase.EvaluationResult) {
				// Run again with same inputs
				result2 := ai.EvaluateMock("Python developer with ML experience",
					"Created neural network for image classification",
					"ML Engineer position",
					"Build ML pipeline")
				assert.Equal(t, result.CVMatchRate, result2.CVMatchRate)
				assert.Equal(t, result.ProjectScore, result2.ProjectScore)
				assert.Equal(t, result.CVFeedback, result2.CVFeedback)
				assert.Equal(t, result.ProjectFeedback, result2.ProjectFeedback)
				assert.Equal(t, result.OverallSummary, result2.OverallSummary)
			},
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			result := ai.EvaluateMock(tt.cvText, tt.prText, tt.jobDesc, tt.brief)
			tt.validate(t, result)
		})
	}
}
