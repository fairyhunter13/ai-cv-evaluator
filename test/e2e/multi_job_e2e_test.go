//go:build e2e
// +build e2e

package e2e_test

import (
	"fmt"
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

// TestE2E_MultiJob_Sequential_Batch runs a small batch of sequential evaluation
// jobs to exercise repeated end-to-end processing while remaining well below
// provider rate limits. It reuses the standard upload/evaluate helpers and
// validates that each job reaches a terminal state and returns a well-formed
// result or error payload.
func TestE2E_MultiJob_Sequential_Batch(t *testing.T) {
	// Skip in smoke mode - this test makes multiple sequential AI calls
	skipIfSmokeMode(t, "multi-job batch test makes multiple sequential AI calls which may trigger rate limits")

	// Clear dump directory before test to avoid mixing artifacts across runs.
	clearDumpDirectory(t)

	httpTimeout := 15 * time.Second
	client := &http.Client{Timeout: httpTimeout}

	// Ensure app is reachable; wait for readiness instead of failing on first attempt.
	waitForAppReady(t, client, 60*time.Second)

	// Use a small batch size that still exercises repeated end-to-end processing but
	// reliably fits under the 5-minute go test timeout when combined with multi-step
	// LLM evaluation latency.
	const totalJobs = 2
	for i := 0; i < totalJobs; i++ {
		jobLabel := fmt.Sprintf("batch-%d", i+1)

		// 1) Upload CV and Project texts for this batch job
		uploadResp := uploadTestFiles(t, client,
			fmt.Sprintf("Sequential batch CV content %s", jobLabel),
			fmt.Sprintf("Sequential batch project content %s", jobLabel),
		)
		dumpJSON(t, fmt.Sprintf("multi_job_%s_upload_response.json", jobLabel), uploadResp)

		cvID, ok := uploadResp["cv_id"].(string)
		require.True(t, ok, "cv_id should be a string")
		projectID, ok := uploadResp["project_id"].(string)
		require.True(t, ok, "project_id should be a string")

		// 2) Enqueue evaluation using the standard helper (explicit job description
		// and study case brief). This exercises the full async queue + worker
		// pipeline multiple times in a single test.
		evalResp := evaluateFiles(t, client, cvID, projectID)
		dumpJSON(t, fmt.Sprintf("multi_job_%s_evaluate_response.json", jobLabel), evalResp)

		jobID, _ := evalResp["id"].(string)
		if jobID == "" {
			t.Fatalf("/evaluate did not return a job id in response for %s: %#v", jobLabel, evalResp)
		}

		// 3) Wait until this job completes.
		final := waitForCompleted(t, client, jobID, 240*time.Second)
		dumpJSON(t, fmt.Sprintf("multi_job_%s_result_response.json", jobLabel), final)

		st, _ := final["status"].(string)
		// CRITICAL: E2E tests must only accept terminal completions (completed or
		// failed) and must not allow jobs to remain stuck in queued/processing.
		require.NotEqual(t, "queued", st, "E2E multi-job failed: job %s stuck in queued state - %#v", jobLabel, final)
		require.NotEqual(t, "processing", st, "E2E multi-job failed: job %s stuck in processing state - %#v", jobLabel, final)
		if st != "completed" {
			// For failed terminal states, ensure a well-formed error payload exists.
			errObj, ok := final["error"].(map[string]any)
			require.True(t, ok, "error object missing for failed job %s: %#v", jobLabel, final)
			if code, _ := errObj["code"].(string); code != "" {
				t.Logf("multi-job %s: job failed with code=%s; accepting as terminal failure", jobLabel, code)
			} else {
				t.Logf("multi-job %s: job failed without explicit code; accepting as terminal failure: %#v", jobLabel, errObj)
			}
			continue
		}

		res, ok := final["result"].(map[string]any)
		require.True(t, ok, "result object missing for completed job %s", jobLabel)
		if _, ok := res["cv_match_rate"]; !ok {
			t.Fatalf("cv_match_rate missing for job %s: %#v", jobLabel, res)
		}
		if _, ok := res["project_score"]; !ok {
			t.Fatalf("project_score missing for job %s: %#v", jobLabel, res)
		}
	}
}
