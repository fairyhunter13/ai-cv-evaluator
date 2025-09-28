package httpserver

import (
	"embed"
	"encoding/json"
	"html/template"
	"io/fs"
	"net/http"
	"strconv"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/fairyhunter13/ai-cv-evaluator/internal/config"
)

//go:embed static
var staticFiles embed.FS

//go:embed templates/*
var templateFiles embed.FS

// AdminServer handles admin interface routes
type AdminServer struct {
	cfg            config.Config
	sessionManager *SessionManager
	server         *Server // Reference to main server for API calls
	templates      *template.Template
}

// NewAdminServer creates a new admin server
func NewAdminServer(cfg config.Config, server *Server) (*AdminServer, error) {
	sessionManager := NewSessionManager(cfg)
	
	// Parse templates
	templates, err := template.ParseFS(templateFiles, "templates/*.html")
	if err != nil {
		return nil, err
	}

	return &AdminServer{
		cfg:            cfg,
		sessionManager: sessionManager,
		server:         server,
		templates:      templates,
	}, nil
}

// MountRoutes mounts admin routes on the router
func (a *AdminServer) MountRoutes(r chi.Router) {
	if !a.cfg.AdminEnabled() {
		return
	}

	r.Route("/admin", func(adminRouter chi.Router) {
		// Static files
		adminRouter.Handle("/static/*", http.StripPrefix("/admin/static/", http.FileServer(http.FS(staticFiles))))
		
		// Public routes (login)
		adminRouter.Get("/login", a.LoginPage)
		adminRouter.Post("/login", a.LoginHandler)
		adminRouter.Post("/logout", a.LogoutHandler)

		// Protected routes
		adminRouter.Group(func(protected chi.Router) {
			protected.Use(a.sessionManager.AuthRequired)
			
			protected.Get("/", a.DashboardPage)
			protected.Get("/upload", a.UploadPage)
			protected.Get("/evaluate", a.EvaluatePage)
			protected.Get("/result", a.ResultPage)
			protected.Get("/rag", a.RAGManagementPage)
			
			// API endpoints for the Vue app
			protected.Post("/api/upload", a.APIUploadHandler)
			protected.Post("/api/evaluate", a.APIEvaluateHandler)
			protected.Get("/api/result/{id}", a.APIResultHandler)
			protected.Get("/api/rag", a.APIRAGListHandler)
			protected.Post("/api/rag/job_description", a.APIRAGUpdateJobDescHandler)
			protected.Post("/api/rag/scoring_rubric", a.APIRAGUpdateRubricHandler)
		})
	})

	// Serve static files
	staticFS, err := fs.Sub(staticFiles, "static")
	if err == nil {
		r.Handle("/admin/static/*", http.StripPrefix("/admin/static/", http.FileServer(http.FS(staticFS))))
	}
}

// LoginPage renders the login form
func (a *AdminServer) LoginPage(w http.ResponseWriter, r *http.Request) {
	// Check if already logged in
	if cookie, err := r.Cookie("session"); err == nil && cookie.Value != "" {
		if _, err := a.sessionManager.ValidateSession(cookie.Value); err == nil {
			http.Redirect(w, r, "/admin/", http.StatusSeeOther)
			return
		}
	}

	data := struct {
		CSRFToken string
		Error     string
	}{
		CSRFToken: "", // Remove CSRF for now
		Error:     r.URL.Query().Get("error"),
	}

	w.Header().Set("Content-Type", "text/html")
	if err := a.templates.ExecuteTemplate(w, "login.html", data); err != nil {
		http.Error(w, "Template error", http.StatusInternalServerError)
		return
	}
}

// LoginHandler processes login form submission
func (a *AdminServer) LoginHandler(w http.ResponseWriter, r *http.Request) {
	username := r.FormValue("username")
	password := r.FormValue("password")

	// Simple credential check (in production, this should use a proper user store)
	if username != a.cfg.AdminUsername || password != a.cfg.AdminPassword {
		http.Redirect(w, r, "/admin/login?error=invalid_credentials", http.StatusSeeOther)
		return
	}

	// Create session
	sessionValue, err := a.sessionManager.CreateSession(username)
	if err != nil {
		http.Error(w, "Failed to create session", http.StatusInternalServerError)
		return
	}

	a.sessionManager.SetSessionCookie(w, sessionValue)
	http.Redirect(w, r, "/admin/", http.StatusSeeOther)
}

// LogoutHandler handles logout
func (a *AdminServer) LogoutHandler(w http.ResponseWriter, r *http.Request) {
	a.sessionManager.ClearSessionCookie(w)
	http.Redirect(w, r, "/admin/login", http.StatusSeeOther)
}

// DashboardPage renders the main dashboard
func (a *AdminServer) DashboardPage(w http.ResponseWriter, r *http.Request) {
	session := r.Context().Value("session").(*SessionData)
	
	data := struct {
		Username  string
		CSRFToken string
	}{
		Username:  session.Username,
		CSRFToken: "", // Remove CSRF for now
	}

	w.Header().Set("Content-Type", "text/html")
	if err := a.templates.ExecuteTemplate(w, "dashboard.html", data); err != nil {
		http.Error(w, "Template error", http.StatusInternalServerError)
		return
	}
}

