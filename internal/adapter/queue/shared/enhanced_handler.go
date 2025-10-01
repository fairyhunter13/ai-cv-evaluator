// Package shared provides enhanced evaluation functionality with full project.md conformance.
package shared

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"
	"time"

	aipkg "github.com/fairyhunter13/ai-cv-evaluator/internal/adapter/ai"
	qdrantcli "github.com/fairyhunter13/ai-cv-evaluator/internal/adapter/vector/qdrant"
	"github.com/fairyhunter13/ai-cv-evaluator/internal/domain"
	"go.opentelemetry.io/otel"
)

// EnhancedEvaluationResult represents the detailed evaluation result with all scoring parameters.
type EnhancedEvaluationResult struct {
	// CV Match Evaluation (1-5 scale per parameter)
	TechnicalSkillsMatch     float64 `json:"technical_skills_match"`
	ExperienceLevel          float64 `json:"experience_level"`
	RelevantAchievements     float64 `json:"relevant_achievements"`
	CulturalCollaborationFit float64 `json:"cultural_collaboration_fit"`

	// Project Deliverable Evaluation (1-5 scale per parameter)
	Correctness              float64 `json:"correctness"`
	CodeQualityStructure     float64 `json:"code_quality_structure"`
	ResilienceErrorHandling  float64 `json:"resilience_error_handling"`
	DocumentationExplanation float64 `json:"documentation_explanation"`
	CreativityBonus          float64 `json:"creativity_bonus"`

	// Text feedback
	CVFeedback      string `json:"cv_feedback"`
	ProjectFeedback string `json:"project_feedback"`
	OverallSummary  string `json:"overall_summary"`
}

// PerformEnhancedEvaluation implements the 4-step LLM chaining process as specified in project.md.
func PerformEnhancedEvaluation(
	ctx context.Context,
	ai domain.AIClient,
	q *qdrantcli.Client,
	cvContent, projectContent, jobDesc, studyCase, scoringRubric string,
	jobID string,
) (domain.Result, error) {
	tracer := otel.Tracer("enhanced.evaluation")
	ctx, span := tracer.Start(ctx, "PerformEnhancedEvaluation")
	defer span.End()

	slog.Info("performing enhanced 4-step evaluation", slog.String("job_id", jobID))

	// Step 1: Extract structured info from CV (skills, experiences, projects)
	extractedCV, err := extractStructuredCVInfo(ctx, ai, cvContent)
	if err != nil {
		slog.Error("step 1: CV extraction failed", slog.String("job_id", jobID), slog.Any("error", err))
		return domain.Result{}, fmt.Errorf("CV extraction failed: %w", err)
	}

	// Step 2: Compare extracted data with job vacancy (LLM prompt)
	jobComparison, err := compareWithJobRequirements(ctx, ai, extractedCV, jobDesc, q)
	if err != nil {
		slog.Error("step 2: job comparison failed", slog.String("job_id", jobID), slog.Any("error", err))
		return domain.Result{}, fmt.Errorf("job comparison failed: %w", err)
	}

	// Step 3: Score match rate & generate CV feedback
	cvEvaluation, err := evaluateCVMatch(ctx, ai, jobComparison, scoringRubric)
	if err != nil {
		slog.Error("step 3: CV evaluation failed", slog.String("job_id", jobID), slog.Any("error", err))
		return domain.Result{}, fmt.Errorf("CV evaluation failed: %w", err)
	}

	// Step 4: Evaluate project report based on scoring rubric → refine via second LLM call
	projectEvaluation, err := evaluateProjectDeliverables(ctx, ai, projectContent, studyCase, scoringRubric, q)
	if err != nil {
		slog.Error("step 4: project evaluation failed", slog.String("job_id", jobID), slog.Any("error", err))
		return domain.Result{}, fmt.Errorf("project evaluation failed: %w", err)
	}

	// Refine evaluation via second LLM call
	finalResult, err := refineEvaluation(ctx, ai, cvEvaluation, projectEvaluation, jobID)
	if err != nil {
		slog.Error("refinement failed", slog.String("job_id", jobID), slog.Any("error", err))
		return domain.Result{}, fmt.Errorf("evaluation refinement failed: %w", err)
	}

	slog.Info("enhanced evaluation completed", slog.String("job_id", jobID),
		slog.Float64("cv_match_rate", finalResult.CVMatchRate),
		slog.Float64("project_score", finalResult.ProjectScore))

	return finalResult, nil
}

