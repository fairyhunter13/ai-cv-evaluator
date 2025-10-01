//go:build e2e

package e2e_test

import (
	"encoding/json"
	"math/rand"
	"net/http"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)


// TestE2E_SmokeRandom uploads a random CV/project pair from testdata and ensures evaluate enqueues and result endpoint responds.
func TestE2E_SmokeRandom(t *testing.T) {
	t.Parallel()
	if testing.Short() {
		t.Skip("short mode")
	}

	httpTimeout := 2 * time.Second
	client := &http.Client{Timeout: httpTimeout}

	// quick health check using configurable baseURL
	healthz := strings.TrimSuffix(baseURL, "/v1") + "/healthz"
	if resp, err := client.Get(healthz); err != nil || (resp != nil && resp.StatusCode != http.StatusOK) {
		if resp != nil {
			resp.Body.Close()
		}
		t.Skip("App not available; skipping smoke random E2E")
	} else if resp != nil {
		resp.Body.Close()
	}

	// pick random pair from test/testdata
	pairs := availablePairs()
	require.NotEmpty(t, pairs)
	rand.Seed(time.Now().UnixNano())
	p := pairs[rand.Intn(len(pairs))]

	// upload & evaluate
	upload := uploadTestFiles(t, client, string(p.CV), string(p.Project))
	dumpJSON(t, "smoke_random_upload_response.json", upload)
	eval := evaluateFiles(t, client, upload["cv_id"], upload["project_id"])
	dumpJSON(t, "smoke_random_evaluate_response.json", eval)

	// wait until completed (AIMock guarantees completion)
	final := waitForCompleted(t, client, eval["id"].(string), 60*time.Second)
	dumpJSON(t, "smoke_random_result_response.json", final)
	st, _ := final["status"].(string)
	require.Equal(t, "completed", st)
	res, ok := final["result"].(map[string]any)
	require.True(t, ok)
	if _, ok := res["cv_match_rate"]; !ok {
		t.Fatalf("cv_match_rate missing: %#v", res)
	}
	if _, ok := res["project_score"]; !ok {
		t.Fatalf("project_score missing: %#v", res)
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
