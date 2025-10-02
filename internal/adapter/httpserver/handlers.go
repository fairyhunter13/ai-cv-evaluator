// Package httpserver contains HTTP handlers and middleware.
//
// It provides REST API endpoints for the application including
// file upload, evaluation triggering, and result retrieval.
// The package follows clean architecture principles and provides
// a clear separation between HTTP concerns and business logic.
package httpserver

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-playground/validator/v10"

	"github.com/fairyhunter13/ai-cv-evaluator/internal/config"
	"github.com/fairyhunter13/ai-cv-evaluator/internal/domain"
	"github.com/fairyhunter13/ai-cv-evaluator/internal/usecase"
	"github.com/fairyhunter13/ai-cv-evaluator/pkg/textx"
	"github.com/gabriel-vasile/mimetype"
)

// Server aggregates handlers dependencies.
type Server struct {
	Cfg         config.Config
	Uploads     usecase.UploadService
	Evaluate    usecase.EvaluateService
	Results     usecase.ResultService
	Extractor   domain.TextExtractor
	DBCheck     func(ctx context.Context) error
	QdrantCheck func(ctx context.Context) error
	TikaCheck   func(ctx context.Context) error
}

// allowedMIME is kept for backward-compatibility with tests. It delegates to allowedMIMEFor
// using a dummy .txt filename to preserve the previous behavior for text/plain checks.
func allowedMIME(m string) bool { return allowedMIMEFor(m, "dummy.txt") }

// extractUploadedText performs text extraction based on the uploaded content and filename.
// - For .pdf/.docx: requires an external extractor (Apache Tika) and streams via a temp file.
// - For .txt: returns sanitized text directly.
func extractUploadedText(ctx context.Context, extractor domain.TextExtractor, h *multipart.FileHeader, data []byte) (string, error) {
	ext := strings.ToLower(filepath.Ext(h.Filename))
	if ext == ".pdf" || ext == ".docx" {
		if extractor == nil {
			return "", fmt.Errorf("%w: %s requires extractor", domain.ErrInvalidArgument, strings.TrimPrefix(ext, "."))
		}
		tmp, err := os.CreateTemp("", "upload-*")
		if err != nil {
			return "", err
		}
		defer func() { _ = os.Remove(tmp.Name()); _ = tmp.Close() }()
		if _, err := io.Copy(tmp, bytes.NewReader(data)); err != nil {
			return "", err
		}
		if _, err := tmp.Seek(0, io.SeekStart); err != nil {
			return "", err
		}
		return extractor.ExtractPath(ctx, h.Filename, tmp.Name())
	}
	// Treat as plain text with sanitization
	return textx.SanitizeText(string(data)), nil
}

// Admin cookie helpers removed; AdminServer with SessionManager handles authentication.

// allowedExt enforces an allowlist for uploads: .txt, .pdf, .docx
func allowedExt(name string) bool {
	n := strings.ToLower(name)
	return strings.HasSuffix(n, ".txt") || strings.HasSuffix(n, ".pdf") || strings.HasSuffix(n, ".docx")
}

func allowedMIMEFor(m string, filename string) bool {
	m = strings.ToLower(m)
	// For .txt files, accept any text/* including text/html as some detectors misclassify rich text
	if strings.HasSuffix(strings.ToLower(filename), ".txt") {
		if strings.HasPrefix(m, "text/") {
			return true
		}
	}
	if strings.HasPrefix(m, "text/plain") { // allow parameters such as charset
		return true
	}
	return m == "application/pdf" || m == "application/vnd.openxmlformats-officedocument.wordprocessingml.document"
}

var (
	vldOnce sync.Once
	vld     *validator.Validate
)

func getValidator() *validator.Validate {
	vldOnce.Do(func() { vld = validator.New() })
	return vld
}

