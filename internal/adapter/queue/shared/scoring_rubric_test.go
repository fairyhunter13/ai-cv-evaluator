package shared

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestCalculateWeightedScores tests the CalculateWeightedScores function
func TestCalculateWeightedScores(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name            string
		rubric          ScoringRubric
		expectedCV      float64
		expectedProject float64
	}{
		{
			name: "perfect_scores",
			rubric: ScoringRubric{
				TechnicalSkillsMatch:     5.0,
				ExperienceLevel:          5.0,
				RelevantAchievements:     5.0,
				CulturalCollaborationFit: 5.0,
				Correctness:              5.0,
				CodeQualityStructure:     5.0,
				ResilienceErrorHandling:  5.0,
				DocumentationExplanation: 5.0,
				CreativityBonus:          5.0,
			},
			expectedCV:      1.0,  // Perfect CV match
			expectedProject: 10.0, // Perfect project score
		},
		{
			name: "average_scores",
			rubric: ScoringRubric{
				TechnicalSkillsMatch:     3.0,
				ExperienceLevel:          3.0,
				RelevantAchievements:     3.0,
				CulturalCollaborationFit: 3.0,
				Correctness:              3.0,
				CodeQualityStructure:     3.0,
				ResilienceErrorHandling:  3.0,
				DocumentationExplanation: 3.0,
				CreativityBonus:          3.0,
			},
			expectedCV:      0.6, // 3.0 * 0.2 = 0.6
			expectedProject: 6.0, // 3.0 * 2 = 6.0
		},
		{
			name: "low_scores",
			rubric: ScoringRubric{
				TechnicalSkillsMatch:     1.0,
				ExperienceLevel:          1.0,
				RelevantAchievements:     1.0,
				CulturalCollaborationFit: 1.0,
				Correctness:              1.0,
				CodeQualityStructure:     1.0,
				ResilienceErrorHandling:  1.0,
				DocumentationExplanation: 1.0,
				CreativityBonus:          1.0,
			},
			expectedCV:      0.2, // 1.0 * 0.2 = 0.2
			expectedProject: 2.0, // 1.0 * 2 = 2.0
		},
		{
			name: "mixed_scores",
			rubric: ScoringRubric{
				TechnicalSkillsMatch:     4.0,
				ExperienceLevel:          2.0,
				RelevantAchievements:     3.0,
				CulturalCollaborationFit: 5.0,
				Correctness:              4.0,
				CodeQualityStructure:     3.0,
				ResilienceErrorHandling:  2.0,
				DocumentationExplanation: 4.0,
				CreativityBonus:          3.0,
			},
			expectedCV:      0.7, // (4*0.4 + 2*0.25 + 3*0.2 + 5*0.15) * 0.2 = 0.7
			expectedProject: 6.5, // (4*0.3 + 3*0.25 + 2*0.2 + 4*0.15 + 3*0.1) * 2 = 6.5
		},
		{
			name: "zero_scores",
			rubric: ScoringRubric{
				TechnicalSkillsMatch:     0.0,
				ExperienceLevel:          0.0,
				RelevantAchievements:     0.0,
				CulturalCollaborationFit: 0.0,
				Correctness:              0.0,
				CodeQualityStructure:     0.0,
				ResilienceErrorHandling:  0.0,
				DocumentationExplanation: 0.0,
				CreativityBonus:          0.0,
			},
			expectedCV:      0.0,
			expectedProject: 1.0, // Clamped to minimum 1.0
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			cvMatch, projectScore := CalculateWeightedScores(tt.rubric)

			assert.InDelta(t, tt.expectedCV, cvMatch, 0.01, "CV match rate should match expected")
			assert.InDelta(t, tt.expectedProject, projectScore, 0.01, "Project score should match expected")
		})
	}
}

