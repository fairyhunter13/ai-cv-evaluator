package tika

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"github.com/stretchr/testify/require"
)

func TestExtractPath_Success(t *testing.T) {
	t.Setenv("TIKA_ALLOW_ABSPATHS", "1")
	// Tika mock returns text
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/plain")
		w.WriteHeader(200)
		_, _ = w.Write([]byte("hello"))
	}))
	defer ts.Close()
	cli := New(ts.URL)
	// Create temp file
	dir := t.TempDir()
	p := filepath.Join(dir, "doc.txt")
	require.NoError(t, os.WriteFile(p, []byte("hi"), 0o600))
	out, err := cli.ExtractPath(context.Background(), "doc.txt", p)
	if err != nil { t.Fatalf("extract: %v", err) }
	if out == "" { t.Fatalf("want non-empty text") }
}

func TestExtractPath_ServerError(t *testing.T) {
	t.Setenv("TIKA_ALLOW_ABSPATHS", "1")
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(500) }))
	defer ts.Close()
	cli := New(ts.URL)
	dir := t.TempDir()
	p := filepath.Join(dir, "doc.pdf")
	require.NoError(t, os.WriteFile(p, []byte("%PDF-1.4\n"), 0o600))
	if _, err := cli.ExtractPath(context.Background(), "doc.pdf", p); err == nil { t.Fatalf("expected error") }
}

func Test_contentTypeFromExt(t *testing.T) {
	if ct := contentTypeFromExt(".pdf"); ct != "application/pdf" { t.Fatalf("ct: %s", ct) }
	if ct := contentTypeFromExt(".docx"); ct != "application/vnd.openxmlformats-officedocument.wordprocessingml.document" { t.Fatalf("ct: %s", ct) }
	if ct := contentTypeFromExt(".txt"); ct != "text/plain" { t.Fatalf("ct: %s", ct) }
}