// Step 1: Extract structured information from CV
func extractStructuredCVInfo(ctx context.Context, ai domain.AIClient, cvContent string) (string, error) {
	slog.Info("step 1: extracting structured CV information")

	extractionPrompt := `You are an expert CV analyst and HR specialist. Extract structured information from the CV following the project.md requirements.

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

	response, err := ai.ChatJSON(ctx, extractionPrompt, cvContent, 1000)
	if err != nil {
		return "", fmt.Errorf("AI extraction failed: %w", err)
	}

	// Clean and validate JSON response
	responseCleaner := aipkg.NewResponseCleaner()
	cleanedResponse, err := responseCleaner.CleanJSONResponse(response)
	if err != nil {
		slog.Error("response cleaning failed",
			slog.Any("error", err),
			slog.String("original_response", response))
		return "", fmt.Errorf("response cleaning failed: %w", err)
	}

	var extractedData map[string]interface{}
	if err := json.Unmarshal([]byte(cleanedResponse), &extractedData); err != nil {
		slog.Error("CV extraction response validation failed after cleaning",
			slog.Any("error", err),
			slog.String("original_response", response),
			slog.String("cleaned_response", cleanedResponse))
		return "", fmt.Errorf("invalid JSON response from extraction: %w", err)
	}

	slog.Info("CV extraction completed", slog.Int("response_length", len(response)))
	return response, nil
}

// Step 2: Compare extracted data with job vacancy
func compareWithJobRequirements(ctx context.Context, ai domain.AIClient, extractedCV, jobDesc string, q *qdrantcli.Client) (string, error) {
	slog.Info("step 2: comparing CV data with job requirements")

	// Retrieve RAG context for job requirements
	var ragContext string
	if q != nil {
		context, err := retrieveEnhancedRAGContext(ctx, ai, q, extractedCV, jobDesc, "job_description")
		if err != nil {
			slog.Warn("RAG context retrieval failed for job comparison", slog.Any("error", err))
		} else {
			ragContext = context
		}
	}

	comparisonPrompt := `You are an HR specialist and recruitment expert. Compare the extracted CV data against the job requirements.

Extracted CV Data:
%s

Job Description:
%s

%s

Analyze the match for each requirement and provide detailed comparison focusing on:

1. Technical Skills Match (40% weight):
   - Backend languages & frameworks alignment
   - Database experience (MySQL, PostgreSQL, MongoDB)
   - API development experience
   - Cloud technologies (AWS, Google Cloud, Azure)
   - AI/LLM exposure and experience

2. Experience Level (25% weight):
   - Years of experience assessment
   - Project complexity indicators
   - Leadership and mentoring experience

3. Relevant Achievements (20% weight):
   - Measurable impact of past work
   - Scale and scope of projects
   - Innovation and problem-solving examples

4. Cultural/Collaboration Fit (15% weight):
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

	response, err := ai.ChatJSON(ctx, comparisonPrompt, fmt.Sprintf("CV Data:\n%s\n\nJob Requirements:\n%s", extractedCV, jobInput), 1500)
	if err != nil {
		return "", fmt.Errorf("AI job comparison failed: %w", err)
	}

	slog.Info("job comparison completed", slog.Int("response_length", len(response)))
	return response, nil
}

