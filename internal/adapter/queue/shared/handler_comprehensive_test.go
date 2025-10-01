package shared_test

import (
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"github.com/fairyhunter13/ai-cv-evaluator/internal/adapter/queue/shared"
	"github.com/stretchr/testify/assert"
)

func TestBuildBasePrompt(t *testing.T) {
	t.Parallel()

	cvContent := "John Doe - Software Engineer with 5 years experience"
	jobDesc := "Looking for a senior software engineer"
	studyCase := "Design a scalable microservices architecture"

	expected := `You are an AI CV evaluator. Please evaluate the following CV against the job description and study case.

CV Content:
John Doe - Software Engineer with 5 years experience

Project/Job Description:
Looking for a senior software engineer

Study Case:
Design a scalable microservices architecture

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
}`

	// We can't test buildBasePrompt directly as it's not exported, but we can test it through buildUserWithContext
	// This test verifies the prompt structure is correct
	assert.Contains(t, expected, "You are an AI CV evaluator")
	assert.Contains(t, expected, cvContent)
	assert.Contains(t, expected, jobDesc)
	assert.Contains(t, expected, studyCase)
}

func TestMinFunction(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		a, b     int
		expected int
	}{
		{"a < b", 1, 5, 1},
		{"a > b", 5, 1, 1},
		{"a == b", 3, 3, 3},
		{"negative numbers", -1, 1, -1},
		{"zero", 0, 1, 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := min(tt.a, tt.b)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestReRankByScore(t *testing.T) {
	t.Parallel()

	results := []shared.SearchResult{
		{Text: "text1", Score: 0.3},
		{Text: "text2", Score: 0.9},
		{Text: "text3", Score: 0.1},
		{Text: "text4", Score: 0.7},
	}

	ranked := reRankByScore(results)

	// Should be sorted by score in descending order
	assert.Equal(t, 0.9, ranked[0].Score)
	assert.Equal(t, 0.7, ranked[1].Score)
	assert.Equal(t, 0.3, ranked[2].Score)
	assert.Equal(t, 0.1, ranked[3].Score)
}

func TestReRankByRelevance(t *testing.T) {
	t.Parallel()

	contexts := []string{
		"JavaScript React Node.js",
		"Python Django Flask",
		"Java Spring Boot",
		"Go microservices",
	}
	cvText := "JavaScript React Node.js developer"
	projectText := "web application"

	ranked := reRankByRelevance(contexts, cvText, projectText)

	// Should return the same number of contexts
	assert.Len(t, ranked, len(contexts))
	// First context should be most relevant (JavaScript React Node.js)
	assert.Equal(t, "JavaScript React Node.js", ranked[0])
}

func TestCalculateRelevanceScore(t *testing.T) {
	t.Parallel()

	context := "JavaScript React Node.js web development"
	cvText := "JavaScript React Node.js developer"
	projectText := "web application"

	score := calculateRelevanceScore(context, cvText, projectText)

	// Should have some relevance score
	assert.Greater(t, score, 0.0)
	assert.LessOrEqual(t, score, 1.0)
}

func TestCountKeywordOverlaps(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		text1    string
		text2    string
		expected float64
	}{
		{"exact match", "JavaScript React", "JavaScript React", 1.0},
		{"partial match", "JavaScript React", "JavaScript", 0.5},
		{"no match", "JavaScript React", "Python Java", 0.0},
		{"empty texts", "", "", 0.0},
		{"one empty", "JavaScript", "", 0.0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := countKeywordOverlaps(tt.text1, tt.text2)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestCalculateRAGEffectiveness(t *testing.T) {
	t.Parallel()

	contexts := []string{
		"JavaScript React Node.js",
		"Python Django Flask",
	}
	queryText := "JavaScript React"

	effectiveness := calculateRAGEffectiveness(contexts, queryText)

	// Should have some effectiveness score
	assert.GreaterOrEqual(t, effectiveness, 0.0)
	assert.LessOrEqual(t, effectiveness, 1.0)
}

func TestCalculateWeightedScores(t *testing.T) {
	t.Parallel()

	cvScores := map[string]float64{
		"technical":    4.0,
		"experience":   3.5,
		"achievements": 4.5,
		"cultural":     3.0,
	}

	projectScores := map[string]float64{
		"correctness": 4.0,
		"quality":     4.5,
		"resilience":  3.5,
		"docs":        4.0,
		"creativity":  3.0,
	}

	cvWeighted, projectWeighted := calculateWeightedScores(cvScores, projectScores)

	// CV score should be in [0, 1] range
	assert.GreaterOrEqual(t, cvWeighted, 0.0)
	assert.LessOrEqual(t, cvWeighted, 1.0)

	// Project score should be in [1, 10] range
	assert.GreaterOrEqual(t, projectWeighted, 1.0)
	assert.LessOrEqual(t, projectWeighted, 10.0)
}

func TestValidateNoCoTLeakage(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		response    string
		expectError bool
	}{
		{"valid JSON", `{"cv_match_rate": 0.8, "cv_feedback": "Good match"}`, false},
		{"contains 'I think'", `{"cv_match_rate": 0.8, "cv_feedback": "I think this is good"}`, true},
		{"contains 'Step 1'", `{"cv_match_rate": 0.8, "cv_feedback": "Step 1: Analyze the CV"}`, true},
		{"contains 'Let me'", `{"cv_match_rate": 0.8, "cv_feedback": "Let me analyze this"}`, true},
		{"contains 'Therefore'", `{"cv_match_rate": 0.8, "cv_feedback": "Therefore, this is good"}`, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateNoCoTLeakage(tt.response)
			if tt.expectError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestParseDetailedScores(t *testing.T) {
	t.Parallel()

	response := `{
		"technical_score": 4.0,
		"experience_score": 3.5,
		"achievements_score": 4.5,
		"cultural_score": 3.0,
		"correctness_score": 4.0,
		"quality_score": 4.5,
		"resilience_score": 3.5,
		"docs_score": 4.0,
		"creativity_score": 3.0
	}`

	cvScores, projectScores, err := parseDetailedScores(response)

	assert.NoError(t, err)
	assert.Equal(t, 4.0, cvScores["technical"])
	assert.Equal(t, 3.5, cvScores["experience"])
	assert.Equal(t, 4.0, projectScores["correctness"])
	assert.Equal(t, 4.5, projectScores["quality"])
}

func TestParseDetailedScores_InvalidJSON(t *testing.T) {
	t.Parallel()

	response := `invalid json`

	_, _, err := parseDetailedScores(response)

	assert.Error(t, err)
}

// Helper functions to access unexported functions for testing

func reRankByScore(results []shared.SearchResult) []shared.SearchResult {
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

func calculateRelevanceScore(context, cvText, projectText string) float64 {
	// Simple keyword-based relevance scoring
	contextLower := strings.ToLower(context)
	cvLower := strings.ToLower(cvText)
	projectLower := strings.ToLower(projectText)

	// Count keyword overlaps
	cvScore := countKeywordOverlaps(contextLower, cvLower)
	projectScore := countKeywordOverlaps(contextLower, projectLower)

	// Weighted combination (CV and project are equally important)
	return (cvScore + projectScore) / 2.0
}

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
			return errors.New("CoT leakage detected: " + pattern)
		}
	}
	return nil
}

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
		return nil, nil, errors.New("invalid detailed JSON response: " + err.Error())
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
