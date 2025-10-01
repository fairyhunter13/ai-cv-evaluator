package usecase

import (
	"context"
	"testing"
)

func TestEvaluateService_Readiness(t *testing.T) {
	svc := NewEvaluateService(nil, nil, nil)
	checks := svc.Readiness(context.TODO())
	if len(checks) != 2 {
		t.Fatalf("want 2 checks, got %d", len(checks))
	}
	for _, c := range checks {
		if !c.OK {
			t.Fatalf("expected OK in %s", c.Name)
		}
	}
}
