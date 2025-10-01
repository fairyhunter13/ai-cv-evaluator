//go:build e2e

package e2e_test

import (
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// baseURL and dumpJSON are provided by helpers_e2e_test.go

// TestE2E_RFC_RealResponses_UploadEvaluateResult
// Fulfills the RFC requirement to produce real responses for /evaluate and /result using
// the candidate's real CV and a project report based on this repository.
// It logs the full JSON responses so they can be captured as screenshots.
func TestE2E_RFC_RealResponses_UploadEvaluateResult(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping E2E tests in short mode")
	}

	httpTimeout := 3 * time.Second
	client := &http.Client{Timeout: httpTimeout}

	// Ensure app is reachable; skip test if not ready (derive from baseURL)
	healthz := strings.TrimSuffix(baseURL, "/v1") + "/healthz"
	if resp, err := client.Get(healthz); err != nil || (resp != nil && resp.StatusCode != http.StatusOK) {
		if resp != nil {
			resp.Body.Close()
		}
		t.Skip("App not available; skipping RFC evidence E2E")
	} else if resp != nil {
		resp.Body.Close()
	}

	// 1) Load real testdata: CV and project report (repository-based)
	cvPath := filepath.FromSlash("../testdata/cv_optimized_2025.md")
	prPath := filepath.FromSlash("../testdata/project_repo_report.txt")
	cvBytes, err := os.ReadFile(cvPath)
	if err != nil {
		t.Fatalf("failed to read CV testdata: %v", err)
	}
	prBytes, err := os.ReadFile(prPath)
	if err != nil {
		t.Fatalf("failed to read project report testdata: %v", err)
	}

	// 2) Upload files
	uploadResp := uploadTestFiles(t, client, string(cvBytes), string(prBytes))
	dumpJSON(t, "rfc_upload_response.json", uploadResp)

	// 3) Enqueue evaluate
	evalResp := evaluateFiles(t, client, uploadResp["cv_id"], uploadResp["project_id"])
	dumpJSON(t, "rfc_evaluate_response.json", evalResp)

	// Log /evaluate JSON response (RFC evidence)
	if b, err := json.MarshalIndent(evalResp, "", "  "); err == nil {
		t.Logf("RFC Evidence - /evaluate response:\n%s", string(b))
	}

	jobID, _ := evalResp["id"].(string)
	if jobID == "" {
		t.Fatalf("/evaluate did not return a job id in response: %#v", evalResp)
	}

	// 4) Poll until terminal (<= ~60s) to gather RFC evidence
	final := waitForCompleted(t, client, jobID, 120*time.Second)
	dumpJSON(t, "rfc_result_response.json", final)
	// Log /result JSON response (RFC evidence)
	if b, err := json.MarshalIndent(final, "", "  "); err == nil {
		t.Logf("RFC Evidence - /result response:\n%s", string(b))
	}
	st, _ := final["status"].(string)
	switch st {
	case "completed":
		res, ok := final["result"].(map[string]any)
		if !ok {
			t.Fatalf("result object missing: %#v", final)
		}
		if _, ok := res["cv_match_rate"]; !ok {
			t.Fatalf("cv_match_rate missing: %#v", res)
		}
		if _, ok := res["project_score"]; !ok {
			t.Fatalf("project_score missing: %#v", res)
		}
	case "failed":
		if _, ok := final["error"].(map[string]any); !ok {
			t.Fatalf("expected error object for failed status: %#v", final)
		}
	default:
		t.Fatalf("expected terminal status (completed/failed), got: %v", st)
	}
}
