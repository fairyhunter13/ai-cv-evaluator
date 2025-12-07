package httpserver

import (
	"net/http/httptest"
	"os"
	"testing"

	"github.com/fairyhunter13/ai-cv-evaluator/internal/config"
	"github.com/fairyhunter13/ai-cv-evaluator/internal/domain"
	"github.com/fairyhunter13/ai-cv-evaluator/internal/usecase"
)

func Test_allowedExt(t *testing.T) {
	t.Run("accepts", func(t *testing.T) {
		for _, n := range []string{"cv.txt", "doc.PDF", "report.Docx"} {
			if !allowedExt(n) {
				t.Fatalf("should allow %s", n)
			}
		}
	})
	t.Run("rejects", func(t *testing.T) {
		for _, n := range []string{"evil.exe", "img.png", "cv"} {
			if allowedExt(n) {
				t.Fatalf("should reject %s", n)
			}
		}
	})
}

func Test_allowedMIME(t *testing.T) {
	if !allowedMIME("text/plain") {
		t.Fatalf("expected to allow text/plain")
	}
	if !allowedMIME("text/plain; charset=utf-8") {
		t.Fatalf("expected to allow text/plain charset")
	}
	if !allowedMIME("application/pdf") {
		t.Fatalf("expected to allow pdf")
	}
	if !allowedMIME("application/vnd.openxmlformats-officedocument.wordprocessingml.document") {
		t.Fatalf("expected to allow docx")
	}
	if allowedMIME("application/octet-stream") {
		t.Fatalf("should not allow octet-stream")
	}
}

func Test_BuildResultEnvelope(t *testing.T) {
	res := usecase.EvaluationResult{CVMatchRate: 0.8, CVFeedback: "a.", ProjectScore: 9, ProjectFeedback: "b.", OverallSummary: "c."}
	m := BuildResultEnvelope("id1", domain.JobCompleted, &res)
	if m["id"].(string) != "id1" {
		t.Fatalf("id mismatch")
	}
	if m["status"].(string) != string(domain.JobCompleted) {
		t.Fatalf("status mismatch")
	}
	inner := m["result"].(map[string]any)
	if inner["project_score"].(float64) != 9 {
		t.Fatalf("score mismatch")
	}

	m2 := BuildResultEnvelope("id2", domain.JobQueued, nil)
	if _, ok := m2["result"]; ok {
		t.Fatalf("queued should not include result")
	}
}

func Test_OpenAPIServe_200(t *testing.T) {
	cfg := config.Config{Port: 8080}
	upSvc := usecase.NewUploadService(nil)
	evSvc := usecase.NewEvaluateService(nil, nil, nil)
	resSvc := usecase.NewResultService(nil, nil)
	// Ensure api/openapi.yaml exists relative to test working dir
	if err := os.MkdirAll("api", 0o750); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	t.Cleanup(func() { _ = os.RemoveAll("api") })
	if err := os.WriteFile("api/openapi.yaml", []byte("openapi: 3.0.0\ninfo:\n  title: test\n  version: 1.0.0\n"), 0o600); err != nil {
		t.Fatalf("write openapi: %v", err)
	}
	s := NewServer(cfg, upSvc, evSvc, resSvc, nil, nil, nil, nil)
	rw := httptest.NewRecorder()
	s.OpenAPIServe()(rw, httptest.NewRequest("GET", "/openapi.yaml", nil))
	if rw.Result().StatusCode != 200 {
		t.Fatalf("want 200, got %d", rw.Result().StatusCode)
	}
}

func Test_newReqID(t *testing.T) {
	t.Parallel()

	// Test that newReqID generates unique IDs
	ids := make(map[string]bool)
	for i := 0; i < 100; i++ {
		id := newReqID()
		if id == "" {
			t.Fatal("newReqID returned empty string")
		}
		if ids[id] {
			t.Fatalf("duplicate ID generated: %s", id)
		}
		ids[id] = true
	}
}

func Test_newReqID_Format(t *testing.T) {
	t.Parallel()

	id := newReqID()
	// ULID is 26 characters
	if len(id) != 26 {
		// If not ULID, it should be timestamp format
		if len(id) < 20 {
			t.Fatalf("unexpected ID format: %s (len=%d)", id, len(id))
		}
	}
}
