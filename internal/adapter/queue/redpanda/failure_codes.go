package redpanda

import "strings"

// classifyFailureCode maps a job error message to a stable error code for metrics.
// This intentionally mirrors the logic in usecase.errorCodeFromJobError so that
// Prometheus labels align with API error codes.
func classifyFailureCode(msg string) string {
	// Defensive guard against empty messages.
	s := strings.ToLower(strings.TrimSpace(msg))
	if s == "" {
		return "INTERNAL"
	}

	switch {
	case strings.Contains(s, "schema invalid"),
		strings.Contains(s, "invalid json"),
		strings.Contains(s, "out of range"),
		strings.Contains(s, "empty"):
		return "SCHEMA_INVALID"
	case strings.Contains(s, "rate limit"):
		return "UPSTREAM_RATE_LIMIT"
	case strings.Contains(s, "timeout"),
		strings.Contains(s, "deadline exceeded"):
		return "UPSTREAM_TIMEOUT"
	case strings.Contains(s, "not found"):
		return "NOT_FOUND"
	case strings.Contains(s, "invalid argument"),
		strings.Contains(s, "ids required"):
		return "INVALID_ARGUMENT"
	default:
		return "INTERNAL"
	}
}
