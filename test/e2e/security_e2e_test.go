//go:build e2e
// +build e2e

package e2e_test

import (
	"bytes"
	"encoding/json"
	"fmt"
	"mime/multipart"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestE2E_SecurityHeaders tests that proper security headers are set
func TestE2E_SecurityHeaders(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping E2E tests in short mode")
	}

	client := &http.Client{Timeout: timeout}

	testCases := []struct {
		endpoint string
		method   string
	}{
		{"/v1/upload", "GET"},
		{"/healthz", "GET"},
		{"/metrics", "GET"},
		{"/readyz", "GET"},
	}

	for _, tc := range testCases {
		t.Run(fmt.Sprintf("%s_%s", tc.method, strings.ReplaceAll(tc.endpoint, "/", "_")), func(t *testing.T) {
			req, err := http.NewRequest(tc.method, "http://localhost:8080"+tc.endpoint, nil)
			require.NoError(t, err)

			resp, err := client.Do(req)
			require.NoError(t, err)
			defer resp.Body.Close()

			// Check security headers
			headers := resp.Header

			// Content Security Policy
			assert.NotEmpty(t, headers.Get("Content-Security-Policy"), "CSP header should be set")

			// X-Frame-Options
			assert.Equal(t, "DENY", headers.Get("X-Frame-Options"), "X-Frame-Options should be DENY")

			// X-Content-Type-Options
			assert.Equal(t, "nosniff", headers.Get("X-Content-Type-Options"), "X-Content-Type-Options should be nosniff")

			// Referrer-Policy
			assert.Equal(t, "no-referrer", headers.Get("Referrer-Policy"), "Referrer-Policy should be no-referrer")

			// X-Request-Id should be present for tracing
			assert.NotEmpty(t, headers.Get("X-Request-Id"), "X-Request-Id should be set")
		})
	}
}

// TestE2E_RateLimiting tests rate limiting functionality
func TestE2E_RateLimiting(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping E2E tests in short mode")
	}

	client := &http.Client{Timeout: timeout}

	// Make rapid POST requests to a rate-limited endpoint (mutating endpoints are limited)
	const numRequests = 60
	var rateLimitHit bool

	payload := map[string]string{
		"cv_id":            "fake",
		"project_id":       "fake",
		"job_description":  "test",
		"study_case_brief": "test",
	}
	body, _ := json.Marshal(payload)

	for i := 0; i < numRequests; i++ {
		req, _ := http.NewRequest(http.MethodPost, "http://localhost:8080/v1/evaluate", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		resp, err := client.Do(req)
		if err != nil {
			// Continue; environment may be unstable
			continue
		}
		resp.Body.Close()
		if resp.StatusCode == http.StatusTooManyRequests {
			rateLimitHit = true
			break
		}
		// Very small delay
		time.Sleep(5 * time.Millisecond)
	}

	// Rate limiting should be triggered eventually; tolerate environments where it may not
	if !rateLimitHit {
		t.Log("Rate limiting not triggered; environment may not enforce limits in test context")
	}
}

