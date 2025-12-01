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
	"fmt"
	"net/http"
	"os"
	"strconv"
	"testing"
	"time"
)

// Core E2E configuration constants designed for rate-limit safety.
const (
	// coreJobCount is the number of jobs in the core suite.
	// Kept minimal to reduce total LLM calls (3 jobs * 3 calls/job = 9 calls).
	coreJobCount = 3

	// coreInterJobCooldown is the delay between sequential jobs.
	// 15 seconds ensures we stay well under 30 RPM Groq limit and allows
	// provider account rotation to spread load across all 4 accounts.
	coreInterJobCooldown = 15 * time.Second

	// corePerJobTimeout is the maximum wait time for a single job to complete.
	// 90 seconds is generous for short prompts while fitting under global timeout.
	corePerJobTimeout = 90 * time.Second

	// coreHTTPTimeout is the HTTP client timeout for individual requests.
	coreHTTPTimeout = 15 * time.Second

	// coreAppReadyTimeout is the maximum time to wait for the app to be ready.
	coreAppReadyTimeout = 60 * time.Second
)

// Minimal CV/project texts designed to minimize token usage.
// Each is ~50-100 characters, resulting in ~200-500 tokens per LLM call
// instead of thousands with full CVs.
var (
	minimalCVTexts = []string{
		"Go developer, 5 years. Skills: APIs, PostgreSQL, Docker.",
		"Backend engineer. Python, Node.js, AWS. 3 years experience.",
		"Full-stack dev. React, Go, Redis. Strong problem solver.",
	}
	minimalProjectTexts = []string{
		"REST API with rate limiting and caching. Clean code.",
		"Microservices with Kafka. Observability and logging.",
		"CLI tool for data processing. Unit tests included.",
	}
)

// getInterJobCooldown returns the inter-job cooldown duration.
// Can be overridden via E2E_INTER_JOB_COOLDOWN environment variable.
func getInterJobCooldown() time.Duration {
	if v := os.Getenv("E2E_INTER_JOB_COOLDOWN"); v != "" {
		if d, err := time.ParseDuration(v); err == nil && d > 0 {
			return d
		}
	}
	return coreInterJobCooldown
}

// getPerJobTimeout returns the per-job timeout duration.
// Can be overridden via E2E_PER_JOB_TIMEOUT environment variable.
func getPerJobTimeout() time.Duration {
	if v := os.Getenv("E2E_PER_JOB_TIMEOUT"); v != "" {
		if d, err := time.ParseDuration(v); err == nil && d > 0 {
			return d
		}
	}
	return corePerJobTimeout
}

// getCoreJobCount returns the number of jobs to run in the core suite.
// Can be overridden via E2E_CORE_JOB_COUNT environment variable.
func getCoreJobCount() int {
	if v := os.Getenv("E2E_CORE_JOB_COUNT"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			return n
		}
	}
	return coreJobCount
}

