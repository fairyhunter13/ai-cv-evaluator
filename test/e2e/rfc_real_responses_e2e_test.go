//go:build e2e
// +build e2e

package e2e_test

import (
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

// baseURL and dumpJSON are provided by helpers_e2e_test.go

// TestE2E_RFC_RealResponses_UploadEvaluateResult
// Fulfills the RFC requirement to produce real responses for /evaluate and /result using
// the candidate's real CV and a project report based on this repository.
// It logs the full JSON responses so they can be captured as screenshots.
func TestE2E_RFC_RealResponses_UploadEvaluateResult(t *testing.T) {
	// NO SKIPPING - E2E tests must always run
	t.Parallel() // Enable parallel execution

	// Clear dump directory before test
	clearDumpDirectory(t)

	httpTimeout := 3 * time.Second
	client := &http.Client{Timeout: httpTimeout}

	// Ensure app is reachable; skip test if not ready (derive from baseURL)
	healthz := strings.TrimSuffix(baseURL, "/v1") + "/healthz"
	if resp, err := client.Get(healthz); err != nil || (resp != nil && resp.StatusCode != http.StatusOK) {
		if resp != nil {
			resp.Body.Close()
		}
		t.Fatalf("App not available; healthz check failed: %v", err)
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
	cvID, ok := uploadResp["cv_id"].(string)
	require.True(t, ok, "cv_id should be a string")
	projectID, ok := uploadResp["project_id"].(string)
	require.True(t, ok, "project_id should be a string")
	evalResp := evaluateFiles(t, client, cvID, projectID)
	dumpJSON(t, "rfc_evaluate_response.json", evalResp)

	// Log /evaluate JSON response (RFC evidence)
	if b, err := json.MarshalIndent(evalResp, "", "  "); err == nil {
		t.Logf("RFC Evidence - /evaluate response:\n%s", string(b))
	}

	jobID, _ := evalResp["id"].(string)
	if jobID == "" {
		t.Fatalf("/evaluate did not return a job id in response: %#v", evalResp)
	}

	// 4) Poll until terminal (<= ~80s) to gather RFC evidence
	final := waitForCompleted(t, client, jobID, 300*time.Second)
	dumpJSON(t, "rfc_result_response.json", final)

	// CRITICAL: E2E tests must only accept successful completions
	st, _ := final["status"].(string)
	require.NotEqual(t, "queued", st, "E2E test failed: job stuck in queued state - %#v", final)
	require.NotEqual(t, "processing", st, "E2E test failed: job stuck in processing state - %#v", final)
	require.Equal(t, "completed", st, "E2E test failed: job did not complete successfully. Status: %v, Response: %#v", st, final)

	// Log /result JSON response (RFC evidence)
	if b, err := json.MarshalIndent(final, "", "  "); err == nil {
		t.Logf("RFC Evidence - /result response:\n%s", string(b))
	}
	// Validate successful completion
	res, ok := final["result"].(map[string]any)
	require.True(t, ok, "result object missing for completed job")
	if _, ok := res["cv_match_rate"]; !ok {
		t.Fatalf("cv_match_rate missing: %#v", res)
	}
	if _, ok := res["project_score"]; !ok {
		t.Fatalf("project_score missing: %#v", res)
	}

	// Test Summary
	t.Logf("=== RFC Real Responses E2E Test Summary ===")
	t.Logf("Test Status: %s", st)
	t.Logf("✅ Test PASSED - RFC evidence collected successfully")
	if res, ok := final["result"].(map[string]any); ok {
		t.Logf("Result contains: cv_match_rate=%v, project_score=%v",
			res["cv_match_rate"] != nil, res["project_score"] != nil)
	}
}
