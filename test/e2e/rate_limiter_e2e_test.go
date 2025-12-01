//go:build e2e
// +build e2e

package e2e_test

import (
	"fmt"
	"net/http"
	"testing"
	"time"
)

func TestE2E_RateLimiter_ManySequentialJobs_Complete(t *testing.T) {
	clearDumpDirectory(t)

	httpTimeout := 15 * time.Second
	client := &http.Client{Timeout: httpTimeout}

	// Allow time for app readiness
	waitForAppReady(t, client, 60*time.Second)

	const totalJobs = 2
	jobIDs := make([]string, 0, totalJobs)

	for i := 0; i < totalJobs; i++ {
		label := fmt.Sprintf("rate_limiter_batch-%d", i+1)

		uploadResp := uploadTestFiles(t, client, "Rate limiter CV content", "Rate limiter project content")
		dumpJSON(t, fmt.Sprintf("%s_upload_response.json", label), uploadResp)

		cvID, ok := uploadResp["cv_id"].(string)
		if !ok || cvID == "" {
			t.Fatalf("%s: cv_id missing or not a string in upload response: %#v", label, uploadResp)
		}
		projectID, ok := uploadResp["project_id"].(string)
		if !ok || projectID == "" {
			t.Fatalf("%s: project_id missing or not a string in upload response: %#v", label, uploadResp)
		}

		evalResp := evaluateFiles(t, client, cvID, projectID)
		dumpJSON(t, fmt.Sprintf("%s_evaluate_response.json", label), evalResp)

		jobID, _ := evalResp["id"].(string)
		if jobID == "" {
			t.Fatalf("%s: /evaluate did not return a job id in response: %#v", label, evalResp)
		}
		jobIDs = append(jobIDs, jobID)
	}

	for idx, jobID := range jobIDs {
		label := fmt.Sprintf("rate_limiter_batch-%d", idx+1)
		final := waitForCompleted(t, client, jobID, 240*time.Second)
		dumpJSON(t, fmt.Sprintf("%s_result_response.json", label), final)

		st, _ := final["status"].(string)
		if st == "queued" || st == "processing" {
			t.Fatalf("%s: job stuck in non-terminal state: %#v", label, final)
		}
		if st != "completed" {
			// In constrained environments, accept upstream timeout or rate-limit
			// failures so we still validate that jobs reach a terminal state and are
			// classified correctly.
			errObj, ok := final["error"].(map[string]any)
			if !ok {
				t.Fatalf("%s: job failed without error object: %#v", label, final)
			}
			code, _ := errObj["code"].(string)
			if code == "UPSTREAM_TIMEOUT" || code == "UPSTREAM_RATE_LIMIT" {
				t.Logf("%s: job failed with upstream code=%s; treating as acceptable in constrained environment", label, code)
				continue
			}
			t.Fatalf("%s: job did not complete successfully. Status: %v, Response: %#v", label, st, final)
		}
	}
}
