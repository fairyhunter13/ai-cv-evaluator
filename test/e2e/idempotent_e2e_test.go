//go:build e2e
// +build e2e

package e2e_test

import (
	"bytes"
	"encoding/json"
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

// postEvaluateWithIdempotency enqueues evaluation with a specific Idempotency-Key
// header and returns the decoded response body.
func postEvaluateWithIdempotency(t *testing.T, client *http.Client, cvID, projectID, idemKey string) map[string]any {
	t.Helper()

	payload := map[string]string{
		"cv_id":            cvID,
		"project_id":       projectID,
		"job_description":  defaultJobDescription,
		"study_case_brief": defaultStudyCaseBrief,
	}
	body, _ := json.Marshal(payload)

	var result map[string]any
	var lastStatus int
	for i := 0; i < 6; i++ {
		req, err := http.NewRequest("POST", baseURL+"/evaluate", bytes.NewReader(body))
		require.NoError(t, err)
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Idempotency-Key", idemKey)
		maybeBasicAuth(req)

		resp, err := client.Do(req)
		require.NoError(t, err)
		lastStatus = resp.StatusCode
		if resp.StatusCode == http.StatusOK {
			defer resp.Body.Close()
			require.NoError(t, json.NewDecoder(resp.Body).Decode(&result))
			break
		}

		resp.Body.Close()
		if resp.StatusCode == http.StatusTooManyRequests {
			// Back off slightly on 429 responses
			time.Sleep(500 * time.Millisecond)
			continue
		}
		break
	}

	require.Equal(t, http.StatusOK, lastStatus, "evaluate should succeed")
	return result
}

// TestE2E_Evaluate_IdempotentJobReuse verifies that repeated /evaluate calls
// with the same Idempotency-Key return the same job id and that the job
// eventually reaches a terminal completed state.
func TestE2E_Evaluate_IdempotentJobReuse(t *testing.T) {
	// NO SKIPPING - E2E tests must always run

	// Clear dump directory before test
	clearDumpDirectory(t)

	httpTimeout := 15 * time.Second
	client := &http.Client{Timeout: httpTimeout}

	// Ensure app is reachable; wait for readiness instead of failing on first attempt.
	waitForAppReady(t, client, 60*time.Second)

	// 1) Upload CV and Project texts
	uploadResp := uploadTestFiles(t, client, "Idempotent CV content", "Idempotent project content")
	dumpJSON(t, "idempotent_upload_response.json", uploadResp)

	cvID, ok := uploadResp["cv_id"].(string)
	require.True(t, ok, "cv_id should be a string")
	projectID, ok := uploadResp["project_id"].(string)
	require.True(t, ok, "project_id should be a string")

	idemKey := "e2e-idem-key-1"

	// 2) Call /evaluate twice with the same Idempotency-Key
	first := postEvaluateWithIdempotency(t, client, cvID, projectID, idemKey)
	second := postEvaluateWithIdempotency(t, client, cvID, projectID, idemKey)
	dumpJSON(t, "idempotent_first_evaluate_response.json", first)
	dumpJSON(t, "idempotent_second_evaluate_response.json", second)

	jobID1, _ := first["id"].(string)
	jobID2, _ := second["id"].(string)
	if jobID1 == "" || jobID2 == "" {
		t.Fatalf("/evaluate did not return job ids: first=%#v second=%#v", first, second)
	}

	// Idempotency: both responses must point to the same job id
	require.Equal(t, jobID1, jobID2, "idempotent evaluate should reuse the same job id")

	// 3) Wait until the job completes; this ensures the reused job id still
	// follows the normal async processing pipeline.
	final := waitForCompleted(t, client, jobID1, 360*time.Second)
	dumpJSON(t, "idempotent_result_response.json", final)

	st, _ := final["status"].(string)
	// CRITICAL: The idempotent job must reach a terminal state and reuse the
	// same job id. In constrained environments (e.g. missing or heavily
	// rate-limited upstream AI), allow a well-classified upstream timeout to
	// pass so this test still validates idempotent job reuse behavior.
	require.NotEqual(t, "queued", st, "E2E test failed: job stuck in queued state - %#v", final)
	require.NotEqual(t, "processing", st, "E2E test failed: job stuck in processing state - %#v", final)
	if st != "completed" {
		// Accept upstream timeout or rate-limit as an allowed terminal failure,
		// mirroring the behavior of the default RAG and other constrained E2E tests.
		errObj, ok := final["error"].(map[string]any)
		require.True(t, ok, "error object missing for failed idempotent job: %#v", final)
		code, _ := errObj["code"].(string)
		if code == "UPSTREAM_TIMEOUT" || code == "UPSTREAM_RATE_LIMIT" {
			t.Logf("Idempotent E2E: job failed with upstream code=%s; treating as acceptable in constrained environment", code)
			return
		}
		require.Equal(t, "completed", st, "unexpected failure code for idempotent job: %#v", errObj)
	}

	res, ok := final["result"].(map[string]any)
	require.True(t, ok, "result object missing for completed job")
	if _, ok := res["cv_match_rate"]; !ok {
		t.Fatalf("cv_match_rate missing: %#v", res)
	}
	if _, ok := res["project_score"]; !ok {
		t.Fatalf("project_score missing: %#v", res)
	}
}
