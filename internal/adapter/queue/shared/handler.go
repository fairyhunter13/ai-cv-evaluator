// Package shared provides common queue handling functionality.
package shared

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/fairyhunter13/ai-cv-evaluator/internal/adapter/observability"
	qdrantcli "github.com/fairyhunter13/ai-cv-evaluator/internal/adapter/vector/qdrant"
	"github.com/fairyhunter13/ai-cv-evaluator/internal/domain"
	"go.opentelemetry.io/otel"
)

// SearchResult represents a search result with text and score.
type SearchResult struct {
	Text  string
	Score float64
}

// HandleEvaluate processes an evaluation task with the given dependencies.
// This is the shared evaluation logic that was previously in the asynq package.
func HandleEvaluate(
	ctx context.Context,
	jobs domain.JobRepository,
	uploads domain.UploadRepository,
	results domain.ResultRepository,
	ai domain.AIClient,
	q *qdrantcli.Client,
	payload domain.EvaluateTaskPayload,
) error {
	tracer := otel.Tracer("queue.handler")
	ctx, span := tracer.Start(ctx, "HandleEvaluate")
	defer span.End()

	// Check for nil dependencies
	if jobs == nil {
		return fmt.Errorf("job repository is nil")
	}
	if uploads == nil {
		return fmt.Errorf("upload repository is nil")
	}
	if results == nil {
		return fmt.Errorf("result repository is nil")
	}
	if ai == nil {
		return fmt.Errorf("AI client is nil")
	}

	observability.StartProcessingJob("evaluate")
	slog.Info("starting evaluation", slog.String("job_id", payload.JobID))

	// Update job status to processing
	if err := jobs.UpdateStatus(ctx, payload.JobID, domain.JobProcessing, nil); err != nil {
		slog.Error("failed to update job status to processing", slog.String("job_id", payload.JobID), slog.Any("error", err))
		return fmt.Errorf("update job status: %w", err)
	}

	// Get CV and project content
	cvUpload, err := uploads.Get(ctx, payload.CVID)
	if err != nil {
		slog.Error("failed to get CV content", slog.String("job_id", payload.JobID), slog.String("cv_id", payload.CVID), slog.Any("error", err))
		_ = jobs.UpdateStatus(ctx, payload.JobID, domain.JobFailed, ptr("failed to get CV content"))
		return fmt.Errorf("get CV content: %w", err)
	}

	projectUpload, err := uploads.Get(ctx, payload.ProjectID)
	if err != nil {
		slog.Error("failed to get project content", slog.String("job_id", payload.JobID), slog.String("project_id", payload.ProjectID), slog.Any("error", err))
		_ = jobs.UpdateStatus(ctx, payload.JobID, domain.JobFailed, ptr("failed to get project content"))
		return fmt.Errorf("get project content: %w", err)
	}

	// Build user prompt with RAG context (if qdrant is available) and chaining (always on by default)
	userPrompt, err := buildUserWithContext(ctx, ai, q, cvUpload.Text, projectUpload.Text, payload.JobDescription, payload.StudyCaseBrief, payload.ScoringRubric)
	if err != nil {
		slog.Error("failed to build user prompt", slog.String("job_id", payload.JobID), slog.Any("error", err))
		_ = jobs.UpdateStatus(ctx, payload.JobID, domain.JobFailed, ptr("failed to build user prompt"))
		return fmt.Errorf("build user prompt: %w", err)
	}

	// Perform AI evaluation (two-pass by default)
	result, err := performEvaluation(ctx, ai, userPrompt, payload.JobID)
	if err != nil {
		slog.Error("evaluation failed", slog.String("job_id", payload.JobID), slog.Any("error", err))
		_ = jobs.UpdateStatus(ctx, payload.JobID, domain.JobFailed, ptr("evaluation failed"))
		return fmt.Errorf("evaluation: %w", err)
	}

	// Update job status to completed BEFORE storing result
	if err := jobs.UpdateStatus(ctx, payload.JobID, domain.JobCompleted, nil); err != nil {
		slog.Error("failed to update job status to completed", slog.String("job_id", payload.JobID), slog.Any("error", err))
		return fmt.Errorf("update job status: %w", err)
	}

	// Store result
	if err := results.Upsert(ctx, result); err != nil {
		slog.Error("failed to store result", slog.String("job_id", payload.JobID), slog.Any("error", err))
		_ = jobs.UpdateStatus(ctx, payload.JobID, domain.JobFailed, ptr("failed to store result"))
		return fmt.Errorf("store result: %w", err)
	}

	observability.CompleteJob("evaluate")
	slog.Info("job completed", slog.String("job_id", payload.JobID))
	return nil
}

