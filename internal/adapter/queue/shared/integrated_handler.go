// Package shared provides integrated evaluation handler with full project.md conformance.
package shared

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"

	aipkg "github.com/fairyhunter13/ai-cv-evaluator/internal/adapter/ai"
	qdrantcli "github.com/fairyhunter13/ai-cv-evaluator/internal/adapter/vector/qdrant"
	"github.com/fairyhunter13/ai-cv-evaluator/internal/domain"
	"go.opentelemetry.io/otel"
)

// IntegratedEvaluationHandler provides the complete evaluation workflow with all enhancements.
type IntegratedEvaluationHandler struct {
	ai domain.AIClient
	q  *qdrantcli.Client
}

// NewIntegratedEvaluationHandler creates a new integrated evaluation handler.
func NewIntegratedEvaluationHandler(ai domain.AIClient, q *qdrantcli.Client) *IntegratedEvaluationHandler {
	return &IntegratedEvaluationHandler{
		ai: ai,
		q:  q,
	}
}

// PerformIntegratedEvaluation performs the complete evaluation workflow with all enhancements.
func (h *IntegratedEvaluationHandler) PerformIntegratedEvaluation(
	ctx context.Context,
	cvContent, projectContent, jobDesc, studyCase, scoringRubric string,
	jobID string,
) (domain.Result, error) {
	tracer := otel.Tracer("integrated.evaluation")
	ctx, span := tracer.Start(ctx, "PerformIntegratedEvaluation")
	defer span.End()

	slog.Info("performing integrated evaluation with full project.md conformance", slog.String("job_id", jobID))

	// Step 1: Extract structured CV information
	extractedCV, err := h.extractStructuredCVInfo(ctx, cvContent, jobID)
	if err != nil {
		return domain.Result{}, fmt.Errorf("CV extraction failed: %w", err)
	}

	// Step 2: Compare with job requirements using RAG
	jobComparison, err := h.compareWithJobRequirements(ctx, extractedCV, jobDesc, jobID)
	if err != nil {
		return domain.Result{}, fmt.Errorf("job comparison failed: %w", err)
	}

	// Step 3: Evaluate CV match with detailed scoring
	cvEvaluation, err := h.evaluateCVMatch(ctx, jobComparison, scoringRubric, jobID)
	if err != nil {
		return domain.Result{}, fmt.Errorf("CV evaluation failed: %w", err)
	}

	// Step 4: Evaluate project deliverables with RAG context
	projectEvaluation, err := h.evaluateProjectDeliverables(ctx, projectContent, studyCase, scoringRubric, jobID)
	if err != nil {
		return domain.Result{}, fmt.Errorf("project evaluation failed: %w", err)
	}

	// Step 5: Refine evaluation with stability controls
	finalResult, err := h.refineEvaluation(ctx, cvEvaluation, projectEvaluation, jobID)
	if err != nil {
		return domain.Result{}, fmt.Errorf("evaluation refinement failed: %w", err)
	}

	// Step 6: Validate and finalize results
	result, err := h.validateAndFinalizeResults(ctx, finalResult, jobID)
	if err != nil {
		return domain.Result{}, fmt.Errorf("result validation failed: %w", err)
	}

	slog.Info("integrated evaluation completed successfully", slog.String("job_id", jobID),
		slog.Float64("cv_match_rate", result.CVMatchRate),
		slog.Float64("project_score", result.ProjectScore))

	return result, nil
}

