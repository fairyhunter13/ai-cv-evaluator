package observability

import (
	"github.com/fairyhunter13/ai-cv-evaluator/internal/config"
	"testing"
)

func TestSetupLogger_DevAndProd(t *testing.T) {
	lg := SetupLogger(config.Config{AppEnv: "dev", OTELServiceName: "svc"})
	if lg == nil {
		t.Fatalf("nil logger")
	}
	lg2 := SetupLogger(config.Config{AppEnv: "prod", OTELServiceName: "svc"})
	if lg2 == nil {
		t.Fatalf("nil logger prod")
	}
}
