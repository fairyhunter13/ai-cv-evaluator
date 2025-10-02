package qdrant_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/fairyhunter13/ai-cv-evaluator/internal/adapter/vector/qdrant"
)

func TestClient_EnsureCollection(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		collection string
		vectorSize int
		distance   string
		handler    http.HandlerFunc
		wantErr    bool
	}{
		{
			name:       "collection already exists",
			collection: "existing_collection",
			vectorSize: 1536,
			distance:   "Cosine",
			handler: func(w http.ResponseWriter, r *http.Request) {
				if r.Method == http.MethodGet {
					w.WriteHeader(http.StatusOK)
					require.NoError(t, json.NewEncoder(w).Encode(map[string]any{"result": "ok"}))
				}
			},
			wantErr: false,
		},
		{
			name:       "create new collection",
			collection: "new_collection",
			vectorSize: 768,
			distance:   "Dot",
			handler: func(w http.ResponseWriter, r *http.Request) {
				if r.Method == http.MethodGet {
					w.WriteHeader(http.StatusNotFound)
					return
				}
				if r.Method == http.MethodPut {
					var payload map[string]any
					require.NoError(t, json.NewDecoder(r.Body).Decode(&payload))

					// Verify payload structure
					vectors := payload["vectors"].(map[string]any)
					assert.Equal(t, float64(768), vectors["size"])
					assert.Equal(t, "Dot", vectors["distance"])

					w.WriteHeader(http.StatusOK)
					require.NoError(t, json.NewEncoder(w).Encode(map[string]any{"result": "ok"}))
				}
			},
			wantErr: false,
		},
		{
			name:       "server error",
			collection: "error_collection",
			vectorSize: 1536,
			distance:   "Cosine",
			handler: func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(http.StatusInternalServerError)
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			server := httptest.NewServer(tt.handler)
			defer server.Close()

			client := qdrant.New(server.URL, "test-api-key")
			ctx := context.Background()

			err := client.EnsureCollection(ctx, tt.collection, tt.vectorSize, tt.distance)

			if tt.wantErr {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func TestClient_UpsertPoints(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		collection string
		vectors    [][]float32
		payloads   []map[string]any
		ids        []any
		handler    http.HandlerFunc
		wantErr    bool
	}{
		{
			name:       "successful upsert",
			collection: "test_collection",
			vectors:    [][]float32{{0.1, 0.2, 0.3}},
			payloads:   []map[string]any{{"text": "test text", "type": "cv"}},
			ids:        []any{uuid.New().String()},
			handler: func(w http.ResponseWriter, r *http.Request) {
				assert.Equal(t, http.MethodPut, r.Method)
				assert.Contains(t, r.URL.Path, "/collections/test_collection/points")

				var payload map[string]any
				require.NoError(t, json.NewDecoder(r.Body).Decode(&payload))

				// Verify points structure
				points := payload["points"].([]any)
				assert.Len(t, points, 1)

				w.WriteHeader(http.StatusOK)
				require.NoError(t, json.NewEncoder(w).Encode(map[string]any{"result": "ok"}))
			},
			wantErr: false,
		},
		{
			name:       "multiple points",
			collection: "multi_collection",
			vectors:    [][]float32{{0.1}, {0.2}, {0.3}},
			payloads:   []map[string]any{{"idx": 1}, {"idx": 2}, {"idx": 3}},
			ids:        []any{"1", "2", "3"},
			handler: func(w http.ResponseWriter, r *http.Request) {
				var payload map[string]any
				require.NoError(t, json.NewDecoder(r.Body).Decode(&payload))

				points := payload["points"].([]any)
				assert.Len(t, points, 3)

				w.WriteHeader(http.StatusOK)
				require.NoError(t, json.NewEncoder(w).Encode(map[string]any{"result": "ok"}))
			},
			wantErr: false,
		},
		{
			name:       "server error",
			collection: "error_collection",
			vectors:    [][]float32{{0.1}},
			payloads:   []map[string]any{{"test": "data"}},
			ids:        []any{"1"},
			handler: func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(http.StatusBadRequest)
				require.NoError(t, json.NewEncoder(w).Encode(map[string]any{"error": "bad request"}))
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			server := httptest.NewServer(tt.handler)
			defer server.Close()

			client := qdrant.New(server.URL, "test-api-key")
			ctx := context.Background()

			err := client.UpsertPoints(ctx, tt.collection, tt.vectors, tt.payloads, tt.ids)

			if tt.wantErr {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func TestClient_Search(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		collection string
		vector     []float32
		limit      int
		handler    http.HandlerFunc
		wantCount  int
		wantErr    bool
	}{
		{
			name:       "successful search",
			collection: "search_collection",
			vector:     []float32{0.1, 0.2, 0.3},
			limit:      5,
			handler: func(w http.ResponseWriter, r *http.Request) {
				assert.Equal(t, http.MethodPost, r.Method)
				assert.Contains(t, r.URL.Path, "/collections/search_collection/points/search")

				var payload map[string]any
				require.NoError(t, json.NewDecoder(r.Body).Decode(&payload))

				// Verify search parameters
				assert.Equal(t, float64(5), payload["limit"])
				assert.NotNil(t, payload["vector"])
				assert.Equal(t, true, payload["with_payload"])

				// Return mock results
				w.WriteHeader(http.StatusOK)
				require.NoError(t, json.NewEncoder(w).Encode(map[string]any{
					"result": []map[string]any{
						{
							"id":      "match-1",
							"score":   0.95,
							"payload": map[string]any{"text": "best match", "weight": 0.5},
						},
						{
							"id":      "match-2",
							"score":   0.85,
							"payload": map[string]any{"text": "good match", "weight": 0.3},
						},
					},
				}))
			},
			wantCount: 2,
			wantErr:   false,
		},
		{
			name:       "empty results",
			collection: "empty_collection",
			vector:     []float32{0.1},
			limit:      10,
			handler: func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(http.StatusOK)
				require.NoError(t, json.NewEncoder(w).Encode(map[string]any{
					"result": []map[string]any{},
				}))
			},
			wantCount: 0,
			wantErr:   false,
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			server := httptest.NewServer(tt.handler)
			defer server.Close()

			client := qdrant.New(server.URL, "test-api-key")
			ctx := context.Background()

			results, err := client.Search(ctx, tt.collection, tt.vector, tt.limit)

			if tt.wantErr {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
				assert.Len(t, results, tt.wantCount)

				// Verify result structure
				for _, result := range results {
					assert.NotEmpty(t, result["id"])
					if score, ok := result["score"].(float64); ok {
						assert.GreaterOrEqual(t, score, 0.0)
					}
					assert.NotNil(t, result["payload"])
				}
			}
		})
	}
}

func TestClient_Ping(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		handler http.HandlerFunc
		wantErr bool
	}{
		{
			name: "successful ping",
			handler: func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(http.StatusOK)
				require.NoError(t, json.NewEncoder(w).Encode(map[string]any{"result": "ok"}))
			},
			wantErr: false,
		},
		{
			name: "ping with server error",
			handler: func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(http.StatusInternalServerError)
			},
			wantErr: true,
		},
		{
			name: "ping with not found",
			handler: func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(http.StatusNotFound)
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			server := httptest.NewServer(tt.handler)
			defer server.Close()

			client := qdrant.New(server.URL, "test-api-key")

			err := client.Ping(context.Background())
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}
