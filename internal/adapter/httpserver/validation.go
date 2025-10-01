package httpserver

import (
	"regexp"
	"strconv"
	"strings"
	"unicode/utf8"
)

// ValidationError represents a validation error
type ValidationError struct {
	Field   string `json:"field"`
	Code    string `json:"code"`
	Message string `json:"message"`
}

// ValidationResult represents the result of validation
type ValidationResult struct {
	Valid  bool              `json:"valid"`
	Errors []ValidationError `json:"errors,omitempty"`
}

// ValidateJobID validates a job ID
func ValidateJobID(jobID string) ValidationResult {
	if jobID == "" {
		return ValidationResult{
			Valid: false,
			Errors: []ValidationError{
				{
					Field:   "id",
					Code:    "REQUIRED",
					Message: "Job ID is required",
				},
			},
		}
	}

	// Check length
	if len(jobID) > 100 {
		return ValidationResult{
			Valid: false,
			Errors: []ValidationError{
				{
					Field:   "id",
					Code:    "TOO_LONG",
					Message: "Job ID is too long (max 100 characters)",
				},
			},
		}
	}

	// Check for valid characters (alphanumeric, hyphens, underscores)
	validJobID := regexp.MustCompile(`^[a-zA-Z0-9_-]+$`)
	if !validJobID.MatchString(jobID) {
		return ValidationResult{
			Valid: false,
			Errors: []ValidationError{
				{
					Field:   "id",
					Code:    "INVALID_FORMAT",
					Message: "Job ID contains invalid characters",
				},
			},
		}
	}

	return ValidationResult{Valid: true}
}

// ValidatePagination validates pagination parameters
func ValidatePagination(page, limit string) ValidationResult {
	var errors []ValidationError

	// Validate page
	if page != "" {
		pageNum, err := strconv.Atoi(page)
		if err != nil || pageNum < 1 {
			errors = append(errors, ValidationError{
				Field:   "page",
				Code:    "INVALID_FORMAT",
				Message: "Page must be a positive integer",
			})
		}
	}

	// Validate limit
	if limit != "" {
		limitNum, err := strconv.Atoi(limit)
		if err != nil || limitNum < 1 || limitNum > 100 {
			errors = append(errors, ValidationError{
				Field:   "limit",
				Code:    "INVALID_FORMAT",
				Message: "Limit must be between 1 and 100",
			})
		}
	}

	if len(errors) > 0 {
		return ValidationResult{
			Valid:  false,
			Errors: errors,
		}
	}

	return ValidationResult{Valid: true}
}

// ValidateSearchQuery validates a search query
func ValidateSearchQuery(query string) ValidationResult {
	if query == "" {
		return ValidationResult{Valid: true}
	}

	// Check length
	if len(query) > 200 {
		return ValidationResult{
			Valid: false,
			Errors: []ValidationError{
				{
					Field:   "search",
					Code:    "TOO_LONG",
					Message: "Search query is too long (max 200 characters)",
				},
			},
		}
	}

	// Check for valid characters (no special characters that could be used for injection)
	validQuery := regexp.MustCompile(`^[a-zA-Z0-9\s_-]+$`)
	if !validQuery.MatchString(query) {
		return ValidationResult{
			Valid: false,
			Errors: []ValidationError{
				{
					Field:   "search",
					Code:    "INVALID_FORMAT",
					Message: "Search query contains invalid characters",
				},
			},
		}
	}

	return ValidationResult{Valid: true}
}

// ValidateStatus validates a job status filter
func ValidateStatus(status string) ValidationResult {
	if status == "" {
		return ValidationResult{Valid: true}
	}

	validStatuses := []string{"queued", "processing", "completed", "failed"}
	for _, validStatus := range validStatuses {
		if status == validStatus {
			return ValidationResult{Valid: true}
		}
	}

	return ValidationResult{
		Valid: false,
		Errors: []ValidationError{
			{
				Field:   "status",
				Code:    "INVALID_VALUE",
				Message: "Status must be one of: queued, processing, completed, failed",
			},
		},
	}
}

// SanitizeString sanitizes a string input
func SanitizeString(input string) string {
	// Remove null bytes and control characters
	input = strings.ReplaceAll(input, "\x00", "")

	// Trim whitespace
	input = strings.TrimSpace(input)

	// Limit length to prevent DoS
	if len(input) > 1000 {
		input = input[:1000]
	}

	// Ensure valid UTF-8
	if !utf8.ValidString(input) {
		input = strings.ToValidUTF8(input, "")
	}

	return input
}

// SanitizeJobID sanitizes a job ID
func SanitizeJobID(jobID string) string {
	// Remove any potentially dangerous characters
	jobID = regexp.MustCompile(`[^a-zA-Z0-9_-]`).ReplaceAllString(jobID, "")

	// Limit length
	if len(jobID) > 100 {
		jobID = jobID[:100]
	}

	return jobID
}
