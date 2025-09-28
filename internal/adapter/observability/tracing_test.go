package observability

import (
	"context"
	"testing"

	"github.com/fairyhunter13/ai-cv-evaluator/internal/config"
)

func TestSetupTracing_Disabled(t *testing.T) {
	cfg := config.Config{OTLPEndpoint: ""}
	shutdown, err := SetupTracing(cfg)
	if err != nil { t.Fatalf("unexpected err: %v", err) }
	if shutdown != nil {
		// Should be nil when disabled
		_ = shutdown(context.Background())
	}
}