// buildUserWithContext enriches the user prompt with optional RAG context and/or
// chaining via extraction passes. If any step fails, it gracefully falls back
// to the provided defaultUser prompt.
func buildUserWithContext(
	ctx context.Context,
	ai domain.AIClient,
	q *qdrantcli.Client,
	cvContent, projectContent, jobDesc, studyCase, scoringRubric string,
) (string, error) {
	// Start with base prompt
	basePrompt := buildBasePrompt(cvContent, projectContent, jobDesc, studyCase, scoringRubric)

	// If no RAG and chaining still proceeds with extraction over base prompt
	// RAG is only used when qdrant client is available. Chaining is always enabled by default.

	// RAG context retrieval (if Qdrant is available)
	var ragContext string
	if q != nil {
		context, err := retrieveRAGContext(ctx, ai, q, cvContent, projectContent)
		if err != nil {
			slog.Warn("RAG context retrieval failed, using base prompt", slog.Any("error", err))
		} else {
			ragContext = context
		}
	}

	// Chaining via extraction pass (always enabled by default)
	extractedContext, err := performExtractionPass(ctx, ai, basePrompt, ragContext)
	if err != nil {
		slog.Warn("extraction pass failed, using base prompt", slog.Any("error", err))
		return basePrompt, nil
	}
	return extractedContext, nil
}

// buildBasePrompt creates the base evaluation prompt.
func buildBasePrompt(cvContent, _ string, jobDesc, studyCase, scoringRubric string) string {
	return fmt.Sprintf(`You are an AI CV evaluator. Please evaluate the following CV against the job description and study case.

CV Content:
%s

Project/Job Description:
%s

Study Case:
%s

Scoring Rubric:
%s

Please provide:
1. CV Match Rate (0-1 scale)
2. CV Feedback
3. Project Score (1-10 scale)
4. Project Feedback
5. Overall Summary

Format your response as JSON with the following structure:
{
  "cv_match_rate": 0.85,
  "cv_feedback": "The candidate shows strong technical skills...",
  "project_score": 8.5,
  "project_feedback": "The project demonstrates good understanding...",
  "overall_summary": "Overall, this is a strong candidate..."
}`, cvContent, jobDesc, studyCase, scoringRubric)
}

// retrieveRAGContext retrieves relevant context using enhanced RAG with re-ranking.
func retrieveRAGContext(ctx context.Context, ai domain.AIClient, q *qdrantcli.Client, cvContent, projectContent string) (string, error) {
	slog.Info("retrieving RAG context", slog.Int("cv_length", len(cvContent)), slog.Int("project_length", len(projectContent)))

	// Generate embeddings for CV and project content
	// We'll use the first 1000 characters to avoid token limits
	cvText := cvContent
	if len(cvText) > 1000 {
		cvText = cvText[:1000]
	}
	projectText := projectContent
	if len(projectText) > 1000 {
		projectText = projectText[:1000]
	}

	// Enhanced RAG with re-ranking and top-k=5 retrieval
	var contexts []string

	// Search job_description collection for CV matching context with top-k=5
	jobContext, err := searchCollectionAdvanced(ctx, ai, q, "job_description", cvText, 5)
	if err != nil {
		slog.Warn("failed to search job_description collection", slog.Any("error", err))
		observability.RecordRAGRetrievalError("job_description", "search_failed")
	} else {
		contexts = append(contexts, jobContext...)
		// Record effectiveness for job description retrieval
		effectiveness := calculateRAGEffectiveness(jobContext, cvText)
		observability.RecordRAGEffectiveness("job_description", "cv_match", effectiveness)
	}

	// Search scoring_rubric collection for project evaluation context with top-k=5
	rubricContext, err := searchCollectionAdvanced(ctx, ai, q, "scoring_rubric", projectText, 5)
	if err != nil {
		slog.Warn("failed to search scoring_rubric collection", slog.Any("error", err))
		observability.RecordRAGRetrievalError("scoring_rubric", "search_failed")
	} else {
		contexts = append(contexts, rubricContext...)
		// Record effectiveness for scoring rubric retrieval
		effectiveness := calculateRAGEffectiveness(rubricContext, projectText)
		observability.RecordRAGEffectiveness("scoring_rubric", "project_eval", effectiveness)
	}

	if len(contexts) == 0 {
		slog.Info("no RAG context found")
		return "", nil
	}

	// Re-rank results by relevance and weight
	rankedContexts := reRankByRelevance(contexts, cvText, projectText)

	// Combine and return relevant context (limit to top 3 for token efficiency)
	topContexts := rankedContexts
	if len(topContexts) > 3 {
		topContexts = topContexts[:3]
	}

	combinedContext := strings.Join(topContexts, "\n\n")
	slog.Info("retrieved enhanced RAG context",
		slog.Int("context_count", len(topContexts)),
		slog.Int("total_length", len(combinedContext)),
		slog.Int("original_count", len(contexts)))

	return combinedContext, nil
}

