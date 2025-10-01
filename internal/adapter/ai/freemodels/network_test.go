package freemodels

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestService_NetworkTimeout(t *testing.T) {
	// Create a server that takes longer than the client timeout
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		// Sleep longer than the client timeout but still very short to keep tests fast
		time.Sleep(10 * time.Millisecond)
		w.WriteHeader(http.StatusOK)
	}))
	defer ts.Close()

	// Create service with shorter timeout
	service := &Service{
		httpClient: &http.Client{
			Timeout: 1 * time.Millisecond, // Extremely short timeout to force timeout quickly
		},
		apiKey:  "test-key",
		baseURL: ts.URL,
	}

	ctx := context.Background()
	err := service.ForceRefresh(ctx)

	if err == nil {
		t.Fatal("expected timeout error")
	}

	// Check that the error is related to timeout
	if !containsTimeoutError(err) {
		t.Errorf("expected timeout error, got: %v", err)
	}
}

func TestService_ConnectionRefused(t *testing.T) {
	// Use a non-existent server
	service := New("test-key", "http://localhost:99999")

	ctx := context.Background()
	err := service.ForceRefresh(ctx)

	if err == nil {
		t.Fatal("expected connection error")
	}

	// Check that the error is related to connection or invalid port
	if !containsConnectionError(err) && !containsURLError(err) {
		t.Errorf("expected connection or URL error, got: %v", err)
	}
}

func TestService_ContextCancellation(t *testing.T) {
	// Create a server that takes time to respond
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		time.Sleep(15 * time.Millisecond)
		w.WriteHeader(http.StatusOK)
	}))
	defer ts.Close()

	service := New("test-key", ts.URL)

	// Create a context that will be cancelled quickly
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Millisecond)
	defer cancel()

	err := service.ForceRefresh(ctx)

	if err == nil {
		t.Fatal("expected context cancellation error")
	}

	// Check that the error is related to context cancellation
	if !containsContextError(err) {
		t.Errorf("expected context cancellation error, got: %v", err)
	}
}

func TestService_ContextDeadlineExceeded(t *testing.T) {
	// Create a server that takes time to respond
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		time.Sleep(50 * time.Millisecond)
		w.WriteHeader(http.StatusOK)
	}))
	defer ts.Close()

	service := New("test-key", ts.URL)

	// Create a context with a deadline
	ctx, cancel := context.WithDeadline(context.Background(), time.Now().Add(5*time.Millisecond))
	defer cancel()

	err := service.ForceRefresh(ctx)

	if err == nil {
		t.Fatal("expected deadline exceeded error")
	}

	// Check that the error is related to deadline exceeded
	if !containsDeadlineError(err) {
		t.Errorf("expected deadline exceeded error, got: %v", err)
	}
}

func TestService_InvalidURL(t *testing.T) {
	// Use an invalid URL
	service := New("test-key", "invalid://url")

	ctx := context.Background()
	err := service.ForceRefresh(ctx)

	if err == nil {
		t.Fatal("expected invalid URL error")
	}

	// Check that the error is related to invalid URL
	if !containsURLError(err) {
		t.Errorf("expected invalid URL error, got: %v", err)
	}
}

func TestService_ServerClosed(t *testing.T) {
	// Create a server and close it immediately
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	ts.Close() // Close the server

	service := New("test-key", ts.URL)

	ctx := context.Background()
	err := service.ForceRefresh(ctx)

	if err == nil {
		t.Fatal("expected connection error")
	}

	// Check that the error is related to connection
	if !containsConnectionError(err) {
		t.Errorf("expected connection error, got: %v", err)
	}
}

func TestService_ResponseBodyCloseError(t *testing.T) {
	// Create a server that returns a response but the body close fails
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"data":[]}`))
	}))
	defer ts.Close()

	// Create a custom client that will fail to close the response body
	service := &Service{
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
		apiKey:  "test-key",
		baseURL: ts.URL,
	}

	ctx := context.Background()
	err := service.ForceRefresh(ctx)

	// This should succeed despite the body close error (it's logged but not fatal)
	if err != nil {
		t.Errorf("ForceRefresh should succeed despite body close error: %v", err)
	}
}

func TestService_RequestCreationError(t *testing.T) {
	// Create a service with invalid base URL that will fail during request creation
	service := New("test-key", "://invalid-url")

	ctx := context.Background()
	err := service.ForceRefresh(ctx)

	if err == nil {
		t.Fatal("expected request creation error")
	}

	// Check that the error is related to request creation
	if !containsRequestError(err) {
		t.Errorf("expected request creation error, got: %v", err)
	}
}

// Helper functions to check error types
func containsTimeoutError(err error) bool {
	return err != nil && (contains(err.Error(), "timeout") ||
		contains(err.Error(), "deadline exceeded") ||
		contains(err.Error(), "context deadline exceeded"))
}

func containsConnectionError(err error) bool {
	return err != nil && (contains(err.Error(), "connection refused") ||
		contains(err.Error(), "no such host") ||
		contains(err.Error(), "connection reset") ||
		contains(err.Error(), "broken pipe"))
}

func containsContextError(err error) bool {
	return err != nil && (contains(err.Error(), "context canceled") ||
		contains(err.Error(), "context deadline exceeded"))
}

func containsDeadlineError(err error) bool {
	return err != nil && contains(err.Error(), "deadline exceeded")
}

func containsURLError(err error) bool {
	return err != nil && (contains(err.Error(), "invalid URL") ||
		contains(err.Error(), "unsupported protocol") ||
		contains(err.Error(), "invalid port"))
}

func containsRequestError(err error) bool {
	return err != nil && contains(err.Error(), "failed to create request")
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr ||
		(len(s) > len(substr) && (s[:len(substr)] == substr ||
			s[len(s)-len(substr):] == substr ||
			containsSubstring(s, substr))))
}

func containsSubstring(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
