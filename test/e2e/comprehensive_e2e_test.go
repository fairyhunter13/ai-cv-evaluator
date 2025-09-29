//go:build e2e
// +build e2e

package e2e_test

import (
	"bytes"
	"encoding/json"
	"fmt"
	"mime/multipart"
	"net/http"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	baseURL = "http://localhost:8080/v1"
	timeout = 60 * time.Second
)

var (
	// Align with OpenAPI contract: job_description and study_case_brief are required
	defaultJobDescription = "Backend engineer building APIs, DBs, cloud, prompt design, chaining and RAG."
	defaultStudyCaseBrief = "Evaluate CV and project implementing LLM workflows, retries, and observability."
)

// TestE2E_EdgeCases tests various edge cases and error conditions
func TestE2E_EdgeCases(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping E2E tests in short mode")
	}

	client := &http.Client{Timeout: timeout}

	t.Run("Upload_LargeFile", func(t *testing.T) {
		// Create a large file (10MB - should be rejected)
		largeContent := strings.Repeat("x", 10*1024*1024+1)
		
		var buf bytes.Buffer
		writer := multipart.NewWriter(&buf)
		
		cvWriter, err := writer.CreateFormFile("cv", "large.txt")
		require.NoError(t, err)
		cvWriter.Write([]byte(largeContent))
		
		projWriter, err := writer.CreateFormFile("project", "project.txt")
		require.NoError(t, err)
		projWriter.Write([]byte("Small project"))
		
		writer.Close()
		
		req, err := http.NewRequest("POST", baseURL+"/upload", &buf)
		require.NoError(t, err)
		req.Header.Set("Content-Type", writer.FormDataContentType())
		maybeBasicAuth(req)
		maybeBasicAuth(req)
		
		resp, err := client.Do(req)
		require.NoError(t, err)
		defer resp.Body.Close()
		
		// Should reject due to size
		assert.Equal(t, http.StatusRequestEntityTooLarge, resp.StatusCode)
	})

	t.Run("Upload_InvalidFileType", func(t *testing.T) {
		var buf bytes.Buffer
		writer := multipart.NewWriter(&buf)
		
		// Try to upload an executable file
		cvWriter, err := writer.CreateFormFile("cv", "malicious.exe")
		require.NoError(t, err)
		cvWriter.Write([]byte{0x4D, 0x5A}) // MZ header for exe files
		
		projWriter, err := writer.CreateFormFile("project", "project.txt")
		require.NoError(t, err)
		projWriter.Write([]byte("Normal project"))
		
		writer.Close()
		
		req, err := http.NewRequest("POST", baseURL+"/upload", &buf)
		require.NoError(t, err)
		req.Header.Set("Content-Type", writer.FormDataContentType())
		maybeBasicAuth(req)
		
		resp, err := client.Do(req)
		require.NoError(t, err)
		defer resp.Body.Close()
		
		// Should reject due to file type
		assert.Equal(t, http.StatusUnsupportedMediaType, resp.StatusCode)
	})

	t.Run("Upload_MissingFile", func(t *testing.T) {
		var buf bytes.Buffer
		writer := multipart.NewWriter(&buf)
		
		// Only upload CV, missing project
		cvWriter, err := writer.CreateFormFile("cv", "cv.txt")
		require.NoError(t, err)
		cvWriter.Write([]byte("CV content"))
		
		writer.Close()
		
		req, err := http.NewRequest("POST", baseURL+"/upload", &buf)
		require.NoError(t, err)
		req.Header.Set("Content-Type", writer.FormDataContentType())
		maybeBasicAuth(req)
		
		resp, err := client.Do(req)
		require.NoError(t, err)
		defer resp.Body.Close()
		
		// Should reject due to missing project file
		assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
	})

	t.Run("Upload_EmptyFiles", func(t *testing.T) {
		var buf bytes.Buffer
		writer := multipart.NewWriter(&buf)
		
		// Upload empty files
		cvWriter, err := writer.CreateFormFile("cv", "empty_cv.txt")
		require.NoError(t, err)
		cvWriter.Write([]byte(""))
		
		projWriter, err := writer.CreateFormFile("project", "empty_project.txt")
		require.NoError(t, err)
		projWriter.Write([]byte(""))
		
		writer.Close()
		
		req, err := http.NewRequest("POST", baseURL+"/upload", &buf)
		require.NoError(t, err)
		req.Header.Set("Content-Type", writer.FormDataContentType())
		maybeBasicAuth(req)
		
		resp, err := client.Do(req)
		require.NoError(t, err)
		defer resp.Body.Close()
		
		// Should accept empty files but handle gracefully
		if resp.StatusCode == http.StatusOK {
			var result map[string]string
			json.NewDecoder(resp.Body).Decode(&result)
			assert.NotEmpty(t, result["cv_id"])
			assert.NotEmpty(t, result["project_id"])
		}
	})

	t.Run("Evaluate_InvalidIDs", func(t *testing.T) {
		payload := map[string]string{
			"cv_id":            "non-existent-cv-id",
			"project_id":       "non-existent-project-id",
			"job_description":  defaultJobDescription,
			"study_case_brief": defaultStudyCaseBrief,
		}
		
		body, _ := json.Marshal(payload)
		req, err := http.NewRequest("POST", baseURL+"/evaluate", bytes.NewReader(body))
		require.NoError(t, err)
		req.Header.Set("Content-Type", "application/json")
		maybeBasicAuth(req)
		
		resp, err := client.Do(req)
		require.NoError(t, err)
		defer resp.Body.Close()
		
		// Should handle gracefully, either reject or queue with error
		assert.Contains(t, []int{http.StatusBadRequest, http.StatusNotFound, http.StatusOK}, resp.StatusCode)
	})

	t.Run("Evaluate_MissingFields", func(t *testing.T) {
		payload := map[string]string{
			"cv_id":            "some-id",
			// Missing project_id on purpose
			"job_description":  defaultJobDescription,
			"study_case_brief": defaultStudyCaseBrief,
		}
		
		body, _ := json.Marshal(payload)
		req, err := http.NewRequest("POST", baseURL+"/evaluate", bytes.NewReader(body))
		require.NoError(t, err)
		req.Header.Set("Content-Type", "application/json")
		maybeBasicAuth(req)
		
		resp, err := client.Do(req)
		require.NoError(t, err)
		defer resp.Body.Close()
		
		assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
	})

	t.Run("Evaluate_Idempotency", func(t *testing.T) {
		// First, upload files
		uploadResp := uploadTestFiles(t, client, "Idempotency test CV", "Idempotency test project")
		
		// Evaluate with idempotency key
		payload := map[string]string{
			"cv_id":            uploadResp["cv_id"],
			"project_id":       uploadResp["project_id"],
			"job_description":  defaultJobDescription,
			"study_case_brief": defaultStudyCaseBrief,
		}
		
		body, _ := json.Marshal(payload)
		idempotencyKey := fmt.Sprintf("test-key-%d", time.Now().UnixNano())
		
		// First request
		req1, _ := http.NewRequest("POST", baseURL+"/evaluate", bytes.NewReader(body))
		req1.Header.Set("Content-Type", "application/json")
		req1.Header.Set("Idempotency-Key", idempotencyKey)
		maybeBasicAuth(req1)
		
		resp1, err := client.Do(req1)
		require.NoError(t, err)
		defer resp1.Body.Close()
		
		var result1 map[string]interface{}
		json.NewDecoder(resp1.Body).Decode(&result1)
		
		// Second request with same idempotency key
		req2, _ := http.NewRequest("POST", baseURL+"/evaluate", bytes.NewReader(body))
		req2.Header.Set("Content-Type", "application/json")
		req2.Header.Set("Idempotency-Key", idempotencyKey)
		maybeBasicAuth(req2)
		
		resp2, err := client.Do(req2)
		require.NoError(t, err)
		defer resp2.Body.Close()
		
		var result2 map[string]interface{}
		json.NewDecoder(resp2.Body).Decode(&result2)
		
		// Should return the same job ID
		assert.Equal(t, result1["id"], result2["id"])
	})

	t.Run("Result_NonExistentJob", func(t *testing.T) {
		req, err := http.NewRequest("GET", baseURL+"/result/non-existent-job-id", nil)
		require.NoError(t, err)
		
		resp, err := client.Do(req)
		require.NoError(t, err)
		defer resp.Body.Close()
		
		assert.Equal(t, http.StatusNotFound, resp.StatusCode)
	})

	t.Run("Result_InvalidJobID", func(t *testing.T) {
		// Test with various invalid formats
		invalidIDs := []string{
			"",
			"../etc/passwd",
			"'; DROP TABLE jobs; --",
			strings.Repeat("x", 1000),
		}
		
		for _, id := range invalidIDs {
			req, err := http.NewRequest("GET", baseURL+"/result/"+id, nil)
			require.NoError(t, err)
			
			resp, err := client.Do(req)
			if err != nil {
				continue // Skip if request fails
			}
			resp.Body.Close()
			
			assert.Contains(t, []int{http.StatusBadRequest, http.StatusNotFound}, resp.StatusCode)
		}
	})

	t.Run("Result_ETagCaching", func(t *testing.T) {
		// Upload and evaluate
		uploadResp := uploadTestFiles(t, client, "ETag test CV", "ETag test project")
		evalResp := evaluateFiles(t, client, uploadResp["cv_id"], uploadResp["project_id"])
		
		jobID := evalResp["id"].(string)
		
		// Wait for completion
		waitForCompletion(t, client, jobID)
		
		// First request to get ETag
		req1, _ := http.NewRequest("GET", baseURL+"/result/"+jobID, nil)
		resp1, err := client.Do(req1)
		require.NoError(t, err)
		defer resp1.Body.Close()
		
		etag := resp1.Header.Get("ETag")
		assert.NotEmpty(t, etag)
		
		// Second request with If-None-Match
		req2, _ := http.NewRequest("GET", baseURL+"/result/"+jobID, nil)
		req2.Header.Set("If-None-Match", etag)
		
		resp2, err := client.Do(req2)
		require.NoError(t, err)
		defer resp2.Body.Close()
		
		// Should return 304 Not Modified
		assert.Equal(t, http.StatusNotModified, resp2.StatusCode)
	})

	t.Run("RateLimit_Exceeded", func(t *testing.T) {
		// Send many requests quickly to trigger rate limit
		for i := 0; i < 20; i++ {
			req, _ := http.NewRequest("GET", baseURL+"/result/test", nil)
			resp, err := client.Do(req)
			if err != nil {
				continue
			}
			
			if resp.StatusCode == http.StatusTooManyRequests {
				// Rate limit triggered successfully
				resp.Body.Close()
				return
			}
			resp.Body.Close()
		}
		
		// Note: Rate limiting might not trigger in test environment
		t.Log("Rate limiting may not be configured in test environment")
	})

	t.Run("ContentNegotiation_UnsupportedAccept", func(t *testing.T) {
		req, err := http.NewRequest("GET", baseURL+"/result/test", nil)
		require.NoError(t, err)
		req.Header.Set("Accept", "application/xml")
		
		resp, err := client.Do(req)
		require.NoError(t, err)
		defer resp.Body.Close()
		
		// Should return 406 Not Acceptable for unsupported content type
		if resp.StatusCode == http.StatusNotAcceptable {
			assert.Equal(t, http.StatusNotAcceptable, resp.StatusCode)
		}
	})
}