// Step 3: Score match rate & generate CV feedback
func evaluateCVMatch(ctx context.Context, ai domain.AIClient, jobComparison, scoringRubric string) (string, error) {
	slog.Info("step 3: evaluating CV match and generating feedback")

	evaluationPrompt := `You are a senior recruitment expert. Evaluate the CV match based on the detailed comparison and scoring rubric.

Job Comparison Analysis:
%s

Scoring Rubric:
%s

Evaluate each parameter on a 1-5 scale according to the project.md requirements:

CV Match Evaluation Parameters:
1. Technical Skills Match (40% weight): Alignment with job requirements (backend, DB, APIs, cloud, AI/LLM)
   - 1 = Irrelevant → 5 = Excellent + AI/LLM

2. Experience Level (25% weight): Years of experience, project complexity
   - 1 = <1yr → 5 = 5+ yrs high-impact

3. Relevant Achievements (20% weight): Impact of past work
   - 1 = None → 5 = Major measurable impact

4. Cultural/Collaboration Fit (15% weight): Communication, learning mindset, teamwork
   - 1 = Not shown → 5 = Excellent

Provide detailed scoring and professional feedback for each parameter.

CRITICAL: Respond with ONLY valid JSON following this structure:
{
  "technical_skills_match": 4.2,
  "experience_level": 3.8,
  "relevant_achievements": 4.5,
  "cultural_collaboration_fit": 3.9,
  "cv_feedback": "Professional feedback on CV match",
  "scoring_rationale": "Brief explanation of scoring decisions"
}

Rules:
- Scores: 1.0-5.0 (1=poor, 5=excellent)
- Text fields: Professional and concise
- NO reasoning, explanations, or chain-of-thought
- NO step-by-step analysis or numbered lists`

	response, err := ai.ChatJSON(ctx, evaluationPrompt, fmt.Sprintf("Comparison:\n%s\n\nRubric:\n%s", jobComparison, scoringRubric), 1200)
	if err != nil {
		return "", fmt.Errorf("AI CV evaluation failed: %w", err)
	}

	slog.Info("CV evaluation completed", slog.Int("response_length", len(response)))
	return response, nil
}

// Step 4: Evaluate project deliverables based on scoring rubric
func evaluateProjectDeliverables(ctx context.Context, ai domain.AIClient, projectContent, studyCase, scoringRubric string, q *qdrantcli.Client) (string, error) {
	slog.Info("step 4: evaluating project deliverables")

	// Retrieve RAG context for project evaluation
	var ragContext string
	if q != nil {
		context, err := retrieveEnhancedRAGContext(ctx, ai, q, projectContent, studyCase, "scoring_rubric")
		if err != nil {
			slog.Warn("RAG context retrieval failed for project evaluation", slog.Any("error", err))
		} else {
			ragContext = context
		}
	}

	projectPrompt := `You are a senior technical reviewer and project evaluator. Evaluate the project deliverables against the study case requirements and scoring rubric.

Project Content:
%s

Study Case Requirements:
%s

Scoring Rubric:
%s

%s

Evaluate each parameter on a 1-5 scale according to the project.md requirements:

Project Deliverable Evaluation Parameters:
1. Correctness (30% weight): Implements prompt design, LLM chaining, RAG
   - 1 = Not implemented → 5 = Fully correct

2. Code Quality & Structure (25% weight): Clean, modular, reusable, tested
   - 1 = Poor → 5 = Excellent + strong tests

3. Resilience & Error Handling (20% weight): Handles jobs, retries, randomness, API failures
   - 1 = Missing → 5 = Robust

4. Documentation & Explanation (15% weight): README clarity, setup instructions, trade-offs
   - 1 = Missing → 5 = Excellent

5. Creativity / Bonus (10% weight): Extra features beyond requirements
   - 1 = None → 5 = Outstanding creativity

Provide detailed scoring and professional feedback for each parameter.

CRITICAL: Respond with ONLY valid JSON following this structure:
{
  "correctness": 4.1,
  "code_quality_structure": 4.3,
  "resilience_error_handling": 3.7,
  "documentation_explanation": 4.0,
  "creativity_bonus": 3.6,
  "project_feedback": "Professional feedback on project quality",
  "scoring_rationale": "Brief explanation of scoring decisions"
}

Rules:
- Scores: 1.0-5.0 (1=poor, 5=excellent)
- Text fields: Professional and concise
- NO reasoning, explanations, or chain-of-thought
- NO step-by-step analysis or numbered lists`

	// Combine study case with RAG context
	studyInput := studyCase
	if ragContext != "" {
		studyInput = fmt.Sprintf("%s\n\nAdditional Evaluation Context:\n%s", studyCase, ragContext)
	}

	response, err := ai.ChatJSON(ctx, projectPrompt, fmt.Sprintf("Project:\n%s\n\nStudy Case:\n%s\n\nRubric:\n%s", projectContent, studyInput, scoringRubric), 1200)
	if err != nil {
		return "", fmt.Errorf("AI project evaluation failed: %w", err)
	}

	slog.Info("project evaluation completed", slog.Int("response_length", len(response)))
	return response, nil
}

