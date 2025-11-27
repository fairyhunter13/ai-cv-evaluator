//go:build e2e
// +build e2e

package e2e_test

import (
	"encoding/json"
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestE2E_HappyPath_UploadEvaluateResult exercises the core flow without
// making strong assumptions about asynchronous completion in constrained envs.
func TestE2E_HappyPath_UploadEvaluateResult(t *testing.T) {
	// NO SKIPPING - E2E tests must always run

	// Clear dump directory before test
	clearDumpDirectory(t)

	httpTimeout := 2 * time.Second
	if testing.Short() {
		httpTimeout = 1 * time.Second
	}
	client := &http.Client{Timeout: httpTimeout}

	// Ensure app is reachable; wait for readiness instead of failing on first attempt.
	waitForAppReady(t, client, 60*time.Second)

	// 1) Upload simple CV and Project texts
	uploadResp := uploadTestFiles(t, client, "Happy path CV", "Happy path project")
	dumpJSON(t, "happy_path_upload_response.json", uploadResp)

	// 2) Enqueue Evaluate
	cvID, ok := uploadResp["cv_id"].(string)
	require.True(t, ok, "cv_id should be a string")
	projectID, ok := uploadResp["project_id"].(string)
	require.True(t, ok, "project_id should be a string")
	evalResp := evaluateFiles(t, client, cvID, projectID)
	dumpJSON(t, "happy_path_evaluate_response.json", evalResp)
	jobID, ok := evalResp["id"].(string)
	require.True(t, ok && jobID != "", "evaluate should return job id")

	// 3) Wait up to 60s and require terminal (completed/failed). Never queued.
	// Note: AI model processing can be slow, so we use a reasonable timeout
	final := waitForCompleted(t, client, jobID, 300*time.Second)
	dumpJSON(t, "happy_path_result_response.json", final)
	st, _ := final["status"].(string)

	// CRITICAL: E2E tests must only accept successful completions
	require.NotEqual(t, "queued", st, "E2E test failed: job stuck in queued state - %#v", final)
	require.NotEqual(t, "processing", st, "E2E test failed: job stuck in processing state - %#v", final)
	require.Equal(t, "completed", st, "E2E test failed: job did not complete successfully. Status: %v, Response: %#v", st, final)
	switch st {
	case "completed":
		res, ok := final["result"].(map[string]any)
		require.True(t, ok, "result object missing")
		_, hasCV := res["cv_match_rate"]
		_, hasCVF := res["cv_feedback"]
		_, hasProj := res["project_score"]
		_, hasProjF := res["project_feedback"]
		_, hasSummary := res["overall_summary"]
		assert.True(t, hasCV && hasCVF && hasProj && hasProjF && hasSummary, "incomplete result payload: %#v", res)
	case "failed":
		if _, ok := final["error"].(map[string]any); !ok {
			t.Fatalf("expected error object for failed status: %#v", final)
		}
	default:
		t.Fatalf("unexpected status: %v", st)
	}

	// Test Summary
	t.Logf("=== HappyPath E2E Test Summary ===")
	t.Logf("Test Status: %s", st)
	if st == "completed" {
		t.Logf("✅ Test PASSED - Job completed successfully")
		if res, ok := final["result"].(map[string]any); ok {
			t.Logf("Result contains: cv_match_rate=%v, project_score=%v",
				res["cv_match_rate"] != nil, res["project_score"] != nil)
		}
	} else if st == "failed" {
		t.Logf("⚠️ Test PASSED - Job failed (expected in E2E due to AI model limitations)")
		if err, ok := final["error"].(map[string]any); ok {
			t.Logf("Error details: %v", err)
		}
	}

	if b, err := json.MarshalIndent(final, "", "  "); err == nil {
		t.Logf("HappyPath - /result completed:\n%s", string(b))
	}
}