// searchCollectionAdvanced performs enhanced search with weight-aware re-ranking.
func searchCollectionAdvanced(ctx context.Context, ai domain.AIClient, q *qdrantcli.Client, collection, queryText string, topK int) ([]string, error) {
	slog.Info("searching collection with advanced ranking", slog.String("collection", collection), slog.String("query", queryText[:minInt(50, len(queryText))]))

	// Generate embeddings for the query text
	embeddings, err := ai.Embed(ctx, []string{queryText})
	if err != nil {
		return nil, fmt.Errorf("failed to generate embeddings: %w", err)
	}

	if len(embeddings) == 0 {
		return nil, fmt.Errorf("no embeddings generated")
	}

	// Search Qdrant for similar vectors with extended top-k for re-ranking
	extendedTopK := topK * 2 // Get more results for re-ranking
	results, err := q.Search(ctx, collection, embeddings[0], extendedTopK)
	if err != nil {
		return nil, fmt.Errorf("failed to search collection %s: %w", collection, err)
	}

	// Extract text and scores from search results

	var searchResults []SearchResult
	for _, result := range results {
		if payload, ok := result["payload"].(map[string]any); ok {
			if text, ok := payload["text"].(string); ok && text != "" {
				// Extract score if available
				score := 0.0
				if scoreVal, ok := result["score"].(float64); ok {
					score = scoreVal
				}
				searchResults = append(searchResults, SearchResult{Text: text, Score: score})
			}
		}
	}

	// Re-rank by relevance score and weight
	rankedResults := reRankByScore(searchResults)

	// Return top-k results
	contexts := make([]string, 0, topK)
	for i, result := range rankedResults {
		if i >= topK {
			break
		}
		contexts = append(contexts, result.Text)
	}

	slog.Info("advanced search completed", slog.String("collection", collection), slog.Int("results", len(contexts)), slog.Int("total_candidates", len(searchResults)))
	return contexts, nil
}

// searchCollection searches a Qdrant collection for similar documents (legacy function).
func searchCollection(ctx context.Context, ai domain.AIClient, q *qdrantcli.Client, collection, queryText string, topK int) ([]string, error) {
	slog.Info("searching collection", slog.String("collection", collection), slog.String("query", queryText[:minInt(50, len(queryText))]))

	// Generate embeddings for the query text
	embeddings, err := ai.Embed(ctx, []string{queryText})
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

	// Extract text from search results
	var contexts []string
	for _, result := range results {
		if payload, ok := result["payload"].(map[string]any); ok {
			if text, ok := payload["text"].(string); ok && text != "" {
				contexts = append(contexts, text)
			}
		}
	}

	slog.Info("search completed", slog.String("collection", collection), slog.Int("results", len(contexts)))
	return contexts, nil
}

