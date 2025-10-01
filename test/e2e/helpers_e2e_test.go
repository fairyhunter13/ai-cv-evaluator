//go:build e2e

package e2e_test

import (
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"testing"
	"time"
)

// getenv returns the value of the environment variable k or def if empty.
func getenv(k, def string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return def
}

// dumpJSON writes a JSON file under test/dump with a timestamped filename.
func dumpJSON(t *testing.T, filename string, v any) {
    t.Helper()
    // Write to repo-root test/dump (go up 2 levels from test/e2e)
    dir := filepath.FromSlash("../../test/dump")
    _ = os.MkdirAll(dir, 0o755)
    ts := time.Now().Format("20060102_150405")
    p := filepath.Join(dir, ts+"_"+filename)
    f, err := os.Create(p)
    if err != nil {
        t.Logf("dumpJSON create error: %v", err)
        return
    }
    defer f.Close()
    enc := json.NewEncoder(f)
    enc.SetIndent("", "  ")
    if err := enc.Encode(v); err != nil {
        t.Logf("dumpJSON encode error: %v", err)
        return
    }
    t.Logf("dumped JSON to %s", p)
}

// maybeBasicAuth sets HTTP Basic Auth on the request if ADMIN_USERNAME and ADMIN_PASSWORD are present.
func maybeBasicAuth(req *http.Request) {
	u := os.Getenv("ADMIN_USERNAME")
	p := os.Getenv("ADMIN_PASSWORD")
	// Fallback to defaults typically used in dev if not provided
	if u == "" {
		u = "admin"
	}
	if p == "" {
		p = "admin123"
	}
	req.SetBasicAuth(u, p)
}

// waitForCompleted polls GET /v1/result/{id} until status becomes "completed" or the maxWait expires.
// It returns the last parsed JSON map and fails the test if request errors occur.
func waitForCompleted(t *testing.T, client *http.Client, jobID string, maxWait time.Duration) map[string]any {
	deadline := time.Now().Add(maxWait)
	var last map[string]any
	pollCount := 0

	// Give workers time to pick up the task
	time.Sleep(2 * time.Second)

	for {
		pollCount++
		req, _ := http.NewRequest("GET", baseURL+"/result/"+jobID, nil)
		resp, err := client.Do(req)
		if err != nil {
			t.Fatalf("GET /result error: %v", err)
		}
		if resp.StatusCode != 200 {
			t.Fatalf("GET /result returned status %d", resp.StatusCode)
		}
		_ = json.NewDecoder(resp.Body).Decode(&last)
		resp.Body.Close()

		status, _ := last["status"].(string)
		if status == "completed" || status == "failed" {
			t.Logf("Job %s completed after %d polls (status: %s)", jobID, pollCount, status)
			return last
		}

		if time.Now().After(deadline) {
			t.Logf("Job %s timed out after %d polls (status: %s)", jobID, pollCount, status)
			return last
		}

		// Log progress every 10 polls
		if pollCount%10 == 0 {
			t.Logf("Job %s still processing... (poll %d, status: %s)", jobID, pollCount, status)
		}

		time.Sleep(1 * time.Second)
	}
}
