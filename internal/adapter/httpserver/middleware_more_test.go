package httpserver

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func Test_SecurityHeaders(t *testing.T) {
	rec := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/x", nil)
	SecurityHeaders(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(204) })).ServeHTTP(rec, r)
	res := rec.Result()
	if res.Header.Get("X-Content-Type-Options") != "nosniff" { t.Fatalf("missing header") }
	if res.Header.Get("X-Frame-Options") != "DENY" { t.Fatalf("missing header") }
	if res.Header.Get("Content-Security-Policy") == "" { t.Fatalf("missing csp") }
}

func Test_RequestID_SetsHeader(t *testing.T) {
	rec := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/x", nil)
	RequestID()(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(204) })).ServeHTTP(rec, r)
	if rec.Result().Header.Get("X-Request-Id") == "" { t.Fatalf("missing request id header") }
}

func Test_Recoverer_HandlesPanic(t *testing.T) {
	rec := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/x", nil)
	Recoverer()(http.HandlerFunc(func(_ http.ResponseWriter, _ *http.Request) { panic("boom") })).ServeHTTP(rec, r)
	if rec.Result().StatusCode != http.StatusInternalServerError { t.Fatalf("want 500") }
}

func Test_TimeoutMiddleware_GatewayTimeout(t *testing.T) {
	rec := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/x", nil)
	TimeoutMiddleware(5 * time.Millisecond)(http.HandlerFunc(func(_ http.ResponseWriter, _ *http.Request) {
		time.Sleep(20 * time.Millisecond)
	})).ServeHTTP(rec, r)
	if rec.Result().StatusCode != http.StatusServiceUnavailable { t.Fatalf("want 503, got %d", rec.Result().StatusCode) }
}

func Test_TraceMiddleware_PassesThrough(t *testing.T) {
	rec := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/x", nil)
	TraceMiddleware(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(204) })).ServeHTTP(rec, r)
	if rec.Result().StatusCode != 204 { t.Fatalf("want 204") }
}
