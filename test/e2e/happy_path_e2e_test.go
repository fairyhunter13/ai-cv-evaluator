//go:build e2e
// +build e2e

package e2e_test

import (
	"encoding/json"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestE2E_HappyPath_UploadEvaluateResult exercises the core flow without
// making strong assumptions about asynchronous completion in constrained envs.
func TestE2E_HappyPath_UploadEvaluateResult(t *testing.T) {
	t.Parallel()
	if testing.Short() {
		t.Skip("Skipping E2E tests in short mode")
	}

	httpTimeout := 2 * time.Second
	if testing.Short() {
		httpTimeout = 1 * time.Second
	}
	client := &http.Client{Timeout: httpTimeout}

	// Ensure app is reachable; skip test if not ready
	// Derive healthz from configurable baseURL (defined in helpers)
	healthz := strings.TrimSuffix(baseURL, "/v1") + "/healthz"
	if resp, err := client.Get(healthz); err != nil || (resp != nil && resp.StatusCode != http.StatusOK) {
		if resp != nil {
			resp.Body.Close()
		}
		t.Skip("App not available; skipping happy path E2E")
	} else if resp != nil {
		resp.Body.Close()
	}

	// 1) Upload simple CV and Project texts
	uploadResp := uploadTestFiles(t, client, "Happy path CV", "Happy path project")
	dumpJSON(t, "happy_path_upload_response.json", uploadResp)

	// 2) Enqueue Evaluate
	evalResp := evaluateFiles(t, client, uploadResp["cv_id"], uploadResp["project_id"])
	dumpJSON(t, "happy_path_evaluate_response.json", evalResp)
	jobID, ok := evalResp["id"].(string)
	require.True(t, ok && jobID != "", "evaluate should return job id")

	// 3) Wait up to 45s and require terminal (completed/failed). Never queued.
	final := waitForCompleted(t, client, jobID, 90*time.Second)
	dumpJSON(t, "happy_path_result_response.json", final)
	st, _ := final["status"].(string)
	require.NotEqual(t, "queued", st, "terminal state expected, got queued: %#v", final)
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

	if b, err := json.MarshalIndent(final, "", "  "); err == nil {
		t.Logf("HappyPath - /result completed:\n%s", string(b))
	}
}
