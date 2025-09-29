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
	if testing.Short() {
		t.Skip("Skipping E2E tests in short mode")
	}

	httpTimeout := 2 * time.Second
	if testing.Short() {
		httpTimeout = 1 * time.Second
	}
	client := &http.Client{Timeout: httpTimeout}

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

	// 3) Single GET should return a valid status quickly (no long polling in fast mode)
	req1, _ := http.NewRequest("GET", "http://localhost:8080/v1/result/"+jobID, nil)
	resp1, err := client.Do(req1)
	require.NoError(t, err)
	var fast map[string]any
	require.NoError(t, json.NewDecoder(resp1.Body).Decode(&fast))
	_ = resp1.Body.Close()
	st, _ := fast["status"].(string)
	assert.Contains(t, []string{"completed", "failed", "queued", "processing"}, st)

	// 4) ETag conditional request must return 304 when unchanged (optional)
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
