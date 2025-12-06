// Package qdrant provides a minimal Qdrant HTTP client used by the app.
//
// It handles vector database operations for RAG functionality.
// The package provides methods for collection management,
// vector operations, and similarity search.
package qdrant

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/fairyhunter13/ai-cv-evaluator/internal/observability"
	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
)

// Client is a minimal Qdrant HTTP client used by the app.
type Client struct {
	baseURL    string
	apiKey     string
	httpClient *http.Client
	obs        *observability.IntegratedObservableClient
}

// New constructs a Qdrant client with baseURL and optional apiKey.
func New(baseURL, apiKey string) *Client {
	// Use otelhttp transport for distributed tracing
	transport := otelhttp.NewTransport(http.DefaultTransport,
		otelhttp.WithSpanNameFormatter(func(_ string, r *http.Request) string {
			return fmt.Sprintf("Qdrant %s %s", r.Method, r.URL.Path)
		}),
	)
	return &Client{
		baseURL:    baseURL,
		apiKey:     apiKey,
		httpClient: &http.Client{Timeout: 10 * time.Second, Transport: transport},
		obs: observability.NewIntegratedObservableClient(
			observability.ConnectionTypeVectorDB,
			observability.OperationTypeQuery,
			baseURL,
			"qdrant",
			10*time.Second,
			2*time.Second,
			30*time.Second,
		),
	}
}

// EnsureCollection creates the collection if it does not exist.
func (c *Client) EnsureCollection(ctx context.Context, name string, vectorSize int, distance string) error {
	return c.obs.ExecuteWithMetrics(ctx, "ensure_collection", func(callCtx context.Context) error {
		// GET /collections/{name}
		req, err := http.NewRequestWithContext(callCtx, http.MethodGet, fmt.Sprintf("%s/collections/%s", c.baseURL, name), nil)
		if err != nil {
			return err
		}
		c.setHeaders(req)
		resp, err := c.httpClient.Do(req)
		if err != nil {
			return err
		}
		defer func() { _ = resp.Body.Close() }()
		if resp.StatusCode == http.StatusOK {
			return nil
		}
		// Create
		payload := map[string]any{
			"vectors": map[string]any{"size": vectorSize, "distance": distance},
		}
		b, _ := json.Marshal(payload)
		req, err = http.NewRequestWithContext(callCtx, http.MethodPut, fmt.Sprintf("%s/collections/%s", c.baseURL, name), bytes.NewReader(b))
		if err != nil {
			return err
		}
		c.setHeaders(req)
		req.Header.Set("Content-Type", "application/json")
		resp, err = c.httpClient.Do(req)
		if err != nil {
			return err
		}
		defer func() { _ = resp.Body.Close() }()
		if resp.StatusCode < 200 || resp.StatusCode >= 300 {
			return fmt.Errorf("qdrant ensure create status %d", resp.StatusCode)
		}
		return nil
	})
}

// UpsertPoints inserts or updates points in a collection.
// vectors: list of float32 slices; payloads: matching metadata per point; ids: optional custom ids (len must match if provided)
func (c *Client) UpsertPoints(ctx context.Context, collection string, vectors [][]float32, payloads []map[string]any, ids []any) error {
	if len(vectors) != len(payloads) {
		return fmt.Errorf("vectors and payloads length mismatch")
	}
	points := make([]map[string]any, 0, len(vectors))
	for i := range vectors {
		pt := map[string]any{
			"vector":  vectors[i],
			"payload": payloads[i],
		}
		if ids != nil && len(ids) == len(vectors) {
			pt["id"] = ids[i]
		}
		points = append(points, pt)
	}
	body := map[string]any{"points": points}
	return c.obs.ExecuteWithMetrics(ctx, "upsert_points", func(callCtx context.Context) error {
		b, _ := json.Marshal(body)
		req, err := http.NewRequestWithContext(callCtx, http.MethodPut, fmt.Sprintf("%s/collections/%s/points", c.baseURL, collection), bytes.NewReader(b))
		if err != nil {
			return err
		}
		c.setHeaders(req)
		req.Header.Set("Content-Type", "application/json")
		resp, err := c.httpClient.Do(req)
		if err != nil {
			return err
		}
		defer func() { _ = resp.Body.Close() }()
		if resp.StatusCode < 200 || resp.StatusCode >= 300 {
			return fmt.Errorf("qdrant upsert status %d", resp.StatusCode)
		}
		return nil
	})
}

// Search returns top-k nearest points for a given vector.
func (c *Client) Search(ctx context.Context, collection string, vector []float32, topK int) ([]map[string]any, error) {
	body := map[string]any{"vector": vector, "limit": topK, "with_payload": true}
	var result []map[string]any
	if err := c.obs.ExecuteWithMetrics(ctx, "search", func(callCtx context.Context) error {
		b, _ := json.Marshal(body)
		req, err := http.NewRequestWithContext(callCtx, http.MethodPost, fmt.Sprintf("%s/collections/%s/points/search", c.baseURL, collection), bytes.NewReader(b))
		if err != nil {
			return err
		}
		c.setHeaders(req)
		req.Header.Set("Content-Type", "application/json")
		resp, err := c.httpClient.Do(req)
		if err != nil {
			return err
		}
		defer func() { _ = resp.Body.Close() }()
		if resp.StatusCode < 200 || resp.StatusCode >= 300 {
			return fmt.Errorf("qdrant search status %d", resp.StatusCode)
		}
		var out struct {
			Result []map[string]any `json:"result"`
		}
		if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
			return err
		}
		result = out.Result
		return nil
	}); err != nil {
		return nil, err
	}
	return result, nil
}

// Ping checks if the Qdrant service is accessible.
func (c *Client) Ping(ctx context.Context) error {
	return c.obs.ExecuteWithMetrics(ctx, "ping", func(callCtx context.Context) error {
		req, err := http.NewRequestWithContext(callCtx, http.MethodGet, fmt.Sprintf("%s/collections", c.baseURL), nil)
		if err != nil {
			return fmt.Errorf("create ping request: %w", err)
		}
		c.setHeaders(req)

		resp, err := c.httpClient.Do(req)
		if err != nil {
			return fmt.Errorf("ping request failed: %w", err)
		}
		defer func() { _ = resp.Body.Close() }()

		if resp.StatusCode < 200 || resp.StatusCode >= 300 {
			return fmt.Errorf("ping failed with status %d", resp.StatusCode)
		}

		return nil
	})
}

func (c *Client) setHeaders(req *http.Request) {
	if c.apiKey != "" {
		req.Header.Set("api-key", c.apiKey)
	}
}
