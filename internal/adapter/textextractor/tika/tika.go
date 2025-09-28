package tika

import (
	"context"
	"fmt"
	"io"
	"mime"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/fairyhunter13/ai-cv-evaluator/pkg/textx"
)

// Client is a minimal Apache Tika HTTP client implementing domain.TextExtractor.
// It performs PUT /tika with Accept: text/plain to retrieve extracted text.
// See: https://tika.apache.org/server/ for API details.

type Client struct {
	baseURL    string
	httpClient *http.Client
}

func New(baseURL string) *Client {
	return &Client{
		baseURL: baseURL,
		httpClient: &http.Client{Timeout: 15 * time.Second},
	}
}

func (c *Client) ExtractPath(ctx context.Context, fileName, path string) (string, error) {
	f, err := os.Open(path)
	if err != nil { return "", err }
	defer f.Close()

	u := c.baseURL
	if u == "" { u = "http://localhost:9998" }
	req, err := http.NewRequestWithContext(ctx, http.MethodPut, u+"/tika", f)
	if err != nil { return "", err }
	req.Header.Set("Accept", "text/plain")
	// Content-Type best-effort from extension
	ct := contentTypeFromExt(filepath.Ext(fileName))
	if ct != "" { req.Header.Set("Content-Type", ct) }

	resp, err := c.httpClient.Do(req)
	if err != nil { return "", err }
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return "", fmt.Errorf("tika status %d", resp.StatusCode)
	}
	b, err := io.ReadAll(resp.Body)
	if err != nil { return "", err }
	// Sanitize control characters and then collapse all whitespace to single spaces
	sanitized := textx.SanitizeText(string(b))
	fields := strings.Fields(sanitized)
	return strings.Join(fields, " "), nil
}

func contentTypeFromExt(ext string) string {
	ext = strings.ToLower(ext)
	switch ext {
	case ".pdf":
		return "application/pdf"
	case ".docx":
		return "application/vnd.openxmlformats-officedocument.wordprocessingml.document"
	case ".txt":
		return "text/plain"
	default:
		if ext != "" {
			return mime.TypeByExtension(ext)
		}
	}
	return ""
}
