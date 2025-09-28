package usecase

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"github.com/fairyhunter13/ai-cv-evaluator/internal/domain"
)

// ResultService provides read access to evaluation results and assembles
// the API response envelope including ETag logic and error mapping.
type ResultService struct {
	Jobs    domain.JobRepository
	Results domain.ResultRepository
}

// NewResultService constructs a ResultService with the given repositories.
func NewResultService(j domain.JobRepository, r domain.ResultRepository) ResultService { return ResultService{Jobs: j, Results: r} }

// Fetch returns the HTTP status code, response body, and ETag for the given job id.
// It implements conditional responses (304 Not Modified) based on If-None-Match ETag
// and returns proper shapes for queued/processing/failed states per API rules.
func (s ResultService) Fetch(ctx domain.Context, id, ifNoneMatch string) (int, map[string]any, string, error) {
	job, err := s.Jobs.Get(ctx, id)
	if err != nil {
		if errWrapped(err, domain.ErrNotFound) {
			return http.StatusNotFound, nil, "", fmt.Errorf("%w: job not found", domain.ErrNotFound)
		}
		return http.StatusInternalServerError, nil, "", err
	}
	if job.Status != domain.JobCompleted {
		// Include error object when failed, per rules (03-api-contracts-and-validation.md)
		m := map[string]any{"id": id, "status": string(job.Status)}
		if job.Status == domain.JobFailed {
			code := errorCodeFromJobError(job.Error)
			m["error"] = map[string]any{
				"code":    code,
				"message": job.Error,
			}
		}
		etag := makeETag(m)
		if etag == ifNoneMatch { return http.StatusNotModified, nil, etag, nil }
		return http.StatusOK, m, etag, nil
	}
	res, err := s.Results.GetByJobID(ctx, id)
	if err != nil { return http.StatusInternalServerError, nil, "", err }
	m := map[string]any{
		"id": id, "status": string(domain.JobCompleted),
		"result": map[string]any{
			"cv_match_rate":    res.CVMatchRate,
			"cv_feedback":      res.CVFeedback,
			"project_score":    res.ProjectScore,
			"project_feedback": res.ProjectFeedback,
			"overall_summary":  res.OverallSummary,
		},
	}
	etag := makeETag(m)
	if etag == ifNoneMatch { return http.StatusNotModified, nil, etag, nil }
	return http.StatusOK, m, etag, nil
}

func makeETag(v any) string {
	b, _ := json.Marshal(v)
	s := sha256.Sum256(b)
	return hex.EncodeToString(s[:])
}

func errWrapped(err error, target error) bool { return err != nil && (err == target || (fmt.Errorf("%w", target) != nil && (errorIs(err, target)))) }

func errorIs(err, target error) bool { for e := err; e != nil; { if e == target { return true }; type unwrapper interface{ Unwrap() error }; u, ok := e.(unwrapper); if !ok { break }; e = u.Unwrap() }; return false }

// errorCodeFromJobError maps a stored job error message to a stable error code per rules.
func errorCodeFromJobError(msg string) string {
    s := strings.ToLower(strings.TrimSpace(msg))
    switch {
    case strings.Contains(s, "schema invalid"), strings.Contains(s, "invalid json"), strings.Contains(s, "out of range"), strings.Contains(s, "empty"):
        return "SCHEMA_INVALID"
    case strings.Contains(s, "rate limit"):
        return "UPSTREAM_RATE_LIMIT"
    case strings.Contains(s, "timeout"), strings.Contains(s, "deadline exceeded"):
        return "UPSTREAM_TIMEOUT"
    case strings.Contains(s, "not found"):
        return "NOT_FOUND"
    case strings.Contains(s, "invalid argument"):
        return "INVALID_ARGUMENT"
    default:
        return "INTERNAL"
    }
}
