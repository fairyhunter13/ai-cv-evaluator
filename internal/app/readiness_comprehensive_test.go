package app

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/fairyhunter13/ai-cv-evaluator/internal/app/mocks"
	"github.com/fairyhunter13/ai-cv-evaluator/internal/config"
	"github.com/stretchr/testify/mock"
)

func TestBuildReadinessChecks_Database(t *testing.T) {
	tests := []struct {
		name        string
		pool        Pinger
		expectError bool
	}{
		{"nil pool", nil, true},
		{"working pool", createMockPinger(t, false, ""), false},
		{"failing pool", createMockPinger(t, true, "connection failed"), true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := config.Config{
				QdrantURL: "http://localhost:6333",
				TikaURL:   "http://localhost:9998",
			}

			dbCheck, _, _ := BuildReadinessChecks(cfg, tt.pool)

			err := dbCheck(context.Background())
			if tt.expectError && err == nil {
				t.Error("Expected error but got none")
			}
			if !tt.expectError && err != nil {
				t.Errorf("Expected no error but got: %v", err)
			}
		})
	}
}

func createMockPinger(t *testing.T, shouldError bool, errorMsg string) *mocks.Pinger {
	mockPinger := mocks.NewPinger(t)
	if shouldError {
		mockPinger.EXPECT().Ping(mock.Anything).Return(fmt.Errorf("%s", errorMsg))
	} else {
		mockPinger.EXPECT().Ping(mock.Anything).Return(nil)
	}
	return mockPinger
}

func TestBuildReadinessChecks_Qdrant(t *testing.T) {
	tests := []struct {
		name        string
		statusCode  int
		expectError bool
		apiKey      string
	}{
		{"success", 200, false, ""},
		{"success with API key", 200, false, "test-key"},
		{"not found", 404, true, ""},
		{"server error", 500, true, ""},
		{"unauthorized", 401, true, ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				// Check if API key is set correctly
				if tt.apiKey != "" {
					if r.Header.Get("api-key") != tt.apiKey {
						t.Errorf("Expected API key %q, got %q", tt.apiKey, r.Header.Get("api-key"))
					}
				}
				w.WriteHeader(tt.statusCode)
			}))
			defer server.Close()

			cfg := config.Config{
				QdrantURL:    server.URL,
				QdrantAPIKey: tt.apiKey,
				TikaURL:      "http://localhost:9998",
			}

			_, qdrantCheck, _ := BuildReadinessChecks(cfg, nil)

			err := qdrantCheck(context.Background())
			if tt.expectError && err == nil {
				t.Error("Expected error but got none")
			}
			if !tt.expectError && err != nil {
				t.Errorf("Expected no error but got: %v", err)
			}
		})
	}
}

func TestBuildReadinessChecks_Tika(t *testing.T) {
	tests := []struct {
		name        string
		tikaURL     string
		statusCode  int
		expectError bool
	}{
		{"success", "http://localhost:9998", 200, false},
		{"not found", "http://localhost:9998", 404, true},
		{"server error", "http://localhost:9998", 500, true},
		{"empty URL", "", 200, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var server *httptest.Server
			if tt.tikaURL != "" {
				server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
					w.WriteHeader(tt.statusCode)
				}))
				defer server.Close()
			}

			cfg := config.Config{
				QdrantURL: "http://localhost:6333",
				TikaURL:   tt.tikaURL,
			}
			if server != nil {
				cfg.TikaURL = server.URL
			}

			_, _, tikaCheck := BuildReadinessChecks(cfg, nil)

			err := tikaCheck(context.Background())
			if tt.expectError && err == nil {
				t.Error("Expected error but got none")
			}
			if !tt.expectError && err != nil {
				t.Errorf("Expected no error but got: %v", err)
			}
		})
	}
}

func TestBuildReadinessChecks_ContextCancellation(t *testing.T) {
	cfg := config.Config{
		QdrantURL: "http://localhost:6333",
		TikaURL:   "http://localhost:9998",
	}

	// Create a context that's already cancelled
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	dbCheck, qdrantCheck, tikaCheck := BuildReadinessChecks(cfg, nil)

	// All checks should fail due to cancelled context
	checks := []struct {
		name string
		fn   func(context.Context) error
	}{
		{"database", dbCheck},
		{"qdrant", qdrantCheck},
		{"tika", tikaCheck},
	}

	for _, check := range checks {
		t.Run(check.name, func(t *testing.T) {
			err := check.fn(ctx)
			if err == nil {
				t.Error("Expected error due to cancelled context")
			}
		})
	}
}

func TestBuildReadinessChecks_Timeout(t *testing.T) {
	// Create a server that takes longer than the timeout
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		time.Sleep(3 * time.Second) // Longer than the 2s timeout to ensure it always times out
		w.WriteHeader(200)
	}))
	defer server.Close()

	cfg := config.Config{
		QdrantURL: server.URL,
		TikaURL:   server.URL,
	}

	_, qdrantCheck, tikaCheck := BuildReadinessChecks(cfg, nil)

	// Both should timeout
	ctx := context.Background()

	if err := qdrantCheck(ctx); err == nil {
		t.Error("Expected qdrant check to timeout")
	}

	if err := tikaCheck(ctx); err == nil {
		t.Error("Expected tika check to timeout")
	}
}

func TestBuildReadinessChecks_WithAllServices(t *testing.T) {
	cfg := config.Config{
		QdrantURL:    "http://localhost:6333",
		TikaURL:      "http://localhost:9998",
		QdrantAPIKey: "test-key",
	}

	// Mock all services
	dbPool := createMockPinger(t, false, "")
	dbCheck, qdrantCheck, tikaCheck := BuildReadinessChecks(cfg, dbPool)

	// Test that all checks can be called
	ctx := context.Background()

	// These should not panic even if they fail
	_ = dbCheck(ctx)
	_ = qdrantCheck(ctx)
	_ = tikaCheck(ctx)
}

func TestBuildReadinessChecks_EmptyConfig(_ *testing.T) {
	cfg := config.Config{}

	dbCheck, qdrantCheck, tikaCheck := BuildReadinessChecks(cfg, nil)

	// Test that all checks can be called with empty config
	ctx := context.Background()

	// These should not panic even with empty config
	_ = dbCheck(ctx)
	_ = qdrantCheck(ctx)
	_ = tikaCheck(ctx)
}
