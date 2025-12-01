//go:build e2e
// +build e2e

// Package e2e_test provides end-to-end tests for the AI CV Evaluator.
//
// This file contains the lightweight "core" E2E test suite designed to be
// rate-limit-friendly and safe to run multiple times consecutively without
// hitting provider rate limits. It uses:
//   - Very short CV/project texts to minimize token usage
//   - Inter-job cooldown delays to stay under RPM limits
//   - All 4 provider accounts (2 Groq + 2 OpenRouter) via automatic fallback
//   - Conservative timeouts that fit under the 5-minute global test timeout
//
// This suite is intended for CI pipelines where tests may run frequently.
package e2e_test

import (
	"net/http"
	"testing"
	"time"
)

// Core E2E configuration constants designed for rate-limit safety.
const (
	// corePerJobTimeout is the maximum wait time for a single job to complete.
	// 90 seconds is generous for short prompts while fitting under global timeout.
	corePerJobTimeout = 90 * time.Second

	// coreHTTPTimeout is the HTTP client timeout for individual requests.
	coreHTTPTimeout = 15 * time.Second

	// coreAppReadyTimeout is the maximum time to wait for the app to be ready.
	coreAppReadyTimeout = 60 * time.Second
)

// TestE2E_Core_SingleJob is a minimal single-job test for quick validation.
// This is the fastest possible E2E test, using minimal texts and a single job.
func TestE2E_Core_SingleJob(t *testing.T) {
	clearDumpDirectory(t)

	client := &http.Client{Timeout: coreHTTPTimeout}
	waitForAppReady(t, client, coreAppReadyTimeout)

	t.Log("=== Core E2E Single Job Test ===")

	// Use the shortest possible texts
	cvText := "Go dev. APIs, DBs."
	projectText := "REST API project."

	t.Logf("Using minimal texts (CV: %d chars, Project: %d chars)", len(cvText), len(projectText))

	// Upload
	uploadResp := uploadTestFiles(t, client, cvText, projectText)
	dumpJSON(t, "core_single_upload.json", uploadResp)

	cvID, ok := uploadResp["cv_id"].(string)
	if !ok || cvID == "" {
		t.Fatalf("cv_id missing: %#v", uploadResp)
	}
	projectID, ok := uploadResp["project_id"].(string)
	if !ok || projectID == "" {
		t.Fatalf("project_id missing: %#v", uploadResp)
	}

	// Evaluate
	evalResp := evaluateFiles(t, client, cvID, projectID)
	dumpJSON(t, "core_single_evaluate.json", evalResp)

	jobID, _ := evalResp["id"].(string)
	if jobID == "" {
		t.Fatalf("/evaluate did not return job id: %#v", evalResp)
	}

	// Wait for completion
	final := waitForCompleted(t, client, jobID, corePerJobTimeout)
	dumpJSON(t, "core_single_result.json", final)

	st, _ := final["status"].(string)

	switch st {
	case "completed":
		t.Log("✅ Single job completed successfully")
		res, ok := final["result"].(map[string]any)
		if !ok {
			t.Error("result object missing")
		} else {
			t.Logf("Result: cv_match_rate=%v, project_score=%v",
				res["cv_match_rate"], res["project_score"])
		}

	case "failed":
		errObj, ok := final["error"].(map[string]any)
		if !ok {
			t.Fatalf("failed without error object: %#v", final)
		}
		code, _ := errObj["code"].(string)
		if code == "UPSTREAM_RATE_LIMIT" || code == "UPSTREAM_TIMEOUT" {
			t.Logf("⚠️ Job failed with %s (acceptable in constrained environment)", code)
		} else {
			t.Fatalf("Job failed with unexpected code: %s", code)
		}

	case "processing":
		// Job still processing after timeout - acceptable for slow LLM responses
		t.Logf("⚠️ Job still processing after timeout (slow LLM response, acceptable)")

	case "queued":
		t.Fatal("Job stuck in queued state (worker not picking up jobs)")

	default:
		t.Fatalf("Unknown status: %s", st)
	}
}