// UploadPage renders the upload interface
func (a *AdminServer) UploadPage(w http.ResponseWriter, r *http.Request) {
	session := r.Context().Value("session").(*SessionData)
	
	data := struct {
		Username  string
		CSRFToken string
	}{
		Username:  session.Username,
		CSRFToken: "", // Remove CSRF for now
	}

	w.Header().Set("Content-Type", "text/html")
	if err := a.templates.ExecuteTemplate(w, "upload.html", data); err != nil {
		http.Error(w, "Template error", http.StatusInternalServerError)
		return
	}
}

// EvaluatePage renders the evaluation interface
func (a *AdminServer) EvaluatePage(w http.ResponseWriter, r *http.Request) {
	session := r.Context().Value("session").(*SessionData)
	
	data := struct {
		Username  string
		CSRFToken string
	}{
		Username:  session.Username,
		CSRFToken: "", // Remove CSRF for now
	}

	w.Header().Set("Content-Type", "text/html")
	if err := a.templates.ExecuteTemplate(w, "evaluate.html", data); err != nil {
		http.Error(w, "Template error", http.StatusInternalServerError)
		return
	}
}

// ResultPage renders the result interface
func (a *AdminServer) ResultPage(w http.ResponseWriter, r *http.Request) {
	session := r.Context().Value("session").(*SessionData)
	
	data := struct {
		Username  string
		CSRFToken string
	}{
		Username:  session.Username,
		CSRFToken: "", // Remove CSRF for now
	}

	w.Header().Set("Content-Type", "text/html")
	if err := a.templates.ExecuteTemplate(w, "result.html", data); err != nil {
		http.Error(w, "Template error", http.StatusInternalServerError)
		return
	}
}

// RAGManagementPage renders the RAG management interface
func (a *AdminServer) RAGManagementPage(w http.ResponseWriter, r *http.Request) {
	session := r.Context().Value("session").(*SessionData)
	
	data := struct {
		Username  string
		CSRFToken string
	}{
		Username:  session.Username,
		CSRFToken: "", // Remove CSRF for now
	}

	w.Header().Set("Content-Type", "text/html")
	if err := a.templates.ExecuteTemplate(w, "rag.html", data); err != nil {
		http.Error(w, "Template error", http.StatusInternalServerError)
		return
	}
}

// API handlers that proxy to the main server

// APIUploadHandler proxies upload requests to the main upload handler
func (a *AdminServer) APIUploadHandler(w http.ResponseWriter, r *http.Request) {
	handler := a.server.UploadHandler()
	handler(w, r)
}

// APIEvaluateHandler proxies evaluate requests to the main evaluate handler
func (a *AdminServer) APIEvaluateHandler(w http.ResponseWriter, r *http.Request) {
	handler := a.server.EvaluateHandler()
	handler(w, r)
}

// APIResultHandler proxies result requests to the main result handler
func (a *AdminServer) APIResultHandler(w http.ResponseWriter, r *http.Request) {
	handler := a.server.ResultHandler()
	handler(w, r)
}

// RAGStatus represents the status of RAG corpora
type RAGStatus struct {
	JobDescription struct {
		Version    string    `json:"version"`
		Source     string    `json:"source"`
		IngestedAt time.Time `json:"ingested_at"`
		Documents  int       `json:"documents"`
	} `json:"job_description"`
	ScoringRubric struct {
		Version    string    `json:"version"`
		Source     string    `json:"source"`
		IngestedAt time.Time `json:"ingested_at"`
		Documents  int       `json:"documents"`
	} `json:"scoring_rubric"`
}

// APIRAGListHandler returns the status of RAG corpora
func (a *AdminServer) APIRAGListHandler(w http.ResponseWriter, r *http.Request) {
	// Mock data for now - in a real implementation, this would query Qdrant
	status := RAGStatus{}
	status.JobDescription.Version = "v1.0"
	status.JobDescription.Source = "configs/rag/job_description.yaml"
	status.JobDescription.IngestedAt = time.Now().Add(-24 * time.Hour)
	status.JobDescription.Documents = 5

	status.ScoringRubric.Version = "v1.0"
	status.ScoringRubric.Source = "configs/rag/scoring_rubric.yaml"
	status.ScoringRubric.IngestedAt = time.Now().Add(-24 * time.Hour)
	status.ScoringRubric.Documents = 8

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(status)
}

// APIRAGUpdateJobDescHandler handles job description corpus updates
func (a *AdminServer) APIRAGUpdateJobDescHandler(w http.ResponseWriter, r *http.Request) {
	// Mock implementation - would integrate with seeding logic
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"success": true,
		"message": "Job description corpus updated successfully",
		"version": "v" + strconv.FormatInt(time.Now().Unix(), 10),
	})
}

// APIRAGUpdateRubricHandler handles scoring rubric corpus updates  
func (a *AdminServer) APIRAGUpdateRubricHandler(w http.ResponseWriter, r *http.Request) {
	// Mock implementation - would integrate with seeding logic
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"success": true,
		"message": "Scoring rubric corpus updated successfully",
		"version": "v" + strconv.FormatInt(time.Now().Unix(), 10),
	})
}
