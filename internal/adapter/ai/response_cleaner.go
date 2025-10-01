// Package ai provides response cleaning utilities for handling malformed LLM responses.
package ai

import (
	"encoding/json"
	"regexp"
	"strings"
)

// ResponseCleaner handles cleaning and sanitizing LLM responses.
type ResponseCleaner struct{}

// NewResponseCleaner creates a new response cleaner.
func NewResponseCleaner() *ResponseCleaner {
	return &ResponseCleaner{}
}

// CleanJSONResponse cleans and sanitizes a JSON response from LLM models.
func (rc *ResponseCleaner) CleanJSONResponse(response string) (string, error) {
	// Step 1: Remove markdown code blocks
	response = rc.removeMarkdownBlocks(response)

	// Step 2: Fix common JSON formatting issues
	response = rc.fixJSONFormatting(response)

	// Step 3: Extract JSON from mixed content
	response = rc.extractJSON(response)

	// Step 4: Validate and fix JSON structure
	response = rc.validateAndFixJSON(response)

	return response, nil
}

// removeMarkdownBlocks removes markdown code blocks from the response.
func (rc *ResponseCleaner) removeMarkdownBlocks(response string) string {
	// Remove ```json and ``` markers
	response = strings.TrimPrefix(response, "```json")
	response = strings.TrimPrefix(response, "```")
	response = strings.TrimSuffix(response, "```")
	response = strings.TrimSpace(response)
	return response
}

// fixJSONFormatting fixes common JSON formatting issues.
func (rc *ResponseCleaner) fixJSONFormatting(response string) string {
	// Replace backticks with proper quotes
	response = strings.ReplaceAll(response, "`", "\"")

	// Fix common quote issues
	response = strings.ReplaceAll(response, "'", "\"")

	// Remove any remaining markdown artifacts
	response = regexp.MustCompile(`\*\*([^*]+)\*\*`).ReplaceAllString(response, `"$1"`)
	response = regexp.MustCompile(`\*([^*]+)\*`).ReplaceAllString(response, `"$1"`)

	return response
}

// extractJSON extracts JSON from mixed content.
func (rc *ResponseCleaner) extractJSON(response string) string {
	// Find the first { and last } to extract JSON object
	start := strings.Index(response, "{")
	if start == -1 {
		return response
	}

	// Find the matching closing brace
	braceCount := 0
	end := start
	for i := start; i < len(response); i++ {
		if response[i] == '{' {
			braceCount++
		} else if response[i] == '}' {
			braceCount--
			if braceCount == 0 {
				end = i
				break
			}
		}
	}

	if end > start {
		return response[start : end+1]
	}

	return response
}

// validateAndFixJSON validates and fixes JSON structure.
func (rc *ResponseCleaner) validateAndFixJSON(response string) string {
	// Try to parse as JSON first
	var temp interface{}
	if err := json.Unmarshal([]byte(response), &temp); err == nil {
		return response // Valid JSON, return as-is
	}

	// If parsing fails, try to fix common issues
	response = rc.fixCommonJSONIssues(response)

	return response
}

// fixCommonJSONIssues fixes common JSON parsing issues.
func (rc *ResponseCleaner) fixCommonJSONIssues(response string) string {
	// Fix trailing commas
	response = regexp.MustCompile(`,(\s*[}\]])`).ReplaceAllString(response, "$1")

	// Fix missing quotes around keys
	response = regexp.MustCompile(`(\w+):`).ReplaceAllString(response, `"$1":`)

	// Fix single quotes to double quotes
	response = strings.ReplaceAll(response, "'", "\"")

	// Remove any non-JSON content before the first {
	start := strings.Index(response, "{")
	if start > 0 {
		response = response[start:]
	}

	return response
}

// IsValidJSON checks if a string is valid JSON.
func (rc *ResponseCleaner) IsValidJSON(response string) bool {
	var temp interface{}
	return json.Unmarshal([]byte(response), &temp) == nil
}

// CleanAndValidateJSON cleans and validates a JSON response.
func (rc *ResponseCleaner) CleanAndValidateJSON(response string) (string, error) {
	cleaned, err := rc.CleanJSONResponse(response)
	if err != nil {
		return "", err
	}

	if !rc.IsValidJSON(cleaned) {
		return "", &JSONValidationError{
			Original: response,
			Cleaned:  cleaned,
			Message:  "cleaned response is still not valid JSON",
		}
	}

	return cleaned, nil
}

// JSONValidationError represents a JSON validation error.
type JSONValidationError struct {
	Original string
	Cleaned  string
	Message  string
}

func (e *JSONValidationError) Error() string {
	return e.Message
}
