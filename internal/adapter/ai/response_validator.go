// Package ai provides comprehensive response validation and processing.
package ai

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/fairyhunter13/ai-cv-evaluator/internal/domain"
)

// ResponseValidator provides comprehensive response validation and processing.
type ResponseValidator struct {
	refusalDetector *RefusalDetector
	responseCleaner *ResponseCleaner
	ai              domain.AIClient
}

// NewResponseValidator creates a new response validator.
func NewResponseValidator(ai domain.AIClient) *ResponseValidator {
	return &ResponseValidator{
		refusalDetector: NewRefusalDetector(ai),
		responseCleaner: NewResponseCleaner(),
		ai:              ai,
	}
}

// ValidationResult represents the result of comprehensive response validation.
type ValidationResult struct {
	IsValid         bool              `json:"is_valid"`
	IsRefusal       bool              `json:"is_refusal"`
	RefusalAnalysis *RefusalAnalysis  `json:"refusal_analysis,omitempty"`
	CleanedResponse string            `json:"cleaned_response"`
	Issues          []ValidationIssue `json:"issues,omitempty"`
	Suggestions     []string          `json:"suggestions,omitempty"`
	ProcessingTime  time.Duration     `json:"processing_time"`
}

// ValidationIssue represents a specific validation issue.
type ValidationIssue struct {
	Type        string `json:"type"`
	Severity    string `json:"severity"` // "low", "medium", "high", "critical"
	Description string `json:"description"`
	Solution    string `json:"solution,omitempty"`
}

// ValidateResponse performs comprehensive response validation.
func (rv *ResponseValidator) ValidateResponse(ctx context.Context, response string) (*ValidationResult, error) {
	startTime := time.Now()

	slog.Debug("starting comprehensive response validation",
		slog.Int("response_length", len(response)))

	result := &ValidationResult{
		IsValid:         true,
		IsRefusal:       false,
		CleanedResponse: response,
		Issues:          []ValidationIssue{},
		Suggestions:     []string{},
	}

	// Step 1: Basic response checks
	if err := rv.performBasicChecks(response, result); err != nil {
		return nil, fmt.Errorf("basic validation failed: %w", err)
	}

	// Step 2: Refusal detection (both AI and code-based)
	if err := rv.performRefusalDetection(ctx, response, result); err != nil {
		slog.Warn("refusal detection failed, continuing with other validations", slog.Any("error", err))
	}

	// Step 3: Response cleaning and formatting
	if err := rv.performResponseCleaning(response, result); err != nil {
		slog.Warn("response cleaning failed", slog.Any("error", err))
		result.Issues = append(result.Issues, ValidationIssue{
			Type:        "cleaning_failed",
			Severity:    "medium",
			Description: "Failed to clean response formatting",
			Solution:    "Manual review required",
		})
	}

	// Step 4: JSON validation (if expected)
	if err := rv.performJSONValidation(result.CleanedResponse, result); err != nil {
		slog.Warn("JSON validation failed", slog.Any("error", err))
	}

	// Step 5: Content quality assessment
	rv.performContentQualityAssessment(result.CleanedResponse, result)

	// Calculate processing time
	result.ProcessingTime = time.Since(startTime)

	// Determine overall validity
	rv.determineOverallValidity(result)

	slog.Debug("response validation completed",
		slog.Bool("is_valid", result.IsValid),
		slog.Bool("is_refusal", result.IsRefusal),
		slog.Int("issues_count", len(result.Issues)),
		slog.Duration("processing_time", result.ProcessingTime))

	return result, nil
}

// performBasicChecks performs basic response validation checks.
func (rv *ResponseValidator) performBasicChecks(response string, result *ValidationResult) error {
	// Check for empty response
	if strings.TrimSpace(response) == "" {
		result.Issues = append(result.Issues, ValidationIssue{
			Type:        "empty_response",
			Severity:    "critical",
			Description: "Response is empty or contains only whitespace",
			Solution:    "Retry with a different model or prompt",
		})
		result.IsValid = false
		return nil
	}

	// Check for extremely short responses (likely refusals)
	if len(strings.TrimSpace(response)) < 20 {
		result.Issues = append(result.Issues, ValidationIssue{
			Type:        "short_response",
			Severity:    "high",
			Description: "Response is extremely short, likely a refusal",
			Solution:    "Try rephrasing the request or using a different model",
		})
	}

	// Check for extremely long responses (potential issues)
	if len(response) > 10000 {
		result.Issues = append(result.Issues, ValidationIssue{
			Type:        "long_response",
			Severity:    "medium",
			Description: "Response is extremely long, may contain unwanted content",
			Solution:    "Review response content for relevance",
		})
	}

	return nil
}

// performRefusalDetection performs both AI and code-based refusal detection.
func (rv *ResponseValidator) performRefusalDetection(ctx context.Context, response string, result *ValidationResult) error {
	// Try AI-powered detection first
	analysis, err := rv.refusalDetector.DetectRefusalWithFallback(ctx, response)
	if err != nil {
		slog.Warn("refusal detection failed", slog.Any("error", err))
		return err
	}

	if analysis.IsRefusal {
		result.IsRefusal = true
		result.RefusalAnalysis = analysis

		// Add refusal as a critical issue
		result.Issues = append(result.Issues, ValidationIssue{
			Type:        "refusal_detected",
			Severity:    "critical",
			Description: fmt.Sprintf("AI model refused to process request: %s", analysis.Reason),
			Solution:    "Try a different model or rephrase the request",
		})

		// Add suggestions based on refusal type
		if analysis.RefusalType != "" {
			suggestions := rv.refusalDetector.GetRefusalHandlingSuggestions(analysis.RefusalType)
			result.Suggestions = append(result.Suggestions, suggestions...)
		}
	}

	return nil
}

