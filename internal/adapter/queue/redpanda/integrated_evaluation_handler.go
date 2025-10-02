// Package redpanda provides Redpanda/Kafka queue integration.
//
// It handles message publishing and consumption for job processing.
// The package provides reliable message delivery with exactly-once
// semantics and supports horizontal scaling of workers.
package redpanda

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"
	"time"

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

	response, err := h.performStableEvaluation(ctx, fmt.Sprintf(prompt, cvContent), jobID)
	if err != nil {
		return "", fmt.Errorf("AI extraction failed: %w", err)
	}

	// Clean and validate JSON response
	cleanedResponse, err := h.cleanJSONResponse(response)
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
		context, err := h.retrieveEnhancedRAGContext(ctx, extractedCV, jobDesc, "job_description")
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

	response, err := h.performStableEvaluation(ctx, fmt.Sprintf(prompt, extractedCV, jobInput, ragContext), jobID)
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
	fullPrompt := h.generateScoringPrompt(jobComparison, "", "", "", scoringRubric)

	response, err := h.performStableEvaluation(ctx, fullPrompt, jobID)
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
		context, err := h.retrieveEnhancedRAGContext(ctx, projectContent, studyCase, "scoring_rubric")
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

	fullPrompt := h.generateScoringPrompt("", projectContent, "", studyInput, scoringRubric)

	response, err := h.performStableEvaluation(ctx, fullPrompt, jobID)
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

	response, err := h.performStableEvaluation(ctx, fmt.Sprintf(prompt, cvEvaluation, projectEvaluation), jobID)
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
	result, err := h.parseRefinedEvaluationResponse(ctx, refinedResponse, jobID)
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

// Helper methods for the integrated evaluation handler

// performStableEvaluation performs a stable AI evaluation with retry logic.
func (h *IntegratedEvaluationHandler) performStableEvaluation(ctx context.Context, prompt, _ string) (string, error) {
	// Use the enhanced retry method with model fallback
	response, err := h.ai.ChatJSONWithRetry(ctx, prompt, "system", 0)
	if err != nil {
		return "", fmt.Errorf("AI evaluation failed: %w", err)
	}
	return response, nil
}

// cleanJSONResponse cleans and validates JSON response from AI.
func (h *IntegratedEvaluationHandler) cleanJSONResponse(response string) (string, error) {
	if response == "" {
		return "", fmt.Errorf("empty response")
	}

	// Remove common AI response artifacts
	cleaned := strings.TrimSpace(response)

	// Remove markdown code blocks if present
	if strings.HasPrefix(cleaned, "```json") {
		cleaned = strings.TrimPrefix(cleaned, "```json")
		cleaned = strings.TrimSuffix(cleaned, "```")
		cleaned = strings.TrimSpace(cleaned)
	} else if strings.HasPrefix(cleaned, "```") {
		cleaned = strings.TrimPrefix(cleaned, "```")
		cleaned = strings.TrimSuffix(cleaned, "```")
		cleaned = strings.TrimSpace(cleaned)
	}

	// Remove common AI response prefixes
	prefixes := []string{
		"Here's the evaluation result:",
		"Here is the evaluation result:",
		"Evaluation result:",
		"Result:",
		"JSON:",
		"Response:",
	}

	for _, prefix := range prefixes {
		if strings.HasPrefix(cleaned, prefix) {
			cleaned = strings.TrimSpace(strings.TrimPrefix(cleaned, prefix))
			break
		}
	}

	// Find JSON object boundaries
	startIdx := strings.Index(cleaned, "{")
	endIdx := strings.LastIndex(cleaned, "}")

	if startIdx == -1 || endIdx == -1 || startIdx >= endIdx {
		return "", fmt.Errorf("no valid JSON object found in response")
	}

	cleaned = cleaned[startIdx : endIdx+1]

	// Validate that it's valid JSON
	var temp map[string]interface{}
	if err := json.Unmarshal([]byte(cleaned), &temp); err != nil {
		return "", fmt.Errorf("invalid JSON after cleaning: %w", err)
	}

	return cleaned, nil
}

