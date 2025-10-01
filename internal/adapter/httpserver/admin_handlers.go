// Package httpserver contains the Admin API server and HTTP adapters.
package httpserver

import (
	"encoding/json"
	"net/http"

	"github.com/fairyhunter13/ai-cv-evaluator/internal/config"
	"github.com/go-chi/chi/v5"
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

// AdminLoginHandler handles API login for separated frontend
func (a *AdminServer) AdminLoginHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		username := r.FormValue("username")
		password := r.FormValue("password")

		// Simple credential check (in production, this should use a proper user store)
		if username != a.cfg.AdminUsername || password != a.cfg.AdminPassword {
			http.Error(w, "Invalid credentials", http.StatusUnauthorized)
			return
		}

		// Create session
		sessionValue, err := a.sessionManager.CreateSession(username)
		if err != nil {
			http.Error(w, "Failed to create session", http.StatusInternalServerError)
			return
		}

		a.sessionManager.SetSessionCookie(w, sessionValue)
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("Login successful"))
	}
}

// AdminLogoutHandler handles API logout for separated frontend
func (a *AdminServer) AdminLogoutHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, _ *http.Request) {
		a.sessionManager.ClearSessionCookie(w)
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("Logout successful"))
	}
}

// AdminStatusHandler returns admin status for separated frontend
func (a *AdminServer) AdminStatusHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// Check for session cookie
		cookie, err := r.Cookie("session")
		if err != nil || cookie.Value == "" {
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}

		// Validate session
		session, err := a.sessionManager.ValidateSession(cookie.Value)
		if err != nil {
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"status": "authenticated", "username": "` + session.Username + `"}`))
	}
}

// AdminStatsHandler returns dashboard statistics
func (a *AdminServer) AdminStatsHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// Check for session cookie
		cookie, err := r.Cookie("session")
		if err != nil || cookie.Value == "" {
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}

		// Validate session
		_, err = a.sessionManager.ValidateSession(cookie.Value)
		if err != nil {
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}

		// Get stats from the main server
		stats := a.server.getDashboardStats(r.Context())

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
		// Check for session cookie
		cookie, err := r.Cookie("session")
		if err != nil || cookie.Value == "" {
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}

		// Validate session
		_, err = a.sessionManager.ValidateSession(cookie.Value)
		if err != nil {
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
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
		jobs := a.server.getJobs(r.Context(), page, limit, search, status)

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
		// Check for session cookie
		cookie, err := r.Cookie("session")
		if err != nil || cookie.Value == "" {
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}

		// Validate session
		_, err = a.sessionManager.ValidateSession(cookie.Value)
		if err != nil {
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}

		// Get and validate job ID from URL path
		jobID := SanitizeJobID(chi.URLParam(r, "id"))

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
		jobDetails := a.server.getJobDetails(r.Context(), jobID)

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)

		// Return job details as JSON
		if err := json.NewEncoder(w).Encode(jobDetails); err != nil {
			http.Error(w, "Failed to encode job details", http.StatusInternalServerError)
			return
		}
	}
}
