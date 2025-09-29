// Package httpserver contains the Admin UI server (templates, routes) and HTTP adapters.
package httpserver

import (
	"embed"
	"html/template"
	"io/fs"
	"net/http"

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
	
	// Parse templates with custom delimiters to avoid clashing with Vue's {{ }}
	templates, err := template.New("admin").Delims("[[", "]]").ParseFS(templateFiles, "templates/*.html")
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
			// RAG management and API endpoints removed per requirements
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
	session, _ := r.Context().Value(sessionKey{}).(*SessionData)
	
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
	session, _ := r.Context().Value(sessionKey{}).(*SessionData)
	
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
	session, _ := r.Context().Value(sessionKey{}).(*SessionData)
	
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
	session, _ := r.Context().Value(sessionKey{}).(*SessionData)
	
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

// API proxy and RAG management endpoints removed per requirements