// retrieveEnhancedRAGContext retrieves enhanced RAG context.
func (h *IntegratedEvaluationHandler) retrieveEnhancedRAGContext(ctx context.Context, query, jobDesc, studyCase string) (string, error) {
	slog.Info("retrieving enhanced RAG context", slog.String("query", query), slog.Int("job_desc_length", len(jobDesc)), slog.Int("study_case_length", len(studyCase)))

	// Create search query combining job description and study case
	searchQuery := fmt.Sprintf("%s %s", jobDesc, studyCase)
	if query != "" {
		searchQuery = fmt.Sprintf("%s %s", query, searchQuery)
	}

	// Generate embeddings for the search query
	embeddings, err := h.ai.Embed(ctx, []string{searchQuery})
	if err != nil {
		slog.Error("failed to generate embeddings for RAG context", slog.Any("error", err))
		return "", fmt.Errorf("generate embeddings: %w", err)
	}

	if len(embeddings) == 0 || len(embeddings[0]) == 0 {
		slog.Warn("empty embeddings generated for RAG context")
		return "", nil
	}

	// Search for relevant context in job_description collection
	jobContext, err := h.q.Search(ctx, "job_description", embeddings[0], 5)
	if err != nil {
		slog.Error("failed to search job description context", slog.Any("error", err))
		// Don't fail completely, just log and continue
		jobContext = []map[string]any{}
	}

	// Search for relevant context in scoring_rubric collection
	rubricContext, err := h.q.Search(ctx, "scoring_rubric", embeddings[0], 3)
	if err != nil {
		slog.Error("failed to search scoring rubric context", slog.Any("error", err))
		// Don't fail completely, just log and continue
		rubricContext = []map[string]any{}
	}

	// Combine and format the context
	var contextParts []string

	// Add job description context
	for _, item := range jobContext {
		if payload, ok := item["payload"].(map[string]any); ok {
			if text, ok := payload["text"].(string); ok && text != "" {
				contextParts = append(contextParts, fmt.Sprintf("Job Context: %s", text))
			}
		}
	}

	// Add scoring rubric context
	for _, item := range rubricContext {
		if payload, ok := item["payload"].(map[string]any); ok {
			if text, ok := payload["text"].(string); ok && text != "" {
				contextParts = append(contextParts, fmt.Sprintf("Scoring Criteria: %s", text))
			}
		}
	}

	// Combine all context
	combinedContext := strings.Join(contextParts, "\n\n")

	slog.Info("retrieved RAG context",
		slog.Int("job_context_count", len(jobContext)),
		slog.Int("rubric_context_count", len(rubricContext)),
		slog.Int("total_context_length", len(combinedContext)))

	return combinedContext, nil
}

