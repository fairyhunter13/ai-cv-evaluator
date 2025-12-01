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
	"strconv"
	"strings"
	"time"

	"github.com/fairyhunter13/ai-cv-evaluator/internal/adapter/observability"
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

	slog.Info("performing multi-step integrated evaluation", slog.String("job_id", jobID))

	// Step 1: evaluate CV match directly against job requirements using the
	// standardized scoring rubric (with optional RAG context).
	step1Ctx, step1Span := tracer.Start(ctx, "PerformIntegratedEvaluation.evaluateCVMatch")
	cvEvaluation, err := h.evaluateCVMatch(step1Ctx, cvContent, jobDesc, scoringRubric, jobID)
	step1Span.End()
	if err != nil {
		slog.Error("step 1: evaluateCVMatch failed; falling back to fast path",
			slog.String("job_id", jobID),
			slog.Any("error", err))
		return h.performFastPathEvaluation(ctx, cvContent, projectContent, jobDesc, studyCase, scoringRubric, jobID)
	}

	// Step 2: evaluate project deliverables (with RAG + standardized rubric)
	step2Ctx, step2Span := tracer.Start(ctx, "PerformIntegratedEvaluation.evaluateProjectDeliverables")
	projectEvaluation, err := h.evaluateProjectDeliverables(step2Ctx, projectContent, studyCase, scoringRubric, jobID)
	step2Span.End()
	if err != nil {
		slog.Error("step 2: evaluateProjectDeliverables failed; falling back to fast path",
			slog.String("job_id", jobID),
			slog.Any("error", err))
		return h.performFastPathEvaluation(ctx, cvContent, projectContent, jobDesc, studyCase, scoringRubric, jobID)
	}

	// Step 3: refine evaluations into final scores and feedback
	step3Ctx, step3Span := tracer.Start(ctx, "PerformIntegratedEvaluation.refineEvaluation")
	refinedResponse, err := h.refineEvaluation(step3Ctx, cvEvaluation, projectEvaluation, jobID)
	step3Span.End()
	if err != nil {
		slog.Error("step 3: refineEvaluation failed; falling back to fast path",
			slog.String("job_id", jobID),
			slog.Any("error", err))
		return h.performFastPathEvaluation(ctx, cvContent, projectContent, jobDesc, studyCase, scoringRubric, jobID)
	}

	// Step 4: validate and finalize results
	step4Ctx, step4Span := tracer.Start(ctx, "PerformIntegratedEvaluation.validateAndFinalizeResults")
	result, err := h.validateAndFinalizeResults(step4Ctx, refinedResponse, jobID)
	step4Span.End()
	if err != nil {
		slog.Error("validateAndFinalizeResults failed for multi-step evaluation; falling back to fast path",
			slog.String("job_id", jobID),
			slog.Any("error", err))
		return h.performFastPathEvaluation(ctx, cvContent, projectContent, jobDesc, studyCase, scoringRubric, jobID)
	}

	slog.Info("integrated evaluation completed successfully with multi-step chain", slog.String("job_id", jobID),
		slog.Float64("cv_match_rate", result.CVMatchRate),
		slog.Float64("project_score", result.ProjectScore))

	observability.ObserveEvaluation(result.CVMatchRate, result.ProjectScore)

	return result, nil
}

// performFastPathEvaluation runs the previous single-prompt evaluation as a fallback.
func (h *IntegratedEvaluationHandler) performFastPathEvaluation(
	ctx context.Context,
	cvContent, projectContent, jobDesc, studyCase, scoringRubric string,
	jobID string,
) (domain.Result, error) {
	tracer := otel.Tracer("integrated.evaluation")
	ctx, span := tracer.Start(ctx, "PerformIntegratedEvaluation.fastPath")
	defer span.End()

	slog.Info("performing fast integrated evaluation", slog.String("job_id", jobID))

	// Optional RAG context: retrieve additional job description and scoring
	// rubric snippets when the vector client is available. This is a
	// best-effort enhancement and must not cause evaluation to fail if
	// embeddings or Qdrant are unavailable.
	var ragContext string
	if h.q != nil {
		ragCtx, err := h.retrieveEnhancedRAGContext(ctx, cvContent+" "+projectContent, jobDesc, studyCase)
		if err != nil {
			slog.Warn("fast path RAG context retrieval failed", slog.String("job_id", jobID), slog.Any("error", err))
		} else {
			ragContext = ragCtx
		}
	}

	extraContext := ""
	if ragContext != "" {
		extraContext = "\n\nAdditional Retrieved Context:\n" + ragContext + "\n"
	}

	prompt := fmt.Sprintf(`You are a senior technical recruiter evaluating a candidate's CV and project.

CV Content:
%s

Project Content:
%s

Job Description:
%s

Study Case:
%s

Scoring Rubric:
%s

%s

Using the information above, produce a single JSON object with the following fields:
{
  "cv_match_rate": 0.85,
  "cv_feedback": "Professional CV feedback",
  "project_score": 8.5,
  "project_feedback": "Technical project feedback",
  "overall_summary": "Candidate summary with recommendations"
}

Guidelines:
- cv_match_rate: 0.0 to 1.0 (0=no match, 1=perfect match)
- project_score: 1.0 to 10.0 (1=poor, 10=excellent)
- Provide professional, constructive feedback in the feedback fields.
- Return only the JSON object, with no extra commentary, prose, or code fences.
`, cvContent, projectContent, jobDesc, studyCase, scoringRubric, extraContext)

	response, err := h.performStableEvaluation(ctx, prompt, jobID)
	if err != nil {
		return domain.Result{}, fmt.Errorf("fast evaluation failed: %w", err)
	}

	result, err := h.validateAndFinalizeResults(ctx, response, jobID)
	if err != nil {
		return domain.Result{}, fmt.Errorf("result validation failed: %w", err)
	}

	slog.Info("integrated evaluation completed successfully with fast path", slog.String("job_id", jobID),
		slog.Float64("cv_match_rate", result.CVMatchRate),
		slog.Float64("project_score", result.ProjectScore))

	observability.ObserveEvaluation(result.CVMatchRate, result.ProjectScore)

	return result, nil
}

