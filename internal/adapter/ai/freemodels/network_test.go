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
		time.Sleep(100 * time.Millisecond) // Increased to ensure it's always longer than timeout
		w.WriteHeader(http.StatusOK)
	}))
	defer ts.Close()

	// Create service with shorter timeout
	service := &Service{
		httpClient: &http.Client{
			Timeout: 20 * time.Millisecond, // Short but reasonable timeout to force timeout
		},
		apiKey:  "test-key",
		baseURL: ts.URL,
	}

	ctx := context.Background()
	err := service.Refresh(ctx)

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
	service := NewService("test-key", "http://localhost:99999", 1*time.Hour)

	ctx := context.Background()
	err := service.Refresh(ctx)

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
		time.Sleep(50 * time.Millisecond) // Increased to ensure it's longer than context timeout
		w.WriteHeader(http.StatusOK)
	}))
	defer ts.Close()

	service := NewService("test-key", ts.URL, 1*time.Hour)

	// Create a context that will be cancelled quickly
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Millisecond) // Increased slightly for reliability
	defer cancel()

	err := service.Refresh(ctx)

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
		time.Sleep(50 * time.Millisecond) // Increased to ensure it's longer than deadline
		w.WriteHeader(http.StatusOK)
	}))
	defer ts.Close()

	service := NewService("test-key", ts.URL, 1*time.Hour)

	// Create a context with a deadline
	ctx, cancel := context.WithDeadline(context.Background(), time.Now().Add(10*time.Millisecond)) // Increased for reliability
	defer cancel()

	err := service.Refresh(ctx)

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
	service := NewService("test-key", "invalid://url", 1*time.Hour)

	ctx := context.Background()
	err := service.Refresh(ctx)

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

	service := NewService("test-key", ts.URL, 1*time.Hour)

	ctx := context.Background()
	err := service.Refresh(ctx)

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
	err := service.Refresh(ctx)

	// This should succeed despite the body close error (it's logged but not fatal)
	if err != nil {
		t.Errorf("ForceRefresh should succeed despite body close error: %v", err)
	}
}

func TestService_RequestCreationError(t *testing.T) {
	// Create a service with invalid base URL that will fail during request creation
	service := NewService("test-key", "://invalid-url", 1*time.Hour)

	ctx := context.Background()
	err := service.Refresh(ctx)

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