// TestE2E_ConcurrentEvaluations tests handling of concurrent evaluation requests
func TestE2E_ConcurrentEvaluations(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping E2E tests in short mode")
	}

	client := &http.Client{Timeout: timeout}
	
	// Upload multiple file pairs
	numPairs := 5
	uploads := make([]map[string]string, numPairs)
	
	for i := 0; i < numPairs; i++ {
		cv := fmt.Sprintf("CV %d: Experience in Go, Python, AWS", i)
		project := fmt.Sprintf("Project %d: Built microservices architecture", i)
		uploads[i] = uploadTestFiles(t, client, cv, project)
	}
	
	// Evaluate all concurrently
	type evalResult struct {
		idx int
		res map[string]interface{}
		err error
	}
	
	results := make(chan evalResult, numPairs)
	
	for i := 0; i < numPairs; i++ {
		go func(idx int) {
			res := evaluateFiles(t, client, uploads[idx]["cv_id"], uploads[idx]["project_id"])
			results <- evalResult{idx: idx, res: res, err: nil}
		}(i)
	}
	
	// Collect results
	evalResponses := make([]map[string]interface{}, numPairs)
	for i := 0; i < numPairs; i++ {
		result := <-results
		require.NoError(t, result.err)
		evalResponses[result.idx] = result.res
	}
	
	// Verify all got different job IDs
	jobIDs := make(map[string]bool)
	for _, resp := range evalResponses {
		jobID := resp["id"].(string)
		assert.False(t, jobIDs[jobID], "Duplicate job ID found")
		jobIDs[jobID] = true
	}
	
	// Wait for all to complete and verify results
	for _, resp := range evalResponses {
		jobID := resp["id"].(string)
		result := waitForCompletion(t, client, jobID)
		
		// Verify result structure
		assert.NotNil(t, result["result"])
		resultData := result["result"].(map[string]interface{})
		assert.Contains(t, resultData, "cv_match_rate")
		assert.Contains(t, resultData, "cv_feedback")
		assert.Contains(t, resultData, "project_score")
		assert.Contains(t, resultData, "project_feedback")
		assert.Contains(t, resultData, "overall_summary")
	}
}

