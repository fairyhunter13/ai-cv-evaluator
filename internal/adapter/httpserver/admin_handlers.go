// Package httpserver contains the Admin API server and HTTP adapters.
package httpserver

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/fairyhunter13/ai-cv-evaluator/internal/config"
	"github.com/go-chi/chi/v5"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
)

// AdminServer handles admin API routes
type AdminServer struct {
	cfg            config.Config
	sessionManager *SessionManager
	server         *Server // Reference to main server for API calls
}

// NewAdminServer creates a new admin server
func NewAdminServer(cfg config.Config, server *Server) (*AdminServer, error) {
	sessionManager := NewSessionManager(cfg)
	return &AdminServer{
		cfg:            cfg,
		sessionManager: sessionManager,
		server:         server,
	}, nil
}

// AdminLoginHandler removed: JWT is default authentication.

// AdminTokenHandler issues a JWT for admin APIs (alternative to cookie sessions)
func (a *AdminServer) AdminTokenHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		tracer := otel.Tracer("http.admin")
		_, span := tracer.Start(r.Context(), "AdminServer.AdminTokenHandler")
		defer span.End()

		lg := LoggerFrom(r)
		// Support form or JSON payload
		var username, password string
		ct := r.Header.Get("Content-Type")
		if strings.HasPrefix(strings.ToLower(ct), "application/json") {
			var body map[string]string
			_ = json.NewDecoder(r.Body).Decode(&body)
			username = strings.TrimSpace(body["username"])
			password = strings.TrimSpace(body["password"])
		} else {
			username = strings.TrimSpace(r.FormValue("username"))
			password = strings.TrimSpace(r.FormValue("password"))
		}

		if username != a.cfg.AdminUsername || password != a.cfg.AdminPassword {
			span.SetAttributes(attribute.Bool("auth.success", false))
			http.Error(w, "Invalid credentials", http.StatusUnauthorized)
			lg.Error("invalid credentials", slog.Any("username", username))
			return
		}

		// Issue JWT (24h)
		token, err := a.sessionManager.GenerateJWT(username, 24*time.Hour)
		if err != nil {
			http.Error(w, "Failed to issue token", http.StatusInternalServerError)
			lg.Error("failed to issue token", slog.Any("error", err))
			return
		}
		span.SetAttributes(
			attribute.Bool("auth.success", true),
			attribute.String("admin.username", username),
		)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(map[string]any{
			"token":    token,
			"username": username,
			"expires":  time.Now().Add(24 * time.Hour).Unix(),
		})
		lg.Info("issued token", slog.Any("username", username))
	}
}

// AdminLogoutHandler removed: JWT is stateless; clients can discard token.