func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// performExtractionPass performs an extraction pass for chaining.
func performExtractionPass(ctx context.Context, ai domain.AIClient, basePrompt, ragContext string) (string, error) {
	slog.Info("performing extraction pass for AI chaining")

	// System prompt for extraction
	extractionPrompt := `You are an expert CV analyzer. Extract key insights from the provided CV and project information.

Focus on:
1. Technical skills and technologies mentioned
2. Experience level and years of experience  
3. Project complexity and achievements
4. Relevant accomplishments and impact

Provide a concise summary of key insights that will help with evaluation.`

	// Combine base prompt with RAG context if available
	extractionInput := basePrompt
	if ragContext != "" {
		extractionInput = fmt.Sprintf("%s\n\nAdditional Context:\n%s", basePrompt, ragContext)
	}

	// Call AI for extraction
	extractedInsights, err := ai.ChatJSON(ctx, extractionPrompt, extractionInput, 500)
	if err != nil {
		slog.Warn("extraction pass failed", slog.Any("error", err))
		return basePrompt, nil // Fallback to base prompt
	}

	// Build enhanced prompt with extracted insights
	enhancedPrompt := fmt.Sprintf(`%s

Key Insights from Analysis:
%s

Please use these insights to provide a more informed evaluation.`, basePrompt, extractedInsights)

	slog.Info("extraction pass completed", slog.Int("insights_length", len(extractedInsights)))
	return enhancedPrompt, nil
}

// performEvaluation performs the actual AI evaluation.
func performEvaluation(ctx context.Context, ai domain.AIClient, userPrompt string, jobID string) (domain.Result, error) {
	slog.Info("performing AI evaluation (two-pass default)", slog.String("job_id", jobID))
	return performTwoPassEvaluation(ctx, ai, userPrompt, jobID)
}

// performTwoPassEvaluation implements true two-pass processing with normalization.
func performTwoPassEvaluation(ctx context.Context, ai domain.AIClient, userPrompt string, jobID string) (domain.Result, error) {
	slog.Info("performing two-pass evaluation", slog.String("job_id", jobID))

	// Pass 1: Raw evaluation with detailed analysis
	rawSystemPrompt := `You are an expert CV evaluator. Perform a detailed analysis of the provided CV and project against the job requirements and study case.

You must respond with valid JSON only. Do not include any reasoning, explanations, or chain-of-thought in your response.

Required JSON structure:
{
  "technical_score": 4.2,
  "experience_score": 3.8,
  "achievements_score": 4.5,
  "cultural_score": 3.9,
  "correctness_score": 4.1,
  "quality_score": 4.3,
  "resilience_score": 3.7,
  "docs_score": 4.0,
  "creativity_score": 3.6,
  "cv_feedback": "Detailed feedback on CV match",
  "project_feedback": "Detailed feedback on project quality", 
  "overall_summary": "Comprehensive summary of the candidate's fit"
}

Scoring guidelines:
- All scores: 1.0 to 5.0 (1 = poor, 5 = excellent)
- All text fields must be detailed and professional
- Focus on technical skills, experience relevance, and project quality`

	// Pass 1: Raw evaluation
	rawResponse, err := ai.ChatJSON(ctx, rawSystemPrompt, userPrompt, 1500)
	if err != nil {
		slog.Error("AI raw evaluation failed", slog.String("job_id", jobID), slog.Any("error", err))
		return domain.Result{}, fmt.Errorf("AI raw evaluation failed: %w", err)
	}

	// Pass 2: Normalization and validation
	normalizationPrompt := `You are a scoring normalization expert. Take the detailed evaluation results and normalize them according to the specified weights and ranges.

You must respond with valid JSON only. Do not include any reasoning, explanations, or chain-of-thought in your response.

Required JSON structure:
{
  "cv_match_rate": 0.85,
  "cv_feedback": "Brief 1-3 sentence feedback on CV match",
  "project_score": 8.5,
  "project_feedback": "Brief 1-3 sentence feedback on project quality", 
  "overall_summary": "3-5 sentence comprehensive summary of the candidate's fit"
}

Normalization rules:
- CV Match: Technical(40%) + Experience(25%) + Achievements(20%) + Cultural(15%) → weighted average (1–5) → normalize to [0,1] by dividing by 5 (×0.2)
- Project Score: Correctness(30%) + Quality(25%) + Resilience(20%) + Docs(15%) + Creativity(10%) → weighted average (1–5) → normalize ×2 → clamp to [1.0,10.0]
- cv_match_rate: 0.0 to 1.0 (0 = no match, 1 = perfect match)
- project_score: 1.0 to 10.0 (1 = poor, 10 = excellent)
- All text fields must be concise and professional`

	// Parse detailed scores from raw response for weighted calculation
	cvScores, projectScores, err := parseDetailedScores(rawResponse)
	if err != nil {
		slog.Warn("failed to parse detailed scores, using AI normalization", slog.String("job_id", jobID), slog.Any("error", err))

		// Fallback to AI normalization
		normalizedResponse, err := ai.ChatJSON(ctx, normalizationPrompt, rawResponse, 800)
		if err != nil {
			slog.Error("AI normalization failed", slog.String("job_id", jobID), slog.Any("error", err))
			return domain.Result{}, fmt.Errorf("AI normalization failed: %w", err)
		}

		// Parse and validate normalized response
		result, err := parseEvaluationResponse(normalizedResponse, jobID)
		if err != nil {
			slog.Error("failed to parse normalized AI response", slog.String("job_id", jobID), slog.String("response", normalizedResponse), slog.Any("error", err))
			return domain.Result{}, fmt.Errorf("parse normalized AI response: %w", err)
		}

		slog.Info("two-pass AI evaluation completed with AI normalization", slog.String("job_id", jobID),
			slog.Float64("cv_match_rate", result.CVMatchRate),
			slog.Float64("project_score", result.ProjectScore))

		return result, nil
	}

	// Calculate weighted scores
	cvWeighted, projectWeighted := calculateWeightedScores(cvScores, projectScores)

	// Parse text fields from raw response
	var rawResponseData struct {
		CVFeedback      string `json:"cv_feedback"`
		ProjectFeedback string `json:"project_feedback"`
		OverallSummary  string `json:"overall_summary"`
	}

	if err := json.Unmarshal([]byte(rawResponse), &rawResponseData); err != nil {
		slog.Warn("failed to parse text fields from raw response", slog.String("job_id", jobID), slog.Any("error", err))
		rawResponseData.CVFeedback = "No feedback provided"
		rawResponseData.ProjectFeedback = "No feedback provided"
		rawResponseData.OverallSummary = "No summary provided"
	}

	// Create result with weighted scores
	result := domain.Result{
		JobID:           jobID,
		CVMatchRate:     cvWeighted,
		CVFeedback:      rawResponseData.CVFeedback,
		ProjectScore:    projectWeighted,
		ProjectFeedback: rawResponseData.ProjectFeedback,
		OverallSummary:  rawResponseData.OverallSummary,
		CreatedAt:       time.Now(),
	}

	slog.Info("two-pass AI evaluation completed", slog.String("job_id", jobID),
		slog.Float64("cv_match_rate", result.CVMatchRate),
		slog.Float64("project_score", result.ProjectScore))

	return result, nil
}

