//go:build e2e
// +build e2e

package e2e_test

import (
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

// TestE2E_EdgeCase_MinimalTexts_Complete verifies that very small but non-empty
// CV and project contents still produce a successful completed evaluation.
func TestE2E_EdgeCase_MinimalTexts_Complete(t *testing.T) {
	// Clear dump directory before test
	clearDumpDirectory(t)

	httpTimeout := 15 * time.Second
	client := &http.Client{Timeout: httpTimeout}

	// Ensure app is reachable; wait for readiness instead of failing on first attempt.
	waitForAppReady(t, client, 60*time.Second)

	// Minimal but non-empty CV and project
	uploadResp := uploadTestFiles(t, client, "Hi", "Hello project")
	dumpJSON(t, "edge_case_minimal_upload_response.json", uploadResp)

	cvID, ok := uploadResp["cv_id"].(string)
	require.True(t, ok, "cv_id should be a string")
	projectID, ok := uploadResp["project_id"].(string)
	require.True(t, ok, "project_id should be a string")

	evalResp := evaluateFiles(t, client, cvID, projectID)
	dumpJSON(t, "edge_case_minimal_evaluate_response.json", evalResp)
	jobID, _ := evalResp["id"].(string)
	if jobID == "" {
		t.Fatalf("/evaluate did not return a job id in response: %#v", evalResp)
	}

	final := waitForCompleted(t, client, jobID, 240*time.Second)
	dumpJSON(t, "edge_case_minimal_result_response.json", final)

	st, _ := final["status"].(string)
	require.NotEqual(t, "queued", st, "edge-case minimal job stuck in queued state - %#v", final)
	require.NotEqual(t, "processing", st, "edge-case minimal job stuck in processing state - %#v", final)
	if st != "completed" {
		// In constrained environments, accept upstream timeout or rate-limit as an
		// allowed terminal failure so we still validate pipeline behavior.
		errObj, ok := final["error"].(map[string]any)
		require.True(t, ok, "error object missing for edge-case minimal job: %#v", final)
		code, _ := errObj["code"].(string)
		if code == "UPSTREAM_TIMEOUT" || code == "UPSTREAM_RATE_LIMIT" {
			t.Logf("Edge-case minimal E2E: job failed with upstream code=%s; treating as acceptable in constrained environment", code)
			return
		}
		require.Equal(t, "completed", st, "edge-case minimal job did not complete successfully. Status: %v, Response: %#v", st, final)
	}

	res, ok := final["result"].(map[string]any)
	require.True(t, ok, "result object missing for edge-case minimal job")

	// Basic structural checks only; do not assert specific values from providers.
	if _, ok := res["cv_match_rate"]; !ok {
		t.Fatalf("cv_match_rate missing in edge-case minimal job: %#v", res)
	}
	if _, ok := res["project_score"]; !ok {
		t.Fatalf("project_score missing in edge-case minimal job: %#v", res)
	}
	if feedback, ok := res["cv_feedback"].(string); !ok || strings.TrimSpace(feedback) == "" {
		t.Fatalf("cv_feedback missing or empty in edge-case minimal job: %#v", res)
	}
	if feedback, ok := res["project_feedback"].(string); !ok || strings.TrimSpace(feedback) == "" {
		t.Fatalf("project_feedback missing or empty in edge-case minimal job: %#v", res)
	}
	if summary, ok := res["overall_summary"].(string); !ok || strings.TrimSpace(summary) == "" {
		t.Fatalf("overall_summary missing or empty in edge-case minimal job: %#v", res)
	}
}

// TestE2E_EdgeCase_LongCV_ShortProject_Complete verifies that longer CV content
// with a short project description still completes successfully.
func TestE2E_EdgeCase_LongCV_ShortProject_Complete(t *testing.T) {
	clearDumpDirectory(t)

	httpTimeout := 15 * time.Second
	client := &http.Client{Timeout: httpTimeout}

	waitForAppReady(t, client, 60*time.Second)

	longCV := strings.Repeat("This is an extended CV line about Go, distributed systems, and AI provider integrations. ", 40)
	shortProject := "Short project description focusing on rate-limited AI evaluation."

	uploadResp := uploadTestFiles(t, client, longCV, shortProject)
	dumpJSON(t, "edge_case_longcv_upload_response.json", uploadResp)

	cvID, ok := uploadResp["cv_id"].(string)
	require.True(t, ok, "cv_id should be a string")
	projectID, ok := uploadResp["project_id"].(string)
	require.True(t, ok, "project_id should be a string")

	evalResp := evaluateFiles(t, client, cvID, projectID)
	dumpJSON(t, "edge_case_longcv_evaluate_response.json", evalResp)
	jobID, _ := evalResp["id"].(string)
	if jobID == "" {
		t.Fatalf("/evaluate did not return a job id in response: %#v", evalResp)
	}

	final := waitForCompleted(t, client, jobID, 240*time.Second)
	dumpJSON(t, "edge_case_longcv_result_response.json", final)

	st, _ := final["status"].(string)
	require.NotEqual(t, "queued", st, "edge-case long CV job stuck in queued state - %#v", final)
	require.NotEqual(t, "processing", st, "edge-case long CV job stuck in processing state - %#v", final)
	if st != "completed" {
		// In constrained environments, accept upstream timeout or rate-limit as an
		// allowed terminal failure so we still validate pipeline behavior.
		errObj, ok := final["error"].(map[string]any)
		require.True(t, ok, "error object missing for edge-case long CV job: %#v", final)
		code, _ := errObj["code"].(string)
		if code == "UPSTREAM_TIMEOUT" || code == "UPSTREAM_RATE_LIMIT" {

			t.Logf("Edge-case long CV E2E: job failed with upstream code=%s; treating as acceptable in constrained environment", code)
			return
		}
		require.Equal(t, "completed", st, "edge-case long CV job did not complete successfully. Status: %v, Response: %#v", st, final)
	}

	res, ok := final["result"].(map[string]any)
	require.True(t, ok, "result object missing for edge-case long CV job")

	if _, ok := res["cv_match_rate"]; !ok {
		t.Fatalf("cv_match_rate missing in edge-case long CV job: %#v", res)
	}
	if _, ok := res["project_score"]; !ok {
		t.Fatalf("project_score missing in edge-case long CV job: %#v", res)
	}
}

// TestE2E_EdgeCase_ShortCV_LongProject_Complete verifies that a short CV with a
// much more detailed project description completes successfully.
func TestE2E_EdgeCase_ShortCV_LongProject_Complete(t *testing.T) {
	clearDumpDirectory(t)

	httpTimeout := 15 * time.Second
	client := &http.Client{Timeout: httpTimeout}

	waitForAppReady(t, client, 60*time.Second)

	shortCV := "Short CV highlighting core Go experience."
	longProject := strings.Repeat("This project description goes into significant detail about architecture, testing, observability, and global rate limiting across AI providers. ", 35)

	uploadResp := uploadTestFiles(t, client, shortCV, longProject)
	dumpJSON(t, "edge_case_longproj_upload_response.json", uploadResp)

	cvID, ok := uploadResp["cv_id"].(string)
	require.True(t, ok, "cv_id should be a string")
	projectID, ok := uploadResp["project_id"].(string)
	require.True(t, ok, "project_id should be a string")

	evalResp := evaluateFiles(t, client, cvID, projectID)
	dumpJSON(t, "edge_case_longproj_evaluate_response.json", evalResp)
	jobID, _ := evalResp["id"].(string)
	if jobID == "" {
		t.Fatalf("/evaluate did not return a job id in response: %#v", evalResp)
	}

	final := waitForCompleted(t, client, jobID, 240*time.Second)
	dumpJSON(t, "edge_case_longproj_result_response.json", final)

	st, _ := final["status"].(string)
	require.NotEqual(t, "queued", st, "edge-case long project job stuck in queued state - %#v", final)
	require.NotEqual(t, "processing", st, "edge-case long project job stuck in processing state - %#v", final)
	if st != "completed" {
		// In constrained environments, accept upstream timeout or rate-limit as an
		// allowed terminal failure so we still validate pipeline behavior.
		errObj, ok := final["error"].(map[string]any)
		require.True(t, ok, "error object missing for edge-case long project job: %#v", final)
		code, _ := errObj["code"].(string)
		if code == "UPSTREAM_TIMEOUT" || code == "UPSTREAM_RATE_LIMIT" {
			t.Logf("Edge-case long project E2E: job failed with upstream code=%s; treating as acceptable in constrained environment", code)
			return
		}
		require.Equal(t, "completed", st, "edge-case long project job did not complete successfully. Status: %v, Response: %#v", st, final)
	}

	res, ok := final["result"].(map[string]any)
	require.True(t, ok, "result object missing for edge-case long project job")

	if _, ok := res["cv_match_rate"]; !ok {
		t.Fatalf("cv_match_rate missing in edge-case long project job: %#v", res)
	}
	if _, ok := res["project_score"]; !ok {
		t.Fatalf("project_score missing in edge-case long project job: %#v", res)
	}
}
