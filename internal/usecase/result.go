package usecase

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/fairyhunter13/ai-cv-evaluator/internal/domain"
)

// ResultService provides read access to evaluation results and assembles
// the API response envelope including ETag logic and error mapping.
type ResultService struct {
	Jobs    domain.JobRepository
	Results domain.ResultRepository
}

// NewResultService constructs a ResultService with the given repositories.
func NewResultService(j domain.JobRepository, r domain.ResultRepository) ResultService {
	return ResultService{Jobs: j, Results: r}
}

// Fetch returns the HTTP status code, response body, and ETag for the given job id.
// It implements conditional responses (304 Not Modified) based on If-None-Match ETag
// and returns proper shapes for queued/processing/failed states per API rules.
func (s ResultService) Fetch(ctx domain.Context, id, ifNoneMatch string) (int, map[string]any, string, error) {
	slog.Info("fetching result", slog.String("job_id", id))
	job, err := s.Jobs.Get(ctx, id)
	if err != nil {
		slog.Error("failed to get job", slog.String("job_id", id), slog.Any("error", err))
		if errWrapped(err, domain.ErrNotFound) {
			return http.StatusNotFound, nil, "", fmt.Errorf("%w: job not found", domain.ErrNotFound)
		}
		return http.StatusInternalServerError, nil, "", err
	}
	slog.Info("job retrieved", slog.String("job_id", id), slog.String("status", string(job.Status)), slog.Time("created_at", job.CreatedAt), slog.Time("updated_at", job.UpdatedAt))
	if job.Status != domain.JobCompleted {
		slog.Info("job not completed", slog.String("job_id", id), slog.String("status", string(job.Status)))
		// Stale timeout policy: mark queued/processing older than 2 minutes as failed
		// Increased from 30s to 2 minutes to allow for real AI processing time
		now := time.Now().UTC()
		stale := false
		if job.Status == domain.JobQueued && now.Sub(job.CreatedAt) > 2*time.Minute {
			stale = true
		}
		if job.Status == domain.JobProcessing && now.Sub(job.UpdatedAt) > 2*time.Minute {
			stale = true
		}
		if stale {
			slog.Warn("job marked as stale", slog.String("job_id", id), slog.String("status", string(job.Status)), slog.Duration("age", now.Sub(job.CreatedAt)))
			msg := "timeout: job exceeded 2 minutes"
			_ = s.Jobs.UpdateStatus(ctx, id, domain.JobFailed, &msg)
			job.Status = domain.JobFailed
			job.Error = msg
		}
		// Include error object when failed, per rules (03-api-contracts-and-validation.md)
		m := map[string]any{"id": id, "status": string(job.Status)}
		if job.Status == domain.JobFailed {
			code := errorCodeFromJobError(job.Error)
			m["error"] = map[string]any{
				"code":    code,
				"message": job.Error,
			}
		}
		slog.Info("returning non-completed status", slog.String("job_id", id), slog.String("status", string(job.Status)), slog.Any("response", m))
		etag := makeETag(m)
		if etag == ifNoneMatch {
			return http.StatusNotModified, nil, etag, nil
		}
		return http.StatusOK, m, etag, nil
	}
	res, err := s.Results.GetByJobID(ctx, id)
	if err != nil {
		return http.StatusInternalServerError, nil, "", err
	}
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
	if etag == ifNoneMatch {
		return http.StatusNotModified, nil, etag, nil
	}
	return http.StatusOK, m, etag, nil
}

func makeETag(v any) string {
	b, _ := json.Marshal(v)
	s := sha256.Sum256(b)
	return hex.EncodeToString(s[:])
}

func errWrapped(err error, target error) bool {
	return err != nil && (err == target || (fmt.Errorf("%w", target) != nil && (errorIs(err, target))))
}

func errorIs(err, target error) bool {
	for e := err; e != nil; {
		if e == target {
			return true
		}
		type unwrapper interface{ Unwrap() error }
		u, ok := e.(unwrapper)
		if !ok {
			break
		}
		e = u.Unwrap()
	}
	return false
}

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