// performSinglePassEvaluation implements single-pass evaluation (original logic).
func performSinglePassEvaluation(ctx context.Context, ai domain.AIClient, userPrompt string, jobID string) (domain.Result, error) {
	slog.Info("performing single-pass evaluation", slog.String("job_id", jobID))

	// System prompt for structured JSON output
	systemPrompt := `You are an expert CV evaluator. Analyze the provided CV and project against the job requirements and study case.

You must respond with valid JSON only. Do not include any reasoning, explanations, or chain-of-thought in your response.

Required JSON structure:
{
  "cv_match_rate": 0.85,
  "cv_feedback": "Brief 1-3 sentence feedback on CV match",
  "project_score": 8.5,
  "project_feedback": "Brief 1-3 sentence feedback on project quality", 
  "overall_summary": "3-5 sentence comprehensive summary of the candidate's fit"
}

Scoring guidelines:
- cv_match_rate: 0.0 to 1.0 (0 = no match, 1 = perfect match)
- project_score: 1.0 to 10.0 (1 = poor, 10 = excellent)
- All text fields must be concise and professional
- Focus on technical skills, experience relevance, and project quality`

	// Call AI service with structured prompt
	response, err := ai.ChatJSON(ctx, systemPrompt, userPrompt, 1000)
	if err != nil {
		slog.Error("AI evaluation failed", slog.String("job_id", jobID), slog.Any("error", err))
		return domain.Result{}, fmt.Errorf("AI evaluation failed: %w", err)
	}

	// Parse and validate JSON response
	result, err := parseEvaluationResponse(response, jobID)
	if err != nil {
		slog.Error("failed to parse AI response", slog.String("job_id", jobID), slog.String("response", response), slog.Any("error", err))
		return domain.Result{}, fmt.Errorf("parse AI response: %w", err)
	}

	slog.Info("single-pass AI evaluation completed", slog.String("job_id", jobID),
		slog.Float64("cv_match_rate", result.CVMatchRate),
		slog.Float64("project_score", result.ProjectScore))

	return result, nil
}