// TestValidateScoringRubric tests the ValidateScoringRubric function
func TestValidateScoringRubric(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		rubric      ScoringRubric
		expectError bool
	}{
		{
			name: "valid_rubric",
			rubric: ScoringRubric{
				TechnicalSkillsMatch:     4.0,
				ExperienceLevel:          3.0,
				RelevantAchievements:     4.0,
				CulturalCollaborationFit: 3.0,
				Correctness:              4.0,
				CodeQualityStructure:     3.0,
				ResilienceErrorHandling:  4.0,
				DocumentationExplanation: 3.0,
				CreativityBonus:          4.0,
			},
			expectError: false,
		},
		{
			name: "scores_out_of_range",
			rubric: ScoringRubric{
				TechnicalSkillsMatch:     6.0, // Out of range
				ExperienceLevel:          3.0,
				RelevantAchievements:     4.0,
				CulturalCollaborationFit: 3.0,
				Correctness:              4.0,
				CodeQualityStructure:     3.0,
				ResilienceErrorHandling:  4.0,
				DocumentationExplanation: 3.0,
				CreativityBonus:          4.0,
			},
			expectError: true,
		},
		{
			name: "negative_scores",
			rubric: ScoringRubric{
				TechnicalSkillsMatch:     -1.0, // Negative
				ExperienceLevel:          3.0,
				RelevantAchievements:     4.0,
				CulturalCollaborationFit: 3.0,
				Correctness:              4.0,
				CodeQualityStructure:     3.0,
				ResilienceErrorHandling:  4.0,
				DocumentationExplanation: 3.0,
				CreativityBonus:          4.0,
			},
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			err := ValidateScoringRubric(tt.rubric)
			if tt.expectError {
				assert.Error(t, err, "Should return error for invalid rubric")
			} else {
				assert.NoError(t, err, "Should not return error for valid rubric")
			}
		})
	}
}

// TestParseDetailedScores tests the ParseDetailedScores function
func TestParseDetailedScores(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name           string
		response       string
		expectError    bool
		expectedScores ScoringRubric
	}{
		{
			name: "valid_json_response",
			response: `{
				"technical_skills_match": 4.0,
				"experience_level": 3.0,
				"relevant_achievements": 4.0,
				"cultural_collaboration_fit": 3.0,
				"correctness": 4.0,
				"code_quality_structure": 3.0,
				"resilience_error_handling": 4.0,
				"documentation_explanation": 3.0,
				"creativity_bonus": 4.0
			}`,
			expectError: false,
			expectedScores: ScoringRubric{
				TechnicalSkillsMatch:     4.0,
				ExperienceLevel:          3.0,
				RelevantAchievements:     4.0,
				CulturalCollaborationFit: 3.0,
				Correctness:              4.0,
				CodeQualityStructure:     3.0,
				ResilienceErrorHandling:  4.0,
				DocumentationExplanation: 3.0,
				CreativityBonus:          4.0,
			},
		},
		{
			name:        "invalid_json",
			response:    "invalid json",
			expectError: true,
		},
		{
			name:        "empty_response",
			response:    "",
			expectError: true,
		},
		{
			name: "partial_scores",
			response: `{
				"technical_skills_match": 4.0,
				"experience_level": 3.0
			}`,
			expectError: true, // Should fail validation because other scores are 0.0 (out of range)
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			scores, err := ParseDetailedScores(tt.response)

			if tt.expectError {
				assert.Error(t, err)
				assert.Equal(t, ScoringRubric{}, scores)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.expectedScores, scores, "Parsed scores should match expected")
			}
		})
	}
}

// TestGenerateScoringPrompt tests the GenerateScoringPrompt function
func TestGenerateScoringPrompt(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name           string
		cvContent      string
		projectContent string
		jobDesc        string
		studyCase      string
		scoringRubric  string
		expectedFields []string
	}{
		{
			name:           "complete_inputs",
			cvContent:      "Software Engineer with 5 years Go and React experience",
			projectContent: "Full-stack web application project",
			jobDesc:        "Looking for Go developer with React experience",
			studyCase:      "Build a web application with Go backend and React frontend",
			scoringRubric:  "Technical skills, experience, achievements",
			expectedFields: []string{"CV Content", "Project Content", "Job Description", "Study Case Requirements", "Scoring Rubric"},
		},
		{
			name:           "minimal_inputs",
			cvContent:      "Engineer",
			projectContent: "Project",
			jobDesc:        "Developer",
			studyCase:      "App",
			scoringRubric:  "Skills",
			expectedFields: []string{"CV Content", "Project Content", "Job Description", "Study Case Requirements", "Scoring Rubric"},
		},
		{
			name:           "empty_inputs",
			cvContent:      "",
			projectContent: "",
			jobDesc:        "",
			studyCase:      "",
			scoringRubric:  "",
			expectedFields: []string{"CV Content", "Project Content", "Job Description", "Study Case Requirements", "Scoring Rubric"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			prompt := GenerateScoringPrompt(tt.cvContent, tt.projectContent, tt.jobDesc, tt.studyCase, tt.scoringRubric)

			assert.NotEmpty(t, prompt, "Generated prompt should not be empty")

			// Check that expected fields are present in the prompt
			for _, field := range tt.expectedFields {
				assert.Contains(t, prompt, field, "Prompt should contain %s", field)
			}
		})
	}
}