// extractStructuredCVInfo extracts structured information from CV with enhanced prompts.
func (h *IntegratedEvaluationHandler) extractStructuredCVInfo(ctx context.Context, cvContent, jobID string) (string, error) {
	slog.Info("step 1: extracting structured CV information", slog.String("job_id", jobID))

	prompt := `You are an expert CV analyst and HR specialist. Extract structured information from the CV following the project.md requirements.

CV Content:
%s

Extract and format as JSON with the following structure:
{
  "technical_skills": ["skill1", "skill2", ...],
  "technologies": ["tech1", "tech2", ...],
  "experience_years": number,
  "project_complexity": "junior|mid|senior|lead",
  "achievements": ["achievement1", "achievement2", ...],
  "ai_llm_exposure": boolean,
  "cloud_experience": boolean,
  "backend_experience": boolean,
  "database_experience": boolean,
  "api_experience": boolean,
  "cultural_indicators": ["indicator1", "indicator2", ...],
  "impact_metrics": ["metric1", "metric2", ...]
}

Focus on:
1. Technical skills alignment with backend, databases, APIs, cloud, AI/LLM
2. Experience level and years of experience
3. Project complexity and achievements
4. Relevant accomplishments and measurable impact
5. Cultural fit indicators (communication, learning attitude, teamwork)

CRITICAL: Respond with ONLY valid JSON. No explanations, reasoning, or step-by-step analysis.`

	response, err := PerformStableEvaluation(ctx, h.ai, fmt.Sprintf(prompt, cvContent), jobID)
	if err != nil {
		return "", fmt.Errorf("AI extraction failed: %w", err)
	}

	// Clean and validate JSON response
	responseCleaner := aipkg.NewResponseCleaner()
	cleanedResponse, err := responseCleaner.CleanJSONResponse(response)
	if err != nil {
		slog.Error("response cleaning failed",
			slog.String("job_id", jobID),
			slog.Any("error", err),
			slog.String("original_response", response))
		return "", fmt.Errorf("response cleaning failed: %w", err)
	}

	var extractedData map[string]interface{}
	if err := json.Unmarshal([]byte(cleanedResponse), &extractedData); err != nil {
		slog.Error("CV extraction response validation failed after cleaning",
			slog.String("job_id", jobID),
			slog.Any("error", err),
			slog.String("original_response", response),
			slog.String("cleaned_response", cleanedResponse))
		return "", fmt.Errorf("invalid JSON response from extraction: %w", err)
	}

	slog.Info("CV extraction completed", slog.String("job_id", jobID), slog.Int("response_length", len(response)))
	return response, nil
}

// compareWithJobRequirements compares CV data with job requirements using RAG.
func (h *IntegratedEvaluationHandler) compareWithJobRequirements(ctx context.Context, extractedCV, jobDesc, jobID string) (string, error) {
	slog.Info("step 2: comparing CV data with job requirements", slog.String("job_id", jobID))

	// Retrieve RAG context for job requirements
	var ragContext string
	if h.q != nil {
		context, err := retrieveEnhancedRAGContext(ctx, h.ai, h.q, extractedCV, jobDesc, "job_description")
		if err != nil {
			slog.Warn("RAG context retrieval failed for job comparison", slog.String("job_id", jobID), slog.Any("error", err))
		} else {
			ragContext = context
		}
	}

	prompt := `You are an HR specialist and recruitment expert. Compare the extracted CV data against the job requirements.

Extracted CV Data:
%s

Job Description:
%s

%s

Analyze the match for each requirement and provide detailed comparison focusing on:

1. Technical Skills Match (40%% weight):
   - Backend languages & frameworks alignment
   - Database experience (MySQL, PostgreSQL, MongoDB)
   - API development experience
   - Cloud technologies (AWS, Google Cloud, Azure)
   - AI/LLM exposure and experience

2. Experience Level (25%% weight):
   - Years of experience assessment
   - Project complexity indicators
   - Leadership and mentoring experience

3. Relevant Achievements (20%% weight):
   - Measurable impact of past work
   - Scale and scope of projects
   - Innovation and problem-solving examples

4. Cultural/Collaboration Fit (15%% weight):
   - Communication skills indicators
   - Learning mindset and adaptability
   - Teamwork and collaboration evidence

Provide detailed analysis for each parameter with specific examples from the CV.

CRITICAL: Respond with ONLY valid JSON. No explanations, reasoning, or step-by-step analysis.`

	// Combine job description with RAG context
	jobInput := jobDesc
	if ragContext != "" {
		jobInput = fmt.Sprintf("%s\n\nAdditional Job Context:\n%s", jobDesc, ragContext)
	}

	response, err := PerformStableEvaluation(ctx, h.ai, fmt.Sprintf(prompt, extractedCV, jobInput, ragContext), jobID)
	if err != nil {
		return "", fmt.Errorf("AI job comparison failed: %w", err)
	}

	slog.Info("job comparison completed", slog.String("job_id", jobID), slog.Int("response_length", len(response)))
	return response, nil
}

