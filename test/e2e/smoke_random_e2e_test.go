//go:build e2e

package e2e_test

import (
    "math/rand"
    "net/http"
    "path/filepath"
    "strings"
    "testing"
    "time"

    "github.com/stretchr/testify/require"
)

// TestE2E_SmokeRandom uploads a random CV/project pair from testdata and ensures evaluate enqueues and result endpoint responds.
func TestE2E_SmokeRandom(t *testing.T) {
    if testing.Short() { t.Skip("short mode") }

    httpTimeout := 2 * time.Second
    client := &http.Client{Timeout: httpTimeout}

    // quick health check
    if resp, err := client.Get("http://localhost:8080/healthz"); err != nil || (resp != nil && resp.StatusCode != http.StatusOK) {
        if resp != nil { resp.Body.Close() }
        t.Skip("App not available; skipping smoke random E2E")
    } else if resp != nil { resp.Body.Close() }

    // pick random pair from test/testdata
    pairs := availablePairs()
    require.NotEmpty(t, pairs)
    rand.Seed(time.Now().UnixNano())
    p := pairs[rand.Intn(len(pairs))]

    // upload & evaluate
    upload := uploadTestFiles(t, client, string(p.CV), string(p.Project))
    eval := evaluateFiles(t, client, upload["cv_id"], upload["project_id"])

    // result endpoint should answer quickly
    req, _ := http.NewRequest("GET", strings.Replace(baseURL, "/v1", "", 1)+"/result/"+eval["id"].(string), nil)
    resp, err := client.Do(req)
    require.NoError(t, err)
    _ = resp.Body.Close()
}

type pair struct{ CV, Project []byte }

func availablePairs() []pair {
    files := []struct{ cv, project string }{
        {"cv_01.txt", "project_01.txt"},
        {"cv_02.txt", "project_02.txt"},
        {"cv_03.txt", "project_03.txt"},
        {"cv_04.txt", "project_04.txt"},
        {"cv_05.txt", "project_05.txt"},
        {"cv_06.txt", "project_06.txt"},
        {"cv_07.txt", "project_07.txt"},
        {"cv_08.txt", "project_08.txt"},
        {"cv_09.txt", "project_09.txt"},
        {"cv_10.txt", "project_10.txt"},
        {"cv_11.txt", "project_11.txt"},
    }
    out := make([]pair, 0, len(files))
    for _, f := range files {
        cvP := filepath.FromSlash("test/testdata/"+f.cv)
        prP := filepath.FromSlash("test/testdata/"+f.project)
        cvB, _ := osReadFile(cvP)
        prB, _ := osReadFile(prP)
        if len(cvB) > 0 && len(prB) > 0 {
            out = append(out, pair{CV: cvB, Project: prB})
        }
    }
    return out
}

// helper to avoid importing os directly here;
// implemented below to keep a minimal import list in this file
func osReadFile(p string) ([]byte, error) {
    return osReadFileImpl(p)
}