// performResponseCleaning performs response cleaning and formatting.
func (rv *ResponseValidator) performResponseCleaning(originalResponse string, result *ValidationResult) error {
	// Clean the response
	cleaned, err := rv.responseCleaner.CleanAndValidateJSON(originalResponse)
	if err != nil {
		// If JSON cleaning fails, try basic cleaning
		cleaned, err = rv.responseCleaner.CleanJSONResponse(originalResponse)
		if err != nil {
			return fmt.Errorf("response cleaning failed: %w", err)
		}
	}

	result.CleanedResponse = cleaned
	return nil
}

// performJSONValidation validates JSON structure and content.
func (rv *ResponseValidator) performJSONValidation(response string, result *ValidationResult) error {
	// Check if response is valid JSON
	if !rv.responseCleaner.IsValidJSON(response) {
		result.Issues = append(result.Issues, ValidationIssue{
			Type:        "invalid_json",
			Severity:    "high",
			Description: "Response is not valid JSON",
			Solution:    "Use response cleaning or try a different model",
		})
		return fmt.Errorf("response is not valid JSON")
	}

	// Try to parse as generic JSON to check structure
	var jsonData interface{}
	if err := json.Unmarshal([]byte(response), &jsonData); err != nil {
		result.Issues = append(result.Issues, ValidationIssue{
			Type:        "json_parse_error",
			Severity:    "high",
			Description: "Response cannot be parsed as JSON",
			Solution:    "Check response format and try cleaning",
		})
		return fmt.Errorf("JSON parsing failed: %w", err)
	}

	return nil
}

// performContentQualityAssessment assesses the quality of the response content.
func (rv *ResponseValidator) performContentQualityAssessment(response string, result *ValidationResult) {
	// Check for common quality issues
	issues := []ValidationIssue{}

	// Check for repetitive content
	if rv.hasRepetitiveContent(response) {
		issues = append(issues, ValidationIssue{
			Type:        "repetitive_content",
			Severity:    "medium",
			Description: "Response contains repetitive content",
			Solution:    "Try a different model or adjust prompt",
		})
	}

	// Check for incomplete responses
	if rv.hasIncompleteContent(response) {
		issues = append(issues, ValidationIssue{
			Type:        "incomplete_content",
			Severity:    "medium",
			Description: "Response appears incomplete",
			Solution:    "Try increasing max_tokens or using a different model",
		})
	}

	// Check for off-topic content
	if rv.hasOffTopicContent(response) {
		issues = append(issues, ValidationIssue{
			Type:        "off_topic_content",
			Severity:    "high",
			Description: "Response contains off-topic content",
			Solution:    "Review and adjust the prompt for clarity",
		})
	}

	result.Issues = append(result.Issues, issues...)
}

// hasRepetitiveContent checks if the response contains repetitive content.
func (rv *ResponseValidator) hasRepetitiveContent(response string) bool {
	words := strings.Fields(strings.ToLower(response))
	if len(words) < 10 {
		return false
	}

	// Check for repeated phrases
	phraseCount := make(map[string]int)
	for i := 0; i < len(words)-2; i++ {
		phrase := strings.Join(words[i:i+3], " ")
		phraseCount[phrase]++
		if phraseCount[phrase] > 2 {
			return true
		}
	}

	return false
}

// hasIncompleteContent checks if the response appears incomplete.
func (rv *ResponseValidator) hasIncompleteContent(response string) bool {
	// Check for common incomplete indicators
	incompleteIndicators := []string{
		"...", "etc.", "and so on", "continue", "more",
		"truncated", "cut off", "incomplete",
	}

	lowerResponse := strings.ToLower(response)
	for _, indicator := range incompleteIndicators {
		if strings.Contains(lowerResponse, indicator) {
			return true
		}
	}

	// Check if response ends abruptly
	trimmed := strings.TrimSpace(response)
	if len(trimmed) > 0 && !strings.HasSuffix(trimmed, ".") && !strings.HasSuffix(trimmed, "}") && !strings.HasSuffix(trimmed, "]") {
		return true
	}

	return false
}

// hasOffTopicContent checks if the response contains off-topic content.
func (rv *ResponseValidator) hasOffTopicContent(response string) bool {
	// This is a simplified check - in practice, you might want more sophisticated content analysis
	offTopicIndicators := []string{
		"unrelated", "off-topic", "different subject",
		"not relevant", "wrong topic", "misunderstood",
	}

	lowerResponse := strings.ToLower(response)
	for _, indicator := range offTopicIndicators {
		if strings.Contains(lowerResponse, indicator) {
			return true
		}
	}

	return false
}

// determineOverallValidity determines the overall validity of the response.
func (rv *ResponseValidator) determineOverallValidity(result *ValidationResult) {
	// Check for critical issues
	for _, issue := range result.Issues {
		if issue.Severity == "critical" {
			result.IsValid = false
			return
		}
	}

	// If it's a refusal, mark as invalid
	if result.IsRefusal {
		result.IsValid = false
		return
	}

	// Check for too many high-severity issues
	highSeverityCount := 0
	for _, issue := range result.Issues {
		if issue.Severity == "high" {
			highSeverityCount++
		}
	}

	if highSeverityCount > 2 {
		result.IsValid = false
		return
	}

	// If we get here, the response is valid
	result.IsValid = true
}