// extractStructuredCVInfo extracts structured information from CV with enhanced prompts.
func (h *IntegratedEvaluationHandler) extractStructuredCVInfo(ctx context.Context, cvContent, jobID string) (string, error) {
	slog.Info("step 1: extracting structured CV information", slog.String("job_id", jobID))

	prompt := `You are a CV analyst. Extract structured information from the CV.

CV Content:
%s

Extract and format as JSON:
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

Focus on technical skills, experience level, achievements, and cultural fit indicators.
Return only valid JSON with no additional text, explanations, or reasoning.

`

	response, err := h.performStableEvaluation(ctx, fmt.Sprintf(prompt, cvContent), jobID)
	if err != nil {
		return "", fmt.Errorf("AI extraction failed: %w", err)
	}

	// Clean and validate JSON response
	cleanedResponse, err := h.cleanJSONResponseWithCoTFallback(ctx, response, jobID)
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

	promptTemplate := `You are an HR specialist and recruitment expert. Compare the extracted CV data against the job requirements using the standardized scoring rubric.

Extracted CV Data:
{{EXTRACTED_CV}}

Job Description:
{{JOB_INPUT}}

{{RAG_CONTEXT}}

## CV Match Evaluation Analysis

Analyze the CV against these weighted parameters (1-5 scale each):

**1. Technical Skills Match (40%% weight):**
- Backend languages & frameworks alignment (Node.js, Django, Rails)
- Database experience (MySQL, PostgreSQL, MongoDB)
- API development experience
- Cloud technologies (AWS, Google Cloud, Azure)
- AI/LLM exposure and experience
Scoring: 1=Irrelevant → 5=Excellent + AI/LLM experience

**2. Experience Level (25%% weight):**
- Years of experience assessment
- Project complexity indicators
- Leadership and mentoring experience
Scoring: 1=<1yr → 5=5+ yrs high-impact

**3. Relevant Achievements (20%% weight):**
- Measurable impact of past work
- Scale and scope of projects
- Innovation and problem-solving examples
Scoring: 1=None → 5=Major measurable impact

**4. Cultural/Collaboration Fit (15%% weight):**
- Communication skills indicators
- Learning mindset and adaptability
- Teamwork and collaboration evidence
Scoring: 1=Not shown → 5=Excellent

## Analysis Requirements

For each parameter, provide:
- Specific examples from the CV
- Detailed comparison with job requirements
- Scoring rationale (1-5 scale)
- Weighted contribution to overall match

## Output Format

Respond with detailed JSON analysis including:
{
  "technical_skills_match": {
    "weight": 40,
    "score": 4,
    "analysis": "Detailed analysis with specific examples",
    "alignment": "Strong/Moderate/Weak alignment explanation"
  },
  "experience_level": {
    "weight": 25,
    "score": 3,
    "analysis": "Experience assessment with examples",
    "complexity": "Project complexity indicators"
  },
  "relevant_achievements": {
    "weight": 20,
    "score": 4,
    "analysis": "Achievement impact analysis",
    "scale": "Project scale and scope assessment"
  },
  "cultural_collaboration_fit": {
    "weight": 15,
    "score": 3,
    "analysis": "Cultural fit indicators",
    "collaboration": "Teamwork evidence"
  },
  "overall_assessment": "Comprehensive summary with specific strengths and gaps"
}

Provide detailed analysis for each parameter with specific examples from the CV.
Return only the JSON object, no additional text or explanations.

`

	// Combine job description with RAG context
	jobInput := jobDesc
	if ragContext != "" {
		jobInput = fmt.Sprintf("%s\n\nAdditional Job Context:\n%s", jobDesc, ragContext)
	}

	prompt := strings.Replace(promptTemplate, "{{EXTRACTED_CV}}", extractedCV, 1)
	prompt = strings.Replace(prompt, "{{JOB_INPUT}}", jobInput, 1)
	prompt = strings.Replace(prompt, "{{RAG_CONTEXT}}", ragContext, 1)

	response, err := h.performStableEvaluation(ctx, prompt, jobID)
	if err != nil {
		return "", fmt.Errorf("AI job comparison failed: %w", err)
	}

	slog.Info("job comparison completed", slog.String("job_id", jobID), slog.Int("response_length", len(response)))
	return response, nil
}

