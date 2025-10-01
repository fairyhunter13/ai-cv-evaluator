// Package httpserver contains HTTP handlers and middleware.
//
// It provides REST API endpoints for the application including
// file upload, evaluation triggering, and result retrieval.
// The package follows clean architecture principles and provides
// a clear separation between HTTP concerns and business logic.
package httpserver

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/fairyhunter13/ai-cv-evaluator/internal/domain"
)

type errorEnvelope struct {
	Error apiError `json:"error"`
}

type apiError struct {
	Code    string      `json:"code"`
	Message string      `json:"message"`
	Details interface{} `json:"details"`
}

func writeJSON(w http.ResponseWriter, status int, v interface{}) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func writeError(w http.ResponseWriter, _ *http.Request, err error, details interface{}) {
	code := http.StatusInternalServerError
	codeStr := "INTERNAL"
	switch {
	case errors.Is(err, domain.ErrInvalidArgument):
		code = http.StatusBadRequest
		codeStr = "INVALID_ARGUMENT"
	case errors.Is(err, domain.ErrNotFound):
		code = http.StatusNotFound
		codeStr = "NOT_FOUND"
	case errors.Is(err, domain.ErrConflict):
		code = http.StatusConflict
		codeStr = "CONFLICT"
	case errors.Is(err, domain.ErrRateLimited):
		code = http.StatusTooManyRequests
		codeStr = "RATE_LIMITED"
	case errors.Is(err, domain.ErrUpstreamTimeout):
		code = http.StatusServiceUnavailable
		codeStr = "UPSTREAM_TIMEOUT"
	case errors.Is(err, domain.ErrUpstreamRateLimit):
		code = http.StatusServiceUnavailable
		codeStr = "UPSTREAM_RATE_LIMIT"
	case errors.Is(err, domain.ErrSchemaInvalid):
		code = http.StatusServiceUnavailable
		codeStr = "SCHEMA_INVALID"
	}
	writeJSON(w, code, errorEnvelope{Error: apiError{Code: codeStr, Message: err.Error(), Details: details}})
}