// generateScoringPrompt generates a scoring prompt.
func (h *IntegratedEvaluationHandler) generateScoringPrompt(cvContent, projectContent, jobDesc, studyCase, scoringRubric string) string {
	slog.Info("generating comprehensive scoring prompt",
		slog.Int("cv_length", len(cvContent)),
		slog.Int("project_length", len(projectContent)),
		slog.Int("job_desc_length", len(jobDesc)),
		slog.Int("study_case_length", len(studyCase)),
		slog.Int("rubric_length", len(scoringRubric)))

	// Build comprehensive evaluation prompt
	// Build comprehensive evaluation prompt with direct string formatting
	prompt := "You are an expert technical recruiter and senior software engineer conducting a comprehensive candidate evaluation. Your task is to evaluate both the CV and project deliverables against the job requirements and scoring criteria.\n\n" +
		"## EVALUATION FRAMEWORK\n\n" +
		"### CV Evaluation Criteria (40% weight):\n" +
		"1. **Technical Skills Match** (40% of CV score):\n" +
		"   - Backend development experience (Go, Python, Java, Node.js)\n" +
		"   - Database expertise (PostgreSQL, MySQL, Redis, MongoDB)\n" +
		"   - API design and development (REST, GraphQL, microservices)\n" +
		"   - Cloud platforms (AWS, GCP, Azure, Docker, Kubernetes)\n" +
		"   - AI/LLM integration experience (OpenAI, Anthropic, vector databases)\n\n" +
		"2. **Experience Level** (25% of CV score):\n" +
		"   - Years of relevant experience\n" +
		"   - Project complexity and scale\n" +
		"   - Leadership and mentoring experience\n" +
		"   - Industry domain expertise\n\n" +
		"3. **Relevant Achievements** (20% of CV score):\n" +
		"   - Quantifiable impact (performance improvements, cost savings)\n" +
		"   - Technical innovations and problem-solving\n" +
		"   - Open source contributions\n" +
		"   - Certifications and continuous learning\n\n" +
		"4. **Cultural/Collaboration Fit** (15% of CV score):\n" +
		"   - Communication skills and documentation\n" +
		"   - Team collaboration and mentoring\n" +
		"   - Learning attitude and adaptability\n" +
		"   - Problem-solving approach\n\n" +
		"### Project Evaluation Criteria (30% weight):\n" +
		"1. **Correctness** (30% of project score):\n" +
		"   - Functional requirements fulfillment\n" +
		"   - Edge case handling\n" +
		"   - Error handling and validation\n" +
		"   - Business logic accuracy\n\n" +
		"2. **Code Quality** (25% of project score):\n" +
		"   - Clean code principles\n" +
		"   - Proper architecture and design patterns\n" +
		"   - Code organization and modularity\n" +
		"   - Documentation and comments\n\n" +
		"3. **Resilience** (20% of project score):\n" +
		"   - Error handling and recovery\n" +
		"   - Performance optimization\n" +
		"   - Security considerations\n" +
		"   - Scalability and maintainability\n\n" +
		"4. **Documentation** (15% of project score):\n" +
		"   - README quality and completeness\n" +
		"   - API documentation\n" +
		"   - Setup and deployment instructions\n" +
		"   - Code comments and explanations\n\n" +
		"5. **Creativity** (10% of project score):\n" +
		"   - Innovative solutions\n" +
		"   - Creative problem-solving\n" +
		"   - User experience considerations\n" +
		"   - Technical excellence beyond requirements\n\n" +
		"## EVALUATION MATERIALS\n\n" +
		"### Job Description:\n" + jobDesc + "\n\n" +
		"### Study Case:\n" + studyCase + "\n\n" +
		"### Scoring Rubric:\n" + scoringRubric + "\n\n" +
		"### CV Content:\n" + cvContent + "\n\n" +
		"### Project Deliverables:\n" + projectContent + "\n\n" +
		"## EVALUATION INSTRUCTIONS\n\n" +
		"1. **Analyze the CV** against the job requirements, focusing on:\n" +
		"   - Technical skills alignment with job needs\n" +
		"   - Experience level appropriateness\n" +
		"   - Relevant achievements and impact\n" +
		"   - Cultural fit indicators\n\n" +
		"2. **Evaluate the Project** against the study case requirements:\n" +
		"   - Functional completeness and correctness\n" +
		"   - Code quality and architecture\n" +
		"   - Documentation and usability\n" +
		"   - Innovation and technical excellence\n\n" +
		"3. **Calculate Weighted Scores**:\n" +
		"   - CV Match Rate: Technical Skills (40%) + Experience (25%) + Achievements (20%) + Cultural Fit (15%)\n" +
		"   - Project Score: Correctness (30%) + Code Quality (25%) + Resilience (20%) + Documentation (15%) + Creativity (10%)\n\n" +
		"4. **Generate Comprehensive Feedback**:\n" +
		"   - CV Feedback: Professional assessment of candidate fit, strengths, and areas for improvement\n" +
		"   - Project Feedback: Technical assessment of deliverable quality, what works well, and suggestions\n" +
		"   - Overall Summary: 3-5 sentences covering key strengths, potential gaps, and hiring recommendations\n\n" +
		"## OUTPUT FORMAT\n\n" +
		"Respond with ONLY valid JSON following this exact structure:\n" +
		"{\n" +
		"  \"cv_match_rate\": 0.85,\n" +
		"  \"cv_feedback\": \"Professional CV feedback focusing on technical alignment and experience\",\n" +
		"  \"project_score\": 8.5,\n" +
		"  \"project_feedback\": \"Technical project feedback highlighting strengths and areas for improvement\",\n" +
		"  \"overall_summary\": \"Comprehensive candidate summary with strengths, gaps, and recommendations\"\n" +
		"}\n\n" +
		"## SCORING RULES\n" +
		"- cv_match_rate: 0.0-1.0 (0=no match, 1=perfect match)\n" +
		"- project_score: 1.0-10.0 (1=poor, 10=excellent)\n" +
		"- All text fields must be professional, comprehensive, and actionable\n" +
		"- NO reasoning, explanations, or step-by-step analysis\n" +
		"- NO markdown formatting or code blocks\n" +
		"- ONLY the JSON response"

	return prompt
}

