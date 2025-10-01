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

// TestE2E_ComprehensiveVariations runs multiple happy path variations in parallel
func TestE2E_ComprehensiveVariations(t *testing.T) {
	t.Parallel()

	// Define test variations (reduced set for better performance)
	variations := []struct {
		name     string
		cvText   string
		projText string
	}{
		{"backend_engineer", "Senior Backend Engineer with Go, PostgreSQL, Redis, Docker, Kubernetes", "Microservices architecture with event-driven design"},
		{"frontend_developer", "Frontend Developer with React, TypeScript, Vue, Angular, Node.js", "Modern web application with responsive design"},
		{"devops_engineer", "DevOps Engineer with AWS, GCP, Azure, Terraform, Ansible", "Infrastructure automation and monitoring"},
		{"data_scientist", "Data Scientist with Python, R, TensorFlow, PyTorch, Pandas", "Machine learning model development and deployment"},
		{"ai_engineer", "AI Engineer with LLM, RAG, Prompt Engineering, LangChain", "AI workflow automation and optimization"},
	}

	// Run all variations in parallel
	for _, variation := range variations {
		variation := variation // capture loop variable
		t.Run(variation.name, func(t *testing.T) {
			t.Parallel()
			runVariationTest(t, variation.name, variation.cvText, variation.projText)
		})
	}
}

// TestE2E_EdgeCaseVariations runs edge case variations in parallel
func TestE2E_EdgeCaseVariations(t *testing.T) {
	t.Parallel()

	// Define edge case variations
	edgeCases := []struct {
		name     string
		cvText   string
		projText string
	}{
		{"empty_cv", "", "Project description"},
		{"empty_project", "CV description", ""},
		{"both_empty", "", ""},
		{"minimal_cv", "Go Developer", "API Service"},
		{"minimal_project", "Senior Engineer", "Web App"},
		{"special_chars", "Engineer 🚀 with @#$%^&*() skills", "Project with unicode: 中文, العربية"},
		{"json_format", `{"name": "John", "skills": ["Go", "Python"]}`, `{"title": "API", "tech": ["Go", "gRPC"]}`},
	}

	// Run all edge cases in parallel
	for _, edgeCase := range edgeCases {
		edgeCase := edgeCase // capture loop variable
		t.Run(edgeCase.name, func(t *testing.T) {
			t.Parallel()
			runVariationTest(t, edgeCase.name, edgeCase.cvText, edgeCase.projText)
		})
	}
}

// runVariationTest runs a single variation test
func runVariationTest(t *testing.T, testName, cvText, projectText string) {
	// Clear dump directory before test
	clearDumpDirectory(t)

	httpTimeout := 2 * time.Second
	if testing.Short() {
		httpTimeout = 1 * time.Second
	}
	client := &http.Client{Timeout: httpTimeout}

	// Ensure app is reachable; fail test if not ready
	healthz := strings.TrimSuffix(baseURL, "/v1") + "/healthz"
	if resp, err := client.Get(healthz); err != nil || (resp != nil && resp.StatusCode != http.StatusOK) {
		if resp != nil {
			resp.Body.Close()
		}
		t.Fatalf("App not available; healthz check failed: %v", err)
	} else if resp != nil {
		resp.Body.Close()
	}

	// 1) Upload CV and Project texts
	uploadResp := uploadTestFiles(t, client, cvText, projectText)
	dumpJSON(t, testName+"_upload_response.json", uploadResp)

	// 2) Enqueue Evaluate
	cvID, ok := uploadResp["cv_id"].(string)
	require.True(t, ok, "cv_id should be a string")
	projectID, ok := uploadResp["project_id"].(string)
	require.True(t, ok, "project_id should be a string")
	evalResp := evaluateFiles(t, client, cvID, projectID)
	dumpJSON(t, testName+"_evaluate_response.json", evalResp)
	jobID, ok := evalResp["id"].(string)
	require.True(t, ok && jobID != "", "evaluate should return job id")

	// 3) Wait for completion
	final := waitForCompleted(t, client, jobID, 300*time.Second)
	dumpJSON(t, testName+"_result_response.json", final)
	st, _ := final["status"].(string)

	// CRITICAL: E2E tests must only accept successful completions
	require.NotEqual(t, "queued", st, "E2E test failed: job stuck in queued state - %#v", final)
	require.NotEqual(t, "processing", st, "E2E test failed: job stuck in processing state - %#v", final)
	require.Equal(t, "completed", st, "E2E test failed: job did not complete successfully. Status: %v, Response: %#v", st, final)

	// Validate result structure
	res, ok := final["result"].(map[string]any)
	require.True(t, ok, "result object missing")
	_, hasCV := res["cv_match_rate"]
	_, hasCVF := res["cv_feedback"]
	_, hasProj := res["project_score"]
	_, hasProjF := res["project_feedback"]
	_, hasSummary := res["overall_summary"]
	assert.True(t, hasCV && hasCVF && hasProj && hasProjF && hasSummary, "incomplete result payload: %#v", res)

	// Test Summary
	t.Logf("=== %s Variation Test Summary ===", strings.ToUpper(testName))
	t.Logf("Test Status: %s", st)
	t.Logf("✅ Test PASSED - %s variation completed successfully", testName)
	t.Logf("Result contains: cv_match_rate=%v, project_score=%v",
		res["cv_match_rate"] != nil, res["project_score"] != nil)

	if b, err := json.MarshalIndent(final, "", "  "); err == nil {
		t.Logf("%s - /result completed:\n%s", testName, string(b))
	}
}