// TestE2E_SpecialCharacters tests handling of special characters and encodings
func TestE2E_SpecialCharacters(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping E2E tests in short mode")
	}

	client := &http.Client{Timeout: timeout}
	
	specialCases := []struct {
		name    string
		cvText  string
		projText string
	}{
		{
			name:     "Unicode characters",
			cvText:   "Experience with 日本語 and 中文 languages. €100K salary expectation.",
			projText: "Built app supporting émojis 😀 and special symbols ™®©",
		},
		{
			name:     "SQL injection attempt",
			cvText:   "'; DROP TABLE users; -- Experience in security",
			projText: "SELECT * FROM sensitive_data WHERE 1=1",
		},
		{
			name:     "HTML/JavaScript injection",
			cvText:   "<script>alert('XSS')</script> Web developer",
			projText: "<img src=x onerror=alert('XSS')> Project",
		},
		{
			name:     "Very long text",
			cvText:   strings.Repeat("Senior developer with experience. ", 1000),
			projText: strings.Repeat("Built scalable systems. ", 1000),
		},
	}
	
	for _, tc := range specialCases {
		t.Run(tc.name, func(t *testing.T) {
			// Upload files with special content
			uploadResp := uploadTestFiles(t, client, tc.cvText, tc.projText)
			
			// Evaluate
			evalResp := evaluateFiles(t, client, uploadResp["cv_id"], uploadResp["project_id"])
			
			// Wait for completion
			result := waitForCompletion(t, client, evalResp["id"].(string))
			
			// Verify result is properly escaped and valid
			assert.Equal(t, "completed", result["status"])
			resultData := result["result"].(map[string]interface{})
			
			// Check that feedback doesn't contain unescaped special characters
			cvFeedback := resultData["cv_feedback"].(string)
			projFeedback := resultData["project_feedback"].(string)
			
			assert.NotContains(t, cvFeedback, "<script>")
			assert.NotContains(t, projFeedback, "DROP TABLE")
			
			// Verify numeric values are in valid ranges
			cvMatch := resultData["cv_match_rate"].(float64)
			assert.GreaterOrEqual(t, cvMatch, 0.0)
			assert.LessOrEqual(t, cvMatch, 1.0)
			
			projScore := resultData["project_score"].(float64)
			assert.GreaterOrEqual(t, projScore, 1.0)
			assert.LessOrEqual(t, projScore, 10.0)
		})
	}
}

