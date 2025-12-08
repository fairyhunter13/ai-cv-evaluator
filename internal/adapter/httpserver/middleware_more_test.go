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
	if res.Header.Get("X-Content-Type-Options") != "nosniff" {
		t.Fatalf("missing header")
	}
	if res.Header.Get("X-Frame-Options") != "DENY" {
		t.Fatalf("missing header")
	}
	if res.Header.Get("Content-Security-Policy") == "" {
		t.Fatalf("missing csp")
	}
}

func Test_RequestID_SetsHeader(t *testing.T) {
	rec := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/x", nil)
	RequestID()(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(204) })).ServeHTTP(rec, r)
	if rec.Result().Header.Get("X-Request-Id") == "" {
		t.Fatalf("missing request id header")
	}
}

func Test_Recoverer_HandlesPanic(t *testing.T) {
	rec := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/x", nil)
	Recoverer()(http.HandlerFunc(func(_ http.ResponseWriter, _ *http.Request) { panic("boom") })).ServeHTTP(rec, r)
	if rec.Result().StatusCode != http.StatusInternalServerError {
		t.Fatalf("want 500")
	}
}

func Test_TimeoutMiddleware_GatewayTimeout(t *testing.T) {
	rec := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/x", nil)
	TimeoutMiddleware(5*time.Millisecond)(http.HandlerFunc(func(_ http.ResponseWriter, _ *http.Request) {
		time.Sleep(20 * time.Millisecond)
	})).ServeHTTP(rec, r)
	if rec.Result().StatusCode != http.StatusServiceUnavailable {
		t.Fatalf("want 503, got %d", rec.Result().StatusCode)
	}
}

func Test_TraceMiddleware_PassesThrough(t *testing.T) {
	rec := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/x", nil)
	TraceMiddleware(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(204) })).ServeHTTP(rec, r)
	if rec.Result().StatusCode != 204 {
		t.Fatalf("want 204")
	}
}

func Test_newReqID_ReturnsNonEmpty(t *testing.T) {
	id := newReqID()
	if id == "" {
		t.Fatalf("expected non-empty request ID")
	}
}

func Test_newReqID_UniqueIDs(t *testing.T) {
	id1 := newReqID()
	id2 := newReqID()
	if id1 == id2 {
		t.Fatalf("expected unique request IDs, got %s and %s", id1, id2)
	}
}

func Test_AccessLog_SetsLogger(t *testing.T) {
	rec := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/test", nil)
	AccessLog()(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(200) })).ServeHTTP(rec, r)
	if rec.Result().StatusCode != 200 {
		t.Fatalf("want 200, got %d", rec.Result().StatusCode)
	}
}

func Test_LoggerFrom_ReturnsDefault(t *testing.T) {
	r := httptest.NewRequest(http.MethodGet, "/test", nil)
	lg := LoggerFrom(r)
	if lg == nil {
		t.Fatalf("expected non-nil logger")
	}
}
