// Package ai provides AI-powered refusal response detection.
package ai

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"

	"github.com/fairyhunter13/ai-cv-evaluator/internal/domain"
)

// RefusalDetector uses AI to intelligently detect refusal responses.
type RefusalDetector struct {
	ai domain.AIClient
}

// NewRefusalDetector creates a new AI-powered refusal detector.
func NewRefusalDetector(ai domain.AIClient) *RefusalDetector {
	return &RefusalDetector{ai: ai}
}

// RefusalAnalysis represents the result of AI-powered refusal detection.
type RefusalAnalysis struct {
	IsRefusal   bool     `json:"is_refusal"`
	Confidence  float64  `json:"confidence"`
	RefusalType string   `json:"refusal_type,omitempty"`
	Reason      string   `json:"reason,omitempty"`
	Suggestions []string `json:"suggestions,omitempty"`
}

// DetectRefusal uses AI to analyze if a response is a refusal.
func (rd *RefusalDetector) DetectRefusal(ctx context.Context, response string) (*RefusalAnalysis, error) {
	slog.Debug("analyzing response for refusal patterns",
		slog.Int("response_length", len(response)))

	// Create a specialized prompt for refusal detection
	analysisPrompt := `You are an expert AI response analyzer. Analyze the following AI model response to determine if it's a refusal to process a request.

REFUSAL INDICATORS TO LOOK FOR:
1. **Direct Refusals**: "I cannot", "I can't", "I'm unable", "I refuse"
2. **Apologetic Refusals**: "I'm sorry, but...", "Unfortunately, I cannot..."
3. **Policy-Based Refusals**: References to guidelines, policies, safety concerns
4. **Capability Limitations**: "I don't have access", "I lack the ability"
5. **Ethical Concerns**: Mentions of harmful content, inappropriate requests
6. **Security Concerns**: References to system instructions, internal access
7. **Technical Limitations**: "Processing error", "Unable to process"

RESPONSE TO ANALYZE:
` + response + `

Analyze this response and determine:
1. Is this a refusal to process the request? (true/false)
2. What is your confidence level? (0.0-1.0)
3. What type of refusal is it? (if applicable)
4. Why do you think it's a refusal? (brief explanation)
5. What suggestions do you have for handling this? (if applicable)

Respond with ONLY valid JSON in this exact format:
{
  "is_refusal": true/false,
  "confidence": 0.0-1.0,
  "refusal_type": "type_of_refusal_or_empty_string",
  "reason": "brief_explanation_or_empty_string",
  "suggestions": ["suggestion1", "suggestion2"]
}`

	// Use a different model for analysis to avoid bias
	analysisResponse, err := rd.ai.ChatJSON(ctx, "", analysisPrompt, 500)
	if err != nil {
		slog.Error("AI refusal detection failed", slog.Any("error", err))
		return nil, fmt.Errorf("AI refusal detection failed: %w", err)
	}

	// Parse the analysis response
	var analysis RefusalAnalysis
	if err := json.Unmarshal([]byte(analysisResponse), &analysis); err != nil {
		slog.Error("failed to parse refusal analysis",
			slog.String("response", analysisResponse),
			slog.Any("error", err))
		return nil, fmt.Errorf("failed to parse refusal analysis: %w", err)
	}

	slog.Debug("refusal analysis completed",
		slog.Bool("is_refusal", analysis.IsRefusal),
		slog.Float64("confidence", analysis.Confidence),
		slog.String("refusal_type", analysis.RefusalType))

	return &analysis, nil
}

// DetectRefusalWithFallback performs refusal detection with fallback to code-based detection.
func (rd *RefusalDetector) DetectRefusalWithFallback(ctx context.Context, response string) (*RefusalAnalysis, error) {
	// Try AI-powered detection first
	analysis, err := rd.DetectRefusal(ctx, response)
	if err != nil {
		slog.Warn("AI refusal detection failed, falling back to code-based detection",
			slog.Any("error", err))

		// Fallback to code-based detection
		codeBasedResult := isRefusalResponseCodeBased(response)
		return &RefusalAnalysis{
			IsRefusal:   codeBasedResult,
			Confidence:  0.7, // Lower confidence for code-based detection
			RefusalType: "code_detected",
			Reason:      "Detected using code-based pattern matching",
		}, nil
	}

	return analysis, nil
}

// isRefusalResponseCodeBased provides a fallback code-based detection method.
func isRefusalResponseCodeBased(response string) bool {
	// Simple but effective code-based detection
	refusalIndicators := []string{
		"i'm sorry", "i cannot", "i can't", "i'm unable", "i apologize",
		"unfortunately", "i'm afraid", "i don't have access",
		"legitimate query", "internal system", "security concerns",
		"policy", "guidelines", "ethical", "harmful",
	}

	lowerResponse := strings.ToLower(response)
	for _, indicator := range refusalIndicators {
		if strings.Contains(lowerResponse, indicator) {
			return true
		}
	}

	return false
}

// GetRefusalHandlingSuggestions provides suggestions for handling different types of refusals.
func (rd *RefusalDetector) GetRefusalHandlingSuggestions(refusalType string) []string {
	suggestions := map[string][]string{
		"security_concerns": {
			"Try rephrasing the request to be more specific",
			"Remove any system instruction references",
			"Use a different model that's less restrictive",
			"Break down the request into smaller parts",
		},
		"policy_violation": {
			"Review and adjust the prompt content",
			"Use more neutral language",
			"Try a different model with different policies",
			"Simplify the request structure",
		},
		"capability_limitation": {
			"Try a more capable model",
			"Simplify the request requirements",
			"Provide more context in the prompt",
			"Use a different approach to the task",
		},
		"technical_limitation": {
			"Check if the input is too short or too long",
			"Try a different model",
			"Simplify the request",
			"Add more context to help the model understand",
		},
		"ethical_concerns": {
			"Review the prompt for potentially problematic content",
			"Use more neutral, professional language",
			"Try a different model with different ethical guidelines",
			"Reframe the request in a more appropriate way",
		},
	}

	if suggestions, exists := suggestions[refusalType]; exists {
		return suggestions
	}

	// Default suggestions for unknown refusal types
	return []string{
		"Try rephrasing the request",
		"Use a different model",
		"Simplify the prompt",
		"Add more context to help the model understand",
	}
}
