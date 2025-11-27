//go:build e2e
// +build e2e

package e2e_test

import (
	"bytes"
	"encoding/json"
	"io"
	"mime/multipart"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

var (
	baseURL               string
	defaultJobDescription = "Backend Developer - APIs, DBs, cloud, prompt design, chaining and RAG."
	defaultStudyCaseBrief = "Mini Project: Evaluate CV and project via AI workflow with retries and observability."
	adminJWTOnce          sync.Once
)

func init() {
	// Allow overriding base URL for E2E via env to avoid port conflicts.
	baseURL = getenv("E2E_BASE_URL", "http://localhost:8080/v1")
}

// getenv returns the value of the environment variable k or def if empty.
func getenv(k, def string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return def
}

func waitForAppReady(t *testing.T, client *http.Client, timeout time.Duration) {
	t.Helper()
	deadline := time.Now().Add(timeout)
	healthz := strings.TrimSuffix(baseURL, "/v1") + "/healthz"
	var lastErr error
	for {
		resp, err := client.Get(healthz)
		if err == nil && resp != nil && resp.StatusCode == http.StatusOK {
			resp.Body.Close()
			return
		}
		if resp != nil {
			resp.Body.Close()
		}
		lastErr = err
		if time.Now().After(deadline) {
			t.Fatalf("App not available; healthz check failed after %v: %v", timeout, lastErr)
		}
		time.Sleep(1 * time.Second)
	}
}

// clearDumpDirectory removes all files from test/dump directory
func clearDumpDirectory(t *testing.T) {
	t.Helper()
	dir := filepath.FromSlash("../../test/dump")
	if err := os.RemoveAll(dir); err != nil {
		t.Logf("clearDumpDirectory error: %v", err)
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Logf("clearDumpDirectory mkdir error: %v", err)
	}
	t.Logf("cleared dump directory: %s", dir)
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

func ensureAdminJWT(t *testing.T, client *http.Client) {
	t.Helper()
	adminJWTOnce.Do(func() {
		if os.Getenv("ADMIN_JWT") != "" {
			return
		}
		username := os.Getenv("ADMIN_USERNAME")
		password := os.Getenv("ADMIN_PASSWORD")
		if username == "" || password == "" {
			t.Fatalf("ADMIN_USERNAME/ADMIN_PASSWORD must be set for E2E tests")
		}
		root := strings.TrimSuffix(baseURL, "/v1")
		form := url.Values{}
		form.Set("username", username)
		form.Set("password", password)
		req, err := http.NewRequest("POST", root+"/admin/token", strings.NewReader(form.Encode()))
		require.NoError(t, err)
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		resp, err := client.Do(req)
		require.NoError(t, err)
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			body, _ := io.ReadAll(resp.Body)
			t.Fatalf("admin token request failed: status=%d body=%s", resp.StatusCode, string(body))
		}
		var payload map[string]any
		require.NoError(t, json.NewDecoder(resp.Body).Decode(&payload))
		token, ok := payload["token"].(string)
		require.True(t, ok && token != "", "admin token missing in response")
		os.Setenv("ADMIN_JWT", token)
	})
}

// maybeBasicAuth sets Authorization for admin-protected endpoints.
// Priority: Bearer token from ADMIN_JWT, else HTTP Basic from ADMIN_USERNAME/PASSWORD.
func maybeBasicAuth(req *http.Request) {
	if tok := os.Getenv("ADMIN_JWT"); tok != "" {
		req.Header.Set("Authorization", "Bearer "+tok)
		return
	}
	// Basic auth removed; require JWT for admin endpoints
}

// waitForCompleted polls GET /v1/result/{id} until status becomes "completed" or the maxWait expires.
// It returns the last parsed JSON map and fails the test if request errors occur.
func waitForCompleted(t *testing.T, client *http.Client, jobID string, maxWait time.Duration) map[string]any {
	deadline := time.Now().Add(maxWait)
	var last map[string]any
	pollCount := 0

	// Give workers time to pick up the task - reduced from 2s to 1s for faster tests
	time.Sleep(1 * time.Second)

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

		// Log progress every 5 polls for better visibility
		if pollCount%5 == 0 {
			t.Logf("Job %s still processing... (poll %d, status: %s)", jobID, pollCount, status)
		}

		// Optimized adaptive polling strategy for better performance
		var pollInterval time.Duration
		switch {
		case pollCount <= 10:
			// Fast polling for initial attempts (100ms)
			pollInterval = 100 * time.Millisecond
		case pollCount <= 30:
			// Moderate polling for middle attempts (300ms)
			pollInterval = 300 * time.Millisecond
		case pollCount <= 60:
			// Slower polling for longer waits (500ms)
			pollInterval = 500 * time.Millisecond
		default:
			// Slow polling for very long waits (1s)
			pollInterval = 1 * time.Second
		}
		time.Sleep(pollInterval)
	}
}