// evaluateCVMatch evaluates CV match directly from the raw CV content and job
// description using the standardized scoring rubric. The output is an
// analytical narrative that is later refined into final scores.
func (h *IntegratedEvaluationHandler) evaluateCVMatch(ctx context.Context, cvContent, jobDesc, scoringRubric, jobID string) (string, error) {
	slog.Info("evaluating CV match and generating feedback", slog.String("job_id", jobID))

	// Retrieve RAG context for job requirements (best-effort; must not fail the
	// evaluation if embeddings or Qdrant are unavailable).
	var ragContext string
	if h.q != nil {
		context, err := h.retrieveEnhancedRAGContext(ctx, cvContent, jobDesc, "job_description")
		if err != nil {
			slog.Warn("RAG context retrieval failed for CV evaluation", slog.String("job_id", jobID), slog.Any("error", err))
		} else {
			ragContext = context
		}
	}

	jobInput := jobDesc
	if ragContext != "" {
		jobInput = fmt.Sprintf("%s\n\nAdditional Job Context:\n%s", jobDesc, ragContext)
	}

	promptTemplate := `You are an HR specialist and recruitment expert. Evaluate the candidate's CV against the job requirements using the standardized scoring rubric.

CV Content:
%s

Job Description and Context:
%s

Scoring Rubric:
%s

Provide a concise analytical assessment focusing on:
- Technical skills alignment with the backend + AI/LLM role
- Experience level and impact of previous work
- Relevant achievements and measurable outcomes
- Cultural and collaboration fit (communication, learning mindset, teamwork)

Return a short analysis (bullet list or structured paragraphs). Do NOT return JSON.`

	fullPrompt := fmt.Sprintf(promptTemplate, cvContent, jobInput, scoringRubric)

	response, err := h.performStableEvaluation(ctx, fullPrompt, jobID)
	if err != nil {
		return "", fmt.Errorf("AI CV evaluation failed: %w", err)
	}

	slog.Info("CV evaluation completed", slog.String("job_id", jobID), slog.Int("response_length", len(response)))
	return response, nil
}