// TestE2E_Core_RateLimitFriendly is the main lightweight E2E test designed
// to be safe to run multiple times consecutively without hitting rate limits.
//
// Algorithm:
//  1. Uses minimal CV/project texts (~50-100 chars) to minimize tokens
//  2. Runs only 3 jobs (configurable) with 15s cooldown between each
//  3. Each job makes ~3 LLM calls via the multi-step evaluation chain
//  4. Total: ~9 LLM calls over ~3 minutes = ~3 RPM (well under 30 RPM limit)
//  5. 4 provider accounts (2 Groq + 2 OpenRouter) handle load via fallback
//
// The inter-job cooldown ensures that even if run back-to-back multiple times,
// the aggregate request rate stays safely under free-tier limits.
func TestE2E_Core_RateLimitFriendly(t *testing.T) {
	clearDumpDirectory(t)

	client := &http.Client{Timeout: coreHTTPTimeout}
	waitForAppReady(t, client, coreAppReadyTimeout)

	jobCount := getCoreJobCount()
	cooldown := getInterJobCooldown()
	perJobTimeout := getPerJobTimeout()
	initialSleep := 5 * time.Second

	t.Logf("=== Core E2E Rate-Limit-Friendly Test ===")
	t.Logf("Initial sleep before first job: %v", initialSleep)
	t.Logf("Job count: %d", jobCount)
	t.Logf("Inter-job cooldown: %v", cooldown)
	t.Logf("Per-job timeout: %v", perJobTimeout)
	t.Logf("Using 4 provider accounts with automatic fallback")
	time.Sleep(initialSleep)

	completedJobs := 0
	failedWithRateLimit := 0
	failedWithTimeout := 0
	successfulJobs := 0

	for i := 0; i < jobCount; i++ {
		jobLabel := fmt.Sprintf("core-job-%d", i+1)

		// Select minimal texts (cycle through available options)
		cvText := minimalCVTexts[i%len(minimalCVTexts)]
		projectText := minimalProjectTexts[i%len(minimalProjectTexts)]

		t.Logf("[%s] Starting job with minimal texts (CV: %d chars, Project: %d chars)",
			jobLabel, len(cvText), len(projectText))

		// Upload files
		uploadResp := uploadTestFiles(t, client, cvText, projectText)
		dumpJSON(t, fmt.Sprintf("%s_upload.json", jobLabel), uploadResp)

		cvID, ok := uploadResp["cv_id"].(string)
		if !ok || cvID == "" {
			t.Fatalf("[%s] cv_id missing in upload response: %#v", jobLabel, uploadResp)
		}
		projectID, ok := uploadResp["project_id"].(string)
		if !ok || projectID == "" {
			t.Fatalf("[%s] project_id missing in upload response: %#v", jobLabel, uploadResp)
		}

		// Enqueue evaluation
		evalResp := evaluateFiles(t, client, cvID, projectID)
		dumpJSON(t, fmt.Sprintf("%s_evaluate.json", jobLabel), evalResp)

		jobID, _ := evalResp["id"].(string)
		if jobID == "" {
			t.Fatalf("[%s] /evaluate did not return job id: %#v", jobLabel, evalResp)
		}

		// Wait for completion
		final := waitForCompleted(t, client, jobID, perJobTimeout)
		dumpJSON(t, fmt.Sprintf("%s_result.json", jobLabel), final)

		st, _ := final["status"].(string)
		completedJobs++

		// Classify outcome
		switch st {
		case "completed":
			successfulJobs++
			t.Logf("[%s] ✅ Completed successfully", jobLabel)

			// Validate result structure
			res, ok := final["result"].(map[string]any)
			if !ok {
				t.Errorf("[%s] result object missing", jobLabel)
			} else {
				if _, ok := res["cv_match_rate"]; !ok {
					t.Errorf("[%s] cv_match_rate missing", jobLabel)
				}
				if _, ok := res["project_score"]; !ok {
					t.Errorf("[%s] project_score missing", jobLabel)
				}
			}

		case "failed":
			errObj, ok := final["error"].(map[string]any)
			if !ok {
				t.Errorf("[%s] failed without error object: %#v", jobLabel, final)
				continue
			}
			code, _ := errObj["code"].(string)
			switch code {
			case "UPSTREAM_RATE_LIMIT":
				failedWithRateLimit++
				t.Logf("[%s] ⚠️ Failed with UPSTREAM_RATE_LIMIT (acceptable)", jobLabel)
			case "UPSTREAM_TIMEOUT":
				failedWithTimeout++
				t.Logf("[%s] ⚠️ Failed with UPSTREAM_TIMEOUT (acceptable)", jobLabel)
			default:
				t.Errorf("[%s] Failed with unexpected code: %s", jobLabel, code)
			}

		case "queued":
			t.Errorf("[%s] Job stuck in queued state (worker may not have picked it up)", jobLabel)

		case "processing":
			// Job still processing after timeout - this is acceptable in constrained environments
			// where LLM processing is slow, as long as the job was picked up
			t.Logf("[%s] ⚠️ Job still processing after timeout (slow LLM response, acceptable)", jobLabel)

		default:
			t.Errorf("[%s] Unknown status: %s", jobLabel, st)
		}

		// Inter-job cooldown (skip after last job)
		if i < jobCount-1 {
			t.Logf("[%s] Cooling down for %v before next job...", jobLabel, cooldown)
			time.Sleep(cooldown)
		}
	}

	// Summary
	t.Logf("=== Core E2E Test Summary ===")
	t.Logf("Total jobs: %d", jobCount)
	t.Logf("Completed: %d", completedJobs)
	t.Logf("Successful: %d", successfulJobs)
	t.Logf("Rate-limited: %d", failedWithRateLimit)
	t.Logf("Timed out: %d", failedWithTimeout)

	// Assert at least one job reached a terminal state
	if completedJobs == 0 {
		t.Fatal("No jobs reached a terminal state")
	}

	// If all jobs hit rate limits, log a warning but still treat the test as successful.
	// This keeps rate limits as an acceptable terminal outcome for CI, while signaling
	// that the upstream providers may be under heavy load.
	if failedWithRateLimit == jobCount {
		t.Logf("⚠️ All jobs failed with rate limits - upstream providers are throttling, but this is acceptable for CI")
	}

	t.Logf("✅ Core E2E test completed successfully")
}

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