// NewServer constructs an HTTP server with all handlers and checks wired.
func NewServer(cfg config.Config, uploads usecase.UploadService, eval usecase.EvaluateService, results usecase.ResultService, extractor domain.TextExtractor, dbCheck func(context.Context) error, qdrantCheck func(context.Context) error, tikaCheck func(context.Context) error) *Server {
	return &Server{Cfg: cfg, Uploads: uploads, Evaluate: eval, Results: results, Extractor: extractor, DBCheck: dbCheck, QdrantCheck: qdrantCheck, TikaCheck: tikaCheck}
}

// UploadHandler handles multipart upload of cv and project files.
func (s *Server) UploadHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// Accept negotiation: only JSON responses supported
		if a := r.Header.Get("Accept"); a != "" && a != "*/*" && !strings.Contains(a, "application/json") {
			w.Header().Set("Content-Type", "application/json; charset=utf-8")
			w.WriteHeader(http.StatusNotAcceptable)
			_ = json.NewEncoder(w).Encode(map[string]any{"error": map[string]any{"code": "INVALID_ARGUMENT", "message": "not acceptable", "details": map[string]any{"accept": a}}})
			return
		}
		if !strings.Contains(r.Header.Get("Content-Type"), "multipart/form-data") {
			writeError(w, r, fmt.Errorf("%w: content-type must be multipart/form-data", domain.ErrInvalidArgument), nil)
			return
		}
		// Limit total multipart size
		maxBytes := s.Cfg.MaxUploadMB * 1024 * 1024
		r.Body = http.MaxBytesReader(w, r.Body, maxBytes*2)
		if err := r.ParseMultipartForm(maxBytes * 2); err != nil {
			// Map body too large to 413
			if strings.Contains(strings.ToLower(err.Error()), "too large") || strings.Contains(strings.ToLower(err.Error()), "request body too large") {
				w.Header().Set("Content-Type", "application/json; charset=utf-8")
				w.WriteHeader(http.StatusRequestEntityTooLarge)
				_ = json.NewEncoder(w).Encode(map[string]any{"error": map[string]any{"code": "INVALID_ARGUMENT", "message": "payload too large", "details": map[string]any{"max_mb": s.Cfg.MaxUploadMB}}})
				return
			}
			writeError(w, r, fmt.Errorf("%w: %v", domain.ErrInvalidArgument, err), nil)
			return
		}
		cvFile, cvHeader, err := r.FormFile("cv")
		if err != nil {
			writeError(w, r, fmt.Errorf("%w: cv file required", domain.ErrInvalidArgument), map[string]string{"field": "cv"})
			return
		}
		defer func() { _ = cvFile.Close() }()
		projFile, projHeader, err := r.FormFile("project")
		if err != nil {
			writeError(w, r, fmt.Errorf("%w: project file required", domain.ErrInvalidArgument), map[string]string{"field": "project"})
			return
		}
		defer func() { _ = projFile.Close() }()

		// Read files into memory (body already capped by MaxBytesReader/ParseMultipartForm)
		cvBytes, err := io.ReadAll(cvFile)
		if err != nil {
			writeError(w, r, fmt.Errorf("%w: cv read: %v", domain.ErrInvalidArgument, err), nil)
			return
		}
		prBytes, err := io.ReadAll(projFile)
		if err != nil {
			writeError(w, r, fmt.Errorf("%w: project read: %v", domain.ErrInvalidArgument, err), nil)
			return
		}

		// Extension allowlist first
		if !allowedExt(cvHeader.Filename) {
			w.Header().Set("Content-Type", "application/json; charset=utf-8")
			w.WriteHeader(http.StatusUnsupportedMediaType)
			_ = json.NewEncoder(w).Encode(map[string]any{"error": map[string]any{"code": "INVALID_ARGUMENT", "message": "unsupported media type for cv (extension)", "details": map[string]any{"filename": cvHeader.Filename}}})
			return
		}
		if !allowedExt(projHeader.Filename) {
			w.Header().Set("Content-Type", "application/json; charset=utf-8")
			w.WriteHeader(http.StatusUnsupportedMediaType)
			_ = json.NewEncoder(w).Encode(map[string]any{"error": map[string]any{"code": "INVALID_ARGUMENT", "message": "unsupported media type for project (extension)", "details": map[string]any{"filename": projHeader.Filename}}})
			return
		}

		// Content sniffing with mimetype; enforce allowlist
		cvMime := mimetype.Detect(cvBytes)
		if !allowedMIMEFor(cvMime.String(), cvHeader.Filename) {
			w.Header().Set("Content-Type", "application/json; charset=utf-8")
			w.WriteHeader(http.StatusUnsupportedMediaType)
			_ = json.NewEncoder(w).Encode(map[string]any{"error": map[string]any{"code": "INVALID_ARGUMENT", "message": "unsupported media type for cv (content)", "details": map[string]any{"mime": cvMime.String(), "filename": cvHeader.Filename}}})
			return
		}
		prMime := mimetype.Detect(prBytes)
		if !allowedMIMEFor(prMime.String(), projHeader.Filename) {
			w.Header().Set("Content-Type", "application/json; charset=utf-8")
			w.WriteHeader(http.StatusUnsupportedMediaType)
			_ = json.NewEncoder(w).Encode(map[string]any{"error": map[string]any{"code": "INVALID_ARGUMENT", "message": "unsupported media type for project (content)", "details": map[string]any{"mime": prMime.String(), "filename": projHeader.Filename}}})
			return
		}

		// Extract text
		cvText, err := extractUploadedText(r.Context(), s.Extractor, cvHeader, cvBytes)
		if err != nil {
			writeError(w, r, fmt.Errorf("%w: cv extract: %v", domain.ErrInvalidArgument, err), nil)
			return
		}
		projText, err := extractUploadedText(r.Context(), s.Extractor, projHeader, prBytes)
		if err != nil {
			writeError(w, r, fmt.Errorf("%w: project extract: %v", domain.ErrInvalidArgument, err), nil)
			return
		}

		ctx := r.Context()
		cvID, projID, err := s.Uploads.Ingest(ctx, cvText, projText, cvHeader.Filename, projHeader.Filename)
		if err != nil {
			writeError(w, r, fmt.Errorf("upload ingest: %w", err), nil)
			return
		}
		writeJSON(w, http.StatusOK, map[string]string{"cv_id": cvID, "project_id": projID})
	}
}

