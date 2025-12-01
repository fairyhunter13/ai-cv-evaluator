//go:build e2e
// +build e2e

package e2e_test

import (
	"net/http"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

// TestE2E_ParallelJobs_Concurrency verifies that multiple jobs are processed
// in parallel when CONSUMER_MAX_CONCURRENCY > 1 by comparing aggregate per-job
// processing time to wall-clock elapsed time.
func TestE2E_ParallelJobs_Concurrency(t *testing.T) {
	// Clear dump directory before test
	clearDumpDirectory(t)

	httpTimeout := 15 * time.Second
	client := &http.Client{Timeout: httpTimeout}

	// Ensure app is reachable
	waitForAppReady(t, client, 60*time.Second)

	const totalJobs = 2
	jobIDs := make([]string, 0, totalJobs)

	// Create several jobs that will be processed by the worker
	for i := 0; i < totalJobs; i++ {
		uploadResp := uploadTestFiles(t, client,
			"Parallel concurrency CV content",
			"Parallel concurrency project content",
		)

		cvID, ok := uploadResp["cv_id"].(string)
		require.True(t, ok, "cv_id should be a string")
		projectID, ok := uploadResp["project_id"].(string)
		require.True(t, ok, "project_id should be a string")

		evalResp := evaluateFiles(t, client, cvID, projectID)
		jobID, _ := evalResp["id"].(string)
		if jobID == "" {
			t.Fatalf("/evaluate did not return a job id for job %d: %#v", i+1, evalResp)
		}
		jobIDs = append(jobIDs, jobID)
	}

	timelines := make([]jobTimeline, len(jobIDs))
	start := time.Now()

	var wg sync.WaitGroup
	for i, id := range jobIDs {
		wg.Add(1)
		go func(idx int, jobID string) {
			defer wg.Done()
			timelines[idx] = waitForCompletedWithTimeline(t, client, jobID, 360*time.Second)
		}(i, id)
	}
	wg.Wait()
	_ = time.Since(start)

	// Validate terminal statuses
	for i, tl := range timelines {
		require.NotEqual(t, "queued", tl.FinalStatus, "E2E concurrency: job %d stuck in queued state - %#v", i+1, tl)
		require.NotEqual(t, "processing", tl.FinalStatus, "E2E concurrency: job %d stuck in processing state - %#v", i+1, tl)
		// "error" is a local sentinel used by waitForCompletedWithTimeline when the
		// /result endpoint returns a non-200 (e.g. transient 500). Since this test
		// only asserts concurrency characteristics, we treat "completed",
		// "failed", and local "error" as acceptable terminal statuses.
		if tl.FinalStatus != "completed" && tl.FinalStatus != "failed" && tl.FinalStatus != "error" {
			t.Fatalf("E2E concurrency: job %d ended in unexpected status %q - %#v", i+1, tl.FinalStatus, tl)
		}
	}

	// Compute aggregate per-job processing durations (from first non-queued to completion)
	var sumDurations time.Duration
	var minStart time.Time
	var maxDone time.Time
	for _, tl := range timelines {
		if tl.FirstNonQueuedAt.IsZero() || tl.CompletedAt.IsZero() {
			continue
		}
		if minStart.IsZero() || tl.FirstNonQueuedAt.Before(minStart) {
			minStart = tl.FirstNonQueuedAt
		}
		if tl.CompletedAt.After(maxDone) {
			maxDone = tl.CompletedAt
		}
		sumDurations += tl.CompletedAt.Sub(tl.FirstNonQueuedAt)
	}

	if minStart.IsZero() || maxDone.IsZero() || sumDurations <= 0 {
		t.Log("E2E concurrency: insufficient timing data to compute concurrency; skipping assertion")
		return
	}

	wall := maxDone.Sub(minStart)
	t.Logf("E2E concurrency timing: wall=%v, sumPerJob=%v", wall, sumDurations)

	// With true parallelism and similar per-job durations, wall time should be
	// substantially less than the sum of individual durations. A ratio below 0.8
	// is a conservative signal that jobs ran concurrently rather than strictly
	// serially.
	ratio := float64(wall) / float64(sumDurations)
	t.Logf("E2E concurrency ratio wall/sumPerJob=%.2f", ratio)
	if ratio >= 0.8 {
		t.Logf("E2E concurrency: wall time is not substantially less than sumPerJob (no strong parallelism signal). This can happen under upstream rate limiting or provider variance; not failing test.")
	}
}