// evaluateCVMatch evaluates CV match with detailed scoring.
func (h *IntegratedEvaluationHandler) evaluateCVMatch(ctx context.Context, jobComparison, scoringRubric, jobID string) (string, error) {
	slog.Info("step 3: evaluating CV match and generating feedback", slog.String("job_id", jobID))

	// Use the job comparison as the CV content for evaluation
	fullPrompt := GenerateScoringPrompt(jobComparison, "", "", "", scoringRubric)

	response, err := PerformStableEvaluation(ctx, h.ai, fullPrompt, jobID)
	if err != nil {
		return "", fmt.Errorf("AI CV evaluation failed: %w", err)
	}

	slog.Info("CV evaluation completed", slog.String("job_id", jobID), slog.Int("response_length", len(response)))
	return response, nil
}

// evaluateProjectDeliverables evaluates project deliverables with RAG context.
func (h *IntegratedEvaluationHandler) evaluateProjectDeliverables(ctx context.Context, projectContent, studyCase, scoringRubric, jobID string) (string, error) {
	slog.Info("step 4: evaluating project deliverables", slog.String("job_id", jobID))

	// Retrieve RAG context for project evaluation
	var ragContext string
	if h.q != nil {
		context, err := retrieveEnhancedRAGContext(ctx, h.ai, h.q, projectContent, studyCase, "scoring_rubric")
		if err != nil {
			slog.Warn("RAG context retrieval failed for project evaluation", slog.String("job_id", jobID), slog.Any("error", err))
		} else {
			ragContext = context
		}
	}

	// Combine study case with RAG context
	studyInput := studyCase
	if ragContext != "" {
		studyInput = fmt.Sprintf("%s\n\nAdditional Evaluation Context:\n%s", studyCase, ragContext)
	}

	fullPrompt := GenerateScoringPrompt("", projectContent, "", studyInput, scoringRubric)

	response, err := PerformStableEvaluation(ctx, h.ai, fullPrompt, jobID)
	if err != nil {
		return "", fmt.Errorf("AI project evaluation failed: %w", err)
	}

	slog.Info("project evaluation completed", slog.String("job_id", jobID), slog.Int("response_length", len(response)))
	return response, nil
}

// refineEvaluation refines evaluation with stability controls.
func (h *IntegratedEvaluationHandler) refineEvaluation(ctx context.Context, cvEvaluation, projectEvaluation, jobID string) (string, error) {
	slog.Info("refining evaluation with stability controls", slog.String("job_id", jobID))

	prompt := `You are a senior recruitment expert and technical reviewer. Refine the evaluation results into final scores and comprehensive feedback.

CV Evaluation Results:
%s

Project Evaluation Results:
%s

Create the final evaluation result with:

1. Calculate weighted CV match rate:
   - Technical Skills Match (40%%) + Experience Level (25%%) + Relevant Achievements (20%%) + Cultural/Collaboration Fit (15%%)
   - Convert to 0-1 scale: weighted average ÷ 5

2. Calculate weighted project score:
   - Correctness (30%%) + Code Quality (25%%) + Resilience (20%%) + Documentation (15%%) + Creativity (10%%)
   - Convert to 1-10 scale: weighted average × 2, clamp to [1,10]

3. Generate comprehensive feedback:
   - CV feedback: Professional assessment of candidate fit
   - Project feedback: Technical assessment of deliverable quality
   - Overall summary: 3-5 sentences covering strengths, gaps, and recommendations

CRITICAL: Respond with ONLY valid JSON following this structure:
{
  "cv_match_rate": 0.85,
  "cv_feedback": "Professional CV feedback",
  "project_score": 8.5,
  "project_feedback": "Professional project feedback",
  "overall_summary": "Comprehensive candidate summary"
}

Rules:
- cv_match_rate: 0.0-1.0 (0=no match, 1=perfect match)
- project_score: 1.0-10.0 (1=poor, 10=excellent)
- Text fields: Professional and comprehensive
- NO reasoning, explanations, or chain-of-thought
- NO step-by-step analysis or numbered lists`

	response, err := PerformStableEvaluation(ctx, h.ai, fmt.Sprintf(prompt, cvEvaluation, projectEvaluation), jobID)
	if err != nil {
		return "", fmt.Errorf("AI refinement failed: %w", err)
	}

	slog.Info("evaluation refinement completed", slog.String("job_id", jobID), slog.Int("response_length", len(response)))
	return response, nil
}