// EvaluateHandler enqueues evaluation job.
func (s *Server) EvaluateHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// Accept negotiation: only JSON responses supported
		if a := r.Header.Get("Accept"); a != "" && a != "*/*" && !strings.Contains(a, "application/json") {
			w.Header().Set("Content-Type", "application/json; charset=utf-8")
			w.WriteHeader(http.StatusNotAcceptable)
			_ = json.NewEncoder(w).Encode(map[string]any{"error": map[string]any{"code": "INVALID_ARGUMENT", "message": "not acceptable", "details": map[string]any{"accept": a}}})
			return
		}
		// Cap body size to prevent abuse
		r.Body = http.MaxBytesReader(w, r.Body, 1<<20) // 1MB
		var req struct {
			CVID           string `json:"cv_id" validate:"required"`
			ProjectID      string `json:"project_id" validate:"required"`
			JobDescription string `json:"job_description" validate:"omitempty,max=5000"`
			StudyCaseBrief string `json:"study_case_brief" validate:"omitempty,max=5000"`
			ScoringRubric  string `json:"scoring_rubric" validate:"omitempty,max=10000"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeError(w, r, fmt.Errorf("%w: invalid json", domain.ErrInvalidArgument), nil)
			return
		}
		if err := getValidator().Struct(req); err != nil {
			verrs := map[string]string{}
			if ve, ok := err.(validator.ValidationErrors); ok {
				for _, fe := range ve {
					verrs[strings.ToLower(fe.Field())] = fe.Tag()
				}
			}
			writeError(w, r, fmt.Errorf("%w: validation failed", domain.ErrInvalidArgument), verrs)
			return
		}
		ctx := r.Context()

		// Use default values if not provided
		jobDescription := req.JobDescription
		studyCaseBrief := req.StudyCaseBrief
		scoringRubric := req.ScoringRubric

		if jobDescription == "" {
			jobDescription = getDefaultJobDescription()
		}
		if studyCaseBrief == "" {
			studyCaseBrief = getDefaultStudyCaseBrief()
		}
		if scoringRubric == "" {
			scoringRubric = getDefaultScoringRubric()
		}

		jobID, err := s.Evaluate.Enqueue(ctx, req.CVID, req.ProjectID, jobDescription, studyCaseBrief, scoringRubric, r.Header.Get("Idempotency-Key"))
		if err != nil {
			writeError(w, r, fmt.Errorf("enqueue: %w", err), nil)
			return
		}
		writeJSON(w, http.StatusOK, map[string]string{"id": jobID, "status": string(domain.JobQueued)})
	}
}

// ResultHandler returns job status and result when completed.
func (s *Server) ResultHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// Accept negotiation: only JSON responses supported
		if a := r.Header.Get("Accept"); a != "" && a != "*/*" && !strings.Contains(a, "application/json") {
			w.Header().Set("Content-Type", "application/json; charset=utf-8")
			w.WriteHeader(http.StatusNotAcceptable)
			_ = json.NewEncoder(w).Encode(map[string]any{"error": map[string]any{"code": "INVALID_ARGUMENT", "message": "not acceptable", "details": map[string]any{"accept": a}}})
			return
		}
		id := chi.URLParam(r, "id")
		if id == "" {
			writeError(w, r, fmt.Errorf("%w: id missing", domain.ErrInvalidArgument), nil)
			return
		}
		ctx := r.Context()
		status, res, etag, err := s.Results.Fetch(ctx, id, r.Header.Get("If-None-Match"))
		if err != nil {
			writeError(w, r, err, nil)
			return
		}
		w.Header().Set("ETag", etag)
		if status != http.StatusNotModified {
			writeJSON(w, status, res)
		} else {
			w.WriteHeader(status)
		}
	}
}

// ReadyzHandler returns a readiness handler that probes DB, Qdrant and Tika.
func (s *Server) ReadyzHandler() http.HandlerFunc {
	type check struct {
		Name    string `json:"name"`
		OK      bool   `json:"ok"`
		Details string `json:"details"`
	}
	return func(w http.ResponseWriter, r *http.Request) {
		ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
		defer cancel()
		checks := make([]check, 0, 3)
		// DB
		if s.DBCheck != nil {
			if err := s.DBCheck(ctx); err != nil {
				checks = append(checks, check{Name: "db", OK: false, Details: err.Error()})
			} else {
				checks = append(checks, check{Name: "db", OK: true})
			}
		}
		// Qdrant
		if s.QdrantCheck != nil {
			if err := s.QdrantCheck(ctx); err != nil {
				checks = append(checks, check{Name: "qdrant", OK: false, Details: err.Error()})
			} else {
				checks = append(checks, check{Name: "qdrant", OK: true})
			}
		}
		// Tika
		if s.TikaCheck != nil {
			if err := s.TikaCheck(ctx); err != nil {
				checks = append(checks, check{Name: "tika", OK: false, Details: err.Error()})
			} else {
				checks = append(checks, check{Name: "tika", OK: true})
			}
		}
		ok := true
		for _, c := range checks {
			if !c.OK {
				ok = false
				break
			}
		}
		st := http.StatusOK
		if !ok {
			st = http.StatusServiceUnavailable
		}
		writeJSON(w, st, map[string]any{"checks": checks})
	}
}

// OpenAPIServe serves api/openapi.yaml if present (used by admin UI and clients).
func (s *Server) OpenAPIServe() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		b, err := os.ReadFile("api/openapi.yaml")
		if err != nil {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "application/yaml; charset=utf-8")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(b)
	}
}

// MountAdmin mounts the admin interface using the AdminServer
func (s *Server) MountAdmin(r chi.Router) {
	// Create admin server instance
	adminServer, err := NewAdminServer(s.Cfg, s)
	if err != nil {
		// Log error but don't fail the main server
		return
	}

	// Mount admin routes directly
	r.Post("/admin/login", adminServer.AdminLoginHandler())
	r.Post("/admin/logout", adminServer.AdminLogoutHandler())
	r.Get("/admin/api/status", adminServer.AdminStatusHandler())
	r.Get("/admin/api/stats", adminServer.AdminStatsHandler())
	r.Get("/admin/api/jobs", adminServer.AdminJobsHandler())
	r.Get("/admin/api/jobs/{id}", adminServer.AdminJobDetailsHandler())
}

// Helper renderer for debug
// (intentionally no generic JSON renderer; use writeJSON)

// BuildResultEnvelope for /result
func BuildResultEnvelope(id string, status domain.JobStatus, result *usecase.EvaluationResult) map[string]any {
	m := map[string]any{"id": id, "status": string(status)}
	if status == domain.JobCompleted && result != nil {
		m["result"] = map[string]any{
			"cv_match_rate":    result.CVMatchRate,
			"cv_feedback":      result.CVFeedback,
			"project_score":    result.ProjectScore,
			"project_feedback": result.ProjectFeedback,
			"overall_summary":  result.OverallSummary,
		}
	}
	return m
}

// getDashboardStats returns dashboard statistics
func (s *Server) getDashboardStats(ctx context.Context) map[string]any {
	// Get total uploads count
	totalUploads, err := s.Uploads.Count(ctx)
	if err != nil {
		// Return error response with proper format
		return map[string]any{
			"error": map[string]any{
				"code":    "UPLOADS_COUNT_ERROR",
				"message": "Failed to retrieve uploads count",
				"details": map[string]any{
					"error": err.Error(),
				},
			},
			"uploads":     0,
			"evaluations": 0,
			"completed":   0,
			"avg_time":    0.0,
			"failed":      0,
		}
	}

	// Get total jobs count (evaluations)
	totalJobs, err := s.Evaluate.Jobs.Count(ctx)
	if err != nil {
		return map[string]any{
			"error": map[string]any{
				"code":    "JOBS_COUNT_ERROR",
				"message": "Failed to retrieve jobs count",
				"details": map[string]any{
					"error": err.Error(),
				},
			},
			"uploads":     totalUploads,
			"evaluations": 0,
			"completed":   0,
			"avg_time":    0.0,
			"failed":      0,
		}
	}

	// Get completed jobs count
	completedJobs, err := s.Evaluate.Jobs.CountByStatus(ctx, domain.JobCompleted)
	if err != nil {
		return map[string]any{
			"error": map[string]any{
				"code":    "COMPLETED_JOBS_COUNT_ERROR",
				"message": "Failed to retrieve completed jobs count",
				"details": map[string]any{
					"error": err.Error(),
				},
			},
			"uploads":     totalUploads,
			"evaluations": totalJobs,
			"completed":   0,
			"avg_time":    0.0,
			"failed":      0,
		}
	}

	// Get failed jobs count
	failedJobs, err := s.Evaluate.Jobs.CountByStatus(ctx, domain.JobFailed)
	if err != nil {
		return map[string]any{
			"error": map[string]any{
				"code":    "FAILED_JOBS_COUNT_ERROR",
				"message": "Failed to retrieve failed jobs count",
				"details": map[string]any{
					"error": err.Error(),
				},
			},
			"uploads":     totalUploads,
			"evaluations": totalJobs,
			"completed":   completedJobs,
			"avg_time":    0.0,
			"failed":      0,
		}
	}

	// Get average processing time for completed jobs
	avgTime, err := s.Evaluate.Jobs.GetAverageProcessingTime(ctx)
	if err != nil {
		return map[string]any{
			"error": map[string]any{
				"code":    "AVERAGE_TIME_ERROR",
				"message": "Failed to retrieve average processing time",
				"details": map[string]any{
					"error": err.Error(),
				},
			},
			"uploads":     totalUploads,
			"evaluations": totalJobs,
			"completed":   completedJobs,
			"avg_time":    0.0,
			"failed":      failedJobs,
		}
	}

	return map[string]any{
		"uploads":     totalUploads,
		"evaluations": totalJobs,
		"completed":   completedJobs,
		"avg_time":    avgTime,
		"failed":      failedJobs,
	}
}

// getJobs returns paginated job list with filtering and search
func (s *Server) getJobs(ctx context.Context, page, limit, search, status string) map[string]any {
	// Parse pagination parameters
	pageNum := 1
	limitNum := 10

	if p, err := strconv.Atoi(page); err == nil && p > 0 {
		pageNum = p
	}
	if l, err := strconv.Atoi(limit); err == nil && l > 0 && l <= 100 {
		limitNum = l
	}

	// Calculate offset
	offset := (pageNum - 1) * limitNum

	// Get jobs from database with filtering
	jobs, err := s.Evaluate.Jobs.ListWithFilters(ctx, offset, limitNum, search, status)
	if err != nil {
		// Return error response with proper format
		return map[string]any{
			"error": map[string]any{
				"code":    "DATABASE_ERROR",
				"message": "Failed to retrieve jobs",
				"details": map[string]any{
					"error": err.Error(),
				},
			},
			"jobs": []map[string]any{},
			"pagination": map[string]any{
				"page":  pageNum,
				"limit": limitNum,
				"total": 0,
			},
		}
	}

	// Get total count for pagination with filters
	totalCount, err := s.Evaluate.Jobs.CountWithFilters(ctx, search, status)
	if err != nil {
		// Use fallback count
		totalCount = int64(len(jobs))
	}

	// Convert domain jobs to response format
	jobList := make([]map[string]any, len(jobs))
	for i, job := range jobs {
		jobItem := map[string]any{
			"id":         job.ID,
			"status":     string(job.Status),
			"created_at": job.CreatedAt.Format(time.RFC3339),
			"updated_at": job.UpdatedAt.Format(time.RFC3339),
			"cv_id":      job.CVID,
			"project_id": job.ProjectID,
		}

		// Add error information if job failed
		if job.Status == domain.JobFailed && job.Error != "" {
			jobItem["error"] = map[string]any{
				"code":    "JOB_FAILED",
				"message": job.Error,
			}
		}

		jobList[i] = jobItem
	}

	return map[string]any{
		"jobs": jobList,
		"pagination": map[string]any{
			"page":  pageNum,
			"limit": limitNum,
			"total": totalCount,
		},
	}
}

// getJobDetails returns detailed information about a specific job
func (s *Server) getJobDetails(ctx context.Context, jobID string) map[string]any {
	// Get job from database
	job, err := s.Evaluate.Jobs.Get(ctx, jobID)
	if err != nil {
		// Return error response
		return map[string]any{
			"error": map[string]any{
				"code":    "JOB_NOT_FOUND",
				"message": "Job not found",
				"details": map[string]any{
					"job_id": jobID,
				},
			},
		}
	}

	// Build job details response
	jobDetails := map[string]any{
		"id":         job.ID,
		"status":     string(job.Status),
		"created_at": job.CreatedAt.Format(time.RFC3339),
		"updated_at": job.UpdatedAt.Format(time.RFC3339),
		"cv_id":      job.CVID,
		"project_id": job.ProjectID,
	}

	// Add error information if job failed
	if job.Status == domain.JobFailed && job.Error != "" {
		jobDetails["error"] = map[string]any{
			"code":    "JOB_FAILED",
			"message": job.Error,
		}
	}

	// If job is completed, try to get the result
	if job.Status == domain.JobCompleted {
		// Get result from the result service
		_, result, _, err := s.Results.Fetch(ctx, jobID, "")
		if err == nil && result != nil {
			// Extract result data from the response map
			if resultData, ok := result["result"].(map[string]any); ok {
				jobDetails["result"] = resultData
			}
		}
	}

	return jobDetails
}

// getDefaultJobDescription returns the default job description from config
func getDefaultJobDescription() string {
	return config.GetDefaultJobDescription()
}

// getDefaultStudyCaseBrief returns the default study case brief from config
func getDefaultStudyCaseBrief() string {
	return config.GetDefaultStudyCaseBrief()
}

// getDefaultScoringRubric returns the default scoring rubric from config
func getDefaultScoringRubric() string {
	return config.GetDefaultScoringRubric()
}