// TestCalculateFinalScores tests the CalculateFinalScores function
func TestCalculateFinalScores(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name            string
		rubric          ScoringRubric
		expectedCV      float64
		expectedProject float64
		expectedOverall float64
	}{
		{
			name: "perfect_scores",
			rubric: ScoringRubric{
				TechnicalSkillsMatch:     5.0,
				ExperienceLevel:          5.0,
				RelevantAchievements:     5.0,
				CulturalCollaborationFit: 5.0,
				Correctness:              5.0,
				CodeQualityStructure:     5.0,
				ResilienceErrorHandling:  5.0,
				DocumentationExplanation: 5.0,
				CreativityBonus:          5.0,
			},
			expectedCV:      1.0,
			expectedProject: 10.0,
			expectedOverall: 5.5, // (1.0 + 10.0) / 2 = 5.5
		},
		{
			name: "average_scores",
			rubric: ScoringRubric{
				TechnicalSkillsMatch:     3.0,
				ExperienceLevel:          3.0,
				RelevantAchievements:     3.0,
				CulturalCollaborationFit: 3.0,
				Correctness:              3.0,
				CodeQualityStructure:     3.0,
				ResilienceErrorHandling:  3.0,
				DocumentationExplanation: 3.0,
				CreativityBonus:          3.0,
			},
			expectedCV:      0.6,
			expectedProject: 6.0,
			expectedOverall: 3.3, // (0.6 + 6.0) / 2 = 3.3
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			cvScore, projectScore, err := CalculateFinalScores(tt.rubric)
			require.NoError(t, err, "CalculateFinalScores should not return error")

			// Calculate overall score as average of CV and project scores
			overallScore := (cvScore + projectScore) / 2.0

			assert.InDelta(t, tt.expectedCV, cvScore, 0.01, "CV score should match expected")
			assert.InDelta(t, tt.expectedProject, projectScore, 0.01, "Project score should match expected")
			assert.InDelta(t, tt.expectedOverall, overallScore, 0.01, "Overall score should match expected")
		})
	}
}

// TestGenerateFeedbackPrompt tests the GenerateFeedbackPrompt function
func TestGenerateFeedbackPrompt(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name           string
		cvContent      string
		projectContent string
		jobDesc        string
		studyCase      string
		expectedFields []string
	}{
		{
			name:           "complete_inputs",
			cvContent:      "Software Engineer with 5 years Go and React experience",
			projectContent: "Full-stack web application project",
			jobDesc:        "Looking for Go developer with React experience",
			studyCase:      "Build a web application with Go backend and React frontend",
			expectedFields: []string{"CV Content", "Project Content", "Job Description", "Study Case Requirements"},
		},
		{
			name:           "minimal_inputs",
			cvContent:      "Engineer",
			projectContent: "Project",
			jobDesc:        "Developer",
			studyCase:      "App",
			expectedFields: []string{"CV Content", "Project Content", "Job Description", "Study Case Requirements"},
		},
		{
			name:           "empty_inputs",
			cvContent:      "",
			projectContent: "",
			jobDesc:        "",
			studyCase:      "",
			expectedFields: []string{"CV Content", "Project Content", "Job Description", "Study Case Requirements"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			prompt := GenerateFeedbackPrompt(tt.cvContent, tt.projectContent, tt.jobDesc, tt.studyCase)

			assert.NotEmpty(t, prompt, "Generated feedback prompt should not be empty")

			// Check that expected fields are present in the prompt
			for _, field := range tt.expectedFields {
				assert.Contains(t, prompt, field, "Prompt should contain %s", field)
			}
		})
	}
}

// TestValidateScoreRange tests the ValidateScoreRange function
func TestValidateScoreRange(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name          string
		score         float64
		minVal        float64
		maxVal        float64
		fieldName     string
		expectedValid bool
	}{
		{
			name:          "score_within_range",
			score:         7.5,
			minVal:        0.0,
			maxVal:        10.0,
			fieldName:     "test_score",
			expectedValid: true,
		},
		{
			name:          "score_at_minimum",
			score:         0.0,
			minVal:        0.0,
			maxVal:        10.0,
			fieldName:     "test_score",
			expectedValid: true,
		},
		{
			name:          "score_at_maximum",
			score:         10.0,
			minVal:        0.0,
			maxVal:        10.0,
			fieldName:     "test_score",
			expectedValid: true,
		},
		{
			name:          "score_below_minimum",
			score:         -1.0,
			minVal:        0.0,
			maxVal:        10.0,
			fieldName:     "test_score",
			expectedValid: false,
		},
		{
			name:          "score_above_maximum",
			score:         11.0,
			minVal:        0.0,
			maxVal:        10.0,
			fieldName:     "test_score",
			expectedValid: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			err := ValidateScoreRange(tt.score, tt.minVal, tt.maxVal, tt.fieldName)
			if tt.expectedValid {
				assert.NoError(t, err, "Score range validation should pass")
			} else {
				assert.Error(t, err, "Score range validation should fail")
			}
		})
	}
}