// parseRefinedEvaluationResponse parses the refined evaluation response.
func (h *IntegratedEvaluationHandler) parseRefinedEvaluationResponse(_ context.Context, response string, jobID string) (domain.Result, error) {
	slog.Info("parsing refined evaluation response", slog.String("job_id", jobID), slog.Int("response_length", len(response)))

	// Clean the JSON response first
	cleanedResponse, err := h.cleanJSONResponse(response)
	if err != nil {
		slog.Error("failed to clean JSON response", slog.String("job_id", jobID), slog.Any("error", err))
		return domain.Result{}, fmt.Errorf("clean JSON response: %w", err)
	}

	// Parse the JSON response
	var evaluationData struct {
		CVMatchRate     float64 `json:"cv_match_rate"`
		CVFeedback      string  `json:"cv_feedback"`
		ProjectScore    float64 `json:"project_score"`
		ProjectFeedback string  `json:"project_feedback"`
		OverallSummary  string  `json:"overall_summary"`
	}

	if err := json.Unmarshal([]byte(cleanedResponse), &evaluationData); err != nil {
		slog.Error("failed to parse evaluation JSON", slog.String("job_id", jobID), slog.Any("error", err), slog.String("response", cleanedResponse))
		return domain.Result{}, fmt.Errorf("parse evaluation JSON: %w", err)
	}

	// Validate the parsed data
	if evaluationData.CVMatchRate < 0 || evaluationData.CVMatchRate > 1 {
		slog.Warn("invalid CV match rate, clamping to valid range",
			slog.String("job_id", jobID),
			slog.Float64("cv_match_rate", evaluationData.CVMatchRate))
		if evaluationData.CVMatchRate < 0 {
			evaluationData.CVMatchRate = 0
		} else if evaluationData.CVMatchRate > 1 {
			evaluationData.CVMatchRate = 1
		}
	}

	if evaluationData.ProjectScore < 1 || evaluationData.ProjectScore > 10 {
		slog.Warn("invalid project score, clamping to valid range",
			slog.String("job_id", jobID),
			slog.Float64("project_score", evaluationData.ProjectScore))
		if evaluationData.ProjectScore < 1 {
			evaluationData.ProjectScore = 1
		} else if evaluationData.ProjectScore > 10 {
			evaluationData.ProjectScore = 10
		}
	}

	// Create the result
	result := domain.Result{
		JobID:           jobID,
		CVMatchRate:     evaluationData.CVMatchRate,
		CVFeedback:      evaluationData.CVFeedback,
		ProjectScore:    evaluationData.ProjectScore,
		ProjectFeedback: evaluationData.ProjectFeedback,
		OverallSummary:  evaluationData.OverallSummary,
		CreatedAt:       time.Now(),
	}

	slog.Info("successfully parsed evaluation response",
		slog.String("job_id", jobID),
		slog.Float64("cv_match_rate", result.CVMatchRate),
		slog.Float64("project_score", result.ProjectScore))

	return result, nil
}