// validateAndFinalizeResults validates and finalizes the evaluation results.
func (h *IntegratedEvaluationHandler) validateAndFinalizeResults(ctx context.Context, refinedResponse, jobID string) (domain.Result, error) {
	slog.Info("validating and finalizing results", slog.String("job_id", jobID))

	// Parse and validate the refined response
	result, err := parseRefinedEvaluationResponse(ctx, h.ai, refinedResponse, jobID)
	if err != nil {
		return domain.Result{}, fmt.Errorf("parse refined evaluation: %w", err)
	}

	// Validate scores are within expected ranges
	if result.CVMatchRate < 0.0 || result.CVMatchRate > 1.0 {
		return domain.Result{}, fmt.Errorf("invalid CV match rate: %.2f (must be 0.0-1.0)", result.CVMatchRate)
	}

	if result.ProjectScore < 1.0 || result.ProjectScore > 10.0 {
		return domain.Result{}, fmt.Errorf("invalid project score: %.2f (must be 1.0-10.0)", result.ProjectScore)
	}

	// Validate text fields are not empty
	if result.CVFeedback == "" {
		result.CVFeedback = "No feedback provided"
	}
	if result.ProjectFeedback == "" {
		result.ProjectFeedback = "No feedback provided"
	}
	if result.OverallSummary == "" {
		result.OverallSummary = "No summary provided"
	}

	// Log detailed scoring information for audit
	slog.Info("evaluation results validated and finalized",
		slog.String("job_id", jobID),
		slog.Float64("cv_match_rate", result.CVMatchRate),
		slog.Float64("project_score", result.ProjectScore),
		slog.Int("cv_feedback_length", len(result.CVFeedback)),
		slog.Int("project_feedback_length", len(result.ProjectFeedback)),
		slog.Int("overall_summary_length", len(result.OverallSummary)))

	return result, nil
}

// MonitorEvaluationHealth monitors the health of the evaluation process.
func (h *IntegratedEvaluationHandler) MonitorEvaluationHealth(ctx context.Context, jobID string) error {
	slog.Info("monitoring evaluation health", slog.String("job_id", jobID))

	// Monitor AI service health
	if !MonitorAIHealth([]string{}) {
		return fmt.Errorf("AI health check failed")
	}

	// Monitor RAG service health if available
	if h.q != nil {
		// Simple RAG health check
		if err := h.monitorRAGHealth(ctx, jobID); err != nil {
			slog.Warn("RAG health check failed", slog.String("job_id", jobID), slog.Any("error", err))
			// Don't fail the entire process if RAG is unavailable
		}
	}

	slog.Info("evaluation health monitoring completed", slog.String("job_id", jobID))
	return nil
}

// monitorRAGHealth monitors the health of the RAG service.
func (h *IntegratedEvaluationHandler) monitorRAGHealth(ctx context.Context, jobID string) error {
	// Simple RAG health check by searching for a known document
	testVector := make([]float32, 1536) // Assuming 1536-dimensional embeddings
	for i := range testVector {
		testVector[i] = 0.1 // Simple test vector
	}

	_, err := h.q.Search(ctx, "job_description", testVector, 1)
	if err != nil {
		return fmt.Errorf("RAG search test failed: %w", err)
	}

	slog.Info("RAG health check completed", slog.String("job_id", jobID))
	return nil
}
