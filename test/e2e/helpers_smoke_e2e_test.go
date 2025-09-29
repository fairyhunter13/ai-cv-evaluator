//go:build e2e

package e2e_test

import (
    "bytes"
    "encoding/json"
    "mime/multipart"
    "net/http"
    "testing"
    "time"

    "github.com/stretchr/testify/require"
)

const baseURL = "http://localhost:8080/v1"

// uploadTestFiles uploads provided CV and project contents and returns ids.
func uploadTestFiles(t *testing.T, client *http.Client, cvContent, projectContent string) map[string]string {
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

// evaluateFiles enqueues evaluation and returns job response body.
func evaluateFiles(t *testing.T, client *http.Client, cvID, projectID string) map[string]interface{} {
    payload := map[string]string{
        "cv_id":            cvID,
        "project_id":       projectID,
        "job_description":  defaultJobDescription,
        "study_case_brief": defaultStudyCaseBrief,
    }

    body, _ := json.Marshal(payload)
    var lastStatus int
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