// TestNormalizeScore tests the NormalizeScore function
func TestNormalizeScore(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name          string
		score         float64
		minVal        float64
		maxVal        float64
		expectedScore float64
	}{
		{
			name:          "normalize_positive_score",
			score:         7.5,
			minVal:        0.0,
			maxVal:        10.0,
			expectedScore: 7.5, // NormalizeScore clamps to range, doesn't normalize to 0-1
		},
		{
			name:          "normalize_maximum_score",
			score:         10.0,
			minVal:        0.0,
			maxVal:        10.0,
			expectedScore: 10.0,
		},
		{
			name:          "normalize_minimum_score",
			score:         0.0,
			minVal:        0.0,
			maxVal:        10.0,
			expectedScore: 0.0,
		},
		{
			name:          "normalize_mid_range",
			score:         5.0,
			minVal:        0.0,
			maxVal:        10.0,
			expectedScore: 5.0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			normalized := NormalizeScore(tt.score, tt.minVal, tt.maxVal)
			assert.InDelta(t, tt.expectedScore, normalized, 0.01, "Normalized score should match expected")
		})
	}
}

// TestCalculateWeightedAverage tests the CalculateWeightedAverage function
func TestCalculateWeightedAverage(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		values      []float64
		weights     []float64
		expectError bool
		expectedAvg float64
	}{
		{
			name:        "valid_values_and_weights",
			values:      []float64{8.0, 9.0, 7.0},
			weights:     []float64{0.4, 0.4, 0.2},
			expectError: false,
			expectedAvg: 8.2, // 8.0*0.4 + 9.0*0.4 + 7.0*0.2 = 8.2
		},
		{
			name:        "mismatched_lengths",
			values:      []float64{8.0, 9.0},
			weights:     []float64{0.5, 0.3, 0.2},
			expectError: true,
		},
		{
			name:        "empty_inputs",
			values:      []float64{},
			weights:     []float64{},
			expectError: true,
		},
		{
			name:        "weights_sum_not_one",
			values:      []float64{8.0, 9.0},
			weights:     []float64{0.5, 0.3},
			expectError: false,
			expectedAvg: 8.375, // (8.0*0.5 + 9.0*0.3) / (0.5 + 0.3) = 8.375
		},
		{
			name:        "negative_weights",
			values:      []float64{8.0, 9.0},
			weights:     []float64{-0.5, 0.5},
			expectError: true,
		},
		{
			name:        "single_value",
			values:      []float64{8.0},
			weights:     []float64{1.0},
			expectError: false,
			expectedAvg: 8.0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			avg, err := CalculateWeightedAverage(tt.values, tt.weights)

			if tt.expectError {
				assert.Error(t, err)
				assert.Equal(t, 0.0, avg)
			} else {
				assert.NoError(t, err)
				assert.InDelta(t, tt.expectedAvg, avg, 0.01, "Weighted average should match expected")
			}
		})
	}
}

// TestLogScoringDetails tests the LogScoringDetails function
func TestLogScoringDetails(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name         string
		jobID        string
		rubric       ScoringRubric
		cvMatchRate  float64
		projectScore float64
	}{
		{
			name:  "complete_scoring_details",
			jobID: "job-123",
			rubric: ScoringRubric{
				TechnicalSkillsMatch:     4.0,
				ExperienceLevel:          3.0,
				RelevantAchievements:     4.0,
				CulturalCollaborationFit: 3.0,
				Correctness:              4.0,
				CodeQualityStructure:     3.0,
				ResilienceErrorHandling:  4.0,
				DocumentationExplanation: 3.0,
				CreativityBonus:          4.0,
			},
			cvMatchRate:  0.7,
			projectScore: 7.5,
		},
		{
			name:  "minimal_scoring_details",
			jobID: "job-456",
			rubric: ScoringRubric{
				TechnicalSkillsMatch: 4.0,
			},
			cvMatchRate:  0.8,
			projectScore: 8.0,
		},
		{
			name:         "empty_rubric",
			jobID:        "job-789",
			rubric:       ScoringRubric{},
			cvMatchRate:  0.0,
			projectScore: 0.0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			// This function logs details, so we just test that it doesn't panic
			assert.NotPanics(t, func() {
				LogScoringDetails(tt.jobID, tt.rubric, tt.cvMatchRate, tt.projectScore)
			}, "LogScoringDetails should not panic")
		})
	}
}
