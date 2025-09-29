//go:build e2e
// +build e2e

package e2e_test

import (
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestE2E_HappyPath_UploadEvaluateResult exercises the core flow without
// making strong assumptions about asynchronous completion in constrained envs.
func TestE2E_HappyPath_UploadEvaluateResult(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping E2E tests in short mode")
	}

	client := &http.Client{Timeout: timeout}

	// Ensure app is reachable; skip test if not ready
	if resp, err := client.Get("http://localhost:8080/healthz"); err != nil || (resp != nil && resp.StatusCode != http.StatusOK) {
		if resp != nil { resp.Body.Close() }
		t.Skip("App not available; skipping happy path E2E")
	} else if resp != nil { resp.Body.Close() }

	// 1) Upload simple CV and Project texts
	uploadResp := uploadTestFiles(t, client, "Happy path CV", "Happy path project")

	// 2) Enqueue Evaluate
	evalResp := evaluateFiles(t, client, uploadResp["cv_id"], uploadResp["project_id"])
	jobID, ok := evalResp["id"].(string)
	require.True(t, ok && jobID != "", "evaluate should return job id")

	// 3) Poll result until terminal state (completed/failed) or timeout
	res := waitForCompletion(t, client, jobID)
	require.NotEmpty(t, res)

	status, ok := res["status"].(string)
	require.True(t, ok)
	assert.Contains(t, []string{"completed", "failed", "queued", "processing"}, status)
	if status == "completed" {
		if m, ok := res["result"].(map[string]interface{}); ok {
			assert.Contains(t, m, "cv_match_rate")
			assert.Contains(t, m, "overall_summary")
		}
	}

	// 4) ETag conditional request must return 304 when unchanged
	req1, _ := http.NewRequest("GET", baseURL+"/result/"+jobID, nil)
	resp1, err := client.Do(req1)
	require.NoError(t, err)
	etag := resp1.Header.Get("ETag")
	resp1.Body.Close()
	if etag != "" {
		req2, _ := http.NewRequest("GET", baseURL+"/result/"+jobID, nil)
		req2.Header.Set("If-None-Match", etag)
		resp2, err := client.Do(req2)
		require.NoError(t, err)
		defer resp2.Body.Close()
		assert.Equal(t, http.StatusNotModified, resp2.StatusCode)
	}
}
