// Package shared provides scoring rubric integration with full project.md conformance.
package shared

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"math"
	"strings"
)

// ScoringRubric represents the standardized evaluation parameters from project.md.
type ScoringRubric struct {
	// CV Match Evaluation Parameters (1-5 scale)
	TechnicalSkillsMatch     float64 `json:"technical_skills_match"`     // 40% weight
	ExperienceLevel          float64 `json:"experience_level"`           // 25% weight
	RelevantAchievements     float64 `json:"relevant_achievements"`      // 20% weight
	CulturalCollaborationFit float64 `json:"cultural_collaboration_fit"` // 15% weight

	// Project Deliverable Evaluation Parameters (1-5 scale)
	Correctness              float64 `json:"correctness"`               // 30% weight
	CodeQualityStructure     float64 `json:"code_quality_structure"`    // 25% weight
	ResilienceErrorHandling  float64 `json:"resilience_error_handling"` // 20% weight
	DocumentationExplanation float64 `json:"documentation_explanation"` // 15% weight
	CreativityBonus          float64 `json:"creativity_bonus"`          // 10% weight
}

// CalculateWeightedScores calculates weighted scores according to project.md and domain model requirements.
func CalculateWeightedScores(rubric ScoringRubric) (cvMatchRate, projectScore float64) {
	// CV Match: Technical(40%) + Experience(25%) + Achievements(20%) + Cultural(15%)
	// → weighted average (1–5) → normalize to [0,1] by dividing by 5 (×0.2)
	cvWeighted := (rubric.TechnicalSkillsMatch*0.4 + rubric.ExperienceLevel*0.25 +
		rubric.RelevantAchievements*0.2 + rubric.CulturalCollaborationFit*0.15) * 0.2

	// Project Score: Correctness(30%) + Quality(25%) + Resilience(20%) + Docs(15%) + Creativity(10%)
	// → weighted average (1–5) → normalize ×2 → clamp to [1.0,10.0]
	projectWeighted := (rubric.Correctness*0.3 + rubric.CodeQualityStructure*0.25 +
		rubric.ResilienceErrorHandling*0.2 + rubric.DocumentationExplanation*0.15 +
		rubric.CreativityBonus*0.1) * 2

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

// ValidateScoringRubric validates that all scores are within the 1-5 range.
func ValidateScoringRubric(rubric ScoringRubric) error {
	scores := []struct {
		name  string
		value float64
	}{
		{"technical_skills_match", rubric.TechnicalSkillsMatch},
		{"experience_level", rubric.ExperienceLevel},
		{"relevant_achievements", rubric.RelevantAchievements},
		{"cultural_collaboration_fit", rubric.CulturalCollaborationFit},
		{"correctness", rubric.Correctness},
		{"code_quality_structure", rubric.CodeQualityStructure},
		{"resilience_error_handling", rubric.ResilienceErrorHandling},
		{"documentation_explanation", rubric.DocumentationExplanation},
		{"creativity_bonus", rubric.CreativityBonus},
	}

	for _, score := range scores {
		if score.value < 1.0 || score.value > 5.0 {
			return fmt.Errorf("invalid score for %s: %.2f (must be 1.0-5.0)", score.name, score.value)
		}
	}

	return nil
}

// ParseDetailedScores parses detailed scores from AI response and validates them.
func ParseDetailedScores(response string) (ScoringRubric, error) {
	// Clean the response
	response = strings.TrimSpace(response)
	response = strings.TrimPrefix(response, "```json")
	response = strings.TrimPrefix(response, "```")
	response = strings.TrimSuffix(response, "```")
	response = strings.TrimSpace(response)

	// Parse JSON response with detailed scores
	var rubric ScoringRubric
	if err := json.Unmarshal([]byte(response), &rubric); err != nil {
		return ScoringRubric{}, fmt.Errorf("invalid detailed JSON response: %w", err)
	}

	// Validate scores
	if err := ValidateScoringRubric(rubric); err != nil {
		return ScoringRubric{}, fmt.Errorf("invalid scoring rubric: %w", err)
	}

	return rubric, nil
}

// GenerateScoringPrompt creates a comprehensive scoring prompt based on project.md requirements.
func GenerateScoringPrompt(cvContent, projectContent, jobDesc, studyCase, scoringRubric string) string {
	return fmt.Sprintf(`You are a senior recruitment expert and technical reviewer. Evaluate the candidate's CV and project deliverables using the standardized scoring rubric.

CV Content:
%s

Project Content:
%s

Job Description:
%s

Study Case Requirements:
%s

Scoring Rubric:
%s

Evaluate each parameter on a 1-5 scale according to the project.md requirements:

CV Match Evaluation Parameters:
1. Technical Skills Match (40%% weight): Alignment with job requirements (backend, DB, APIs, cloud, AI/LLM)
   - 1 = Irrelevant → 5 = Excellent + AI/LLM
   - Focus on: Backend languages & frameworks, database experience, API development, cloud technologies, AI/LLM exposure

2. Experience Level (25%% weight): Years of experience, project complexity
   - 1 = <1yr → 5 = 5+ yrs high-impact
   - Focus on: Years of experience, project complexity indicators, leadership experience

3. Relevant Achievements (20%% weight): Impact of past work
   - 1 = None → 5 = Major measurable impact
   - Focus on: Measurable impact, scale and scope of projects, innovation examples

4. Cultural/Collaboration Fit (15%% weight): Communication, learning mindset, teamwork
   - 1 = Not shown → 5 = Excellent
   - Focus on: Communication skills, learning attitude, teamwork evidence

Project Deliverable Evaluation Parameters:
1. Correctness (30%% weight): Implements prompt design, LLM chaining, RAG
   - 1 = Not implemented → 5 = Fully correct
   - Focus on: Prompt design quality, LLM chaining implementation, RAG integration

2. Code Quality & Structure (25%% weight): Clean, modular, reusable, tested
   - 1 = Poor → 5 = Excellent + strong tests
   - Focus on: Code organization, modularity, reusability, test coverage

3. Resilience & Error Handling (20%% weight): Handles jobs, retries, randomness, API failures
   - 1 = Missing → 5 = Robust
   - Focus on: Error handling, retry mechanisms, API failure handling, randomness control

4. Documentation & Explanation (15%% weight): README clarity, setup instructions, trade-offs
   - 1 = Missing → 5 = Excellent
   - Focus on: README quality, setup instructions, trade-off explanations

5. Creativity / Bonus (10%% weight): Extra features beyond requirements
   - 1 = None → 5 = Outstanding creativity
   - Focus on: Additional features, innovative solutions, bonus implementations

CRITICAL: Respond with ONLY valid JSON following this structure:
{
  "technical_skills_match": 4.2,
  "experience_level": 3.8,
  "relevant_achievements": 4.5,
  "cultural_collaboration_fit": 3.9,
  "correctness": 4.1,
  "code_quality_structure": 4.3,
  "resilience_error_handling": 3.7,
  "documentation_explanation": 4.0,
  "creativity_bonus": 3.6,
  "cv_feedback": "Professional feedback on CV match",
  "project_feedback": "Professional feedback on project quality",
  "overall_summary": "Comprehensive candidate summary"
}

Rules:
- Scores: 1.0-5.0 (1=poor, 5=excellent)
- Text fields: Professional and concise
- NO reasoning, explanations, or chain-of-thought
- NO step-by-step analysis or numbered lists`, cvContent, projectContent, jobDesc, studyCase, scoringRubric)
}

// CalculateFinalScores calculates final scores with proper weighting and normalization.
func CalculateFinalScores(rubric ScoringRubric) (cvMatchRate, projectScore float64, err error) {
	// Validate input scores
	if err := ValidateScoringRubric(rubric); err != nil {
		return 0, 0, fmt.Errorf("invalid scoring rubric: %w", err)
	}

	// Calculate weighted scores according to domain model
	cvMatchRate, projectScore = CalculateWeightedScores(rubric)

	// Ensure scores are within valid ranges
	cvMatchRate = math.Max(0.0, math.Min(1.0, cvMatchRate))
	projectScore = math.Max(1.0, math.Min(10.0, projectScore))

	return cvMatchRate, projectScore, nil
}

// GenerateFeedbackPrompt creates a prompt for generating professional feedback.
func GenerateFeedbackPrompt(cvContent, projectContent, jobDesc, studyCase string) string {
	return fmt.Sprintf(`You are a senior recruitment expert and technical reviewer. Generate professional feedback for the candidate evaluation.

CV Content:
%s

Project Content:
%s

Job Description:
%s

Study Case Requirements:
%s

Provide comprehensive feedback covering:

1. CV Feedback:
   - Technical skills alignment with job requirements
   - Experience level assessment
   - Achievement highlights and impact
   - Cultural fit indicators
   - Areas for improvement

2. Project Feedback:
   - Technical implementation quality
   - Code structure and organization
   - Error handling and resilience
   - Documentation quality
   - Creative elements and bonus features

3. Overall Summary (3-5 sentences):
   - Candidate strengths and highlights
   - Key gaps or areas for improvement
   - Hiring recommendation and next steps

CRITICAL: Respond with ONLY valid JSON following this structure:
{
  "cv_feedback": "Professional CV feedback",
  "project_feedback": "Professional project feedback",
  "overall_summary": "Comprehensive candidate summary"
}

Rules:
- Text fields: Professional, comprehensive, and actionable
- NO reasoning, explanations, or chain-of-thought
- NO step-by-step analysis or numbered lists`, cvContent, projectContent, jobDesc, studyCase)
}

// ValidateScoreRange validates that a score is within the specified range.
func ValidateScoreRange(score float64, minVal, maxVal float64, name string) error {
	if score < minVal || score > maxVal {
		return fmt.Errorf("invalid score for %s: %.2f (must be %.1f-%.1f)", name, score, minVal, maxVal)
	}
	return nil
}

// NormalizeScore normalizes a score to the specified range.
func NormalizeScore(score, minVal, maxVal float64) float64 {
	return math.Max(minVal, math.Min(maxVal, score))
}

// CalculateWeightedAverage calculates weighted average of scores.
func CalculateWeightedAverage(scores []float64, weights []float64) (float64, error) {
	if len(scores) != len(weights) {
		return 0, fmt.Errorf("scores and weights length mismatch: %d vs %d", len(scores), len(weights))
	}

	if len(scores) == 0 {
		return 0, fmt.Errorf("no scores provided")
	}

	var weightedSum, totalWeight float64
	for i, score := range scores {
		weight := weights[i]
		weightedSum += score * weight
		totalWeight += weight
	}

	if totalWeight == 0 {
		return 0, fmt.Errorf("total weight is zero")
	}

	return weightedSum / totalWeight, nil
}

// LogScoringDetails logs detailed scoring information for audit purposes.
func LogScoringDetails(jobID string, rubric ScoringRubric, cvMatchRate, projectScore float64) {
	slog.Info("scoring details",
		slog.String("job_id", jobID),
		slog.Float64("technical_skills_match", rubric.TechnicalSkillsMatch),
		slog.Float64("experience_level", rubric.ExperienceLevel),
		slog.Float64("relevant_achievements", rubric.RelevantAchievements),
		slog.Float64("cultural_collaboration_fit", rubric.CulturalCollaborationFit),
		slog.Float64("correctness", rubric.Correctness),
		slog.Float64("code_quality_structure", rubric.CodeQualityStructure),
		slog.Float64("resilience_error_handling", rubric.ResilienceErrorHandling),
		slog.Float64("documentation_explanation", rubric.DocumentationExplanation),
		slog.Float64("creativity_bonus", rubric.CreativityBonus),
		slog.Float64("final_cv_match_rate", cvMatchRate),
		slog.Float64("final_project_score", projectScore))
}