// uploadTestFiles uploads provided CV and project contents and returns ids.
func uploadTestFiles(t *testing.T, client *http.Client, cvContent, projectContent string) map[string]any {
	t.Helper()
	ensureAdminJWT(t, client)
	var buf bytes.Buffer
	writer := multipart.NewWriter(&buf)

	cvWriter, err := writer.CreateFormFile("cv", "test_cv.txt")
	require.NoError(t, err)
	_, _ = cvWriter.Write([]byte(cvContent))

	projWriter, err := writer.CreateFormFile("project", "test_project.txt")
	require.NoError(t, err)
	_, _ = projWriter.Write([]byte(projectContent))

	_ = writer.Close()

	// quick retry loop (<= ~3s)
	var lastStatus int
	for i := 0; i < 6; i++ {
		req, err := http.NewRequest("POST", baseURL+"/upload", &buf)
		require.NoError(t, err)
		req.Header.Set("Content-Type", writer.FormDataContentType())
		maybeBasicAuth(req)
		resp, err := client.Do(req)
		require.NoError(t, err)
		lastStatus = resp.StatusCode
		if resp.StatusCode == http.StatusOK {
			defer resp.Body.Close()
			var result map[string]any
			err = json.NewDecoder(resp.Body).Decode(&result)
			require.NoError(t, err)
			return result
		}
		resp.Body.Close()
		if resp.StatusCode == http.StatusTooManyRequests {
			time.Sleep(500 * time.Millisecond)
			continue
		}
		break
	}
	require.Equal(t, http.StatusOK, lastStatus)
	return map[string]any{}
}

// evaluateFiles enqueues evaluation and returns job response body.
func evaluateFiles(t *testing.T, client *http.Client, cvID, projectID string) map[string]any {
	t.Helper()
	ensureAdminJWT(t, client)
	payload := map[string]string{
		"cv_id":            cvID,
		"project_id":       projectID,
		"job_description":  defaultJobDescription,
		"study_case_brief": defaultStudyCaseBrief,
	}

	body, _ := json.Marshal(payload)
	var lastStatus int
	var lastErrorResponse string
	for i := 0; i < 6; i++ {
		req, err := http.NewRequest("POST", baseURL+"/evaluate", bytes.NewReader(body))
		require.NoError(t, err)
		req.Header.Set("Content-Type", "application/json")
		maybeBasicAuth(req)
		resp, err := client.Do(req)
		require.NoError(t, err)
		lastStatus = resp.StatusCode
		if resp.StatusCode == http.StatusOK {
			defer resp.Body.Close()
			var result map[string]any
			err = json.NewDecoder(resp.Body).Decode(&result)
			require.NoError(t, err)
			return result
		}

		// Capture error response for detailed logging
		if resp.StatusCode != http.StatusOK {
			bodyBytes, _ := json.Marshal(map[string]any{
				"status_code": resp.StatusCode,
				"status":      resp.Status,
				"headers":     resp.Header,
			})
			lastErrorResponse = string(bodyBytes)

			// Try to read response body for error details
			if resp.Body != nil {
				// Try to decode error response
				var errorResp map[string]any
				if json.NewDecoder(resp.Body).Decode(&errorResp) == nil {
					errorBytes, _ := json.Marshal(errorResp)
					lastErrorResponse = string(errorBytes)
				}
			}

			t.Logf("Evaluate API Error (attempt %d): Status %d - %s", i+1, resp.StatusCode, resp.Status)
			t.Logf("Error Response: %s", lastErrorResponse)
		}

		resp.Body.Close()
		if resp.StatusCode == http.StatusTooManyRequests {
			time.Sleep(500 * time.Millisecond)
			continue
		}
		break
	}

	// Enhanced error logging before failing
	t.Logf("=== EVALUATE API FAILURE DETAILS ===")
	t.Logf("Final Status Code: %d", lastStatus)
	t.Logf("Error Response: %s", lastErrorResponse)
	t.Logf("Request Payload: %s", string(body))
	t.Logf("Base URL: %s", baseURL)
	t.Logf("=====================================")

	require.Equal(t, http.StatusOK, lastStatus)
	return map[string]any{}
}
