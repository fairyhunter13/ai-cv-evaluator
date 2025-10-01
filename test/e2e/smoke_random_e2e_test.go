//go:build e2e
// +build e2e

package e2e_test

import (
	"encoding/json"
	"net/http"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

// TestE2E_SmokeRandom uploads a random CV/project pair from testdata and ensures evaluate enqueues and result endpoint responds.
func TestE2E_SmokeRandom(t *testing.T) {
	// NO SKIPPING - E2E tests must always run
	t.Parallel() // Enable parallel execution

	// Clear dump directory before test
	clearDumpDirectory(t)

	httpTimeout := 2 * time.Second
	client := &http.Client{Timeout: httpTimeout}

	// quick health check using configurable baseURL
	healthz := strings.TrimSuffix(baseURL, "/v1") + "/healthz"
	if resp, err := client.Get(healthz); err != nil || (resp != nil && resp.StatusCode != http.StatusOK) {
		if resp != nil {
			resp.Body.Close()
		}
		t.Fatalf("App not available; healthz check failed: %v", err)
	} else if resp != nil {
		resp.Body.Close()
	}

	// pick deterministic pair from test/testdata to avoid flakiness
	pairs := availablePairs()
	require.NotEmpty(t, pairs)
	// Use test name hash for deterministic selection instead of random
	seed := int64(len(t.Name())) % int64(len(pairs))
	p := pairs[seed]

	// upload & evaluate
	upload := uploadTestFiles(t, client, string(p.CV), string(p.Project))
	dumpJSON(t, "smoke_random_upload_response.json", upload)
	cvID, ok := upload["cv_id"].(string)
	require.True(t, ok, "cv_id should be a string")
	projectID, ok := upload["project_id"].(string)
	require.True(t, ok, "project_id should be a string")
	eval := evaluateFiles(t, client, cvID, projectID)
	dumpJSON(t, "smoke_random_evaluate_response.json", eval)

	// wait until completed (AI model processing can be slow)
	final := waitForCompleted(t, client, eval["id"].(string), 300*time.Second)
	dumpJSON(t, "smoke_random_result_response.json", final)

	// CRITICAL: E2E tests must only accept successful completions
	st, _ := final["status"].(string)
	require.NotEqual(t, "queued", st, "E2E test failed: job stuck in queued state - %#v", final)
	require.NotEqual(t, "processing", st, "E2E test failed: job stuck in processing state - %#v", final)
	require.Equal(t, "completed", st, "E2E test failed: job did not complete successfully. Status: %v, Response: %#v", st, final)

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
	t.Logf("=== Smoke Random E2E Test Summary ===")
	t.Logf("Test Status: %s", st)
	t.Logf("✅ Test PASSED - Random CV/Project pair completed successfully")
	if res, ok := final["result"].(map[string]any); ok {
		t.Logf("Result contains: cv_match_rate=%v, project_score=%v",
			res["cv_match_rate"] != nil, res["project_score"] != nil)
	}

	if b, err := json.MarshalIndent(final, "", "  "); err == nil {
		t.Logf("SmokeRandom - /result completed:\n%s", string(b))
	}
}

type pair struct{ CV, Project []byte }

func availablePairs() []pair {
	files := []struct{ cv, project string }{
		{"cv_01.txt", "project_01.txt"},
		{"cv_02.txt", "project_02.txt"},
		{"cv_03.txt", "project_03.txt"},
		{"cv_04.txt", "project_04.txt"},
		{"cv_05.txt", "project_05.txt"},
		{"cv_06.txt", "project_06.txt"},
		{"cv_07.txt", "project_07.txt"},
		{"cv_08.txt", "project_08.txt"},
		{"cv_09.txt", "project_09.txt"},
		{"cv_10.txt", "project_10.txt"},
		{"cv_11.txt", "project_11.txt"},
	}
	out := make([]pair, 0, len(files))
	for _, f := range files {
		cvP := filepath.FromSlash("../testdata/" + f.cv)
		prP := filepath.FromSlash("../testdata/" + f.project)
		cvB, _ := osReadFile(cvP)
		prB, _ := osReadFile(prP)
		if len(cvB) > 0 && len(prB) > 0 {
			out = append(out, pair{CV: cvB, Project: prB})
		}
	}
	return out
}

// helper to avoid importing os directly here;
// implemented below to keep a minimal import list in this file
func osReadFile(p string) ([]byte, error) {
	return osReadFileImpl(p)
}