// evaluateProjectDeliverables evaluates project deliverables with RAG context.
func (h *IntegratedEvaluationHandler) evaluateProjectDeliverables(ctx context.Context, projectContent, studyCase, scoringRubric, jobID string) (string, error) {
	slog.Info("evaluating project deliverables", slog.String("job_id", jobID))

	// Create a timeout context for the entire project evaluation process. This
	// remains generous but we now perform a single scoring call (no separate
	// summarization step) to reduce overall latency.
	timeoutDuration := 5 * time.Minute // 5 minutes timeout for AI processing
	evalCtx, cancel := context.WithTimeout(ctx, timeoutDuration)
	defer cancel()

	// Retrieve RAG context for project evaluation (best-effort).
	var ragContext string
	if h.q != nil {
		context, err := h.retrieveEnhancedRAGContext(evalCtx, projectContent, studyCase, "scoring_rubric")
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

	// Generate comprehensive project evaluation prompt directly from the raw
	// project content. We intentionally skip a separate summarization call to
	// keep the chain leaner while still providing rich context to the model.
	fullPrompt := h.generateProjectEvaluationPrompt(projectContent, studyInput, scoringRubric)

	response, err := h.performStableEvaluation(evalCtx, fullPrompt, jobID)
	if err != nil {
		return "", fmt.Errorf("AI project evaluation failed: %w", err)
	}

	slog.Info("project evaluation completed", slog.String("job_id", jobID), slog.Int("response_length", len(response)))
	return response, nil
}

// refineEvaluation refines evaluation with stability controls.
func (h *IntegratedEvaluationHandler) refineEvaluation(ctx context.Context, cvEvaluation, projectEvaluation, jobID string) (string, error) {
	slog.Info("refining evaluation with stability controls", slog.String("job_id", jobID))

	prompt := `You are a technical reviewer. Refine the evaluation results into final scores and feedback.

CV Evaluation Results:
%s

Project Evaluation Results:
%s

Please provide the final evaluation in JSON format (no explanations in the output):
{
  "cv_match_rate": 0.85,
  "cv_feedback": "Professional CV feedback",
  "project_score": 8.5,
  "project_feedback": "Technical project feedback",
  "overall_summary": "Candidate summary with recommendations"
}

Guidelines:
- cv_match_rate: 0.0 to 1.0 (0=no match, 1=perfect match)
- project_score: 1.0 to 10.0 (1=poor, 10=excellent)
- Provide professional, constructive feedback
- Return only the JSON object, no additional text

`

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

// summarizeProjectContent produces a compact summary of the project deliverables that
// captures the key aspects relevant for scoring (correctness, resilience, docs, etc.).
// This is intentionally a simpler prompt than the full scoring rubric to improve
// reliability with free models.
func (h *IntegratedEvaluationHandler) summarizeProjectContent(ctx context.Context, projectContent, jobID string) (string, error) {
	slog.Info("summarizing project content before scoring", slog.String("job_id", jobID), slog.Int("project_length", len(projectContent)))

	prompt := `You are summarizing a backend and AI-enabled project implementation.

Project content:
%s

Summarize the project in a concise, technical way focusing on:
- Core features and architecture
- Use of AI/LLM and RAG (if any)
- Resilience and error handling (retries, backoff, fallbacks)
- Observability/monitoring
- Documentation and explanation quality
- Notable creativity or bonus features

Return only a short markdown bullet list (no JSON, no code blocks, no additional prose).`

	response, err := h.performStableEvaluation(ctx, fmt.Sprintf(prompt, projectContent), jobID)
	if err != nil {
		return "", fmt.Errorf("AI project summarization failed: %w", err)
	}

	slog.Info("project summarization completed", slog.String("job_id", jobID), slog.Int("summary_length", len(response)))
	return response, nil
}

// performStableEvaluation performs a stable AI evaluation with retry logic.
func (h *IntegratedEvaluationHandler) performStableEvaluation(ctx context.Context, prompt, _ string) (string, error) {
	// Choose a reasonable maxTokens based on prompt length to avoid provider defaults
	// that may result in very long or stalled generations.
	maxTokens := 512
	plen := len(prompt)
	switch {
	case plen > 16000:
		maxTokens = 2048
	case plen > 8000:
		maxTokens = 1536
	case plen > 4000:
		maxTokens = 1024
	}

	// Use the enhanced retry method with model fallback
	response, err := h.ai.ChatJSONWithRetry(ctx, prompt, "user", maxTokens)
	if err != nil {
		return "", fmt.Errorf("AI evaluation failed: %w", err)
	}
	return response, nil
}

// cleanJSONResponseWithCoTFallback first attempts to clean JSON directly and, on failure,
// uses the AI client's CoT-cleaning endpoint as a fallback before re-attempting cleaning.
func (h *IntegratedEvaluationHandler) cleanJSONResponseWithCoTFallback(ctx context.Context, response string, jobID string) (string, error) {
	cleaned, err := h.cleanJSONResponse(response)
	if err == nil {
		return cleaned, nil
	}

	slog.Warn("primary JSON cleaning failed, attempting CoT cleaning",
		slog.String("job_id", jobID),
		slog.Any("error", err))

	if h.ai == nil {
		return "", err
	}

	cleanedCoT, cotErr := h.ai.CleanCoTResponse(ctx, response)
	if cotErr != nil {
		slog.Error("CoT cleaning failed",
			slog.String("job_id", jobID),
			slog.Any("error", cotErr))
		return "", fmt.Errorf("clean JSON response after CoT cleaning: %w", err)
	}

	cleanedAfterCoT, err2 := h.cleanJSONResponse(cleanedCoT)
	if err2 != nil {
		slog.Error("JSON cleaning failed after CoT cleaning",
			slog.String("job_id", jobID),
			slog.Any("error", err2))
		return "", fmt.Errorf("clean JSON response after CoT cleaning: %w", err2)
	}

	slog.Info("successfully cleaned JSON response after CoT cleaning", slog.String("job_id", jobID))
	return cleanedAfterCoT, nil
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

	// Try to parse as JSON first
	var temp map[string]interface{}
	if err := json.Unmarshal([]byte(cleaned), &temp); err != nil {
		// If parsing fails, try to transform the response to match our expected format
		slog.Warn("JSON parsing failed, attempting transformation",
			slog.String("error", err.Error()),
			slog.String("response_preview", truncateString(cleaned, 200)))

		transformedResponse := h.transformAIResponseToExpectedFormat(cleaned)
		if transformedResponse != "" {
			slog.Info("successfully transformed AI response to expected format")
			return transformedResponse, nil
		}

		return "", fmt.Errorf("invalid JSON after cleaning: %w", err)
	}

	return cleaned, nil
}

// transformAIResponseToExpectedFormat attempts to transform AI responses that don't match our expected format
// nolint:gocyclo // Complex branching is intentional to salvage diverse AI response shapes.
func (h *IntegratedEvaluationHandler) transformAIResponseToExpectedFormat(response string) string {
	// Try to parse the response as JSON
	var data map[string]interface{}
	if err := json.Unmarshal([]byte(response), &data); err != nil {
		return ""
	}

	// Check if it's already in the correct format
	if _, hasCVMatch := data["cv_match_rate"]; hasCVMatch {
		if _, hasProjectScore := data["project_score"]; hasProjectScore {
			return response // Already in correct format
		}
	}

	// Try to extract values from nested analysis structure
	transformed := map[string]interface{}{}

	// Extract CV match rate from various possible structures
	var cvMatchRate float64
	var hasCVMatchRate bool

	// Try multiple possible field names and structures for CV match rate
	cvMatchFields := []string{"cv_match_rate", "cv_match", "match_rate", "cv_score", "technical_skills_match"}
	for _, field := range cvMatchFields {
		if val, ok := data[field]; ok {
			switch v := val.(type) {
			case float64:
				cvMatchRate = v
				hasCVMatchRate = true
			case map[string]interface{}:
				if score, ok := v["score"].(float64); ok {
					cvMatchRate = score
					hasCVMatchRate = true
				}
			case string:
				// Try to parse string as float
				if parsed, err := strconv.ParseFloat(v, 64); err == nil {
					cvMatchRate = parsed
					hasCVMatchRate = true
				}
			}
		}
		if hasCVMatchRate {
			break
		}
	}

	// Extract project score from various possible structures
	var projectScore float64
	var hasProjectScore bool

	// Try multiple possible field names and structures for project score
	projectScoreFields := []string{"project_score", "project_evaluation", "project_rating", "deliverable_score", "code_quality"}
	for _, field := range projectScoreFields {
		if val, ok := data[field]; ok {
			switch v := val.(type) {
			case float64:
				projectScore = v
				hasProjectScore = true
			case map[string]interface{}:
				if score, ok := v["score"].(float64); ok {
					projectScore = score
					hasProjectScore = true
				}
			case string:
				// Try to parse string as float
				if parsed, err := strconv.ParseFloat(v, 64); err == nil {
					projectScore = parsed
					hasProjectScore = true
				}
			}
		}
		if hasProjectScore {
			break
		}
	}

	// If we don't have direct scores, try to calculate from analysis fields
	if !hasCVMatchRate {
		cvMatchRate = h.calculateCVMatchRateFromAnalysis(data)
		if cvMatchRate > 0 {
			hasCVMatchRate = true
			slog.Info("calculated CV match rate from analysis fields", slog.Float64("cv_match_rate", cvMatchRate))
		}
	}

	if !hasProjectScore {
		projectScore = h.calculateProjectScoreFromAnalysis(data)
		if projectScore > 0 {
			hasProjectScore = true
			slog.Info("calculated project score from analysis fields", slog.Float64("project_score", projectScore))
		}
	}

	// Only proceed if we have both required scores from AI response
	if !hasCVMatchRate || !hasProjectScore {
		slog.Warn("AI response missing required scores, transformation failed",
			slog.Bool("has_cv_match_rate", hasCVMatchRate),
			slog.Bool("has_project_score", hasProjectScore))
		return "" // Return empty to indicate transformation failed - will trigger retry with different model
	}

	transformed["cv_match_rate"] = cvMatchRate
	transformed["project_score"] = projectScore

	// Extract feedback fields - NO DEFAULT VALUES
	var hasCvFeedback, hasProjectFeedback, hasOverallSummary bool

	// Try multiple possible field names for CV feedback
	cvFeedbackFields := []string{"cv_feedback", "cv_assessment", "cv_analysis", "candidate_feedback", "cv_review"}
	for _, field := range cvFeedbackFields {
		if feedback, ok := data[field].(string); ok && feedback != "" {
			transformed["cv_feedback"] = feedback
			hasCvFeedback = true
			break
		}
	}

	// Try multiple possible field names for project feedback
	projectFeedbackFields := []string{"project_feedback", "project_assessment", "project_analysis", "deliverable_feedback", "code_review"}
	for _, field := range projectFeedbackFields {
		if feedback, ok := data[field].(string); ok && feedback != "" {
			transformed["project_feedback"] = feedback
			hasProjectFeedback = true
			break
		}
	}

	// Try multiple possible field names for overall summary
	overallSummaryFields := []string{"overall_summary", "summary", "conclusion", "final_assessment", "recommendation", "overall_analysis"}
	for _, field := range overallSummaryFields {
		if summary, ok := data[field].(string); ok && summary != "" {
			transformed["overall_summary"] = summary
			hasOverallSummary = true
			break
		}
	}

	// Check if we have all required fields from AI response
	if !hasCvFeedback || !hasProjectFeedback || !hasOverallSummary {
		slog.Warn("AI response missing required feedback fields, transformation failed",
			slog.Bool("has_cv_feedback", hasCvFeedback),
			slog.Bool("has_project_feedback", hasProjectFeedback),
			slog.Bool("has_overall_summary", hasOverallSummary))
		return "" // Return empty to indicate transformation failed - will trigger retry with different model
	}

	// Convert to JSON
	jsonBytes, err := json.Marshal(transformed)
	if err != nil {
		return ""
	}

	return string(jsonBytes)
}

// calculateCVMatchRateFromAnalysis calculates CV match rate from analysis fields
func (h *IntegratedEvaluationHandler) calculateCVMatchRateFromAnalysis(data map[string]interface{}) float64 {
	// Try to extract from technical_skills array
	if skills, ok := data["technical_skills"].([]interface{}); ok && len(skills) > 0 {
		// Calculate based on number of skills (more skills = higher match)
		skillCount := float64(len(skills))
		// Normalize to 0-1 scale (assume 10+ skills = perfect match)
		matchRate := skillCount / 10.0
		if matchRate > 1.0 {
			matchRate = 1.0
		}
		return matchRate
	}

	// Try to extract from experience_years
	if years, ok := data["experience_years"].(float64); ok {
		// Calculate based on experience (more years = higher match)
		// Normalize to 0-1 scale (assume 5+ years = perfect match)
		matchRate := years / 5.0
		if matchRate > 1.0 {
			matchRate = 1.0
		}
		return matchRate
	}

	// Try to extract from project_complexity
	if complexity, ok := data["project_complexity"].(string); ok {
		switch strings.ToLower(complexity) {
		case "senior", "expert", "advanced":
			return 0.9
		case "mid", "intermediate", "medium":
			return 0.7
		case "junior", "beginner", "entry":
			return 0.5
		default:
			return 0.6
		}
	}

	return 0.0
}

// calculateProjectScoreFromAnalysis calculates project score from analysis fields
func (h *IntegratedEvaluationHandler) calculateProjectScoreFromAnalysis(data map[string]interface{}) float64 {
	// Try to extract from technologies array
	if technologies, ok := data["technologies"].([]interface{}); ok && len(technologies) > 0 {
		// Calculate based on number of technologies (more tech = higher score)
		techCount := float64(len(technologies))
		// Normalize to 1-10 scale (assume 5+ technologies = excellent)
		score := (techCount / 5.0) * 10.0
		if score > 10.0 {
			score = 10.0
		}
		if score < 1.0 {
			score = 1.0
		}
		return score
	}

	// Try to extract from project_complexity
	if complexity, ok := data["project_complexity"].(string); ok {
		switch strings.ToLower(complexity) {
		case "senior", "expert", "advanced":
			return 9.0
		case "mid", "intermediate", "medium":
			return 7.0
		case "junior", "beginner", "entry":
			return 5.0
		default:
			return 6.0
		}
	}

	return 0.0
}

// truncateString truncates a string to the specified length
func truncateString(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
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
		// Do not fail the entire evaluation if embedding provider is unavailable in E2E
		slog.Warn("embedding generation failed; continuing without RAG context",
			slog.Any("error", err))
		return "", nil
	}

	if len(embeddings) == 0 || len(embeddings[0]) == 0 {
		slog.Warn("empty embeddings generated for RAG context")
		return "", nil
	}

	// Search for relevant context in job_description collection (fewer entries for shorter prompts)
	jobContext, err := h.q.Search(ctx, "job_description", embeddings[0], 3)
	if err != nil {
		slog.Error("failed to search job description context", slog.Any("error", err))
		// Don't fail completely, just log and continue
		jobContext = []map[string]any{}
	}

	// Search for relevant context in scoring_rubric collection (fewer entries for shorter prompts)
	rubricContext, err := h.q.Search(ctx, "scoring_rubric", embeddings[0], 2)
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

// generateScoringPrompt generates a comprehensive scoring prompt based on the detailed rubric.
func (h *IntegratedEvaluationHandler) generateScoringPrompt(cvContent, projectContent, jobDesc, studyCase, scoringRubric string) string {
	slog.Info("generating comprehensive scoring prompt with detailed rubric",
		slog.Int("cv_length", len(cvContent)),
		slog.Int("project_length", len(projectContent)),
		slog.Int("job_desc_length", len(jobDesc)),
		slog.Int("study_case_length", len(studyCase)),
		slog.Int("rubric_length", len(scoringRubric)))

	// Build comprehensive evaluation prompt with detailed scoring rubric
	prompt := "You are a technical recruiter evaluating a candidate's CV and project using a standardized scoring rubric.\n\n" +
		"Job Description:\n" + jobDesc + "\n\n" +
		"Study Case:\n" + studyCase + "\n\n" +
		"Additional Scoring Rubric:\n" + scoringRubric + "\n\n" +
		"CV Content:\n" + cvContent + "\n\n" +
		"Project Content:\n" + projectContent + "\n\n" +
		"## CV Match Evaluation (Weighted Scoring)\n\n" +
		"Evaluate the CV against these parameters (1-5 scale each):\n\n" +
		"**1. Technical Skills Match (40%% weight):**\n" +
		"- Backend languages & frameworks alignment (Node.js, Django, Rails)\n" +
		"- Database experience (MySQL, PostgreSQL, MongoDB)\n" +
		"- API development experience\n" +
		"- Cloud technologies (AWS, Google Cloud, Azure)\n" +
		"- AI/LLM exposure and experience\n" +
		"Scoring: 1=Irrelevant → 5=Excellent + AI/LLM experience\n\n" +
		"**2. Experience Level (25%% weight):**\n" +
		"- Years of experience assessment\n" +
		"- Project complexity indicators\n" +
		"- Leadership and mentoring experience\n" +
		"Scoring: 1=<1yr → 5=5+ yrs high-impact\n\n" +
		"**3. Relevant Achievements (20%% weight):**\n" +
		"- Measurable impact of past work\n" +
		"- Scale and scope of projects\n" +
		"- Innovation and problem-solving examples\n" +
		"Scoring: 1=None → 5=Major measurable impact\n\n" +
		"**4. Cultural/Collaboration Fit (15%% weight):**\n" +
		"- Communication skills indicators\n" +
		"- Learning mindset and adaptability\n" +
		"- Teamwork and collaboration evidence\n" +
		"Scoring: 1=Not shown → 5=Excellent\n\n" +
		"## Project Deliverable Evaluation (Weighted Scoring)\n\n" +
		"Evaluate the project against these parameters (1-5 scale each):\n\n" +
		"**1. Correctness (30%% weight):**\n" +
		"- Implements prompt design and LLM chaining\n" +
		"- RAG (retrieval, embeddings, vector DB) implementation\n" +
		"- Meets all specified requirements\n" +
		"Scoring: 1=Not implemented → 5=Fully correct\n\n" +
		"**2. Code Quality & Structure (25%% weight):**\n" +
		"- Clean, modular, reusable code\n" +
		"- Testable architecture\n" +
		"- Strong test coverage\n" +
		"Scoring: 1=Poor → 5=Excellent + strong tests\n\n" +
		"**3. Resilience & Error Handling (20%% weight):**\n" +
		"- Handles jobs, retries, randomness\n" +
		"- API failures and timeouts\n" +
		"- Graceful error recovery\n" +
		"Scoring: 1=Missing → 5=Robust\n\n" +
		"**4. Documentation & Explanation (15%% weight):**\n" +
		"- README clarity and setup instructions\n" +
		"- Explanation of trade-offs\n" +
		"- Design decisions documented\n" +
		"Scoring: 1=Missing → 5=Excellent\n\n" +
		"**5. Creativity/Bonus (10%% weight):**\n" +
		"- Extra features beyond requirements\n" +
		"- Innovative solutions\n" +
		"- Outstanding creativity\n" +
		"Scoring: 1=None → 5=Outstanding creativity\n\n" +
		"## Final Evaluation Format\n\n" +
		"Calculate weighted averages and provide final scores:\n\n" +
		"**CV Match Rate:** Weighted average (1-5) → Convert to percentage (×20)\n" +
		"**Project Score:** Weighted average (1-5) → Scale to 1-10\n" +
		"**Overall Summary:** 3-5 sentences covering strengths, gaps, and recommendations\n\n" +
		"Respond with JSON in this exact format:\n" +
		"{\n" +
		"  \"cv_match_rate\": 0.85,\n" +
		"  \"cv_feedback\": \"Detailed CV analysis with specific examples\",\n" +
		"  \"project_score\": 8.5,\n" +
		"  \"project_feedback\": \"Comprehensive project evaluation with technical details\",\n" +
		"  \"overall_summary\": \"Candidate summary with specific strengths, gaps, and actionable recommendations\"\n" +
		"}\n\n" +
		"**Scoring Guidelines:**\n" +
		"- cv_match_rate: 0.0 to 1.0 (0=no match, 1=perfect match)\n" +
		"- project_score: 1.0 to 10.0 (1=poor, 10=excellent)\n" +
		"- Provide specific, actionable feedback with examples\n" +
		"- Focus on technical skills, experience, and project quality\n" +
		"- Return only the JSON object, no additional text"

	return prompt
}

// generateProjectEvaluationPrompt generates a comprehensive project evaluation prompt.
func (h *IntegratedEvaluationHandler) generateProjectEvaluationPrompt(projectContent, studyCase, scoringRubric string) string {
	slog.Info("generating comprehensive project evaluation prompt",
		slog.Int("project_length", len(projectContent)),
		slog.Int("study_case_length", len(studyCase)),
		slog.Int("rubric_length", len(scoringRubric)))

	// Build comprehensive project evaluation prompt with detailed scoring rubric
	prompt := "You are a technical reviewer evaluating a candidate's project deliverables using a standardized scoring rubric.\n\n" +
		"Study Case:\n" + studyCase + "\n\n" +
		"Additional Scoring Rubric:\n" + scoringRubric + "\n\n" +
		"Project Content:\n" + projectContent + "\n\n" +
		"## Project Deliverable Evaluation (Weighted Scoring)\n\n" +
		"Evaluate the project against these parameters (1-5 scale each):\n\n" +
		"**1. Correctness (30% weight):**\n" +
		"- Implements prompt design and LLM chaining\n" +
		"- RAG (retrieval, embeddings, vector DB) implementation\n" +
		"- Meets all specified requirements\n" +
		"- API endpoints work correctly\n" +
		"- Async job processing implemented\n" +
		"Scoring: 1=Not implemented → 5=Fully correct\n\n" +
		"**2. Code Quality & Structure (25% weight):**\n" +
		"- Clean, modular, reusable code\n" +
		"- Testable architecture\n" +
		"- Strong test coverage\n" +
		"- Proper error handling\n" +
		"- Code organization and separation of concerns\n" +
		"Scoring: 1=Poor → 5=Excellent + strong tests\n\n" +
		"**3. Resilience & Error Handling (20% weight):**\n" +
		"- Handles jobs, retries, randomness\n" +
		"- API failures and timeouts\n" +
		"- Graceful error recovery\n" +
		"- Backoff strategies\n" +
		"- Fallback mechanisms\n" +
		"Scoring: 1=Missing → 5=Robust\n\n" +
		"**4. Documentation & Explanation (15% weight):**\n" +
		"- README clarity and setup instructions\n" +
		"- Explanation of trade-offs\n" +
		"- Design decisions documented\n" +
		"- API documentation\n" +
		"- Architecture explanations\n" +
		"Scoring: 1=Missing → 5=Excellent\n\n" +
		"**5. Creativity/Bonus (10% weight):**\n" +
		"- Extra features beyond requirements\n" +
		"- Innovative solutions\n" +
		"- Outstanding creativity\n" +
		"- Performance optimizations\n" +
		"- Additional integrations\n" +
		"Scoring: 1=None → 5=Outstanding creativity\n\n" +
		"## Analysis Requirements\n\n" +
		"For each parameter, provide:\n" +
		"- Specific examples from the project\n" +
		"- Technical assessment with details\n" +
		"- Scoring rationale (1-5 scale)\n" +
		"- Weighted contribution to overall score\n\n" +
		"## Output Format\n\n" +
		"Respond with detailed JSON analysis including:\n" +
		"{\n" +
		"  \"correctness\": {\n" +
		"    \"weight\": 30,\n" +
		"    \"score\": 4,\n" +
		"    \"analysis\": \"Detailed technical analysis with specific examples\",\n" +
		"    \"implementation\": \"Specific implementation details assessed\"\n" +
		"  },\n" +
		"  \"code_quality\": {\n" +
		"    \"weight\": 25,\n" +
		"    \"score\": 4,\n" +
		"    \"analysis\": \"Code quality assessment with examples\",\n" +
		"    \"structure\": \"Architecture and organization analysis\"\n" +
		"  },\n" +
		"  \"resilience\": {\n" +
		"    \"weight\": 20,\n" +
		"    \"score\": 3,\n" +
		"    \"analysis\": \"Error handling and resilience assessment\",\n" +
		"    \"robustness\": \"Failure handling and recovery mechanisms\"\n" +
		"  },\n" +
		"  \"documentation\": {\n" +
		"    \"weight\": 15,\n" +
		"    \"score\": 4,\n" +
		"    \"analysis\": \"Documentation quality assessment\",\n" +
		"    \"clarity\": \"Setup instructions and explanations\"\n" +
		"  },\n" +
		"  \"creativity\": {\n" +
		"    \"weight\": 10,\n" +
		"    \"score\": 3,\n" +
		"    \"analysis\": \"Creativity and bonus features assessment\",\n" +
		"    \"innovation\": \"Innovative solutions and extra features\"\n" +
		"  },\n" +
		"  \"overall_assessment\": \"Comprehensive project summary with specific strengths and areas for improvement\"\n" +
		"}\n\n" +
		"Provide detailed analysis for each parameter with specific examples from the project."

	return prompt
}

// parseRefinedEvaluationResponse parses the refined evaluation response.
func (h *IntegratedEvaluationHandler) parseRefinedEvaluationResponse(ctx context.Context, response string, jobID string) (domain.Result, error) {
	slog.Info("parsing refined evaluation response", slog.String("job_id", jobID), slog.Int("response_length", len(response)))

	// Clean the JSON response first (with CoT fallback when needed)
	cleanedResponse, err := h.cleanJSONResponseWithCoTFallback(ctx, response, jobID)
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
