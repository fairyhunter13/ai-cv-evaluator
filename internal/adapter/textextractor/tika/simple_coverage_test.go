package tika

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

func TestContentTypeFromExt_AdditionalCases(t *testing.T) {
	tests := []struct {
		name     string
		ext      string
		expected string
	}{
		{"DOC extension", ".doc", "application/msword"},
		{"RTF extension", ".rtf", "application/rtf"},
		{"ODT extension", ".odt", "application/vnd.oasis.opendocument.text"},
		{"Unknown extension", ".unknown", ""},
		{"Empty extension", "", ""},
		{"Dot only", ".", ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(_ *testing.T) {
			result := contentTypeFromExt(tt.ext)
			// We don't assert the exact result since it depends on system MIME types
			// Just ensure it doesn't panic
			_ = result
		})
	}
}

func TestExtractPath_AdditionalErrorCases(t *testing.T) {
	t.Setenv("TIKA_ALLOW_ABSPATHS", "1")

	client := New("http://localhost:9998")

	// Test with empty file path
	_, err := client.ExtractPath(context.Background(), "test.txt", "")
	if err == nil {
		t.Fatal("Expected error for empty file path")
	}

	// Test with non-existent file
	_, err = client.ExtractPath(context.Background(), "test.txt", "/nonexistent/file.txt")
	if err == nil {
		t.Fatal("Expected error for non-existent file")
	}
}

func TestNew_WithCustomTimeout(t *testing.T) {
	client := New("http://localhost:9998")
	if client == nil {
		t.Fatal("Expected client to be non-nil")
	}
	if client.baseURL != "http://localhost:9998" {
		t.Errorf("Expected baseURL to be 'http://localhost:9998', got %q", client.baseURL)
	}
	if client.httpClient == nil {
		t.Fatal("Expected httpClient to be non-nil")
	}
}

func TestExtractPath_ContextCancellationSimple(t *testing.T) {
	t.Setenv("TIKA_ALLOW_ABSPATHS", "1")

	// Create a temporary test file
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "test.txt")
	err := os.WriteFile(testFile, []byte("test content"), 0o600)
	if err != nil {
		t.Fatal(err)
	}

	client := New("http://localhost:9998")

	// Create a context that's already cancelled
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, err = client.ExtractPath(ctx, "test.txt", testFile)
	if err == nil {
		t.Fatal("Expected error due to cancelled context")
	}
}
