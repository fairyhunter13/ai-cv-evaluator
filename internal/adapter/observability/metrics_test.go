package observability

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestHTTPMetricsMiddleware_Basic(t *testing.T) {
	rec := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/x", nil)
	mw := HTTPMetricsMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(204) }))
	mw.ServeHTTP(rec, r)
	if rec.Result().StatusCode != 204 { t.Fatalf("want 204") }
}

func TestJobMetricsHelpers(t *testing.T) {
	InitMetrics()
	EnqueueJob("eval")
	StartProcessingJob("eval")
	CompleteJob("eval")
	FailJob("eval")
	ObserveEvaluation(0.5, 7)
}