// AdminStatusHandler returns dashboard statistics
func (a *AdminServer) AdminStatusHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		tracer := otel.Tracer("http.admin")
		_, span := tracer.Start(r.Context(), "AdminServer.AdminStatusHandler")
		defer span.End()

		lg := LoggerFrom(r)
		// Prefer SSO header injected by reverse proxy (e.g. oauth2-proxy)
		username := getSSOUsernameFromHeaders(r)
		if username == "" {
			// Fallback to Bearer JWT
			authz := strings.TrimSpace(r.Header.Get("Authorization"))
			if !strings.HasPrefix(strings.ToLower(authz), "bearer ") {
				http.Error(w, "Unauthorized", http.StatusUnauthorized)
				lg.Error("unauthorized", slog.Any("authz", authz))
				return
			}
			token := strings.TrimSpace(authz[len("Bearer "):])
			sub, err := a.sessionManager.ValidateJWT(token)
			if err != nil || sub == "" {
				http.Error(w, "Unauthorized", http.StatusUnauthorized)
				lg.Error("invalid token", slog.Any("error", err))
				return
			}
			username = sub
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"status": "authenticated", "username": "` + username + `"}`))
	}
}

// AdminStatsHandler returns dashboard statistics
func (a *AdminServer) AdminStatsHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		tracer := otel.Tracer("http.admin")
		ctx, span := tracer.Start(r.Context(), "AdminServer.AdminStatsHandler")
		defer span.End()
		// Prefer SSO header injected by reverse proxy (e.g. oauth2-proxy)
		if getSSOUsernameFromHeaders(r) == "" {
			// Fallback to Bearer JWT
			authz := strings.TrimSpace(r.Header.Get("Authorization"))
			if !strings.HasPrefix(strings.ToLower(authz), "bearer ") {
				http.Error(w, "Unauthorized", http.StatusUnauthorized)
				return
			}
			token := strings.TrimSpace(authz[len("Bearer "):])
			if _, err := a.sessionManager.ValidateJWT(token); err != nil {
				http.Error(w, "Unauthorized", http.StatusUnauthorized)
				return
			}
		}

		// Get stats from the main server
		stats := a.server.getDashboardStats(ctx)

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)

		// Return stats as JSON
		if err := json.NewEncoder(w).Encode(stats); err != nil {
			http.Error(w, "Failed to encode stats", http.StatusInternalServerError)
			return
		}
	}
}

// AdminJobsHandler returns paginated job list
func (a *AdminServer) AdminJobsHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		tracer := otel.Tracer("http.admin")
		ctx, span := tracer.Start(r.Context(), "AdminServer.AdminJobsHandler")
		defer span.End()
		// Prefer SSO header injected by reverse proxy (e.g. oauth2-proxy)
		if getSSOUsernameFromHeaders(r) == "" {
			// Fallback to Bearer JWT
			authz := strings.TrimSpace(r.Header.Get("Authorization"))
			if !strings.HasPrefix(strings.ToLower(authz), "bearer ") {
				http.Error(w, "Unauthorized", http.StatusUnauthorized)
				return
			}
			token := strings.TrimSpace(authz[len("Bearer "):])
			if _, err := a.sessionManager.ValidateJWT(token); err != nil {
				http.Error(w, "Unauthorized", http.StatusUnauthorized)
				return
			}
		}

		// Parse and validate query parameters
		page := SanitizeString(r.URL.Query().Get("page"))
		limit := SanitizeString(r.URL.Query().Get("limit"))
		search := SanitizeString(r.URL.Query().Get("search"))
		status := SanitizeString(r.URL.Query().Get("status"))

		// Validate pagination parameters
		if validation := ValidatePagination(page, limit); !validation.Valid {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusBadRequest)
			if err := json.NewEncoder(w).Encode(map[string]any{
				"error": map[string]any{
					"code":    "VALIDATION_ERROR",
					"message": "Invalid pagination parameters",
					"details": validation.Errors,
				},
			}); err != nil {
				http.Error(w, "Failed to encode error response", http.StatusInternalServerError)
			}
			return
		}

		// Validate search query
		if validation := ValidateSearchQuery(search); !validation.Valid {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusBadRequest)
			if err := json.NewEncoder(w).Encode(map[string]any{
				"error": map[string]any{
					"code":    "VALIDATION_ERROR",
					"message": "Invalid search query",
					"details": validation.Errors,
				},
			}); err != nil {
				http.Error(w, "Failed to encode error response", http.StatusInternalServerError)
			}
			return
		}

		// Validate status filter
		if validation := ValidateStatus(status); !validation.Valid {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusBadRequest)
			if err := json.NewEncoder(w).Encode(map[string]any{
				"error": map[string]any{
					"code":    "VALIDATION_ERROR",
					"message": "Invalid status filter",
					"details": validation.Errors,
				},
			}); err != nil {
				http.Error(w, "Failed to encode error response", http.StatusInternalServerError)
			}
			return
		}

		// Get jobs from the main server
		jobs := a.server.getJobs(ctx, page, limit, search, status)

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)

		// Return jobs as JSON
		if err := json.NewEncoder(w).Encode(jobs); err != nil {
			http.Error(w, "Failed to encode jobs", http.StatusInternalServerError)
			return
		}
	}
}

// AdminJobDetailsHandler returns individual job details
func (a *AdminServer) AdminJobDetailsHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		tracer := otel.Tracer("http.admin")
		ctx, span := tracer.Start(r.Context(), "AdminServer.AdminJobDetailsHandler")
		defer span.End()
		// Prefer SSO header injected by reverse proxy (e.g. oauth2-proxy)
		if getSSOUsernameFromHeaders(r) == "" {
			// Fallback to Bearer JWT
			authz := strings.TrimSpace(r.Header.Get("Authorization"))
			if !strings.HasPrefix(strings.ToLower(authz), "bearer ") {
				http.Error(w, "Unauthorized", http.StatusUnauthorized)
				return
			}
			token := strings.TrimSpace(authz[len("Bearer "):])
			if _, err := a.sessionManager.ValidateJWT(token); err != nil {
				http.Error(w, "Unauthorized", http.StatusUnauthorized)
				return
			}
		}

		// Get and validate job ID from URL path
		jobID := SanitizeJobID(chi.URLParam(r, "id"))
		span.SetAttributes(attribute.String("job.id", jobID))

		// Validate job ID
		if validation := ValidateJobID(jobID); !validation.Valid {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusBadRequest)
			if err := json.NewEncoder(w).Encode(map[string]any{
				"error": map[string]any{
					"code":    "VALIDATION_ERROR",
					"message": "Invalid job ID",
					"details": validation.Errors,
				},
			}); err != nil {
				http.Error(w, "Failed to encode error response", http.StatusInternalServerError)
			}
			return
		}

		// Get job details from the main server
		jobDetails := a.server.getJobDetails(ctx, jobID)

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)

		// Return job details as JSON
		if err := json.NewEncoder(w).Encode(jobDetails); err != nil {
			http.Error(w, "Failed to encode job details", http.StatusInternalServerError)
			return
		}
	}
}

// AdminAuthRequired middleware for protecting admin routes
func (a *AdminServer) AdminAuthRequired(next http.HandlerFunc) http.HandlerFunc {
	return a.sessionManager.AuthRequired(next).ServeHTTP
}