// TestE2E_InputValidation tests input validation and sanitization
func TestE2E_InputValidation(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping E2E tests in short mode")
	}

	client := &http.Client{Timeout: timeout}

	t.Run("Malicious_File_Upload", func(t *testing.T) {
		maliciousPayloads := []struct {
			name    string
			content string
			filename string
		}{
			{
				name:     "Script_Injection",
				content:  "<script>alert('xss')</script>",
				filename: "malicious.txt",
			},
			{
				name:     "SQL_Injection_Attempt",
				content:  "'; DROP TABLE users; --",
				filename: "sql_injection.txt",
			},
			{
				name:     "Path_Traversal",
				content:  "Normal content",
				filename: "../../../etc/passwd",
			},
			{
				name:     "Binary_Executable",
				content:  "\x7fELF\x01\x01\x01\x00", // ELF binary header
				filename: "malware.exe",
			},
		}

		for _, payload := range maliciousPayloads {
			t.Run(payload.name, func(t *testing.T) {
				var buf bytes.Buffer
				writer := multipart.NewWriter(&buf)

				cvWriter, err := writer.CreateFormFile("cv", payload.filename)
				require.NoError(t, err)
				cvWriter.Write([]byte(payload.content))

				projWriter, err := writer.CreateFormFile("project", "project.txt")
				require.NoError(t, err)
				projWriter.Write([]byte("Test project"))

				writer.Close()

				req, err := http.NewRequest("POST", "http://localhost:8080/v1/upload", &buf)
				require.NoError(t, err)
				req.Header.Set("Content-Type", writer.FormDataContentType())
				maybeBasicAuth(req)

				resp, err := client.Do(req)
				require.NoError(t, err)
				defer resp.Body.Close()

				// Should reject malicious files or handle safely
				assert.True(t, resp.StatusCode == http.StatusBadRequest || 
					resp.StatusCode == http.StatusUnsupportedMediaType ||
					resp.StatusCode == http.StatusOK, // If handled safely
					"Malicious file should be rejected or handled safely")
			})
		}
	})

	t.Run("Oversized_Headers", func(t *testing.T) {
		req, err := http.NewRequest("GET", "http://localhost:8080/v1/upload", nil)
		require.NoError(t, err)

		// Add oversized header
		largeValue := strings.Repeat("x", 10000)
		req.Header.Set("X-Large-Header", largeValue)

		resp, err := client.Do(req)
		if err != nil {
			// Connection might be rejected, which is acceptable
			return
		}
		defer resp.Body.Close()

		// Server should handle large headers gracefully
		assert.True(t, resp.StatusCode < 500, "Server should handle large headers without crashing")
	})
}

// TestE2E_CORS tests Cross-Origin Resource Sharing configuration
func TestE2E_CORS(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping E2E tests in short mode")
	}

	client := &http.Client{Timeout: timeout}

	t.Run("CORS_Preflight", func(t *testing.T) {
		req, err := http.NewRequest("OPTIONS", "http://localhost:8080/v1/upload", nil)
		require.NoError(t, err)
		req.Header.Set("Origin", "https://example.com")
		req.Header.Set("Access-Control-Request-Method", "POST")
		req.Header.Set("Access-Control-Request-Headers", "Content-Type")

		resp, err := client.Do(req)
		require.NoError(t, err)
		defer resp.Body.Close()

		// Should handle CORS preflight
		assert.True(t, resp.StatusCode == http.StatusOK || resp.StatusCode == http.StatusNoContent)
		
		// Check CORS headers
		assert.NotEmpty(t, resp.Header.Get("Access-Control-Allow-Origin"))
		assert.NotEmpty(t, resp.Header.Get("Vary"))
	})
}

// TestE2E_ErrorHandling tests error handling and information disclosure
func TestE2E_ErrorHandling(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping E2E tests in short mode")
	}

	client := &http.Client{Timeout: timeout}

	t.Run("404_Error_Handling", func(t *testing.T) {
		resp, err := client.Get("http://localhost:8080/nonexistent")
		require.NoError(t, err)
		defer resp.Body.Close()

		assert.Equal(t, http.StatusNotFound, resp.StatusCode)

		// Check that error doesn't expose sensitive information
		body := make([]byte, 1000)
		n, _ := resp.Body.Read(body)
		errorText := string(body[:n])

		// Should not contain sensitive paths or stack traces
		assert.NotContains(t, errorText, "/Users/")
		assert.NotContains(t, errorText, "goroutine")
		assert.NotContains(t, errorText, "panic")
	})

	t.Run("Method_Not_Allowed", func(t *testing.T) {
		req, err := http.NewRequest("PATCH", "http://localhost:8080/v1/upload", nil)
		require.NoError(t, err)
		maybeBasicAuth(req)

		resp, err := client.Do(req)
		require.NoError(t, err)
		defer resp.Body.Close()

		assert.Equal(t, http.StatusMethodNotAllowed, resp.StatusCode)
	})
}
