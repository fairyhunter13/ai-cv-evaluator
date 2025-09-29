//go:build e2e

package e2e_test

import (
	"bytes"
	"encoding/json"
	"io"
	"mime/multipart"
	"net/http"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

type uploadResp struct{ CVID string `json:"cv_id"`; ProjectID string `json:"project_id"` }

type evalEnqueueResp struct{ ID string `json:"id"`; Status string `json:"status"` }

func TestLive_E2E(t *testing.T) {
	baseURL := getenv("BASE_URL", "http://localhost:8080")
	// Require OpenRouter chat keys for real calls
	if os.Getenv("OPENROUTER_API_KEY") == "" {
		t.Skip("OPENROUTER_API_KEY not set; skipping live E2E")
	}

	client := &http.Client{Timeout: 15 * time.Second}

	// 1) Upload
	cvPath := filepath.FromSlash("test/testdata/cv.txt")
	prPath := filepath.FromSlash("test/testdata/project.txt")
	body := &bytes.Buffer{}
	w := multipart.NewWriter(body)
	addFilePart(t, w, "cv", cvPath)
	addFilePart(t, w, "project", prPath)
	require.NoError(t, w.Close())

	req, err := http.NewRequest(http.MethodPost, baseURL+"/v1/upload", body)
	require.NoError(t, err)
	req.Header.Set("Content-Type", w.FormDataContentType())
	req.Header.Set("Accept", "application/json")
	resp, err := client.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()
	require.Equal(t, 200, resp.StatusCode)
	var up uploadResp
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&up))
	require.NotEmpty(t, up.CVID)
	require.NotEmpty(t, up.ProjectID)

	// 2) Enqueue evaluate
	payload := map[string]any{
		"cv_id": up.CVID,
		"project_id": up.ProjectID,
		"job_description": "Backend engineer building APIs, DBs, cloud, prompt design, chaining and RAG.",
		"study_case_brief": "Evaluate CV and project implementing LLM workflows, retries, and observability.",
	}
	b, _ := json.Marshal(payload)
	req, err = http.NewRequest(http.MethodPost, baseURL+"/v1/evaluate", bytes.NewReader(b))
	require.NoError(t, err)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Idempotency-Key", "live-e2e-001")
	resp, err = client.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()
	require.Equal(t, 200, resp.StatusCode)
	var enq evalEnqueueResp
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&enq))
	require.NotEmpty(t, enq.ID)

	// 3) Poll result
	deadline := time.Now().Add(2 * time.Minute)
	for {
		if time.Now().After(deadline) {
			t.Fatal("timeout waiting for completed result")
		}
		req, _ = http.NewRequest(http.MethodGet, baseURL+"/v1/result/"+enq.ID, nil)
		req.Header.Set("Accept", "application/json")
		resp, err = client.Do(req)
		require.NoError(t, err)
		b, _ = io.ReadAll(resp.Body); _ = resp.Body.Close()
		// minimal checks: must contain status and result when completed
		var m map[string]any
		require.NoError(t, json.Unmarshal(b, &m))
		st, _ := m["status"].(string)
		if st == "completed" {
			res, ok := m["result"].(map[string]any)
			require.True(t, ok)
			_, hasMR := res["cv_match_rate"]
			_, hasPS := res["project_score"]
			require.True(t, hasMR && hasPS)
			break
		}
		time.Sleep(1 * time.Second)
	}
}

func addFilePart(t *testing.T, w *multipart.Writer, field, p string) {
	f, err := os.Open(p)
	require.NoError(t, err)
	defer f.Close()
	part, err := w.CreateFormFile(field, filepath.Base(p))
	require.NoError(t, err)
	_, err = io.Copy(part, f)
	require.NoError(t, err)
}

