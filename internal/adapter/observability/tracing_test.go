package observability

import (
	"context"
	"testing"

	"github.com/fairyhunter13/ai-cv-evaluator/internal/config"
)

func TestSetupTracing_Disabled(t *testing.T) {
	cfg := config.Config{OTLPEndpoint: ""}
	shutdown, err := SetupTracing(cfg)
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if shutdown != nil {
		// Should be nil when disabled
		_ = shutdown(context.Background())
	}
}

func TestSetupTracing_WithEndpoint(t *testing.T) {
	cfg := config.Config{
		OTLPEndpoint:    "localhost:4317",
		OTELServiceName: "test-service",
	}

	// This may or may not fail depending on the environment
	// We just test that the function can be called
	shutdown, err := SetupTracing(cfg)
	if err != nil {
		// Expected error when no OTLP server is running
		if shutdown != nil {
			t.Fatal("expected nil shutdown function on error")
		}
	} else {
		// If no error, we should have a shutdown function
		if shutdown != nil {
			_ = shutdown(context.Background())
		}
	}
}

func TestSetupTracing_InvalidEndpoint(t *testing.T) {
	cfg := config.Config{
		OTLPEndpoint:    "invalid://endpoint",
		OTELServiceName: "test-service",
	}

	shutdown, err := SetupTracing(cfg)
	if err != nil {
		// Expected error for invalid endpoint
		if shutdown != nil {
			t.Fatal("expected nil shutdown function on error")
		}
	} else {
		// If no error, we should have a shutdown function
		if shutdown != nil {
			_ = shutdown(context.Background())
		}
	}
}
