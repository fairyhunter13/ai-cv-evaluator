package observability

import (
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/prometheus/client_golang/prometheus"
)

var (
	HTTPRequestsTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "http_requests_total",
			Help: "Total number of HTTP requests",
		},
		[]string{"route", "method", "status"},
	)
	HTTPRequestDuration = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "http_request_duration_seconds",
			Help:    "HTTP request duration in seconds",
			Buckets: []float64{0.05, 0.1, 0.25, 0.5, 1, 2, 5},
		},
		[]string{"route", "method"},
	)

	AIRequestsTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "ai_requests_total",
			Help: "Total number of AI requests by provider and operation",
		},
		[]string{"provider", "operation"},
	)
	AIRequestDuration = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "ai_request_duration_seconds",
			Help:    "AI request duration in seconds",
			Buckets: []float64{0.05, 0.1, 0.25, 0.5, 1, 2, 5, 10},
		},
		[]string{"provider", "operation"},
	)

	JobsEnqueuedTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "jobs_enqueued_total",
			Help: "Total number of jobs enqueued",
		},
		[]string{"type"},
	)
	JobsProcessing = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "jobs_processing",
			Help: "Number of jobs currently processing",
		},
		[]string{"type"},
	)
	JobsCompletedTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "jobs_completed_total",
			Help: "Total number of jobs completed",
		},
		[]string{"type"},
	)
	JobsFailedTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "jobs_failed_total",
			Help: "Total number of jobs failed",
		},
		[]string{"type"},
	)

	// Evaluation outcome distributions
	CVMatchRateHistogram = prometheus.NewHistogram(
		prometheus.HistogramOpts{
			Name:    "evaluation_cv_match_rate",
			Help:    "Distribution of cv_match_rate (normalized fraction [0,1])",
			Buckets: []float64{0, 0.1, 0.2, 0.3, 0.4, 0.5, 0.6, 0.7, 0.8, 0.9, 1.0},
		},
	)
	ProjectScoreHistogram = prometheus.NewHistogram(
		prometheus.HistogramOpts{
			Name:    "evaluation_project_score",
			Help:    "Distribution of project_score ([1,10])",
			Buckets: []float64{1, 2, 3, 4, 5, 6, 7, 8, 9, 10},
		},
	)
)

func InitMetrics() {
	prometheus.MustRegister(HTTPRequestsTotal)
	prometheus.MustRegister(HTTPRequestDuration)
	prometheus.MustRegister(AIRequestsTotal)
	prometheus.MustRegister(AIRequestDuration)
	prometheus.MustRegister(JobsEnqueuedTotal)
	prometheus.MustRegister(JobsProcessing)
	prometheus.MustRegister(JobsCompletedTotal)
	prometheus.MustRegister(JobsFailedTotal)
	prometheus.MustRegister(CVMatchRateHistogram)
	prometheus.MustRegister(ProjectScoreHistogram)
}

// HTTPMetricsMiddleware records Prometheus metrics for each request.
func HTTPMetricsMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		ww := middleware.NewWrapResponseWriter(w, r.ProtoMajor)
		next.ServeHTTP(ww, r)
		dur := time.Since(start).Seconds()
		// Route pattern may be unavailable outside chi router; guard nil
		var route string
		if rc := chi.RouteContext(r.Context()); rc != nil {
			route = rc.RoutePattern()
		}
		if route == "" {
			// fallback when route pattern is unavailable
			route = r.URL.Path
		}
		method := r.Method
		status := ww.Status()
		HTTPRequestsTotal.WithLabelValues(route, method, http.StatusText(status)).Inc()
		HTTPRequestDuration.WithLabelValues(route, method).Observe(dur)
	})
}

func EnqueueJob(jobType string) {
	JobsEnqueuedTotal.WithLabelValues(jobType).Inc()
}

func StartProcessingJob(jobType string) {
	JobsProcessing.WithLabelValues(jobType).Inc()
}

func CompleteJob(jobType string) {
	JobsProcessing.WithLabelValues(jobType).Dec()
	JobsCompletedTotal.WithLabelValues(jobType).Inc()
}

func FailJob(jobType string) {
	JobsProcessing.WithLabelValues(jobType).Dec()
	JobsFailedTotal.WithLabelValues(jobType).Inc()
}

// ObserveEvaluation records the resulting scores from completed evaluations.
func ObserveEvaluation(cvMatchRate, projectScore float64) {
	if cvMatchRate >= 0 && cvMatchRate <= 1 {
		CVMatchRateHistogram.Observe(cvMatchRate)
	}
	if projectScore >= 1 && projectScore <= 10 {
		ProjectScoreHistogram.Observe(projectScore)
	}
}
