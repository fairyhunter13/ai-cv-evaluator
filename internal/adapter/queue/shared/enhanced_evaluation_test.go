package shared

import (
	"context"
	"testing"

	"github.com/fairyhunter13/ai-cv-evaluator/internal/domain"
	domainmocks "github.com/fairyhunter13/ai-cv-evaluator/internal/domain/mocks"
	"github.com/stretchr/testify/assert"
	mock "github.com/stretchr/testify/mock"
)

func TestIntegratedEvaluationHandler(t *testing.T) {
	// Create mock AI client
	mockAI := &domainmocks.AIClient{}

	// Track call count to differentiate between similar prompts
	callCount := 0

	// Mock successful responses for each step - handle multiple calls
	// The mock needs to return different responses based on the prompt content
	mockAI.On("ChatJSON", mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(func(_ domain.Context, _, userPrompt string, _ int) (string, error) {
		callCount++
		// Step 1: CV extraction - look for CV extraction prompt
		if contains(userPrompt, "Extract structured information from the CV") {
			return `{
				"technical_skills": ["Go", "Python", "Docker"],
				"technologies": ["Kubernetes", "PostgreSQL"],
				"experience_years": 5,
				"project_complexity": "senior",
				"achievements": ["Led microservices migration"],
				"ai_llm_exposure": true,
				"cloud_experience": true,
				"backend_experience": true,
				"database_experience": true,
				"api_experience": true,
				"cultural_indicators": ["team player", "mentor"],
				"impact_metrics": ["reduced latency by 50%"]
			}`, nil
		}

		// Step 2: Job comparison - look for job comparison prompt
		if contains(userPrompt, "Compare the extracted CV data against the job requi") {
			return `{
				"technical_alignment": 0.9,
				"experience_match": 0.8,
				"skills_gap": ["GraphQL"],
				"strengths": ["Go expertise", "Cloud experience"],
				"recommendations": "Strong candidate with minor skills gap"
			}`, nil
		}

		// Step 3: CV evaluation - look for CV evaluation prompt (call #3)
		if contains(userPrompt, "Evaluate the candidate's CV and project") && !contains(userPrompt, "Refine the evaluation results") && callCount == 3 {
			return `{
				"technical_skills_match": 4.5,
				"experience_level": 4.0,
				"relevant_achievements": 4.5,
				"cultural_collaboration_fit": 4.0,
				"cv_feedback": "Excellent technical background with strong cloud experience"
			}`, nil
		}

		// Step 4: Project evaluation - look for project evaluation prompt (call #4)
		if contains(userPrompt, "Evaluate the candidate's CV and project") && !contains(userPrompt, "Refine the evaluation results") && callCount == 4 {
			return `{
				"correctness": 4.5,
				"code_quality_structure": 4.0,
				"resilience_error_handling": 4.0,
				"documentation_explanation": 3.5,
				"creativity_bonus": 3.0,
				"project_feedback": "Well-implemented solution with good error handling"
			}`, nil
		}

		// Step 5: Final refinement - look for refinement prompt
		if contains(userPrompt, "Refine the evaluation results") {
			return `{
				"cv_match_rate": 0.85,
				"cv_feedback": "Strong technical background with excellent cloud experience",
				"project_score": 8.5,
				"project_feedback": "Well-implemented solution with good architecture",
				"overall_summary": "Excellent candidate with strong technical skills and good project delivery"
			}`, nil
		}

		// Default fallback - return a valid JSON response
		return `{
			"cv_match_rate": 0.85,
			"cv_feedback": "Strong technical background with excellent cloud experience",
			"project_score": 8.5,
			"project_feedback": "Well-implemented solution with good architecture",
			"overall_summary": "Excellent candidate with strong technical skills and good project delivery"
		}`, nil
	}).Maybe() // Allow multiple calls

	// Create handler
	handler := NewIntegratedEvaluationHandler(mockAI, nil)

	// Test data
	cvContent := "John Doe - Senior Software Engineer with 5 years experience in Go, Python, Docker, Kubernetes"
	projectContent := "Implemented microservices architecture with proper error handling and monitoring"
	jobDesc := "Looking for senior backend engineer with Go and cloud experience"
	studyCase := "Build a scalable API with proper error handling"
	scoringRubric := "Evaluate technical skills, experience, and project quality"
	jobID := "test-job-123"

	// Perform evaluation
	ctx := context.Background()
	result, err := handler.PerformIntegratedEvaluation(ctx, cvContent, projectContent, jobDesc, studyCase, scoringRubric, jobID)

	// Assertions
	assert.NoError(t, err)
	assert.Equal(t, "test-job-123", result.JobID)
	assert.Greater(t, result.CVMatchRate, 0.0)
	assert.LessOrEqual(t, result.CVMatchRate, 1.0)
	assert.Greater(t, result.ProjectScore, 0.0)
	assert.LessOrEqual(t, result.ProjectScore, 10.0)
	assert.NotEmpty(t, result.CVFeedback)
	assert.NotEmpty(t, result.ProjectFeedback)
	assert.NotEmpty(t, result.OverallSummary)

	// Verify all mock calls were made
	mockAI.AssertExpectations(t)
}

func TestPerformStableEvaluation(t *testing.T) {
	// Create mock AI client
	mockAI := &domainmocks.AIClient{}
	mockAI.On("ChatJSON", mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(`{
		"test": "response"
	}`, nil)

	// Test stable evaluation
	ctx := context.Background()
	response, err := PerformStableEvaluation(ctx, mockAI, "Test prompt", "test-job")

	// Assertions
	assert.NoError(t, err)
	assert.Contains(t, response, "test")

	// Verify mock call
	mockAI.AssertExpectations(t)
}

// Helper function to check if string contains substring
func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr ||
		(len(s) > len(substr) &&
			(s[:len(substr)] == substr ||
				s[len(s)-len(substr):] == substr ||
				contains(s[1:], substr))))
}
