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

// evaluateFilesWithDefaults enqueues evaluation using only required fields (optional fields use defaults).
func evaluateFilesWithDefaults(t *testing.T, client *http.Client, cvID, projectID string) map[string]interface{} {
	payload := map[string]string{
		"cv_id":      cvID,
		"project_id": projectID,
		// job_description and study_case_brief are omitted - should use defaults
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
			var result map[string]interface{}
			err = json.NewDecoder(resp.Body).Decode(&result)
			require.NoError(t, err)
			return result
		}

		// Capture error response for detailed logging
		if resp.StatusCode != http.StatusOK {
			bodyBytes, _ := json.Marshal(map[string]interface{}{
				"status_code": resp.StatusCode,
				"status":      resp.Status,
				"headers":     resp.Header,
			})
			lastErrorResponse = string(bodyBytes)

			// Try to read response body for error details
			if resp.Body != nil {
				// Try to decode error response
				var errorResp map[string]interface{}
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
	t.Logf("=== EVALUATE API FAILURE DETAILS (With Defaults) ===")
	t.Logf("Final Status Code: %d", lastStatus)
	t.Logf("Error Response: %s", lastErrorResponse)
	t.Logf("Request Payload: %s", string(body))
	t.Logf("Base URL: %s", baseURL)
	t.Logf("==================================================")

	require.Equal(t, http.StatusOK, lastStatus)
	return map[string]interface{}{}
}
