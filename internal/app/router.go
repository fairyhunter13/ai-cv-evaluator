package app

import (
	"net/http"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/cors"
	"github.com/go-chi/httprate"
	"github.com/prometheus/client_golang/prometheus/promhttp"

	httpserver "github.com/fairyhunter13/ai-cv-evaluator/internal/adapter/httpserver"
	"github.com/fairyhunter13/ai-cv-evaluator/internal/adapter/observability"
	"github.com/fairyhunter13/ai-cv-evaluator/internal/config"
)

// ParseOrigins splits a comma-separated origin list into a slice, trimming spaces.
// If the input is empty, returns ["*"].
func ParseOrigins(s string) []string {
	s = strings.TrimSpace(s)
	if s == "" {
		return []string{"*"}
	}
	if s == "*" {
		return []string{"*"}
	}
	parts := strings.Split(s, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p != "" {
			out = append(out, p)
		}
	}
	if len(out) == 0 {
		return []string{"*"}
	}
	return out
}

// BuildRouter constructs the HTTP handler with all middlewares and routes.
func BuildRouter(cfg config.Config, srv *httpserver.Server) http.Handler {
	r := chi.NewRouter()
	// Security & instrumentation middleware
	r.Use(httpserver.Recoverer())
	r.Use(httpserver.RequestID())
	r.Use(httpserver.TimeoutMiddleware(30 * time.Second))
	r.Use(httpserver.TraceMiddleware)
	r.Use(httpserver.AccessLog())
	r.Use(observability.HTTPMetricsMiddleware)

	// CORS - Updated for frontend separation
	r.Use(cors.Handler(cors.Options{
		AllowedOrigins:   append(ParseOrigins(cfg.CORSAllowOrigins), "http://localhost:3001"),
		AllowedMethods:   []string{"GET", "POST", "OPTIONS"},
		AllowedHeaders:   []string{"*"},
		ExposedHeaders:   []string{"X-Request-Id"},
		AllowCredentials: true, // Enable credentials for session management
		MaxAge:           300,
	}))

	// Rate limit mutating endpoints
	r.Group(func(wr chi.Router) {
		wr.Use(httprate.LimitByIP(cfg.RateLimitPerMin, 1*time.Minute))
		// If admin credentials are configured, require either session or Basic Auth
		if cfg.AdminEnabled() {
			wr.Use(srv.AdminAPIGuard())
		}
		wr.Post("/v1/upload", srv.UploadHandler())
		wr.Post("/v1/evaluate", srv.EvaluateHandler())
	})
	// Read-only endpoints
	r.Get("/v1/result/{id}", srv.ResultHandler())

	// Health and metrics
	r.Get("/healthz", func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusOK) })
	r.Get("/metrics", func(w http.ResponseWriter, r *http.Request) { promhttp.Handler().ServeHTTP(w, r) })
	r.Get("/readyz", srv.ReadyzHandler())

	// OpenAPI if present
	r.Get("/openapi.yaml", srv.OpenAPIServe())

	// Admin API endpoints for frontend authentication
	if cfg.AdminEnabled() {
		admin, err := httpserver.NewAdminServer(cfg, srv)
		if err == nil {
			r.Post("/admin/login", admin.AdminLoginHandler())
			r.Post("/admin/logout", admin.AdminLogoutHandler())
			r.Get("/admin/api/status", admin.AdminStatusHandler())
			r.Get("/admin/api/stats", admin.AdminStatsHandler())
			r.Get("/admin/api/jobs", admin.AdminJobsHandler())
			r.Get("/admin/api/jobs/{id}", admin.AdminJobDetailsHandler())
		}
	}

	return httpserver.SecurityHeaders(r)
}