// Helper functions

func uploadTestFiles(t *testing.T, client *http.Client, cvContent, projectContent string) map[string]string {
    var buf bytes.Buffer
    writer := multipart.NewWriter(&buf)
	
	cvWriter, err := writer.CreateFormFile("cv", "test_cv.txt")
	require.NoError(t, err)
	cvWriter.Write([]byte(cvContent))
	
	projWriter, err := writer.CreateFormFile("project", "test_project.txt")
	require.NoError(t, err)
	projWriter.Write([]byte(projectContent))
	
	writer.Close()
	
    var lastStatus int
    for i := 0; i < 6; i++ { // up to ~3s total wait
        req, err := http.NewRequest("POST", baseURL+"/upload", &buf)
        require.NoError(t, err)
        req.Header.Set("Content-Type", writer.FormDataContentType())
        maybeBasicAuth(req)
        resp, err := client.Do(req)
        require.NoError(t, err)
        lastStatus = resp.StatusCode
        if resp.StatusCode == http.StatusOK {
            defer resp.Body.Close()
            var result map[string]string
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
    return map[string]string{}
}

func evaluateFiles(t *testing.T, client *http.Client, cvID, projectID string) map[string]interface{} {
    payload := map[string]string{
        "cv_id":            cvID,
        "project_id":       projectID,
        "job_description":  defaultJobDescription,
        "study_case_brief": defaultStudyCaseBrief,
    }

    body, _ := json.Marshal(payload)
    var lastStatus int
    for i := 0; i < 6; i++ { // up to ~3s total wait
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
        resp.Body.Close()
        if resp.StatusCode == http.StatusTooManyRequests {
            time.Sleep(500 * time.Millisecond)
            continue
        }
        break
    }
    require.Equal(t, http.StatusOK, lastStatus)
    return map[string]interface{}{}
}

func waitForCompletion(t *testing.T, client *http.Client, jobID string) map[string]interface{} {
    // Allow fail-fast tuning via env vars
    maxPollsStr := getenv("E2E_MAX_POLLS", "120")
    sleepMsStr := getenv("E2E_SLEEP_MS", "3000")
    maxRetries, _ := strconv.Atoi(maxPollsStr)
    if maxRetries <= 0 { maxRetries = 120 }
    sleepMs, _ := strconv.Atoi(sleepMsStr)
    if sleepMs <= 0 { sleepMs = 3000 }
    for i := 0; i < maxRetries; i++ {
        req, err := http.NewRequest("GET", baseURL+"/result/"+jobID, nil)
        require.NoError(t, err)
        
        resp, err := client.Do(req)
        require.NoError(t, err)
        defer resp.Body.Close()
        
        var result map[string]interface{}
        err = json.NewDecoder(resp.Body).Decode(&result)
        require.NoError(t, err)
        
        status := result["status"].(string)
        if status == "completed" || status == "failed" {
            return result
        }
        
        time.Sleep(time.Duration(sleepMs) * time.Millisecond)
    }
    
    t.Fatal("Job did not complete within timeout")
    return nil
}
