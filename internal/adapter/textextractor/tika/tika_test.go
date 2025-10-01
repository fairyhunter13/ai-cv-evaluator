package tika_test

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/fairyhunter13/ai-cv-evaluator/internal/adapter/textextractor/tika"
)

func TestClient_ExtractPath(t *testing.T) {
	t.Setenv("TIKA_ALLOW_ABSPATHS", "1")

	// Create a temporary test file
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "test.txt")
	err := os.WriteFile(testFile, []byte("This is test content"), 0o600)
	require.NoError(t, err)

	tests := []struct {
		name     string
		fileName string
		filePath string
		handler  http.HandlerFunc
		want     string
		wantErr  bool
		errMsg   string
	}{
		{
			name:     "successful text extraction",
			fileName: "test.txt",
			filePath: testFile,
			handler: func(w http.ResponseWriter, r *http.Request) {
				assert.Equal(t, http.MethodPut, r.Method)
				assert.Equal(t, "/tika", r.URL.Path)
				assert.Equal(t, "text/plain", r.Header.Get("Accept"))

				body, _ := io.ReadAll(r.Body)
				assert.Equal(t, "This is test content", string(body))

				w.WriteHeader(http.StatusOK)
				_, _ = w.Write([]byte("Extracted text content"))
			},
			want:    "Extracted text content",
			wantErr: false,
		},
		{
			name:     "PDF file with content type",
			fileName: "document.pdf",
			filePath: testFile,
			handler: func(w http.ResponseWriter, r *http.Request) {
				assert.Equal(t, "application/pdf", r.Header.Get("Content-Type"))

				w.WriteHeader(http.StatusOK)
				_, _ = w.Write([]byte("PDF content extracted"))
			},
			want:    "PDF content extracted",
			wantErr: false,
		},
		{
			name:     "DOCX file with content type",
			fileName: "document.docx",
			filePath: testFile,
			handler: func(w http.ResponseWriter, r *http.Request) {
				assert.Equal(t, "application/vnd.openxmlformats-officedocument.wordprocessingml.document",
					r.Header.Get("Content-Type"))

				w.WriteHeader(http.StatusOK)
				_, _ = w.Write([]byte("DOCX content extracted"))
			},
			want:    "DOCX content extracted",
			wantErr: false,
		},
		{
			name:     "server error",
			fileName: "test.txt",
			filePath: testFile,
			handler: func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(http.StatusInternalServerError)
				_, _ = w.Write([]byte("Internal Server Error"))
			},
			wantErr: true,
			errMsg:  "tika status 500",
		},
		{
			name:     "file not found",
			fileName: "nonexistent.txt",
			filePath: "/path/to/nonexistent/file.txt",
			handler:  func(_ http.ResponseWriter, _ *http.Request) {},
			wantErr:  true,
			errMsg:   "no such file",
		},
		{
			name:     "unsupported status",
			fileName: "test.txt",
			filePath: testFile,
			handler: func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(http.StatusUnsupportedMediaType)
			},
			wantErr: true,
			errMsg:  "tika status 415",
		},
		{
			name:     "normalized text with special characters",
			fileName: "test.txt",
			filePath: testFile,
			handler: func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(http.StatusOK)
				_, _ = w.Write([]byte("Text with\ttabs\nand\r\nnewlines   and    spaces"))
			},
			want:    "Text with tabs and newlines and spaces",
			wantErr: false,
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {

			server := httptest.NewServer(tt.handler)
			defer server.Close()

			client := tika.New(server.URL)
			ctx := context.Background()

			got, err := client.ExtractPath(ctx, tt.fileName, tt.filePath)

			if tt.wantErr {
				require.Error(t, err)
				if tt.errMsg != "" {
					assert.Contains(t, err.Error(), tt.errMsg)
				}
			} else {
				require.NoError(t, err)
				assert.Equal(t, tt.want, got)
			}
		})
	}
}

func TestNew(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		baseURL string
	}{
		{
			name:    "with base URL",
			baseURL: "http://tika-server:9998",
		},
		{
			name:    "empty base URL",
			baseURL: "",
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			client := tika.New(tt.baseURL)
			assert.NotNil(t, client)
		})
	}
}