// parseEvaluationResponse parses and validates the AI response JSON.
func parseEvaluationResponse(response, jobID string) (domain.Result, error) {
	// Clean the response - remove any markdown code blocks or extra text
	response = strings.TrimSpace(response)
	response = strings.TrimPrefix(response, "```json")
	response = strings.TrimPrefix(response, "```")
	response = strings.TrimSuffix(response, "```")
	response = strings.TrimSpace(response)

	// Validate no chain-of-thought leakage
	if err := validateNoCoTLeakage(response); err != nil {
		slog.Error("CoT leakage detected in AI response", slog.String("job_id", jobID), slog.String("response", response), slog.Any("error", err))
		return domain.Result{}, fmt.Errorf("CoT leakage detected: %w", err)
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

	// Validate and clamp values
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

// validateNoCoTLeakage validates that the response doesn't contain chain-of-thought patterns.
func validateNoCoTLeakage(response string) error {
	cotPatterns := []string{
		"Step 1", "Step 2", "Step 3", "Step 4", "Step 5",
		"First,", "Second,", "Third,", "Fourth,", "Fifth,",
		"I think", "I believe", "I consider", "I evaluate",
		"Let me", "Let's", "To begin", "To start",
		"Reasoning:", "Analysis:", "Process:", "Method:",
		"Therefore", "Thus", "Hence", "So",
		"Based on", "According to", "Given that",
		"Let me analyze", "Let me evaluate", "Let me consider",
		"Here's my", "Here's the", "Here's how",
		"Now I'll", "Next I'll", "Then I'll",
		"After analyzing", "After reviewing", "After considering",
		"Before I", "Before we", "Before proceeding",
		"In conclusion", "To summarize", "To conclude",
		"Looking at", "Examining", "Reviewing",
		"On one hand", "On the other hand",
		"However", "Moreover", "Furthermore", "Additionally",
		"First of all", "Secondly", "Thirdly", "Finally",
		"To be honest", "To be fair", "To be clear",
		"Let me explain", "Let me clarify", "Let me elaborate",
	}

	responseLower := strings.ToLower(response)
	for _, pattern := range cotPatterns {
		if strings.Contains(responseLower, strings.ToLower(pattern)) {
			return fmt.Errorf("CoT leakage detected: %s", pattern)
		}
	}
	return nil
}

// reRankByScore re-ranks search results by score in descending order.
func reRankByScore(results []SearchResult) []SearchResult {
	// Simple sort by score (descending)
	for i := 0; i < len(results)-1; i++ {
		for j := i + 1; j < len(results); j++ {
			if results[i].Score < results[j].Score {
				results[i], results[j] = results[j], results[i]
			}
		}
	}
	return results
}

// reRankByRelevance re-ranks contexts by relevance to both CV and project content.
func reRankByRelevance(contexts []string, cvText, projectText string) []string {
	if len(contexts) <= 1 {
		return contexts
	}

	// Simple relevance scoring based on keyword overlap
	type ContextScore struct {
		Text  string
		Score float64
	}

	var scoredContexts []ContextScore
	for _, context := range contexts {
		score := calculateRelevanceScore(context, cvText, projectText)
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

	// Extract re-ranked contexts
	rankedContexts := make([]string, len(scoredContexts))
	for i, cs := range scoredContexts {
		rankedContexts[i] = cs.Text
	}

	return rankedContexts
}

// calculateRelevanceScore calculates a relevance score for a context.
func calculateRelevanceScore(context, cvText, projectText string) float64 {
	// Simple keyword-based relevance scoring
	// In a production system, this would use more sophisticated NLP techniques

	contextLower := strings.ToLower(context)
	cvLower := strings.ToLower(cvText)
	projectLower := strings.ToLower(projectText)

	// Count keyword overlaps
	cvScore := countKeywordOverlaps(contextLower, cvLower)
	projectScore := countKeywordOverlaps(contextLower, projectLower)

	// Weighted combination (CV and project are equally important)
	return (cvScore + projectScore) / 2.0
}

// countKeywordOverlaps counts overlapping keywords between two texts.
func countKeywordOverlaps(text1, text2 string) float64 {
	words1 := strings.Fields(text1)
	words2 := strings.Fields(text2)

	if len(words1) == 0 || len(words2) == 0 {
		return 0.0
	}

	// Create word frequency maps
	freq1 := make(map[string]int)
	freq2 := make(map[string]int)

	for _, word := range words1 {
		if len(word) > 2 { // Ignore very short words
			freq1[word]++
		}
	}

	for _, word := range words2 {
		if len(word) > 2 { // Ignore very short words
			freq2[word]++
		}
	}

	// Count overlaps
	overlaps := 0
	totalWords := len(words1)

	for word := range freq1 {
		if freq2[word] > 0 {
			overlaps++
		}
	}

	if totalWords == 0 {
		return 0.0
	}

	return float64(overlaps) / float64(totalWords)
}

// calculateRAGEffectiveness calculates the effectiveness of RAG retrieval.
func calculateRAGEffectiveness(contexts []string, queryText string) float64 {
	if len(contexts) == 0 {
		return 0.0
	}

	// Calculate average relevance score for all contexts
	totalScore := 0.0
	for _, context := range contexts {
		score := calculateRelevanceScore(context, queryText, queryText)
		totalScore += score
	}

	return totalScore / float64(len(contexts))
}

// calculateWeightedScores calculates weighted scores according to project specification.
func calculateWeightedScores(cvScores, projectScores map[string]float64) (float64, float64) {
	// CV Match: Technical(40%) + Experience(25%) + Achievements(20%) + Cultural(15%) → weighted average (1–5) → normalize to [0,1] by dividing by 5 (×0.2)
	cvWeighted := (cvScores["technical"]*0.4 + cvScores["experience"]*0.25 +
		cvScores["achievements"]*0.2 + cvScores["cultural"]*0.15) * 0.2

	// Project Score: Correctness(30%) + Quality(25%) + Resilience(20%) + Docs(15%) + Creativity(10%) → weighted average (1–5) → normalize ×2 → clamp to [1.0,10.0]
	projectWeighted := (projectScores["correctness"]*0.3 + projectScores["quality"]*0.25 +
		projectScores["resilience"]*0.2 + projectScores["docs"]*0.15 +
		projectScores["creativity"]*0.1) * 2

	// Clamp project score to [1.0, 10.0]
	if projectWeighted < 1.0 {
		projectWeighted = 1.0
	} else if projectWeighted > 10.0 {
		projectWeighted = 10.0
	}

	// Clamp CV score to [0.0, 1.0]
	if cvWeighted < 0.0 {
		cvWeighted = 0.0
	} else if cvWeighted > 1.0 {
		cvWeighted = 1.0
	}

	return cvWeighted, projectWeighted
}

// parseDetailedScores parses detailed scores from two-pass evaluation response.
func parseDetailedScores(response string) (map[string]float64, map[string]float64, error) {
	// Clean the response
	response = strings.TrimSpace(response)
	response = strings.TrimPrefix(response, "```json")
	response = strings.TrimPrefix(response, "```")
	response = strings.TrimSuffix(response, "```")
	response = strings.TrimSpace(response)

	// Parse JSON response with detailed scores
	var detailedResponse struct {
		TechnicalScore    float64 `json:"technical_score"`
		ExperienceScore   float64 `json:"experience_score"`
		AchievementsScore float64 `json:"achievements_score"`
		CulturalScore     float64 `json:"cultural_score"`
		CorrectnessScore  float64 `json:"correctness_score"`
		QualityScore      float64 `json:"quality_score"`
		ResilienceScore   float64 `json:"resilience_score"`
		DocsScore         float64 `json:"docs_score"`
		CreativityScore   float64 `json:"creativity_score"`
	}

	if err := json.Unmarshal([]byte(response), &detailedResponse); err != nil {
		return nil, nil, fmt.Errorf("invalid detailed JSON response: %w", err)
	}

	// Build score maps
	cvScores := map[string]float64{
		"technical":    detailedResponse.TechnicalScore,
		"experience":   detailedResponse.ExperienceScore,
		"achievements": detailedResponse.AchievementsScore,
		"cultural":     detailedResponse.CulturalScore,
	}

	projectScores := map[string]float64{
		"correctness": detailedResponse.CorrectnessScore,
		"quality":     detailedResponse.QualityScore,
		"resilience":  detailedResponse.ResilienceScore,
		"docs":        detailedResponse.DocsScore,
		"creativity":  detailedResponse.CreativityScore,
	}

	return cvScores, projectScores, nil
}

func ptr(s string) *string { return &s }