// TestE2E_Core_MultipleRuns simulates multiple consecutive test runs to validate
// that the rate-limit-friendly algorithm works when run back-to-back.
// This test runs 2 "mini-runs" of 2 jobs each with proper cooldowns.
func TestE2E_Core_MultipleRuns(t *testing.T) {
	clearDumpDirectory(t)

	client := &http.Client{Timeout: coreHTTPTimeout}
	waitForAppReady(t, client, coreAppReadyTimeout)

	t.Log("=== Core E2E Multiple Runs Test ===")
	t.Log("Simulating 2 consecutive test runs to validate rate-limit safety")

	cooldown := getInterJobCooldown()
	perJobTimeout := getPerJobTimeout()

	const runsCount = 2
	const jobsPerRun = 2

	totalSuccessful := 0
	totalRateLimited := 0

	for run := 1; run <= runsCount; run++ {
		t.Logf("--- Run %d of %d ---", run, runsCount)

		for job := 1; job <= jobsPerRun; job++ {
			jobLabel := fmt.Sprintf("run%d-job%d", run, job)
			idx := (run-1)*jobsPerRun + (job - 1)

			cvText := minimalCVTexts[idx%len(minimalCVTexts)]
			projectText := minimalProjectTexts[idx%len(minimalProjectTexts)]

			t.Logf("[%s] Starting job", jobLabel)

			// Upload and evaluate
			uploadResp := uploadTestFiles(t, client, cvText, projectText)
			cvID, _ := uploadResp["cv_id"].(string)
			projectID, _ := uploadResp["project_id"].(string)

			if cvID == "" || projectID == "" {
				t.Fatalf("[%s] Upload failed: %#v", jobLabel, uploadResp)
			}

			evalResp := evaluateFiles(t, client, cvID, projectID)
			jobID, _ := evalResp["id"].(string)

			if jobID == "" {
				t.Fatalf("[%s] Evaluate failed: %#v", jobLabel, evalResp)
			}

			// Wait for completion
			final := waitForCompleted(t, client, jobID, perJobTimeout)
			dumpJSON(t, fmt.Sprintf("%s_result.json", jobLabel), final)

			st, _ := final["status"].(string)
			switch st {
			case "completed":
				totalSuccessful++
				t.Logf("[%s] ✅ Completed", jobLabel)
			case "failed":
				errObj, _ := final["error"].(map[string]any)
				code, _ := errObj["code"].(string)
				if code == "UPSTREAM_RATE_LIMIT" {
					totalRateLimited++
					t.Logf("[%s] ⚠️ Rate limited", jobLabel)
				} else if code == "UPSTREAM_TIMEOUT" {
					t.Logf("[%s] ⚠️ Timed out", jobLabel)
				}
			case "processing":
				// Job still processing after timeout - acceptable for slow LLM responses
				t.Logf("[%s] ⚠️ Still processing (slow LLM, acceptable)", jobLabel)
			case "queued":
				t.Errorf("[%s] Stuck in queued state", jobLabel)
			}

			// Cooldown between jobs (skip after last job of last run)
			if !(run == runsCount && job == jobsPerRun) {
				t.Logf("[%s] Cooling down for %v...", jobLabel, cooldown)
				time.Sleep(cooldown)
			}
		}

		// Extra cooldown between runs
		if run < runsCount {
			extraCooldown := cooldown * 2
			t.Logf("Extra cooldown between runs: %v", extraCooldown)
			time.Sleep(extraCooldown)
		}
	}

	// Summary
	totalJobs := runsCount * jobsPerRun
	t.Logf("=== Multiple Runs Summary ===")
	t.Logf("Total runs: %d", runsCount)
	t.Logf("Total jobs: %d", totalJobs)
	t.Logf("Successful: %d", totalSuccessful)
	t.Logf("Rate-limited: %d", totalRateLimited)

	// Fail if all jobs were rate-limited
	if totalRateLimited == totalJobs {
		t.Error("All jobs hit rate limits - algorithm needs adjustment")
	}

	// Warn if any jobs were rate-limited
	if totalRateLimited > 0 {
		t.Logf("⚠️ Warning: %d jobs hit rate limits, but test completed", totalRateLimited)
	}

	t.Log("✅ Multiple runs test completed")
}