// Refine evaluation via second LLM call
func refineEvaluation(ctx context.Context, ai domain.AIClient, cvEvaluation, projectEvaluation, jobID string) (domain.Result, error) {
	slog.Info("refining evaluation with second LLM call", slog.String("job_id", jobID))

	refinementPrompt := `You are a senior recruitment expert and technical reviewer. Refine the evaluation results into final scores and comprehensive feedback.

CV Evaluation Results:
%s

Project Evaluation Results:
%s

Create the final evaluation result with:

1. Calculate weighted CV match rate:
   - Technical Skills Match (40%) + Experience Level (25%) + Relevant Achievements (20%) + Cultural/Collaboration Fit (15%)
   - Convert to 0-1 scale: weighted average ÷ 5

2. Calculate weighted project score:
   - Correctness (30%) + Code Quality (25%) + Resilience (20%) + Documentation (15%) + Creativity (10%)
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

	response, err := ai.ChatJSON(ctx, refinementPrompt, fmt.Sprintf("CV Evaluation:\n%s\n\nProject Evaluation:\n%s", cvEvaluation, projectEvaluation), 1000)
	if err != nil {
		return domain.Result{}, fmt.Errorf("AI refinement failed: %w", err)
	}

	// Parse and validate the refined response
	result, err := parseRefinedEvaluationResponse(ctx, ai, response, jobID)
	if err != nil {
		return domain.Result{}, fmt.Errorf("parse refined evaluation: %w", err)
	}

	slog.Info("evaluation refinement completed", slog.String("job_id", jobID),
		slog.Float64("cv_match_rate", result.CVMatchRate),
		slog.Float64("project_score", result.ProjectScore))

	return result, nil
}

// retrieveEnhancedRAGContext retrieves relevant context using enhanced RAG with semantic understanding.
func retrieveEnhancedRAGContext(ctx context.Context, ai domain.AIClient, q *qdrantcli.Client, content1, content2, collection string) (string, error) {
	slog.Info("retrieving enhanced RAG context", slog.String("collection", collection))

	// Generate multiple query strategies for better context retrieval
	queries := generateSemanticQueries(content1, content2)

	var contexts []string

	// Search with multiple query strategies
	for _, query := range queries {
		context, err := searchWithSemanticUnderstanding(ctx, ai, q, collection, query, 3)
		if err != nil {
			slog.Warn("semantic search failed", slog.String("query", query), slog.Any("error", err))
			continue
		}
		contexts = append(contexts, context...)
	}

	if len(contexts) == 0 {
		slog.Info("no RAG context found")
		return "", nil
	}

	// Validate and rank contexts by relevance
	validatedContexts := validateContextRelevance(contexts, content1, content2)

	// Limit to top 3 contexts for token efficiency
	if len(validatedContexts) > 3 {
		validatedContexts = validatedContexts[:3]
	}

	combinedContext := strings.Join(validatedContexts, "\n\n")
	slog.Info("retrieved enhanced RAG context",
		slog.Int("context_count", len(validatedContexts)),
		slog.Int("total_length", len(combinedContext)))

	return combinedContext, nil
}

// generateSemanticQueries creates multiple query strategies for better context retrieval.
func generateSemanticQueries(content1, content2 string) []string {
	queries := []string{
		content1, // Original content
		content2, // Secondary content
	}

	// Add focused queries for better retrieval
	if len(content1) > 200 {
		queries = append(queries, content1[:200]) // First 200 chars
	}
	if len(content2) > 200 {
		queries = append(queries, content2[:200]) // First 200 chars
	}

	return queries
}

// searchWithSemanticUnderstanding performs semantic search with understanding.
func searchWithSemanticUnderstanding(ctx context.Context, ai domain.AIClient, q *qdrantcli.Client, collection, query string, topK int) ([]string, error) {
	// Generate embeddings for the query
	embeddings, err := ai.Embed(ctx, []string{query})
	if err != nil {
		return nil, fmt.Errorf("failed to generate embeddings: %w", err)
	}

	if len(embeddings) == 0 {
		return nil, fmt.Errorf("no embeddings generated")
	}

	// Search Qdrant for similar vectors
	results, err := q.Search(ctx, collection, embeddings[0], topK)
	if err != nil {
		return nil, fmt.Errorf("failed to search collection %s: %w", collection, err)
	}

	// Extract and validate text from search results
	var contexts []string
	for _, result := range results {
		if payload, ok := result["payload"].(map[string]any); ok {
			if text, ok := payload["text"].(string); ok && text != "" {
				contexts = append(contexts, text)
			}
		}
	}

	slog.Info("semantic search completed", slog.String("collection", collection), slog.Int("results", len(contexts)))
	return contexts, nil
}

// validateContextRelevance validates and ranks contexts by relevance.
func validateContextRelevance(contexts []string, content1, content2 string) []string {
	if len(contexts) <= 1 {
		return contexts
	}

	// Simple relevance scoring based on keyword overlap and semantic similarity
	type ContextScore struct {
		Text  string
		Score float64
	}

	var scoredContexts []ContextScore
	for _, context := range contexts {
		score := calculateSemanticRelevance(context, content1, content2)
		scoredContexts = append(scoredContexts, ContextScore{Text: context, Score: score})
	}

	// Sort by score (descending)
	for i := 0; i < len(scoredContexts)-1; i++ {
		for j := i + 1; j < len(scoredContexts); j++ {
			if scoredContexts[i].Score < scoredContexts[j].Score {
				scoredContexts[i], scoredContexts[j] = scoredContexts[j], scoredContexts[i]
			}
		}
	}

	// Extract ranked contexts
	rankedContexts := make([]string, len(scoredContexts))
	for i, cs := range scoredContexts {
		rankedContexts[i] = cs.Text
	}

	return rankedContexts
}

// calculateSemanticRelevance calculates semantic relevance score.
func calculateSemanticRelevance(context, content1, content2 string) float64 {
	// Enhanced relevance scoring with semantic understanding
	contextLower := strings.ToLower(context)
	content1Lower := strings.ToLower(content1)
	content2Lower := strings.ToLower(content2)

	// Calculate keyword overlap with semantic weighting
	score1 := calculateSemanticOverlap(contextLower, content1Lower)
	score2 := calculateSemanticOverlap(contextLower, content2Lower)

	// Weighted combination
	return (score1 + score2) / 2.0
}

// calculateSemanticOverlap calculates semantic overlap between texts.
func calculateSemanticOverlap(text1, text2 string) float64 {
	words1 := strings.Fields(text1)
	words2 := strings.Fields(text2)

	if len(words1) == 0 || len(words2) == 0 {
		return 0.0
	}

	// Create weighted word frequency maps (longer words get higher weight)
	freq1 := make(map[string]float64)
	freq2 := make(map[string]float64)

	for _, word := range words1 {
		if len(word) > 2 { // Ignore very short words
			weight := float64(len(word)) / 10.0 // Weight by word length
			freq1[word] += weight
		}
	}

	for _, word := range words2 {
		if len(word) > 2 { // Ignore very short words
			weight := float64(len(word)) / 10.0 // Weight by word length
			freq2[word] += weight
		}
	}

	// Calculate weighted overlaps
	overlaps := 0.0
	totalWeight := 0.0

	for word, weight := range freq1 {
		totalWeight += weight
		if freq2[word] > 0 {
			overlaps += weight
		}
	}

	if totalWeight == 0 {
		return 0.0
	}

	return overlaps / totalWeight
}

// parseRefinedEvaluationResponse parses and validates the refined evaluation response.
func parseRefinedEvaluationResponse(ctx context.Context, ai domain.AIClient, response, jobID string) (domain.Result, error) {
	// Clean the response using ResponseCleaner
	responseCleaner := aipkg.NewResponseCleaner()
	cleanedResponse, err := responseCleaner.CleanJSONResponse(response)
	if err != nil {
		slog.Error("response cleaning failed",
			slog.String("job_id", jobID),
			slog.Any("error", err),
			slog.String("original_response", response))
		return domain.Result{}, fmt.Errorf("response cleaning failed: %w", err)
	}
	response = cleanedResponse

	// Check for chain-of-thought leakage and clean if needed
	if err := validateNoCoTLeakageEnhanced(response); err != nil {
		slog.Warn("CoT leakage detected in refined response, attempting to clean",
			slog.String("job_id", jobID),
			slog.String("response", response),
			slog.Any("error", err))

		// Try to clean the response
		cleanedResponse, cleanErr := ai.CleanCoTResponse(ctx, response)
		if cleanErr != nil {
			slog.Error("CoT cleaning failed",
				slog.String("job_id", jobID),
				slog.Any("clean_error", cleanErr))
			return domain.Result{}, fmt.Errorf("CoT leakage detected and cleaning failed: %w", cleanErr)
		}

		// Validate the cleaned response
		if cleanErr := validateNoCoTLeakageEnhanced(cleanedResponse); cleanErr != nil {
			slog.Error("CoT leakage still present after cleaning",
				slog.String("job_id", jobID),
				slog.String("cleaned_response", cleanedResponse),
				slog.Any("error", cleanErr))
			return domain.Result{}, fmt.Errorf("CoT leakage still present after cleaning: %w", cleanErr)
		}

		slog.Info("CoT cleaning successful",
			slog.String("job_id", jobID),
			slog.Int("original_length", len(response)),
			slog.Int("cleaned_length", len(cleanedResponse)))

		response = cleanedResponse
	}

	// Parse JSON response
	var aiResponse struct {
		CVMatchRate     float64 `json:"cv_match_rate"`
		CVFeedback      string  `json:"cv_feedback"`
		ProjectScore    float64 `json:"project_score"`
		ProjectFeedback string  `json:"project_feedback"`
		OverallSummary  string  `json:"overall_summary"`
	}

	if err := json.Unmarshal([]byte(response), &aiResponse); err != nil {
		return domain.Result{}, fmt.Errorf("invalid JSON response: %w", err)
	}

	// Validate and clamp values according to domain model
	cvMatchRate := aiResponse.CVMatchRate
	if cvMatchRate < 0 {
		cvMatchRate = 0
	} else if cvMatchRate > 1 {
		cvMatchRate = 1
	}

	projectScore := aiResponse.ProjectScore
	if projectScore < 1 {
		projectScore = 1
	} else if projectScore > 10 {
		projectScore = 10
	}

	// Validate required fields
	if aiResponse.CVFeedback == "" {
		aiResponse.CVFeedback = "No feedback provided"
	}
	if aiResponse.ProjectFeedback == "" {
		aiResponse.ProjectFeedback = "No feedback provided"
	}
	if aiResponse.OverallSummary == "" {
		aiResponse.OverallSummary = "No summary provided"
	}

	return domain.Result{
		JobID:           jobID,
		CVMatchRate:     cvMatchRate,
		CVFeedback:      aiResponse.CVFeedback,
		ProjectScore:    projectScore,
		ProjectFeedback: aiResponse.ProjectFeedback,
		OverallSummary:  aiResponse.OverallSummary,
		CreatedAt:       time.Now(),
	}, nil
}

// validateNoCoTLeakageEnhanced validates that the response doesn't contain chain-of-thought patterns.
func validateNoCoTLeakageEnhanced(response string) error {
	// Enhanced CoT patterns that indicate step-by-step reasoning
	cotPatterns := []string{
		"Step 1:", "Step 2:", "Step 3:", "Step 4:", "Step 5:",
		"First,", "Second,", "Third,", "Fourth,", "Fifth,",
		"I think", "I believe", "I consider", "I evaluate",
		"Let me analyze", "Let me evaluate", "Let me consider",
		"Here's my analysis", "Here's my evaluation", "Here's how I",
		"Now I'll", "Next I'll", "Then I'll",
		"After analyzing", "After reviewing", "After considering",
		"Before I proceed", "Before we continue", "Before proceeding",
		"In conclusion", "To summarize", "To conclude",
		"Looking at this", "Examining this", "Reviewing this",
		"On one hand", "On the other hand",
		"First of all", "Secondly", "Thirdly", "Finally",
		"Let me explain", "Let me clarify", "Let me elaborate",
		"Reasoning:", "Analysis:", "Process:", "Method:",
		"Based on my analysis", "According to my evaluation", "Given my assessment",
	}

	responseLower := strings.ToLower(response)
	for _, pattern := range cotPatterns {
		if strings.Contains(responseLower, strings.ToLower(pattern)) {
			return fmt.Errorf("CoT leakage detected: %s", pattern)
		}
	}
	return nil
}
