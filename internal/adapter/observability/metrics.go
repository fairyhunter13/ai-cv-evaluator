// Package observability provides logging, metrics, and tracing.
//
// It integrates with OpenTelemetry for system monitoring.
// The package provides comprehensive observability features
// including metrics collection, distributed tracing, and logging.
package observability

import (
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/prometheus/client_golang/prometheus"
)

var (
	// HTTPRequestsTotal counts HTTP requests by route, method, and status label.
	HTTPRequestsTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "http_requests_total",
			Help: "Total number of HTTP requests",
		},
		[]string{"route", "method", "status"},
	)
	// HTTPRequestDuration records request durations by route and method.
	HTTPRequestDuration = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "http_request_duration_seconds",
			Help:    "HTTP request duration in seconds",
			Buckets: []float64{0.05, 0.1, 0.25, 0.5, 1, 2, 5},
		},
		[]string{"route", "method"},
	)

	// AIRequestsTotal counts AI requests by provider and operation.
	AIRequestsTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "ai_requests_total",
			Help: "Total number of AI requests by provider and operation",
		},
		[]string{"provider", "operation"},
	)
	// AIRequestDuration records durations of AI requests by provider and operation.
	AIRequestDuration = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "ai_request_duration_seconds",
			Help:    "AI request duration in seconds",
			Buckets: []float64{0.05, 0.1, 0.25, 0.5, 1, 2, 5, 10},
		},
		[]string{"provider", "operation"},
	)

	// JobsEnqueuedTotal counts jobs enqueued by type.
	JobsEnqueuedTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "jobs_enqueued_total",
			Help: "Total number of jobs enqueued",
		},
		[]string{"type"},
	)
	// JobsProcessing is a gauge of the number of currently processing jobs by type.
	JobsProcessing = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "jobs_processing",
			Help: "Number of jobs currently processing",
		},
		[]string{"type"},
	)
	// JobsCompletedTotal counts jobs completed by type.
	JobsCompletedTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "jobs_completed_total",
			Help: "Total number of jobs completed",
		},
		[]string{"type"},
	)
	// JobsFailedTotal counts jobs failed by type.
	JobsFailedTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "jobs_failed_total",
			Help: "Total number of jobs failed",
		},
		[]string{"type"},
	)

	// CVMatchRateHistogram is the histogram of normalized cv_match_rate [0,1].
	CVMatchRateHistogram = prometheus.NewHistogram(
		prometheus.HistogramOpts{
			Name:    "evaluation_cv_match_rate",
			Help:    "Distribution of cv_match_rate (normalized fraction [0,1])",
			Buckets: []float64{0, 0.1, 0.2, 0.3, 0.4, 0.5, 0.6, 0.7, 0.8, 0.9, 1.0},
		},
	)
	// ProjectScoreHistogram is the histogram of project_score [1,10].
	ProjectScoreHistogram = prometheus.NewHistogram(
		prometheus.HistogramOpts{
			Name:    "evaluation_project_score",
			Help:    "Distribution of project_score ([1,10])",
			Buckets: []float64{1, 2, 3, 4, 5, 6, 7, 8, 9, 10},
		},
	)

	// AITokenUsage tracks AI token consumption by provider, type, and model.
	AITokenUsage = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "ai_tokens_total",
			Help: "Total AI tokens used",
		},
		[]string{"provider", "type", "model"},
	)

	// RAGEffectiveness tracks RAG retrieval effectiveness scores.
	RAGEffectiveness = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "rag_retrieval_effectiveness",
			Help:    "RAG retrieval effectiveness score",
			Buckets: []float64{0, 0.1, 0.2, 0.3, 0.4, 0.5, 0.6, 0.7, 0.8, 0.9, 1.0},
		},
		[]string{"collection", "query_type"},
	)

	// ScoreDriftDetector tracks score drift from baseline.
	ScoreDriftDetector = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "evaluation_score_drift",
			Help: "Detected score drift from baseline",
		},
		[]string{"metric_type", "model_version", "corpus_version"},
	)

	// CircuitBreakerStatus tracks circuit breaker state.
	CircuitBreakerStatus = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "circuit_breaker_status",
			Help: "Circuit breaker status (0=closed, 1=open, 2=half-open)",
		},
		[]string{"service", "operation"},
	)

	// RAGRetrievalErrors tracks RAG retrieval failures.
	RAGRetrievalErrors = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "rag_retrieval_errors_total",
			Help: "Total RAG retrieval errors",
		},
		[]string{"collection", "error_type"},
	)
)

// InitMetrics registers all Prometheus metrics with the default registry.
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
	prometheus.MustRegister(AITokenUsage)
	prometheus.MustRegister(RAGEffectiveness)
	prometheus.MustRegister(ScoreDriftDetector)
	prometheus.MustRegister(CircuitBreakerStatus)
	prometheus.MustRegister(RAGRetrievalErrors)
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

// EnqueueJob increments the enqueued jobs counter for the given type.
func EnqueueJob(jobType string) {
	JobsEnqueuedTotal.WithLabelValues(jobType).Inc()
}

// StartProcessingJob increments the processing gauge for the given type.
func StartProcessingJob(jobType string) {
	JobsProcessing.WithLabelValues(jobType).Inc()
}

// CompleteJob marks a job complete by decrementing processing gauge and incrementing completed counter.
func CompleteJob(jobType string) {
	JobsProcessing.WithLabelValues(jobType).Dec()
	JobsCompletedTotal.WithLabelValues(jobType).Inc()
}

// FailJob marks a job failed by decrementing processing gauge and incrementing failed counter.
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

// RecordAITokenUsage records AI token consumption.
func RecordAITokenUsage(provider, tokenType, model string, tokens int) {
	AITokenUsage.WithLabelValues(provider, tokenType, model).Add(float64(tokens))
}

// RecordRAGEffectiveness records RAG retrieval effectiveness.
func RecordRAGEffectiveness(collection, queryType string, effectiveness float64) {
	RAGEffectiveness.WithLabelValues(collection, queryType).Observe(effectiveness)
}

// RecordScoreDrift records score drift from baseline.
func RecordScoreDrift(metricType, modelVersion, corpusVersion string, drift float64) {
	ScoreDriftDetector.WithLabelValues(metricType, modelVersion, corpusVersion).Set(drift)
}

// RecordCircuitBreakerStatus records circuit breaker state.
func RecordCircuitBreakerStatus(service, operation string, status int) {
	CircuitBreakerStatus.WithLabelValues(service, operation).Set(float64(status))
}

// RecordRAGRetrievalError records RAG retrieval errors.
func RecordRAGRetrievalError(collection, errorType string) {
	RAGRetrievalErrors.WithLabelValues(collection, errorType).Inc()
}
